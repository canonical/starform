package starform

import (
	"context"
	"crypto/sha512"
	"fmt"
	"sort"
	"strings"

	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/syntax"
)

type ScriptSet struct {
	options *ScriptSetOptions
}

type ScriptSource interface {
	Path() string
	Content(ctx context.Context) ([]byte, error)
}

type ScriptSetOptions struct {
	Cache               ScriptCache
	PrintHandler        func(thread *starlark.Thread, msg string)
	RequiredSafety      starlark.SafetyFlags
	MaxAllocs, MaxSteps uint64
}

var starlarkDialect = syntax.FileOptions{
	Set:             true,
	While:           false,
	TopLevelControl: false,
	GlobalReassign:  false,
	Recursion:       false,
}

type scriptStatus int

const (
	scriptUninitialised scriptStatus = iota
	scriptInitialising
	scriptInitialised
)

type scriptState struct {
	path        string
	programKey  [sha512.Size384]byte
	program     *starlark.Program
	status      scriptStatus
	toplevelEnv starlark.StringDict
}

func NewScriptSet(options *ScriptSetOptions) (*ScriptSet, error) {
	if options.RequiredSafety.Contains(starlark.MemSafe) && options.MaxAllocs == 0 {
		return nil, fmt.Errorf("cannot run MemSafe Starlark with unbounded MaxAllocs")
	}
	if options.RequiredSafety.Contains(starlark.CPUSafe) && options.MaxSteps == 0 {
		return nil, fmt.Errorf("cannot run CPUSafe Starlark with unbounded MaxSteps")
	}

	return &ScriptSet{
		options: options,
	}, nil
}

func (ss *ScriptSet) LoadSources(ctx context.Context, sources []ScriptSource) error {
	scripts, err := ss.compilePrograms(ctx, sources)
	if err != nil {
		return err
	}
	scriptByPath := make(map[string]*scriptState, len(scripts))
	for _, m := range scripts {
		scriptByPath[m.path] = m
	}
	sort.Slice(scripts, func(i, j int) bool {
		return scripts[i].path < scripts[j].path
	})

	thread := ss.options.makeThread()
	thread.Load = func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
		script, ok := scriptByPath[module]
		if !ok {
			return nil, fmt.Errorf("%s not found", module)
		}
		if err := script.runTopLevel(thread); err != nil {
			return nil, err
		}
		return script.toplevelEnv, nil
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			thread.Cancel("operation canceled")
		case <-done:
		}
	}()
	for _, script := range scripts {
		if err := script.runTopLevel(thread); err != nil {
			return err
		}
	}

	for _, script := range scripts {
		init, ok := script.toplevelEnv["init"]
		if !ok {
			continue
		}
		if _, ok := init.(*starlark.Function); !ok {
			return fmt.Errorf("init must be a Starlark function, got %s", init.Type())
		}

		if _, err := starlark.Call(thread, init, nil, nil); err != nil {
			return fmt.Errorf("cannot load script: %w", err)
		}
	}

	return nil
}

func (ss *ScriptSet) compilePrograms(ctx context.Context, sources []ScriptSource) ([]*scriptState, error) {
	scriptStateStorage := make([]scriptState, 0, len(sources))
	striptStates := make([]*scriptState, 0, len(sources))
	isPredeclared := func(string) bool { return false }
	cache := ss.options.Cache
	if cache == nil {
		cache = &noopScriptCache{}
	}
	for _, source := range sources {
		path := source.Path()
		if !strings.HasSuffix(path, ".star") {
			continue
		}
		if strings.Contains(path, "..") {
			return nil, fmt.Errorf("can't load path with navigation operators")
		}
		content, err := source.Content(ctx)
		if err != nil {
			return nil, fmt.Errorf("cannot read script: %s: %w", path, err)
		}
		programKey := sha512.Sum384(content)
		var program *starlark.Program
		if entry, err := cache.Get(programKey); err == ErrNoCache {
			_, program, err = starlark.SourceProgramOptions(&starlarkDialect, path, content, isPredeclared)
			if err != nil {
				return nil, fmt.Errorf("cannot load script: %s: %w", path, err)
			}
			cache.Put(programKey, program, source)
		} else if err != nil {
			return nil, err
		} else if prog, ok := entry.(*starlark.Program); ok {
			program = prog
		} else {
			return nil, fmt.Errorf("unknown cache value: %v", entry)
		}
		scriptStateStorage = append(scriptStateStorage, scriptState{
			path:       path,
			programKey: programKey,
			program:    program,
		})
		striptStates = append(striptStates, &scriptStateStorage[len(scriptStateStorage)-1])
	}
	return striptStates, nil
}

func (script *scriptState) runTopLevel(thread *starlark.Thread) error {
	switch script.status {
	case scriptInitialising:
		return fmt.Errorf("load cycle detected")
	case scriptUninitialised:
		script.status = scriptInitialising
		toplevelEnv, err := script.program.Init(thread, nil)
		if err != nil {
			return err
		}
		script.toplevelEnv = toplevelEnv
		script.status = scriptInitialised
	}
	return nil
}

func (options *ScriptSetOptions) makeThread() *starlark.Thread {
	thread := &starlark.Thread{}
	thread.Print = options.PrintHandler
	thread.RequireSafety(options.RequiredSafety)
	thread.SetMaxSteps(options.MaxSteps)
	thread.SetMaxAllocs(options.MaxAllocs)
	return thread
}

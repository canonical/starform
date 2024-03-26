package starform

import (
	"context"
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

type loadingScript struct {
	status         scriptStatus
	path           string
	source         ScriptSource
	compiledSource *starlark.Program
	toplevelEnv    starlark.StringDict
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
	loadingScripts, err := ss.compileSources(ctx, sources)
	if err != nil {
		return err
	}
	loadingScriptByPath := make(map[string]*loadingScript, len(loadingScripts))
	for _, m := range loadingScripts {
		loadingScriptByPath[m.path] = m
	}
	sort.Slice(loadingScripts, func(i, j int) bool {
		return loadingScripts[i].path < loadingScripts[j].path
	})

	thread := ss.options.makeThread()
	thread.SetContext(ctx)
	thread.Load = func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
		script, ok := loadingScriptByPath[module]
		if !ok {
			return nil, fmt.Errorf("%s not found", module)
		}
		if err := script.runTopLevel(thread); err != nil {
			return nil, err
		}
		return script.toplevelEnv, nil
	}

	for _, script := range loadingScripts {
		if err := script.runTopLevel(thread); err != nil {
			return err
		}
	}

	for _, script := range loadingScripts {
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

func (ss *ScriptSet) compileSources(ctx context.Context, sources []ScriptSource) ([]*loadingScript, error) {
	loadingScriptStorage := make([]loadingScript, 0, len(sources))
	loadingScripts := make([]*loadingScript, 0, len(sources))
	isPredeclared := func(string) bool { return false }
	for _, source := range sources {
		path := source.Path()
		if !strings.HasSuffix(path, ".star") {
			continue
		}
		content, err := source.Content(ctx)
		if err != nil {
			return nil, fmt.Errorf("cannot read script: %s: %w", path, err)
		}
		_, program, err := starlark.SourceProgramOptions(&starlarkDialect, path, content, isPredeclared)
		if err != nil {
			return nil, fmt.Errorf("cannot load script: %s: %w", path, err)
		}
		loadingScriptStorage = append(loadingScriptStorage, loadingScript{
			path:           path,
			source:         source,
			compiledSource: program,
		})
		loadingScripts = append(loadingScripts, &loadingScriptStorage[len(loadingScriptStorage)-1])
	}
	return loadingScripts, nil
}

func (script *loadingScript) runTopLevel(thread *starlark.Thread) error {
	switch script.status {
	case scriptInitialising:
		return fmt.Errorf("load cycle detected")
	case scriptUninitialised:
		script.status = scriptInitialising
		toplevelEnv, err := script.compiledSource.Init(thread, nil)
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

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
	scriptNotInitialized scriptStatus = iota
	scriptInitializing
	scriptInitialized
)

type scriptLoadInfo struct {
	path      string
	source    ScriptSource
	program   *starlark.Program
	status    scriptStatus
	globalEnv starlark.StringDict
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
	scripts, err := ss.compileSources(ctx, sources)
	if err != nil {
		return err
	}
	scriptByPath := make(map[string]*scriptLoadInfo, len(scripts))
	for _, m := range scripts {
		scriptByPath[m.path] = m
	}
	sort.Slice(scripts, func(i, j int) bool {
		return scripts[i].path < scripts[j].path
	})

	thread := ss.options.makeThread()
	thread.SetContext(ctx)
	thread.Load = func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
		info, ok := scriptByPath[module]
		if !ok {
			return nil, fmt.Errorf("can't find load target %s", module)
		}
		if err := ss.globalInitScript(thread, info); err != nil {
			return nil, err
		}
		return info.globalEnv, nil
	}
	for _, module := range scripts {
		if err := ss.globalInitScript(thread, module); err != nil {
			return err
		}
	}
	for _, m := range scripts {
		init, ok := m.globalEnv["init"]
		if !ok {
			continue
		}
		if _, ok := init.(*starlark.Function); !ok {
			return fmt.Errorf("init must be a Starlark function, got %s", init.Type())
		}
		_, err := starlark.Call(thread, init, nil, nil)
		if err != nil {
			return fmt.Errorf("cannot load script: %w", err)
		}
	}

	return nil
}

func (ss *ScriptSet) compileSources(ctx context.Context, sources []ScriptSource) ([]*scriptLoadInfo, error) {
	moduleStorage := make([]scriptLoadInfo, 0, len(sources))
	modules := make([]*scriptLoadInfo, 0, len(sources))
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
		moduleStorage = append(moduleStorage, scriptLoadInfo{
			path:    path,
			source:  source,
			program: program,
		})
		modules = append(modules, &moduleStorage[len(moduleStorage)-1])
	}
	return modules, nil
}

func (ss *ScriptSet) globalInitScript(thread *starlark.Thread, script *scriptLoadInfo) error {
	if script.status == scriptInitializing {
		return fmt.Errorf("load cycle detected") // TODO: trace
	}
	if script.status == scriptNotInitialized {
		script.status = scriptInitializing
		globalEnv, err := script.program.Init(thread, nil)
		if err != nil {
			return fmt.Errorf("cannot load script: %s: %w", script.path, err)
		}
		script.globalEnv = globalEnv
		script.status = scriptInitialized
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

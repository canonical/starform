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

type module struct { // TODO rename
	path         string
	loadPriority int
	source       ScriptSource
	program      *starlark.Program
	globals      starlark.StringDict
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
	moduleStorage := make([]module, len(sources))
	moduleFs := make(map[string]*module, len(sources))
	modules := make([]*module, 0, len(sources))
	isPredeclared := func(string) bool { return false }
	for i, source := range sources {
		path := source.Path()
		if !strings.HasSuffix(path, ".star") {
			continue
		}
		content, err := source.Content(ctx)
		if err != nil {
			return fmt.Errorf("cannot read script: %s: %w", path, err)
		}
		_, program, err := starlark.SourceProgramOptions(&starlarkDialect, path, content, isPredeclared)
		if err != nil {
			return fmt.Errorf("cannot load script: %s: %w", path, err)
		}
		moduleStorage[i] = module{
			path:    path,
			source:  source,
			program: program,
		}
		modules = append(modules, &moduleStorage[i])
		moduleFs[moduleStorage[i].path] = &moduleStorage[i]
	}
	if err := computeLoadPriority(modules, moduleFs); err != nil {
		return err
	}
	sort.Slice(modules, func(i, j int) bool {
		if modules[i].loadPriority == modules[j].loadPriority {
			return modules[i].path < modules[j].path
		}
		return modules[i].loadPriority < modules[j].loadPriority
	})

	// Load phase
	for _, m := range modules {
		thread := makeThread(ctx, ss.options)
		thread.Load = func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
			return moduleFs[module].globals, nil
		}
		globals, err := m.program.Init(thread, nil)
		if err != nil {
			return fmt.Errorf("cannot load script: %s: %w", m.path, err)
		}
		m.globals = globals
	}

	// Init phase
	for _, m := range modules {
		init, ok := m.globals["init"]
		if !ok {
			continue
		}
		if _, ok := init.(*starlark.Function); !ok {
			return fmt.Errorf("init: expected Starlark function, got %s", init.Type())
		}

		_, err := starlark.Call(makeThread(ctx, ss.options), init, nil, nil)
		if err != nil {
			return fmt.Errorf("cannot load script: %w", err)
		}
	}

	return nil
}

func makeThread(ctx context.Context, options *ScriptSetOptions) *starlark.Thread {
	thread := &starlark.Thread{}
	thread.SetContext(ctx)
	thread.Print = options.PrintHandler
	thread.RequireSafety(options.RequiredSafety)
	thread.SetMaxSteps(options.MaxSteps)
	thread.SetMaxAllocs(options.MaxAllocs)
	return thread
}

func computeLoadPriority(modules []*module, moduleFs map[string]*module) error {
	var visit func(m *module) (int, error)
	visit = func(m *module) (int, error) {
		if m.loadPriority > 0 {
			return m.loadPriority, nil // Already visited.
		}
		if m.loadPriority < 0 {
			// TODO: trace
			return 0, fmt.Errorf("load loop detected")
		}
		m.loadPriority = -1
		maxChildPriority := 1
		for i, numLoads := 0, m.program.NumLoads(); i < numLoads; i++ {
			loadPath, pos := m.program.Load(i)
			loadModule, ok := moduleFs[loadPath]
			if !ok {
				return 0, fmt.Errorf("%s: can't find load target %s", pos, loadPath)
			}
			if childPriority, err := visit(loadModule); err != nil {
				return 0, err
			} else if childPriority > maxChildPriority {
				maxChildPriority = childPriority
			}
		}
		m.loadPriority = maxChildPriority + 1
		return m.loadPriority, nil
	}
	for _, module := range modules {
		if _, err := visit(module); err != nil {
			return err
		}
	}
	return nil
}

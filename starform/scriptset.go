package starform

import (
	"fmt"
	"sort"
	"strings"

	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/syntax"
)

type ScriptSet struct {
	Name string
}

type ScriptSetOptions struct {
	Loader              ScriptLoader
	Cache               ScriptCache
	PrintHandler        func(thread *starlark.Thread, msg string)
	RequiredSafety      starlark.SafetyFlags
	MaxAllocs, MaxSteps uint64
}

func (options *ScriptSetOptions) CheckValid() error {
	if options.Loader == nil {
		return fmt.Errorf("Loader cannot be nil")
	}
	if options.RequiredSafety.Contains(starlark.MemSafe) && options.MaxAllocs == 0 {
		return fmt.Errorf("cannot run starlark with unbounded MaxAllocs")
	}
	if options.RequiredSafety.Contains(starlark.CPUSafe) && options.MaxSteps == 0 {
		return fmt.Errorf("cannot run starlark with unbounded MaxSteps")
	}
	return nil
}

var starlarkDialect = syntax.FileOptions{
	Set:             true,
	While:           false,
	TopLevelControl: false,
	GlobalReassign:  false,
	Recursion:       false,
}

func NewScriptSet(options *ScriptSetOptions, name string) (*ScriptSet, error) {
	if err := options.CheckValid(); err != nil {
		return nil, err
	}
	scriptlets, err := options.Loader.Load(name)
	if err != nil {
		return nil, err
	}
	sort.Slice(scriptlets, func(i, j int) bool {
		return scriptlets[i].Path() < scriptlets[j].Path()
	})

	isPredeclared := func(string) bool { return false }
	modules := make([]starlark.StringDict, 0, len(scriptlets))
	for _, scriptlet := range scriptlets {
		source, err := scriptlet.Content()
		if err != nil {
			return nil, err
		}
		if path := scriptlet.Path(); !strings.HasSuffix(path, ".star") {
			continue
		}

		_, prog, err := starlark.SourceProgramOptions(&starlarkDialect, scriptlet.Path(), source, isPredeclared)
		if err != nil {
			return nil, err
		}
		if prog.NumLoads() > 0 {
			return nil, fmt.Errorf("load statements not supported")
		}

		module, err := prog.Init(makeThread(options), nil)
		if err != nil {
			return nil, err
		}
		modules = append(modules, module)
	}

	for _, module := range modules {
		init, ok := module["init"]
		if !ok {
			continue
		}
		if _, ok := init.(starlark.Callable); !ok {
			continue
		}

		_, err := starlark.Call(makeThread(options), init, nil, nil)
		if err != nil {
			return nil, err
		}
	}

	return &ScriptSet{
		Name: name,
	}, nil
}

func makeThread(options *ScriptSetOptions) *starlark.Thread {
	thread := &starlark.Thread{
		Print: options.PrintHandler,
	}
	thread.RequireSafety(options.RequiredSafety)
	thread.SetMaxSteps(options.MaxSteps)
	thread.SetMaxAllocs(options.MaxAllocs)
	return thread
}

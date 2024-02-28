package starform

import (
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
	Content() ([]byte, error)
}

type ScriptSetOptions struct {
	Sources             []ScriptSource
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

func NewScriptSet(options *ScriptSetOptions) (*ScriptSet, error) {
	if options.RequiredSafety.Contains(starlark.MemSafe) && options.MaxAllocs == 0 {
		return nil, fmt.Errorf("cannot run starlark with unbounded MaxAllocs")
	}
	if options.RequiredSafety.Contains(starlark.CPUSafe) && options.MaxSteps == 0 {
		return nil, fmt.Errorf("cannot run starlark with unbounded MaxSteps")
	}

	sort.Slice(options.Sources, func(i, j int) bool {
		return options.Sources[i].Path() < options.Sources[j].Path()
	})

	isPredeclared := func(string) bool { return false }
	modules := make([]starlark.StringDict, 0, len(options.Sources))
	for _, source := range options.Sources {
		content, err := source.Content()
		if err != nil {
			return nil, err
		}
		// TODO: check for proper naming convention here
		if path := source.Path(); !strings.HasSuffix(path, ".star") {
			continue
		}

		_, prog, err := starlark.SourceProgramOptions(&starlarkDialect, source.Path(), content, isPredeclared)
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
		options: options,
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

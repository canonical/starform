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
	Content(context.Context) ([]byte, error)
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

func NewScriptSet(ctx context.Context, options *ScriptSetOptions) (*ScriptSet, error) {
	if options.RequiredSafety.Contains(starlark.MemSafe) && options.MaxAllocs == 0 {
		return nil, fmt.Errorf("cannot run MemSafe Starlark with unbounded MaxAllocs")
	}
	if options.RequiredSafety.Contains(starlark.CPUSafe) && options.MaxSteps == 0 {
		return nil, fmt.Errorf("cannot run CPUSafe Starlark with unbounded MaxSteps")
	}

	sort.Slice(options.Sources, func(i, j int) bool {
		return options.Sources[i].Path() < options.Sources[j].Path()
	})

	isPredeclared := func(string) bool { return false }
	modules := make([]starlark.StringDict, 0, len(options.Sources))
	for _, source := range options.Sources {
		path := source.Path()
		if !strings.HasSuffix(path, ".star") {
			continue
		}
		content, err := source.Content(ctx)
		if err != nil {
			return nil, fmt.Errorf("cannot read script: %s: %w", path, err)
		}
		_, prog, err := starlark.SourceProgramOptions(&starlarkDialect, path, content, isPredeclared)
		if err != nil {
			return nil, fmt.Errorf("cannot load script: %s: %w", path, err)
		}
		if prog.NumLoads() > 0 {
			return nil, fmt.Errorf("load statements not supported")
		}

		module, err := prog.Init(makeThread(ctx, options), nil)
		if err != nil {
			return nil, fmt.Errorf("cannot load script: %s: %w", path, err)
		}
		modules = append(modules, module)
	}

	for _, module := range modules {
		init, ok := module["init"]
		if !ok {
			continue
		}
		if _, ok := init.(*starlark.Function); !ok {
			return nil, fmt.Errorf("cannot call non-function init")
		}

		_, err := starlark.Call(makeThread(ctx, options), init, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("cannot load script: %w", err)
		}
	}

	return &ScriptSet{
		options: options,
	}, nil
}

func makeThread(ctx context.Context, options *ScriptSetOptions) *starlark.Thread {
	thread := &starlark.Thread{
		Print: options.PrintHandler,
	}
	thread.RequireSafety(options.RequiredSafety)
	thread.SetMaxSteps(options.MaxSteps)
	thread.SetMaxAllocs(options.MaxAllocs)
	thread.SetContext(ctx)
	return thread
}

package starform

import (
	"fmt"
	"sort"
	"strings"

	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/syntax"
)

type ExtensionSet struct {
	loader              ExtensionLoader
	cache               ExtensionCache
	printHandler        func(thread *starlark.Thread, msg string)
	requiredSafety      starlark.SafetyFlags
	maxAllocs, maxSteps uint64
}

type ExtensionSetOptions struct {
	Loader              ExtensionLoader
	Cache               ExtensionCache
	PrintHandler        func(thread *starlark.Thread, msg string)
	RequiredSafety      starlark.SafetyFlags
	MaxAllocs, MaxSteps uint64
}

func NewExtensionSet(options *ExtensionSetOptions) (*ExtensionSet, error) {
	if options.Loader == nil {
		return nil, fmt.Errorf("Loader cannot be nil")
	}
	if options.RequiredSafety.Contains(starlark.MemSafe) && options.MaxAllocs == 0 {
		return nil, fmt.Errorf("cannot run starlark with unbounded MaxAllocs")
	}
	if options.RequiredSafety.Contains(starlark.CPUSafe) && options.MaxSteps == 0 {
		return nil, fmt.Errorf("cannot run starlark with unbounded MaxSteps")
	}

	result := &ExtensionSet{
		printHandler:   options.PrintHandler,
		loader:         options.Loader,
		maxAllocs:      options.MaxAllocs,
		maxSteps:       options.MaxSteps,
		requiredSafety: options.RequiredSafety,
		cache:          options.Cache,
	}
	if result.cache == nil {
		result.cache = NoopExtensionCache
	}
	return result, nil
}

var starlarkDialect = syntax.FileOptions{
	Set:             true,
	While:           false,
	TopLevelControl: false,
	GlobalReassign:  false,
	Recursion:       false,
}

func (es *ExtensionSet) Load(name string) (*Extension, error) {
	scriptlets, err := es.loader.Load(name)
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

		module, err := prog.Init(es.makeThread(), nil)
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

		_, err := starlark.Call(es.makeThread(), init, nil, nil)
		if err != nil {
			return nil, err
		}
	}

	return &Extension{
		Name: name,
	}, nil
}

func (es *ExtensionSet) makeThread() *starlark.Thread {
	thread := &starlark.Thread{
		Print: es.printHandler,
	}
	thread.RequireSafety(es.requiredSafety)
	thread.SetMaxSteps(es.maxSteps)
	thread.SetMaxAllocs(es.maxAllocs)
	return thread
}

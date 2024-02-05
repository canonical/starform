package starform

import (
	"fmt"
	"sort"

	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/syntax"
)

type ExtensionSet struct {
	printHandler        func(thread *starlark.Thread, msg string) // FIXME non so se mi piace
	loader              ScriptletLoader
	cache               ScriptletCache
	requiredSafety      starlark.SafetyFlags
	maxAllocs, maxSteps uint64
}

type ExtensionSetOptions struct {
	PrintHandler        func(thread *starlark.Thread, msg string) // FIXME non so se mi piace
	Loader              ScriptletLoader
	RequireSafety       starlark.SafetyFlags
	MaxAllocs, MaxSteps uint64
	Cache               ScriptletCache
}

func NewExtensionSet(options ExtensionSetOptions) (*ExtensionSet, error) {
	if options.Loader == nil {
		return nil, fmt.Errorf("Loader cannot be nil")
	}

	result := &ExtensionSet{
		printHandler:   options.PrintHandler,
		loader:         options.Loader,
		maxAllocs:      options.MaxAllocs,
		maxSteps:       options.MaxSteps,
		requiredSafety: options.RequireSafety,
		cache:          options.Cache,
	}
	if result.cache == nil {
		result.cache = DefaultScriptletCache
	}
	return result, nil
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
	modules := make([]starlark.StringDict, len(scriptlets))
	for i, scriptlet := range scriptlets {
		source, err := scriptlet.Content()
		if err != nil {
			return nil, err
		}

		_, comp, err := starlark.SourceProgramOptions(&starlarkDialect, scriptlet.Path(), source, isPredeclared)
		if err != nil {
			return nil, err
		}
		if numLoads := comp.NumLoads(); numLoads > 0 {
			return nil, fmt.Errorf("load statements not yet supported: %s has %d", scriptlet.Path(), numLoads)
		}

		module, err := comp.Init(es.makeThread(), nil)
		if err != nil {
			return nil, err
		}
		modules[i] = module
	}

	for _, module := range modules {
		if init, ok := module["init"]; ok {
			if _, ok := init.(starlark.Callable); !ok {
				continue
			}
			_, err := starlark.Call(es.makeThread(), init, nil, nil)
			if err != nil {
				return nil, err
			}
		}
	}

	return &Extension{
		Name:    name,
		modules: modules,
	}, nil
}

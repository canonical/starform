package starform

import (
	"fmt"
	"math"
	"sort"

	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/syntax"
)

type ExtensionSet struct {
	starlarkOptions     syntax.FileOptions
	printHandler        func(thread *starlark.Thread, msg string) // FIXME non so se mi piace
	loader              ScriptLoader
	cache               ScriptCache
	flags               starlark.SafetyFlags
	maxAllocs, maxSteps uint64
}

func (es *ExtensionSet) Load(name string) (*Extension, error) {
	scripts, err := es.loader.Load(name)
	if err != nil {
		return nil, err
	}
	sort.Slice(scripts, func(i, j int) bool {
		return scripts[i].Name() < scripts[j].Name()
	})

	isPredeclared := func(string) bool { return false }
	modules := make([]starlark.StringDict, len(scripts))
	for i, script := range scripts {
		source, err := script.Content()
		if err != nil {
			return nil, err
		}
		_, comp, err := starlark.SourceProgramOptions(&es.starlarkOptions, script.Name(), source, isPredeclared)
		if err != nil {
			return nil, err
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

type ExtensionSetOptions struct {
	PrintHandler        func(thread *starlark.Thread, msg string) // FIXME non so se mi piace
	LanguageOptions     syntax.FileOptions
	Flags               starlark.SafetyFlags
	MaxAllocs, MaxSteps *uint64
	Loader              ScriptLoader
	Cache               ScriptCache
}

func NewExtensionSet(options ExtensionSetOptions) (*ExtensionSet, error) {
	if options.Loader == nil {
		return nil, fmt.Errorf("Loader cannot be nil")
	}

	result := &ExtensionSet{
		printHandler:    options.PrintHandler,
		starlarkOptions: options.LanguageOptions,
		flags:           options.Flags,
		maxAllocs:       math.MaxUint64,
		maxSteps:        math.MaxUint64,
		loader:          options.Loader,
	}

	result.cache = options.Cache
	if result.cache == nil {
		result.cache = DefaultScriptCache
	}

	if options.MaxAllocs != nil {
		result.maxAllocs = *options.MaxAllocs
	}
	if options.MaxSteps != nil {
		result.maxSteps = *options.MaxSteps
	}

	return result, nil
}

func (es *ExtensionSet) makeThread() *starlark.Thread {
	thread := &starlark.Thread{
		Print: es.printHandler,
	}
	thread.RequireSafety(es.flags)
	thread.SetMaxSteps(es.maxSteps)
	thread.SetMaxAllocs(es.maxAllocs)
	return thread
}

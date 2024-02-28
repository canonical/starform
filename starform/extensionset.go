package starform

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/syntax"
)

type ExtensionSet struct {
	loader              ScriptletLoader
	cache               ScriptletCache
	printHandler        func(thread *starlark.Thread, msg string) // FIXME non so se mi piace
	requiredSafety      starlark.SafetyFlags
	maxAllocs, maxSteps uint64
}

type ExtensionSetOptions struct {
	Loader              ScriptletLoader
	Cache               ScriptletCache
	PrintHandler        func(thread *starlark.Thread, msg string) // FIXME non so se mi piace
	RequiredSafety      starlark.SafetyFlags
	MaxAllocs, MaxSteps uint64
}

func NewExtensionSet(options *ExtensionSetOptions) (*ExtensionSet, error) {
	if options.Loader == nil {
		return nil, fmt.Errorf("Loader cannot be nil")
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
		result.cache = DefaultScriptletCache
	}
	return result, nil
}

func (es *ExtensionSet) makeThread() *starlark.Thread {
	thread := &starlark.Thread{
		Print: es.printHandler,
		Load: func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
			// Dumb hack to fix tests.
			// TODO: Remove this in favour of a proper load implementation.
			return starlark.StringDict{
				"unused": starlark.None,
			}, nil
		},
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
	modules := make([]starlark.StringDict, 0, len(scriptlets))
	for _, scriptlet := range scriptlets {
		source, err := scriptlet.Content()
		if err != nil {
			return nil, err
		}

		_, prog, err := starlark.SourceProgramOptions(&starlarkDialect, scriptlet.Path(), source, isPredeclared)
		if err != nil {
			return nil, err
		}
		for i := 0; i < prog.NumLoads(); i++ {
			loadPath, _ := prog.Load(i)
			if err := checkLoadPath(loadPath); err != nil {
				return nil, err
			}
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
		Name:    name,
		modules: modules,
	}, nil
}

func checkLoadPath(path string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot load %q: %v", path, err)
		}
	}()

	if path == "" {
		return errors.New("path is empty")
	}
	if path[0] == '/' {
		return errors.New("path is absolute")
	}
	if !strings.HasSuffix(path, ".star") {
		return errors.New("path must have '.star' extension")
	}

	parsingPathOperator := false
	componentStartIndex := 0
	normalComponentFound := false
	midwayDotFound := false
	prevR := rune(0)
	for i, r := range path {
		rIsAlnum := ('a' <= r && r <= 'z') || ('0' <= r && r <= '9')

		if !(rIsAlnum || r == '_' || r == '.' || r == '/') {
			return fmt.Errorf("path contains nonstandard character %c", r)
		}

		if parsingPathOperator {
			if r == '_' || rIsAlnum {
				return fmt.Errorf("path includes hidden files")
			}
			if r == '.' && i-componentStartIndex > 2 {
				return fmt.Errorf("path includes more than two successive dots")
			}
			if r == '/' && normalComponentFound {
				return fmt.Errorf("path mixes path-operators and normal components")
			}
		}

		if r == '/' {
			if midwayDotFound {
				return fmt.Errorf("path must only use dots in file extensions")
			}
			if i-componentStartIndex < 3 {
				return fmt.Errorf("path components must be at least 3 characters")
			}

			componentStartIndex = i
			parsingPathOperator = false
		}

		switch prevR {
		case '_':
			switch r {
			case '_':
				return fmt.Errorf("path includes `__`")
			case '.':
				return fmt.Errorf("path includes `_.`")
			case '/':
				return fmt.Errorf("path has component which ends with underscore")
			}
		case '/':
			if rIsAlnum {
				normalComponentFound = true
			}
			switch r {
			case '_':
				return fmt.Errorf("path has component which starts with underscore")
			case '.':
				parsingPathOperator = true
			case '/':
				return fmt.Errorf("paths contains successive slashes")
			}
		default:
			if r == '.' {
				if !parsingPathOperator && i-componentStartIndex < 3 {
					return fmt.Errorf("path file stem too short")
				}

				midwayDotFound = true
			}
		}
		prevR = r
	}

	return nil
}

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
	defer func(path string) {
		if err != nil {
			err = fmt.Errorf("cannot load %q: %v", path, err)
		}
	}(path)

	if path == "" {
		return errors.New("path is empty")
	}
	if path[0] == '/' {
		return errors.New("path is absolute")
	}
	if !strings.HasSuffix(path, ".star") {
		return errors.New(`path must have ".star" extension`)
	}
	path = path[:len(path)-len(".star")]

	switch path[len(path)-1] {
	case '_':
		return errors.New(`path contains "_."`)
	case '.':
		return errors.New(`path contains extra "."`)
	case '/':
		return errors.New(`path is directory`)
	}

	if strings.HasPrefix(path, "./") {
		path = path[2:]
	} else {
		for strings.HasPrefix(path, "../") {
			path = path[3:]
		}
	}

	type componentInfo struct {
		startIndex   int
		startsDotDot bool
	}
	var component = componentInfo{}
	var prevComponent componentInfo
	prevR := rune(0)
	for i, r := range path {
		if !(('a' <= r && r <= 'z') || ('0' <= r && r <= '9') || r == '_' || r == '.' || r == '/') {
			return fmt.Errorf(`path contains nonstandard character "%c"`, r)
		}

		if r == '/' {
			if component.startsDotDot {
				return errors.New(`path contains late "../"`)
			}

			prevComponent = component
			component = componentInfo{
				startIndex: i + 1,
			}
		}

		switch prevR {
		case '_':
			switch r {
			case '_':
				return errors.New(`path contains "__"`)
			case '.':
				return errors.New(`path contains "_."`)
			case '/':
				return errors.New(`path has component which ends with "_"`)
			}
		case '/', rune(0):
			switch r {
			case '_':
				return errors.New(`path has component which starts with "_"`)
			case '/':
				// Precondition: path is not absolute.
				return errors.New(`path contains "//"`)
			}
		case '.':
			switch r {
			case '.':
				if component.startsDotDot {
					return errors.New(`path contains "..."`)
				}
				component.startsDotDot = true
			case '/':
				if component.startsDotDot {
					return errors.New(`path contains late "../"`)
				}
				return errors.New(`path contains late "./"`)
			default:
				// Precondition: r is alphanumeric, '_' or '\0'
				return errors.New(`path contains extra "."`)
			}
		default:
			// Precondition: prevR is alphanumeric or '\0'.
			if r == '.' {
				return errors.New(`path contains extra "."`)
			}
		}

		if r == '/' && prevR != '.' && i-prevComponent.startIndex < 3 {
			return fmt.Errorf("path component %q too short", path[prevComponent.startIndex:i])
		}

		prevR = r
	}

	if len(path)-component.startIndex < 3 {
		return errors.New("file name too short")
	}

	return nil
}

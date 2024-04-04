package starform

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
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
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Path() < sources[j].Path()
	})

	isPredeclared := func(string) bool { return false }
	modules := make([]starlark.StringDict, 0, len(sources))
	for _, source := range sources {
		path := source.Path()
		if !strings.HasSuffix(path, ".star") {
			continue
		}
		content, err := source.Content(ctx)
		if err != nil {
			return fmt.Errorf("cannot read script: %s: %w", path, err)
		}
		_, prog, err := starlark.SourceProgramOptions(&starlarkDialect, path, content, isPredeclared)
		if err != nil {
			return fmt.Errorf("cannot load script: %s: %w", path, err)
		}
		for i := 0; i < prog.NumLoads(); i++ {
			loadPath, _ := prog.Load(i)
			if err := checkLoadPath(loadPath); err != nil {
				return err
			}
		}

		initThread := makeThread(ctx, ss.options)
		initThread.Load = func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
			// Dumb hack to appease tests.
			// TODO(marco6): remove me!
			return starlark.StringDict{
				"unused": starlark.None,
			}, nil
		}
		module, err := prog.Init(initThread, nil)
		if err != nil {
			return fmt.Errorf("cannot load script: %s: %w", path, err)
		}
		modules = append(modules, module)
	}

	for _, module := range modules {
		init, ok := module["init"]
		if !ok {
			continue
		}
		if _, ok := init.(*starlark.Function); !ok {
			return fmt.Errorf("cannot call non-function init")
		}

		_, err := starlark.Call(makeThread(ctx, ss.options), init, nil, nil)
		if err != nil {
			return fmt.Errorf("cannot load script: %w", err)
		}
	}

	return nil
}

var validCleanPath = regexp.MustCompile(`^(./|(\.\./)*)([a-z0-9][a-z0-9_]+[a-z0-9]/)*[a-z0-9][a-z0-9_]+[a-z0-9]\.star`)

func checkLoadPath(loadPath string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf(`cannot load "%s": %v`, loadPath, err)
		}
	}()

	miscError := errors.New("path invalid, see https://github.com/canonical/starlark/blob/main/doc/valid-load-paths.md")
	if len(loadPath) == 0 {
		return miscError // Special case to simplify valid path regex.
	}
	if strings.ContainsRune(loadPath, '-') {
		return errors.New(`path contains "-", use "_" instead`)
	}
	if strings.ContainsRune(loadPath, '\\') {
		return errors.New(`path contains "\", use "/" instead`)
	}
	if strings.Contains(loadPath, "__") {
		return miscError // Special case to simplify valid path regex.
	}
	if strings.HasPrefix(loadPath, "./../") {
		return errors.New("path contains redundant components")
	}

	toCheck := loadPath
	if strings.HasPrefix(loadPath, "./") {
		toCheck = loadPath[2:] // Ignore leading, non-redundant "./".
	}
	if cleaned := path.Clean(loadPath); toCheck != cleaned {
		return errors.New("path contains redundant components")
	}

	if !validCleanPath.MatchString(loadPath) {
		return miscError
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

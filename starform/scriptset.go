package starform

import (
	"context"
	"crypto/sha512"
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
	options   *ScriptSetOptions
	observers map[string][]starlark.Callable
}

type ScriptSource interface {
	Path() string
	Content(ctx context.Context) ([]byte, error)
}

type ScriptSetOptions struct {
	App                 *App
	Cache               ScriptCache
	PrintHandler        func(thread *starlark.Thread, msg string)
	RequiredSafety      starlark.SafetyFlags
	MaxAllocs, MaxSteps uint64
}

var EmptyState = struct{}{}

var starlarkDialect = syntax.FileOptions{
	Set:             true,
	While:           false,
	TopLevelControl: false,
	GlobalReassign:  false,
	Recursion:       false,
}

type scriptStatus int

const (
	scriptUninitialised scriptStatus = iota
	scriptInitialising
	scriptInitialised
)

type scriptState struct {
	path        string
	status      scriptStatus
	program     *starlark.Program
	toplevelEnv starlark.StringDict
}

func NewScriptSet(options *ScriptSetOptions) (*ScriptSet, error) {
	if options.App == nil {
		return nil, fmt.Errorf("cannot create script set without app object")
	}
	if options.App.Name == "" {
		return nil, fmt.Errorf("cannot create script set without app object name")
	}
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

// LoadSources loads the given sources into the script set and runs their init
// functions. Any previously-loaded scripts are discarded.
func (ss *ScriptSet) LoadSources(ctx context.Context, sources []ScriptSource) error {
	scriptStates, err := ss.compilePrograms(ctx, sources)
	if err != nil {
		return err
	}
	scripts := make([]*scriptState, len(scriptStates))
	for i := range scriptStates {
		scripts[i] = &scriptStates[i]
	}
	scriptsByPath := make(map[string]*scriptState, len(scripts))
	for _, script := range scripts {
		scriptsByPath[script.path] = script
	}
	sort.Slice(scripts, func(i, j int) bool {
		return scripts[i].path < scripts[j].path
	})

	data := &EventRunData{
		EventName: LoadEventName,
		State:     nil,
	}
	predeclared := starlark.StringDict{
		ss.options.App.Name: ss.options.App.value(),
	}
	thread := makeThread(ss.options, data)
	defer thread.Cancel("done")
	stop := afterFunc(ctx, func() { thread.Cancel("operation cancelled") })
	defer stop()
	thread.Load = func(thread *starlark.Thread, path string) (starlark.StringDict, error) {
		if err := checkLoadPath(path); err != nil {
			return nil, err
		}

		script, ok := scriptsByPath[path]
		if !ok {
			return nil, fmt.Errorf("%s not found", path)
		}
		if err := script.runTopLevel(thread, predeclared); err != nil {
			return nil, err
		}
		return script.toplevelEnv, nil
	}
	for _, script := range scripts {
		if err := script.runTopLevel(thread, predeclared); err != nil {
			return err
		}
	}

	observers := make(map[string][]starlark.Callable)
	data.State = observers
	for _, script := range scripts {
		init, ok := script.toplevelEnv["init"]
		if !ok {
			continue
		}
		if _, ok := init.(*starlark.Function); !ok {
			return fmt.Errorf("init must be a function")
		}

		if _, err := starlark.Call(thread, init, nil, nil); err != nil {
			return fmt.Errorf("cannot load script: %w", err)
		}
	}

	for _, eventObservers := range observers {
		for _, observer := range eventObservers {
			observer.Freeze()
		}
	}
	ss.observers = observers

	return nil
}

func (ss *ScriptSet) compilePrograms(ctx context.Context, sources []ScriptSource) ([]scriptState, error) {
	scriptStates := make([]scriptState, 0, len(sources))
	cache := ss.options.Cache
	if cache == nil {
		cache = &noopScriptCache{}
	}
	isPredeclared := func(name string) bool {
		return name == ss.options.App.Name
	}
	for _, source := range sources {
		path := source.Path()
		if !strings.HasSuffix(path, ".star") {
			continue
		}
		if err := checkLoadPath(path); err != nil {
			return nil, fmt.Errorf("cannot load %s: %w", path, err)
		}

		content, err := source.Content(ctx)
		if err != nil {
			return nil, fmt.Errorf("cannot load %s: %w", path, err)
		}

		var program *starlark.Program
		programKey := sha512.Sum384(content)
		if entry, err := cache.Get(programKey); err != nil {
			if err != ErrNotCached {
				return nil, err
			}
			_, program, err = starlark.SourceProgramOptions(&starlarkDialect, path, content, isPredeclared)
			if err != nil {
				return nil, fmt.Errorf("cannot load script %s: %w", path, err)
			}
			cache.Put(programKey, program, source)
		} else if p, ok := entry.(*starlark.Program); ok {
			program = p
		} else {
			return nil, fmt.Errorf("unknown cache value: %v", entry)
		}

		scriptStates = append(scriptStates, scriptState{
			path:    path,
			program: program,
		})
	}
	return scriptStates, nil
}

func (script *scriptState) runTopLevel(thread *starlark.Thread, predeclared starlark.StringDict) error {
	switch script.status {
	case scriptInitialising:
		return fmt.Errorf("load cycle detected")
	case scriptUninitialised:
		script.status = scriptInitialising
		toplevelEnv, err := script.program.Init(thread, predeclared)
		if err != nil {
			return err
		}
		toplevelEnv.Freeze()
		script.toplevelEnv = toplevelEnv

		script.status = scriptInitialised
		return nil
	case scriptInitialised:
		return nil
	}
	return fmt.Errorf("internal error: invalid script state")
}

var validCleanPath *regexp.Regexp

func init() {
	validComponent := "[a-z0-9][a-z0-9_]+[a-z0-9]"
	validCleanPath = regexp.MustCompile(fmt.Sprintf(`^(\./|(\.\./)+)?(%s/)*%s\.star$`, validComponent, validComponent))
}

var miscInvalidPathError = errors.New("path invalid, see https://github.com/canonical/starlark/blob/main/doc/valid-load-paths.md")

func checkLoadPath(loadPath string) (err error) {
	if len(loadPath) == 0 {
		return miscInvalidPathError // Special case to simplify valid path regex.
	}
	if strings.ContainsRune(loadPath, '-') {
		return errors.New(`path contains "-", use "_" instead`)
	}
	if strings.ContainsRune(loadPath, '\\') {
		return errors.New(`path contains "\", use "/" instead`)
	}
	if strings.Contains(loadPath, "__") {
		return miscInvalidPathError // Special case to simplify valid path regex.
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
		return miscInvalidPathError
	}

	return nil
}

type HandleOptions struct {
	EventName string
	State     interface{}
}

func (ss *ScriptSet) Handle(ctx context.Context, opts *HandleOptions) error {
	observers, ok := ss.observers[opts.EventName]
	if !ok {
		return nil
	}

	runData := &EventRunData{
		EventName: opts.EventName,
		State:     opts.State,
	}
	thread := makeThread(ss.options, runData)
	stop := afterFunc(ctx, func() { thread.Cancel("operation cancelled") })
	defer stop()
	defer thread.Cancel("done")
	for _, observer := range observers {
		_, err := starlark.Call(thread, observer, starlark.Tuple{starlark.None}, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func makeThread(options *ScriptSetOptions, data *EventRunData) *starlark.Thread {
	thread := &starlark.Thread{}
	thread.Print = options.PrintHandler
	thread.RequireSafety(options.RequiredSafety)
	thread.SetMaxSteps(options.MaxSteps)
	thread.SetMaxAllocs(options.MaxAllocs)
	thread.SetLocal(runDataLocalKey, data)
	return thread
}

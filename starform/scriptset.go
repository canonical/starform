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
	options        *ScriptSetOptions
	pathByFilename map[string]string
	observers      map[string][]starlark.Callable
}

type ScriptSource interface {
	Path() string
	Content(ctx context.Context) ([]byte, error)
}

type ScriptSetOptions struct {
	AppObject           *AppObject
	Cache               ScriptCache
	Logger              Logger
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

type scriptStatus int

const (
	scriptUninitialised scriptStatus = iota
	scriptInitialising
	scriptInitialised
)

type scriptState struct {
	path        string
	programKey  [sha512.Size384]byte
	status      scriptStatus
	program     *starlark.Program
	toplevelEnv starlark.StringDict
}

func NewScriptSet(options *ScriptSetOptions) (*ScriptSet, error) {
	if options.AppObject == nil {
		return nil, fmt.Errorf("cannot create script set without app object")
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

	pathByFilename := make(map[string]string, len(scripts))
	scriptsByPath := make(map[string]*scriptState, len(scripts))
	for _, script := range scripts {
		scriptsByPath[script.path] = script
		pathByFilename[script.program.Filename()] = script.path
	}
	sort.Slice(scripts, func(i, j int) bool {
		return scripts[i].path < scripts[j].path
	})

	data := &runData{eventName: LoadEventName, pathByFilename: pathByFilename}
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
		if err := script.runTopLevel(thread, ss.options.AppObject); err != nil {
			return nil, err
		}
		return script.toplevelEnv, nil
	}
	for _, script := range scripts {
		if err := script.runTopLevel(thread, ss.options.AppObject); err != nil {
			return err
		}
	}

	data.observeAvailable = true
	data.observers = make(map[string][]starlark.Callable)
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

	for _, eventObservers := range data.observers {
		for _, observer := range eventObservers {
			observer.Freeze()
		}
	}
	ss.observers = data.observers
	ss.pathByFilename = pathByFilename

	return nil
}

func (ss *ScriptSet) compilePrograms(ctx context.Context, sources []ScriptSource) ([]scriptState, error) {
	scriptStates := make([]scriptState, 0, len(sources))
	cache := ss.options.Cache
	if cache == nil {
		cache = &noopScriptCache{}
	}
	isPredeclared := func(name string) bool {
		return name == ss.options.AppObject.name ||
			name == "debug"
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
			cachedFilename := fmt.Sprintf("cache-%x.star", programKey[:4])
			_, program, err = starlark.SourceProgramOptions(&starlarkDialect, cachedFilename, content, isPredeclared)
			if err != nil {
				if err, ok := err.(*syntax.Error); ok {
					err.Pos = syntax.MakePosition(&path, err.Pos.Line, err.Pos.Col)
				}
				return nil, fmt.Errorf("cannot load script %s: %w", path, err)
			}
			if err := cache.Put(programKey, program, source); err != nil {
				if ss.options.Logger != nil {
					ss.options.Logger.Log(ctx, LogEntry{
						Message:   fmt.Sprintf("failed to put %s into cache: %v", path, err),
						Level:     DebugLevel,
						EventName: LoadEventName,
					})
				}
			}
		} else if p, ok := entry.(*starlark.Program); ok {
			program = p
		} else {
			return nil, fmt.Errorf("unknown cache value: %v", entry)
		}
		scriptStates = append(scriptStates, scriptState{
			path:       path,
			programKey: programKey,
			program:    program,
		})
	}
	return scriptStates, nil
}

func (script *scriptState) runTopLevel(thread *starlark.Thread, app *AppObject) error {
	switch script.status {
	case scriptInitialising:
		return fmt.Errorf("load cycle detected")
	case scriptUninitialised:
		script.status = scriptInitialising
		predeclared := starlark.StringDict{
			"debug":  debugBuiltin,
			app.name: app,
		}
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

func (ss *ScriptSet) Handle(ctx context.Context, eventName string) error {
	observers, ok := ss.observers[eventName]
	if !ok {
		return nil
	}

	data := &runData{eventName: eventName}
	thread := makeThread(ss.options, data)
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

func makeThread(options *ScriptSetOptions, data *runData) *starlark.Thread {
	thread := &starlark.Thread{}
	thread.Print = makePrintFunction(options.Logger)
	thread.PrintSafety = starlark.CPUSafe | starlark.MemSafe | starlark.TimeSafe | starlark.IOSafe
	thread.RequireSafety(options.RequiredSafety)
	thread.SetMaxSteps(options.MaxSteps)
	thread.SetMaxAllocs(options.MaxAllocs)
	thread.SetLocal(runDataLocalKey, data)
	return thread
}

func makePrintFunction(logger Logger) func(thread *starlark.Thread, msg string) {
	if logger == nil {
		// Avoid default print behaviour which writes to stdout.
		return func(thread *starlark.Thread, msg string) {
			// Do nothing.
		}
	}

	return func(thread *starlark.Thread, msg string) {
		data, err := getRunData(thread)

		eventName := "<unknown>"
		if err == nil {
			eventName = data.eventName
		}

		level := PrintLevel
		currentFrame := thread.CallFrame(0)
		if currentFrame.Name == "debug" {
			level = DebugLevel
		}

		line := int32(0)
		path := "<unknown>"
		if thread.CallStackDepth() > 1 {
			callerFrame := thread.CallFrame(1)
			path = data.GetPath(callerFrame.Pos.Filename())
			line = callerFrame.Pos.Line
		}

		logger.Log(thread.Context(), LogEntry{
			Message:   msg,
			Level:     level,
			EventName: eventName,
			Path:      path,
			Line:      line,
		})
	}
}

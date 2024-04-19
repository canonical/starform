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
	options          *ScriptSetOptions
	pathByProgramKey map[[sha512.Size384]byte]string
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

func (ss *ScriptSet) LoadSources(ctx context.Context, sources []ScriptSource) error {
	scriptStates, err := ss.compilePrograms(ctx, sources)
	if err != nil {
		return err
	}
	scripts := make([]*scriptState, len(scriptStates))
	for i := range scriptStates {
		scripts[i] = &scriptStates[i]
	}

	pathByProgramKey := make(map[[sha512.Size384]byte]string, len(scripts))
	scriptsByPath := make(map[string]*scriptState, len(scripts))
	for _, script := range scripts {
		scriptsByPath[script.path] = script
		pathByProgramKey[script.programKey] = script.path
	}
	sort.Slice(scripts, func(i, j int) bool {
		return scripts[i].path < scripts[j].path
	})

	data := &runData{eventName: LoadEventName, pathByProgramKey: pathByProgramKey}
	thread := makeThread(ss.options, data)
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
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			thread.Cancel("operation cancelled")
		case <-done:
		}
	}()
	for _, script := range scripts {
		if err := script.runTopLevel(thread, ss.options.AppObject); err != nil {
			return err
		}
	}

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

	ss.pathByProgramKey = pathByProgramKey
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
			_, program, err = starlark.SourceProgramOptions(&starlarkDialect, string(programKey[:]), content, isPredeclared)
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

func makeThread(options *ScriptSetOptions, data *runData) *starlark.Thread {
	thread := &starlark.Thread{}
	thread.Print = makePrintFunction(options.Logger)
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
		currentFrame := thread.CallFrame(0)
		callerFrame := thread.CallFrame(1)
		level := PrintLevel
		if currentFrame.Name == "debug" {
			level = DebugLevel
		}
		eventName := "<unknown>"
		data, err := getRunData(thread)
		if err == nil {
			eventName = data.eventName
		}
		path := data.GetPath(callerFrame.Pos.Filename())
		logger.Log(thread.Context(), LogEntry{
			Message:   msg,
			Level:     level,
			EventName: eventName,
			Path:      path,
			Line:      callerFrame.Pos.Line,
		})
	}
}

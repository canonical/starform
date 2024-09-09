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
	appValue       *appValue
	eventObservers map[string][]starlark.Callable
}

type ScriptSource interface {
	Path() string
	Content(ctx context.Context) ([]byte, error)
}

type ScriptSetOptions struct {
	App                 *AppObject
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

var validIdentifier = regexp.MustCompile(`[a-z]\w*`)

func NewScriptSet(options *ScriptSetOptions) (*ScriptSet, error) {
	if options.App == nil {
		return nil, fmt.Errorf("cannot create script set: app not supplied")
	}
	if options.App.Name == "" {
		return nil, fmt.Errorf("cannot create script set: app name missing")
	}
	if !validIdentifier.Match([]byte(options.App.Name)) {
		return nil, fmt.Errorf("cannot create script set: app name invalid")
	}
	if len(options.App.Name) < 3 {
		return nil, fmt.Errorf("cannot create script set: app name too short")
	}
	if len(options.App.Name) > 15 {
		return nil, fmt.Errorf("cannot create script set: app name too long")
	}
	if options.RequiredSafety.Contains(starlark.MemSafe) && options.MaxAllocs == 0 {
		return nil, fmt.Errorf("cannot create script set: MemSafe requested but no MaxAllocs set")
	}
	if options.RequiredSafety.Contains(starlark.CPUSafe) && options.MaxSteps == 0 {
		return nil, fmt.Errorf("cannot create script set: CPUSafe requested but no MaxSteps set")
	}

	appValue := options.App.value()
	appValue.Freeze()

	return &ScriptSet{
		options:  options,
		appValue: appValue,
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

	event := &EventObject{
		Name:  loadEventName,
		State: nil,
	}

	thread := makeThread(ss.options, event)
	defer thread.Cancel("done")
	stop := afterFunc(ctx, func() { thread.Cancel("operation cancelled") })
	defer stop()
	loadStack := []string{}
	thread.Load = func(thread *starlark.Thread, loadPath string) (starlark.StringDict, error) {
		if err := checkLoadPath(loadPath); err != nil {
			return nil, err
		}

		absLoadPath := loadPath
		if strings.HasPrefix(loadPath, "./") || strings.HasPrefix(loadPath, "../") {
			currPath := loadStack[len(loadStack)-1]
			dir := path.Dir(currPath)
			sb := &strings.Builder{}
			sb.Grow(len(dir) + 1 + len(loadPath))
			sb.WriteString(dir)
			sb.WriteRune('/')
			sb.WriteString(loadPath)
			absLoadPath = path.Clean(sb.String())
		}
		normalisedLoadPath := path.Clean(absLoadPath)

		script, ok := scriptsByPath[normalisedLoadPath]
		if !ok {
			return nil, fmt.Errorf("%s not found", loadPath)
		}

		loadStack = append(loadStack, normalisedLoadPath)
		defer func() { loadStack = loadStack[:len(loadStack)-1] }()

		if err := ss.runTopLevel(thread, script); err != nil {
			return nil, err
		}
		return script.toplevelEnv, nil
	}
	for _, script := range scripts {
		loadStack = append(loadStack[:0], script.path)
		if err := ss.runTopLevel(thread, script); err != nil {
			return err
		}
	}

	state := &initState{
		eventObservers: make(map[string][]starlark.Callable),
	}
	event.State = state
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

	for _, observers := range state.eventObservers {
		for _, observer := range observers {
			observer.Freeze()
		}
	}
	ss.eventObservers = state.eventObservers

	return nil
}

func (ss *ScriptSet) compilePrograms(ctx context.Context, sources []ScriptSource) ([]scriptState, error) {
	scriptStates := make([]scriptState, 0, len(sources))
	cache := ss.options.Cache
	if cache == nil {
		cache = &noopScriptCache{}
	}
	isPredeclared := func(name string) bool {
		return name == ss.options.App.Name ||
			name == "debug" ||
			name == "print"
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
			cachedFilename := fmt.Sprintf("cached-%x.star", programKey[:4])
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
						Message:   fmt.Sprintf("cannot put %s into cache: %v", path, err),
						Level:     DebugLevel,
						EventName: loadEventName,
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

func (ss *ScriptSet) runTopLevel(thread *starlark.Thread, script *scriptState) error {
	switch script.status {
	case scriptInitialising:
		return fmt.Errorf("load cycle detected")
	case scriptUninitialised:
		script.status = scriptInitialising
		logger := &scriptLogger{
			logger: ss.options.Logger,
			path:   script.path,
		}
		predeclared := starlark.StringDict{
			ss.options.App.Name: ss.appValue,
			"print":             printBuiltin.BindReceiver(logger),
			"debug":             debugBuiltin.BindReceiver(logger),
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

func (ss *ScriptSet) Handle(ctx context.Context, event *EventObject) error {
	observers, ok := ss.eventObservers[event.Name]
	if !ok {
		return nil
	}

	thread := makeThread(ss.options, event)
	stop := afterFunc(ctx, func() { thread.Cancel("operation cancelled") })
	defer stop()
	defer thread.Cancel("done")
	for _, observer := range observers {
		_, err := starlark.Call(thread, observer, starlark.Tuple{event}, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func makeThread(options *ScriptSetOptions, data *EventObject) *starlark.Thread {
	thread := &starlark.Thread{}
	thread.Print = func(thread *starlark.Thread, msg string) {}
	thread.RequireSafety(options.RequiredSafety)
	thread.SetMaxSteps(options.MaxSteps)
	thread.SetMaxAllocs(options.MaxAllocs)
	thread.SetLocal(eventObjectLocalKey, data)
	return thread
}

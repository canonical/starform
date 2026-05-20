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

	"github.com/canonical/starform/internal/lib"
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/syntax"
)

type ScriptSet struct {
	options            *ScriptSetOptions
	appValue           *appValue
	modules            map[string]Module
	predeclared        starlark.StringDict
	eventObservers     map[string][]starlark.Callable
	observedEventNames []string
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
	MaxAllocs, MaxSteps int64
	Modules             []Module
}

var starlarkDialect = syntax.FileOptions{
	Set:             true,
	While:           false,
	TopLevelControl: false,
	GlobalReassign:  false,
	Recursion:       true,
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

var validIdentifier = regexp.MustCompile(`^[a-z][a-z0-9_]*[a-z0-9]$`)
var validModuleName = regexp.MustCompile(`^[a-z]\w*(\/[a-z]\w*)*$`)

func NewScriptSet(options *ScriptSetOptions) (*ScriptSet, error) {
	if options.App == nil {
		return nil, fmt.Errorf("cannot create script set: app not supplied")
	}
	if options.App.Name == "" {
		return nil, fmt.Errorf("cannot create script set: app name missing")
	}
	if len(options.App.Name) < 3 {
		return nil, fmt.Errorf("cannot create script set: app name too short")
	}
	if len(options.App.Name) > 15 {
		return nil, fmt.Errorf("cannot create script set: app name too long")
	}
	if strings.Contains(options.App.Name, "__") {
		return nil, fmt.Errorf("cannot create script set: app name contains consecutive underscores")
	}
	if !validIdentifier.Match([]byte(options.App.Name)) {
		return nil, fmt.Errorf("cannot create script set: app name invalid")
	}
	if options.RequiredSafety.Contains(starlark.MemSafe) && options.MaxAllocs == 0 {
		return nil, fmt.Errorf("cannot create script set: MemSafe requested but no MaxAllocs set")
	}
	if options.RequiredSafety.Contains(starlark.CPUSafe) && options.MaxSteps == 0 {
		return nil, fmt.Errorf("cannot create script set: CPUSafe requested but no MaxSteps set")
	}

	appNs := options.App.Name + "/"
	modules := make(map[string]Module)
	predeclared := make(starlark.StringDict)
	for _, module := range options.Modules {
		systemModule, isSystemModule := module.(*lib.SystemModule)
		if isSystemModule && systemModule.Predeclared {
			for name, value := range module.Members() {
				predeclared[name] = value
			}
			continue
		}

		if !isSystemModule {
			name := module.Name()
			if !strings.HasPrefix(name, appNs) {
				return nil, fmt.Errorf("cannot use %q as module name: missing '%s' prefix", name, appNs)
			}
			if path.Ext(name) != "" {
				return nil, fmt.Errorf("cannot use %q as module name: file extension present", name)
			}
			if !validModuleName.Match([]byte(name)) {
				return nil, fmt.Errorf("cannot use %q as module name: invalid", name)
			}
		}
		modules[module.Name()] = module
	}

	appValue := options.App.value()
	appValue.Freeze()

	return &ScriptSet{
		options:     options,
		appValue:    appValue,
		modules:     modules,
		predeclared: predeclared,
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

	thread := starlark.ContextThread(ctx)
	if thread == nil {
		thread = makeThread(ctx, ss.options, event)
		defer thread.Cancel("done")
	} else {
		if err := checkThread(ss.options, thread); err != nil {
			return err
		}
		prevEvent := setEventObject(thread, event)
		defer setEventObject(thread, prevEvent)
	}

	loadDir := "." // Directory where the load is being made from.
	loadDirStack := []string{}
	pushd := func(dir string) {
		loadDir = dir
		loadDirStack = append(loadDirStack, dir)
	}
	popd := func() {
		if len(loadDirStack) < 2 {
			loadDirStack = loadDirStack[:0]
			loadDir = "."
			return
		}
		loadDir = loadDirStack[len(loadDirStack)-2]
		loadDirStack = loadDirStack[:len(loadDirStack)-1]
	}

	thread.Load = func(thread *starlark.Thread, loadPath string) (starlark.StringDict, error) {
		if module, ok := ss.modules[loadPath]; ok {
			systemModule, isSystemModule := module.(*lib.SystemModule)
			if !isSystemModule || !systemModule.Predeclared {
				return module.Members(), nil
			}
		}

		sanitisedLoadPath, err := sanitiseLoadPath(loadDir, loadPath)
		if err != nil {
			return nil, err
		}

		script, ok := scriptsByPath[sanitisedLoadPath]
		if !ok {
			return nil, errors.New("file not found")
		}

		pushd(path.Dir(sanitisedLoadPath))
		defer popd()
		if err := ss.runTopLevel(thread, script); err != nil {
			return nil, err
		}
		return script.toplevelEnv, nil
	}
	for _, script := range scripts {
		pushd(path.Dir(script.path))
		if err := ss.runTopLevel(thread, script); err != nil {
			return err
		}
		popd()
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

	observedEventNames := make([]string, 0, len(state.eventObservers))
	for name, observers := range state.eventObservers {
		observedEventNames = append(observedEventNames, name)

		for _, observer := range observers {
			observer.Freeze()
		}
	}
	sort.Strings(observedEventNames)
	ss.observedEventNames = observedEventNames
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
		if name == ss.options.App.Name ||
			name == "debug" ||
			name == "print" {
			return true
		}
		return ss.predeclared.Has(name)
	}
	for _, source := range sources {
		path := source.Path()
		if !strings.HasSuffix(path, ".star") {
			continue
		}
		if _, err := sanitiseLoadPath(".", path); err != nil {
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
		for name, value := range ss.predeclared {
			predeclared[name] = value
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

func sanitiseLoadPath(loadDir, loadPath string) (string, error) {
	if len(loadPath) == 0 {
		return "", miscInvalidPathError // Special case to simplify valid path regex.
	}
	if strings.ContainsRune(loadPath, '-') {
		return "", errors.New(`path contains "-", use "_" instead`)
	}
	if strings.ContainsRune(loadPath, '\\') {
		return "", errors.New(`path contains "\", use "/" instead`)
	}
	if strings.Contains(loadPath, "__") {
		return "", miscInvalidPathError // Special case to simplify valid path regex.
	}
	if strings.HasPrefix(loadPath, "./../") {
		return "", errors.New("path contains redundant components")
	}

	toCheck := loadPath
	if strings.HasPrefix(loadPath, "./") {
		toCheck = loadPath[2:] // Ignore leading, non-redundant "./".
	}
	if cleaned := path.Clean(loadPath); toCheck != cleaned {
		return "", errors.New("path contains redundant components")
	}

	if !validCleanPath.MatchString(loadPath) {
		return "", miscInvalidPathError
	}

	sanitisedPath := loadPath
	if strings.HasPrefix(loadPath, "./") || strings.HasPrefix(loadPath, "../") {
		sanitisedPath = path.Clean(path.Join(loadDir, loadPath))
		if strings.HasPrefix(sanitisedPath, "../") {
			return "", errors.New("file not found")
		}
	}
	return sanitisedPath, nil
}

// ObservedEventNames returns a sorted slice of the names of the
// events for which the script set has observers.
func (ss *ScriptSet) ObservedEventNames() []string {
	return ss.observedEventNames
}

var validEventName = regexp.MustCompile(`^[a-z][a-z0-9_]*[a-z0-9]$`)

func (ss *ScriptSet) Handle(ctx context.Context, event *EventObject) error {
	if len(event.Name) < 3 {
		return fmt.Errorf("cannot handle %q event: name too short", event.Name)
	}
	if len(event.Name) > 20 {
		return fmt.Errorf("cannot handle %q event: name too long", event.Name)
	}
	if strings.Contains(event.Name, "__") {
		return fmt.Errorf("cannot handle %q event: name contains consecutive underscores", event.Name)
	}
	if !validEventName.Match([]byte(event.Name)) {
		return fmt.Errorf("cannot handle %q event: name invalid", event.Name)
	}
	observers, ok := ss.eventObservers[event.Name]
	if !ok {
		return nil
	}

	thread := starlark.ContextThread(ctx)
	if thread == nil {
		thread = makeThread(ctx, ss.options, event)
		defer thread.Cancel("done")
	} else {
		if err := checkThread(ss.options, thread); err != nil {
			return err
		}
		prevEvent := setEventObject(thread, event)
		defer setEventObject(thread, prevEvent)
	}

	for _, observer := range observers {
		_, err := starlark.Call(thread, observer, starlark.Tuple{event}, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func makeThread(ctx context.Context, options *ScriptSetOptions, event *EventObject) *starlark.Thread {
	thread := &starlark.Thread{
		Print: func(thread *starlark.Thread, msg string) {},
	}
	thread.SetParentContext(ctx)
	thread.RequireSafety(options.RequiredSafety)
	thread.SetMaxSteps(options.MaxSteps)
	thread.SetMaxAllocs(options.MaxAllocs)
	thread.SetLocal(eventObjectLocalKey, &eventObjectStorage{
		Event: event,
	})
	return thread
}

func checkThread(options *ScriptSetOptions, thread *starlark.Thread) error {
	_, ok := thread.Local(eventObjectLocalKey).(*eventObjectStorage)
	if !ok {
		return fmt.Errorf("internal error: thread is not a starform thread")
	}
	return thread.CheckPermits(options.RequiredSafety)
}

func setEventObject(thread *starlark.Thread, event *EventObject) (previous *EventObject) {
	storage := thread.Local(eventObjectLocalKey).(*eventObjectStorage)
	oldEvent := storage.Event
	storage.Event = event
	return oldEvent
}

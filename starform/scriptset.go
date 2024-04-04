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

type scriptStatus int

const (
	scriptUninitialised scriptStatus = iota
	scriptInitialising
	scriptInitialised
)

type scriptState struct {
	path        string
	program     *starlark.Program
	status      scriptStatus
	toplevelEnv starlark.StringDict
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
	scripts, err := ss.compilePrograms(ctx, sources)
	if err != nil {
		return err
	}
	scriptByPath := make(map[string]*scriptState, len(scripts))
	for _, script := range scripts {
		scriptByPath[script.path] = script
	}
	sort.Slice(scripts, func(i, j int) bool {
		return scripts[i].path < scripts[j].path
	})

	thread := ss.options.makeThread()
	thread.Load = func(thread *starlark.Thread, path string) (starlark.StringDict, error) {
		if err := checkLoadPath(path); err != nil {
			return nil, err
		}

		script, ok := scriptByPath[path]
		if !ok {
			return nil, fmt.Errorf("%s not found", path)
		}
		if err := script.runTopLevel(thread); err != nil {
			return nil, err
		}
		return script.toplevelEnv, nil
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			thread.Cancel("operation canceled")
		case <-done:
		}
	}()
	for _, script := range scripts {
		if err := script.runTopLevel(thread); err != nil {
			return err
		}
	}

	for _, script := range scripts {
		init, ok := script.toplevelEnv["init"]
		if !ok {
			continue
		}
		if _, ok := init.(*starlark.Function); !ok {
			return fmt.Errorf("init must be a Starlark function, got %s", init.Type())
		}

		if _, err := starlark.Call(thread, init, nil, nil); err != nil {
			return fmt.Errorf("cannot load script: %w", err)
		}
	}

	return nil
}

func (ss *ScriptSet) compilePrograms(ctx context.Context, sources []ScriptSource) ([]*scriptState, error) {
	scriptStateStorage := make([]scriptState, 0, len(sources))
	scriptStates := make([]*scriptState, 0, len(sources))
	isPredeclared := func(string) bool { return false }
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
		_, program, err := starlark.SourceProgramOptions(&starlarkDialect, path, content, isPredeclared)
		if err != nil {
			return nil, fmt.Errorf("cannot load %s: %w", path, err)
		}
		scriptStateStorage = append(scriptStateStorage, scriptState{
			path:    path,
			program: program,
		})
		scriptStates = append(scriptStates, &scriptStateStorage[len(scriptStateStorage)-1])
	}
	return scriptStates, nil
}

func (script *scriptState) runTopLevel(thread *starlark.Thread) error {
	switch script.status {
	case scriptInitialising:
		return fmt.Errorf("load cycle detected")
	case scriptUninitialised:
		script.status = scriptInitialising
		toplevelEnv, err := script.program.Init(thread, nil)
		if err != nil {
			return err
		}
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

func (options *ScriptSetOptions) makeThread() *starlark.Thread {
	thread := &starlark.Thread{}
	thread.Print = options.PrintHandler
	thread.RequireSafety(options.RequiredSafety)
	thread.SetMaxSteps(options.MaxSteps)
	thread.SetMaxAllocs(options.MaxAllocs)
	return thread
}

package formtest

import (
	"context"
	"strings"
	"testing/fstest"

	"github.com/canonical/starform/internal/lib"
	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/starlarkstruct"
	"github.com/canonical/starlark/starlarktest"
	"github.com/canonical/starlark/startest"
)

// eventObjectLocalKey must have the same value as starform.eventObjectLocalKey
const eventObjectLocalKey = "starform-event-object"

// eventObjectStorage is used as an indirection to store the event
// object in the thread locals, so that the event can be modified.
// This type must remain an unnamed type and must be kept in
// sync with formtest.eventObjectStorage.
type eventObjectStorage = struct {
	Event  *starform.EventObject
	Thread *starlark.Thread
}

var assertModule starform.Module

func init() {
	assertMembers, err := starlarktest.LoadAssertModule()
	if err != nil {
		panic(err)
	}
	assertModule = &lib.SystemModule{
		Module: &starlarkstruct.Module{
			Name:    "assert",
			Members: assertMembers,
		},
		Predeclared: true,
	}
}

type TestBase startest.TestBase

type FT struct {
	*startest.ST

	parentCtx context.Context
	event     *starform.EventObject
	app       *starform.AppObject
	cache     starform.ScriptCache
	logger    starform.Logger
	predecl   lib.SystemModule
}

func From(base TestBase) *FT {
	st, ok := base.(*startest.ST)
	if !ok {
		st = startest.From(base)
	}

	ft := &FT{
		ST:        st,
		parentCtx: context.Background(),
		predecl: lib.SystemModule{
			Module: &starlarkstruct.Module{
				Name:    "ft",
				Members: starlark.StringDict{},
			},
			Predeclared: true,
		},
	}
	ft.predecl.Module.Members["ft"] = ft
	return ft
}

func (ft *FT) SetParentContext(ctx context.Context) { ft.parentCtx = ctx }
func (ft *FT) SetEvent(event *starform.EventObject) { ft.event = event }
func (ft *FT) SetApp(app *starform.AppObject)       { ft.app = app }
func (ft *FT) SetCache(cache starform.ScriptCache)  { ft.cache = cache }
func (ft *FT) SetLogger(logger starform.Logger)     { ft.logger = logger }

const ftSafe = starlark.MemSafe | starlark.CPUSafe | starlark.TimeSafe | starlark.IOSafe

func (ft *FT) RunString(code string) (ok bool) {
	if ft.app == nil {
		ft.Error("cannot run formtest without app")
		return
	}
	if ft.event == nil {
		ft.Error("cannot run formtest without event")
		return
	}

	modules := []starform.Module{
		assertModule,
		&ft.predecl,
	}

	if code = strings.TrimRight(code, " \t\r\n"); code == "" {
		return true
	}
	code, err := startest.Reindent(code)
	if err != nil {
		ft.Error(err)
		return false
	}

	set, err := starform.NewScriptSet(&starform.ScriptSetOptions{
		App:     ft.app,
		Cache:   ft.cache,
		Logger:  ft.logger,
		Modules: modules,
	})
	if err != nil {
		ft.Error(err)
		return
	}

	ft.AddLocal("Reporter", ft) // Set starlarktest reporter outside of RunThread.
	ft.RunThread(func(thread *starlark.Thread) {
		sources, err := starform.LoadDirSources(thread.Context(), &starform.LoadDirSourcesOptions{
			Fs: fstest.MapFS{
				"test.star": &fstest.MapFile{
					Data: []byte(code),
				},
			},
		})
		if err != nil {
			ft.Error(err)
			return
		}

		if err := set.LoadSources(thread.Context(), sources); err != nil {
			ft.Error(err)
			return
		}

		if err := set.Handle(thread.Context(), ft.event); err != nil {
			ft.Error(err)
			return
		}
	})

	return ft.Failed()
}

func (ft *FT) AddValue(name string, value starlark.Value) {
	if value == nil {
		ft.Errorf("AddValue expected a value: got %T", value)
		return
	}
	if _, ok := ft.predecl.Module.Members[name]; ok {
		ft.Errorf("AddValue: %q already defined", name)
	}
	ft.predecl.Module.Members[name] = value
}

func (ft *FT) AddBuiltin(name string, fn starlark.Value) {
	builtin, ok := fn.(*starlark.Builtin)
	if !ok {
		ft.Errorf("AddBuiltin expected a builtin: got %T", fn)
		return
	}
	ft.AddValue(builtin.Name(), builtin)
}

func (ft *FT) RunThread(fn func(thread *starlark.Thread)) {
	if ft.event == nil {
		ft.Error("cannot run formtest without event")
		return
	}

	storage := &eventObjectStorage{
		Event: ft.event,
	}
	ft.ST.AddLocal(eventObjectLocalKey, storage)
	ft.ST.RunThread(func(thread *starlark.Thread) {
		storage.Thread = thread
		fn(thread)
	})
}

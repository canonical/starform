package formtest

import (
	"context"
	"strings"
	"testing/fstest"

	"github.com/canonical/starform/internal"
	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/starlarkstruct"
	"github.com/canonical/starlark/starlarktest"
	"github.com/canonical/starlark/startest"
)

var assertModule starform.Module

func init() {
	assertMembers, err := starlarktest.LoadAssertModule()
	if err != nil {
		panic(err)
	}
	assertModule = &internal.SystemModule{
		Module: &starlarkstruct.Module{
			Name:    "assert",
			Members: assertMembers,
		},
		Predeclared: true,
	}
}

type TestBase interface {
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Failed() bool
	Log(args ...interface{})
	Logf(fmt string, args ...interface{})
}

type FT struct {
	st *startest.ST

	parentCtx context.Context
	event     *starform.EventObject
	app       *starform.AppObject
	cache     starform.ScriptCache
	logger    starform.Logger
}

func From(base TestBase) *FT {
	st, ok := base.(*startest.ST)
	if !ok {
		st = startest.From(base)
	}

	return &FT{
		st:        st,
		parentCtx: context.Background(),
	}
}

func (ft *FT) Error(args ...interface{})                 { ft.st.Error(args...) }
func (ft *FT) Errorf(format string, args ...interface{}) { ft.st.Errorf(format, args...) }
func (ft *FT) Failed() bool                              { return ft.st.Failed() }
func (ft *FT) Fatal(args ...interface{})                 { ft.st.Fatal(args...) }
func (ft *FT) Fatalf(format string, args ...interface{}) { ft.st.Fatalf(format, args...) }
func (ft *FT) Log(args ...interface{})                   { ft.st.Log(args...) }
func (ft *FT) Logf(fmt string, args ...interface{})      { ft.st.Logf(fmt, args...) }

func (ft *FT) SetParentContext(ctx context.Context) { ft.parentCtx = ctx }
func (ft *FT) SetEvent(event *starform.EventObject) { ft.event = event }
func (ft *FT) SetApp(app *starform.AppObject)       { ft.app = app }
func (ft *FT) SetCache(cache starform.ScriptCache)  { ft.cache = cache }
func (ft *FT) SetLogger(logger starform.Logger)     { ft.logger = logger }

func (ft *FT) RequireSafety(requiredSafety starlark.SafetyFlags) { ft.st.RequireSafety(requiredSafety) }
func (ft *FT) SetMaxAllocs(maxAllocs int64)                      { ft.st.SetMaxAllocs(maxAllocs) }
func (ft *FT) SetMaxSteps(maxSteps int64)                        { ft.st.SetMaxSteps(maxSteps) }

func (ft *FT) MakeAppValue() starlark.Value { return internal.NewAppValue(ft.app) }
func (ft *FT) ST() *startest.ST             { return ft.st }

var ftSafe = starlark.MemSafe | starlark.CPUSafe | starlark.TimeSafe | starlark.IOSafe

func (ft *FT) RunString(code string) (ok bool) {
	modules := []starform.Module{
		assertModule,
		&internal.SystemModule{
			Module: &starlarkstruct.Module{
				Name: "st",
				Members: starlark.StringDict{
					"st": ft.st,
				},
			},
			Predeclared: true,
		},
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

	ft.st.AddLocal("Reporter", ft.st) // Set starlarktest reporter outside of RunThread.
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
			ft.st.Error(err)
			return
		}

		if err := set.Handle(thread.Context(), ft.event); err != nil {
			ft.st.Error(err)
			return
		}
	})

	return ft.st.Failed()
}

func (ft *FT) RunThread(fn func(thread *starlark.Thread)) {
	if ft.app == nil {
		ft.Error("cannot run formtest without app")
		return
	}
	if ft.event == nil {
		ft.Error("cannot run formtest without event")
		return
	}

	rundata := &internal.Rundata{}
	ft.st.SetParentContext(context.WithValue(ft.parentCtx, internal.RunDataLocalKey, rundata))
	ft.st.RunThread(func(thread *starlark.Thread) {
		rundata.Thread = thread
		rundata.Event = ft.event

		fn(thread)
	})
}

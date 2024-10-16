package formtest

import (
	"context"
	"math"
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
	base TestBase

	parentCtx context.Context
	event     *starform.EventObject
	app       *starform.AppObject
	cache     starform.ScriptCache
	logger    starform.Logger

	requiredSafety      starlark.SafetyFlags
	safetyGiven         bool
	maxAllocs, maxSteps int64
}

func From(t TestBase) *FT {
	return &FT{
		base:      t,
		parentCtx: context.Background(),
		maxAllocs: math.MaxInt64,
		maxSteps:  math.MaxInt64,
	}
}

func (ft *FT) Error(args ...interface{})                 { ft.base.Error(args...) }
func (ft *FT) Errorf(format string, args ...interface{}) { ft.base.Errorf(format, args...) }
func (ft *FT) Failed() bool                              { return ft.base.Failed() }
func (ft *FT) Fatal(args ...interface{})                 { ft.base.Fatal(args...) }
func (ft *FT) Fatalf(format string, args ...interface{}) { ft.base.Fatalf(format, args...) }
func (ft *FT) Log(args ...interface{})                   { ft.base.Log(args...) }
func (ft *FT) Logf(fmt string, args ...interface{})      { ft.base.Logf(fmt, args...) }

func (ft *FT) RequireSafety(requiredSafety starlark.SafetyFlags) {
	ft.requiredSafety |= requiredSafety
	ft.safetyGiven = true
}

func (ft *FT) SetParentContext(ctx context.Context) { ft.parentCtx = ctx }
func (ft *FT) SetEvent(event *starform.EventObject) { ft.event = event }
func (ft *FT) SetApp(app *starform.AppObject)       { ft.app = app }
func (ft *FT) SetCache(cache starform.ScriptCache)  { ft.cache = cache }
func (ft *FT) SetLogger(logger starform.Logger)     { ft.logger = logger }

func (ft *FT) SetMaxAllocs(maxAllocs int64) { ft.maxAllocs = maxAllocs }
func (ft *FT) SetMaxSteps(maxSteps int64)   { ft.maxSteps = maxSteps }

var ftSafe = starlark.MemSafe | starlark.CPUSafe | starlark.TimeSafe | starlark.IOSafe

func (ft *FT) RunString(code string) (ok bool) {
	if ft.app == nil {
		ft.Error("cannot run formtest without app")
		return false
	}
	if ft.event == nil {
		ft.Error("cannot run formtest without event")
		return false
	}
	if !ft.safetyGiven {
		ft.requiredSafety = ftSafe
	}
	// TODO: Should we validate the app and event names here?

	st, ok := ft.base.(*startest.ST)
	if !ok {
		st = startest.From(ft.base)
	}
	st.RequireSafety(ft.requiredSafety)
	st.SetMaxAllocs(ft.maxAllocs)
	st.SetMaxSteps(ft.maxSteps)

	if code = strings.TrimRight(code, " \t\r\n"); code == "" {
		return true
	}
	code, err := startest.Reindent(code)
	if err != nil {
		ft.Error(err)
		return false
	}

	modules := []starform.Module{
		assertModule,
		&internal.SystemModule{
			Module: &starlarkstruct.Module{
				Name: "st",
				Members: starlark.StringDict{
					"st": st,
				},
			},
			Predeclared: true,
		},
	}
	st.AddLocal("Reporter", st) // Set starlarktest reporter outside of RunThread.
	rundata := &internal.Rundata{}
	st.SetParentContext(context.WithValue(ft.parentCtx, internal.RunDataLocalKey, rundata))
	st.RunThread(func(thread *starlark.Thread) {
		rundata.Thread = thread

		set, err := starform.NewScriptSet(&starform.ScriptSetOptions{
			App:            ft.app,
			Cache:          ft.cache,
			Logger:         ft.logger,
			RequiredSafety: ft.requiredSafety,
			MaxAllocs:      ft.maxAllocs,
			MaxSteps:       ft.maxSteps,
			Modules:        modules,
		})
		if err != nil {
			ft.Error(err)
			return
		}

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
			st.Error(err)
			return
		}

		if err := set.Handle(thread.Context(), ft.event); err != nil {
			st.Error(err)
			return
		}
	})

	return st.Failed()
}

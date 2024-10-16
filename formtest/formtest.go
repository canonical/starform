package formtest

import (
	"context"
	"math"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/startest"
)

type FT struct {
	t *testing.T

	parentCtx context.Context
	event     *starform.EventObject
	app       *starform.AppObject
	cache     starform.ScriptCache
	logger    starform.Logger

	requiredSafety      starlark.SafetyFlags
	safetyGiven         bool
	maxAllocs, maxSteps int64
}

func From(t *testing.T) *FT {
	return &FT{
		t:         t,
		parentCtx: context.Background(),
		maxAllocs: math.MaxInt64,
		maxSteps:  math.MaxInt64,
	}
}

func (ft *FT) Error(args ...interface{})                 { ft.t.Error(args...) }
func (ft *FT) Errorf(format string, args ...interface{}) { ft.t.Errorf(format, args...) }
func (ft *FT) Failed() bool                              { return ft.t.Failed() }
func (ft *FT) Fatal(args ...interface{})                 { ft.t.Fatal(args...) }
func (ft *FT) Fatalf(format string, args ...interface{}) { ft.t.Fatalf(format, args...) }
func (ft *FT) Log(args ...interface{})                   { ft.t.Log(args...) }
func (ft *FT) Logf(fmt string, args ...interface{})      { ft.t.Logf(fmt, args...) }

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

	if code = strings.TrimRight(code, " \t\r\n"); code == "" {
		return true
	}
	code, err := startest.Reindent(code)
	if err != nil {
		ft.Error(err)
		return false
	}

	set, err := starform.NewScriptSet(&starform.ScriptSetOptions{
		App:            ft.app,
		Cache:          ft.cache,
		Logger:         ft.logger,
		RequiredSafety: ft.requiredSafety,
		MaxAllocs:      ft.maxAllocs,
		MaxSteps:       ft.maxSteps,
	})
	if err != nil {
		ft.Error(err)
		return false
	}

	sources, err := starform.LoadDirSources(ft.parentCtx, &starform.LoadDirSourcesOptions{
		Fs: fstest.MapFS{
			"test.star": &fstest.MapFile{
				Data: []byte(code),
			},
		},
	})
	if err != nil {
		ft.Error(err)
		return false
	}

	err = set.LoadSources(ft.parentCtx, sources)
	if err != nil {
		ft.Error(err)
		return false
	}

	err = set.Handle(ft.parentCtx, ft.event)
	if err != nil {
		ft.Error(err)
		return false
	}

	return true
}

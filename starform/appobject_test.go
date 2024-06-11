package starform_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/startest"
)

func isStarlarkCancellation(err error) bool {
	return strings.Contains(err.Error(), "Starlark computation cancelled:")
}

func TestAppObjectAsStarlarkValue(t *testing.T) {
	const appName = "testApp"

	app := starform.NewAppValue(&starform.App{
		Name: appName,
	})

	if !bool(app.Truth()) {
		t.Errorf("app object should be truthy")
	}
	if appString := app.String(); appString != fmt.Sprintf("<App %s>", appName) {
		t.Errorf("incorrect string representation: expected %q but got %q", appName, appString)
	}
	if appType := app.Type(); appType != "App" {
		t.Errorf("incorrect type representation: expected %q but got %q", appName, "App")
	}
	if _, err := app.Hash(); err == nil {
		t.Errorf("app object should not be hashable")
	}
}

func TestAppObjectSafeString(t *testing.T) {
	const appName = "testApp"

	t.Run("nil-thread", func(t *testing.T) {
		defer func() {
			if err := recover(); err != nil {
				t.Errorf("unexpected panic: %s", err)
			}
		}()

		app := starform.NewAppValue(&starform.App{
			Name: appName,
		})
		sb := &strings.Builder{}
		app.(starlark.SafeStringer).SafeString(nil, sb)
		if str := sb.String(); str != fmt.Sprintf("<App %s>", appName) {
			t.Error("invalid SafeString value")
		}
	})

	t.Run("regular-operation", func(t *testing.T) {
		st := startest.From(t)
		st.RequireSafety(starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe)
		st.SetMaxSteps(uint64(len(fmt.Sprintf("<%s object>", appName))))
		st.RunThread(func(thread *starlark.Thread) {
			app := starform.NewAppValue(&starform.App{
				Name: appName,
			})
			for i := 0; i < st.N; i++ {
				sb := starlark.NewSafeStringBuilder(thread)
				if err := app.(starlark.SafeStringer).SafeString(thread, sb); err != nil {
					st.Error(err)
				}
				if err := sb.Err(); err != nil {
					st.Error(err)
				}
				if err := thread.AddAllocs(starlark.StringTypeOverhead); err != nil {
					st.Error(err)
				}
				st.KeepAlive(sb.String())
			}
		})
	})

	t.Run("cancellation", func(t *testing.T) {
		st := startest.From(t)
		st.RequireSafety(starlark.TimeSafe)
		st.SetMaxSteps(0)
		st.RunThread(func(thread *starlark.Thread) {
			thread.Cancel("done")
			app := starform.NewAppValue(&starform.App{
				Name: appName,
			})
			for i := 0; i < st.N; i++ {
				sb := starlark.NewSafeStringBuilder(thread)
				if err := app.(starlark.SafeStringer).SafeString(thread, sb); err == nil {
					st.Error("expected cancellation")
				} else if !isStarlarkCancellation(err) {
					st.Errorf("expected cancellation, got %v", err)
				}
			}
		})
	})
}

func TestAppObjectSafeAttr(t *testing.T) {
	app := starform.NewAppValue(&starform.App{
		Name: "test",
	})

	t.Run("allocs-steps-io", func(t *testing.T) {
		st := startest.From(t)
		st.RequireSafety(starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe)
		st.SetMaxSteps(0)
		st.RunThread(func(thread *starlark.Thread) {
			starform.PutRunDataIn(thread, &starform.EventRunData{
				EventName: starform.LoadEventName,
				State:     make(map[string][]starlark.Callable),
			})
			for i := 0; i < st.N; i++ {
				result, err := app.SafeAttr(thread, "observe")
				if err != nil {
					st.Error(err)
				}
				st.KeepAlive(result)
			}
		})
	})

	t.Run("cancellaton", func(t *testing.T) {
		st := startest.From(t)
		st.RequireSafety(starlark.TimeSafe)
		st.SetMaxSteps(0)
		st.RunThread(func(thread *starlark.Thread) {
			thread.Cancel("done")
			starform.PutRunDataIn(thread, &starform.EventRunData{
				EventName: starform.LoadEventName,
				State:     make(map[string][]starlark.Callable),
			})
			for i := 0; i < st.N; i++ {
				_, err := app.SafeAttr(thread, "observe")
				if err != nil && !isStarlarkCancellation(err) {
					st.Error(err)
				}
			}
		})
	})
}

func TestAppObjectObserveSafety(t *testing.T) {
	observer := starlark.NewBuiltin("observer", func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})

	t.Run("return-value", func(t *testing.T) {
		thread := &starlark.Thread{}
		starform.PutRunDataIn(thread, &starform.EventRunData{
			EventName: starform.LoadEventName,
			State:     make(map[string][]starlark.Callable),
		})

		app := starform.NewAppValue(&starform.App{
			Name: "test",
		})
		observe, _ := app.SafeAttr(thread, "observe")
		if observe == nil {
			t.Fatal("no such method: test.observe")
		}

		args := starlark.Tuple{starlark.String("event_name"), observer}
		result, err := starlark.Call(thread, observe, args, nil)
		if err != nil {
			t.Error(err)
		} else if result != starlark.None {
			t.Errorf("expected None return: got %v", result)
		}
	})

	t.Run("allocs-steps-io-safety", func(t *testing.T) {
		st := startest.From(t)
		st.RequireSafety(starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe)
		st.SetMaxSteps(1)
		st.RunThread(func(thread *starlark.Thread) {
			starform.PutRunDataIn(thread, &starform.EventRunData{
				EventName: starform.LoadEventName,
				State:     make(map[string][]starlark.Callable),
			})

			app := starform.NewAppValue(&starform.App{
				Name: "test",
			})
			observe, _ := app.SafeAttr(thread, "observe")
			if observe == nil {
				t.Fatal("no such method: test.observe")
			}

			data := &starform.EventRunData{
				EventName: starform.LoadEventName,
				State:     make(map[string][]starlark.Callable),
			}
			if err := thread.AddAllocs(starlark.EstimateSize(data)); err != nil {
				t.Error(err)
			}
			starform.PutRunDataIn(thread, data)

			args := starlark.Tuple{starlark.String("event_name"), observer}
			for i := 0; i < st.N; i++ {
				_, err := starlark.Call(thread, observe, args, nil)
				if err != nil {
					st.Error(err)
				}
			}
			st.KeepAlive(data)
		})
	})

	t.Run("cancellaton", func(t *testing.T) {
		st := startest.From(t)
		st.RequireSafety(starlark.TimeSafe)
		st.SetMaxSteps(0)
		st.RunThread(func(thread *starlark.Thread) {
			thread.Cancel("done")
			starform.PutRunDataIn(thread, &starform.EventRunData{
				EventName: starform.LoadEventName,
				State:     make(map[string][]starlark.Callable),
			})

			app := starform.NewAppValue(&starform.App{
				Name: "test",
			})
			app.Freeze()
			observe, _ := app.SafeAttr(thread, "observe")
			if observe == nil {
				t.Fatal("no such method: test.observe")
			}

			args := starlark.Tuple{starlark.String("event_name"), observer}
			_, err := starlark.Call(thread, observe, args, nil)
			if err != nil && !isStarlarkCancellation(err) {
				st.Error(err)
			}
		})
	})
}

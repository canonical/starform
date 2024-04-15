package starform_test

import (
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

	app := starform.NewAppObject(appName)
	app.Freeze()

	if !bool(app.Truth()) {
		t.Errorf("app object should be truthy")
	}
	if appString := app.String(); appString != appName {
		t.Errorf("incorrect string representation: expected %q but got %q", appName, appString)
	}
	if appType := app.Type(); appType != appName {
		t.Errorf("incorrect type representation: expected %q but got %q", appName, appType)
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

		app := starform.NewAppObject(appName)
		sb := &strings.Builder{}
		app.SafeString(nil, sb)
		if str := sb.String(); str != appName {
			t.Error("invalid SafeString value")
		}
	})

	t.Run("regular-operation", func(t *testing.T) {
		st := startest.From(t)
		st.RequireSafety(starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe)
		st.SetMaxSteps(uint64(len(appName)))
		st.RunThread(func(thread *starlark.Thread) {
			app := starform.NewAppObject(appName)
			for i := 0; i < st.N; i++ {
				sb := starlark.NewSafeStringBuilder(thread)
				if err := app.SafeString(thread, sb); err != nil {
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
			app := starform.NewAppObject(appName)
			for i := 0; i < st.N; i++ {
				sb := starlark.NewSafeStringBuilder(thread)
				if err := app.SafeString(thread, sb); err == nil {
					st.Error("expected cancellation")
				} else if !isStarlarkCancellation(err) {
					st.Errorf("expected cancellation, got %v", err)
				}
			}
		})
	})
}

func TestAppObjectSafeAttr(t *testing.T) {
	appObject := starform.NewAppObject("test")
	appObject.Freeze()

	t.Run("allocs-steps-io-safety", func(t *testing.T) {
		st := startest.From(t)
		st.RequireSafety(starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe)
		st.SetMaxSteps(0)
		st.RunThread(func(thread *starlark.Thread) {
			for i := 0; i < st.N; i++ {
				result, err := appObject.SafeAttr(thread, "observe")
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
			for i := 0; i < st.N; i++ {
				_, err := appObject.SafeAttr(thread, "observe")
				if err != nil && !isStarlarkCancellation(err) {
					st.Error(err)
				}
			}
		})
	})
}

func TestAppObjectObserveSafety(t *testing.T) {
	t.Run("return-value", func(t *testing.T) {
		appObject := starform.NewAppObject("test")
		appObject.Freeze()
		observe, err := appObject.Attr("observe")
		if err != nil {
			t.Fatal("no such method: test.observe")
		}

		thread := &starlark.Thread{}
		args := starlark.Tuple{starlark.String("asdf"), observe}
		result, err := starlark.Call(thread, observe, args, nil)
		if err != nil {
			t.Error(err)
		}
		if result != starlark.None {
			t.Errorf("expected None return: got %v", result)
		}
	})

	t.Run("allocs-steps-io-safety", func(t *testing.T) {
		st := startest.From(t)
		st.RequireSafety(starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe)
		st.SetMaxSteps(0)
		st.RunThread(func(thread *starlark.Thread) {
			appObject := starform.NewAppObject("test")
			appObject.Freeze()
			observe, err := appObject.Attr("observe")
			if err != nil {
				t.Fatal("no such method: test.observe")
			}
			// TODO(kcza): This is not nice! Fix it!
			thread.SetLocal(starform.RunDataLocalKey, &starform.RunData{})
			args := starlark.Tuple{starlark.String("asdf"), observe}
			for i := 0; i < st.N; i++ {
				_, err := starlark.Call(thread, observe, args, nil)
				if err != nil {
					st.Error(err)
				}
			}
			st.KeepAlive(thread.Local(starform.RunDataLocalKey))
		})
	})

	t.Run("cancellaton", func(t *testing.T) {
		st := startest.From(t)
		st.RequireSafety(starlark.TimeSafe)
		st.SetMaxSteps(0)
		st.RunThread(func(thread *starlark.Thread) {
			thread.Cancel("done")

			appObject := starform.NewAppObject("test")
			appObject.Freeze()
			observe, err := appObject.Attr("observe")
			if err != nil {
				t.Fatal("no such method: test.observe")
			}
			args := starlark.Tuple{starlark.String("asdf"), observe}
			for i := 0; i < st.N; i++ {
				_, err := starlark.Call(thread, observe, args, nil)
				if err != nil {
					st.Error(err)
				}
			}
		})
	})
}

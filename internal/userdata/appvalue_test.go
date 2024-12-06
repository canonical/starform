package userdata_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/canonical/starform/formtest"
	"github.com/canonical/starform/internal/userdata"
	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/startest"
)

func isStarlarkCancellation(err error) bool {
	return strings.Contains(err.Error(), "Starlark computation cancelled:")
}

var noAttrs = func(_ *starlark.Thread, _ string) (starlark.Value, error) {
	return nil, starlark.ErrNoAttr
}

func TestAppAsStarlarkValue(t *testing.T) {
	const appName = "testApp"

	app := &starform.AppObject{
		Name: appName,
	}
	appValue := userdata.NewAppValue(app)
	appValue.Freeze()

	if !bool(appValue.Truth()) {
		t.Errorf("app should be truthy")
	}
	if appString := appValue.String(); appString != fmt.Sprintf("<app %s>", appName) {
		t.Errorf("incorrect string representation: expected %q but got %q", appName, appString)
	}
	if appType := appValue.Type(); appType != "App" {
		t.Errorf("incorrect type representation: expected %q but got %q", appName, "App")
	}
	if _, err := appValue.Hash(); err == nil {
		t.Errorf("app should not be hashable")
	}
}

func TestAppValueSafeString(t *testing.T) {
	const appName = "testApp"

	t.Run("nil-thread", func(t *testing.T) {
		defer func() {
			if err := recover(); err != nil {
				t.Errorf("unexpected panic: %s", err)
			}
		}()

		app := &starform.AppObject{
			Name: appName,
		}
		appValue := userdata.NewAppValue(app)
		appValue.Freeze()
		sb := &strings.Builder{}
		appValue.SafeString(nil, sb)
		if str := sb.String(); str != fmt.Sprintf("<app %s>", appName) {
			t.Error("invalid SafeString value")
		}
	})

	t.Run("regular-operation", func(t *testing.T) {
		st := startest.From(t)
		st.RequireSafety(starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe)
		st.SetMaxSteps(int64(len(fmt.Sprintf("<app %s>", appName))))
		st.RunThread(func(thread *starlark.Thread) {
			appValue := userdata.NewAppValue(&starform.AppObject{
				Name: appName,
			})
			for i := 0; i < st.N; i++ {
				sb := starlark.NewSafeStringBuilder(thread)
				if err := appValue.SafeString(thread, sb); err != nil {
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
			appValue := userdata.NewAppValue(&starform.AppObject{
				Name: appName,
			})
			for i := 0; i < st.N; i++ {
				sb := starlark.NewSafeStringBuilder(thread)
				if err := appValue.SafeString(thread, sb); err == nil {
					st.Error("expected cancellation")
				} else if !isStarlarkCancellation(err) {
					st.Errorf("expected cancellation, got %v", err)
				}
			}
		})
	})
}

func TestAppValueSafeAttr(t *testing.T) {
	app := &starform.AppObject{
		Name: "test",
	}

	t.Run("allocs-steps-io", func(t *testing.T) {
		ft := formtest.From(t)
		ft.SetApp(app)
		ft.SetEvent(&starform.EventObject{
			Name:  userdata.LoadEventName,
			State: &userdata.InitState{},
		})
		ft.RequireSafety(starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe)
		ft.SetMaxSteps(0)
		ft.RunThread(func(thread *starlark.Thread) {
			appValue := ft.MakeAppValue()
			for i := 0; i < ft.N; i++ {
				result, err := appValue.(starlark.HasSafeAttrs).SafeAttr(thread, "observe")
				if err != nil {
					ft.Error(err)
				}
				ft.KeepAlive(result)
			}
		})
	})

	t.Run("cancellaton", func(t *testing.T) {
		ft := formtest.From(t)
		ft.SetApp(app)
		ft.SetEvent(&starform.EventObject{
			Name:  userdata.LoadEventName,
			State: &userdata.InitState{},
		})
		ft.RequireSafety(starlark.TimeSafe)
		ft.SetMaxSteps(0)
		ft.RunThread(func(thread *starlark.Thread) {
			thread.Cancel("done")
			appValue := ft.MakeAppValue()
			for i := 0; i < ft.N; i++ {
				_, err := appValue.(starlark.HasSafeAttrs).SafeAttr(thread, "observe")
				if err != nil && !isStarlarkCancellation(err) {
					ft.Error(err)
				}
			}
		})
	})
}

func TestAppValueAttrNames(t *testing.T) {
	makeGetter := func(name string) *starlark.Builtin {
		const methodSafey = starlark.CPUSafe | starlark.MemSafe | starlark.TimeSafe | starlark.IOSafe
		return starlark.NewBuiltinWithSafety(name, methodSafey, func(thread *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
			if err := thread.AddAllocs(starlark.StringTypeOverhead); err != nil {
				return nil, err
			}
			return starlark.String(name), nil
		})
	}

	tests := []struct {
		name          string
		inputMethods  []*starlark.Builtin
		expectedAttrs []string
	}{{
		name: "non-overloading",
		inputMethods: []*starlark.Builtin{
			makeGetter("foo"),
			makeGetter("bar"),
			makeGetter("baz"),
			makeGetter("qux"),
		},
		expectedAttrs: []string{"bar", "baz", "foo", "observe", "qux"},
	}, {
		name: "overloading",
		inputMethods: []*starlark.Builtin{
			makeGetter("foo"),
			makeGetter("bar"),
			makeGetter("baz"),
			makeGetter("observe"),
			makeGetter("qux"),
		},
		expectedAttrs: []string{"bar", "baz", "foo", "observe", "qux"},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := &starform.AppObject{
				Name:    "test",
				Methods: test.inputMethods,
			}
			appValue := userdata.NewAppValue(app)
			appValue.Freeze()

			attrNames := appValue.AttrNames()
			if len(attrNames) != len(test.expectedAttrs) {
				t.Errorf("unexpected number of attributes: expected %d, got %d", len(test.expectedAttrs), len(attrNames))
			}
			sort.Strings(attrNames)
			for i := range test.expectedAttrs {
				if test.expectedAttrs[i] != attrNames[i] {
					t.Errorf("unexpected attribute %v", attrNames[i])
				}
			}
		})
	}
}

func TestAppValueObserveSafety(t *testing.T) {
	app := &starform.AppObject{
		Name: "test",
	}

	observer := starlark.NewBuiltin("observer", func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})

	t.Run("return-value", func(t *testing.T) {
		ft := formtest.From(t)
		ft.SetApp(app)
		ft.SetEvent(&starform.EventObject{
			Name:  userdata.LoadEventName,
			State: &userdata.InitState{},
		})
		ft.RunString(fmt.Sprintf(`
			def init():
				test.observe('%s', on_load)

			def on_load(event):
				assert.eq(test.observe('foo', lambda x: x), None)
		`, userdata.LoadEventName))
	})

	t.Run("allocs-steps-io-safety", func(t *testing.T) {
		ft := formtest.From(t)
		ft.SetApp(app)
		ft.SetEvent(&starform.EventObject{
			Name:  userdata.LoadEventName,
			State: &userdata.InitState{},
		})
		ft.RequireSafety(starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe)
		ft.SetMaxSteps(1)
		ft.RunThread(func(thread *starlark.Thread) {
			appValue := ft.MakeAppValue()

			observe, _ := appValue.(starlark.HasSafeAttrs).SafeAttr(thread, "observe")
			if observe == nil {
				t.Fatal("no such method: test.observe")
			}

			args := starlark.Tuple{starlark.String("event_name"), observer}
			for i := 0; i < ft.N; i++ {
				_, err := starlark.Call(thread, observe, args, nil)
				if err != nil {
					ft.Error(err)
				}
			}
		})
	})

	t.Run("cancellaton", func(t *testing.T) {
		ft := formtest.From(t)
		ft.SetApp(app)
		ft.SetEvent(&starform.EventObject{
			Name:  userdata.LoadEventName,
			State: &userdata.InitState{},
		})
		ft.RequireSafety(starlark.TimeSafe)
		ft.SetMaxSteps(0)
		ft.RunThread(func(thread *starlark.Thread) {
			thread.Cancel("done")
			appValue := ft.MakeAppValue().(starlark.HasSafeAttrs)
			observe, _ := appValue.SafeAttr(thread, "observe")
			if observe == nil {
				t.Fatal("no such method: test.observe")
			}

			args := starlark.Tuple{starlark.String("event_name"), observer}
			_, err := starlark.Call(thread, observe, args, nil)
			if err != nil && !isStarlarkCancellation(err) {
				ft.Error(err)
			}
		})
	})
}

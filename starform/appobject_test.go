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
	const appObjectName = "testObject"

	appObject := starform.NewAppObject(appObjectName)
	appObject.Freeze()

	if !bool(appObject.Truth()) {
		t.Errorf("app object should be truthy")
	}
	if appObjectString := appObject.String(); appObjectString != appObjectName {
		t.Errorf("incorrect string representation: expected %q but got %q", appObjectName, appObjectString)
	}
	if appObjectType := appObject.Type(); appObjectType != appObjectName {
		t.Errorf("incorrect type representation: expected %q but got %q", appObjectName, appObjectType)
	}
	if _, err := appObject.Hash(); err == nil {
		t.Errorf("app object should not be hashable")
	}
}

func TestAppObjectSafeString(t *testing.T) {
	const appObjectName = "testObject"

	t.Run("nil-thread", func(t *testing.T) {
		defer func() {
			if err := recover(); err != nil {
				t.Errorf("unexpected panic: %s", err)
			}
		}()

		appObject := starform.NewAppObject(appObjectName)
		sb := &strings.Builder{}
		appObject.SafeString(nil, sb)
		if str := sb.String(); str != appObjectName {
			t.Error("invalid SafeString value")
		}
	})

	t.Run("regular-operation", func(t *testing.T) {
		st := startest.From(t)
		st.RequireSafety(starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe)
		st.SetMaxSteps(uint64(len(appObjectName)))
		st.RunThread(func(thread *starlark.Thread) {
			appObject := starform.NewAppObject(appObjectName)
			for i := 0; i < st.N; i++ {
				sb := starlark.NewSafeStringBuilder(thread)
				if err := appObject.SafeString(thread, sb); err != nil {
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
			appObject := starform.NewAppObject(appObjectName)
			for i := 0; i < st.N; i++ {
				sb := starlark.NewSafeStringBuilder(thread)
				if err := appObject.SafeString(thread, sb); err == nil {
					st.Error("expected cancellation")
				} else if !isStarlarkCancellation(err) {
					st.Errorf("expected cancellation, got %v", err)
				}
			}
		})
	})
}

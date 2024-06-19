package starform_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/startest"
)

func TestEventSafeString(t *testing.T) {
	const eventName = "test-event"

	t.Run("nil-thread", func(t *testing.T) {
		defer func() {
			if err := recover(); err != nil {
				t.Errorf("unexpected panic: %s", err)
			}
		}()

		event := &starform.EventObject{
			Name: eventName,
		}
		sb := &strings.Builder{}
		event.SafeString(nil, sb)
	})

	t.Run("regular-operation", func(t *testing.T) {
		st := startest.From(t)
		st.RequireSafety(starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe)
		st.SetMaxSteps(uint64(len(fmt.Sprintf("<Event %s>", eventName))))
		st.RunThread(func(thread *starlark.Thread) {
			event := &starform.EventObject{
				Name: eventName,
			}
			for i := 0; i < st.N; i++ {
				sb := starlark.NewSafeStringBuilder(thread)
				if err := event.SafeString(thread, sb); err != nil {
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
			event := &starform.EventObject{
				Name: eventName,
			}
			for i := 0; i < st.N; i++ {
				sb := starlark.NewSafeStringBuilder(thread)
				if err := event.SafeString(thread, sb); err == nil {
					st.Error("expected cancellation")
				} else if !isStarlarkCancellation(err) {
					st.Errorf("expected cancellation, got %v", err)
				}
			}
		})
	})
}

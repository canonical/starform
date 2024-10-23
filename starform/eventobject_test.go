package starform_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/canonical/starform/formtest"
	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
)

func isStarlarkCancellation(err error) bool {
	return strings.Contains(err.Error(), "Starlark computation cancelled:")
}

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
		ft := formtest.From(t)
		ft.SetApp(&starform.AppObject{
			Name: "test",
		})
		ft.SetEvent(&starform.EventObject{
			Name: eventName,
		})
		ft.RequireSafety(starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe)
		ft.SetMaxSteps(int64(len(fmt.Sprintf("<Event %s>", eventName))))
		ft.RunThread(func(thread *starlark.Thread) {
			event := starform.Event(thread)
			for i := 0; i < ft.N; i++ {
				sb := starlark.NewSafeStringBuilder(thread)
				if err := event.SafeString(thread, sb); err != nil {
					ft.Error(err)
				}
				if err := sb.Err(); err != nil {
					ft.Error(err)
				}
				if err := thread.AddAllocs(starlark.StringTypeOverhead); err != nil {
					ft.Error(err)
				}
				ft.KeepAlive(sb.String())
			}
		})
	})

	t.Run("cancellation", func(t *testing.T) {
		ft := formtest.From(t)
		ft.SetApp(&starform.AppObject{
			Name: "test",
		})
		ft.SetEvent(&starform.EventObject{
			Name: eventName,
		})
		ft.RequireSafety(starlark.TimeSafe)
		ft.SetMaxSteps(0)
		ft.RunThread(func(thread *starlark.Thread) {
			thread.Cancel("done")
			event := starform.Event(thread)
			for i := 0; i < ft.N; i++ {
				sb := starlark.NewSafeStringBuilder(thread)
				if err := event.SafeString(thread, sb); err == nil {
					ft.Error("expected cancellation")
				} else if !isStarlarkCancellation(err) {
					ft.Errorf("expected cancellation, got %v", err)
				}
			}
		})
	})
}

func TestEventObjectSafeAttr(t *testing.T) {
	event := &starform.EventObject{
		Name: "test-event",
		Attrs: starlark.StringDict{
			"foo": starlark.String("bar"),
		},
	}

	attrNames := event.AttrNames()
	if len(attrNames) != len(event.Attrs)+1 {
		t.Fatalf("expected 2 attributes, got %d", len(attrNames))
	}

	for _, attrName := range attrNames {
		t.Run(attrName, func(t *testing.T) {
			t.Run("allocs-steps-io-safety", func(t *testing.T) {
				ft := formtest.From(t)
				ft.SetApp(&starform.AppObject{
					Name: "test",
				})
				ft.SetEvent(event)
				ft.RequireSafety(starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe)
				ft.SetMaxSteps(0)
				ft.RunThread(func(thread *starlark.Thread) {
					for i := 0; i < ft.N; i++ {
						result, err := event.SafeAttr(thread, attrName)
						if err != nil {
							ft.Error(err)
						}
						ft.KeepAlive(result)
					}
				})
			})
		})

		t.Run("cancellaton", func(t *testing.T) {
			ft := formtest.From(t)
			ft.RequireSafety(starlark.TimeSafe)
			ft.SetMaxSteps(0)
			ft.SetApp(&starform.AppObject{
				Name: "test",
			})
			ft.SetEvent(&starform.EventObject{})
			ft.RunThread(func(thread *starlark.Thread) {
				thread.Cancel("done")
				for i := 0; i < ft.N; i++ {
					_, err := event.SafeAttr(thread, attrName)
					if err != nil && !isStarlarkCancellation(err) {
						ft.Error(err)
					}
				}
			})
		})
	}
}

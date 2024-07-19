package starform_test

import (
	"testing"

	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/startest"
)

func TestDeclareIntentsSteps(t *testing.T) {
	intent := interface{}("put up shelves")

	st := startest.From(t)
	st.RequireSafety(starlark.CPUSafe)
	st.SetMinSteps(1)
	st.SetMaxSteps(1)
	st.RunThread(func(thread *starlark.Thread) {
		event := &starform.EventObject{Name: "unused"}
		starform.PrepareEvent(thread, event)
		for i := 0; i < st.N; i++ {
			if err := starform.DeclareIntent(thread, intent); err != nil {
				t.Error(err)
			}
		}
	})
}

func TestDeclareIntentsAllocs(t *testing.T) {
	intent := interface{}("hello, world")

	st := startest.From(t)
	st.RequireSafety(starlark.MemSafe)
	st.RunThread(func(thread *starlark.Thread) {
		event := &starform.EventObject{Name: "unused"}
		starform.PrepareEvent(thread, event)
		for i := 0; i < st.N; i++ {
			if err := starform.DeclareIntent(thread, intent); err != nil {
				t.Error(err)
			}
		}
	})
}

func TestDeclareIntentsCancellation(t *testing.T) {
	intent := interface{}("hello, world")

	st := startest.From(t)
	st.RequireSafety(starlark.TimeSafe)
	st.RunThread(func(thread *starlark.Thread) {
		thread.Cancel("done")

		event := &starform.EventObject{Name: "unused"}
		starform.PrepareEvent(thread, event)

		if err := starform.DeclareIntent(thread, intent); err != nil && !isStarlarkCancellation(err) {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

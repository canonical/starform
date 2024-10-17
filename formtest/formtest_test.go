package formtest_test

import (
	"testing"

	"github.com/canonical/starform/formtest"
	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
)

func TestExampleString(t *testing.T) {
	ft := formtest.From(t)
	ft.SetApp(&starform.AppObject{
		Name: "app",
	})
	ft.SetEvent(&starform.EventObject{
		Name:  "event",
		State: "anything_is_ok",
	})
	ft.RunString(`
		assert.eq(1, 2)

		def init():
			app.observe('event', on_event)

		def on_event(event):
			for _ in st.ntimes():
				debug('hello, world')
	`)
}

func TestExampleThread(t *testing.T) {
	ft := formtest.From(t)
	ft.SetApp(&starform.AppObject{
		Name: "app",
	})
	ft.SetEvent(&starform.EventObject{
		Name:  "event",
		State: "anything_is_ok",
	})
	ft.RunThread(func(thread *starlark.Thread) {
		// ...
	})
}

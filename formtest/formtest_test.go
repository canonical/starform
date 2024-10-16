package formtest_test

import (
	"testing"

	"github.com/canonical/starform/formtest"
	"github.com/canonical/starform/starform"
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
			assert.eq(2, 3)
			app.observe('event', on_event)

		def on_event(event):
			assert.eq(4, 6)
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
	ft.RunString(`
		def init():
			app.observe('event', on_event)

		def on_event(event):
			debug('hello, world')
	`)
}

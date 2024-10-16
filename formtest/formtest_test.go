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
		def init():
			app.observe('event', on_event)

		def on_event(event):
			debug('hello, world')
	`)
}

func TestExample(t *testing.T) {
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

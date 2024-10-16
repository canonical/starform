package starform

import (
	"github.com/canonical/starform/internal"
	"github.com/canonical/starlark/starlark"
)

type TestCacheBase struct{}

func (*TestCacheBase) private() {}

const LoadEventName = loadEventName

func SetEventObject(thread *starlark.Thread, data *EventObject) {
	thread.SetLocal(internal.RunDataLocalKey, &internal.Rundata{
		Thread: thread,
		Event:  data,
	})
}

func (app *AppObject) Value() starlark.HasSafeAttrs {
	return app.value()
}

var PrintBuiltin = printBuiltin
var DebugBuiltin = debugBuiltin

func NewScriptLogger(logger Logger, path string) starlark.Value {
	return &scriptLogger{
		logger: logger,
		path:   path,
	}
}

var AfterFunc = afterFunc

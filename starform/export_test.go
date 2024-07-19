package starform

import "github.com/canonical/starlark/starlark"

type TestCacheBase struct{}

func (*TestCacheBase) private() {}

const LoadEventName = loadEventName

func PrepareEvent(thread *starlark.Thread, data *EventObject) {
	thread.SetLocal(eventObjectLocalKey, data)
	thread.SetLocal(intentStoreLocalKey, &intentStore{})
}

func (app *AppObject) Value() starlark.HasSafeAttrs {
	return app.value()
}

func InitState() interface{} {
	return &initState{
		phase: initPhase,
	}
}

var PrintBuiltin = printBuiltin
var DebugBuiltin = debugBuiltin

func NewScriptLogger(logger Logger, path string) starlark.Value {
	return &scriptLogger{
		logger: logger,
		path:   path,
	}
}

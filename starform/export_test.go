package starform

import (
	"github.com/canonical/starform/internal/userdata"
	"github.com/canonical/starlark/starlark"
)

type TestCacheBase struct{}

func (*TestCacheBase) private() {}

const LoadEventName = userdata.LoadEventName

func SetEventObject(thread *starlark.Thread, data *EventObject) {
	thread.SetLocal(userdata.EventObjectLocalKey, data)
}

func (app *AppObject) Value() starlark.HasSafeAttrs {
	return app.value()
}

func InitState() interface{} {
	return &initState{
		eventObservers: make(map[string][]starlark.Callable),
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

var AfterFunc = afterFunc

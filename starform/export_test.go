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

func NewInitEvent() *EventObject {
	return &EventObject{
		Name: userdata.LoadEventName,
		State: &userdata.InitState{
			EventObservers: map[string][]starlark.Callable{},
		},
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

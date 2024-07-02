package starform

import "github.com/canonical/starlark/starlark"

type TestCacheBase struct{}

func (*TestCacheBase) private() {}

const LoadEventName = loadEventName

func SetEventObject(thread *starlark.Thread, data *EventObject) {
	thread.SetLocal(eventObjectLocalKey, data)
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

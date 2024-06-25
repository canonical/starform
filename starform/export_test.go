package starform

import "github.com/canonical/starlark/starlark"

type TestCacheBase struct{}

func (*TestCacheBase) private() {}

type RunData = runData

func LoadingRunData() *RunData {
	return &RunData{
		eventName: LoadEventName,
	}
}

func InitingRunData() *RunData {
	return &RunData{
		eventName:        LoadEventName,
		observeAvailable: true,
		observers:        make(map[string][]starlark.Callable),
	}
}

func PutRunDataIn(thread *starlark.Thread, data *RunData) {
	thread.SetLocal(runDataLocalKey, data)
}

var PrintBuiltin = printBuiltin
var DebugBuiltin = debugBuiltin

func NewScriptLogger(logger Logger, path string) starlark.Value {
	return &scriptLogger{
		logger: logger,
		path:   path,
	}
}

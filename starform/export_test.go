package starform

import "github.com/canonical/starlark/starlark"

type TestCacheBase struct{}

func (*TestCacheBase) private() {}

func InitingRunData() *EventRunData {
	return &EventRunData{
		EventName: LoadEventName,
		observers: make(map[string][]starlark.Callable),
	}
}

func PutRunDataIn(thread *starlark.Thread, data *EventRunData) {
	thread.SetLocal(runDataLocalKey, data)
}

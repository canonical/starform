package starform

import "github.com/canonical/starlark/starlark"

type TestCacheBase struct{}

func (*TestCacheBase) private() {}

func SetEventObject(thread *starlark.Thread, data *EventObject) {
	thread.SetLocal(eventObjectLocalKey, data)
}

func InitState() interface{} {
	return &initState{
		eventObservers: make(map[string][]starlark.Callable),
	}
}

const LoadEventName = loadEventName

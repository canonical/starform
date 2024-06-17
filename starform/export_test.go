package starform

import "github.com/canonical/starlark/starlark"

type TestCacheBase struct{}

func (*TestCacheBase) private() {}

func SetRunData(thread *starlark.Thread, data *EventRunData) {
	thread.SetLocal(runDataLocalKey, data)
}

func InitState() interface{} {
	return &initState{
		eventObservers: make(map[string][]starlark.Callable),
	}
}

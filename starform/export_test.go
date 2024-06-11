package starform

import "github.com/canonical/starlark/starlark"

type TestCacheBase struct{}

func (*TestCacheBase) private() {}

func PutRunDataIn(thread *starlark.Thread, data *EventRunData) {
	thread.SetLocal(runDataLocalKey, data)
}

func NewAppValue(app *App) starlark.HasSafeAttrs {
	return newAppValue(app)
}

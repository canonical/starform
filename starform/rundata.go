package starform

import (
	"github.com/canonical/starlark/starlark"
)

var LoadEventName = "<load>"

type EventRunData struct {
	EventName string
	State     interface{}
}

const runDataLocalKey = "starform-run-data"

func RunData(thread *starlark.Thread) *EventRunData {
	ret, ok := thread.Local(runDataLocalKey).(*EventRunData)
	if !ok {
		panic("starform internal data missing")
	}
	return ret
}

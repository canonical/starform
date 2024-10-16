package internal

import (
	"github.com/canonical/starlark/starlark"
)

type Rundata struct {
	Event  *EventObject
	Thread *starlark.Thread
}

const RunDataLocalKey = "starform-event-object"

func Event(thread *starlark.Thread) *EventObject {
	ret, ok := thread.Context().Value(RunDataLocalKey).(*Rundata)
	if !ok {
		return &EventObject{} // Avoid panics.
	}
	// if ret.Thread == nil {
	// 	ret.Thread = thread
	// }
	return ret.Event
}

type InitState struct {
	EventObservers map[string][]starlark.Callable
}

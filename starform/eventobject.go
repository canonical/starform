package starform

import (
	"github.com/canonical/starlark/starlark"
)

const LoadEventName = "<load>"

type EventObject struct {
	Name string

	// State is the developer-supplied value passed to the (*ScriptSet).Handle
	// method. This is never used by Starform.
	State interface{}
}

type initState struct {
	eventObservers map[string][]starlark.Callable
}

const eventObjectLocalKey = "starform-event-object"

func Event(thread *starlark.Thread) *EventObject {
	ret, ok := thread.Local(eventObjectLocalKey).(*EventObject)
	if !ok {
		panic("starform internal data missing")
	}
	return ret
}

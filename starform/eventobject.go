package starform

import (
	"github.com/canonical/starlark/starlark"
)

const loadEventName = "<load>"

type EventObject struct {
	Name string

	// State is the developer-supplied value passed to the (*ScriptSet).Handle
	// method. Starform itself does not use this after the load phase.
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

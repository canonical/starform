package starform

import (
	"github.com/canonical/starlark/starlark"
)

const loadEventName = "<load>"

type EventObject struct {
	Name string

	// State is the developer-supplied value passed to the ScriptSet.Handle method.
	State interface{}
}

type loadPhase int

const (
	topLevelPhase loadPhase = iota
	initPhase
)

type initState struct {
	phase loadPhase
}

type observeIntent struct {
	event    string
	observer starlark.Callable
}

const eventObjectLocalKey = "starform-event-object"

func Event(thread *starlark.Thread) *EventObject {
	ret, ok := thread.Local(eventObjectLocalKey).(*EventObject)
	if !ok {
		return &EventObject{} // Avoid panics.
	}
	return ret
}

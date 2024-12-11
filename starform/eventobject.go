package starform

import (
	"github.com/canonical/starform/internal/userdata"
	"github.com/canonical/starlark/starlark"
)

type EventObject = userdata.EventObject

func Event(thread *starlark.Thread) *EventObject {
	return userdata.Event(thread)
}

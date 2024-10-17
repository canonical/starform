package starform

import (
	"github.com/canonical/starform/internal"
	"github.com/canonical/starlark/starlark"
)

type EventObject = internal.EventObject

func Event(thread *starlark.Thread) *EventObject {
	return internal.Event(thread)
}

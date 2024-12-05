package starform

import (
	"github.com/canonical/starform/internal/lib"
	"github.com/canonical/starlark/starlark"
)

type EventObject = lib.EventObject

func Event(thread *starlark.Thread) *EventObject {
	return lib.Event(thread)
}

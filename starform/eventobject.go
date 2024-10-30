package starform

import (
	"fmt"

	"github.com/canonical/starlark/starlark"
)

const loadEventName = "<load>"

// eventObjectLocalKey must have the same value as formtest.eventObjectLocalKey
const eventObjectLocalKey = "starform-event-object"

// eventObjectStorage is used as an indirection to store the event
// object in the thread locals, so that the event can be modified.
// This type must remain an unnamed type and must be kept in
// sync with formtest.eventObjectStorage.
type eventObjectStorage = struct {
	Event  *EventObject
	Thread *starlark.Thread
}

type EventObject struct {
	Name string

	// State is the developer-supplied value passed to the ScriptSet.Handle method.
	State interface{}

	Attrs starlark.StringDict
}

var _ starlark.HasSafeAttrs = &EventObject{}
var _ starlark.SafeStringer = &EventObject{}

func (e *EventObject) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: %s", e.Type()) }
func (e *EventObject) Truth() starlark.Bool  { return starlark.True }
func (e *EventObject) Type() string          { return "Event" }
func (e *EventObject) String() string        { return fmt.Sprintf("<Event %s>", e.Name) }

func (e *EventObject) SafeString(thread *starlark.Thread, sb starlark.StringBuilder) error {
	const safety = starlark.CPUSafe | starlark.MemSafe | starlark.TimeSafe | starlark.IOSafe
	if err := starlark.CheckSafety(thread, safety); err != nil {
		return err
	}

	_, err := sb.WriteString(e.String())
	return err
}

func (e *EventObject) Freeze() {
	for _, attr := range e.Attrs {
		attr.Freeze()
	}
}

func (e *EventObject) AttrNames() []string {
	attrNames := e.Attrs.Keys()
	if _, ok := e.Attrs["name"]; ok {
		return attrNames
	}
	return append(e.Attrs.Keys(), "name")
}

func (e *EventObject) Attr(name string) (starlark.Value, error) {
	return e.SafeAttr(nil, name)
}

func (e *EventObject) SafeAttr(thread *starlark.Thread, name string) (starlark.Value, error) {
	const safety = starlark.CPUSafe | starlark.MemSafe | starlark.TimeSafe | starlark.IOSafe
	if err := starlark.CheckSafety(thread, safety); err != nil {
		return nil, err
	}

	if attr, ok := e.Attrs[name]; ok {
		return attr, nil
	}
	if name != "name" {
		return nil, starlark.ErrNoAttr
	}
	if err := thread.AddAllocs(starlark.StringTypeOverhead); err != nil {
		return nil, err
	}
	return starlark.String(e.Name), nil
}

type initState struct {
	eventObservers map[string][]starlark.Callable
}

func Event(thread *starlark.Thread) *EventObject {
	storage, ok := thread.Local(eventObjectLocalKey).(*eventObjectStorage)
	if !ok {
		return &EventObject{} // Avoid panics.
	}
	return storage.Event
}

package starform

import (
	"errors"
	"fmt"

	"github.com/canonical/starlark/starlark"
)

// An AppObject is the common point for exposing the state of the application
// into Starlark and for Starlark to declare intents.
type AppObject struct {
	name string
}

var ErrUnavailable = errors.New("unavailable")

func NewAppObject(name string) *AppObject {
	return &AppObject{name: name}
}

var _ starlark.Value = &AppObject{}
var _ starlark.SafeStringer = &AppObject{}
var _ starlark.HasSafeAttrs = &AppObject{}

func (app *AppObject) String() string       { return app.name }
func (app *AppObject) Type() string         { return app.name }
func (app *AppObject) Freeze()              {}
func (app *AppObject) Truth() starlark.Bool { return true }
func (app *AppObject) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", app.Type())
}
func (app *AppObject) SafeString(thread *starlark.Thread, sb starlark.StringBuilder) error {
	if err := starlark.CheckSafety(thread, starlark.CPUSafe|starlark.MemSafe|starlark.TimeSafe|starlark.IOSafe); err != nil {
		return err
	}

	_, err := sb.WriteString(app.String())
	return err
}

func (app *AppObject) AttrNames() []string {
	return []string{"observe"}
}

func (app *AppObject) Attr(name string) (starlark.Value, error) {
	return app.SafeAttr(nil, name)
}

func (app *AppObject) SafeAttr(thread *starlark.Thread, name string) (starlark.Value, error) {
	if thread == nil {
		return nil, errors.New("cannot access app fields in unconstrained environment")
	}

	const safety = starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe | starlark.TimeSafe
	if err := starlark.CheckSafety(thread, safety); err != nil {
		return nil, err
	}

	if name == "observe" {
		event := Event(thread)
		if event.State == nil {
			return nil, ErrUnavailable
		}
		if err := thread.AddAllocs(starlark.EstimateSize(&starlark.Builtin{})); err != nil {
			return nil, err
		}
		return observeBuiltin.BindReceiver(app), nil
	}
	return nil, nil
}

var observeBuiltinSafety = starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe | starlark.TimeSafe
var observeBuiltin = starlark.NewBuiltinWithSafety("observe", observeBuiltinSafety, func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var eventName string
	var observer starlark.Callable
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 2, &eventName, &observer); err != nil {
		return nil, err
	}

	event := Event(thread)
	state := event.State.(*initState)
	obs, ok := state.eventObservers[eventName]
	if !ok {
		// Precondition: events are never removed from data.observers.
		delta := starlark.EstimateMakeSize(map[string][]starlark.Callable{}, 1+len(state.eventObservers)) -
			starlark.EstimateMakeSize(map[string][]starlark.Callable{}, len(state.eventObservers))
		if err := thread.AddAllocs(delta); err != nil {
			return nil, err
		}
	}
	safeAppender := starlark.NewSafeAppender(thread, &obs)
	if err := safeAppender.Append(observer); err != nil {
		return nil, err
	}
	state.eventObservers[eventName] = obs

	return starlark.None, nil
})

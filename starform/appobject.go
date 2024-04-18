package starform

import (
	"fmt"

	"github.com/canonical/starlark/starlark"
)

// An AppObject is the common point for exposing the state of the application
// into Starlark and for Starlark to declare intents.
type AppObject struct {
	name string
}

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
	if err := starlark.CheckSafety(thread, starlark.MemSafe|starlark.CPUSafe|starlark.IOSafe|starlark.TimeSafe); err != nil {
		return nil, err
	}

	if name == "observe" {
		data, err := getRunData(thread)
		if err != nil {
			return nil, err
		}
		if !data.observeAvailable {
			return nil, fmt.Errorf("observe is unavailable")
		}
		if thread != nil {
			if err := thread.AddAllocs(starlark.EstimateSize(&starlark.Builtin{})); err != nil {
				return nil, err
			}
		}
		return observeBuiltin.BindReceiver(app), nil
	}
	return nil, nil
}

var observeBuiltinSafety = starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe | starlark.TimeSafe
var observeBuiltin = starlark.NewBuiltinWithSafety("observe", observeBuiltinSafety, func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var eventName string
	var observer starlark.Value
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 2, &eventName, &observer); err != nil {
		return nil, err
	}

	data, err := getRunData(thread)
	if err != nil {
		return nil, err
	}
	observer.Freeze()
	obs, ok := data.observers[eventName]
	if !ok {
		// Precondition: events are never removed from data.observers.
		delta := starlark.EstimateMakeSize(map[string][]starlark.Value{}, 1+len(data.observers)) -
			starlark.EstimateMakeSize(map[string][]starlark.Value{}, len(data.observers))
		if err := thread.AddAllocs(delta); err != nil {
			return nil, err
		}
		obs = make([]starlark.Value, 0, 1)
	}
	safeAppender := starlark.NewSafeAppender(thread, &obs)
	if err := safeAppender.Append(observer); err != nil {
		return nil, err
	}
	data.observers[eventName] = obs

	return starlark.None, nil
})

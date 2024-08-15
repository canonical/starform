package starform

import (
	"errors"
	"fmt"
	"sort"

	"github.com/canonical/starlark/starlark"
)

// AppObject is the common point for exposing the state of the application
// into Starlark and for Starlark to declare intents.
type AppObject struct {
	Name string

	Methods []*starlark.Builtin
}

var ErrUnavailable = errors.New("unavailable")

func (app *AppObject) value() *appValue {
	attrNames := make([]string, 0, len(app.Methods))
	customMethods := make(map[string]*starlark.Builtin, len(app.Methods))
	for _, method := range app.Methods {
		methodName := method.Name()
		attrNames = append(attrNames, methodName)
		customMethods[methodName] = method
	}
	for _, method := range commonAppMethods {
		methodName := method.Name()
		if _, ok := customMethods[methodName]; ok {
			continue // Method overridden, hence its name is already present.
		}
		attrNames = append(attrNames, methodName)
	}
	sort.Strings(attrNames)

	return &appValue{
		name:          app.Name,
		attrNames:     attrNames,
		customMethods: customMethods,
	}
}

// appValue is the global app value available in all scripts in a script set.
type appValue struct {
	name          string
	attrNames     []string
	customMethods map[string]*starlark.Builtin
}

var _ starlark.Value = &appValue{}
var _ starlark.SafeStringer = &appValue{}
var _ starlark.HasSafeAttrs = &appValue{}

func (app *appValue) String() string       { return fmt.Sprintf("<app %s>", app.name) }
func (app *appValue) Type() string         { return "App" }
func (app *appValue) Freeze()              {}
func (app *appValue) Truth() starlark.Bool { return true }
func (app *appValue) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", app.Type())
}
func (app *appValue) SafeString(thread *starlark.Thread, sb starlark.StringBuilder) error {
	const safety = starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe | starlark.TimeSafe
	if err := starlark.CheckSafety(thread, safety); err != nil {
		return err
	}

	_, err := sb.WriteString(app.String())
	return err
}

func (app *appValue) AttrNames() []string {
	return app.attrNames
}

func (app *appValue) Attr(name string) (starlark.Value, error) {
	return app.SafeAttr(nil, name)
}

func (app *appValue) SafeAttr(thread *starlark.Thread, name string) (starlark.Value, error) {
	if thread == nil {
		return nil, errors.New("cannot access app fields in unconstrained environment")
	}

	const safety = starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe | starlark.TimeSafe
	if err := starlark.CheckSafety(thread, safety); err != nil {
		return nil, err
	}

	methodIsCustom := true
	method, ok := app.customMethods[name]
	if !ok {
		methodIsCustom = false
		method, ok = commonAppMethods[name]
		if !ok {
			return nil, starlark.ErrNoSuchAttr
		}
	}

	event := Event(thread)
	if methodIsCustom {
		if event.Name == loadEventName {
			return nil, ErrUnavailable
		}
	} else {
		if event.Name != loadEventName || event.State == nil {
			// The observe method is only available during init so we can tell
			// which events to be dispatched to the script. For now this is
			// also enforced for all common methods since it's safer to force
			// future patch authors to come here and change this logic than
			// risk methods being used globally by mistake.
			return nil, ErrUnavailable
		}
	}

	if err := thread.AddAllocs(starlark.EstimateSize(&starlark.Builtin{})); err != nil {
		return nil, err
	}
	return method.BindReceiver(app), nil
}

var commonAppMethods = map[string]*starlark.Builtin{
	"observe": starlark.NewBuiltinWithSafety("observe", observeSafety, observe),
}

var observeSafety = starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe | starlark.TimeSafe

func observe(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var eventName string
	var observer starlark.Callable
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 2, &eventName, &observer); err != nil {
		return nil, err
	}

	event := Event(thread)
	if event.Name != loadEventName {
		return nil, ErrUnavailable
	}

	state, ok := event.State.(*initState)
	if !ok {
		return nil, errors.New("starform internal data missing")
	}

	observers, ok := state.eventObservers[eventName]
	if !ok {
		newSize := starlark.EstimateMakeSize(map[string][]starlark.Callable{}, 1+len(state.eventObservers))
		oldSize := starlark.EstimateMakeSize(map[string][]starlark.Callable{}, len(state.eventObservers))
		if err := thread.AddAllocs(newSize, -oldSize); err != nil {
			return nil, err
		}
	}
	observersAppender := starlark.NewSafeAppender(thread, &observers)
	if err := observersAppender.Append(observer); err != nil {
		return nil, err
	}
	state.eventObservers[eventName] = observers

	return starlark.None, nil
}

package starform

import (
	"errors"
	"fmt"
	"sort"
	"strings"

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
	methods := make(map[string]*starlark.Builtin, len(app.Methods))
	for _, method := range app.Methods {
		methodName := method.Name()
		attrNames = append(attrNames, methodName)
		methods[methodName] = method
	}
	sort.Strings(attrNames) // This is necessary for hasAttr to work.

	return &appValue{
		name:      app.Name,
		attrNames: attrNames,
		methods:   methods,
	}
}

// appValue is the global app value available in all scripts in a script set.
type appValue struct {
	name      string
	attrNames []string
	methods   map[string]*starlark.Builtin
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
	// TODO(marco6): use slice package when we bump Go version
	attrNames := make([]string, 0, len(app.attrNames)+len(commonAppMethods))
	for attr := range commonAppMethods {
		if !app.hasAttr(attr) {
			attrNames = append(attrNames, attr)
		}
	}
	return append(attrNames, app.attrNames...)
}

func (app *appValue) hasAttr(needle string) bool {
	_, found := sort.Find(len(app.attrNames), func(i int) int {
		return strings.Compare(needle, app.attrNames[i])
	})
	return found
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

	event := Event(thread)
	if event.Name == loadEventName {
		if app.hasAttr(name) {
			return nil, ErrUnavailable
		}
		method := commonAppMethods[name]
		if method == nil {
			return nil, starlark.ErrNoSuchAttr
		}
		if event.State == nil {
			return nil, ErrUnavailable
		}
		if err := thread.AddAllocs(starlark.EstimateSize(&starlark.Builtin{})); err != nil {
			return nil, err
		}
		return method.BindReceiver(app), nil
	}

	method, ok := app.methods[name]
	if !ok {
		if _, ok := commonAppMethods[name]; ok {
			// Provided App methods are only available during init.
			return nil, ErrUnavailable
		}
		return nil, starlark.ErrNoSuchAttr
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

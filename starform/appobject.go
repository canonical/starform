package starform

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/canonical/starlark/starlark"
)

// App is the common point for exposing the state of the application
// into Starlark and for Starlark to declare intents.
type App struct {
	Name      string
	AttrNames []string

	// Attr returns the attribute with the given name for the thread's current event.
	// If the attribute does not exists, it should return starlark.ErrNoSuchAttr.
	// If the attribute is not available for thread's current event, it should return ErrUnavailable.
	Attr func(thread *starlark.Thread, name string) (starlark.Value, error)
}

var ErrUnavailable = errors.New("unavailable")

func (app *App) value() *appValue {
	attrNames := make([]string, len(app.AttrNames))
	copy(attrNames, app.AttrNames)
	sort.Strings(attrNames) // this is necessary for hasAttr to work

	return &appValue{
		name:      app.Name,
		attrNames: attrNames,
		attr:      app.Attr,
	}
}

// appValue represents the global app value available in all scripts in a script set.
type appValue struct {
	name      string
	attrNames []string
	attr      func(thread *starlark.Thread, name string) (starlark.Value, error)
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
	attrNames := make([]string, 0, len(app.attrNames)+len(appMethods))
	for attr := range appMethods {
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
		return nil, fmt.Errorf("can't access app fields in unconstrained environment")
	}
	const safety = starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe | starlark.TimeSafe
	if err := starlark.CheckSafety(thread, safety); err != nil {
		return nil, err
	}

	rundata := RunData(thread)
	if rundata.EventName == LoadEventName {
		if app.hasAttr(name) {
			return nil, ErrUnavailable
		}
		b := appMethods[name]
		if b == nil {
			return nil, starlark.ErrNoSuchAttr
		}
		if rundata.State == nil {
			return nil, ErrUnavailable
		}
		if err := thread.AddAllocs(starlark.EstimateSize(&starlark.Builtin{})); err != nil {
			return nil, err
		}
		return b.BindReceiver(app), nil
	}

	attr, err := app.attr(thread, name)
	if isNoSuchAttr(err) {
		if _, ok := appMethods[name]; ok {
			return nil, ErrUnavailable
		}
	}
	return attr, err
}

func isNoSuchAttr(err error) bool {
	if err == nil {
		return false
	}
	if err == starlark.ErrNoSuchAttr {
		return true
	}
	_, ok := err.(*starlark.NoSuchAttrError)
	return ok
}

var appMethods = map[string]*starlark.Builtin{
	"observe": starlark.NewBuiltinWithSafety("observe", observeBuiltinSafety, observe),
}

var observeBuiltinSafety = starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe | starlark.TimeSafe

func observe(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var eventName string
	var observer starlark.Callable
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 2, &eventName, &observer); err != nil {
		return nil, err
	}

	data := RunData(thread)
	state := data.State.(*initState)
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

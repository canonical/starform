package starform

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/canonical/starlark/starlark"
)

// An App is the common point for exposing the state of the application
// into Starlark and for Starlark to declare intents.
type App struct {
	Name      string
	AttrNames []string
	Attr      func(thread *starlark.Thread, name string) (starlark.Value, error)
}

func (app *App) value() *appValue {
	attrNames := make([]string, len(app.AttrNames))
	copy(attrNames, app.AttrNames)
	sort.Strings(attrNames)

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

	_, err := fmt.Fprintf(sb, "<app %s>", app.name)
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
	attrNames = append(attrNames, app.attrNames...)
	sort.Strings(attrNames)
	return attrNames
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

var ErrUnavailable = errors.New("unavailable") // FIXME(marco6): better error message

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
	obs, ok := state.observers[eventName]
	if !ok {
		newSize := starlark.EstimateMakeSize(map[string][]starlark.Callable{}, 1+len(state.observers))
		oldSize := starlark.EstimateMakeSize(map[string][]starlark.Callable{}, len(state.observers))
		if err := thread.AddAllocs(newSize, -oldSize); err != nil {
			return nil, err
		}
	}
	safeAppender := starlark.NewSafeAppender(thread, &obs)
	if err := safeAppender.Append(observer); err != nil {
		return nil, err
	}
	state.observers[eventName] = obs

	return starlark.None, nil
}

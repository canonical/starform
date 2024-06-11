package starform

import (
	"fmt"
	"sort"
	"strings"

	"github.com/canonical/starlark/starlark"
)

type App struct {
	Name      string
	AttrNames []string
	Attr      func(thread *starlark.Thread, name string) (starlark.Value, error)
}

// An appValue is the common point for exposing the state of the application
// into Starlark and for Starlark to declare intents.
type appValue struct {
	name      string
	attrNames []string
	attr      func(thread *starlark.Thread, name string) (starlark.Value, error)
}

func newAppValue(app *App) *appValue {
	// FIXME should I duplicate AttrNames?
	sort.Strings(app.AttrNames)

	attr := app.Attr
	if attr == nil {
		attr = func(thread *starlark.Thread, name string) (starlark.Value, error) {
			return nil, starlark.ErrNoSuchAttr
		}
	}
	return &appValue{
		name:      app.Name,
		attrNames: app.AttrNames,
		attr:      attr,
	}
}

var _ starlark.Value = &appValue{}
var _ starlark.SafeStringer = &appValue{}
var _ starlark.HasSafeAttrs = &appValue{}

func (app *appValue) String() string       { return fmt.Sprintf("<App %s>", app.name) }
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

	_, err := fmt.Fprintf(sb, "<App %s>", app.name)
	return err
}

func (app *appValue) AttrNames() []string {
	// TODO(marco6): use slice package when we bump Go version
	attrNames := make([]string, 0, len(app.attrNames)+len(appObjectMethods))
	for attr := range appObjectMethods {
		if hasAttr(app.attrNames, attr) {
			attrNames = append(attrNames, attr)
		}
	}
	attrNames = append(attrNames, app.attrNames...)
	sort.Strings(attrNames)
	return attrNames
}

func hasAttr(haystack []string, needle string) bool {
	_, found := sort.Find(len(haystack), func(i int) int {
		return strings.Compare(needle, haystack[i])
	})
	return found
}

func (app *appValue) Attr(name string) (starlark.Value, error) {
	return app.SafeAttr(nil, name)
}

var ErrUnavailable = fmt.Errorf("unavailable") // FIXME(marco6): better error message

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
		if hasAttr(app.attrNames, name) {
			return nil, ErrUnavailable
		}
		b := appObjectMethods[name]
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
		if _, ok := appObjectMethods[name]; ok {
			return nil, ErrUnavailable
		}
	}
	return attr, err
}

var appObjectMethods = map[string]*starlark.Builtin{
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
	observers := data.State.(map[string][]starlark.Callable)
	obs, ok := observers[eventName]
	if !ok {
		newSize := starlark.EstimateMakeSize(map[string][]starlark.Callable{}, 1+len(observers))
		oldSize := starlark.EstimateMakeSize(map[string][]starlark.Callable{}, len(observers))
		if err := thread.AddAllocs(newSize, -oldSize); err != nil {
			return nil, err
		}
	}
	safeAppender := starlark.NewSafeAppender(thread, &obs)
	if err := safeAppender.Append(observer); err != nil {
		return nil, err
	}
	observers[eventName] = obs

	return starlark.None, nil
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

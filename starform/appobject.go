package starform

import (
	"fmt"
	"sort"
	"strings"

	"github.com/canonical/starlark/starlark"
)

type ThreadAppObject interface {
	SafeAttr(thread *starlark.Thread, name string) (starlark.Value, error)
}

// An appObject is the common point for exposing the state of the application
// into Starlark and for Starlark to declare intents.
type appObject struct {
	name, typ string
	attrNames []string
}

func NewAppObject(name, typ string, attrNames []string) starlark.Value {
	sort.Strings(attrNames)

	return &appObject{
		name:      name,
		typ:       typ,
		attrNames: attrNames,
	}
}

var _ starlark.Value = &appObject{}
var _ starlark.SafeStringer = &appObject{}
var _ starlark.HasSafeAttrs = &appObject{}

func (app *appObject) String() string       { return app.name }
func (app *appObject) Type() string         { return app.typ }
func (app *appObject) Freeze()              {}
func (app *appObject) Truth() starlark.Bool { return true }
func (app *appObject) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", app.Type())
}
func (app *appObject) SafeString(thread *starlark.Thread, sb starlark.StringBuilder) error {
	const safety = starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe | starlark.TimeSafe
	if err := starlark.CheckSafety(thread, safety); err != nil {
		return err
	}

	_, err := sb.WriteString(app.String())
	return err
}

func (app *appObject) AttrNames() []string {
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
		// FIXME(marco6): check if it's the right args order
		return strings.Compare(haystack[i], needle)
	})
	return found
}

func (app *appObject) Attr(name string) (starlark.Value, error) {
	return app.SafeAttr(nil, name)
}

var errUnavailable = fmt.Errorf("unavailable") // FIXME(marco6): better error message

func (app *appObject) SafeAttr(thread *starlark.Thread, name string) (starlark.Value, error) {
	if thread == nil {
		return nil, fmt.Errorf("can't access %s.%s in unconstrained environment", app.name, name)
	}
	const safety = starlark.MemSafe | starlark.CPUSafe | starlark.IOSafe | starlark.TimeSafe
	if err := starlark.CheckSafety(thread, safety); err != nil {
		return nil, err
	}

	rundata := RunData(thread)
	if rundata.EventName == LoadEventName {
		// If it is a user attr => unavailable
		if hasAttr(app.attrNames, name) {
			return nil, errUnavailable
		}
		b := appObjectMethods[name]
		if b == nil {
			return nil, starlark.ErrNoSuchAttr
		}
		if rundata.observers == nil {
			return nil, errUnavailable
		}
		if err := thread.AddAllocs(starlark.EstimateSize(&starlark.Builtin{})); err != nil {
			return nil, err
		}
		return b.BindReceiver(app), nil
	}

	attr, err := rundata.ThreadAppObject.SafeAttr(thread, name)
	if isNoSuchAttr(err) {
		if _, ok := appObjectMethods[name]; ok {
			return nil, errUnavailable
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
	obs, ok := data.observers[eventName]
	if !ok {
		newSize := starlark.EstimateMakeSize(map[string][]starlark.Callable{}, 1+len(data.observers))
		oldSize := starlark.EstimateMakeSize(map[string][]starlark.Callable{}, len(data.observers))
		if err := thread.AddAllocs(newSize, -oldSize); err != nil {
			return nil, err
		}
	}
	safeAppender := starlark.NewSafeAppender(thread, &obs)
	if err := safeAppender.Append(observer); err != nil {
		return nil, err
	}
	data.observers[eventName] = obs

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

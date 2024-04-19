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

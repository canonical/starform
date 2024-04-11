package starform

import (
	"fmt"

	"github.com/canonical/starlark/starlark"
)

// AppObject is the common point for exposing the current state of the
// application into Starlark and for Starlark to declare intents.
type AppObject struct {
	name string
}

func NewAppObject(name string) *AppObject {
	return &AppObject{name: name}
}

var _ starlark.Value = &AppObject{}

func (ao *AppObject) String() string       { return ao.name }
func (ao *AppObject) Type() string         { return ao.name }
func (ao *AppObject) Freeze()              {}
func (ao *AppObject) Truth() starlark.Bool { return true }
func (ao *AppObject) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", ao.Type())
}

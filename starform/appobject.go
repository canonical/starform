package starform

import (
	"github.com/canonical/starform/internal/lib"
)

// AppObject is the common point for exposing the state of the application
// into Starlark and for Starlark to declare intents.
type AppObject = lib.AppObject

var ErrUnavailable = lib.ErrUnavailable

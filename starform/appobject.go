package starform

import (
	"github.com/canonical/starform/internal"
)

// AppObject is the common point for exposing the state of the application
// into Starlark and for Starlark to declare intents.
type AppObject = internal.AppObject

var ErrUnavailable = internal.ErrUnavailable

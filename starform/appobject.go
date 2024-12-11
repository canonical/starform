package starform

import "github.com/canonical/starform/internal/userdata"

// AppObject is the common point for exposing the state of the application
// into Starlark and for Starlark to declare intents.
type AppObject = userdata.AppObject

var ErrUnavailable = userdata.ErrUnavailable

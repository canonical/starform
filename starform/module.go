package starform

import (
	"github.com/canonical/starform/internal"
	"github.com/canonical/starlark/lib/json"
	"github.com/canonical/starlark/lib/time"
	"github.com/canonical/starlark/starlark"
)

type Module interface {
	Name() string
	Members() starlark.StringDict
}

var JsonModule Module = &internal.SystemModule{Module: json.Module}
var TimeModule Module = &internal.SystemModule{Module: time.Module}

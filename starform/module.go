package starform

import (
	"github.com/canonical/starform/internal/lib"
	"github.com/canonical/starlark/lib/json"
	"github.com/canonical/starlark/lib/time"
	"github.com/canonical/starlark/starlark"
)

type Module interface {
	Name() string
	Members() starlark.StringDict
}

var JsonModule = Module(&lib.SystemModule{Module: json.Module})
var TimeModule = Module(&lib.SystemModule{Module: time.Module})

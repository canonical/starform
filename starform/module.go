package starform

import (
	"github.com/canonical/starform/internal/module"
	"github.com/canonical/starlark/lib/json"
	"github.com/canonical/starlark/lib/time"
	"github.com/canonical/starlark/starlark"
)

type Module interface {
	Name() string
	Members() starlark.StringDict
}

var JsonModule = Module(&module.SystemModule{Module: json.Module})
var TimeModule = Module(&module.SystemModule{Module: time.Module})

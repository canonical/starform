package module

import (
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/starlarkstruct"
)

type SystemModule struct {
	Module      *starlarkstruct.Module
	Predeclared bool
}

func (m *SystemModule) Name() string                 { return m.Module.Name }
func (m *SystemModule) Members() starlark.StringDict { return m.Module.Members }

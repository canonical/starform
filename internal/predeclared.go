package internal

import (
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/starlarkstruct"
)

type Predeclared struct {
	Module *starlarkstruct.Module
}

func NewPredeclared(module *starlarkstruct.Module) *Predeclared {
	return &Predeclared{
		Module: module,
	}
}

func (p *Predeclared) Name() string                 { return p.Module.Name }
func (p *Predeclared) Members() starlark.StringDict { return p.Module.Members }

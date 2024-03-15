package starform

import (
	"crypto/sha256"

	"github.com/canonical/starlark/starlark"
)

type ProgramKey [sha256.Size]byte

type ScriptCache interface {
	GetProgram(key ProgramKey, load func() (*starlark.Program, error)) (*starlark.Program, error)

	private() // This will be removed once this interface is stable.
}

type noopScriptCache struct{}

var _ ScriptCache = &noopScriptCache{}

func (n *noopScriptCache) GetProgram(key ProgramKey, load func() (*starlark.Program, error)) (*starlark.Program, error) {
	return load()
}

func (*noopScriptCache) private() {}

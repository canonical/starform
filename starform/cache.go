package starform

import (
	"crypto/sha256"

	"github.com/canonical/starlark/starlark"
)

type ProgramKey [sha256.Size]byte

type ScriptCache interface {
	GetProgram(key ProgramKey) *starlark.Program
	CacheProgram(key ProgramKey, prog *starlark.Program)

	private() // This will be removed once this interface is stable.
}

type noopScriptCache struct{}

var _ ScriptCache = &noopScriptCache{}

func (n *noopScriptCache) GetProgram(key ProgramKey) *starlark.Program {
	return nil
}

func (n *noopScriptCache) CacheProgram(key ProgramKey, prog *starlark.Program) {}

func (*noopScriptCache) private() {}

package starform

import (
	"crypto/sha256"
	"errors"

	"github.com/canonical/starlark/starlark"
)

var ErrNotInCache error = errors.New("key not found")

type ProgramKey [sha256.Size]byte

type ScriptCache interface {
	GetProgram(key ProgramKey) (*starlark.Program, error)
	CacheProgram(key ProgramKey, prog *starlark.Program)

	private() // This will be removed once this interface is stable.
}

type noopScriptCache struct{}

var _ ScriptCache = &noopScriptCache{}

func (n *noopScriptCache) GetProgram(key ProgramKey) (*starlark.Program, error) {
	return nil, ErrNotInCache
}

func (n *noopScriptCache) CacheProgram(key ProgramKey, prog *starlark.Program) {}

func (n *noopScriptCache) private() {}

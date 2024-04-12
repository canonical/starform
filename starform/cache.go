package starform

import (
	"errors"
)

var ErrNotCached error = errors.New("not cached")

// ScriptCache is a cache for use with ScriptSet values. All methods are
// required to be thread-safe.
type ScriptCache interface {
	private() // This will be removed once this interface is stable.

	// Get returns the CacheValue for key, if present. Otherwise, it returns ErrNotCached.
	Get(key interface{}) (interface{}, error)

	// Put adds the given key and value to the cache.
	Put(key, value interface{}, source ScriptSource) error

	// Drop removes the value associated with key.
	Drop(key interface{})

	// Len returns the current number of entries in the cache.
	Len() int

	// Visit calls f for every key-value pair in the cache. The visiting order is unspecfied.
	// If f returns an error, the iteration stops and Visit returns the given error.
	Visit(f func(key, value interface{}) error) error
}

type noopScriptCache struct{}

var _ ScriptCache = &noopScriptCache{}

func (n *noopScriptCache) private()                                              {}
func (n *noopScriptCache) Get(key interface{}) (interface{}, error)              { return nil, ErrNotCached }
func (n *noopScriptCache) Put(key, value interface{}, source ScriptSource) error { return nil }
func (n *noopScriptCache) Drop(key interface{})                                  {}
func (n *noopScriptCache) Len() int                                              { return 0 }
func (n *noopScriptCache) Visit(f func(key, value interface{}) error) error      { return nil }

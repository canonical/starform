package starform

import (
	"errors"
)

var ErrNoCache error = errors.New("not cached")

type CacheKey interface{}
type CacheValue interface{}

// ScriptCache interface lists the requirements for a ScriptSet cache.
// All methods of this interface are expected to work when called from
// more than one goroutine.
type ScriptCache interface {
	// Get returns the CacheValue for key, if present. Otherwise, it returns ErrNoCache.
	Get(key CacheKey) (CacheValue, error)
	// Put inserts the value associated with key in the cache, following the cache
	// retention policies. The last parameter is the source which generated this cache
	// entry.
	Put(key CacheKey, value CacheValue, source ScriptSource) error
	// Drop removes the value associated with key.
	Drop(key CacheKey)
	// Len returns the current number of keys in the cache.
	Len() int
	// Visit calls f for every key-value pair in the cache. The visitation order is unspecfied.
	// Returning false from f stops the iteration.
	Visit(f func(key CacheKey, value CacheValue) bool)
}

type noopScriptCache struct{}

var _ ScriptCache = &noopScriptCache{}

func (n *noopScriptCache) Get(key CacheKey) (CacheValue, error)                          { return nil, ErrNoCache }
func (n *noopScriptCache) Put(key CacheKey, value CacheValue, source ScriptSource) error { return nil }
func (n *noopScriptCache) Drop(key CacheKey)                                             {}
func (n *noopScriptCache) Len() int                                                      { return 0 }
func (n *noopScriptCache) Visit(f func(key CacheKey, value CacheValue) bool)             {}

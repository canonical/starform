package starform

import (
	"errors"
	"sync"
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

type lruEntry struct {
	key, value interface{}
	next, prev *lruEntry
}

func (le *lruEntry) removeFrom(list **lruEntry) {
	le.prev.next = le.next
	le.next.prev = le.prev
	if le == *list {
		if le.next == le {
			*list = nil
		} else {
			*list = le.next
		}
	}
}

func (le *lruEntry) addTo(list **lruEntry) {
	if *list != nil {
		le.prev = (*list).prev
		le.next = *list
		(*list).prev.next = le
		(*list).prev = le
	}
	*list = le
}

type LRUCache struct {
	mu         sync.Mutex
	maxSize    int
	storage    map[interface{}]*lruEntry
	mostRecent *lruEntry
}

var _ ScriptCache = &LRUCache{}

func NewLruCache(maxSize int) *LRUCache {
	return &LRUCache{maxSize: maxSize}
}

func (lc *LRUCache) private() {}

func (lc *LRUCache) touch(entry *lruEntry) {
	entry.removeFrom(&lc.mostRecent)
	entry.addTo(&lc.mostRecent)
}

func (lc *LRUCache) Get(key interface{}) (interface{}, error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if entry, ok := lc.storage[key]; ok {
		lc.touch(entry)
		return entry.value, nil
	}
	return nil, ErrNotCached
}

func (lc *LRUCache) Put(key, value interface{}, source ScriptSource) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	entry, ok := lc.storage[key]
	if ok {
		entry.value = value
	} else {
		if lc.storage == nil {
			lc.storage = make(map[interface{}]*lruEntry)
		}
		entry = &lruEntry{
			key:   key,
			value: value,
		}
		entry.next = entry
		entry.prev = entry
		if len(lc.storage) >= lc.maxSize {
			lc.dropEntry(lc.mostRecent.prev)
		}
		lc.storage[key] = entry
	}
	lc.touch(entry)

	return nil
}

func (lc *LRUCache) dropEntry(entry *lruEntry) {
	entry.removeFrom(&lc.mostRecent)
	delete(lc.storage, entry.key)
}

func (lc *LRUCache) Drop(key interface{}) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if entry, ok := lc.storage[key]; ok {
		lc.dropEntry(entry)
	}
}

func (lc *LRUCache) Len() int {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	return len(lc.storage)
}

func (lc *LRUCache) Visit(f func(key, value interface{}) error) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	for k, v := range lc.storage {
		lc.mu.Unlock()
		err := f(k, v)
		lc.mu.Lock()
		if err != nil {
			return err
		}
	}

	return nil
}

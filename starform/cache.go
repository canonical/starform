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

func (n *noopScriptCache) private()                                              {} // TODO: remove once interface is stable
func (n *noopScriptCache) Get(key interface{}) (interface{}, error)              { return nil, ErrNotCached }
func (n *noopScriptCache) Put(key, value interface{}, source ScriptSource) error { return nil }
func (n *noopScriptCache) Drop(key interface{})                                  {}
func (n *noopScriptCache) Len() int                                              { return 0 }
func (n *noopScriptCache) Visit(f func(key, value interface{}) error) error      { return nil }

type LRUCache struct {
	mu      sync.Mutex
	maxSize int
	store   map[interface{}]*lruEntry
	lruList lruList
}

var _ ScriptCache = &LRUCache{}

func NewLruCache(maxSize int) *LRUCache {
	return &LRUCache{
		maxSize: maxSize,
		store:   make(map[interface{}]*lruEntry, maxSize),
	}
}

func (lc *LRUCache) private() {} // TODO: remove once interface is stable

func (lc *LRUCache) touch(entry *lruEntry) {
	lc.lruList.remove(entry)
	lc.lruList.add(entry)
}

func (lc *LRUCache) Get(key interface{}) (interface{}, error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if entry, ok := lc.store[key]; ok {
		lc.touch(entry)
		return entry.value, nil
	}
	return nil, ErrNotCached
}

func (lc *LRUCache) Put(key, value interface{}, source ScriptSource) error {
	if lc.maxSize <= 0 {
		return nil
	}

	lc.mu.Lock()
	defer lc.mu.Unlock()

	entry, ok := lc.store[key]
	if ok {
		entry.value = value
		lc.touch(entry)
		return nil
	}

	if len(lc.store) >= lc.maxSize {
		lc.drop(lc.lruList.tail())
	}

	entry = &lruEntry{
		key:   key,
		value: value,
	}
	lc.store[key] = entry
	lc.lruList.add(entry)
	return nil
}

func (lc *LRUCache) Drop(key interface{}) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if entry, ok := lc.store[key]; ok {
		lc.drop(entry)
	}
}

func (lc *LRUCache) drop(entry *lruEntry) {
	lc.lruList.remove(entry)
	delete(lc.store, entry.key)
}

func (lc *LRUCache) Len() int {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	return len(lc.store)
}

func (lc *LRUCache) Visit(f func(key, value interface{}) error) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	for k, v := range lc.store {
		lc.mu.Unlock()
		err := f(k, v)
		lc.mu.Lock()
		if err != nil {
			return err
		}
	}

	return nil
}

type lruList struct {
	// lruList is implemented as a circular linked list
	head *lruEntry
}

type lruEntry struct {
	key, value interface{}
	next, prev *lruEntry
}

func (cll *lruList) remove(entry *lruEntry) {
	entry.prev.next = entry.next
	entry.next.prev = entry.prev
	if entry != cll.head {
		return
	}
	if entry.next == entry {
		cll.head = nil
	} else {
		cll.head = entry.next
	}
}

func (cll *lruList) add(entry *lruEntry) {
	if cll.head != nil {
		entry.prev = cll.head.prev
		entry.next = cll.head
		cll.head.prev.next = entry
		cll.head.prev = entry
	} else {
		entry.prev = entry
		entry.next = entry
	}
	cll.head = entry
}

func (cll *lruList) tail() *lruEntry {
	if cll.head == nil {
		return nil
	}
	return cll.head.prev
}

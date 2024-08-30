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

type DefaultCache struct {
	mu      sync.Mutex
	maxSize int
	store   map[interface{}]*lruEntry
	lruList lruList
}

var _ ScriptCache = &DefaultCache{}

type DefaultCacheOptions struct {
	MaxSize int
}

func NewDefaultCache(options *DefaultCacheOptions) (*DefaultCache, error) {
	return &DefaultCache{
		maxSize: options.MaxSize,
		store:   make(map[interface{}]*lruEntry, options.MaxSize),
	}, nil
}

func (lc *DefaultCache) private() {} // TODO: remove once interface is stable

func (lc *DefaultCache) touch(entry *lruEntry) {
	lc.lruList.remove(entry)
	lc.lruList.add(entry)
}

func (lc *DefaultCache) Get(key interface{}) (interface{}, error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if entry, ok := lc.store[key]; ok {
		lc.touch(entry)
		return entry.value, nil
	}
	return nil, ErrNotCached
}

func (lc *DefaultCache) Put(key, value interface{}, source ScriptSource) error {
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

func (lc *DefaultCache) Drop(key interface{}) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if entry, ok := lc.store[key]; ok {
		lc.drop(entry)
	}
}

func (lc *DefaultCache) drop(entry *lruEntry) {
	lc.lruList.remove(entry)
	delete(lc.store, entry.key)
}

func (lc *DefaultCache) Len() int {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	return len(lc.store)
}

func (lc *DefaultCache) Visit(f func(key, value interface{}) error) error {
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
	// Adjacent entries, unless at the tail of the list, the
	// next entry was added less recently tham the current.
	// Likewise prev was added more recently.
	next, prev *lruEntry
}

func (ll *lruList) remove(entry *lruEntry) {
	entry.prev.next = entry.next
	entry.next.prev = entry.prev
	if entry != ll.head {
		return
	}
	if entry.next == entry {
		ll.head = nil
	} else {
		ll.head = entry.next
	}
}

func (ll *lruList) add(entry *lruEntry) {
	if ll.head != nil {
		entry.prev = ll.head.prev
		entry.next = ll.head
		ll.head.prev.next = entry
		ll.head.prev = entry
	} else {
		entry.prev = entry
		entry.next = entry
	}
	ll.head = entry
}

func (ll *lruList) tail() *lruEntry {
	if ll.head == nil {
		return nil
	}
	return ll.head.prev
}

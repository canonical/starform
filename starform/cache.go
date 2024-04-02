package starform

import (
	"errors"
)

var ErrNoCache error = errors.New("key not found")

type ScriptCache interface {
	Get(key interface{}) (interface{}, error)
	Put(key, value interface{}, source ScriptSource) error
	Drop(key interface{})
	Len() int
	Visit(f func(key, value interface{}) bool)
}

type noopScriptCache struct{}

var _ ScriptCache = &noopScriptCache{}

func (n *noopScriptCache) Get(key interface{}) (interface{}, error) {
	return nil, ErrNoCache
}

func (n *noopScriptCache) Put(key, value interface{}, source ScriptSource) error {
	return nil
}

func (n *noopScriptCache) Drop(key interface{}) {}
func (n *noopScriptCache) Len() int             { return 0 }

func (n *noopScriptCache) Visit(f func(key interface{}, value interface{}) bool) {}

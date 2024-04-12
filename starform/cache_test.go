package starform_test

import (
	"testing"

	"github.com/canonical/starform/starform"
)

func TestLRUCache(t *testing.T) {
	t.Run("single-entry", func(t *testing.T) {
		t.Run("replace-key", func(t *testing.T) {
			cache := starform.NewLruCache(1)
			cache.Put(1, "one", nil)
			cache.Put(2, "two", nil)

			if cache.Len() != 1 {
				t.Errorf("unexpected cache length: want 1 got %d", cache.Len())
			}
			if _, err := cache.Get(1); err == nil {
				t.Errorf("unexpected cache hit")
			}
			if value, err := cache.Get(2); err != nil {
				t.Errorf("unexpected cache miss")
			} else if value != "two" {
				t.Errorf("unexpected value for entry")
			}
		})

		t.Run("replace-value", func(t *testing.T) {
			cache := starform.NewLruCache(1)
			cache.Put(2, "two-old", nil)
			cache.Put(2, "two", nil)

			if cache.Len() != 1 {
				t.Errorf("unexpected cache length: want 1 got %d", cache.Len())
			}
			if _, err := cache.Get(1); err == nil {
				t.Errorf("unexpected cache hit")
			}
			if value, err := cache.Get(2); err != nil {
				t.Errorf("unexpected cache miss")
			} else if value != "two" {
				t.Errorf("unexpected value for entry")
			}
		})

		t.Run("drop-key", func(t *testing.T) {
			cache := starform.NewLruCache(1)
			cache.Put(2, "two", nil)
			cache.Drop(2)

			if cache.Len() != 0 {
				t.Errorf("unexpected cache length: want 0 got %d", cache.Len())
			}
			if _, err := cache.Get(2); err == nil {
				t.Errorf("unexpected cache hit")
			}
		})
	})

	t.Run("many-entries", func(t *testing.T) {
		const entryNum = 100

		t.Run("replace-keys", func(t *testing.T) {
			cache := starform.NewLruCache(entryNum)
			for i := 0; i < entryNum*2; i++ {
				cache.Put(i, i, nil)
			}

			if cache.Len() != entryNum {
				t.Errorf("unexpected cache length: want %d got %d", entryNum, cache.Len())
			}
			for i := 0; i < entryNum*2; i++ {
				if i < entryNum {
					if _, err := cache.Get(i); err == nil {
						t.Errorf("unexpected cache hit: %d", i)
					}
				} else {
					if value, err := cache.Get(i); err != nil {
						t.Errorf("unexpected cache miss: %d", i)
					} else if value != i {
						t.Errorf("unexpected value for entry")
					}
				}
			}
		})

		t.Run("used-entries", func(t *testing.T) {
			cache := starform.NewLruCache(entryNum)
			for i := 0; i < entryNum; i++ {
				cache.Put(i, i, nil)
			}
			for i := 0; i < entryNum; i += 10 {
				cache.Get(i)
			}
			for i := 1; i <= (entryNum - entryNum/10); i++ {
				cache.Put(-i, -i, nil)
			}

			if cache.Len() != entryNum {
				t.Errorf("unexpected cache length: want %d got %d", entryNum, cache.Len())
			}
			for i := 0; i < entryNum; i += 1 {
				if i%10 == 0 {
					if value, err := cache.Get(i); err != nil {
						t.Errorf("unexpected cache miss: %d", i)
					} else if value != i {
						t.Errorf("unexpected value for entry")
					}
				} else if _, err := cache.Get(i); err == nil {
					t.Errorf("unexpected cache hit: %d", i)
				}
			}
		})

		t.Run("dropped-keys", func(t *testing.T) {
			cache := starform.NewLruCache(entryNum)
			for i := 0; i < entryNum; i++ {
				cache.Put(i, i, nil)
			}
			for i := 0; i < entryNum; i++ {
				cache.Drop(entryNum/2 + i)
			}
			if cache.Len() != entryNum/2 {
				t.Errorf("unexpected cache length: want %d got %d", entryNum/2, cache.Len())
			}
			for i := 0; i < entryNum; i += 1 {
				if i < entryNum/2 {
					if value, err := cache.Get(i); err != nil {
						t.Errorf("unexpected cache miss: %d", i)
					} else if value != i {
						t.Errorf("unexpected value for entry")
					}
				} else if _, err := cache.Get(i); err == nil {
					t.Errorf("unexpected cache hit: %d", i)
				}
			}
		})
	})
}

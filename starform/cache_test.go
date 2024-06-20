package starform_test

import (
	"testing"

	"github.com/canonical/starform/starform"
)

func TestDefaultCache(t *testing.T) {
	t.Run("max-size-one", func(t *testing.T) {
		t.Run("replace-key", func(t *testing.T) {
			cache := starform.NewDefaultCache(1)
			cache.Put(1, "one", &testScriptSource{})
			cache.Put(2, "two", &testScriptSource{}) // Expect eviction to occur here.

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
			cache := starform.NewDefaultCache(1)
			cache.Put(2, "two-old", &testScriptSource{})
			cache.Put(2, "two", &testScriptSource{})

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
			cache := starform.NewDefaultCache(1)
			cache.Put(2, "two", &testScriptSource{})
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
		const maxEntries = 100

		t.Run("replace-keys", func(t *testing.T) {
			cache := starform.NewDefaultCache(maxEntries)
			for i := 0; i < maxEntries*2; i++ {
				cache.Put(i, i, &testScriptSource{})
			}

			if cache.Len() != maxEntries {
				t.Errorf("unexpected cache length: want %d got %d", maxEntries, cache.Len())
			}
			for i := 0; i < maxEntries; i++ {
				if _, err := cache.Get(i); err == nil {
					t.Errorf("unexpected cache hit: %d", i)
				}
			}
			for i := maxEntries; i < maxEntries*2; i++ {
				if value, err := cache.Get(i); err != nil {
					t.Errorf("unexpected cache miss: %d", i)
				} else if value != i {
					t.Errorf("unexpected value for entry")
				}
			}
		})

		t.Run("used-entries", func(t *testing.T) {
			cache := starform.NewDefaultCache(maxEntries)
			for i := 0; i < maxEntries; i++ {
				cache.Put(i, i, &testScriptSource{})
			}
			// Make sure multiples of 10 have been recently used.
			for i := 0; i < maxEntries; i += 10 {
				cache.Get(i)
			}
			// Add enough elements to flush everything except the recently used ones.
			for i := 1; i <= (maxEntries - maxEntries/10); i++ {
				cache.Put(-i, -i, &testScriptSource{})
			}

			if cache.Len() != maxEntries {
				t.Errorf("unexpected cache length: want %d got %d", maxEntries, cache.Len())
			}
			for i := 0; i < maxEntries; i += 1 {
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
			cache := starform.NewDefaultCache(maxEntries)
			for i := 0; i < maxEntries; i++ {
				cache.Put(i, i, &testScriptSource{})
			}
			for i := 0; i < maxEntries; i++ {
				cache.Drop(maxEntries/2 + i)
			}
			if cache.Len() != maxEntries/2 {
				t.Errorf("unexpected cache length: want %d got %d", maxEntries/2, cache.Len())
			}
			for i := 0; i < maxEntries; i += 1 {
				if i < maxEntries/2 {
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

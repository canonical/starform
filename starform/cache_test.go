package starform_test

import (
	"testing"

	"github.com/canonical/starform/starform"
)

func TestDefaultCache(t *testing.T) {
	t.Run("max-size-one", func(t *testing.T) {
		t.Run("replace-key", func(t *testing.T) {
			cache, err := starform.NewDefaultCache(&starform.DefaultCacheOptions{
				MaxSize: 1,
			})
			if err != nil {
				t.Fatal(err)
			}

			if err := cache.Put(1, "one", &testScriptSource{}); err != nil {
				t.Error(err)
			}

			// Expect eviction to occur here.
			if err := cache.Put(2, "two", &testScriptSource{}); err != nil {
				t.Error(err)
			}

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
			cache, err := starform.NewDefaultCache(&starform.DefaultCacheOptions{
				MaxSize: 1,
			})
			if err != nil {
				t.Fatal(err)
			}

			if err := cache.Put(2, "two-old", &testScriptSource{}); err != nil {
				t.Error(err)
			}
			if err := cache.Put(2, "two", &testScriptSource{}); err != nil {
				t.Error(err)
			}

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
			cache, err := starform.NewDefaultCache(&starform.DefaultCacheOptions{
				MaxSize: 1,
			})
			if err != nil {
				t.Fatal(err)
			}

			if err := cache.Put(2, "two", &testScriptSource{}); err != nil {
				t.Error(err)
			}
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
		const maxCacheSize = 100

		t.Run("replace-keys", func(t *testing.T) {
			cache, err := starform.NewDefaultCache(&starform.DefaultCacheOptions{
				MaxSize: maxCacheSize,
			})
			if err != nil {
				t.Fatal(err)
			}

			for i := 0; i < maxCacheSize*2; i++ {
				if err := cache.Put(i, i, &testScriptSource{}); err != nil {
					t.Fatal(err)
				}
			}

			if cache.Len() != maxCacheSize {
				t.Errorf("unexpected cache length: want %d got %d", maxCacheSize, cache.Len())
			}
			for i := 0; i < maxCacheSize; i++ {
				if _, err := cache.Get(i); err == nil {
					t.Errorf("unexpected cache hit: %d", i)
				}
			}
			for i := maxCacheSize; i < maxCacheSize*2; i++ {
				if value, err := cache.Get(i); err != nil {
					t.Errorf("unexpected cache miss: %d", i)
				} else if value != i {
					t.Errorf("unexpected value for entry")
				}
			}
		})

		t.Run("used-entries", func(t *testing.T) {
			cache, err := starform.NewDefaultCache(&starform.DefaultCacheOptions{
				MaxSize: maxCacheSize,
			})
			if err != nil {
				t.Fatal(err)
			}

			for i := 0; i < maxCacheSize; i++ {
				if err := cache.Put(i, i, &testScriptSource{}); err != nil {
					t.Fatal(err)
				}
			}
			// Make sure multiples of 10 have been recently used.
			for i := 0; i < maxCacheSize; i += 10 {
				if _, err := cache.Get(i); err != nil {
					t.Fatal(err)
				}
			}
			// Add enough elements to flush everything except the recently used ones.
			for i := 1; i <= (maxCacheSize - maxCacheSize/10); i++ {
				if err := cache.Put(-i, -i, &testScriptSource{}); err != nil {
					t.Error(err)
				}
			}

			if cache.Len() != maxCacheSize {
				t.Errorf("unexpected cache length: want %d got %d", maxCacheSize, cache.Len())
			}
			for i := 0; i < maxCacheSize; i += 1 {
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
			cache, err := starform.NewDefaultCache(&starform.DefaultCacheOptions{
				MaxSize: maxCacheSize,
			})
			if err != nil {
				t.Fatal(err)
			}

			for i := 0; i < maxCacheSize; i++ {
				if err := cache.Put(i, i, &testScriptSource{}); err != nil {
					t.Error(err)
				}
			}
			for i := 0; i < maxCacheSize; i++ {
				cache.Drop(maxCacheSize/2 + i)
			}
			if cache.Len() != maxCacheSize/2 {
				t.Errorf("unexpected cache length: want %d got %d", maxCacheSize/2, cache.Len())
			}
			for i := 0; i < maxCacheSize; i += 1 {
				if i < maxCacheSize/2 {
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

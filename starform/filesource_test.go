package starform_test

import (
	"bytes"
	"context"
	"testing"
	"testing/fstest"

	"github.com/canonical/starform/starform"
)

func TestLoadDir(t *testing.T) {
	fs := fstest.MapFS{
		"aaa/foo.star": &fstest.MapFile{},
		"aaa/bar.star": &fstest.MapFile{},
		"aaa/very_big.star": &fstest.MapFile{
			Data: bytes.Repeat([]byte{' '}, 1024*1024),
		},

		"bbb/wrong_extension.foo": &fstest.MapFile{},
		"bbb/.hidden.star":        &fstest.MapFile{},
		"bbb/malformed-name.star": &fstest.MapFile{},
	}

	t.Run("well-formed", func(t *testing.T) {
		sources, err := starform.LoadDirSources(context.Background(), &starform.LoadDirSourcesOptions{
			Fs:   fs,
			Root: "aaa",
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(sources) != 3 {
			t.Errorf("expected 3 sources, got %d", len(sources))
		}
	})

	t.Run("misnamed", func(t *testing.T) {
		sources, err := starform.LoadDirSources(context.Background(), &starform.LoadDirSourcesOptions{
			Fs:   fs,
			Root: "bbb",
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(sources) != 0 {
			t.Errorf("expected 0 sources, got %d", len(sources))
		}
	})

	t.Run("mixed", func(t *testing.T) {
		sources, err := starform.LoadDirSources(context.Background(), &starform.LoadDirSourcesOptions{
			Fs: fs,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(sources) != 3 {
			t.Errorf("expected 3 sources, got %d", len(sources))
		}
	})
}

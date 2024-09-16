package starform

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"strings"
)

type fileSource struct {
	fsys fs.FS
	path string
}

func (f *fileSource) Path() string { return f.path }
func (f *fileSource) Content(ctx context.Context) ([]byte, error) {
	file, err := f.fsys.Open(f.path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cancel := afterFunc(ctx, func() { file.Close() })
	defer cancel()

	return io.ReadAll(file)
}

type LoadDirSourcesOptions struct {
	Fs          fs.FS
	Root        string
	MaxFileSize int64
}

// LoadDirSources returns the valid script sources found in the
// given file system under the given root. Individual files are
// not accessed, directory access errors are ignored.
func LoadDirSources(ctx context.Context, options *LoadDirSourcesOptions) ([]ScriptSource, error) {
	sources := []ScriptSource{}
	root := options.Root
	if root == "" {
		root = "."
	}
	err := fs.WalkDir(options.Fs, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if !strings.HasSuffix(path, ".star") {
			return nil
		}
		if options.MaxFileSize != 0 {
			if info, err := d.Info(); err != nil {
				return nil
			} else if size := info.Size(); size > options.MaxFileSize {
				return fmt.Errorf("cannot load %s: size limit exceeded (%d > %d)", path, size, options.MaxFileSize)
			}
		}

		if source, err := newFileSource(options.Fs, path); err != nil {
			return nil
		} else {
			sources = append(sources, source)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return sources, nil
}

// newFileSource creates a ScriptSource for the file pointed by
// path in the filesystem fsys. The filesystem is not
// accessed until ScriptSource.Content is called.
func newFileSource(fsys fs.FS, path string) (ScriptSource, error) {
	if err := checkLoadPath(path); err != nil {
		return nil, err
	}
	return &fileSource{
		fsys: fsys,
		path: path,
	}, nil
}

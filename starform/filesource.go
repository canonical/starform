package starform

import (
	"context"
	"fmt"
	"io"
	"io/fs"
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
// not accessed.
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
		if _, err := sanitiseLoadPath(".", path); err != nil {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if options.MaxFileSize != 0 {
			if info, err := d.Info(); err != nil {
				return err
			} else if size := info.Size(); size > options.MaxFileSize {
				return fmt.Errorf("cannot load %s: size limit exceeded (%d > %d)", path, size, options.MaxFileSize)
			}
		}

		sources = append(sources, &fileSource{
			fsys: options.Fs,
			path: path,
		})

		return nil
	})
	if err != nil {
		return nil, err
	}
	return sources, nil
}

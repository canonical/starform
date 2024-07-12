package starform

import (
	"context"
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

// NewFileSource creates a ScriptSource for the file pointed by
// path in the filesystem fsys. The filesystem is not
// accessed untile ScriptSource.Content is called.
func NewFileSource(fsys fs.FS, path string) ScriptSource {
	return &fileSource{
		fsys: fsys,
		path: path,
	}
}

// LoadDirSources walks the file tree rooted at root
// and returns a slice of ScriptSources containing all
// .star files that can be loaded.
// If a folder cannot be accessed, the error is ignored.
func LoadDirSources(ctx context.Context, fsys fs.FS, root string) ([]ScriptSource, error) {
	sources := []ScriptSource{}
	err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if !strings.HasSuffix(path, ".star") {
			return nil
		}
		if checkLoadPath(path) != nil {
			return nil
		}

		sources = append(sources, NewFileSource(fsys, path))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return sources, nil
}

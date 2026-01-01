package gwt

import (
	"io/fs"
	"os"

	"github.com/bmatcuk/doublestar/v4"
)

// FileSystem abstracts filesystem operations for testability.
type FileSystem interface {
	Stat(name string) (fs.FileInfo, error)
	Symlink(oldname, newname string) error
	IsNotExist(err error) bool
	Glob(dir, pattern string) ([]string, error)
	MkdirAll(path string, perm fs.FileMode) error
}

type osFS struct{}

func (osFS) Stat(name string) (fs.FileInfo, error) { return os.Stat(name) }
func (osFS) Symlink(oldname, newname string) error { return os.Symlink(oldname, newname) }
func (osFS) IsNotExist(err error) bool             { return os.IsNotExist(err) }
func (osFS) Glob(dir, pattern string) ([]string, error) {
	return doublestar.Glob(os.DirFS(dir), pattern)
}
func (osFS) MkdirAll(path string, perm fs.FileMode) error { return os.MkdirAll(path, perm) }

package twig

import (
	"io/fs"
	"os"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// isPathWithin reports whether path is within or equal to basePath.
// It properly handles path separators to avoid false positives like
// "/path/to/repo-worktree" being considered within "/path/to/repo".
func isPathWithin(path, basePath string) bool {
	if path == basePath {
		return true
	}
	return strings.HasPrefix(path, basePath+string(os.PathSeparator))
}

// FileSystem abstracts filesystem operations for testability.
type FileSystem interface {
	Stat(name string) (fs.FileInfo, error)
	Lstat(name string) (fs.FileInfo, error)
	Symlink(oldname, newname string) error
	IsNotExist(err error) bool
	Glob(dir, pattern string) ([]string, error)
	MkdirAll(path string, perm fs.FileMode) error
	ReadDir(name string) ([]os.DirEntry, error)
	Remove(name string) error
	WriteFile(name string, data []byte, perm fs.FileMode) error
}

type osFS struct{}

func (osFS) Stat(name string) (fs.FileInfo, error)  { return os.Stat(name) }
func (osFS) Lstat(name string) (fs.FileInfo, error) { return os.Lstat(name) }
func (osFS) Symlink(oldname, newname string) error  { return os.Symlink(oldname, newname) }
func (osFS) IsNotExist(err error) bool              { return os.IsNotExist(err) }
func (osFS) Glob(dir, pattern string) ([]string, error) {
	return doublestar.Glob(os.DirFS(dir), pattern)
}
func (osFS) MkdirAll(path string, perm fs.FileMode) error { return os.MkdirAll(path, perm) }
func (osFS) ReadDir(name string) ([]os.DirEntry, error)   { return os.ReadDir(name) }
func (osFS) Remove(name string) error                     { return os.Remove(name) }
func (osFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}

package gwt

import (
	"io/fs"
	"os"
)

// FileSystem abstracts filesystem operations for testability.
type FileSystem interface {
	Getwd() (string, error)
	Stat(name string) (fs.FileInfo, error)
	Symlink(oldname, newname string) error
	IsNotExist(err error) bool
}

type osFS struct{}

func (osFS) Getwd() (string, error)                { return os.Getwd() }
func (osFS) Stat(name string) (fs.FileInfo, error) { return os.Stat(name) }
func (osFS) Symlink(oldname, newname string) error { return os.Symlink(oldname, newname) }
func (osFS) IsNotExist(err error) bool             { return os.IsNotExist(err) }

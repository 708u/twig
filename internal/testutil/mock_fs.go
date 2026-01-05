package testutil

import (
	"errors"
	"io/fs"
	"os"
	"slices"
)

// MockFS is a mock implementation of twig.FileSystem for testing.
type MockFS struct {
	// Override functions (takes precedence if set)
	StatFunc      func(name string) (fs.FileInfo, error)
	SymlinkFunc   func(oldname, newname string) error
	IsNotExistFunc func(err error) bool
	GlobFunc      func(dir, pattern string) ([]string, error)
	MkdirAllFunc  func(path string, perm fs.FileMode) error
	ReadDirFunc   func(name string) ([]os.DirEntry, error)
	RemoveFunc    func(name string) error
	WriteFileFunc func(name string, data []byte, perm fs.FileMode) error

	// ExistingPaths is a list of paths that exist (Stat returns nil, nil).
	ExistingPaths []string

	// SymlinkErr is returned by Symlink if set.
	SymlinkErr error

	// GlobResults maps pattern to matching paths.
	GlobResults map[string][]string

	// GlobErr is returned by Glob if set.
	GlobErr error

	// MkdirAllErr is returned by MkdirAll if set.
	MkdirAllErr error

	// DirContents maps directory path to its entries.
	DirContents map[string][]os.DirEntry

	// ReadDirErr is returned by ReadDir if set.
	ReadDirErr error

	// RemoveErr is returned by Remove if set.
	RemoveErr error

	// WriteFileErr is returned by WriteFile if set.
	WriteFileErr error

	// WrittenFiles records files written by WriteFile.
	WrittenFiles map[string][]byte
}

func (m *MockFS) Stat(name string) (fs.FileInfo, error) {
	if m.StatFunc != nil {
		return m.StatFunc(name)
	}
	if slices.Contains(m.ExistingPaths, name) {
		return nil, nil
	}
	return nil, fs.ErrNotExist
}

func (m *MockFS) Symlink(oldname, newname string) error {
	if m.SymlinkFunc != nil {
		return m.SymlinkFunc(oldname, newname)
	}
	return m.SymlinkErr
}

func (m *MockFS) IsNotExist(err error) bool {
	if m.IsNotExistFunc != nil {
		return m.IsNotExistFunc(err)
	}
	return errors.Is(err, fs.ErrNotExist)
}

func (m *MockFS) Glob(dir, pattern string) ([]string, error) {
	if m.GlobFunc != nil {
		return m.GlobFunc(dir, pattern)
	}
	if m.GlobErr != nil {
		return nil, m.GlobErr
	}
	if m.GlobResults != nil {
		return m.GlobResults[pattern], nil
	}
	return nil, nil
}

func (m *MockFS) MkdirAll(path string, perm fs.FileMode) error {
	if m.MkdirAllFunc != nil {
		return m.MkdirAllFunc(path, perm)
	}
	return m.MkdirAllErr
}

func (m *MockFS) ReadDir(name string) ([]os.DirEntry, error) {
	if m.ReadDirFunc != nil {
		return m.ReadDirFunc(name)
	}
	if m.ReadDirErr != nil {
		return nil, m.ReadDirErr
	}
	if m.DirContents != nil {
		if entries, ok := m.DirContents[name]; ok {
			return entries, nil
		}
	}
	return nil, nil
}

func (m *MockFS) Remove(name string) error {
	if m.RemoveFunc != nil {
		return m.RemoveFunc(name)
	}
	return m.RemoveErr
}

func (m *MockFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if m.WriteFileFunc != nil {
		return m.WriteFileFunc(name, data, perm)
	}
	if m.WrittenFiles != nil {
		m.WrittenFiles[name] = data
	}
	return m.WriteFileErr
}

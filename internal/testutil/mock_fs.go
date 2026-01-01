package testutil

import (
	"errors"
	"io/fs"
	"slices"
)

// MockFS is a mock implementation of gwt.FileSystem for testing.
type MockFS struct {
	// Override functions (takes precedence if set)
	StatFunc       func(name string) (fs.FileInfo, error)
	SymlinkFunc    func(oldname, newname string) error
	IsNotExistFunc func(err error) bool
	GlobFunc       func(dir, pattern string) ([]string, error)
	MkdirAllFunc   func(path string, perm fs.FileMode) error

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

package testutil

import (
	"errors"
	"io/fs"
	"slices"
)

// MockFS is a mock implementation of gwt.FileSystem for testing.
type MockFS struct {
	// Override functions (takes precedence if set)
	GetwdFunc      func() (string, error)
	StatFunc       func(name string) (fs.FileInfo, error)
	SymlinkFunc    func(oldname, newname string) error
	IsNotExistFunc func(err error) bool

	// Cwd is the current working directory returned by Getwd.
	Cwd string

	// GetwdErr is returned by Getwd if set.
	GetwdErr error

	// ExistingPaths is a list of paths that exist (Stat returns nil, nil).
	ExistingPaths []string

	// SymlinkErr is returned by Symlink if set.
	SymlinkErr error
}

func (m *MockFS) Getwd() (string, error) {
	if m.GetwdFunc != nil {
		return m.GetwdFunc()
	}
	if m.GetwdErr != nil {
		return "", m.GetwdErr
	}
	if m.Cwd != "" {
		return m.Cwd, nil
	}
	return "/repo/main", nil
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

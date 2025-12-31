package testutil

import (
	"io/fs"
)

// MockFS is a mock implementation of gwt.FileSystem for testing.
type MockFS struct {
	GetwdFunc   func() (string, error)
	StatFunc    func(name string) (fs.FileInfo, error)
	SymlinkFunc func(oldname, newname string) error
}

func (m *MockFS) Getwd() (string, error) {
	if m.GetwdFunc != nil {
		return m.GetwdFunc()
	}
	return "", nil
}

func (m *MockFS) Stat(name string) (fs.FileInfo, error) {
	if m.StatFunc != nil {
		return m.StatFunc(name)
	}
	return nil, nil
}

func (m *MockFS) Symlink(oldname, newname string) error {
	if m.SymlinkFunc != nil {
		return m.SymlinkFunc(oldname, newname)
	}
	return nil
}

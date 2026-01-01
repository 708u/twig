package testutil

// MockGitExecutor is a mock implementation of gwt.GitExecutor for testing.
type MockGitExecutor struct {
	RunFunc func(args ...string) ([]byte, error)
}

func (m *MockGitExecutor) Run(args ...string) ([]byte, error) {
	if m.RunFunc != nil {
		return m.RunFunc(args...)
	}
	return nil, nil
}

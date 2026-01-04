package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/708u/gwt"
	"github.com/708u/gwt/internal/testutil"
)

func TestResolveDirectory(t *testing.T) {
	t.Parallel()

	t.Run("EmptyDirFlag", func(t *testing.T) {
		t.Parallel()

		baseCwd := "/some/path"
		got, err := resolveDirectory("", baseCwd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != baseCwd {
			t.Errorf("got %q, want %q", got, baseCwd)
		}
	})

	t.Run("NonexistentPath", func(t *testing.T) {
		t.Parallel()

		baseCwd := t.TempDir()
		_, err := resolveDirectory("/nonexistent/path", baseCwd)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "cannot change to '/nonexistent/path'") {
			t.Errorf("error %q should contain path", err.Error())
		}
	})

	t.Run("PathIsFile", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "file.txt")
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := resolveDirectory(filePath, tmpDir)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not a directory") {
			t.Errorf("error %q should contain 'not a directory'", err.Error())
		}
	})

	t.Run("ValidAbsolutePath", func(t *testing.T) {
		t.Parallel()

		targetDir := t.TempDir()
		baseCwd := t.TempDir()

		got, err := resolveDirectory(targetDir, baseCwd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Resolve symlinks for comparison (macOS /var -> /private/var)
		want, _ := filepath.EvalSymlinks(targetDir)
		gotResolved, _ := filepath.EvalSymlinks(got)
		if gotResolved != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("ValidRelativePath", func(t *testing.T) {
		t.Parallel()

		baseCwd := t.TempDir()
		subDir := filepath.Join(baseCwd, "subdir")
		if err := os.Mkdir(subDir, 0755); err != nil {
			t.Fatal(err)
		}

		got, err := resolveDirectory("subdir", baseCwd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want, _ := filepath.EvalSymlinks(subDir)
		gotResolved, _ := filepath.EvalSymlinks(got)
		if gotResolved != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// mockCleanCommander is a test double for CleanCommander interface.
type mockCleanCommander struct {
	result gwt.CleanResult
	err    error
}

func (m *mockCleanCommander) Run(cwd string, opts gwt.CleanOptions) (gwt.CleanResult, error) {
	return m.result, m.err
}

func TestCleanCmd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		stdin      string
		result     gwt.CleanResult
		wantStdout string
		wantErr    bool
	}{
		{
			name:  "check_shows_candidates",
			args:  []string{"clean", "--check"},
			stdin: "",
			result: gwt.CleanResult{
				Candidates: []gwt.CleanCandidate{
					{Branch: "feat/a", Skipped: false},
				},
				Check: true,
			},
			wantStdout: "clean:\n  feat/a\n",
		},
		{
			name:  "check_no_candidates",
			args:  []string{"clean", "--check"},
			stdin: "",
			result: gwt.CleanResult{
				Candidates: []gwt.CleanCandidate{},
				Check:      true,
			},
			wantStdout: "No worktrees to clean\n",
		},
		{
			name:  "prompt_declined",
			args:  []string{"clean"},
			stdin: "n\n",
			result: gwt.CleanResult{
				Candidates: []gwt.CleanCandidate{
					{Branch: "feat/a", Skipped: false},
				},
				Check: true,
			},
			wantStdout: "clean:\n  feat/a\n\nProceed? [y/N]: ",
		},
		{
			name:  "verbose_shows_skipped",
			args:  []string{"clean", "--check", "-v"},
			stdin: "",
			result: gwt.CleanResult{
				Candidates: []gwt.CleanCandidate{
					{Branch: "feat/a", Skipped: false},
					{Branch: "feat/b", Skipped: true, SkipReason: gwt.SkipNotMerged},
				},
				Check: true,
			},
			wantStdout: "clean:\n  feat/a\n\nskip:\n  feat/b (not merged)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockCleanCommander{result: tt.result}

			cmd := newRootCmd(WithNewCleanCommander(func(cfg *gwt.Config) CleanCommander {
				return mock
			}))

			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			stdin := strings.NewReader(tt.stdin)

			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
			cmd.SetIn(stdin)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if stdout.String() != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout.String(), tt.wantStdout)
			}
		})
	}
}

// mockAddCommander is a mock implementation of AddCommander for testing.
type mockAddCommander struct {
	result     gwt.AddResult
	err        error
	calledName string
	calledOpts gwt.AddOptions
}

func (m *mockAddCommander) Run(name string) (gwt.AddResult, error) {
	m.calledName = name
	return m.result, m.err
}

// setupTestRepo creates a test git repository with gwt settings.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	_, mainDir := testutil.SetupTestRepo(t)

	gwtDir := filepath.Join(mainDir, ".gwt")
	if err := os.MkdirAll(gwtDir, 0755); err != nil {
		t.Fatal(err)
	}

	settingsContent := fmt.Sprintf(`worktree_source_dir = %q
worktree_destination_base_dir = %q
`, mainDir, filepath.Dir(mainDir))
	if err := os.WriteFile(filepath.Join(gwtDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
		t.Fatal(err)
	}

	testutil.RunGit(t, mainDir, "add", ".gwt")
	testutil.RunGit(t, mainDir, "commit", "-m", "add gwt settings")

	return mainDir
}

func TestAddCmd(t *testing.T) {
	t.Parallel()

	t.Run("BasicExecution", func(t *testing.T) {
		t.Parallel()

		mainDir := setupTestRepo(t)

		mock := &mockAddCommander{
			result: gwt.AddResult{
				Branch:       "feat/test",
				WorktreePath: "/path/to/worktree",
				Symlinks:     []gwt.SymlinkResult{},
			},
		}

		cmd := newRootCmd(WithNewAddCommander(func(cfg *gwt.Config, opts gwt.AddOptions) AddCommander {
			mock.calledOpts = opts
			return mock
		}))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "feat/test"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.calledName != "feat/test" {
			t.Errorf("calledName = %q, want %q", mock.calledName, "feat/test")
		}
	})

	t.Run("SyncFlag", func(t *testing.T) {
		t.Parallel()

		mainDir := setupTestRepo(t)

		mock := &mockAddCommander{
			result: gwt.AddResult{
				Branch:        "feat/sync",
				WorktreePath:  "/path/to/worktree",
				ChangesSynced: true,
			},
		}

		cmd := newRootCmd(WithNewAddCommander(func(cfg *gwt.Config, opts gwt.AddOptions) AddCommander {
			mock.calledOpts = opts
			return mock
		}))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "--sync", "feat/sync"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !mock.calledOpts.Sync {
			t.Error("expected Sync option to be true")
		}
	})

	t.Run("QuietFlag", func(t *testing.T) {
		t.Parallel()

		mainDir := setupTestRepo(t)

		mock := &mockAddCommander{
			result: gwt.AddResult{
				Branch:       "feat/quiet",
				WorktreePath: "/path/to/worktree",
			},
		}

		cmd := newRootCmd(WithNewAddCommander(func(cfg *gwt.Config, opts gwt.AddOptions) AddCommander {
			return mock
		}))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "--quiet", "feat/quiet"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Quiet mode should output only the worktree path
		if !strings.Contains(stdout.String(), "/path/to/worktree") {
			t.Errorf("stdout = %q, want to contain worktree path", stdout.String())
		}
		if strings.Contains(stdout.String(), "gwt add:") {
			t.Errorf("stdout = %q, should not contain 'gwt add:' in quiet mode", stdout.String())
		}
	})

	t.Run("LockFlags", func(t *testing.T) {
		t.Parallel()

		mainDir := setupTestRepo(t)

		mock := &mockAddCommander{
			result: gwt.AddResult{
				Branch:       "feat/lock",
				WorktreePath: "/path/to/worktree",
			},
		}

		cmd := newRootCmd(WithNewAddCommander(func(cfg *gwt.Config, opts gwt.AddOptions) AddCommander {
			mock.calledOpts = opts
			return mock
		}))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "--lock", "--reason", "USB work", "feat/lock"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !mock.calledOpts.Lock {
			t.Error("expected Lock option to be true")
		}
		if mock.calledOpts.LockReason != "USB work" {
			t.Errorf("LockReason = %q, want %q", mock.calledOpts.LockReason, "USB work")
		}
	})

	t.Run("ReasonWithoutLock", func(t *testing.T) {
		t.Parallel()

		mainDir := setupTestRepo(t)

		mock := &mockAddCommander{
			result: gwt.AddResult{
				Branch:       "feat/error",
				WorktreePath: "/path/to/worktree",
			},
		}

		cmd := newRootCmd(WithNewAddCommander(func(cfg *gwt.Config, opts gwt.AddOptions) AddCommander {
			return mock
		}))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "--reason", "some reason", "feat/error"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "--reason requires --lock") {
			t.Errorf("error = %q, want to contain '--reason requires --lock'", err.Error())
		}
	})

	t.Run("ErrorFromCommand", func(t *testing.T) {
		t.Parallel()

		mainDir := setupTestRepo(t)

		mock := &mockAddCommander{
			err: errors.New("worktree creation failed"),
		}

		cmd := newRootCmd(WithNewAddCommander(func(cfg *gwt.Config, opts gwt.AddOptions) AddCommander {
			return mock
		}))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "feat/fail"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "worktree creation failed") {
			t.Errorf("error = %q, want to contain 'worktree creation failed'", err.Error())
		}
	})

	t.Run("OutputWithSymlinks", func(t *testing.T) {
		t.Parallel()

		mainDir := setupTestRepo(t)

		mock := &mockAddCommander{
			result: gwt.AddResult{
				Branch:       "feat/symlinks",
				WorktreePath: "/path/to/worktree",
				Symlinks: []gwt.SymlinkResult{
					{Src: "/src/.envrc", Dst: "/dst/.envrc"},
					{Src: "/src/.tool-versions", Dst: "/dst/.tool-versions"},
				},
			},
		}

		cmd := newRootCmd(WithNewAddCommander(func(cfg *gwt.Config, opts gwt.AddOptions) AddCommander {
			return mock
		}))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "feat/symlinks"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Default output should show symlink count
		if !strings.Contains(stdout.String(), "2 symlinks") {
			t.Errorf("stdout = %q, want to contain '2 symlinks'", stdout.String())
		}
	})

	t.Run("OutputWithWarnings", func(t *testing.T) {
		t.Parallel()

		mainDir := setupTestRepo(t)

		mock := &mockAddCommander{
			result: gwt.AddResult{
				Branch:       "feat/warn",
				WorktreePath: "/path/to/worktree",
				Symlinks: []gwt.SymlinkResult{
					{Src: "/src/.envrc", Dst: "/dst/.envrc"},
					{Skipped: true, Reason: "pattern.txt does not match any files, skipping"},
				},
			},
		}

		cmd := newRootCmd(WithNewAddCommander(func(cfg *gwt.Config, opts gwt.AddOptions) AddCommander {
			return mock
		}))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "feat/warn"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Warning should be in stderr
		if !strings.Contains(stderr.String(), "pattern.txt does not match any files") {
			t.Errorf("stderr = %q, want to contain warning", stderr.String())
		}
		// Only 1 symlink created (1 skipped)
		if !strings.Contains(stdout.String(), "1 symlinks") {
			t.Errorf("stdout = %q, want to contain '1 symlinks'", stdout.String())
		}
	})

	t.Run("SyncAndCarryMutuallyExclusive", func(t *testing.T) {
		t.Parallel()

		mainDir := setupTestRepo(t)

		mock := &mockAddCommander{
			result: gwt.AddResult{
				Branch:       "feat/conflict",
				WorktreePath: "/path/to/worktree",
			},
		}

		cmd := newRootCmd(WithNewAddCommander(func(cfg *gwt.Config, opts gwt.AddOptions) AddCommander {
			return mock
		}))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "--sync", "--carry=@", "feat/conflict"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "cannot use --sync and --carry together") {
			t.Errorf("error = %q, want to contain mutual exclusion message", err.Error())
		}
	})
}

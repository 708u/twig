package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/708u/twig"
	"github.com/708u/twig/internal/testutil"
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

func TestResolveCarryFrom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		carryValue  string
		originalCwd string
		worktrees   []testutil.MockWorktree
		want        string
		wantErr     string
	}{
		{
			name:        "EmptyValue",
			carryValue:  "",
			originalCwd: "/original",
			wantErr:     "carry value cannot be empty",
		},
		{
			name:        "CurrentValue",
			carryValue:  carryFromCurrent,
			originalCwd: "/path/to/original",
			want:        "/path/to/original",
		},
		{
			name:        "BranchValue",
			carryValue:  "main",
			originalCwd: "/original",
			worktrees:   []testutil.MockWorktree{{Path: "/path/to/main", Branch: "main"}},
			want:        "/path/to/main",
		},
		{
			name:        "BranchNotFound",
			carryValue:  "nonexistent",
			originalCwd: "/original",
			worktrees:   []testutil.MockWorktree{{Path: "/path/to/main", Branch: "main"}},
			wantErr:     "failed to find worktree for branch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var git *twig.GitRunner
			if tt.worktrees != nil {
				git = &twig.GitRunner{
					Executor: &testutil.MockGitExecutor{Worktrees: tt.worktrees},
					Dir:      "/mock",
				}
			}

			got, err := resolveCarryFrom(tt.carryValue, tt.originalCwd, git)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// mockCleanCommander is a test double for CleanCommander interface.
type mockCleanCommander struct {
	result twig.CleanResult
	err    error
}

func (m *mockCleanCommander) Run(cwd string, opts twig.CleanOptions) (twig.CleanResult, error) {
	return m.result, m.err
}

func TestCleanCmd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		stdin      string
		result     twig.CleanResult
		wantStdout string
		wantErr    bool
	}{
		{
			name:  "check_shows_candidates",
			args:  []string{"clean", "--check"},
			stdin: "",
			result: twig.CleanResult{
				Candidates: []twig.CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: twig.CleanMerged},
				},
				Check: true,
			},
			wantStdout: "clean:\n  feat/a (merged)\n",
		},
		{
			name:  "check_no_candidates",
			args:  []string{"clean", "--check"},
			stdin: "",
			result: twig.CleanResult{
				Candidates: []twig.CleanCandidate{},
				Check:      true,
			},
			wantStdout: "No worktrees to clean\n",
		},
		{
			name:  "prompt_declined",
			args:  []string{"clean"},
			stdin: "n\n",
			result: twig.CleanResult{
				Candidates: []twig.CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: twig.CleanMerged},
				},
				Check: true,
			},
			wantStdout: "clean:\n  feat/a (merged)\n\nProceed? [y/N]: ",
		},
		{
			name:  "verbose_shows_skipped",
			args:  []string{"clean", "--check", "-v"},
			stdin: "",
			result: twig.CleanResult{
				Candidates: []twig.CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: twig.CleanMerged},
					{Branch: "feat/b", Skipped: true, SkipReason: twig.SkipNotMerged},
				},
				Check: true,
			},
			wantStdout: "clean:\n  feat/a (merged)\n\nskip:\n  feat/b (not merged)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockCleanCommander{result: tt.result}

			cmd := newRootCmd(WithCleanCommander(mock))

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
	result     twig.AddResult
	err        error
	calledName string
}

func (m *mockAddCommander) Run(name string) (twig.AddResult, error) {
	m.calledName = name
	return m.result, m.err
}

// mockListCommander is a test double for ListCommander interface.
type mockListCommander struct {
	result twig.ListResult
	err    error
}

func (m *mockListCommander) Run() (twig.ListResult, error) {
	return m.result, m.err
}

func TestListCmd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		result     twig.ListResult
		err        error
		wantStdout string
		wantErr    bool
	}{
		{
			name: "default output",
			args: []string{"list"},
			result: twig.ListResult{
				Worktrees: []twig.Worktree{
					{Path: "/repo/main", Branch: "main", HEAD: "abc1234567890"},
					{Path: "/repo/feat-a", Branch: "feat/a", HEAD: "def5678901234"},
				},
			},
			wantStdout: "/repo/main    abc1234 [main]\n/repo/feat-a  def5678 [feat/a]\n",
		},
		{
			name: "quiet flag outputs paths only",
			args: []string{"list", "--quiet"},
			result: twig.ListResult{
				Worktrees: []twig.Worktree{
					{Path: "/repo/main", Branch: "main", HEAD: "abc1234567890"},
					{Path: "/repo/feat-a", Branch: "feat/a", HEAD: "def5678901234"},
				},
			},
			wantStdout: "/repo/main\n/repo/feat-a\n",
		},
		{
			name: "short flag -q",
			args: []string{"list", "-q"},
			result: twig.ListResult{
				Worktrees: []twig.Worktree{
					{Path: "/repo/main", Branch: "main", HEAD: "abc1234567890"},
				},
			},
			wantStdout: "/repo/main\n",
		},
		{
			name: "empty list",
			args: []string{"list"},
			result: twig.ListResult{
				Worktrees: []twig.Worktree{},
			},
			wantStdout: "",
		},
		{
			name:    "error from commander",
			args:    []string{"list"},
			err:     errors.New("git error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockListCommander{result: tt.result, err: tt.err}

			cmd := newRootCmd(WithListCommander(mock))

			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
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

// mockRemoveCommander implements RemoveCommander for testing.
type mockRemoveCommander struct {
	calls   []removeCall
	results []removeResult
	idx     int
}

type removeCall struct {
	branch string
	cwd    string
	opts   twig.RemoveOptions
}

type removeResult struct {
	wt  twig.RemovedWorktree
	err error
}

func (m *mockRemoveCommander) Run(branch, cwd string, opts twig.RemoveOptions) (twig.RemovedWorktree, error) {
	m.calls = append(m.calls, removeCall{branch, cwd, opts})
	if m.idx < len(m.results) {
		r := m.results[m.idx]
		m.idx++
		return r.wt, r.err
	}
	return twig.RemovedWorktree{Branch: branch, WorktreePath: "/test/" + branch}, nil
}

// mockInitCommander implements InitCommander for testing.
type mockInitCommander struct {
	result     twig.InitResult
	err        error
	calledDir  string
	calledOpts twig.InitOptions
}

func (m *mockInitCommander) Run(dir string, opts twig.InitOptions) (twig.InitResult, error) {
	m.calledDir = dir
	m.calledOpts = opts
	return m.result, m.err
}

func TestAddCmd(t *testing.T) {
	t.Parallel()

	t.Run("BasicExecution", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)
		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}
		settingsContent := fmt.Sprintf(`worktree_destination_base_dir = %q
`, filepath.Dir(mainDir))
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

		mock := &mockAddCommander{
			result: twig.AddResult{
				Branch:       "feat/test",
				WorktreePath: "/path/to/worktree",
				Symlinks:     []twig.SymlinkResult{},
			},
		}

		cmd := newRootCmd(WithAddCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "feat/test"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.calledName != "feat/test" {
			t.Errorf("calledName = %q, want %q", mock.calledName, "feat/test")
		}
	})

	t.Run("SyncFlag", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)
		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}
		settingsContent := fmt.Sprintf(`worktree_destination_base_dir = %q
`, filepath.Dir(mainDir))
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

		mock := &mockAddCommander{
			result: twig.AddResult{
				Branch:        "feat/sync",
				WorktreePath:  "/path/to/worktree",
				ChangesSynced: true,
			},
		}

		cmd := newRootCmd(WithAddCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "--sync", "feat/sync"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.calledName != "feat/sync" {
			t.Errorf("calledName = %q, want %q", mock.calledName, "feat/sync")
		}
	})

	t.Run("QuietFlag", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)
		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}
		settingsContent := fmt.Sprintf(`worktree_destination_base_dir = %q
`, filepath.Dir(mainDir))
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

		mock := &mockAddCommander{
			result: twig.AddResult{
				Branch:       "feat/quiet",
				WorktreePath: "/path/to/worktree",
			},
		}

		cmd := newRootCmd(WithAddCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "--quiet", "feat/quiet"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(stdout.String(), "/path/to/worktree") {
			t.Errorf("stdout = %q, want to contain worktree path", stdout.String())
		}
		if strings.Contains(stdout.String(), "twig add:") {
			t.Errorf("stdout = %q, should not contain 'twig add:'", stdout.String())
		}
	})

	t.Run("LockFlags", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)
		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}
		settingsContent := fmt.Sprintf(`worktree_destination_base_dir = %q
`, filepath.Dir(mainDir))
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

		mock := &mockAddCommander{
			result: twig.AddResult{
				Branch:       "feat/lock",
				WorktreePath: "/path/to/worktree",
			},
		}

		cmd := newRootCmd(WithAddCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "--lock", "--reason", "USB work", "feat/lock"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.calledName != "feat/lock" {
			t.Errorf("calledName = %q, want %q", mock.calledName, "feat/lock")
		}
	})

	t.Run("ReasonWithoutLock", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)
		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}
		settingsContent := fmt.Sprintf(`worktree_destination_base_dir = %q
`, filepath.Dir(mainDir))
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

		mock := &mockAddCommander{
			result: twig.AddResult{
				Branch:       "feat/error",
				WorktreePath: "/path/to/worktree",
			},
		}

		cmd := newRootCmd(WithAddCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "--reason", "some reason", "feat/error"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "--reason requires --lock") {
			t.Errorf("error = %q, want to contain %q", err.Error(), "--reason requires --lock")
		}
	})

	t.Run("ErrorFromCommand", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)
		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}
		settingsContent := fmt.Sprintf(`worktree_destination_base_dir = %q
`, filepath.Dir(mainDir))
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

		mock := &mockAddCommander{
			result: twig.AddResult{
				Branch:       "feat/fail",
				WorktreePath: "/path/to/worktree",
			},
			err: errors.New("worktree creation failed"),
		}

		cmd := newRootCmd(WithAddCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "feat/fail"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "worktree creation failed") {
			t.Errorf("error = %q, want to contain %q", err.Error(), "worktree creation failed")
		}
	})

	t.Run("OutputWithSymlinks", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)
		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}
		settingsContent := fmt.Sprintf(`worktree_destination_base_dir = %q
`, filepath.Dir(mainDir))
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

		mock := &mockAddCommander{
			result: twig.AddResult{
				Branch:       "feat/symlinks",
				WorktreePath: "/path/to/worktree",
				Symlinks: []twig.SymlinkResult{
					{Src: "/src/.envrc", Dst: "/dst/.envrc"},
					{Src: "/src/.tool-versions", Dst: "/dst/.tool-versions"},
				},
			},
		}

		cmd := newRootCmd(WithAddCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "feat/symlinks"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(stdout.String(), "2 symlinks") {
			t.Errorf("stdout = %q, want to contain '2 symlinks'", stdout.String())
		}
	})

	t.Run("OutputWithWarnings", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)
		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}
		settingsContent := fmt.Sprintf(`worktree_destination_base_dir = %q
`, filepath.Dir(mainDir))
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

		mock := &mockAddCommander{
			result: twig.AddResult{
				Branch:       "feat/warn",
				WorktreePath: "/path/to/worktree",
				Symlinks: []twig.SymlinkResult{
					{Src: "/src/.envrc", Dst: "/dst/.envrc"},
					{Skipped: true, Reason: "pattern.txt does not match any files, skipping"},
				},
			},
		}

		cmd := newRootCmd(WithAddCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "feat/warn"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(stdout.String(), "1 symlinks") {
			t.Errorf("stdout = %q, want to contain '1 symlinks'", stdout.String())
		}
		if !strings.Contains(stderr.String(), "pattern.txt does not match any files") {
			t.Errorf("stderr = %q, want to contain warning", stderr.String())
		}
	})

	t.Run("CarryFlagWithoutValue", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		mock := &mockAddCommander{
			result: twig.AddResult{
				Branch:       "feat/carry",
				WorktreePath: "/path/to/worktree",
			},
		}

		cmd := newRootCmd(WithAddCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "feat/carry", "--carry"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.calledName != "feat/carry" {
			t.Errorf("calledName = %q, want %q", mock.calledName, "feat/carry")
		}
	})

	t.Run("CarryShortFlagWithoutValue", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		mock := &mockAddCommander{
			result: twig.AddResult{
				Branch:       "feat/carry-short",
				WorktreePath: "/path/to/worktree",
			},
		}

		cmd := newRootCmd(WithAddCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "feat/carry-short", "-c"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mock.calledName != "feat/carry-short" {
			t.Errorf("calledName = %q, want %q", mock.calledName, "feat/carry-short")
		}
	})

	t.Run("CarrySpaceSeparatedValueNotAllowed", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		mock := &mockAddCommander{
			result: twig.AddResult{
				Branch:       "feat/target",
				WorktreePath: "/path/to/worktree",
			},
		}

		cmd := newRootCmd(WithAddCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		// --carry feat/other uses space-separated syntax which is not allowed
		// feat/other becomes a second positional argument, causing an error
		cmd.SetArgs([]string{"-C", mainDir, "add", "feat/target", "--carry", "feat/other"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error for space-separated --carry value, got nil")
		}
		if !strings.Contains(err.Error(), "accepts 1 arg(s), received 2") {
			t.Errorf("error = %q, want to contain %q", err.Error(), "accepts 1 arg(s), received 2")
		}
	})

	t.Run("SyncAndCarryMutuallyExclusive", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)
		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}
		settingsContent := fmt.Sprintf(`worktree_destination_base_dir = %q
`, filepath.Dir(mainDir))
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

		mock := &mockAddCommander{
			result: twig.AddResult{
				Branch:       "feat/conflict",
				WorktreePath: "/path/to/worktree",
			},
		}

		cmd := newRootCmd(WithAddCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "--sync", "--carry=@", "feat/conflict"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "cannot use --sync and --carry together") {
			t.Errorf("error = %q, want to contain %q", err.Error(), "cannot use --sync and --carry together")
		}
	})

	t.Run("file_requires_carry_or_sync", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		cmd := newRootCmd()

		var stderr bytes.Buffer
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", mainDir, "add", "--file", "*.go", "feat/test"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "--file requires --carry or --sync flag") {
			t.Errorf("error = %q, want to contain %q", err.Error(), "--file requires --carry or --sync flag")
		}
	})

	t.Run("file_with_carry", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		mock := &mockAddCommander{
			result: twig.AddResult{
				Branch:       "feat/file-test",
				WorktreePath: "/path/to/worktree",
			},
		}

		cmd := newRootCmd(WithAddCommander(mock))

		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetArgs([]string{"-C", mainDir, "add", "--carry", "--file", "*.go", "--file", "cmd/**", "feat/file-test"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestRemoveCmd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		wantForce twig.WorktreeForceLevel
		wantDry   bool
	}{
		{
			name:      "no_flags",
			args:      []string{"remove", "feat/a"},
			wantForce: twig.WorktreeForceLevelNone,
			wantDry:   false,
		},
		{
			name:      "force_flag",
			args:      []string{"remove", "--force", "feat/a"},
			wantForce: twig.WorktreeForceLevelUnclean,
			wantDry:   false,
		},
		{
			name:      "force_short_flag",
			args:      []string{"remove", "-f", "feat/a"},
			wantForce: twig.WorktreeForceLevelUnclean,
			wantDry:   false,
		},
		{
			name:      "force_double_short_flag",
			args:      []string{"remove", "-ff", "feat/a"},
			wantForce: twig.WorktreeForceLevelLocked,
			wantDry:   false,
		},
		{
			name:      "force_double_separate_flags",
			args:      []string{"remove", "-f", "-f", "feat/a"},
			wantForce: twig.WorktreeForceLevelLocked,
			wantDry:   false,
		},
		{
			name:      "dry_run_flag",
			args:      []string{"remove", "--dry-run", "feat/a"},
			wantForce: twig.WorktreeForceLevelNone,
			wantDry:   true,
		},
		{
			name:      "force_and_dry_run",
			args:      []string{"remove", "--force", "--dry-run", "feat/a"},
			wantForce: twig.WorktreeForceLevelUnclean,
			wantDry:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockRemoveCommander{}

			cmd := newRootCmd(WithRemoveCommander(mock))

			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
			cmd.SetArgs(tt.args)

			_ = cmd.Execute()

			if len(mock.calls) != 1 {
				t.Fatalf("expected 1 call, got %d", len(mock.calls))
			}

			call := mock.calls[0]
			if call.opts.Force != tt.wantForce {
				t.Errorf("Force = %v, want %v", call.opts.Force, tt.wantForce)
			}
			if call.opts.DryRun != tt.wantDry {
				t.Errorf("DryRun = %v, want %v", call.opts.DryRun, tt.wantDry)
			}
		})
	}
}

func TestRemoveCmd_OutputFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		results    []removeResult
		wantStdout string
		wantStderr string
	}{
		{
			name: "success_output",
			args: []string{"remove", "feat/a"},
			results: []removeResult{
				{wt: twig.RemovedWorktree{Branch: "feat/a", WorktreePath: "/test/feat/a"}},
			},
			wantStdout: "twig remove: feat/a\n",
			wantStderr: "",
		},
		{
			name: "error_output",
			args: []string{"remove", "feat/a"},
			results: []removeResult{
				{wt: twig.RemovedWorktree{}, err: errors.New("not found")},
			},
			wantStdout: "",
			wantStderr: "error: feat/a: not found\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockRemoveCommander{results: tt.results}

			cmd := newRootCmd(WithRemoveCommander(mock))

			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
			cmd.SetArgs(tt.args)

			_ = cmd.Execute()

			if stdout.String() != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout.String(), tt.wantStdout)
			}
			if stderr.String() != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestRemoveCmd_MultipleBranches(t *testing.T) {
	t.Parallel()

	mock := &mockRemoveCommander{}

	cmd := newRootCmd(WithRemoveCommander(mock))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"remove", "feat/a", "feat/b", "feat/c"})

	_ = cmd.Execute()

	if len(mock.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(mock.calls))
	}

	branches := []string{mock.calls[0].branch, mock.calls[1].branch, mock.calls[2].branch}
	expected := []string{"feat/a", "feat/b", "feat/c"}
	for i, got := range branches {
		if got != expected[i] {
			t.Errorf("call[%d].branch = %q, want %q", i, got, expected[i])
		}
	}

	// Check output contains all branches
	out := stdout.String()
	for _, b := range expected {
		if !strings.Contains(out, b) {
			t.Errorf("output should contain %q", b)
		}
	}
}

func TestInitCmd(t *testing.T) {
	t.Parallel()

	t.Run("BasicExecution", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		mock := &mockInitCommander{
			result: twig.InitResult{
				Created: true,
			},
		}

		cmd := newRootCmd(WithInitCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", tmpDir, "init"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !mock.calledOpts.Force {
			// Expected: Force should be false by default
		} else {
			t.Error("expected Force to be false")
		}
	})

	t.Run("ForceFlag", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		mock := &mockInitCommander{
			result: twig.InitResult{
				Created:     true,
				Overwritten: true,
			},
		}

		cmd := newRootCmd(WithInitCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", tmpDir, "init", "--force"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !mock.calledOpts.Force {
			t.Error("expected Force to be true")
		}
	})

	t.Run("ForceShortFlag", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		mock := &mockInitCommander{
			result: twig.InitResult{
				Created:     true,
				Overwritten: true,
			},
		}

		cmd := newRootCmd(WithInitCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", tmpDir, "init", "-f"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !mock.calledOpts.Force {
			t.Error("expected Force to be true")
		}
	})

	t.Run("ErrorFromCommand", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		mock := &mockInitCommander{
			err: errors.New("permission denied"),
		}

		cmd := newRootCmd(WithInitCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", tmpDir, "init"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("error = %q, want to contain %q", err.Error(), "permission denied")
		}
	})

	t.Run("SkippedOutput", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		mock := &mockInitCommander{
			result: twig.InitResult{
				Skipped: true,
			},
		}

		cmd := newRootCmd(WithInitCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", tmpDir, "init"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(stdout.String(), "Skipped") {
			t.Errorf("stdout = %q, want to contain 'Skipped'", stdout.String())
		}
	})

	t.Run("CreatedOutput", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		mock := &mockInitCommander{
			result: twig.InitResult{
				Created: true,
			},
		}

		cmd := newRootCmd(WithInitCommander(mock))

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-C", tmpDir, "init"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(stdout.String(), "Created") {
			t.Errorf("stdout = %q, want to contain 'Created'", stdout.String())
		}
	})
}

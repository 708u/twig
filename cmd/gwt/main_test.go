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

// mockRemoveCommander implements RemoveCommander for testing.
type mockRemoveCommander struct {
	calls   []removeCall
	results []removeResult
	idx     int
}

type removeCall struct {
	branch string
	cwd    string
	opts   gwt.RemoveOptions
}

type removeResult struct {
	wt  gwt.RemovedWorktree
	err error
}

func (m *mockRemoveCommander) Run(branch, cwd string, opts gwt.RemoveOptions) (gwt.RemovedWorktree, error) {
	m.calls = append(m.calls, removeCall{branch, cwd, opts})
	if m.idx < len(m.results) {
		r := m.results[m.idx]
		m.idx++
		return r.wt, r.err
	}
	return gwt.RemovedWorktree{Branch: branch, WorktreePath: "/test/" + branch}, nil
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

	tests := []struct {
		name                  string
		args                  []string
		result                gwt.AddResult
		err                   error
		wantOpts              gwt.AddOptions
		wantCalledName        string
		wantStdoutContains    []string
		wantStdoutNotContains []string
		wantStderrContains    []string
		wantErrContains       string
	}{
		{
			name: "basic_execution",
			args: []string{"add", "feat/test"},
			result: gwt.AddResult{
				Branch:       "feat/test",
				WorktreePath: "/path/to/worktree",
				Symlinks:     []gwt.SymlinkResult{},
			},
			wantCalledName: "feat/test",
		},
		{
			name: "sync_flag",
			args: []string{"add", "--sync", "feat/sync"},
			result: gwt.AddResult{
				Branch:        "feat/sync",
				WorktreePath:  "/path/to/worktree",
				ChangesSynced: true,
			},
			wantOpts: gwt.AddOptions{Sync: true},
		},
		{
			name: "quiet_flag",
			args: []string{"add", "--quiet", "feat/quiet"},
			result: gwt.AddResult{
				Branch:       "feat/quiet",
				WorktreePath: "/path/to/worktree",
			},
			wantStdoutContains:    []string{"/path/to/worktree"},
			wantStdoutNotContains: []string{"gwt add:"},
		},
		{
			name: "lock_flags",
			args: []string{"add", "--lock", "--reason", "USB work", "feat/lock"},
			result: gwt.AddResult{
				Branch:       "feat/lock",
				WorktreePath: "/path/to/worktree",
			},
			wantOpts: gwt.AddOptions{Lock: true, LockReason: "USB work"},
		},
		{
			name: "reason_without_lock",
			args: []string{"add", "--reason", "some reason", "feat/error"},
			result: gwt.AddResult{
				Branch:       "feat/error",
				WorktreePath: "/path/to/worktree",
			},
			wantErrContains: "--reason requires --lock",
		},
		{
			name: "error_from_command",
			args: []string{"add", "feat/fail"},
			err:  errors.New("worktree creation failed"),
			result: gwt.AddResult{
				Branch:       "feat/fail",
				WorktreePath: "/path/to/worktree",
			},
			wantErrContains: "worktree creation failed",
		},
		{
			name: "output_with_symlinks",
			args: []string{"add", "feat/symlinks"},
			result: gwt.AddResult{
				Branch:       "feat/symlinks",
				WorktreePath: "/path/to/worktree",
				Symlinks: []gwt.SymlinkResult{
					{Src: "/src/.envrc", Dst: "/dst/.envrc"},
					{Src: "/src/.tool-versions", Dst: "/dst/.tool-versions"},
				},
			},
			wantStdoutContains: []string{"2 symlinks"},
		},
		{
			name: "output_with_warnings",
			args: []string{"add", "feat/warn"},
			result: gwt.AddResult{
				Branch:       "feat/warn",
				WorktreePath: "/path/to/worktree",
				Symlinks: []gwt.SymlinkResult{
					{Src: "/src/.envrc", Dst: "/dst/.envrc"},
					{Skipped: true, Reason: "pattern.txt does not match any files, skipping"},
				},
			},
			wantStdoutContains: []string{"1 symlinks"},
			wantStderrContains: []string{"pattern.txt does not match any files"},
		},
		{
			name: "sync_and_carry_mutually_exclusive",
			args: []string{"add", "--sync", "--carry=@", "feat/conflict"},
			result: gwt.AddResult{
				Branch:       "feat/conflict",
				WorktreePath: "/path/to/worktree",
			},
			wantErrContains: "cannot use --sync and --carry together",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mainDir := setupTestRepo(t)

			mock := &mockAddCommander{
				result: tt.result,
				err:    tt.err,
			}

			cmd := newRootCmd(WithNewAddCommander(func(cfg *gwt.Config, opts gwt.AddOptions) AddCommander {
				mock.calledOpts = opts
				return mock
			}))

			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(append([]string{"-C", mainDir}, tt.args...))

			err := cmd.Execute()

			if tt.wantErrContains != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErrContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantCalledName != "" && mock.calledName != tt.wantCalledName {
				t.Errorf("calledName = %q, want %q", mock.calledName, tt.wantCalledName)
			}

			if tt.wantOpts.Sync && !mock.calledOpts.Sync {
				t.Error("expected Sync option to be true")
			}
			if tt.wantOpts.Lock && !mock.calledOpts.Lock {
				t.Error("expected Lock option to be true")
			}
			if tt.wantOpts.LockReason != "" && mock.calledOpts.LockReason != tt.wantOpts.LockReason {
				t.Errorf("LockReason = %q, want %q", mock.calledOpts.LockReason, tt.wantOpts.LockReason)
			}

			for _, want := range tt.wantStdoutContains {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("stdout = %q, want to contain %q", stdout.String(), want)
				}
			}

			for _, notWant := range tt.wantStdoutNotContains {
				if strings.Contains(stdout.String(), notWant) {
					t.Errorf("stdout = %q, should not contain %q", stdout.String(), notWant)
				}
			}

			for _, want := range tt.wantStderrContains {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr = %q, want to contain %q", stderr.String(), want)
				}
			}
		})
	}
}

func TestRemoveCmd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		wantForce bool
		wantDry   bool
	}{
		{
			name:      "no_flags",
			args:      []string{"remove", "feat/a"},
			wantForce: false,
			wantDry:   false,
		},
		{
			name:      "force_flag",
			args:      []string{"remove", "--force", "feat/a"},
			wantForce: true,
			wantDry:   false,
		},
		{
			name:      "force_short_flag",
			args:      []string{"remove", "-f", "feat/a"},
			wantForce: true,
			wantDry:   false,
		},
		{
			name:      "dry_run_flag",
			args:      []string{"remove", "--dry-run", "feat/a"},
			wantForce: false,
			wantDry:   true,
		},
		{
			name:      "both_flags",
			args:      []string{"remove", "--force", "--dry-run", "feat/a"},
			wantForce: true,
			wantDry:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockRemoveCommander{}

			cmd := newRootCmd(WithNewRemoveCommander(func(cfg *gwt.Config) RemoveCommander {
				return mock
			}))

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
				{wt: gwt.RemovedWorktree{Branch: "feat/a", WorktreePath: "/test/feat/a"}},
			},
			wantStdout: "gwt remove: feat/a\n",
			wantStderr: "",
		},
		{
			name: "error_output",
			args: []string{"remove", "feat/a"},
			results: []removeResult{
				{wt: gwt.RemovedWorktree{}, err: errors.New("not found")},
			},
			wantStdout: "",
			wantStderr: "error: feat/a: not found\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockRemoveCommander{results: tt.results}

			cmd := newRootCmd(WithNewRemoveCommander(func(cfg *gwt.Config) RemoveCommander {
				return mock
			}))

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

	cmd := newRootCmd(WithNewRemoveCommander(func(cfg *gwt.Config) RemoveCommander {
		return mock
	}))

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

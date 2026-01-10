package twig

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/708u/twig/internal/testutil"
)

func TestInitCommand_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		opts            InitOptions
		setupFS         func() *testutil.MockFS
		wantCreated     bool
		wantSkipped     bool
		wantOverwritten bool
		wantErr         bool
		errContains     string
	}{
		{
			name: "creates settings file in new directory",
			opts: InitOptions{Force: false},
			setupFS: func() *testutil.MockFS {
				return &testutil.MockFS{
					WrittenFiles: make(map[string][]byte),
				}
			},
			wantCreated: true,
			wantSkipped: false,
		},
		{
			name: "skips when settings file already exists",
			opts: InitOptions{Force: false},
			setupFS: func() *testutil.MockFS {
				return &testutil.MockFS{
					ExistingPaths: []string{filepath.Join("/test", ".twig", "settings.toml")},
					WrittenFiles:  make(map[string][]byte),
				}
			},
			wantCreated: false,
			wantSkipped: true,
		},
		{
			name: "overwrites when force is true and file exists",
			opts: InitOptions{Force: true},
			setupFS: func() *testutil.MockFS {
				return &testutil.MockFS{
					ExistingPaths: []string{filepath.Join("/test", ".twig", "settings.toml")},
					WrittenFiles:  make(map[string][]byte),
				}
			},
			wantCreated:     true,
			wantSkipped:     false,
			wantOverwritten: true,
		},
		{
			name: "returns error when mkdir fails",
			opts: InitOptions{Force: false},
			setupFS: func() *testutil.MockFS {
				return &testutil.MockFS{
					MkdirAllErr:  errors.New("permission denied"),
					WrittenFiles: make(map[string][]byte),
				}
			},
			wantErr:     true,
			errContains: "failed to create config directory",
		},
		{
			name: "returns error when write fails",
			opts: InitOptions{Force: false},
			setupFS: func() *testutil.MockFS {
				return &testutil.MockFS{
					WriteFileErr: errors.New("disk full"),
					WrittenFiles: make(map[string][]byte),
				}
			},
			wantErr:     true,
			errContains: "failed to write settings file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockFS := tt.setupFS()
			cmd := NewInitCommand(mockFS)

			result, err := cmd.Run("/test", tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Created != tt.wantCreated {
				t.Errorf("Created = %v, want %v", result.Created, tt.wantCreated)
			}
			if result.Skipped != tt.wantSkipped {
				t.Errorf("Skipped = %v, want %v", result.Skipped, tt.wantSkipped)
			}
			if result.Overwritten != tt.wantOverwritten {
				t.Errorf("Overwritten = %v, want %v", result.Overwritten, tt.wantOverwritten)
			}

			// Verify file was written when created
			if tt.wantCreated {
				expectedPath := filepath.Join("/test", ".twig", "settings.toml")
				if _, ok := mockFS.WrittenFiles[expectedPath]; !ok {
					t.Errorf("expected file to be written at %s", expectedPath)
				}
			}
		})
	}
}

func TestInitResult_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		result     InitResult
		opts       InitFormatOptions
		wantStdout string
	}{
		{
			name: "created message",
			result: InitResult{
				Created: true,
			},
			opts:       InitFormatOptions{},
			wantStdout: "Created .twig/settings.toml\n",
		},
		{
			name: "skipped message",
			result: InitResult{
				Skipped: true,
			},
			opts:       InitFormatOptions{},
			wantStdout: "Skipped .twig/settings.toml (already exists)\n",
		},
		{
			name: "overwritten message",
			result: InitResult{
				Created:     true,
				Overwritten: true,
			},
			opts:       InitFormatOptions{},
			wantStdout: "Created .twig/settings.toml (overwritten)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			formatted := tt.result.Format(tt.opts)

			if formatted.Stdout != tt.wantStdout {
				t.Errorf("Stdout = %q, want %q", formatted.Stdout, tt.wantStdout)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

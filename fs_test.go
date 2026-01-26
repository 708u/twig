package twig

import "testing"

func TestIsPathWithin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		basePath string
		want     bool
	}{
		{
			name:     "exact_match",
			path:     "/path/to/repo",
			basePath: "/path/to/repo",
			want:     true,
		},
		{
			name:     "direct_child",
			path:     "/path/to/repo/subdir",
			basePath: "/path/to/repo",
			want:     true,
		},
		{
			name:     "nested_child",
			path:     "/path/to/repo/subdir/nested",
			basePath: "/path/to/repo",
			want:     true,
		},
		{
			name:     "similar_prefix_not_within",
			path:     "/path/to/repo-worktree/feat/x",
			basePath: "/path/to/repo",
			want:     false,
		},
		{
			name:     "completely_different_path",
			path:     "/other/path",
			basePath: "/path/to/repo",
			want:     false,
		},
		{
			name:     "parent_of_base_not_within",
			path:     "/path/to",
			basePath: "/path/to/repo",
			want:     false,
		},
		{
			name:     "sibling_directory_not_within",
			path:     "/path/to/other",
			basePath: "/path/to/repo",
			want:     false,
		},
		{
			name:     "base_with_trailing_separator_prefix",
			path:     "/path/to/repoextra",
			basePath: "/path/to/repo",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := isPathWithin(tt.path, tt.basePath)
			if got != tt.want {
				t.Errorf("isPathWithin(%q, %q) = %v, want %v",
					tt.path, tt.basePath, got, tt.want)
			}
		})
	}
}

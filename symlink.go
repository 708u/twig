package twig

import (
	"fmt"
	"path/filepath"
)

// CreateSymlinksOptions configures symlink creation behavior.
type CreateSymlinksOptions struct {
	Force bool // Replace existing symlinks
}

// createSymlinks creates symlinks from srcDir to dstDir based on glob patterns.
// Returns results for each symlink operation.
func createSymlinks(fs FileSystem, srcDir, dstDir string, patterns []string, opts CreateSymlinksOptions) ([]SymlinkResult, error) {
	var results []SymlinkResult

	for _, pattern := range patterns {
		matches, err := fs.Glob(srcDir, pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %s: %w", pattern, err)
		}
		if len(matches) == 0 {
			results = append(results, SymlinkResult{
				Skipped: true,
				Reason:  fmt.Sprintf("%s does not match any files, skipping", pattern),
			})
			continue
		}

		for _, match := range matches {
			src := filepath.Join(srcDir, match)
			dst := filepath.Join(dstDir, match)

			// Check if destination already exists
			if info, err := fs.Lstat(dst); err == nil && info != nil {
				// Check if it's a symlink
				isSymlink := info.Mode()&0o120000 != 0
				if opts.Force && isSymlink {
					// Force mode: remove existing symlink and recreate
					if err := fs.Remove(dst); err != nil {
						return nil, fmt.Errorf("failed to remove existing symlink for %s: %w", match, err)
					}
				} else {
					// Skip existing file/symlink
					results = append(results, SymlinkResult{
						Src:     src,
						Dst:     dst,
						Skipped: true,
						Reason:  fmt.Sprintf("skipping symlink for %s (already exists)", match),
					})
					continue
				}
			}

			if dir := filepath.Dir(dst); dir != dstDir {
				if err := fs.MkdirAll(dir, 0755); err != nil {
					return nil, fmt.Errorf("failed to create directory for %s: %w", match, err)
				}
			}

			if err := fs.Symlink(src, dst); err != nil {
				return nil, fmt.Errorf("failed to create symlink for %s: %w", match, err)
			}

			results = append(results, SymlinkResult{Src: src, Dst: dst})
		}
	}

	return results, nil
}

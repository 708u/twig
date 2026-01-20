package twig

import (
	"fmt"
	"io/fs"
	"path/filepath"
)

// createSymlinks creates symlinks from srcDir to dstDir based on glob patterns.
// Existing symlinks are replaced. Regular files are skipped to prevent data loss.
// Returns results for each symlink operation.
func createSymlinks(fsys FileSystem, srcDir, dstDir string, patterns []string) ([]SymlinkResult, error) {
	var results []SymlinkResult

	for _, pattern := range patterns {
		matches, err := fsys.Glob(srcDir, pattern)
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
			if info, err := fsys.Lstat(dst); err == nil && info != nil {
				isSymlink := info.Mode()&fs.ModeSymlink != 0
				if isSymlink {
					// Remove existing symlink and recreate
					if err := fsys.Remove(dst); err != nil {
						return nil, fmt.Errorf("failed to remove existing symlink for %s: %w", match, err)
					}
				} else {
					// Skip regular files to prevent data loss
					results = append(results, SymlinkResult{
						Src:     src,
						Dst:     dst,
						Skipped: true,
						Reason:  fmt.Sprintf("skipping symlink for %s (regular file exists)", match),
					})
					continue
				}
			}

			if dir := filepath.Dir(dst); dir != dstDir {
				if err := fsys.MkdirAll(dir, 0755); err != nil {
					return nil, fmt.Errorf("failed to create directory for %s: %w", match, err)
				}
			}

			if err := fsys.Symlink(src, dst); err != nil {
				return nil, fmt.Errorf("failed to create symlink for %s: %w", match, err)
			}

			results = append(results, SymlinkResult{Src: src, Dst: dst})
		}
	}

	return results, nil
}

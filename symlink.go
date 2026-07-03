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
			dstParent := filepath.Dir(dst)

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

			if dstParent != dstDir {
				if err := fsys.MkdirAll(dstParent, 0755); err != nil {
					return nil, fmt.Errorf("failed to create directory for %s: %w", match, err)
				}
			}

			relSrc, err := filepath.Rel(dstParent, src)
			if err != nil {
				return nil, fmt.Errorf("failed to compute relative path for %s: %w", match, err)
			}
			if err := fsys.Symlink(relSrc, dst); err != nil {
				return nil, fmt.Errorf("failed to create symlink for %s: %w", match, err)
			}

			results = append(results, SymlinkResult{Src: src, Dst: dst})
		}
	}

	return results, nil
}

// filterSymlinkManagedFiles removes entries from changedFiles whose path
// matches one of the symlink glob patterns evaluated against dir. It returns
// the filtered list and the paths that were excluded.
func filterSymlinkManagedFiles(fsys FileSystem, dir string, patterns []string, changedFiles []FileStatus) ([]FileStatus, []string) {
	if len(patterns) == 0 || len(changedFiles) == 0 {
		return changedFiles, nil
	}

	symlinkPaths := make(map[string]bool)
	for _, pattern := range patterns {
		matches, err := fsys.Glob(dir, pattern)
		if err != nil {
			// An invalid pattern should not block uncommitted-changes detection.
			continue
		}
		for _, m := range matches {
			symlinkPaths[m] = true
		}
	}
	if len(symlinkPaths) == 0 {
		return changedFiles, nil
	}

	filtered := make([]FileStatus, 0, len(changedFiles))
	var excluded []string
	for _, f := range changedFiles {
		if symlinkPaths[f.Path] {
			excluded = append(excluded, f.Path)
			continue
		}
		filtered = append(filtered, f)
	}
	return filtered, excluded
}

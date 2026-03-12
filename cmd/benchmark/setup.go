package main

import "github.com/spf13/cobra"

var (
	setupFiles     int
	setupCommits   int
	setupWorktrees int
	setupMerged    bool
)

var setupCmd = &cobra.Command{
	Use:   "setup <output-dir>",
	Short: "Generate a benchmark repository",
	Long: `Generate a benchmark repository for twig CLI performance testing.

Examples:
  benchmark setup --files=1000 --worktrees=10 /tmp/bench-small
  benchmark setup --files=10000 --worktrees=100 --merged /tmp/bench-large`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setupRepo(args[0], setupFiles, setupCommits, setupWorktrees, setupMerged)
	},
}

func init() {
	setupCmd.Flags().IntVar(&setupFiles, "files", 1000, "Number of files to generate")
	setupCmd.Flags().IntVar(&setupCommits, "commits", 100, "Number of commits to create")
	setupCmd.Flags().IntVar(&setupWorktrees, "worktrees", 10, "Number of worktrees to create")
	setupCmd.Flags().BoolVar(&setupMerged, "merged", false, "Create merged worktrees (for clean benchmark)")
}

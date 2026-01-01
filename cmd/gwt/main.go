package main

import (
	"fmt"
	"os"

	"github.com/708u/gwt"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gwt",
	Short: "Manage git worktrees and branches together",
}

var addCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Create a new worktree with a new branch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		cfg, err := gwt.LoadConfig(cwd)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		return gwt.NewAddCommand(cfg, gwt.NewGitRunner()).Run(args[0])
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

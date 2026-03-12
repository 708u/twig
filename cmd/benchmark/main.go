// Package main provides a benchmark tool for twig CLI performance testing.
package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Benchmark tool for twig CLI performance testing",
}

func init() {
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(runCmd)
}

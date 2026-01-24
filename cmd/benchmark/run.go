package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

const benchBase = "/tmp/twig-bench"

type scale struct {
	files     int
	commits   int
	worktrees int
}

var scales = map[string]scale{
	"small":  {1000, 100, 10},
	"medium": {5000, 500, 50},
	"large":  {10000, 1000, 100},
}

var (
	runFiles     int
	runCommits   int
	runWorktrees int
)

var runCmd = &cobra.Command{
	Use:   "run <benchmark> [scale]",
	Short: "Run benchmarks using hyperfine",
	Long: `Run twig benchmarks using hyperfine.

Benchmarks:
  list        Benchmark twig list
  add         Benchmark twig add
  remove      Benchmark twig remove
  clean       Benchmark twig clean --yes
  all         Run all benchmarks

Scales (preset):
  small       1000 files, 100 commits, 10 worktrees
  medium      5000 files, 500 commits, 50 worktrees
  large       10000 files, 1000 commits, 100 worktrees

Custom scale flags override preset values when specified.

Examples:
  benchmark run list small
  benchmark run clean large
  benchmark run all medium
  benchmark run list small --files=2000
  benchmark run add small --worktrees=20
  benchmark run all small --files=500 --commits=50 --worktrees=5`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		benchmark := args[0]

		var s scale
		if len(args) >= 2 {
			scaleName := args[1]
			var ok bool
			s, ok = scales[scaleName]
			if !ok {
				return fmt.Errorf("unknown scale '%s' (available: small, medium, large)", scaleName)
			}
		} else {
			s = scales["small"]
		}

		if runFiles > 0 {
			s.files = runFiles
		}
		if runCommits > 0 {
			s.commits = runCommits
		}
		if runWorktrees > 0 {
			s.worktrees = runWorktrees
		}

		if err := checkDeps(); err != nil {
			return err
		}

		defer func() {
			fmt.Println("\nCleaning up benchmark directory...")
			_ = os.RemoveAll(benchBase)
		}()

		switch benchmark {
		case "list":
			return benchList(s)
		case "add":
			return benchAdd(s)
		case "remove":
			return benchRemove(s)
		case "clean":
			return benchClean(s)
		case "all":
			return benchAll(s)
		default:
			return fmt.Errorf("unknown benchmark '%s' (available: list, add, remove, clean, all)", benchmark)
		}
	},
}

func checkDeps() error {
	if _, err := exec.LookPath("hyperfine"); err != nil {
		return fmt.Errorf("hyperfine is required but not installed\n  Install with: brew install hyperfine")
	}
	if _, err := exec.LookPath("twig"); err != nil {
		return fmt.Errorf("twig is required but not installed\n  Install with: go install github.com/708u/twig/cmd/twig@latest")
	}
	return nil
}

func benchList(s scale) error {
	fmt.Println("\n=== Benchmark: twig list ===")

	if err := setupRepo(benchBase, s.files, s.commits, s.worktrees, false); err != nil {
		return err
	}

	return runHyperfine(
		"--warmup", "3",
		"--runs", "20",
		fmt.Sprintf("twig list -C %s/main", benchBase),
	)
}

func benchAdd(s scale) error {
	fmt.Println("\n=== Benchmark: twig add ===")

	if err := setupRepo(benchBase, s.files, s.commits, s.worktrees, false); err != nil {
		return err
	}

	return runHyperfine(
		"--warmup", "1",
		"--runs", "10",
		"--prepare", fmt.Sprintf("twig remove bench/bench-test -C %s/main --force 2>/dev/null || true", benchBase),
		fmt.Sprintf("twig add bench/bench-test -C %s/main", benchBase),
	)
}

func benchRemove(s scale) error {
	fmt.Println("\n=== Benchmark: twig remove ===")

	if err := setupRepo(benchBase, s.files, s.commits, s.worktrees, false); err != nil {
		return err
	}

	return runHyperfine(
		"--warmup", "1",
		"--runs", "10",
		"--prepare", fmt.Sprintf("twig add bench/bench-test -C %s/main 2>/dev/null || true", benchBase),
		fmt.Sprintf("twig remove bench/bench-test -C %s/main --force", benchBase),
	)
}

func benchClean(s scale) error {
	fmt.Println("\n=== Benchmark: twig clean ===")

	if err := setupRepo(benchBase, s.files, s.commits, s.worktrees, true); err != nil {
		return err
	}

	prepareCmd := fmt.Sprintf("go run ./cmd/benchmark setup --files=%d --commits=%d --worktrees=%d --merged %s 2>/dev/null",
		s.files, s.commits, s.worktrees, benchBase)

	return runHyperfine(
		"--warmup", "1",
		"--runs", "5",
		"--prepare", prepareCmd,
		fmt.Sprintf("twig clean --yes -C %s/main", benchBase),
	)
}

func benchAll(s scale) error {
	benchmarks := []func(scale) error{benchList, benchAdd, benchRemove, benchClean}
	for _, bench := range benchmarks {
		if err := bench(s); err != nil {
			return err
		}
	}
	fmt.Println("\nAll benchmarks completed.")
	return nil
}

func runHyperfine(args ...string) error {
	cmd := exec.Command("hyperfine", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func init() {
	runCmd.Flags().IntVar(&runFiles, "files", 0, "Override number of files (0 = use scale default)")
	runCmd.Flags().IntVar(&runCommits, "commits", 0, "Override number of commits (0 = use scale default)")
	runCmd.Flags().IntVar(&runWorktrees, "worktrees", 0, "Override number of worktrees (0 = use scale default)")
}

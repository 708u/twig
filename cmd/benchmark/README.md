# twig Benchmarks

Performance benchmarks for the twig CLI using hyperfine.

## Prerequisites

Install required tools:

```bash
# Install hyperfine (macOS)
brew install hyperfine

# Install twig
go install github.com/708u/twig/cmd/twig@latest
```

## Quick Start

Run benchmarks from the repository root:

```bash
# Run all benchmarks with small scale
make benchmark-all

# Run specific benchmark
make benchmark-list
make benchmark-add
make benchmark-remove
make benchmark-clean
```

## Manual Usage

### Run Benchmarks

```bash
# Run specific command benchmark
go run ./cmd/benchmark run list small
go run ./cmd/benchmark run add medium
go run ./cmd/benchmark run clean large

# Run all benchmarks
go run ./cmd/benchmark run all small

# Custom scale options (override preset values)
go run ./cmd/benchmark run list small --files=2000
go run ./cmd/benchmark run add small --worktrees=20
go run ./cmd/benchmark run all small --files=500 --commits=50 --worktrees=5
```

### Setup Repository Only

Generate a benchmark repository without running benchmarks:

```bash
# Small scale (quick testing)
go run ./cmd/benchmark setup --files=1000 --commits=100 --worktrees=10 /tmp/twig-bench

# Large scale (stress testing)
go run ./cmd/benchmark setup --files=10000 --commits=1000 --worktrees=100 /tmp/twig-bench

# With merged worktrees (for clean benchmark)
go run ./cmd/benchmark setup --files=1000 --worktrees=10 --merged /tmp/twig-bench
```

## Scale Settings

| Scale  | Files  | Commits | Worktrees |
|--------|--------|---------|-----------|
| small  | 1,000  | 100     | 10        |
| medium | 5,000  | 500     | 50        |
| large  | 10,000 | 1,000   | 100       |

### Custom Scale Options

Override preset values with flags:

| Flag          | Description                              |
|---------------|------------------------------------------|
| `--files`     | Override number of files (0 = default)   |
| `--commits`   | Override number of commits (0 = default) |
| `--worktrees` | Override number of worktrees (0 = default)|

The scale argument becomes optional when using custom flags (defaults to
`small`).

## Benchmarked Commands

| Command | Setup | Measurement |
|---------|-------|-------------|
| `list`  | Normal worktrees | `twig list` |
| `add`   | Normal worktrees | `twig add bench/bench-test` |
| `remove`| Create worktree first | `twig remove bench/bench-test` |
| `clean` | Merged worktrees | `twig clean --yes` |

## Understanding Results

hyperfine outputs statistics including:

- **Mean**: Average execution time
- **Min/Max**: Fastest and slowest runs
- **Relative**: Comparison between commands (when running multiple)

Example output:

```text
Benchmark 1: twig list -C /tmp/twig-bench/main
  Time (mean +/- std):      11.4 ms +/-  0.6 ms    [User: 5.5 ms, System: 5.3 ms]
  Range (min ... max):      10.3 ms ... 13.2 ms    20 runs
```

## Output Directory

Benchmarks use `/tmp/twig-bench` as the working directory. This is
automatically cleaned up after benchmarks complete.

## Tips

### Reduce Noise

For more accurate results:

- Close other applications
- Disable CPU throttling if possible
- Run multiple times and compare

### Export Results

hyperfine supports various export formats:

```bash
hyperfine --export-json results.json 'twig list -C /tmp/twig-bench/main'
hyperfine --export-markdown results.md 'twig list -C /tmp/twig-bench/main'
```

# version subcommand

Print version information.

## Usage

```txt
twig version
twig --version
```

## Behavior

- `twig version`: Displays version, commit hash, and build date
- `twig --version`: Displays version only
- Version is embedded at build time via ldflags
- Makefile builds show `<latest-tag>-dev` (e.g., `0.7.0-dev`)
- Direct `go build` without ldflags shows `dev`

## Examples

```txt
# Detailed output (subcommand)
twig version
version: 1.0.0
commit:  abc1234
date:    2025-01-06T12:00:00Z

# Short output (flag)
twig --version
1.0.0

# Local development build (via Makefile)
twig version
version: 0.7.0-dev
commit:  def5678
date:    2025-01-06T10:30:00Z
```

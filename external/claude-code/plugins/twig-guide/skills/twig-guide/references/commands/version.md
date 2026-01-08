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
- Local builds show "dev" as the version

## Examples

```txt
# Detailed output (subcommand)
twig version
version: v1.0.0
commit:  abc1234
date:    2025-01-06T12:00:00Z

# Short output (flag)
twig --version
v1.0.0

# Local development build
twig version
version: dev
commit:  def5678
date:    2025-01-06T10:30:00Z
```

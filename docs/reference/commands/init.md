# init subcommand

Initialize twig configuration in the current directory.

## Usage

```txt
twig init [flags]
```

## Flags

| Flag      | Short | Description                        |
|-----------|-------|------------------------------------|
| `--force` | `-f`  | Overwrite existing configuration   |

## Behavior

- Creates `.twig/` directory if it doesn't exist
- Generates `.twig/settings.toml` with default configuration template
- If `settings.toml` already exists, skips creation (unless `--force` is used)

See [Configuration](../configuration.md) for available settings.

## Examples

```txt
# Initialize twig in current directory
twig init
Created .twig/settings.toml

# Running again without force skips
twig init
Skipped .twig/settings.toml (already exists)

# Force overwrite existing configuration
twig init --force
Created .twig/settings.toml (overwritten)
```

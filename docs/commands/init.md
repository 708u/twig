# init subcommand

Initialize gwt configuration in the current directory.

## Usage

```txt
gwt init [flags]
```

## Flags

| Flag      | Short | Description                        |
|-----------|-------|------------------------------------|
| `--force` | `-f`  | Overwrite existing configuration   |

## Behavior

- Creates `.gwt/` directory if it doesn't exist
- Generates `.gwt/settings.toml` with default configuration template
- If `settings.toml` already exists, skips creation (unless `--force` is used)

See [Configuration](../configuration.md) for available settings.

## Examples

```txt
# Initialize gwt in current directory
gwt init
Created .gwt/settings.toml

# Running again without force skips
gwt init
Skipped .gwt/settings.toml (already exists)

# Force overwrite existing configuration
gwt init --force
Created .gwt/settings.toml (overwritten)
```

## Exit Code

- 0: Configuration created or skipped successfully
- 1: Failed to create configuration

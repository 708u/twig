# Configuration

gwt reads configuration from TOML files in the `.gwt/` directory.

## Files

| File                         | Purpose                                      |
|------------------------------|----------------------------------------------|
| `.gwt/settings.toml`         | Project-level settings (commit to repository)|
| `.gwt/settings.local.toml`   | Local settings (add to .gitignore)           |

## Fields

### symlinks

Glob patterns for files to symlink from source worktree to new worktrees.

```toml
symlinks = [".envrc", "config/**/*.toml"]
```

### extra_symlinks

Additional symlink patterns. Collected from both project and local configs.

```toml
extra_symlinks = [".tool-versions", ".claude"]
```

### worktree_destination_base_dir

Base directory where new worktrees are created.

```toml
worktree_destination_base_dir = "/path/to/worktrees"
```

Default: `<repo-name>-worktree` sibling directory.

### worktree_source_dir

Source directory for symlinks and worktree operations.

```toml
worktree_source_dir = "/path/to/main/worktree"
```

Default: Directory where config is loaded from.

### default_source

Default branch to use as source when creating new worktrees.

```toml
default_source = "main"
```

See [add subcommand](commands/add.md#default-source-configuration) for details.

## Merge Rules

When both files exist, settings are merged:

| Field                           | Behavior                 |
|---------------------------------|--------------------------|
| `symlinks`                      | Local overrides project  |
| `extra_symlinks`                | Collected from both      |
| `worktree_destination_base_dir` | Local overrides project  |
| `worktree_source_dir`           | Local overrides project  |
| `default_source`                | Local overrides project  |

## symlinks vs extra_symlinks

Use `symlinks` for base patterns shared with the team.
Use `extra_symlinks` to add personal patterns without overriding the base.

Example:

```toml
# .gwt/settings.toml (shared)
symlinks = [".envrc", "config/**"]
```

```toml
# .gwt/settings.local.toml (personal)
extra_symlinks = [".tool-versions"]
```

Result: `.envrc`, `config/**`, `.tool-versions` are all symlinked.

To completely replace project symlinks locally:

```toml
# .gwt/settings.local.toml
symlinks = [".my-envrc"]
```

Result: Only `.my-envrc` is symlinked (project symlinks ignored).

## Example Configuration

```toml
# .gwt/settings.toml
worktree_source_dir = "/Users/dev/projects/myapp"
worktree_destination_base_dir = "/Users/dev/projects/myapp-worktree"
symlinks = [".envrc", ".tool-versions", "config/**"]
default_source = "main"
```

```toml
# .gwt/settings.local.toml
extra_symlinks = [".claude", ".local-config"]
default_source = "develop"
```

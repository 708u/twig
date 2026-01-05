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
worktree_destination_base_dir = "/Users/dev/projects/myapp-worktree"
symlinks = [".envrc", ".tool-versions", "config/**"]
default_source = "main"
```

```toml
# .gwt/settings.local.toml
extra_symlinks = [".claude", ".local-config"]
default_source = "develop"
```

## Recommended: Setting default_source

Setting `default_source` ensures symlinks are always created from the same
worktree (e.g., main branch), preventing symlink chaining when creating
worktrees from derived branches.

Without `default_source`, symlinks are created from the current worktree.
For example, if you create `feat/api` from `main`, then `feat/api-v2` from
`feat/api`, the symlinks chain: `feat/api-v2 -> feat/api -> main`.
With `default_source = "main"`, symlinks always point directly to main.

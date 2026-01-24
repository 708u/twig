# Release Command

Create a new release for twig.

## Prerequisites

- Working directory must be clean (no uncommitted changes)
- Must be on the main branch

## Procedure

### 1. Check current state

```bash
# Latest tag
git tag --sort=-v:refname | head -1

# Commits since last tag
git log $(git tag --sort=-v:refname | head -1)..HEAD --oneline
```

### 2. Determine version bump

Based on conventional commits since last tag:

| Commit Type    | Version Bump | Changelog Group  |
|----------------|--------------|------------------|
|  ! (breaking)  | MAJOR        | Breaking Changes |
| `feat`         | MINOR        | New Features     |
| `fix`          | PATCH        | Bug Fixes        |
| `perf`         | PATCH        | Performance      |
| `docs`         | -            | Documentation    |
| `build`        | PATCH        | Other Changes    |
| `chore`        | PATCH        | Other Changes    |
| `refactor`     | PATCH        | Other Changes    |
| `ci`           | -            | Other Changes    |
| `style`        | -            | Other Changes    |
| `test`         | -            | Other Changes    |

#### Judgment criteria

- **Breaking change ('!')**: CLI flags/arguments or config file format changes

### 3. Create and push tag

```bash
git tag v<version>
git push origin v<version>
```

### 4. Wait for goreleaser

The goreleaser GitHub Action triggers automatically on tag push.

```bash
gh run list --limit 1
gh run watch <run-id>
```

### 5. Verify release

```bash
gh release view v<version>
```

## Important

- **DO NOT** use `gh release create` - goreleaser creates the release
  automatically
- Once a release is created for a tag, that tag becomes **immutable** and cannot
  be reused even after deleting the release
- If goreleaser fails before creating a release, you can re-trigger by deleting
  and re-pushing the tag
- If a release was already created (even partially), you must use a new version
  number (e.g., v0.10.1 failed → use v0.10.2)

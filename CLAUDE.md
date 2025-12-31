# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gwt is a Go project.

## Common Commands

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run a single test
go test -v ./path/to/package -run TestFunctionName

# Build the project
go build ./...

# Run linter (if golangci-lint is installed)
golangci-lint run

# Format code
go fmt ./...

# Tidy dependencies
go mod tidy
```

## Code Style

- Follow standard Go conventions and idioms
- Use `gofmt` for formatting
- Error handling: return errors rather than panicking
- Naming: use camelCase for unexported, PascalCase for exported identifiers

## Git Commit

- Use Conventional Commits format
- Example: `feat: add new feature`, `fix: resolve bug`, `docs: update README`

package twig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

// CLIHandler is a slog.Handler that outputs plain text for CLI usage.
// It formats log messages as "[category] message" where category is
// derived from the "category" attribute or the log level name.
type CLIHandler struct {
	w     io.Writer
	level slog.Level
	mu    sync.Mutex
}

// NewCLIHandler creates a new CLIHandler that writes to w.
// Only messages at or above the specified level are output.
func NewCLIHandler(w io.Writer, level slog.Level) *CLIHandler {
	return &CLIHandler{w: w, level: level}
}

// Enabled reports whether the handler handles records at the given level.
func (h *CLIHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle writes a log record to the handler's writer.
// Format: [category] message
func (h *CLIHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Get category from attributes, fallback to level name
	category := strings.ToLower(r.Level.String())
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "category" {
			category = a.Value.String()
			return false
		}
		return true
	})

	_, err := fmt.Fprintf(h.w, "[%s] %s\n", category, r.Message)
	return err
}

// WithAttrs returns a new handler with the given attributes.
// This implementation returns the same handler for simplicity.
func (h *CLIHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

// WithGroup returns a new handler with the given group name.
// This implementation returns the same handler for simplicity.
func (h *CLIHandler) WithGroup(_ string) slog.Handler {
	return h
}

// NewNopLogger creates a logger that discards all output.
// Useful for tests that don't need to verify log output.
func NewNopLogger() *slog.Logger {
	return slog.New(NewCLIHandler(io.Discard, slog.LevelError+1))
}

// VerbosityToLevel converts a verbosity count to a slog.Level.
//
//	0 (no flag): LevelWarn - errors and warnings only
//	1 (-v):      LevelInfo - detailed results
//	2+ (-vv):    LevelDebug - trace output
func VerbosityToLevel(verbosity int) slog.Level {
	switch {
	case verbosity >= 2:
		return slog.LevelDebug
	case verbosity == 1:
		return slog.LevelInfo
	default:
		return slog.LevelWarn
	}
}

// Log categories for consistent output prefixes.
const (
	LogCategoryDebug  = "debug"
	LogCategoryGit    = "git"
	LogCategoryConfig = "config"
	LogCategoryGlob   = "glob"
)

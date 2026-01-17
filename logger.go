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
// Format: "2006-01-02 15:04:05 [LEVEL] category: message"
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
func (h *CLIHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle writes a log record to the handler's writer.
// Format: 2006-01-02 15:04:05 [LEVEL] category: message
func (h *CLIHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	timestamp := r.Time.Format("2006-01-02 15:04:05")
	level := strings.ToUpper(r.Level.String())

	// Get category from attributes
	var category string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "category" {
			category = a.Value.String()
			return false
		}
		return true
	})

	if category != "" {
		_, err := fmt.Fprintf(h.w, "%s [%s] %s: %s\n", timestamp, level, category, r.Message)
		return err
	}
	_, err := fmt.Fprintf(h.w, "%s [%s] %s\n", timestamp, level, r.Message)
	return err
}

// WithAttrs returns a new handler with the given attributes.
// Currently not implemented: attrs are ignored. Use Handle's attrs instead.
func (h *CLIHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

// WithGroup returns a new handler with the given group name.
// Currently not implemented: group is ignored.
func (h *CLIHandler) WithGroup(_ string) slog.Handler {
	return h
}

// NewNopLogger creates a logger that discards all output.
// Used as the default logger when no logging is needed.
func NewNopLogger() *slog.Logger {
	// LevelError+1 sets threshold above all log levels, filtering everything
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
	LogCategoryRemove = "remove"
)

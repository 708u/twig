package twig

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
)

// CLIHandler is a slog.Handler that outputs plain text for CLI usage.
// Format: "2006-01-02 15:04:05.000 [LEVEL] [cmd_id] category: message"
type CLIHandler struct {
	w     io.Writer
	level slog.Level
	attrs []slog.Attr
	cmdID string
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
// Format: 2006-01-02 15:04:05.000 [LEVEL] [cmd_id] category: message
func (h *CLIHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	timestamp := r.Time.Format("2006-01-02 15:04:05.000")
	level := strings.ToUpper(r.Level.String())

	// Get category from record attributes (takes precedence over handler attrs)
	var category string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == LogAttrKeyCategory.String() {
			category = a.Value.String()
			return false
		}
		return true
	})

	// Fall back to handler's stored attrs if not found in record
	if category == "" {
		for _, a := range h.attrs {
			if a.Key == LogAttrKeyCategory.String() {
				category = a.Value.String()
				break
			}
		}
	}

	// Build output with optional cmd_id
	var sb strings.Builder
	sb.WriteString(timestamp)
	sb.WriteString(" [")
	sb.WriteString(level)
	sb.WriteString("]")

	if h.cmdID != "" {
		sb.WriteString(" [")
		sb.WriteString(h.cmdID)
		sb.WriteString("]")
	}

	if category != "" {
		sb.WriteString(" ")
		sb.WriteString(category)
		sb.WriteString(": ")
	} else {
		sb.WriteString(" ")
	}
	sb.WriteString(r.Message)
	sb.WriteString("\n")

	_, err := io.WriteString(h.w, sb.String())
	return err
}

// WithAttrs returns a new handler with the given attributes.
// The cmd_id attribute is stored separately for efficient access.
func (h *CLIHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandler := &CLIHandler{
		w:     h.w,
		level: h.level,
		attrs: slices.Clone(h.attrs),
		cmdID: h.cmdID,
	}
	for _, a := range attrs {
		if a.Key == LogAttrKeyCmdID.String() {
			newHandler.cmdID = a.Value.String()
		} else {
			newHandler.attrs = append(newHandler.attrs, a)
		}
	}
	return newHandler
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

// LogAttrKey is a type-safe key for slog attributes.
type LogAttrKey string

// String returns the string value of the key.
func (k LogAttrKey) String() string {
	return string(k)
}

// Attr creates a slog.Attr with this key and the given value.
func (k LogAttrKey) Attr(value string) slog.Attr {
	return slog.String(string(k), value)
}

// Log attribute keys for slog records.
const (
	LogAttrKeyCategory LogAttrKey = "category"
	LogAttrKeyCmdID    LogAttrKey = "cmd_id"
)

// Log category values for consistent output prefixes.
const (
	LogCategoryDebug  = "debug"
	LogCategoryGit    = "git"
	LogCategoryConfig = "config"
	LogCategoryGlob   = "glob"
	LogCategoryRemove = "remove"
	LogCategoryClean  = "clean"
	LogCategorySync   = "sync"
)

// Command ID generation settings.
const (
	// DefaultCommandIDBytes is the number of random bytes for command ID generation.
	// This produces an 8-character hex string (4 bytes = 8 hex chars).
	DefaultCommandIDBytes = 4
)

// GenerateCommandID generates a random command ID for log grouping.
// Returns an 8-character hex string (e.g., "a1b2c3d4").
func GenerateCommandID() string {
	return GenerateCommandIDWithLength(DefaultCommandIDBytes)
}

// GenerateCommandIDWithLength generates a command ID with the specified byte length.
// The returned string is hex-encoded, so it has 2*byteLen characters.
func GenerateCommandIDWithLength(byteLen int) string {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is extremely rare (system-level issue).
		// Return empty to skip cmd_id in log output.
		return ""
	}
	return hex.EncodeToString(b)
}

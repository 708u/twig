package twig

import (
	"bytes"
	"log/slog"
	"testing"
	"time"
)

func TestCLIHandler_Handle(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 1, 17, 12, 34, 56, 0, time.UTC)

	tests := []struct {
		name     string
		level    slog.Level
		logLevel slog.Level
		message  string
		category string
		want     string
	}{
		{
			name:     "debug level with category",
			level:    slog.LevelDebug,
			logLevel: slog.LevelDebug,
			message:  "checking branch",
			category: "debug",
			want:     "2026-01-17 12:34:56 [DEBUG] debug: checking branch\n",
		},
		{
			name:     "debug level with git category",
			level:    slog.LevelDebug,
			logLevel: slog.LevelDebug,
			message:  "worktree add -b feat/new",
			category: "git",
			want:     "2026-01-17 12:34:56 [DEBUG] git: worktree add -b feat/new\n",
		},
		{
			name:     "info level without category",
			level:    slog.LevelDebug,
			logLevel: slog.LevelInfo,
			message:  "operation complete",
			category: "",
			want:     "2026-01-17 12:34:56 [INFO] operation complete\n",
		},
		{
			name:     "warn level without category",
			level:    slog.LevelDebug,
			logLevel: slog.LevelWarn,
			message:  "something happened",
			category: "",
			want:     "2026-01-17 12:34:56 [WARN] something happened\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			handler := NewCLIHandler(&buf, tt.level)

			record := slog.NewRecord(fixedTime, tt.logLevel, tt.message, 0)
			if tt.category != "" {
				record.AddAttrs(LogAttrKeyCategory.Attr(tt.category))
			}

			if err := handler.Handle(t.Context(), record); err != nil {
				t.Fatalf("Handle() error: %v", err)
			}

			if got := buf.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCLIHandler_Enabled(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		handlerLevel slog.Level
		logLevel     slog.Level
		want         bool
	}{
		{
			name:         "debug handler enables debug",
			handlerLevel: slog.LevelDebug,
			logLevel:     slog.LevelDebug,
			want:         true,
		},
		{
			name:         "debug handler enables info",
			handlerLevel: slog.LevelDebug,
			logLevel:     slog.LevelInfo,
			want:         true,
		},
		{
			name:         "info handler disables debug",
			handlerLevel: slog.LevelInfo,
			logLevel:     slog.LevelDebug,
			want:         false,
		},
		{
			name:         "warn handler disables info",
			handlerLevel: slog.LevelWarn,
			logLevel:     slog.LevelInfo,
			want:         false,
		},
		{
			name:         "warn handler enables warn",
			handlerLevel: slog.LevelWarn,
			logLevel:     slog.LevelWarn,
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := NewCLIHandler(nil, tt.handlerLevel)
			got := handler.Enabled(t.Context(), tt.logLevel)
			if got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCLIHandler_LevelFiltering(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		handlerLevel slog.Level
		logLevel     slog.Level
		wantOutput   bool
	}{
		{
			name:         "debug message output when handler at debug",
			handlerLevel: slog.LevelDebug,
			logLevel:     slog.LevelDebug,
			wantOutput:   true,
		},
		{
			name:         "debug message filtered when handler at info",
			handlerLevel: slog.LevelInfo,
			logLevel:     slog.LevelDebug,
			wantOutput:   false,
		},
		{
			name:         "info message filtered when handler at warn",
			handlerLevel: slog.LevelWarn,
			logLevel:     slog.LevelInfo,
			wantOutput:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			handler := NewCLIHandler(&buf, tt.handlerLevel)
			logger := slog.New(handler)

			logger.Log(t.Context(), tt.logLevel, "test message")

			hasOutput := buf.Len() > 0
			if hasOutput != tt.wantOutput {
				t.Errorf("hasOutput = %v, want %v (buf: %q)", hasOutput, tt.wantOutput, buf.String())
			}
		})
	}
}

func TestVerbosityToLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		verbosity int
		want      slog.Level
	}{
		{0, slog.LevelWarn},
		{1, slog.LevelInfo},
		{2, slog.LevelDebug},
		{3, slog.LevelDebug}, // -vvv treated same as -vv
	}

	for _, tt := range tests {
		got := VerbosityToLevel(tt.verbosity)
		if got != tt.want {
			t.Errorf("VerbosityToLevel(%d) = %v, want %v", tt.verbosity, got, tt.want)
		}
	}
}

func TestNewNopLogger(t *testing.T) {
	t.Parallel()

	logger := NewNopLogger()
	if logger == nil {
		t.Fatal("NewNopLogger() returned nil")
	}

	// Should not panic when logging
	logger.Debug("test debug")
	logger.Info("test info")
	logger.Warn("test warn")
	logger.Error("test error")
}

func TestCLIHandler_WithAttrs(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 1, 17, 12, 34, 56, 0, time.UTC)

	tests := []struct {
		name    string
		attrs   []slog.Attr
		message string
		want    string
	}{
		{
			name:    "with category attr",
			attrs:   []slog.Attr{LogAttrKeyCategory.Attr("git")},
			message: "test message",
			want:    "2026-01-17 12:34:56 [DEBUG] git: test message\n",
		},
		{
			name:    "with cmd_id attr",
			attrs:   []slog.Attr{LogAttrKeyCmdID.Attr("a1b2c3d4")},
			message: "test message",
			want:    "2026-01-17 12:34:56 [DEBUG] [a1b2c3d4] test message\n",
		},
		{
			name: "with both cmd_id and category",
			attrs: []slog.Attr{
				LogAttrKeyCmdID.Attr("a1b2c3d4"),
				LogAttrKeyCategory.Attr("git"),
			},
			message: "test message",
			want:    "2026-01-17 12:34:56 [DEBUG] [a1b2c3d4] git: test message\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			handler := NewCLIHandler(&buf, slog.LevelDebug)
			handlerWithAttrs := handler.WithAttrs(tt.attrs)

			record := slog.NewRecord(fixedTime, slog.LevelDebug, tt.message, 0)

			if err := handlerWithAttrs.Handle(t.Context(), record); err != nil {
				t.Fatalf("Handle() error: %v", err)
			}

			if got := buf.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCLIHandler_WithAttrs_Chained(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 1, 17, 12, 34, 56, 0, time.UTC)

	var buf bytes.Buffer
	handler := NewCLIHandler(&buf, slog.LevelDebug)

	// Chain WithAttrs calls
	h1 := handler.WithAttrs([]slog.Attr{LogAttrKeyCmdID.Attr("a1b2c3d4")})
	h2 := h1.WithAttrs([]slog.Attr{LogAttrKeyCategory.Attr("git")})

	record := slog.NewRecord(fixedTime, slog.LevelDebug, "test message", 0)

	if err := h2.Handle(t.Context(), record); err != nil {
		t.Fatalf("Handle() error: %v", err)
	}

	want := "2026-01-17 12:34:56 [DEBUG] [a1b2c3d4] git: test message\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCLIHandler_RecordAttrsOverrideHandlerAttrs(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 1, 17, 12, 34, 56, 0, time.UTC)

	var buf bytes.Buffer
	handler := NewCLIHandler(&buf, slog.LevelDebug)
	handlerWithAttrs := handler.WithAttrs([]slog.Attr{LogAttrKeyCategory.Attr("config")})

	record := slog.NewRecord(fixedTime, slog.LevelDebug, "test message", 0)
	// Record attribute should override handler attribute
	record.AddAttrs(LogAttrKeyCategory.Attr("git"))

	if err := handlerWithAttrs.Handle(t.Context(), record); err != nil {
		t.Fatalf("Handle() error: %v", err)
	}

	want := "2026-01-17 12:34:56 [DEBUG] git: test message\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateCommandID(t *testing.T) {
	t.Parallel()

	id := GenerateCommandID()

	// Should be 8 hex characters
	if len(id) != 8 {
		t.Errorf("GenerateCommandID() length = %d, want 8", len(id))
	}

	// Should be valid hex
	for _, c := range id {
		isDigit := c >= '0' && c <= '9'
		isHexLower := c >= 'a' && c <= 'f'
		if !isDigit && !isHexLower {
			t.Errorf("GenerateCommandID() contains non-hex character: %c", c)
		}
	}
}

func TestGenerateCommandIDWithLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		byteLen int
		wantLen int
	}{
		{2, 4},
		{4, 8},
		{8, 16},
	}

	for _, tt := range tests {
		id := GenerateCommandIDWithLength(tt.byteLen)
		if len(id) != tt.wantLen {
			t.Errorf("GenerateCommandIDWithLength(%d) length = %d, want %d",
				tt.byteLen, len(id), tt.wantLen)
		}
	}
}

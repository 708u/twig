package twig

import (
	"bytes"
	"log/slog"
	"testing"
	"time"
)

func TestCLIHandler_Handle(t *testing.T) {
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
			var buf bytes.Buffer
			handler := NewCLIHandler(&buf, tt.level)

			record := slog.NewRecord(fixedTime, tt.logLevel, tt.message, 0)
			if tt.category != "" {
				record.AddAttrs(slog.String("category", tt.category))
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
			handler := NewCLIHandler(nil, tt.handlerLevel)
			got := handler.Enabled(t.Context(), tt.logLevel)
			if got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCLIHandler_LevelFiltering(t *testing.T) {
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

package twig

import (
	"strings"
	"testing"
)

func TestColorMode_Enabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mode     ColorMode
		detected bool
		want     bool
	}{
		{
			name:     "always_enables_color_without_tty",
			mode:     ColorModeAlways,
			detected: false,
			want:     true,
		},
		{
			name:     "never_disables_color_with_tty",
			mode:     ColorModeNever,
			detected: true,
			want:     false,
		},
		{
			name:     "auto_follows_detection_enabled",
			mode:     ColorModeAuto,
			detected: true,
			want:     true,
		},
		{
			name:     "auto_follows_detection_disabled",
			mode:     ColorModeAuto,
			detected: false,
			want:     false,
		},
		{
			name:     "unknown_mode_follows_detection",
			mode:     ColorMode("unknown"),
			detected: true,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.mode.enabled(tt.detected); got != tt.want {
				t.Errorf("enabled(%v) = %v, want %v", tt.detected, got, tt.want)
			}
		})
	}
}

func TestColorFunctions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fn   func(a ...any) string
		text string
	}{
		{"colorClean", colorClean, "clean:"},
		{"colorSkip", colorSkip, "skip:"},
		{"colorSuccess", colorSuccess, "✓"},
		{"colorFailure", colorFailure, "✗"},
		{"colorReason", colorReason, "(merged)"},
		{"colorError", colorError, "error:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.fn(tt.text)
			// Color functions add ANSI codes regardless of color.NoColor
			if result == tt.text {
				t.Errorf("%s should add ANSI codes", tt.name)
			}
			if !strings.Contains(result, tt.text) {
				t.Errorf("%s result should contain original text", tt.name)
			}
		})
	}
}

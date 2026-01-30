package twig

import (
	"testing"

	"github.com/fatih/color"
)

func TestSetColorMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mode        ColorMode
		wantNoColor bool
	}{
		{
			name:        "always_enables_color",
			mode:        ColorModeAlways,
			wantNoColor: false,
		},
		{
			name:        "never_disables_color",
			mode:        ColorModeNever,
			wantNoColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original state
			original := color.NoColor
			defer func() { color.NoColor = original }()

			SetColorMode(tt.mode)

			if color.NoColor != tt.wantNoColor {
				t.Errorf("color.NoColor = %v, want %v", color.NoColor, tt.wantNoColor)
			}
		})
	}
}

func TestIsColorEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		noColor  bool
		wantBool bool
	}{
		{
			name:     "returns_true_when_color_enabled",
			noColor:  false,
			wantBool: true,
		},
		{
			name:     "returns_false_when_color_disabled",
			noColor:  true,
			wantBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original state
			original := color.NoColor
			defer func() { color.NoColor = original }()

			color.NoColor = tt.noColor
			got := IsColorEnabled()

			if got != tt.wantBool {
				t.Errorf("IsColorEnabled() = %v, want %v", got, tt.wantBool)
			}
		})
	}
}

func TestColorFunctions(t *testing.T) {
	// Save original state
	original := color.NoColor
	defer func() { color.NoColor = original }()

	// Test with color enabled
	color.NoColor = false

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
		t.Run(tt.name+"_with_color", func(t *testing.T) {
			result := tt.fn(tt.text)
			// With color enabled, result should contain ANSI codes
			if result == tt.text {
				t.Errorf("%s should add ANSI codes when color is enabled", tt.name)
			}
			// Result should still contain the original text
			if len(result) < len(tt.text) {
				t.Errorf("%s result should contain original text", tt.name)
			}
		})
	}

	// Test with color disabled
	color.NoColor = true

	for _, tt := range tests {
		t.Run(tt.name+"_without_color", func(t *testing.T) {
			result := tt.fn(tt.text)
			// With color disabled, result should equal original text
			if result != tt.text {
				t.Errorf("%s() = %q, want %q when color disabled", tt.name, result, tt.text)
			}
		})
	}
}

package twig

import "github.com/fatih/color"

// ColorMode defines color output behavior.
type ColorMode string

const (
	ColorModeAuto   ColorMode = "auto"   // Color when TTY
	ColorModeAlways ColorMode = "always" // Always color
	ColorModeNever  ColorMode = "never"  // No color
)

var (
	// Section headers
	colorClean = color.New(color.FgGreen, color.Bold).SprintFunc()
	colorSkip  = color.New(color.FgYellow, color.Bold).SprintFunc()

	// Status markers
	colorSuccess = color.New(color.FgGreen).SprintFunc() // ✓
	colorFailure = color.New(color.FgRed).SprintFunc()   // ✗

	// Reasons
	colorReason = color.New(color.FgHiBlack).SprintFunc() // (merged)

	// Errors
	colorError = color.New(color.FgRed).SprintFunc()
)

// SetColorMode configures color output based on mode.
func SetColorMode(mode ColorMode) {
	switch mode {
	case ColorModeAlways:
		color.NoColor = false
	case ColorModeNever:
		color.NoColor = true
	case ColorModeAuto:
		// Use fatih/color default behavior (TTY detection)
	}
}

// IsColorEnabled returns whether color output is enabled.
// This should be called after SetColorMode.
func IsColorEnabled() bool {
	return !color.NoColor
}

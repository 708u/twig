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
	colorClean = newColorFunc(color.FgGreen, color.Bold)
	colorSkip  = newColorFunc(color.FgYellow, color.Bold)

	// Status markers
	colorSuccess = newColorFunc(color.FgGreen) // ✓
	colorFailure = newColorFunc(color.FgRed)   // ✗

	// Reasons
	colorReason = newColorFunc(color.FgHiBlack) // (merged)

	// Errors
	colorError = newColorFunc(color.FgRed)
)

// newColorFunc ignores the global color.NoColor, so FormatOptions.ColorEnabled
// alone decides whether output is colored.
func newColorFunc(attrs ...color.Attribute) func(a ...any) string {
	c := color.New(attrs...)
	c.EnableColor()
	return c.SprintFunc()
}

// Enabled reports whether output should be colored in mode. ColorModeAuto
// follows fatih/color's detection of NO_COLOR, TERM=dumb, and whether stdout
// is a terminal.
func (m ColorMode) Enabled() bool {
	return m.enabled(!color.NoColor)
}

func (m ColorMode) enabled(detected bool) bool {
	switch m {
	case ColorModeAlways:
		return true
	case ColorModeNever:
		return false
	default:
		return detected
	}
}

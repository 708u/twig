package twig

import (
	"fmt"
	"strings"
)

// FormatOptions configures output formatting.
type FormatOptions struct {
	Quiet        bool
	Verbose      bool
	ColorEnabled bool // Enable color output (--color=auto/always)
}

// FormatResult holds formatted output strings.
type FormatResult struct {
	Stdout string
	Stderr string
}

// Formatter formats command results.
type Formatter interface {
	Format(opts FormatOptions) FormatResult
}

// lineWriter provides indented line writing for formatted output.
type lineWriter struct {
	w *strings.Builder
}

// Line writes a formatted line with the specified indentation level.
// Each level adds 2 spaces of indentation.
func (lw *lineWriter) Line(level int, format string, args ...any) {
	fmt.Fprintf(lw.w, "%s"+format+"\n",
		append([]any{strings.Repeat("  ", level)}, args...)...)
}

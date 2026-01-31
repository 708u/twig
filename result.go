package twig

// FormatOptions configures output formatting.
type FormatOptions struct {
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

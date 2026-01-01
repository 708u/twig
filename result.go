package gwt

// FormatOptions configures output formatting.
type FormatOptions struct {
	Verbose bool
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

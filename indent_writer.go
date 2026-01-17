package twig

import (
	"fmt"
	"io"
	"strings"
)

// IndentWriter provides indented writing with configurable indent levels.
// It wraps an io.Writer and automatically prepends the current indent
// to each line written.
type IndentWriter struct {
	w      io.Writer
	indent string
	level  int
}

// NewIndentWriter creates a new IndentWriter with the given writer and indent string.
// The indent string is repeated for each indent level.
func NewIndentWriter(w io.Writer, indent string) *IndentWriter {
	return &IndentWriter{
		w:      w,
		indent: indent,
		level:  0,
	}
}

// Indent increases the indent level by 1 and returns the writer for chaining.
func (iw *IndentWriter) Indent() *IndentWriter {
	iw.level++
	return iw
}

// Dedent decreases the indent level by 1 and returns the writer for chaining.
// The level cannot go below 0.
func (iw *IndentWriter) Dedent() *IndentWriter {
	if iw.level > 0 {
		iw.level--
	}
	return iw
}

// Level returns the current indent level.
func (iw *IndentWriter) Level() int {
	return iw.level
}

// SetLevel sets the indent level directly and returns the writer for chaining.
func (iw *IndentWriter) SetLevel(level int) *IndentWriter {
	if level >= 0 {
		iw.level = level
	}
	return iw
}

// prefix returns the current indent prefix string.
func (iw *IndentWriter) prefix() string {
	return strings.Repeat(iw.indent, iw.level)
}

// Writef writes a formatted string with the current indent prefix.
// A newline is automatically appended.
func (iw *IndentWriter) Writef(format string, args ...any) {
	fmt.Fprintf(iw.w, iw.prefix()+format+"\n", args...)
}

// Writeln writes a string with the current indent prefix.
// A newline is automatically appended.
func (iw *IndentWriter) Writeln(s string) {
	fmt.Fprintln(iw.w, iw.prefix()+s)
}

// Blankln writes a blank line (no indent prefix).
func (iw *IndentWriter) Blankln() {
	fmt.Fprintln(iw.w)
}

// WriteRaw writes content directly without indent prefix or automatic newline.
// Use this for content that already includes proper formatting.
func (iw *IndentWriter) WriteRaw(b []byte) {
	_, _ = iw.w.Write(b)
}

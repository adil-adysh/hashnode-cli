package output

import (
	"fmt"
	"io"
	"os"
)

var Out io.Writer = os.Stdout

// Info prints an informational message to the user.
func Info(format string, a ...interface{}) {
	fmt.Fprintf(Out, format, a...)
}

// Success prints a success message (keeps formatting consistent).
func Success(format string, a ...interface{}) {
	fmt.Fprintf(Out, format, a...)
}

// Error prints an error message to stderr.
func Error(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

// List prints a list of lines with a prefix.
func List(prefix string, items []string) {
	for _, it := range items {
		fmt.Fprintf(Out, "%s %s\n", prefix, it)
	}
}

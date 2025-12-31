package log

import (
	"fmt"
	"io"
	"os"
)

// Out is the destination for informational logs. Tests can replace this
// to capture logs without writing to the real stdout.
var Out io.Writer = os.Stdout

// Printf writes a formatted informational message to Out.
func Printf(format string, a ...interface{}) {
	fmt.Fprintf(Out, format, a...)
}

// Println writes a line to Out.
func Println(a ...interface{}) {
	fmt.Fprintln(Out, a...)
}

// Warnf writes warnings to stderr by default so they don't get mixed into
// normal program output. Tests can still capture stderr if needed.
func Warnf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

// Warnln writes a warning line to stderr.
func Warnln(a ...interface{}) {
	fmt.Fprintln(os.Stderr, a...)
}

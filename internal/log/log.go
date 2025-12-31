package log

import (
    "fmt"
    "io"
    "os"
)

var Out io.Writer = os.Stdout

func Printf(format string, a ...interface{}) {
    fmt.Fprintf(Out, format, a...)
}

func Println(a ...interface{}) {
    fmt.Fprintln(Out, a...)
}

func Warnf(format string, a ...interface{}) {
    fmt.Fprintf(Out, format, a...)
}

func Warnln(a ...interface{}) {
    fmt.Fprintln(Out, a...)
}

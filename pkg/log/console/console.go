package console

import (
	"fmt"
	"os"
)

// Logger implements Logger interface, display logs on stdout
type Logger struct {
	Quiet bool
}

// Infof displays a formated string, honoring the Quiet config setting
func (l *Logger) Infof(format string, v ...interface{}) {
	if l.Quiet {
		return
	}
	fmt.Printf(format, v...)
}

// Fatal displays a message then exit the program
func (l *Logger) Fatal(v ...interface{}) {
	fmt.Print(v...)
	os.Exit(1)
}

// Fatalf displays a formated string then exit the program
func (l *Logger) Fatalf(format string, v ...interface{}) {
	fmt.Printf(format, v...)
	os.Exit(1)
}

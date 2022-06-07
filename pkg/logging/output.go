package logging

import (
	"fmt"
	"os"
)

func Stderr(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", a...)
}

var DisplayDebug bool

func Debugf(format string, a ...interface{}) {
	if DisplayDebug {
		fmt.Fprintf(os.Stdout, format, a...)
	}
}

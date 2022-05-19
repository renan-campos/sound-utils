package logging

import (
	"fmt"
	"os"
)

func Stderr(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", a...)
}

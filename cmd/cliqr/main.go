package main

import (
	"io"
	"os"

	"paepcke.de/cliqr"
)

func main() {
	switch {
	case isPipe():
		out(cliqr.QR(getPipe()))
	case isOsArgs():
		out(cliqr.QR(getOsArg()))
	default:
		out("[cliqr] [error] no pipe or input parameter found, example: cat /etc/ssh/key | clirq")
		os.Exit(1)
	}
}

//
// LITTLE GENERIC HELPER SECTION
//

// out ...
func out(msg string) {
	os.Stdout.Write([]byte(msg + "\n"))
}

// isPipe ...
func isPipe() bool {
	out, _ := os.Stdin.Stat()
	return out.Mode()&os.ModeCharDevice == 0
}

// getPipe ...
func getPipe() string {
	pipe, err := io.ReadAll(os.Stdin)
	if err != nil {
		out("[cliqr] [error] reading data from pipe")
		os.Exit(1)
	}
	return string(pipe)
}

// isOsArgs ...
func isOsArgs() bool {
	return len(os.Args) > 1
}

// getOsArgs ...
func getOsArg() string {
	return os.Args[1]
}

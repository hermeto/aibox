// Command aibox is the AIBox command-line interface.
//
// This is a placeholder so that Go CI jobs have something to build, lint and test.
// Feature 002 (cli-skeleton) replaces it with the real CLI.
package main

import (
	"fmt"
	"io"
	"os"
)

func run(w io.Writer) int {
	if _, err := fmt.Fprintln(w, "aibox: under construction — see the v1.0 roadmap issue"); err != nil {
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Stdout))
}

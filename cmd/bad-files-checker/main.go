package main

import (
	"fmt"
	"io"
	"os"

	"bad-files-checker/internal/app"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	code, err := app.Run(args, stdout, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
	}
	return code
}

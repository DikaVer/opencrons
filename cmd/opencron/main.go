// Package main is the entry point for the opencron binary.
// It delegates all command-line parsing and execution to [cmd.Execute].
package main

import (
	"os"

	"github.com/DikaVer/opencron/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

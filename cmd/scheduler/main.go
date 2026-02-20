package main

import (
	"os"

	"github.com/dika-maulidal/cli-scheduler/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

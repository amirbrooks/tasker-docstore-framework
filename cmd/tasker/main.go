package main

import (
	"os"

	"github.com/amirbrooks/tasker-docstore-framework/internal/cli"
)

func main() {
	code := cli.Run(os.Args[1:])
	os.Exit(code)
}

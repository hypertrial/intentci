package main

import (
	"os"

	"github.com/hypertrial/intentci/internal/cli"
)

var exitFunc = os.Exit

func main() {
	exitFunc(cli.RunMain(os.Args[1:], os.Stdout, os.Stderr))
}

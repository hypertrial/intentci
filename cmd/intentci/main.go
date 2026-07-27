package main

import (
	"os"

	"github.com/hypertrial/intentci/v2/internal/app"
)

var exitFunc = os.Exit

func main() {
	exitFunc(app.Main(os.Args[1:], os.Stdout, os.Stderr))
}

package initcmd

import (
	"os"
	"path/filepath"
)

var (
	absPath   = filepath.Abs
	mkdirAll  = os.MkdirAll
	writeFile = os.WriteFile
	fileStat  = os.Stat
)

package changespec

import "os"

var (
	mkdirAll  = os.MkdirAll
	writeFile = os.WriteFile
	pathStat  = os.Stat
)

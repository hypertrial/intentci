package exitcode

// Process exit codes from v1.md §17.
const (
	Pass             = 0
	Fail             = 1
	Unproven         = 2
	Uncertain        = 3
	ReviewRequired   = 4
	CompileFailed    = 5
	VerifierError    = 6
	Internal         = 7
	Usage            = 8
	RepairExhausted  = 9
	SecurityBoundary = 10
)

package cli

// ExitError carries a process exit code.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *ExitError) ExitCode() int { return e.Code }

func (e *ExitError) Unwrap() error { return e.Err }

func exitErr(code int, err error) error {
	return &ExitError{Code: code, Err: err}
}

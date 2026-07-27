package counter

// Add is intentionally incorrect before the repair agent runs.
func Add(left, right int) int {
	return left - right
}

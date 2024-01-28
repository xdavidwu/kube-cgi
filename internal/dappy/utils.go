package dappy

func ExitCodeToHttpStatus(c int) int {
	switch {
	case c == 0:
		return 200
	case c > 128: // killed by signal
		return 500
	default:
		return c + 399
	}
}

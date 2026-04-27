package oneshot

import (
	"os"

	"golang.org/x/term"
)

// isStdinTTY reports whether os.Stdin is connected to a terminal.
func isStdinTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

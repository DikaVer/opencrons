// screen.go provides terminal screen utilities.
//
// ClearScreen resets the terminal using ANSI escape codes. IsAborted checks
// whether an error is huh.ErrUserAborted, indicating the user pressed Escape.
package tui

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
)

// ClearScreen clears the terminal and moves the cursor to the top-left.
func ClearScreen() {
	fmt.Print("\033[H\033[2J")
}

// IsAborted returns true if the error is huh.ErrUserAborted (user pressed Escape).
func IsAborted(err error) bool {
	return errors.Is(err, huh.ErrUserAborted)
}

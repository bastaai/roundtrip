package runner

import (
	"os"

	"golang.org/x/sys/windows"
)

// Terminal reports whether f is a console that understands ANSI colors,
// switching that on where the console supports it.
func Terminal(f *os.File) bool {
	h := windows.Handle(f.Fd())
	var mode uint32
	if windows.GetConsoleMode(h, &mode) != nil {
		return false
	}
	return windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}

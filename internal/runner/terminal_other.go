//go:build !windows

package runner

import "os"

// Terminal reports whether f is a terminal that understands ANSI colors.
func Terminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

//go:build darwin
// +build darwin

package evaluator

import "syscall"

func getTermiosGet() uintptr {
	return syscall.TIOCGETA
}

func getTermiosSet() uintptr {
	return syscall.TIOCSETA
}

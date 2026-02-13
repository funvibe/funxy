//go:build freebsd
// +build freebsd

package evaluator

import "syscall"

func getTermiosGet() uintptr {
	return syscall.TIOCGETA
}

func getTermiosSet() uintptr {
	return syscall.TIOCSETA
}

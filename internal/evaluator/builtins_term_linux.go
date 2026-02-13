//go:build linux
// +build linux

package evaluator

import "syscall"

func getTermiosGet() uintptr {
	return uintptr(syscall.TCGETS)
}

func getTermiosSet() uintptr {
	return uintptr(syscall.TCSETS)
}

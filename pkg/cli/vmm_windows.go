//go:build windows

package cli

import (
	"os"
	"os/signal"
	"syscall"
)

func vmmNotifySignals(sigCh chan os.Signal) {
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
}

func vmmIsHotReloadSignal(sig os.Signal) bool {
	return false
}

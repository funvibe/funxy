//go:build !darwin && !linux && !freebsd && !openbsd

package evaluator

import (
	"context"
	"fmt"
	"time"
)

func startTermInputSession() error {
	return fmt.Errorf("lib/termio is supported only on Linux, macOS, FreeBSD, and OpenBSD")
}

func stopTermInputSession() error {
	return nil
}

func waitTermInputEvent(context.Context, time.Duration) (termInputEvent, bool, error) {
	return termInputEvent{}, false, fmt.Errorf("lib/termio is supported only on Linux, macOS, FreeBSD, and OpenBSD")
}

func termInputSessionActive() bool {
	return false
}

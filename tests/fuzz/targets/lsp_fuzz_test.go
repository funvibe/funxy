package targets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/funvibe/funxy/tests/fuzz/generators"
)

// FuzzLSP tests the Language Server Protocol implementation by spawning the server process
// and sending it random JSON-RPC messages.
//
// This test is inherently heavier than other fuzz targets because each iteration
// spawns a real OS process. Three precautions prevent the fuzz worker from crashing:
//  1. Input size is capped to limit message count and program complexity.
//  2. All I/O goroutines are awaited via sync.WaitGroup to prevent accumulation.
//  3. Process group kill ensures clean subprocess teardown on timeout.
func FuzzLSP(f *testing.F) {
	// Add seed corpus
	f.Add([]byte("seed"))
	f.Add([]byte("hover"))
	f.Add([]byte("completion"))

	// Build the LSP server binary once
	lspBin, cleanup := buildLSPBinary(f)
	defer cleanup()

	f.Fuzz(func(t *testing.T, data []byte) {
		// Cap input size to prevent generating too many / too large messages.
		// Large inputs create many didOpen/didChange messages, each containing
		// a generated program that triggers full analysis in the LSP server.
		// Over thousands of iterations this exhausts the fuzz worker's resources.
		if len(data) > 256 {
			return
		}

		// Generate a sequence of LSP messages
		gen := generators.NewLSPGenerator(data)
		messages := gen.GenerateLSPSequence()

		// 5-second timeout for the LSP server process.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, lspBin)
		// WaitDelay ensures cmd.Wait() returns in bounded time even if the
		// process doesn't exit promptly after SIGKILL. Without this, Wait()
		// can block indefinitely, freezing the fuzz worker.
		cmd.WaitDelay = 3 * time.Second
		// Create a new process group so we can kill the server and any children.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error {
			// Kill the entire process group, not just the leader.
			err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			if errors.Is(err, syscall.ESRCH) {
				return os.ErrProcessDone
			}
			return err
		}

		stdin, err := cmd.StdinPipe()
		if err != nil {
			return // Skip on pipe creation failure under heavy load
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return
		}

		if err := cmd.Start(); err != nil {
			return // Process creation can fail under heavy system load
		}

		// WaitGroup ensures all I/O goroutines finish before we return.
		// Without this, goroutines accumulate across the ~30K iterations
		// of a typical fuzz run and eventually crash the worker ("exit status 2").
		var wg sync.WaitGroup

		// Read stdout (discard, but must drain to prevent server blocking)
		wg.Add(1)
		go func() {
			defer wg.Done()
			io.Copy(io.Discard, stdout)
		}()

		// Read stderr — capture limited amount for crash diagnostics.
		// The LSP server logs every received message fully via log.Printf,
		// so unbounded capture can cause OOM over many iterations.
		var stderrBuf bytes.Buffer
		wg.Add(1)
		go func() {
			defer wg.Done()
			limited := io.LimitReader(stderr, 32*1024) // 32KB max
			io.Copy(&stderrBuf, limited)
			io.Copy(io.Discard, stderr) // drain the rest to unblock the server
		}()

		// Write messages to stdin
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer stdin.Close()
			for _, msg := range messages {
				if ctx.Err() != nil {
					return
				}
				_, err := fmt.Fprint(stdin, msg)
				if err != nil {
					return // Server closed connection
				}
			}
		}()

		// Wait for process, then wait for all goroutines.
		err = cmd.Wait()
		wg.Wait()

		if ctx.Err() != nil {
			return // Timeout is acceptable (server was still running)
		}
		if err != nil {
			if errors.Is(err, exec.ErrWaitDelay) {
				return // Process didn't exit in time after kill — acceptable
			}
			if exitErr, ok := err.(*exec.ExitError); ok {
				if exitErr.ExitCode() != 0 {
					t.Fatalf("LSP server crashed with exit code %d.\nStderr (first 32KB):\n%s",
						exitErr.ExitCode(), stderrBuf.String())
				}
			}
			// Other wait errors are acceptable in a fuzzing context
		}
	})
}

// buildLSPBinary builds the lsp command and returns the path to the binary.
// It returns a cleanup function to remove the binary.
func buildLSPBinary(t testing.TB) (string, func()) {
	// Get project root
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	root := findProjectRoot(wd)
	if root == "" {
		t.Fatal("Could not find project root (go.mod)")
	}

	// Create a unique binary path to avoid races between fuzz workers
	pattern := fmt.Sprintf("funxy-lsp-fuzz-%d-*", os.Getpid())
	tmpFile, err := os.CreateTemp("", pattern)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	binPath := tmpFile.Name()
	tmpFile.Close()
	os.Remove(binPath)

	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/lsp")
	cmd.Dir = root

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build LSP binary: %v\n%s", err, out)
	}

	return binPath, func() {
		os.Remove(binPath)
	}
}

func findProjectRoot(start string) string {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

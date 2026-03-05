package funxy

import (
	"bytes"
	"io"
	"sync"
)

// PrefixWriter wraps an io.Writer and prefixes each line with a given string.
// It is thread-safe.
type PrefixWriter struct {
	mu     sync.Mutex
	writer io.Writer
	prefix []byte
	buf    []byte // Buffer for partial lines
}

// NewPrefixWriter creates a new PrefixWriter.
func NewPrefixWriter(w io.Writer, prefix string) *PrefixWriter {
	return &PrefixWriter{
		writer: w,
		prefix: []byte(prefix),
	}
}

// Write implements io.Writer.
func (pw *PrefixWriter) Write(p []byte) (n int, err error) {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	n = len(p)

	// If we have buffered data, prepend it
	if len(pw.buf) > 0 {
		p = append(pw.buf, p...)
		pw.buf = nil
	}

	for len(p) > 0 {
		// Find next newline
		idx := bytes.IndexByte(p, '\n')
		if idx == -1 {
			// No newline, buffer the rest
			pw.buf = append(pw.buf, p...)
			break
		}

		// Write line with prefix
		line := p[:idx+1]
		if _, err := pw.writer.Write(pw.prefix); err != nil {
			return 0, err
		}
		if _, err := pw.writer.Write(line); err != nil {
			return 0, err
		}

		p = p[idx+1:]
	}

	return n, nil
}

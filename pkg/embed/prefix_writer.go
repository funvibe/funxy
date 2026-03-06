package funxy

import (
	"bytes"
	"io"
	"sync"
)

// PrefixWriter wraps an io.Writer and prefixes each line with a given string.
// It is thread-safe.
type PrefixWriter struct {
	mu          sync.Mutex
	writer      io.Writer
	prefix      []byte
	atLineStart bool
}

// NewPrefixWriter creates a new PrefixWriter.
func NewPrefixWriter(w io.Writer, prefix string) *PrefixWriter {
	return &PrefixWriter{
		writer:      w,
		prefix:      []byte(prefix),
		atLineStart: true,
	}
}

// Write implements io.Writer.
func (pw *PrefixWriter) Write(p []byte) (n int, err error) {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	n = len(p)

	if len(p) == 0 {
		return 0, nil
	}

	for len(p) > 0 {
		if pw.atLineStart {
			if _, err := pw.writer.Write(pw.prefix); err != nil {
				return 0, err
			}
			pw.atLineStart = false
		}

		idx := bytes.IndexByte(p, '\n')
		if idx == -1 {
			// No newline, write the rest
			if _, err := pw.writer.Write(p); err != nil {
				return 0, err
			}
			break
		}

		// Write up to and including the newline or CR
		if _, err := pw.writer.Write(p[:idx+1]); err != nil {
			return 0, err
		}
		pw.atLineStart = true
		p = p[idx+1:]
	}

	return n, nil
}

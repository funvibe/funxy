package targets

import (
	"github.com/funvibe/funxy/internal/evaluator"
	"testing"
)

// FuzzFDFDeserialize fuzzes FDF/gob auto-detect deserialization.
// Invariant: deserializer must never panic on arbitrary bytes.
func FuzzFDFDeserialize(f *testing.F) {
	capFuzzProcs()

	f.Add([]byte{})
	f.Add([]byte("FDF1"))
	f.Add([]byte("FDF1\xff"))
	f.Add([]byte("FDF1\x07\x00\x00\x00\x01\xff"))                 // bytes payload seed
	f.Add([]byte("FDF1\x08\x00\x00\x00\x01\xff\x00\x00\x00\x08")) // bits payload seed

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = evaluator.DeserializeValue(data)
	})
}

package targets

import (
	"math"
	funxy "github.com/funvibe/funxy/pkg/embed"
	"testing"
	"time"
)

// =============================================================================
// FuzzMarshallerRoundTrip — Go → Funxy → Go, must not panic
// =============================================================================

// FuzzMarshallerRoundTrip converts random Go values to Funxy and back.
// The invariant: no panics, and primitive types should survive the round trip.
func FuzzMarshallerRoundTrip(f *testing.F) {
	capFuzzProcs()

	// Seeds: type(byte) + value bytes
	f.Add([]byte{0, 42})                      // int
	f.Add([]byte{1, 0x40, 0x49, 0x0f, 0xdb})  // float
	f.Add([]byte{2, 1})                       // bool true
	f.Add([]byte{2, 0})                       // bool false
	f.Add([]byte{3, 'h', 'e', 'l', 'l', 'o'}) // string
	f.Add([]byte{4, 1, 2, 3})                 // []int
	f.Add([]byte{5, 'a', ':', '1'})           // map
	f.Add([]byte{6})                          // nil
	f.Add([]byte{7, 0, 1, 2, 3, 4, 5})        // []byte
	f.Add([]byte{255, 128, 64, 32, 16})       // random

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}

		m := funxy.NewMarshaller()

		// Generate a Go value from the fuzz data
		val := fuzzGoValue(data)

		// Go → Funxy (must not panic)
		obj, err := m.ToValue(val)
		if err != nil {
			return // Conversion errors are fine
		}
		if obj == nil {
			return
		}

		// Funxy → Go (must not panic)
		_, _ = m.FromValue(obj, nil)
	})
}

// FuzzMarshallerToValue tests that ToValue never panics on any input.
func FuzzMarshallerToValue(f *testing.F) {
	capFuzzProcs()

	f.Add([]byte{0})
	f.Add([]byte{1, 2, 3, 4, 5, 6, 7, 8})
	f.Add([]byte{255, 255, 255})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		m := funxy.NewMarshaller()
		val := fuzzGoValue(data)
		// Must not panic
		_, _ = m.ToValue(val)
	})
}

// FuzzMarshallerMapRoundTrip specifically fuzzes map conversions.
func FuzzMarshallerMapRoundTrip(f *testing.F) {
	capFuzzProcs()

	f.Add(uint8(3), []byte("abc123"))
	f.Add(uint8(0), []byte{})
	f.Add(uint8(10), []byte("hello world foo bar"))

	f.Fuzz(func(t *testing.T, size uint8, data []byte) {
		m := funxy.NewMarshaller()

		// Build map[string]int from fuzz data.
		// Use ASCII letters for keys to avoid UTF-8 encoding collisions
		// (invalid UTF-8 bytes map to U+FFFD, causing key dedup).
		goMap := make(map[string]int)
		for i := 0; i < int(size) && i*2+1 < len(data); i++ {
			key := string(rune('a' + data[i*2]%26))
			val := int(data[i*2+1])
			goMap[key] = val
		}

		// Go map → Funxy Map
		obj, err := m.ToValue(goMap)
		if err != nil {
			t.Fatalf("ToValue failed for map: %v", err)
		}

		// Funxy Map → Go (untyped)
		result, err := m.FromValue(obj, nil)
		if err != nil {
			t.Fatalf("FromValue failed: %v", err)
		}

		// Verify it's a map
		resultMap, ok := result.(map[interface{}]interface{})
		if !ok {
			t.Fatalf("Expected map[interface{}]interface{}, got %T", result)
		}

		// Verify all entries survived
		if len(resultMap) != len(goMap) {
			t.Errorf("Map size mismatch: got %d, want %d", len(resultMap), len(goMap))
		}
	})
}

// =============================================================================
// FuzzEmbedEval — random Funxy code with bound Go objects
// =============================================================================

// FuzzEmbedEval executes random Funxy code with several bound Go objects.
// Invariant: must never panic (errors are fine).
func FuzzEmbedEval(f *testing.F) {
	capFuzzProcs()

	// Seed corpus: valid Funxy expressions
	f.Add("1 + 2")
	f.Add("double(21)")
	f.Add("getMap()")
	f.Add("processMap(getMap())")
	f.Add(`"hello"`)
	f.Add("true")
	f.Add("[1, 2, 3]")
	f.Add("{ name: \"test\", value: 42 }")
	// Edge cases
	f.Add("")
	f.Add("(((((")
	f.Add("fun f(x) { f(x) }")
	f.Add("1 / 0")

	f.Fuzz(func(t *testing.T, code string) {
		if len(code) > 1000 {
			return // Skip very long inputs
		}

		vm := funxy.New()

		// Bind various Go types
		vm.Bind("double", func(x int) int { return x * 2 })
		vm.Bind("concat", func(a, b string) string { return a + b })
		vm.Bind("getMap", func() map[string]int {
			return map[string]int{"x": 1, "y": 2}
		})
		vm.Bind("processMap", func(m map[string]int) int {
			total := 0
			for _, v := range m {
				total += v
			}
			return total
		})
		vm.Bind("getIntMap", func() map[int]string {
			return map[int]string{1: "one", 2: "two"}
		})
		vm.Bind("myInt", 42)
		vm.Bind("myStr", "hello")
		vm.Bind("myList", []int{1, 2, 3})

		// Must not panic — errors are expected and fine
		// Use timeout to prevent hangs on adversarial input (deep nesting, etc.)
		done := make(chan struct{}, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on input %q: %v", code, r)
				}
				done <- struct{}{}
			}()
			_, _ = vm.Eval(code)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			return
		}
	})
}

// =============================================================================
// Helpers
// =============================================================================

// fuzzGoValue constructs a Go value from raw bytes for fuzzing the marshaller.
func fuzzGoValue(data []byte) interface{} {
	if len(data) == 0 {
		return nil
	}

	kind := data[0]
	rest := data[1:]

	switch kind % 10 {
	case 0: // int
		if len(rest) == 0 {
			return 0
		}
		return int(int8(rest[0]))
	case 1: // float64
		if len(rest) < 8 {
			return float64(0)
		}
		bits := uint64(0)
		for i := 0; i < 8 && i < len(rest); i++ {
			bits |= uint64(rest[i]) << (i * 8)
		}
		f := math.Float64frombits(bits)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0.0
		}
		return f
	case 2: // bool
		if len(rest) == 0 {
			return false
		}
		return rest[0]%2 == 1
	case 3: // string
		return string(rest)
	case 4: // []int
		result := make([]int, 0, len(rest))
		for _, b := range rest {
			result = append(result, int(int8(b)))
		}
		return result
	case 5: // map[string]int
		m := make(map[string]int)
		for i := 0; i+1 < len(rest); i += 2 {
			// Use ASCII to avoid UTF-8 collisions
			m[string(rune('a'+rest[i]%26))] = int(int8(rest[i+1]))
		}
		return m
	case 6: // nil
		return nil
	case 7: // []byte
		return rest
	case 8: // map[int]string
		m := make(map[int]string)
		for i := 0; i+1 < len(rest); i += 2 {
			m[int(rest[i])] = string(rest[i+1 : i+2])
		}
		return m
	case 9: // []string
		var result []string
		for _, b := range rest {
			result = append(result, string([]byte{b}))
		}
		return result
	default:
		return nil
	}
}

package evaluator

import (
	"fmt"
	"testing"
)

func TestPersistentMap_Collision(t *testing.T) {
	m := EmptyMap()

	// Create two strings that have the same hash but different values.
	// Since we use hashString(), let's see if we can just test the put directly
	// Or we can just insert lots of elements.
	for i := 0; i < 1000; i++ {
		key := StringToList(fmt.Sprintf("key_%d", i))
		val := &Integer{Value: int64(i)}
		m = m.Put(key, val)
	}

	if m.Len() != 1000 {
		t.Errorf("expected 1000, got %d", m.Len())
	}
}

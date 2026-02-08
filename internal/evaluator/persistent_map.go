package evaluator

import (
	"fmt"
	"github.com/funvibe/funxy/internal/typesystem"
	"unsafe"
)

// Persistent Hash Array Mapped Trie (HAMT) implementation
// Provides efficient immutable map operations

const (
	hamtBits = 5
	hamtSize = 1 << hamtBits // 32
	hamtMask = hamtSize - 1
)

// PersistentMap is an immutable hash map
type PersistentMap struct {
	root  *hamtNode
	count int
}

// hamtNode is a node in the HAMT
type hamtNode struct {
	bitmap uint32        // which indices are populated
	nodes  []interface{} // hamtEntry or *hamtNode
}

// hamtEntry holds a key-value pair
type hamtEntry struct {
	hash  uint32
	key   Object
	value Object
}

// EmptyMap returns an empty persistent map
func EmptyMap() *PersistentMap {
	return &PersistentMap{
		root:  nil,
		count: 0,
	}
}

// MapFrom creates a persistent map from key-value pairs
func MapFrom(pairs []struct{ Key, Value Object }) *PersistentMap {
	m := EmptyMap()
	for _, p := range pairs {
		m = m.Put(p.Key, p.Value)
	}
	return m
}

// Implement Object interface for PersistentMap so it can be stored inside itself
func (m *PersistentMap) Type() ObjectType             { return "PERSISTENT_MAP" }
func (m *PersistentMap) Inspect() string              { return fmt.Sprintf("<persistent map %d>", m.count) }
func (m *PersistentMap) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "PersistentMap"} }
func (m *PersistentMap) Hash() uint32                 { return uint32(uintptr(unsafe.Pointer(m))) }

// Len returns the number of entries
func (m *PersistentMap) Len() int {
	return m.count
}

// Get returns the value for a key, or nil if not found
func (m *PersistentMap) Get(key Object) Object {
	if m.root == nil {
		return nil
	}
	hash := hashObject(key)
	return m.root.get(hash, key, 0)
}

// Put returns a new map with the key-value pair added/updated
func (m *PersistentMap) Put(key, value Object) *PersistentMap {
	hash := hashObject(key)

	var newRoot *hamtNode
	var added bool

	if m.root == nil {
		newRoot = &hamtNode{}
		newRoot, added = newRoot.put(hash, key, value, 0)
	} else {
		newRoot, added = m.root.put(hash, key, value, 0)
	}

	newCount := m.count
	if added {
		newCount++
	}

	return &PersistentMap{
		root:  newRoot,
		count: newCount,
	}
}

// Remove returns a new map with the key removed
func (m *PersistentMap) Remove(key Object) *PersistentMap {
	if m.root == nil {
		return m
	}

	hash := hashObject(key)
	newRoot, removed := m.root.remove(hash, key, 0)

	if !removed {
		return m
	}

	return &PersistentMap{
		root:  newRoot,
		count: m.count - 1,
	}
}

// Contains checks if a key exists
func (m *PersistentMap) Contains(key Object) bool {
	return m.Get(key) != nil
}

// Keys returns all keys as a slice
func (m *PersistentMap) Keys() []Object {
	keys := make([]Object, 0, m.count)
	if m.root != nil {
		m.root.collectKeys(&keys)
	}
	return keys
}

// Values returns all values as a slice
func (m *PersistentMap) Values() []Object {
	values := make([]Object, 0, m.count)
	if m.root != nil {
		m.root.collectValues(&values)
	}
	return values
}

// Items returns all key-value pairs
func (m *PersistentMap) Items() []struct{ Key, Value Object } {
	items := make([]struct{ Key, Value Object }, 0, m.count)
	if m.root != nil {
		m.root.collectItems(&items)
	}
	return items
}

// Merge returns a new map with entries from other (other wins on conflict)
func (m *PersistentMap) Merge(other *PersistentMap) *PersistentMap {
	result := m
	for _, item := range other.Items() {
		result = result.Put(item.Key, item.Value)
	}
	return result
}

// --- hamtNode methods ---

func (n *hamtNode) get(hash uint32, key Object, shift uint) Object {
	if shift >= 32 {
		// Collision bucket search
		for _, node := range n.nodes {
			if entry, ok := node.(hamtEntry); ok {
				if objectsEqualForMap(entry.key, key) {
					return entry.value
				}
			}
		}
		return nil
	}

	idx := (hash >> shift) & hamtMask
	bit := uint32(1) << idx

	if n.bitmap&bit == 0 {
		return nil // not present
	}

	pos := popcount(n.bitmap & (bit - 1))
	node := n.nodes[pos]

	switch v := node.(type) {
	case hamtEntry:
		if v.hash == hash && objectsEqualForMap(v.key, key) {
			return v.value
		}
		return nil
	case *hamtNode:
		return v.get(hash, key, shift+hamtBits)
	}

	return nil
}

func (n *hamtNode) put(hash uint32, key, value Object, shift uint) (*hamtNode, bool) {
	// Handle hash collisions (identical hash, different keys)
	// If we exhausted the hash bits, we store multiple entries in a collision bucket.
	if shift >= 32 {
		// Clone node to serve as collision bucket
		newNode := &hamtNode{
			bitmap: n.bitmap,
			nodes:  make([]interface{}, len(n.nodes)),
		}
		copy(newNode.nodes, n.nodes)

		// Check if key exists in the bucket
		for i, node := range newNode.nodes {
			if entry, ok := node.(hamtEntry); ok {
				if objectsEqualForMap(entry.key, key) {
					newNode.nodes[i] = hamtEntry{hash: hash, key: key, value: value}
					return newNode, false
				}
			}
		}

		// Not found, append new entry
		newNode.nodes = append(newNode.nodes, hamtEntry{hash: hash, key: key, value: value})
		return newNode, true
	}

	idx := (hash >> shift) & hamtMask
	bit := uint32(1) << idx

	// Clone node
	newNode := &hamtNode{
		bitmap: n.bitmap,
		nodes:  make([]interface{}, len(n.nodes)),
	}
	copy(newNode.nodes, n.nodes)

	if n.bitmap&bit == 0 {
		// New entry
		newNode.bitmap |= bit
		pos := popcount(newNode.bitmap & (bit - 1))

		// Insert at position
		newNode.nodes = append(newNode.nodes, nil)
		copy(newNode.nodes[pos+1:], newNode.nodes[pos:])
		newNode.nodes[pos] = hamtEntry{hash: hash, key: key, value: value}

		return newNode, true
	}

	pos := popcount(n.bitmap & (bit - 1))
	existing := newNode.nodes[pos]

	switch v := existing.(type) {
	case hamtEntry:
		if v.hash == hash && objectsEqualForMap(v.key, key) {
			// Update existing value
			newNode.nodes[pos] = hamtEntry{hash: hash, key: key, value: value}
			return newNode, false
		}

		// Collision - create child node and push both entries down
		child := &hamtNode{}
		var added1, added2 bool

		// Put existing entry first
		child, added1 = child.put(v.hash, v.key, v.value, shift+hamtBits)

		// Put new entry
		child, added2 = child.put(hash, key, value, shift+hamtBits)

		newNode.nodes[pos] = child
		return newNode, added1 || added2

	case *hamtNode:
		newChild, added := v.put(hash, key, value, shift+hamtBits)
		newNode.nodes[pos] = newChild
		return newNode, added
	}

	return newNode, false
}

func (n *hamtNode) remove(hash uint32, key Object, shift uint) (*hamtNode, bool) {
	if shift >= 32 {
		// Collision bucket remove
		for i, node := range n.nodes {
			if entry, ok := node.(hamtEntry); ok {
				if objectsEqualForMap(entry.key, key) {
					// Remove this entry
					newNode := &hamtNode{
						bitmap: n.bitmap,
						nodes:  make([]interface{}, len(n.nodes)-1),
					}
					copy(newNode.nodes[:i], n.nodes[:i])
					copy(newNode.nodes[i:], n.nodes[i+1:])
					return newNode, true
				}
			}
		}
		return n, false
	}

	idx := (hash >> shift) & hamtMask
	bit := uint32(1) << idx

	if n.bitmap&bit == 0 {
		return n, false // not present
	}

	pos := popcount(n.bitmap & (bit - 1))
	existing := n.nodes[pos]

	switch v := existing.(type) {
	case hamtEntry:
		if v.hash == hash && objectsEqualForMap(v.key, key) {
			// Remove this entry
			newNode := &hamtNode{
				bitmap: n.bitmap &^ bit, // Clear bit
				nodes:  make([]interface{}, len(n.nodes)-1),
			}
			// Copy elements before pos
			copy(newNode.nodes[:pos], n.nodes[:pos])
			// Copy elements after pos
			copy(newNode.nodes[pos:], n.nodes[pos+1:])

			return newNode, true
		}
		return n, false

	case *hamtNode:
		newChild, removed := v.remove(hash, key, shift+hamtBits)
		if !removed {
			return n, false
		}

		// If child became empty, remove it?
		// Or if child has only 1 entry, collapse it?
		// For simplicity, just update the child for now.
		// Optimization: if child has 0 entries, remove it from this node.
		if len(newChild.nodes) == 0 {
			newNode := &hamtNode{
				bitmap: n.bitmap &^ bit,
				nodes:  make([]interface{}, len(n.nodes)-1),
			}
			copy(newNode.nodes[:pos], n.nodes[:pos])
			copy(newNode.nodes[pos:], n.nodes[pos+1:])
			return newNode, true
		}

		// Optimization: if child has 1 entry (and it's a leaf), pull it up?
		// Only if it's an entry, not another node.
		if len(newChild.nodes) == 1 {
			if entry, ok := newChild.nodes[0].(hamtEntry); ok {
				newNode := &hamtNode{
					bitmap: n.bitmap,
					nodes:  make([]interface{}, len(n.nodes)),
				}
				copy(newNode.nodes, n.nodes)
				newNode.nodes[pos] = entry
				return newNode, true
			}
		}

		newNode := &hamtNode{
			bitmap: n.bitmap,
			nodes:  make([]interface{}, len(n.nodes)),
		}
		copy(newNode.nodes, n.nodes)
		newNode.nodes[pos] = newChild
		return newNode, true
	}

	return n, false
}

func (n *hamtNode) collectKeys(keys *[]Object) {
	for _, node := range n.nodes {
		switch v := node.(type) {
		case hamtEntry:
			*keys = append(*keys, v.key)
		case *hamtNode:
			v.collectKeys(keys)
		}
	}
}

func (n *hamtNode) collectValues(values *[]Object) {
	for _, node := range n.nodes {
		switch v := node.(type) {
		case hamtEntry:
			*values = append(*values, v.value)
		case *hamtNode:
			v.collectValues(values)
		}
	}
}

func (n *hamtNode) collectItems(items *[]struct{ Key, Value Object }) {
	for _, node := range n.nodes {
		switch v := node.(type) {
		case hamtEntry:
			*items = append(*items, struct{ Key, Value Object }{v.key, v.value})
		case *hamtNode:
			v.collectItems(items)
		}
	}
}

// --- Helper functions ---

// hashObject computes a hash for any Object
func hashObject(obj Object) uint32 {
	return obj.Hash()
}

// objectsEqualForMap checks equality for map keys
func objectsEqualForMap(a, b Object) bool {
	return a.Inspect() == b.Inspect()
}

// popcount counts set bits
func popcount(x uint32) int {
	x = x - ((x >> 1) & 0x55555555)
	x = (x & 0x33333333) + ((x >> 2) & 0x33333333)
	x = (x + (x >> 4)) & 0x0f0f0f0f
	x = x + (x >> 8)
	x = x + (x >> 16)
	return int(x & 0x3f)
}

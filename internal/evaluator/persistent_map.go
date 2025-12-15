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
	bitmap   uint32      // which indices are populated
	entries  []hamtEntry // actual entries (compressed)
	children []*hamtNode // child nodes for hash collisions at deeper levels
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
	idx := (hash >> shift) & hamtMask
	bit := uint32(1) << idx

	if n.bitmap&bit == 0 {
		return nil // not present
	}

	pos := popcount(n.bitmap & (bit - 1))

	// Check entries first
	if pos < len(n.entries) {
		entry := n.entries[pos]
		if entry.hash == hash && objectsEqualForMap(entry.key, key) {
			return entry.value
		}
	}

	// Check children for deeper traversal
	childIdx := pos - len(n.entries)
	if childIdx >= 0 && childIdx < len(n.children) && n.children[childIdx] != nil {
		return n.children[childIdx].get(hash, key, shift+hamtBits)
	}

	return nil
}

func (n *hamtNode) put(hash uint32, key, value Object, shift uint) (*hamtNode, bool) {
	idx := (hash >> shift) & hamtMask
	bit := uint32(1) << idx

	// Clone node
	newNode := &hamtNode{
		bitmap:   n.bitmap,
		entries:  make([]hamtEntry, len(n.entries)),
		children: make([]*hamtNode, len(n.children)),
	}
	copy(newNode.entries, n.entries)
	copy(newNode.children, n.children)

	if n.bitmap&bit == 0 {
		// New entry
		newNode.bitmap |= bit
		pos := popcount(newNode.bitmap & (bit - 1))
		newEntry := hamtEntry{hash: hash, key: key, value: value}

		// Insert at position
		newNode.entries = append(newNode.entries, hamtEntry{})
		copy(newNode.entries[pos+1:], newNode.entries[pos:])
		newNode.entries[pos] = newEntry

		return newNode, true
	}

	pos := popcount(n.bitmap & (bit - 1))

	// Check if updating existing entry
	if pos < len(newNode.entries) {
		entry := newNode.entries[pos]
		if entry.hash == hash && objectsEqualForMap(entry.key, key) {
			// Update existing
			newNode.entries[pos] = hamtEntry{hash: hash, key: key, value: value}
			return newNode, false
		}

		// Hash collision at this level - need to go deeper or create child
		if shift+hamtBits >= 32 {
			// Max depth - linear search in entries
			for i, e := range newNode.entries {
				if e.hash == hash && objectsEqualForMap(e.key, key) {
					newNode.entries[i] = hamtEntry{hash: hash, key: key, value: value}
					return newNode, false
				}
			}
			// Append new entry
			newNode.entries = append(newNode.entries, hamtEntry{hash: hash, key: key, value: value})
			return newNode, true
		}

		// Create child node with existing entry and new entry
		existingEntry := newNode.entries[pos]
		child := &hamtNode{}
		child, _ = child.put(existingEntry.hash, existingEntry.key, existingEntry.value, shift+hamtBits)
		child, added := child.put(hash, key, value, shift+hamtBits)

		// Remove entry from this level, add child
		newNode.entries = append(newNode.entries[:pos], newNode.entries[pos+1:]...)
		newNode.children = append(newNode.children, child)

		return newNode, added
	}

	// Delegate to child
	childIdx := pos - len(newNode.entries)
	if childIdx >= 0 && childIdx < len(newNode.children) && newNode.children[childIdx] != nil {
		newChild, added := newNode.children[childIdx].put(hash, key, value, shift+hamtBits)
		newNode.children[childIdx] = newChild
		return newNode, added
	}

	// Should not reach here
	return newNode, false
}

func (n *hamtNode) remove(hash uint32, key Object, shift uint) (*hamtNode, bool) {
	idx := (hash >> shift) & hamtMask
	bit := uint32(1) << idx

	if n.bitmap&bit == 0 {
		return n, false // not present
	}

	pos := popcount(n.bitmap & (bit - 1))

	// Clone node
	newNode := &hamtNode{
		bitmap:   n.bitmap,
		entries:  make([]hamtEntry, len(n.entries)),
		children: make([]*hamtNode, len(n.children)),
	}
	copy(newNode.entries, n.entries)
	copy(newNode.children, n.children)

	// Check entries
	if pos < len(newNode.entries) {
		entry := newNode.entries[pos]
		if entry.hash == hash && objectsEqualForMap(entry.key, key) {
			// Remove entry
			newNode.entries = append(newNode.entries[:pos], newNode.entries[pos+1:]...)
			if len(newNode.entries) == 0 && len(newNode.children) == 0 {
				newNode.bitmap &^= bit
			}
			return newNode, true
		}
	}

	// Check children
	childIdx := pos - len(newNode.entries)
	if childIdx >= 0 && childIdx < len(newNode.children) && newNode.children[childIdx] != nil {
		newChild, removed := newNode.children[childIdx].remove(hash, key, shift+hamtBits)
		if removed {
			newNode.children[childIdx] = newChild
			return newNode, true
		}
	}

	return n, false
}

func (n *hamtNode) collectKeys(keys *[]Object) {
	for _, entry := range n.entries {
		*keys = append(*keys, entry.key)
	}
	for _, child := range n.children {
		if child != nil {
			child.collectKeys(keys)
		}
	}
}

func (n *hamtNode) collectValues(values *[]Object) {
	for _, entry := range n.entries {
		*values = append(*values, entry.value)
	}
	for _, child := range n.children {
		if child != nil {
			child.collectValues(values)
		}
	}
}

func (n *hamtNode) collectItems(items *[]struct{ Key, Value Object }) {
	for _, entry := range n.entries {
		*items = append(*items, struct{ Key, Value Object }{entry.key, entry.value})
	}
	for _, child := range n.children {
		if child != nil {
			child.collectItems(items)
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

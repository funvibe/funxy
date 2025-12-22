package vm

import (
	"bytes"
	"hash/fnv"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/typesystem"
	"unsafe"
)

// Persistent Hash Array Mapped Trie (HAMT) implementation
// Optimized for string keys (VM globals)

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

// ModuleScope wraps PersistentMap to provide shared mutable access to globals within a module
type ModuleScope struct {
	Globals *PersistentMap
}

// NewModuleScope creates a new module scope
func NewModuleScope() *ModuleScope {
	return &ModuleScope{
		Globals: EmptyMap(),
	}
}

// Implement evaluator.Object interface so PersistentMap can be nested
func (m *PersistentMap) Type() evaluator.ObjectType { return "PERSISTENT_MAP" }
func (m *PersistentMap) Inspect() string {
	var out bytes.Buffer
	out.WriteString("PersistentMap{")
	first := true
	m.Range(func(k string, v evaluator.Object) bool {
		if !first {
			out.WriteString(", ")
		}
		out.WriteString(k)
		out.WriteString(": ")
		out.WriteString(v.Inspect())
		first = false
		return true
	})
	out.WriteString("}")
	return out.String()
}
func (m *PersistentMap) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Map"} }
func (m *PersistentMap) Hash() uint32                 { return uint32(uintptr(unsafe.Pointer(m))) }

// hamtNode is a node in the HAMT
type hamtNode struct {
	bitmap   uint32        // which indices are populated
	contents []interface{} // stores *hamtEntry or *hamtNode
}

// hamtEntry holds a key-value pair
// We use pointer to entry to distinguish from *hamtNode in contents
type hamtEntry struct {
	hash  uint32
	key   string
	value evaluator.Object
}

// EmptyMap returns an empty persistent map
func EmptyMap() *PersistentMap {
	return &PersistentMap{
		root:  nil,
		count: 0,
	}
}

// Len returns the number of entries
func (m *PersistentMap) Len() int {
	return m.count
}

// Get returns the value for a key, or nil if not found
func (m *PersistentMap) Get(key string) evaluator.Object {
	if m.root == nil {
		return nil
	}
	hash := hashString(key)
	return m.root.get(hash, key, 0)
}

// Put returns a new map with the key-value pair added/updated
func (m *PersistentMap) Put(key string, value evaluator.Object) *PersistentMap {
	hash := hashString(key)

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

// --- hamtNode methods ---

func (n *hamtNode) get(hash uint32, key string, shift uint) evaluator.Object {
	idx := (hash >> shift) & hamtMask
	bit := uint32(1) << idx

	if n.bitmap&bit == 0 {
		return nil // not present
	}

	pos := popcount(n.bitmap & (bit - 1))
	item := n.contents[pos]

	switch v := item.(type) {
	case *hamtEntry:
		if v.hash == hash && v.key == key {
			return v.value
		}
		// Hash collision at this level (but different full hash or key)
		return nil
	case *hamtNode:
		return v.get(hash, key, shift+hamtBits)
	case []*hamtEntry: // Collision bucket
		for _, e := range v {
			if e.hash == hash && e.key == key {
				return e.value
			}
		}
		return nil
	}

	return nil
}

func (n *hamtNode) put(hash uint32, key string, value evaluator.Object, shift uint) (*hamtNode, bool) {
	idx := (hash >> shift) & hamtMask
	bit := uint32(1) << idx

	// Clone node
	newNode := &hamtNode{
		bitmap:   n.bitmap,
		contents: make([]interface{}, len(n.contents)),
	}
	copy(newNode.contents, n.contents)

	if n.bitmap&bit == 0 {
		// New entry
		newNode.bitmap |= bit
		pos := popcount(newNode.bitmap & (bit - 1))
		newEntry := &hamtEntry{hash: hash, key: key, value: value}

		// Insert at position
		newNode.contents = append(newNode.contents, nil)
		copy(newNode.contents[pos+1:], newNode.contents[pos:])
		newNode.contents[pos] = newEntry

		return newNode, true
	}

	pos := popcount(n.bitmap & (bit - 1))
	item := newNode.contents[pos]

	switch v := item.(type) {
	case *hamtEntry:
		if v.hash == hash && v.key == key {
			// Update existing entry
			newNode.contents[pos] = &hamtEntry{hash: hash, key: key, value: value}
			return newNode, false
		}

		// Collision: convert entry to child node or bucket
		if shift >= 30 { // Max depth ~6 levels for 32-bit hash
			// Create bucket
			bucket := []*hamtEntry{v, {hash: hash, key: key, value: value}}
			newNode.contents[pos] = bucket
			return newNode, true
		}

		// Create new child node
		child := &hamtNode{}
		child, _ = child.put(v.hash, v.key, v.value, shift+hamtBits) // Insert existing
		child, added := child.put(hash, key, value, shift+hamtBits)  // Insert new
		newNode.contents[pos] = child
		return newNode, added

	case *hamtNode:
		// Delegate to child
		newChild, added := v.put(hash, key, value, shift+hamtBits)
		newNode.contents[pos] = newChild
		return newNode, added

	case []*hamtEntry:
		// Bucket collision
		for i, e := range v {
			if e.hash == hash && e.key == key {
				// Update
				newBucket := make([]*hamtEntry, len(v))
				copy(newBucket, v)
				newBucket[i] = &hamtEntry{hash: hash, key: key, value: value}
				newNode.contents[pos] = newBucket
				return newNode, false
			}
		}
		// Append
		newBucket := make([]*hamtEntry, len(v)+1)
		copy(newBucket, v)
		newBucket[len(v)] = &hamtEntry{hash: hash, key: key, value: value}
		newNode.contents[pos] = newBucket
		return newNode, true
	}

	return newNode, false
}

// --- Helper functions ---

func hashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func popcount(x uint32) int {
	x = x - ((x >> 1) & 0x55555555)
	x = (x & 0x33333333) + ((x >> 2) & 0x33333333)
	x = (x + (x >> 4)) & 0x0f0f0f0f
	x = x + (x >> 8)
	x = x + (x >> 16)
	return int(x & 0x3f)
}

// Range iterates over all entries
func (m *PersistentMap) Range(f func(key string, value evaluator.Object) bool) {
	if m.root != nil {
		m.root.iterate(f)
	}
}

func (n *hamtNode) iterate(f func(key string, value evaluator.Object) bool) bool {
	for _, item := range n.contents {
		switch v := item.(type) {
		case *hamtEntry:
			if !f(v.key, v.value) {
				return false
			}
		case *hamtNode:
			if !v.iterate(f) {
				return false
			}
		case []*hamtEntry:
			for _, e := range v {
				if !f(e.key, e.value) {
					return false
				}
			}
		}
	}
	return true
}

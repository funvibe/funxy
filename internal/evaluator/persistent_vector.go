package evaluator

import (
	"bytes"
	"encoding/gob"
)

func init() {
	// Register gob serialization types
	gob.Register(&gobPersistentVector{})
	gob.Register(&gobPvNode{})
}

// Persistent Vector implementation (Clojure-style)
// Uses a 32-way branching trie with tail optimization
// All operations are O(log₃₂n) which is effectively O(1) for practical sizes

const (
	// Branching factor: 32 = 2^5, allows bit manipulation
	bits  = 5
	width = 1 << bits // 32
	mask  = width - 1 // 0x1f = 31
)

// PersistentVector is an immutable vector with efficient operations
type PersistentVector struct {
	count int      // number of elements
	shift uint     // bits to shift for root level (5 * depth)
	root  *pvNode  // trie root (nil for empty or tail-only vectors)
	tail  []Object // tail buffer for O(1) append (up to 32 elements)
}

// pvNode is a node in the trie
type pvNode struct {
	array []interface{} // either []Object (leaf) or []*pvNode (branch)
}

// EmptyVector returns an empty persistent vector
func EmptyVector() *PersistentVector {
	return &PersistentVector{
		count: 0,
		shift: bits,
		root:  nil,
		tail:  make([]Object, 0, width),
	}
}

// VectorFrom creates a persistent vector from a slice
func VectorFrom(elements []Object) *PersistentVector {
	if len(elements) == 0 {
		return EmptyVector()
	}

	v := EmptyVector()
	for _, el := range elements {
		v = v.Append(el)
	}
	return v
}

// Len returns the number of elements
func (v *PersistentVector) Len() int {
	return v.count
}

// Get returns the element at index i, or nil if out of bounds
func (v *PersistentVector) Get(i int) Object {
	if i < 0 || i >= v.count {
		return nil
	}

	// Check if index is in tail
	if i >= v.tailOffset() {
		return v.tail[i-v.tailOffset()]
	}

	// Navigate trie
	node := v.root
	for level := v.shift; level > 0; level -= bits {
		idx := (i >> level) & mask
		node = node.array[idx].(*pvNode)
	}
	return node.array[i&mask].(Object)
}

// Append returns a new vector with element added at the end
func (v *PersistentVector) Append(val Object) *PersistentVector {
	// Room in tail?
	if len(v.tail) < width {
		newTail := make([]Object, len(v.tail)+1, width)
		copy(newTail, v.tail)
		newTail[len(v.tail)] = val
		return &PersistentVector{
			count: v.count + 1,
			shift: v.shift,
			root:  v.root,
			tail:  newTail,
		}
	}

	// Tail is full, push it into trie
	var newRoot *pvNode
	var newShift = v.shift

	tailNode := &pvNode{array: make([]interface{}, len(v.tail))}
	for i, el := range v.tail {
		tailNode.array[i] = el
	}

	// Root overflow? Need new level
	if v.count>>bits > 1<<v.shift {
		newRoot = &pvNode{array: make([]interface{}, width)}
		newRoot.array[0] = v.root
		newRoot.array[1] = v.newPath(v.shift, tailNode)
		newShift += bits
	} else {
		newRoot = v.pushTail(v.shift, v.root, tailNode)
	}

	return &PersistentVector{
		count: v.count + 1,
		shift: newShift,
		root:  newRoot,
		tail:  []Object{val},
	}
}

// Update returns a new vector with element at index i replaced
func (v *PersistentVector) Update(i int, val Object) *PersistentVector {
	if i < 0 || i >= v.count {
		return v // out of bounds, return unchanged
	}

	// Update in tail?
	if i >= v.tailOffset() {
		newTail := make([]Object, len(v.tail))
		copy(newTail, v.tail)
		newTail[i-v.tailOffset()] = val
		return &PersistentVector{
			count: v.count,
			shift: v.shift,
			root:  v.root,
			tail:  newTail,
		}
	}

	// Update in trie
	return &PersistentVector{
		count: v.count,
		shift: v.shift,
		root:  v.doAssoc(v.shift, v.root, i, val),
		tail:  v.tail,
	}
}

// Slice returns a new vector with elements from start to end (exclusive)
func (v *PersistentVector) Slice(start, end int) *PersistentVector {
	if start < 0 {
		start = 0
	}
	if end > v.count {
		end = v.count
	}
	if start >= end {
		return EmptyVector()
	}

	// For simplicity, rebuild from slice
	// TODO: optimize with structural sharing for large slices
	result := EmptyVector()
	for i := start; i < end; i++ {
		result = result.Append(v.Get(i))
	}
	return result
}

// Prepend returns a new vector with element added at the beginning
// Note: O(n) operation - vectors are optimized for append, not prepend
// For frequent prepend, consider using a different data structure
func (v *PersistentVector) Prepend(val Object) *PersistentVector {
	// Rebuild with new element first
	result := EmptyVector().Append(val)
	for i := 0; i < v.count; i++ {
		result = result.Append(v.Get(i))
	}
	return result
}

// Concat returns a new vector with other appended
func (v *PersistentVector) Concat(other *PersistentVector) *PersistentVector {
	result := v
	for i := 0; i < other.count; i++ {
		result = result.Append(other.Get(i))
	}
	return result
}

// ToSlice converts to a Go slice (for compatibility)
func (v *PersistentVector) ToSlice() []Object {
	result := make([]Object, v.count)
	for i := 0; i < v.count; i++ {
		result[i] = v.Get(i)
	}
	return result
}

// ForEach iterates over all elements
func (v *PersistentVector) ForEach(fn func(i int, val Object) bool) {
	for i := 0; i < v.count; i++ {
		if !fn(i, v.Get(i)) {
			break
		}
	}
}

// --- Internal helper methods ---

func (v *PersistentVector) tailOffset() int {
	if v.count < width {
		return 0
	}
	return ((v.count - 1) >> bits) << bits
}

func (v *PersistentVector) pushTail(level uint, parent, tailNode *pvNode) *pvNode {
	subIdx := ((v.count - 1) >> level) & mask

	var newChild interface{}
	if level == bits {
		newChild = tailNode
	} else if parent != nil && subIdx < len(parent.array) && parent.array[subIdx] != nil {
		newChild = v.pushTail(level-bits, parent.array[subIdx].(*pvNode), tailNode)
	} else {
		newChild = v.newPath(level-bits, tailNode)
	}

	var ret *pvNode
	if parent == nil {
		ret = &pvNode{array: make([]interface{}, width)}
	} else {
		ret = &pvNode{array: make([]interface{}, len(parent.array))}
		copy(ret.array, parent.array)
	}

	if subIdx >= len(ret.array) {
		newArray := make([]interface{}, subIdx+1)
		copy(newArray, ret.array)
		ret.array = newArray
	}
	ret.array[subIdx] = newChild
	return ret
}

func (v *PersistentVector) newPath(level uint, node *pvNode) *pvNode {
	if level == 0 {
		return node
	}
	ret := &pvNode{array: make([]interface{}, width)}
	ret.array[0] = v.newPath(level-bits, node)
	return ret
}

func (v *PersistentVector) doAssoc(level uint, node *pvNode, i int, val Object) *pvNode {
	ret := &pvNode{array: make([]interface{}, len(node.array))}
	copy(ret.array, node.array)

	if level == 0 {
		ret.array[i&mask] = val
	} else {
		subIdx := (i >> level) & mask
		ret.array[subIdx] = v.doAssoc(level-bits, node.array[subIdx].(*pvNode), i, val)
	}
	return ret
}

// gobPersistentVector is a serializable representation of PersistentVector
type gobPersistentVector struct {
	Count int
	Shift uint
	Root  *gobPvNode
	Tail  []Object
}

// gobPvNode is a serializable representation of pvNode
type gobPvNode struct {
	Array []interface{} // Can contain Object or *gobPvNode
}

// GobEncode implements gob encoding for PersistentVector
func (v *PersistentVector) GobEncode() ([]byte, error) {
	gobV := gobPersistentVector{
		Count: v.count,
		Shift: v.shift,
		Tail:  v.tail,
		Root:  convertNodeToGob(v.root),
	}
	return gobV.encode()
}

// GobDecode implements gob decoding for PersistentVector
func (v *PersistentVector) GobDecode(data []byte) error {
	gobV := &gobPersistentVector{}
	if err := gobV.decode(data); err != nil {
		return err
	}
	v.count = gobV.Count
	v.shift = gobV.Shift
	v.tail = gobV.Tail
	v.root = convertNodeFromGob(gobV.Root)
	return nil
}

// Helper functions for conversion
func convertNodeToGob(node *pvNode) *gobPvNode {
	if node == nil {
		return nil
	}
	gobNode := &gobPvNode{
		Array: make([]interface{}, len(node.array)),
	}
	for i, item := range node.array {
		switch val := item.(type) {
		case *pvNode:
			gobNode.Array[i] = convertNodeToGob(val)
		case Object:
			gobNode.Array[i] = val
		default:
			gobNode.Array[i] = val
		}
	}
	return gobNode
}

func convertNodeFromGob(gobNode *gobPvNode) *pvNode {
	if gobNode == nil {
		return nil
	}
	node := &pvNode{
		array: make([]interface{}, len(gobNode.Array)),
	}
	for i, item := range gobNode.Array {
		switch val := item.(type) {
		case *gobPvNode:
			node.array[i] = convertNodeFromGob(val)
		case Object:
			node.array[i] = val
		default:
			node.array[i] = val
		}
	}
	return node
}

// encode/decode helper methods for gobPersistentVector
func (g *gobPersistentVector) encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	if err := enc.Encode(g); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (g *gobPersistentVector) decode(data []byte) error {
	buf := bytes.NewReader(data)
	dec := gob.NewDecoder(buf)
	return dec.Decode(g)
}

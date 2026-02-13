package evaluator

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"github.com/funvibe/funxy/internal/typesystem"
	"strings"
)

// Tuple represents a heterogeneous immutable collection of objects.
type Tuple struct {
	Elements []Object
}

func (t *Tuple) Type() ObjectType { return TUPLE_OBJ }
func (t *Tuple) Inspect() string {
	out := "("
	for i, el := range t.Elements {
		if i > 0 {
			out += ", "
		}
		out += el.Inspect()
	}
	out += ")"
	return out
}
func (t *Tuple) RuntimeType() typesystem.Type {
	if t == nil {
		return typesystem.TTuple{Elements: []typesystem.Type{}}
	}
	elemTypes := make([]typesystem.Type, len(t.Elements))
	for i, el := range t.Elements {
		elemTypes[i] = el.RuntimeType()
	}
	return typesystem.TTuple{Elements: elemTypes}
}
func (t *Tuple) Hash() uint32 {
	h := uint32(1)
	for _, el := range t.Elements {
		h = 31*h + el.Hash()
	}
	return h
}

// List represents a homogeneous (in principle, though runtime allows heterogenous) immutable collection.
// It uses a hybrid representation:
// - If vector is non-nil, it relies on PersistentVector (O(1) append, O(1) index).
// - If vector is nil, it acts as a Cons list (head/tail) (O(1) prepend).
type List struct {
	vector      *PersistentVector
	head        Object
	tail        *List
	length      int    // Cached length for Cons lists (vector tracks its own length)
	ElementType string // Optional: declared element type
}

// newList creates a new List from a slice of Objects (internal)
func newList(elements []Object) *List {
	v := VectorFrom(elements)
	return &List{vector: v}
}

// NewList creates a new List from a slice of Objects (exported for VM)
func NewList(elements []Object) *List {
	v := VectorFrom(elements)
	return &List{vector: v}
}

// newListWithType creates a new List with a specified element type
func newListWithType(elements []Object, elemType string) *List {
	v := VectorFrom(elements)
	return &List{vector: v, ElementType: elemType}
}

func (l *List) Type() ObjectType { return LIST_OBJ }

// len returns the number of elements in the list
func (l *List) len() int {
	if l.vector != nil {
		return l.vector.Len()
	}
	return l.length
}

// Len returns the number of elements (exported for VM)
func (l *List) Len() int {
	return l.len()
}

// get returns the element at index i, or nil if out of bounds
func (l *List) get(i int) Object {
	if i < 0 || i >= l.len() {
		return nil
	}
	// Fast path for vector
	if l.vector != nil {
		return l.vector.Get(i)
	}

	// Traversal for Cons
	curr := l
	idx := i
	for curr != nil && curr.vector == nil {
		if idx == 0 {
			return curr.head
		}
		curr = curr.tail
		idx--
	}

	if curr == nil {
		return nil
	}
	// We reached a vector part of the list
	return curr.vector.Get(idx)
}

// Get returns the element at index i (exported for VM)
func (l *List) Get(i int) Object {
	return l.get(i)
}

// Set returns a new List with the element at index i replaced with value
func (l *List) Set(i int, value Object) *List {
	// Create new slice with updated element
	elements := l.ToSlice()
	if i < 0 {
		i = len(elements) + i
	}
	if i < 0 || i >= len(elements) {
		return l // Out of bounds, return unchanged
	}
	newElements := make([]Object, len(elements))
	copy(newElements, elements)
	newElements[i] = value
	return NewList(newElements)
}

// toSlice returns a copy of elements as a slice (for iteration)
func (l *List) ToSlice() []Object {
	if l.vector != nil {
		return l.vector.ToSlice()
	}

	result := make([]Object, 0, l.len())
	curr := l
	for curr != nil && curr.vector == nil {
		result = append(result, curr.head)
		curr = curr.tail
	}
	if curr != nil && curr.vector != nil {
		result = append(result, curr.vector.ToSlice()...)
	}
	return result
}

// slice returns a new List with elements from start to end (exclusive)
func (l *List) Slice(start, end int) *List {
	// If it's a vector, delegate
	if l.vector != nil {
		return &List{vector: l.vector.Slice(start, end), ElementType: l.ElementType}
	}

	length := l.len()
	if start < 0 || end > length || start > end {
		panic(fmt.Sprintf("slice bounds out of range: [%d:%d] length=%d", start, end, length))
	}

	// Optimization: tail slicing
	if end == length {
		curr := l
		for i := 0; i < start; i++ {
			if curr == nil {
				break
			}
			curr = curr.tail
		}
		if curr != nil {
			return curr
		}
	}

	// General case: copy slice
	slice := l.ToSlice()[start:end]
	return NewList(slice)
}

// prepend returns a new List with element added at the beginning
func (l *List) prepend(val Object) *List {
	// Use Cons cell for O(1) prepend
	return &List{
		head:        val,
		tail:        l,
		length:      l.len() + 1,
		ElementType: l.ElementType,
		// vector is nil
	}
}

// Prepend prepends element to list (exported for VM)
func (l *List) Prepend(val Object) *List {
	return l.prepend(val)
}

// concat returns a new List with another list appended
func (l *List) concat(other *List) *List {
	// If both are vectors, use vector concat
	if l.vector != nil && other.vector != nil {
		return &List{vector: l.vector.Concat(other.vector), ElementType: l.ElementType}
	}

	// Fallback: convert to slice and create new Vector-based list
	result := make([]Object, 0, l.len()+other.len())
	result = append(result, l.ToSlice()...)
	result = append(result, other.ToSlice()...)
	return NewList(result)
}

// Concat concatenates two lists (exported for VM)
func (l *List) Concat(other *List) *List {
	return l.concat(other)
}

func (l *List) Inspect() string {
	// Heuristic: If all elements are chars, print as string
	if l.len() > 0 {
		allChars := true
		for _, el := range l.ToSlice() {
			if _, ok := el.(*Char); !ok {
				allChars = false
				break
			}
		}
		if allChars {
			var out bytes.Buffer
			out.WriteString("\"")
			for _, el := range l.ToSlice() {
				out.WriteRune(rune(el.(*Char).Value))
			}
			out.WriteString("\"")
			return out.String()
		}
	}

	var out bytes.Buffer
	out.WriteString("[")
	for i, el := range l.ToSlice() {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(el.Inspect())
	}
	out.WriteString("]")
	return out.String()
}

func (l *List) RuntimeType() typesystem.Type {
	if l == nil {
		return typesystem.TCon{Name: "List"}
	}
	if l.ElementType != "" {
		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: "List"},
			Args:        []typesystem.Type{typesystem.TCon{Name: l.ElementType}},
		}
	}
	if l.len() > 0 {
		elemType := l.Get(0).RuntimeType()
		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: "List"},
			Args:        []typesystem.Type{elemType},
		}
	}
	// Empty list without type annotation - return just List
	return typesystem.TCon{Name: "List"}
}

func (l *List) Hash() uint32 {
	h := uint32(1)
	for _, obj := range l.ToSlice() {
		h = 31*h + obj.Hash()
	}
	return h
}

// GobEncode implements gob encoding for List
func (l *List) GobEncode() ([]byte, error) {
	// Serialize as a simple slice of elements plus element type
	// This avoids dealing with the complex internal structure
	elements := l.ToSlice()
	gobList := struct {
		Elements    []Object
		ElementType string
	}{
		Elements:    elements,
		ElementType: l.ElementType,
	}
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	if err := enc.Encode(gobList); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobDecode implements gob decoding for List
func (l *List) GobDecode(data []byte) error {
	buf := bytes.NewReader(data)
	dec := gob.NewDecoder(buf)
	var gobList struct {
		Elements    []Object
		ElementType string
	}
	if err := dec.Decode(&gobList); err != nil {
		return err
	}
	// Reconstruct the list
	newList := NewList(gobList.Elements)
	newList.ElementType = gobList.ElementType
	*l = *newList
	return nil
}

// Map represents an immutable hash map (HAMT-based)
type Map struct {
	hamt    *PersistentMap
	KeyType string // Optional: declared key type
	ValType string // Optional: declared value type
}

// newMap creates a new empty Map
func newMap() *Map {
	return &Map{hamt: EmptyMap()}
}

// NewMap creates a new empty Map (exported for VM)
func NewMap() *Map {
	return newMap()
}

func (m *Map) Type() ObjectType { return MAP_OBJ }

// len returns the number of entries in the map
func (m *Map) len() int {
	return m.hamt.Len()
}

// Len returns the number of entries (exported for VM)
func (m *Map) Len() int {
	return m.hamt.Len()
}

// get returns the value for a key, or nil if not found
func (m *Map) get(key Object) Object {
	return m.hamt.Get(key)
}

// Get returns value for key and whether it exists (exported for VM)
func (m *Map) Get(key Object) (Object, bool) {
	val := m.hamt.Get(key)
	return val, val != nil
}

// put returns a new Map with the key-value pair added/updated
func (m *Map) put(key, value Object) *Map {
	return &Map{hamt: m.hamt.Put(key, value), KeyType: m.KeyType, ValType: m.ValType}
}

// Put adds key-value pair to map (exported for VM)
func (m *Map) Put(key, value Object) *Map {
	return m.put(key, value)
}

// remove returns a new Map with the key removed
func (m *Map) remove(key Object) *Map {
	return &Map{hamt: m.hamt.Remove(key), KeyType: m.KeyType, ValType: m.ValType}
}

// contains checks if a key exists
func (m *Map) contains(key Object) bool {
	return m.hamt.Contains(key)
}

// keys returns all keys as a List
func (m *Map) keys() *List {
	return newList(m.hamt.Keys())
}

// values returns all values as a List
func (m *Map) values() *List {
	return newList(m.hamt.Values())
}

// equals checks if two maps have the same entries
func (m *Map) equals(other *Map, e *Evaluator) bool {
	if m.len() != other.len() {
		return false
	}
	// Check that all keys in m exist in other with equal values
	for _, kv := range m.hamt.Items() {
		otherVal, ok := other.Get(kv.Key)
		if !ok {
			return false
		}
		if !e.areObjectsEqual(kv.Value, otherVal) {
			return false
		}
	}
	return true
}

// items returns all key-value pairs as a List of Tuples
func (m *Map) items() *List {
	hamtItems := m.hamt.Items()
	tuples := make([]Object, len(hamtItems))
	for i, item := range hamtItems {
		tuples[i] = &Tuple{Elements: []Object{item.Key, item.Value}}
	}
	return newList(tuples)
}

// merge returns a new Map with entries from other (other wins on conflict)
func (m *Map) merge(other *Map) *Map {
	return &Map{hamt: m.hamt.Merge(other.hamt), KeyType: m.KeyType, ValType: m.ValType}
}

func (m *Map) Inspect() string {
	var out bytes.Buffer
	out.WriteString("%{")
	items := m.hamt.Items()
	for i, item := range items {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(item.Key.Inspect())
		out.WriteString(" => ")
		out.WriteString(item.Value.Inspect())
	}
	out.WriteString("}")
	return out.String()
}

func (m *Map) RuntimeType() typesystem.Type {
	if m == nil {
		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: "Map"},
			Args:        []typesystem.Type{typesystem.TVar{Name: "k"}, typesystem.TVar{Name: "v"}},
		}
	}
	var keyType typesystem.Type = typesystem.TVar{Name: "k"}
	var valType typesystem.Type = typesystem.TVar{Name: "v"}
	if m.KeyType != "" {
		keyType = typesystem.TCon{Name: m.KeyType}
	}
	if m.ValType != "" {
		valType = typesystem.TCon{Name: m.ValType}
	}
	// Try to infer from content
	if m.len() > 0 && m.KeyType == "" {
		items := m.hamt.Items()
		if len(items) > 0 {
			keyType = items[0].Key.RuntimeType()
			valType = items[0].Value.RuntimeType()
		}
	}
	return typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Map"},
		Args:        []typesystem.Type{keyType, valType},
	}
}

func (m *Map) Hash() uint32 {
	h := uint32(0)
	for _, item := range m.hamt.Items() {
		// XOR key and value hash, then XOR to accumulator (commutative)
		// Multiply value hash to distinguish key/value roles
		h ^= (item.Key.Hash() ^ (item.Value.Hash() * 31))
	}
	return h
}

// Bytes represents an immutable byte sequence
type Bytes struct {
	data []byte
}

// bytesNew creates a new empty Bytes
func bytesNew() *Bytes {
	return &Bytes{data: []byte{}}
}

// bytesFromSlice creates a Bytes from a Go byte slice
func bytesFromSlice(data []byte) *Bytes {
	// Make a copy to ensure immutability
	copied := make([]byte, len(data))
	copy(copied, data)
	return &Bytes{data: copied}
}

// BytesFromSlice creates Bytes from slice (exported for VM)
func BytesFromSlice(data []byte) *Bytes {
	return bytesFromSlice(data)
}

// bytesFromString creates a Bytes from a string (UTF-8)
func bytesFromString(s string) *Bytes {
	return &Bytes{data: []byte(s)}
}

// BytesFromString creates Bytes from string (exported for VM)
func BytesFromString(s string) *Bytes {
	return bytesFromString(s)
}

func (b *Bytes) Type() ObjectType { return BYTES_OBJ }

// len returns the number of bytes
func (b *Bytes) Len() int {
	return len(b.data)
}

// get returns the byte at index i, or -1 if out of bounds
func (b *Bytes) get(i int) int {
	if i < 0 || i >= len(b.data) {
		return -1
	}
	return int(b.data[i])
}

// slice returns a new Bytes from start to end (exclusive)
func (b *Bytes) slice(start, end int) *Bytes {
	if start < 0 || end > len(b.data) || start > end {
		panic(fmt.Sprintf("slice bounds out of range: [%d:%d] length=%d", start, end, len(b.data)))
	}
	return bytesFromSlice(b.data[start:end])
}

// Concat returns a new Bytes with other appended
func (b *Bytes) Concat(other *Bytes) *Bytes {
	result := make([]byte, len(b.data)+len(other.data))
	copy(result, b.data)
	copy(result[len(b.data):], other.data)
	return &Bytes{data: result}
}

// toSlice returns the underlying byte slice (should not be mutated)
func (b *Bytes) ToSlice() []byte {
	return b.data
}

// toString converts bytes to string (UTF-8)
func (b *Bytes) toString() string {
	return string(b.data)
}

// toHex converts bytes to hex string
func (b *Bytes) toHex() string {
	return hex.EncodeToString(b.data)
}

// equals checks if two Bytes are equal
func (b *Bytes) equals(other *Bytes) bool {
	return bytes.Equal(b.data, other.data)
}

// compare returns -1, 0, or 1 for lexicographic comparison
func (b *Bytes) compare(other *Bytes) int {
	return bytes.Compare(b.data, other.data)
}

func (b *Bytes) Inspect() string {
	// For display, show as @x"..." if contains non-printable chars, otherwise @"..."
	allPrintable := true
	for _, c := range b.data {
		if c < 32 || c > 126 {
			allPrintable = false
			break
		}
	}
	if allPrintable && len(b.data) < 100 {
		return "@\"" + string(b.data) + "\""
	}
	return "@x\"" + hex.EncodeToString(b.data) + "\""
}

func (b *Bytes) RuntimeType() typesystem.Type {
	if b == nil {
		return typesystem.TCon{Name: "Bytes"}
	}
	return typesystem.TCon{Name: "Bytes"}
}

func (b *Bytes) Hash() uint32 {
	h := fnv.New32a()
	h.Write(b.data)
	return h.Sum32()
}

// GobEncode implements gob encoding for Bytes
func (b *Bytes) GobEncode() ([]byte, error) {
	return b.data, nil
}

// GobDecode implements gob decoding for Bytes
func (b *Bytes) GobDecode(data []byte) error {
	b.data = make([]byte, len(data))
	copy(b.data, data)
	return nil
}

// Bits represents an immutable sequence of bits.
// Unlike Bytes, Bits can have any length (not necessarily multiple of 8).
type Bits struct {
	data   []byte // Stores bits packed in bytes
	length int    // Number of valid bits (may be less than len(data)*8)
}

// bitsNew creates an empty Bits
func bitsNew() *Bits {
	return &Bits{data: []byte{}, length: 0}
}

// bitsFromBinary creates Bits from a binary string like "10101010"
func bitsFromBinary(s string) (*Bits, error) {
	if len(s) == 0 {
		return bitsNew(), nil
	}

	// Validate all chars are 0 or 1
	for _, c := range s {
		if c != '0' && c != '1' {
			return nil, fmt.Errorf("invalid binary character: %c", c)
		}
	}

	numBits := len(s)
	numBytes := (numBits + 7) / 8
	data := make([]byte, numBytes)

	for i, c := range s {
		if c == '1' {
			byteIdx := i / 8
			bitIdx := 7 - (i % 8) // MSB first
			data[byteIdx] |= 1 << bitIdx
		}
	}

	return &Bits{data: data, length: numBits}, nil
}

// bitsFromHex creates Bits from a hex string like "FF"
func bitsFromHex(s string) (*Bits, error) {
	if len(s) == 0 {
		return bitsNew(), nil
	}

	numBits := len(s) * 4
	numBytes := (numBits + 7) / 8
	data := make([]byte, numBytes)

	for i := 0; i < len(s); i++ {
		char := s[i]
		var val byte
		if char >= '0' && char <= '9' {
			val = char - '0'
		} else if char >= 'a' && char <= 'f' {
			val = char - 'a' + 10
		} else if char >= 'A' && char <= 'F' {
			val = char - 'A' + 10
		} else {
			return nil, hex.InvalidByteError(char)
		}

		// Write 4 bits
		// Even index i -> High nibble of byte i/2
		// Odd index i -> Low nibble of byte i/2
		byteIdx := i / 2
		if i%2 == 0 {
			data[byteIdx] |= val << 4
		} else {
			data[byteIdx] |= val
		}
	}

	return &Bits{data: data, length: numBits}, nil
}

// bitsFromOctal creates Bits from an octal string like "377"
// Each octal digit represents 3 bits
func bitsFromOctal(s string) (*Bits, error) {
	if len(s) == 0 {
		return bitsNew(), nil
	}

	// Validate and convert octal to binary
	numBits := len(s) * 3
	numBytes := (numBits + 7) / 8
	data := make([]byte, numBytes)

	for i, c := range s {
		if c < '0' || c > '7' {
			return nil, fmt.Errorf("invalid octal character: %c", c)
		}
		val := int(c - '0')
		// Each octal digit is 3 bits
		for j := 0; j < 3; j++ {
			bitPos := i*3 + j
			if bitPos < numBits {
				bit := (val >> (2 - j)) & 1
				if bit == 1 {
					byteIdx := bitPos / 8
					bitIdx := 7 - (bitPos % 8)
					data[byteIdx] |= 1 << bitIdx
				}
			}
		}
	}

	return &Bits{data: data, length: numBits}, nil
}

// BitsFromBinary creates Bits from binary string (exported for VM)
func BitsFromBinary(s string) (*Bits, error) {
	return bitsFromBinary(s)
}

// BitsFromHex creates Bits from hex string (exported for VM)
func BitsFromHex(s string) (*Bits, error) {
	return bitsFromHex(s)
}

// BitsFromOctal creates Bits from octal string (exported for VM)
func BitsFromOctal(s string) (*Bits, error) {
	return bitsFromOctal(s)
}

// bitsFromBytes creates Bits from Bytes
func bitsFromBytes(b *Bytes) *Bits {
	copied := make([]byte, len(b.data))
	copy(copied, b.data)
	return &Bits{data: copied, length: len(b.data) * 8}
}

func (b *Bits) Type() ObjectType { return BITS_OBJ }

// len returns the number of bits
func (b *Bits) Len() int {
	return b.length
}

// get returns the bit at index i (0 or 1), or -1 if out of bounds
func (b *Bits) Get(i int) int {
	if i < 0 || i >= b.length {
		return -1
	}
	byteIdx := i / 8
	bitIdx := 7 - (i % 8) // MSB first
	if (b.data[byteIdx] & (1 << bitIdx)) != 0 {
		return 1
	}
	return 0
}

// slice returns a new Bits from start to end (exclusive)
func (b *Bits) slice(start, end int) *Bits {
	if start < 0 || end > b.length || start > end {
		panic(fmt.Sprintf("slice bounds out of range: [%d:%d] length=%d", start, end, b.length))
	}

	newLength := end - start
	numBytes := (newLength + 7) / 8
	newData := make([]byte, numBytes)

	for i := 0; i < newLength; i++ {
		srcBit := b.Get(start + i)
		if srcBit == 1 {
			byteIdx := i / 8
			bitIdx := 7 - (i % 8)
			newData[byteIdx] |= 1 << bitIdx
		}
	}

	return &Bits{data: newData, length: newLength}
}

// Concat returns a new Bits with other appended
func (b *Bits) Concat(other *Bits) *Bits {
	if b.length == 0 {
		return other
	}
	if other.length == 0 {
		return b
	}

	newLength := b.length + other.length
	numBytes := (newLength + 7) / 8
	newData := make([]byte, numBytes)

	// Copy bits from b
	for i := 0; i < b.length; i++ {
		if b.Get(i) == 1 {
			byteIdx := i / 8
			bitIdx := 7 - (i % 8)
			newData[byteIdx] |= 1 << bitIdx
		}
	}

	// Copy bits from other
	for i := 0; i < other.length; i++ {
		if other.Get(i) == 1 {
			pos := b.length + i
			byteIdx := pos / 8
			bitIdx := 7 - (pos % 8)
			newData[byteIdx] |= 1 << bitIdx
		}
	}

	return &Bits{data: newData, length: newLength}
}

// toBytes converts Bits to Bytes with padding
// padding: "low" pads with zeros at the end (right), "high" pads at the beginning (left)
func (b *Bits) toBytes(padding string) *Bytes {
	if b.length == 0 {
		return bytesNew()
	}

	// Number of bytes needed
	numBytes := (b.length + 7) / 8
	result := make([]byte, numBytes)

	if padding == "high" {
		// Pad at the beginning (shift bits right)
		offset := numBytes*8 - b.length
		for i := 0; i < b.length; i++ {
			if b.Get(i) == 1 {
				pos := offset + i
				byteIdx := pos / 8
				bitIdx := 7 - (pos % 8)
				result[byteIdx] |= 1 << bitIdx
			}
		}
	} else {
		// "low" - pad at the end (default, bits are already at MSB positions)
		copy(result, b.data)
	}

	return bytesFromSlice(result)
}

// toBinary returns the binary string representation
func (b *Bits) toBinary() string {
	var sb strings.Builder
	for i := 0; i < b.length; i++ {
		if b.Get(i) == 1 {
			sb.WriteByte('1')
		} else {
			sb.WriteByte('0')
		}
	}
	return sb.String()
}

// equals checks if two Bits are equal
func (b *Bits) equals(other *Bits) bool {
	if b.length != other.length {
		return false
	}
	for i := 0; i < b.length; i++ {
		if b.Get(i) != other.Get(i) {
			return false
		}
	}
	return true
}

func (b *Bits) Inspect() string { return "#b\"" + b.toBinary() + "\"" }
func (b *Bits) RuntimeType() typesystem.Type {
	if b == nil {
		return typesystem.TCon{Name: "Bits"}
	}
	return typesystem.TCon{Name: "Bits"}
}

func (b *Bits) Hash() uint32 {
	h := fnv.New32a()
	h.Write(b.data)
	// Mix in length to distinguish 00 (2 bits) from 000 (3 bits) if packed in same byte
	return h.Sum32() ^ uint32(b.length)
}

// GobEncode implements gob encoding for Bits
func (b *Bits) GobEncode() ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	if err := enc.Encode(b.data); err != nil {
		return nil, err
	}
	if err := enc.Encode(b.length); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobDecode implements gob decoding for Bits
func (b *Bits) GobDecode(data []byte) error {
	buf := bytes.NewReader(data)
	dec := gob.NewDecoder(buf)
	if err := dec.Decode(&b.data); err != nil {
		return err
	}
	if err := dec.Decode(&b.length); err != nil {
		return err
	}
	return nil
}

package evaluator

import (
	"bytes"
	"hash/fnv"
	"github.com/funvibe/funxy/internal/typesystem"
	"unsafe"
)

// StringMap is an immutable HAMT with string keys and Object values.
// Used for VM globals and evaluator environments.
type StringMap struct {
	root  *smNode
	count int
}

func (m *StringMap) Type() ObjectType             { return "PERSISTENT_MAP" }
func (m *StringMap) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Map"} }
func (m *StringMap) Hash() uint32                 { return uint32(uintptr(unsafe.Pointer(m))) }
func (m *StringMap) Inspect() string {
	var out bytes.Buffer
	out.WriteString("StringMap{")
	first := true
	m.Range(func(k string, v Object) bool {
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

type smNode struct {
	bitmap   uint32
	contents []interface{}
}

type smEntry struct {
	hash  uint32
	key   string
	value Object
}

func EmptyStringMap() *StringMap {
	return &StringMap{root: nil, count: 0}
}

func (m *StringMap) Len() int { return m.count }

func (m *StringMap) Get(key string) Object {
	if m.root == nil {
		return nil
	}
	return m.root.get(smHash(key), key, 0)
}

func (m *StringMap) Put(key string, value Object) *StringMap {
	h := smHash(key)
	var newRoot *smNode
	var added bool
	if m.root == nil {
		newRoot = &smNode{}
		newRoot, added = newRoot.put(h, key, value, 0)
	} else {
		newRoot, added = m.root.put(h, key, value, 0)
	}
	newCount := m.count
	if added {
		newCount++
	}
	return &StringMap{root: newRoot, count: newCount}
}

func (m *StringMap) Delete(key string) *StringMap {
	if m.root == nil {
		return m
	}
	newRoot, removed := m.root.delete(smHash(key), key, 0)
	if !removed {
		return m
	}
	if newRoot == nil {
		return EmptyStringMap()
	}
	return &StringMap{root: newRoot, count: m.count - 1}
}

func (m *StringMap) Range(f func(key string, value Object) bool) {
	if m.root != nil {
		m.root.iterate(f)
	}
}

// ToMap converts to a plain Go map (used for backward-compatible APIs).
func (m *StringMap) ToMap() map[string]Object {
	out := make(map[string]Object, m.count)
	m.Range(func(k string, v Object) bool {
		out[k] = v
		return true
	})
	return out
}

// --- smNode methods ---

func (n *smNode) get(hash uint32, key string, shift uint) Object {
	idx := (hash >> shift) & hamtMask
	bit := uint32(1) << idx
	if n.bitmap&bit == 0 {
		return nil
	}
	pos := popcount(n.bitmap & (bit - 1))
	switch v := n.contents[pos].(type) {
	case *smEntry:
		if v.hash == hash && v.key == key {
			return v.value
		}
		return nil
	case *smNode:
		return v.get(hash, key, shift+hamtBits)
	case []*smEntry:
		for _, e := range v {
			if e.hash == hash && e.key == key {
				return e.value
			}
		}
		return nil
	}
	return nil
}

func (n *smNode) put(hash uint32, key string, value Object, shift uint) (*smNode, bool) {
	idx := (hash >> shift) & hamtMask
	bit := uint32(1) << idx

	nn := &smNode{bitmap: n.bitmap, contents: make([]interface{}, len(n.contents))}
	copy(nn.contents, n.contents)

	if n.bitmap&bit == 0 {
		nn.bitmap |= bit
		pos := popcount(nn.bitmap & (bit - 1))
		nn.contents = append(nn.contents, nil)
		copy(nn.contents[pos+1:], nn.contents[pos:])
		nn.contents[pos] = &smEntry{hash: hash, key: key, value: value}
		return nn, true
	}

	pos := popcount(n.bitmap & (bit - 1))
	switch v := nn.contents[pos].(type) {
	case *smEntry:
		if v.hash == hash && v.key == key {
			nn.contents[pos] = &smEntry{hash: hash, key: key, value: value}
			return nn, false
		}
		if shift >= 30 {
			nn.contents[pos] = []*smEntry{v, {hash: hash, key: key, value: value}}
			return nn, true
		}
		child := &smNode{}
		child, _ = child.put(v.hash, v.key, v.value, shift+hamtBits)
		child, added := child.put(hash, key, value, shift+hamtBits)
		nn.contents[pos] = child
		return nn, added
	case *smNode:
		newChild, added := v.put(hash, key, value, shift+hamtBits)
		nn.contents[pos] = newChild
		return nn, added
	case []*smEntry:
		for i, e := range v {
			if e.hash == hash && e.key == key {
				nb := make([]*smEntry, len(v))
				copy(nb, v)
				nb[i] = &smEntry{hash: hash, key: key, value: value}
				nn.contents[pos] = nb
				return nn, false
			}
		}
		nb := make([]*smEntry, len(v)+1)
		copy(nb, v)
		nb[len(v)] = &smEntry{hash: hash, key: key, value: value}
		nn.contents[pos] = nb
		return nn, true
	}
	return nn, false
}

func (n *smNode) delete(hash uint32, key string, shift uint) (*smNode, bool) {
	idx := (hash >> shift) & hamtMask
	bit := uint32(1) << idx
	if n.bitmap&bit == 0 {
		return n, false
	}
	pos := popcount(n.bitmap & (bit - 1))
	switch v := n.contents[pos].(type) {
	case *smEntry:
		if v.hash == hash && v.key == key {
			return n.smRemoveAt(pos, bit), true
		}
		return n, false
	case *smNode:
		newChild, removed := v.delete(hash, key, shift+hamtBits)
		if !removed {
			return n, false
		}
		if newChild == nil {
			return n.smRemoveAt(pos, bit), true
		}
		nn := n.smClone()
		nn.contents[pos] = newChild
		return nn, true
	case []*smEntry:
		for i, e := range v {
			if e.hash == hash && e.key == key {
				if len(v) == 1 {
					return n.smRemoveAt(pos, bit), true
				}
				if len(v) == 2 {
					nn := n.smClone()
					nn.contents[pos] = v[1-i]
					return nn, true
				}
				nb := make([]*smEntry, len(v)-1)
				copy(nb, v[:i])
				copy(nb[i:], v[i+1:])
				nn := n.smClone()
				nn.contents[pos] = nb
				return nn, true
			}
		}
		return n, false
	}
	return n, false
}

func (n *smNode) smClone() *smNode {
	nn := &smNode{bitmap: n.bitmap, contents: make([]interface{}, len(n.contents))}
	copy(nn.contents, n.contents)
	return nn
}

func (n *smNode) smRemoveAt(pos int, bit uint32) *smNode {
	if len(n.contents) == 1 {
		return nil
	}
	nn := &smNode{
		bitmap:   n.bitmap &^ bit,
		contents: make([]interface{}, len(n.contents)-1),
	}
	copy(nn.contents[:pos], n.contents[:pos])
	copy(nn.contents[pos:], n.contents[pos+1:])
	return nn
}

func (n *smNode) iterate(f func(string, Object) bool) bool {
	for _, item := range n.contents {
		switch v := item.(type) {
		case *smEntry:
			if !f(v.key, v.value) {
				return false
			}
		case *smNode:
			if !v.iterate(f) {
				return false
			}
		case []*smEntry:
			for _, e := range v {
				if !f(e.key, e.value) {
					return false
				}
			}
		}
	}
	return true
}

func smHash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

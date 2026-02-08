package evaluator

import (
	"bytes"
	"fmt"
	"github.com/funvibe/funxy/internal/typesystem"
	"sort"
)

// TypeObject represents a runtime type value.
type TypeObject struct {
	TypeVal typesystem.Type
	Alias   string // Optional: nominal alias name (e.g. "String" for List<Char>)
}

func (t *TypeObject) Type() ObjectType { return TYPE_OBJ }
func (t *TypeObject) Inspect() string  { return "type(" + t.TypeVal.String() + ")" }
func (t *TypeObject) RuntimeType() typesystem.Type {
	if t == nil {
		return typesystem.TCon{Name: "Type"}
	}
	return typesystem.TCon{Name: "Type"}
}
func (t *TypeObject) Hash() uint32 { return hashString(t.TypeVal.String()) }

// ClassMethod represents a method belonging to a type class.
// When called, it dynamically dispatches to the correct implementation.
type ClassMethod struct {
	Name            string
	ClassName       string
	Arity           int // number of arguments (0 for nullary like mempty/pure)
	DispatchSources []typesystem.DispatchSource
}

func (tm *ClassMethod) Type() ObjectType { return CLASS_METHOD_OBJ }
func (tm *ClassMethod) Inspect() string {
	return fmt.Sprintf("class method %s.%s", tm.ClassName, tm.Name)
}
func (tm *ClassMethod) RuntimeType() typesystem.Type {
	if tm == nil {
		return typesystem.TCon{Name: "ClassMethod"}
	}
	return typesystem.TCon{Name: "ClassMethod"}
}
func (tm *ClassMethod) Hash() uint32 {
	return hashString(tm.ClassName + "." + tm.Name)
}

// RecordField represents a single field in a RecordInstance.
type RecordField struct {
	Key   string
	Value Object
}

// RecordInstance represents an instance of a Record/Struct.
// Uses a sorted slice of fields for compact memory and efficient access (O(log N)).
type RecordInstance struct {
	Fields          []RecordField // Sorted by Key
	TypeName        string        // Optional: nominal type name
	ModuleName      string        // Optional: module name for qualified access
	RowPolyExtended bool          // If true, record was extended via Row Polymorphism and should use structural typing
}

// NewRecord creates a new RecordInstance from a map of fields.
// It converts the map to a sorted slice.
func NewRecord(fieldMap map[string]Object) *RecordInstance {
	fields := make([]RecordField, 0, len(fieldMap))
	for k, v := range fieldMap {
		fields = append(fields, RecordField{Key: k, Value: v})
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Key < fields[j].Key
	})
	return &RecordInstance{Fields: fields}
}

// Get returns the value for a key, or nil if not found.
func (r *RecordInstance) Get(key string) Object {
	idx := sort.Search(len(r.Fields), func(i int) bool {
		return r.Fields[i].Key >= key
	})
	if idx < len(r.Fields) && r.Fields[idx].Key == key {
		return r.Fields[idx].Value
	}
	return nil
}

// Put returns a new RecordInstance with the key set to val.
// Since records are immutable, this creates a copy.
func (r *RecordInstance) Put(key string, val Object) *RecordInstance {
	// Check if key exists
	idx := sort.Search(len(r.Fields), func(i int) bool {
		return r.Fields[i].Key >= key
	})

	newFields := make([]RecordField, len(r.Fields)+1)

	if idx < len(r.Fields) && r.Fields[idx].Key == key {
		// Update existing: copy all, replace at idx
		copy(newFields, r.Fields)
		newFields[idx] = RecordField{Key: key, Value: val}
		// Slice length was +1, truncate back to original length
		return &RecordInstance{Fields: newFields[:len(r.Fields)], TypeName: r.TypeName, ModuleName: r.ModuleName}
	}

	// Insert new: copy up to idx, insert, copy rest
	copy(newFields, r.Fields[:idx])
	newFields[idx] = RecordField{Key: key, Value: val}
	copy(newFields[idx+1:], r.Fields[idx:])

	return &RecordInstance{Fields: newFields, TypeName: r.TypeName, ModuleName: r.ModuleName}
}

// Set updates the value for a key in place, or adds it if not found.
// This supports mutable assignment for records.
func (r *RecordInstance) Set(key string, val Object) {
	idx := sort.Search(len(r.Fields), func(i int) bool {
		return r.Fields[i].Key >= key
	})

	if idx < len(r.Fields) && r.Fields[idx].Key == key {
		r.Fields[idx].Value = val
		return
	}

	// Insert new
	r.Fields = append(r.Fields, RecordField{})
	copy(r.Fields[idx+1:], r.Fields[idx:])
	r.Fields[idx] = RecordField{Key: key, Value: val}
}

func (r *RecordInstance) Type() ObjectType { return RECORD_OBJ }
func (r *RecordInstance) Inspect() string {
	var out bytes.Buffer
	out.WriteString("{")
	for i, field := range r.Fields {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(field.Key)
		out.WriteString(": ")
		out.WriteString(field.Value.Inspect())
	}
	out.WriteString("}")
	return out.String()
}

func (r *RecordInstance) RuntimeType() typesystem.Type {
	if r == nil {
		return typesystem.TRecord{Fields: nil}
	}
	// If record was extended via Row Polymorphism, always use structural typing
	if r.RowPolyExtended {
		fields := make(map[string]typesystem.Type)
		for _, f := range r.Fields {
			fields[f.Key] = f.Value.RuntimeType()
		}
		return typesystem.TRecord{Fields: fields}
	}
	if r.TypeName != "" {
		return typesystem.TCon{Name: r.TypeName}
	}
	fields := make(map[string]typesystem.Type)
	for _, f := range r.Fields {
		fields[f.Key] = f.Value.RuntimeType()
	}
	return typesystem.TRecord{Fields: fields}
}

func (r *RecordInstance) Hash() uint32 {
	h := uint32(0)
	if r.TypeName != "" {
		h = hashString(r.TypeName)
	}
	for _, field := range r.Fields {
		// Commutative mix for map fields? No, records are ordered now (sorted)
		// So we can use ordered hash mixing, which is stronger.
		h = 31*h + (hashString(field.Key) ^ (field.Value.Hash() * 31))
	}
	return h
}

// Dictionary represents a Virtual Method Table (VTable) for a Type Class instance.
// It allows O(1) dynamic dispatch by passing explicit dictionaries.
type Dictionary struct {
	TraitName string
	// Array of implementation functions. Index corresponds to method index in trait definition.
	// Example for Monad: [0: BindFn, 1: ReturnFn]
	Methods []Object

	// References to parent dictionaries (Super Traits).
	// Example for Ord: [0: EqDictionary]
	Supers []*Dictionary
}

func (d *Dictionary) Type() ObjectType { return DICTIONARY_OBJ }
func (d *Dictionary) Inspect() string {
	return fmt.Sprintf("<dict %s>", d.TraitName)
}
func (d *Dictionary) RuntimeType() typesystem.Type {
	if d == nil {
		return typesystem.TCon{Name: "Dictionary"}
	}
	return typesystem.TCon{Name: "Dictionary"}
}
func (d *Dictionary) Hash() uint32 {
	h := hashString(d.TraitName)
	for _, m := range d.Methods {
		h = 31*h + m.Hash()
	}
	return h
}

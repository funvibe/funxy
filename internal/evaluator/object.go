package evaluator

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"math"
	"math/big"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/typesystem"
	"sort"
	"strings"
	"unsafe"
)

type ObjectType string

const (
	INTEGER_OBJ         = "INTEGER"
	FLOAT_OBJ           = "FLOAT"
	NIL_OBJ             = "NIL"
	ERROR_OBJ           = "ERROR"
	FUNCTION_OBJ        = "FUNCTION"
	BUILTIN_OBJ         = "BUILTIN"
	DATA_INSTANCE_OBJ   = "DATA_INSTANCE"
	CONSTRUCTOR_OBJ     = "CONSTRUCTOR"
	BOOLEAN_OBJ         = "BOOLEAN"
	TUPLE_OBJ           = "TUPLE"
	TYPE_OBJ            = "TYPE"
	LIST_OBJ            = "LIST"
	CHAR_OBJ            = "CHAR"
	RETURN_VALUE_OBJ    = "RETURN_VALUE"
	CLASS_METHOD_OBJ    = "CLASS_METHOD"    // New
	RECORD_OBJ          = "RECORD"          // New
	BREAK_SIGNAL_OBJ    = "BREAK_SIGNAL"    // New
	CONTINUE_SIGNAL_OBJ = "CONTINUE_SIGNAL" // New
	MAP_OBJ             = "MAP"             // Immutable hash map
	BYTES_OBJ           = "BYTES"           // Byte sequence
	BITS_OBJ            = "BITS"            // Bit sequence

	// Runtime Type Names (Canonical)
	RUNTIME_TYPE_INT        = "Int"
	RUNTIME_TYPE_FLOAT      = "Float"
	RUNTIME_TYPE_BOOL       = "Bool"
	RUNTIME_TYPE_CHAR       = "Char"
	RUNTIME_TYPE_STRING     = "String" // Not used directly as type of object, but conceptually
	RUNTIME_TYPE_LIST       = "List"
	RUNTIME_TYPE_TUPLE      = "TUPLE"
	RUNTIME_TYPE_RECORD     = "RECORD"
	RUNTIME_TYPE_FUNCTION   = "FUNCTION"
	RUNTIME_TYPE_RANGE      = "Range"        // New
	RUNTIME_TYPE_BIGINT     = "BigInt"       // New
	RUNTIME_TYPE_RATIONAL   = "Rational"     // New
	BOUND_METHOD_OBJ        = "BOUND_METHOD" // New
	TAIL_CALL_OBJ           = "TAIL_CALL"    // New for TCO
	BIG_INT_OBJ             = "BIG_INT"
	RATIONAL_OBJ            = "RATIONAL"
	COMPOSED_FUNC_OBJ       = "COMPOSED_FUNC"
	PARTIAL_APPLICATION_OBJ = "PARTIAL_APPLICATION"
	DICTIONARY_OBJ          = "DICTIONARY" // New: VTable for Type Classes
	RANGE_OBJ               = "RANGE"
	HOST_OBJ                = "HOST"        // New: Host Object
)

type Object interface {
	Type() ObjectType
	Inspect() string
	RuntimeType() typesystem.Type // Returns the type system representation
	Hash() uint32
}

// Helper for hashing strings
func hashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// TailCall represents a function call that should be executed via trampoline.
type TailCall struct {
	Func     Object
	Args     []Object
	Line     int
	Column   int
	Name     string                       // Function name for stack trace
	File     string                       // File name for stack trace
	Witness  map[string][]typesystem.Type // Witnesses for this call
	CallNode ast.Node                     // Original call node for type dispatch
}

func (tc *TailCall) Type() ObjectType             { return TAIL_CALL_OBJ }
func (tc *TailCall) Inspect() string              { return "TailCall" }
func (tc *TailCall) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "TailCall"} }
func (tc *TailCall) Hash() uint32                 { return 0 }

// Boolean
type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType             { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string              { return fmt.Sprintf("%t", b.Value) }
func (b *Boolean) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Bool"} }
func (b *Boolean) Hash() uint32 {
	if b.Value {
		return 1
	}
	return 0
}

// Integer
type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType             { return INTEGER_OBJ }
func (i *Integer) Inspect() string              { return fmt.Sprintf("%d", i.Value) }
func (i *Integer) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Int"} }
func (i *Integer) Hash() uint32 {
	return uint32(i.Value ^ (i.Value >> 32))
}

// Float
type Float struct {
	Value float64
}

func (f *Float) Type() ObjectType             { return FLOAT_OBJ }
func (f *Float) Inspect() string              { return fmt.Sprintf("%g", f.Value) }
func (f *Float) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Float"} }
func (f *Float) Hash() uint32 {
	bits := math.Float64bits(f.Value)
	return uint32(bits ^ (bits >> 32))
}

// BigInt
type BigInt struct {
	Value *big.Int
}

func (bi *BigInt) Type() ObjectType             { return BIG_INT_OBJ }
func (bi *BigInt) Inspect() string              { return bi.Value.String() }
func (bi *BigInt) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "BigInt"} }
func (bi *BigInt) Hash() uint32 {
	return hashString(bi.Value.String())
}

// Rational
type Rational struct {
	Value *big.Rat
}

func (r *Rational) Type() ObjectType             { return RATIONAL_OBJ }
func (r *Rational) Inspect() string              { return r.Value.FloatString(10) }
func (r *Rational) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Rational"} }
func (r *Rational) Hash() uint32 {
	return hashString(r.Value.String())
}

// Nil (e.g. for statements that don't return a value, or print)
type Nil struct{}

func (n *Nil) Type() ObjectType             { return NIL_OBJ }
func (n *Nil) Inspect() string              { return "Nil" }
func (n *Nil) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Nil"} }
func (n *Nil) Hash() uint32                 { return 0 }

// Error
type Error struct {
	Message    string
	Line       int
	Column     int
	StackTrace []StackFrame
}

// StackFrame for error stack traces
type StackFrame struct {
	Name   string
	File   string
	Line   int
	Column int
}

func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) Inspect() string {
	var result string
	if e.Line > 0 {
		result = fmt.Sprintf("ERROR at %d:%d: %s", e.Line, e.Column, e.Message)
	} else {
		result = "ERROR: " + e.Message
	}

	// Add stack trace if available
	// Shows call chain from innermost (most recent) to outermost
	// Format: at <caller>:<line> (called <callee>)
	if len(e.StackTrace) > 0 {
		result += "\nStack trace:"
		for i := len(e.StackTrace) - 1; i >= 0; i-- {
			frame := e.StackTrace[i]
			// The caller is the NEXT frame (outer), or filename for the outermost
			var callerName string
			if i > 0 {
				callerName = e.StackTrace[i-1].Name
			} else {
				// Use filename without extension for top-level calls
				callerName = frame.File
				if idx := strings.LastIndex(callerName, "."); idx > 0 {
					callerName = callerName[:idx]
				}
			}
			result += fmt.Sprintf("\n  at %s:%d (called %s)", callerName, frame.Line, frame.Name)
		}
	}

	return result
}
func (e *Error) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Error"} }
func (e *Error) Hash() uint32 {
	return hashString(e.Message)
}

// ReturnValue wraps a value that is being returned prematurely
type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() ObjectType             { return RETURN_VALUE_OBJ }
func (rv *ReturnValue) Inspect() string              { return rv.Value.Inspect() }
func (rv *ReturnValue) RuntimeType() typesystem.Type { return rv.Value.RuntimeType() }
func (rv *ReturnValue) Hash() uint32                 { return rv.Value.Hash() }

// BreakSignal is an internal object used to signal a break from a loop.
type BreakSignal struct {
	Value Object // Optional value to return from loop (default Zero)
}

func (bs *BreakSignal) Type() ObjectType             { return BREAK_SIGNAL_OBJ }
func (bs *BreakSignal) Inspect() string              { return "Break" }
func (bs *BreakSignal) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Break"} }
func (bs *BreakSignal) Hash() uint32                 { return 0 }

// ContinueSignal is an internal object used to signal a continue in a loop.
type ContinueSignal struct{}

func (cs *ContinueSignal) Type() ObjectType             { return CONTINUE_SIGNAL_OBJ }
func (cs *ContinueSignal) Inspect() string              { return "Continue" }
func (cs *ContinueSignal) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Continue"} }
func (cs *ContinueSignal) Hash() uint32                 { return 0 }

// Function (User defined)
type Function struct {
	Name                 string // Function name (empty for lambdas)
	Parameters           []*ast.Parameter
	WitnessParams        []string // Names of implicit dictionary parameters
	ReturnType           ast.Type // Optional, for type display
	Body                 *ast.BlockStatement
	Env                  *Environment
	CapturedWitnessStack []map[string][]typesystem.Type // Captured witness stack for closures
	Line                 int                            // Source location for stack traces
	Column               int
}

func (f *Function) Type() ObjectType { return FUNCTION_OBJ }
func (f *Function) Inspect() string {
	params := []string{}
	for _, p := range f.Parameters {
		params = append(params, p.Name.Value)
	}
	return fmt.Sprintf("fn(%s) { ... }", strings.Join(params, ", "))
}
func (f *Function) RuntimeType() typesystem.Type {
	paramTypes := make([]typesystem.Type, len(f.Parameters))
	for i, p := range f.Parameters {
		if p.Type != nil {
			paramTypes[i] = astTypeToTypesystem(p.Type)
		} else {
			paramTypes[i] = typesystem.TVar{Name: "?"}
		}
	}
	var retType typesystem.Type = typesystem.TVar{Name: "?"}
	if f.ReturnType != nil {
		retType = astTypeToTypesystem(f.ReturnType)
	}
	return typesystem.TFunc{Params: paramTypes, ReturnType: retType}
}
func (f *Function) Hash() uint32 {
	// Use pointer address for function identity
	return uint32(uintptr(unsafe.Pointer(f)))
}

// OperatorFunction represents an operator used as a function, e.g., (+)
type OperatorFunction struct {
	Operator  string
	Evaluator *Evaluator // Need evaluator reference to call evalInfixExpression (not serialized)
}

// GobEncode implements custom serialization - only encodes Operator, not Evaluator
func (of *OperatorFunction) GobEncode() ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	if err := enc.Encode(of.Operator); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobDecode implements custom deserialization - restores Operator, leaves Evaluator nil
func (of *OperatorFunction) GobDecode(data []byte) error {
	buf := bytes.NewReader(data)
	dec := gob.NewDecoder(buf)
	if err := dec.Decode(&of.Operator); err != nil {
		return err
	}
	of.Evaluator = nil // Evaluator is runtime-only, not persisted
	return nil
}

func (of *OperatorFunction) Type() ObjectType { return FUNCTION_OBJ }
func (of *OperatorFunction) Inspect() string  { return "(" + of.Operator + ")" }
func (of *OperatorFunction) RuntimeType() typesystem.Type {
	// Operators are polymorphic, return generic type
	return typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "a"}, typesystem.TVar{Name: "b"}},
		ReturnType: typesystem.TVar{Name: "c"},
	}
}
func (of *OperatorFunction) Hash() uint32 {
	return hashString(of.Operator)
}

// ComposedFunction represents f ,, g (right-to-left composition)
// When called with x, returns f(g(x))
type ComposedFunction struct {
	F         Object     // Left function (applied second)
	G         Object     // Right function (applied first)
	Evaluator *Evaluator // Need evaluator reference for applyFunction
}

func (cf *ComposedFunction) Type() ObjectType { return COMPOSED_FUNC_OBJ }
func (cf *ComposedFunction) Inspect() string  { return "(composed function)" }
func (cf *ComposedFunction) RuntimeType() typesystem.Type {
	return typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
		ReturnType: typesystem.TVar{Name: "c"},
	}
}
func (cf *ComposedFunction) Hash() uint32 {
	return cf.F.Hash() ^ cf.G.Hash()
}

// Builtin Function
type BuiltinFunction func(e *Evaluator, args ...Object) Object

type Builtin struct {
	Fn          BuiltinFunction
	Name        string          // Name of the builtin
	TypeInfo    typesystem.Type // Type signature for getType()
	DefaultArgs []Object        // Default values for trailing parameters (applied when args missing)
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "builtin function" }
func (b *Builtin) RuntimeType() typesystem.Type {
	if b.TypeInfo != nil {
		return b.TypeInfo
	}
	return typesystem.TCon{Name: "Builtin"}
}
func (b *Builtin) Hash() uint32 {
	return hashString(b.Name)
}

// PartialApplication represents a function with some arguments already applied.
type PartialApplication struct {
	Function        *Function    // User-defined function (nil if builtin/constructor)
	Builtin         *Builtin     // Builtin function (nil if user-defined/constructor)
	Constructor     *Constructor // Type constructor (nil if function/builtin)
	VMClosure       Object       // VM closure (ObjClosure) for VM partial application
	AppliedArgs     []Object     // Already applied arguments
	RemainingParams int          // Number of remaining required parameters
}

func (p *PartialApplication) Type() ObjectType { return PARTIAL_APPLICATION_OBJ }
func (p *PartialApplication) Inspect() string {
	applied := len(p.AppliedArgs)
	remaining := p.RemainingParams
	if p.Function != nil {
		return fmt.Sprintf("<partial %d/%d args>", applied, applied+remaining)
	}
	if p.Builtin != nil {
		return fmt.Sprintf("<partial %s %d/%d args>", p.Builtin.Name, applied, applied+remaining)
	}
	if p.Constructor != nil {
		return fmt.Sprintf("<partial %s %d/%d args>", p.Constructor.Name, applied, applied+remaining)
	}
	return "<partial>"
}

func (p *PartialApplication) RuntimeType() typesystem.Type {
	var originalType typesystem.Type
	if p.Function != nil {
		originalType = p.Function.RuntimeType()
	} else if p.Builtin != nil {
		originalType = p.Builtin.RuntimeType()
	} else if p.Constructor != nil {
		originalType = p.Constructor.RuntimeType()
	} else {
		return typesystem.TCon{Name: "PartialApplication"}
	}
	// Slice off applied params from function type
	if fnType, ok := originalType.(typesystem.TFunc); ok {
		appliedCount := len(p.AppliedArgs)
		if appliedCount < len(fnType.Params) {
			return typesystem.TFunc{
				Params:     fnType.Params[appliedCount:],
				ReturnType: fnType.ReturnType,
			}
		}
	}
	return typesystem.TCon{Name: "PartialApplication"}
}
func (p *PartialApplication) Hash() uint32 {
	h := uint32(0)
	if p.Function != nil {
		h = p.Function.Hash()
	} else if p.Builtin != nil {
		h = p.Builtin.Hash()
	} else if p.Constructor != nil {
		h = p.Constructor.Hash()
	}
	for _, arg := range p.AppliedArgs {
		h = 31*h + arg.Hash()
	}
	return h
}

// DataInstance represents an instance of an ADT case (e.g. Just(5), Empty).
type DataInstance struct {
	Name     string
	Fields   []Object
	TypeName string
	TypeArgs []typesystem.Type // Type arguments for generic types (e.g., [Int] for Option<Int>)
}

func (d *DataInstance) Type() ObjectType { return DATA_INSTANCE_OBJ }
func (d *DataInstance) Inspect() string {
	if len(d.Fields) == 0 {
		return d.Name
	}
	out := d.Name + "("
	for i, field := range d.Fields {
		if i > 0 {
			out += ", "
		}
		out += field.Inspect()
	}
	out += ")"
	return out
}

func (d *DataInstance) RuntimeType() typesystem.Type {
	if len(d.TypeArgs) > 0 {
		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: d.TypeName},
			Args:        d.TypeArgs,
		}
	}
	// Don't infer type args from fields - they are constructor arguments, not type parameters
	return typesystem.TCon{Name: d.TypeName}
}
func (d *DataInstance) Hash() uint32 {
	h := hashString(d.Name)
	for _, field := range d.Fields {
		h = 31*h + field.Hash()
	}
	return h
}

// Constructor represents a function that creates a DataInstance.
type Constructor struct {
	Name     string
	TypeName string
	Arity    int // Number of expected arguments
}

func (c *Constructor) Type() ObjectType { return CONSTRUCTOR_OBJ }
func (c *Constructor) Inspect() string  { return "constructor " + c.Name }
func (c *Constructor) RuntimeType() typesystem.Type {
	// Constructor is a function that returns its TypeName
	paramTypes := make([]typesystem.Type, c.Arity)
	for i := range paramTypes {
		paramTypes[i] = typesystem.TVar{Name: fmt.Sprintf("a%d", i)}
	}
	return typesystem.TFunc{
		Params:     paramTypes,
		ReturnType: typesystem.TCon{Name: c.TypeName},
	}
}
func (c *Constructor) Hash() uint32 {
	return hashString(c.Name)
}

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

// TypeObject represents a runtime type value.
type TypeObject struct {
	TypeVal typesystem.Type
	Alias   string // Optional: nominal alias name (e.g. "String" for List<Char>)
}

func (t *TypeObject) Type() ObjectType             { return TYPE_OBJ }
func (t *TypeObject) Inspect() string              { return "type(" + t.TypeVal.String() + ")" }
func (t *TypeObject) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Type"} }
func (t *TypeObject) Hash() uint32                 { return hashString(t.TypeVal.String()) }

// Char represents a character.
type Char struct {
	Value int64
}

func (c *Char) Type() ObjectType             { return CHAR_OBJ }
func (c *Char) Inspect() string              { return fmt.Sprintf("'%c'", c.Value) }
func (c *Char) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Char"} }
func (c *Char) Hash() uint32                 { return uint32(c.Value) }

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
	if start < 0 {
		start = 0
	}
	if end > length {
		end = length
	}
	if start >= end {
		return NewList([]Object{})
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
	if start < 0 {
		start = 0
	}
	if end > len(b.data) {
		end = len(b.data)
	}
	if start >= end {
		return bytesNew()
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

func (b *Bytes) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Bytes"} }

func (b *Bytes) Hash() uint32 {
	h := fnv.New32a()
	h.Write(b.data)
	return h.Sum32()
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

	decoded, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}

	return &Bits{data: decoded, length: len(decoded) * 8}, nil
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
func BitsFromBinary(s string) *Bits {
	bits, _ := bitsFromBinary(s)
	return bits
}

// BitsFromHex creates Bits from hex string (exported for VM)
func BitsFromHex(s string) *Bits {
	bits, _ := bitsFromHex(s)
	return bits
}

// BitsFromOctal creates Bits from octal string (exported for VM)
func BitsFromOctal(s string) *Bits {
	bits, _ := bitsFromOctal(s)
	return bits
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
	if start < 0 {
		start = 0
	}
	if end > b.length {
		end = b.length
	}
	if start >= end {
		return bitsNew()
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

func (b *Bits) Inspect() string              { return "#b\"" + b.toBinary() + "\"" }
func (b *Bits) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Bits"} }

func (b *Bits) Hash() uint32 {
	h := fnv.New32a()
	h.Write(b.data)
	// Mix in length to distinguish 00 (2 bits) from 000 (3 bits) if packed in same byte
	return h.Sum32() ^ uint32(b.length)
}

// ClassMethod represents a method belonging to a type class.
// When called, it dynamically dispatches to the correct implementation.
type ClassMethod struct {
	Name      string // e.g. "show"
	ClassName string // e.g. "Show"
	Arity     int    // number of arguments (0 for nullary like mempty/pure)
}

func (tm *ClassMethod) Type() ObjectType { return CLASS_METHOD_OBJ }
func (tm *ClassMethod) Inspect() string {
	return fmt.Sprintf("class method %s.%s", tm.ClassName, tm.Name)
}
func (tm *ClassMethod) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "ClassMethod"} }
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
		return &RecordInstance{Fields: newFields[:len(r.Fields)], TypeName: r.TypeName}
	}

	// Insert new: copy up to idx, insert, copy rest
	copy(newFields, r.Fields[:idx])
	newFields[idx] = RecordField{Key: key, Value: val}
	copy(newFields[idx+1:], r.Fields[idx:])

	return &RecordInstance{Fields: newFields, TypeName: r.TypeName}
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

// BoundMethod represents a method bound to a receiver object (Extension Method or similar).
type BoundMethod struct {
	Receiver Object
	Function Object // Can be *Function or *Builtin
}

func (bm *BoundMethod) Type() ObjectType             { return BOUND_METHOD_OBJ }
func (bm *BoundMethod) Inspect() string              { return fmt.Sprintf("bound method %s", bm.Function.Inspect()) }
func (bm *BoundMethod) RuntimeType() typesystem.Type { return bm.Function.RuntimeType() }
func (bm *BoundMethod) Hash() uint32 {
	return bm.Receiver.Hash() ^ bm.Function.Hash()
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
func (d *Dictionary) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Dictionary"} }
func (d *Dictionary) Hash() uint32 {
	h := hashString(d.TraitName)
	for _, m := range d.Methods {
		h = 31*h + m.Hash()
	}
	return h
}

package vm

import (
	"fmt"
	"hash/fnv"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/typesystem"
	"unsafe"
)

// CompiledFunction represents a function compiled to bytecode
type CompiledFunction struct {
	Arity         int    // Total number of parameters (not including variadic)
	RequiredArity int    // Number of required parameters (without defaults)
	Chunk         *Chunk // Bytecode
	Name          string // Function name (for debugging)
	LocalCount    int    // Number of local variables (including params)
	UpvalueCount  int    // Number of upvalues this function captures
	IsVariadic    bool   // True if last param is variadic (args...)
	// Defaults stores compiled default values
	// Defaults[i] corresponds to parameter at index RequiredArity + i
	// Can be: constant index (>=0), or -1 if default needs runtime evaluation
	Defaults []int // Constant indices for default values (-1 if complex)
	// DefaultChunks stores bytecode for complex default expressions
	DefaultChunks []*Chunk // Bytecode chunks for defaults that need evaluation
	// TypeInfo stores the function's type signature for getType()
	TypeInfo typesystem.Type
}

func (f *CompiledFunction) Type() evaluator.ObjectType { return "COMPILED_FUNCTION" }
func (f *CompiledFunction) Inspect() string            { return fmt.Sprintf("<fn %s>", f.Name) }
func (f *CompiledFunction) RuntimeType() typesystem.Type {
	if f.TypeInfo != nil {
		return f.TypeInfo
	}
	return typesystem.TCon{Name: "Function"}
}
func (f *CompiledFunction) Hash() uint32 {
	return uint32(uintptr(unsafe.Pointer(f)))
}

// ObjClosure wraps a CompiledFunction with its captured upvalues
type ObjClosure struct {
	Function *CompiledFunction
	Upvalues []*ObjUpvalue
	Globals  *ModuleScope // Shared mutable scope for module globals
}

func (c *ObjClosure) Type() evaluator.ObjectType { return "CLOSURE" }
func (c *ObjClosure) Inspect() string            { return fmt.Sprintf("<closure %s>", c.Function.Name) }
func (c *ObjClosure) RuntimeType() typesystem.Type {
	if c.Function != nil && c.Function.TypeInfo != nil {
		return c.Function.TypeInfo
	}
	return typesystem.TCon{Name: "Function"}
}
func (c *ObjClosure) Hash() uint32 {
	return uint32(uintptr(unsafe.Pointer(c)))
}

// ObjUpvalue represents a captured variable from an enclosing scope
// It can be "open" (pointing to stack) or "closed" (holding value directly)
type ObjUpvalue struct {
	// When open: Location points to the stack slot index
	// When closed: Location is -1 and Closed holds the value
	Location int
	Closed   evaluator.Object

	// For the VM's open upvalue list (singly linked, sorted by location)
	Next *ObjUpvalue
}

// spreadArg is a marker for spread arguments in function calls
type spreadArg struct {
	Value evaluator.Object
}

func (s *spreadArg) Type() evaluator.ObjectType      { return "SPREAD_ARG" }
func (s *spreadArg) Inspect() string                 { return "..." + s.Value.Inspect() }
func (s *spreadArg) RuntimeType() typesystem.Type    { return nil }
func (s *spreadArg) Hash() uint32                    { return 0 }

// VMComposedFunction represents f ,, g - native VM function composition
type VMComposedFunction struct {
	F evaluator.Object // First function to apply (after G)
	G evaluator.Object // Second function to apply (first)
}

func (c *VMComposedFunction) Type() evaluator.ObjectType   { return "VM_COMPOSED_FUNC" }
func (c *VMComposedFunction) Inspect() string              { return "<composed>" }
func (c *VMComposedFunction) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Function"} }
func (c *VMComposedFunction) Hash() uint32 {
	return c.F.Hash() ^ c.G.Hash()
}

// BuiltinClosure wraps a Go function as a VM-callable closure
// Used for registering builtin trait implementations
type BuiltinClosure struct {
	Name string
	Fn   func(args []evaluator.Object) evaluator.Object
}

func (b *BuiltinClosure) Type() evaluator.ObjectType   { return "BUILTIN_CLOSURE" }
func (b *BuiltinClosure) Inspect() string              { return "<builtin " + b.Name + ">" }
func (b *BuiltinClosure) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Function"} }
func (b *BuiltinClosure) Hash() uint32 {
	h := fnv.New32a()
	h.Write([]byte(b.Name))
	return h.Sum32()
}

package evaluator

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/typesystem"
	"strings"
	"unsafe"
)

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
	if f == nil {
		return typesystem.TFunc{
			Params:     nil,
			ReturnType: typesystem.TVar{Name: "?"},
		}
	}
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
	if of == nil {
		return typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TVar{Name: "a"}, typesystem.TVar{Name: "b"}},
			ReturnType: typesystem.TVar{Name: "c"},
		}
	}
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
	if cf == nil {
		return typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
			ReturnType: typesystem.TVar{Name: "c"},
		}
	}
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
	if b == nil {
		return typesystem.TCon{Name: "Builtin"}
	}
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
	ClassMethod     *ClassMethod // Class method (e.g. specialized generic call)
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
	if p == nil {
		return typesystem.TCon{Name: "PartialApplication"}
	}
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

// BoundMethod represents a method bound to a receiver object (Extension Method or similar).
type BoundMethod struct {
	Receiver Object
	Function Object // Can be *Function or *Builtin
}

func (bm *BoundMethod) Type() ObjectType { return BOUND_METHOD_OBJ }
func (bm *BoundMethod) Inspect() string  { return fmt.Sprintf("bound method %s", bm.Function.Inspect()) }
func (bm *BoundMethod) RuntimeType() typesystem.Type {
	if bm == nil {
		return typesystem.TCon{Name: "BoundMethod"}
	}
	return bm.Function.RuntimeType()
}
func (bm *BoundMethod) Hash() uint32 {
	return bm.Receiver.Hash() ^ bm.Function.Hash()
}

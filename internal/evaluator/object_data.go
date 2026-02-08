package evaluator

import (
	"fmt"
	"github.com/funvibe/funxy/internal/typesystem"
)

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
	if d == nil {
		return typesystem.TCon{Name: "DataInstance"} // Or use d.TypeName if we could access it, but we can't
	}
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
	if c == nil {
		return typesystem.TFunc{
			Params:     nil,
			ReturnType: typesystem.TCon{Name: "Constructor"},
		}
	}
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

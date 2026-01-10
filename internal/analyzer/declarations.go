package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/typesystem"
)

func (w *walker) VisitPackageDeclaration(n *ast.PackageDeclaration) {}

// findMaxTypeArgs recursively finds the maximum number of type arguments applied to a named type.
// This is used to infer the Kind of trait type parameters (e.g. F<A> -> *->*).
func findMaxTypeArgs(name string, t ast.Type) int {
	if t == nil {
		return 0
	}
	switch n := t.(type) {
	case *ast.NamedType:
		max := 0
		if n.Name.Value == name {
			max = len(n.Args)
		}
		for _, arg := range n.Args {
			if c := findMaxTypeArgs(name, arg); c > max {
				max = c
			}
		}
		return max
	case *ast.FunctionType:
		max := findMaxTypeArgs(name, n.ReturnType)
		for _, p := range n.Parameters {
			if c := findMaxTypeArgs(name, p); c > max {
				max = c
			}
		}
		return max
	case *ast.TupleType:
		max := 0
		for _, el := range n.Types {
			if c := findMaxTypeArgs(name, el); c > max {
				max = c
			}
		}
		return max
	case *ast.RecordType:
		max := 0
		for _, v := range n.Fields {
			if c := findMaxTypeArgs(name, v); c > max {
				max = c
			}
		}
		return max
	case *ast.UnionType:
		max := 0
		for _, t := range n.Types {
			if c := findMaxTypeArgs(name, t); c > max {
				max = c
			}
		}
		return max
	}
	return 0
}

func tagModule(t typesystem.Type, moduleName string, exportedTypes map[string]bool) typesystem.Type {
	if t == nil {
		return nil
	}

	switch t := t.(type) {
	case typesystem.TCon:
		if exportedTypes[t.Name] {
			t.Module = moduleName
		}
		// Preserve UnderlyingType when tagging with module
		// This is crucial for type aliases to work correctly with qualified names
		if t.UnderlyingType != nil {
			t.UnderlyingType = tagModule(t.UnderlyingType, moduleName, exportedTypes)
		}
		return t
	case typesystem.TApp:
		newConstructor := tagModule(t.Constructor, moduleName, exportedTypes)
		newArgs := []typesystem.Type{}
		for _, arg := range t.Args {
			newArgs = append(newArgs, tagModule(arg, moduleName, exportedTypes))
		}
		return typesystem.TApp{Constructor: newConstructor, Args: newArgs}
	case typesystem.TFunc:
		newParams := []typesystem.Type{}
		for _, p := range t.Params {
			newParams = append(newParams, tagModule(p, moduleName, exportedTypes))
		}
		newRet := tagModule(t.ReturnType, moduleName, exportedTypes)
		return typesystem.TFunc{
			Params:       newParams,
			ReturnType:   newRet,
			IsVariadic:   t.IsVariadic,
			DefaultCount: t.DefaultCount,
			Constraints:  t.Constraints,
		}
	case typesystem.TTuple:
		newElems := []typesystem.Type{}
		for _, el := range t.Elements {
			newElems = append(newElems, tagModule(el, moduleName, exportedTypes))
		}
		return typesystem.TTuple{Elements: newElems}
	case typesystem.TRecord:
		newFields := make(map[string]typesystem.Type)
		for k, v := range t.Fields {
			newFields[k] = tagModule(v, moduleName, exportedTypes)
		}
		return typesystem.TRecord{Fields: newFields, IsOpen: t.IsOpen}
	case typesystem.TType:
		return typesystem.TType{Type: tagModule(t.Type, moduleName, exportedTypes)}
	}
	return t
}

func (w *walker) VisitNamedType(n *ast.NamedType) {}

func (w *walker) VisitDataConstructor(n *ast.DataConstructor) {}

func (w *walker) VisitTupleType(t *ast.TupleType) {}

func (w *walker) VisitFunctionType(n *ast.FunctionType) {
	// Just check sub types
	for _, p := range n.Parameters {
		p.Accept(w)
	}
	n.ReturnType.Accept(w)
}

func (w *walker) VisitRecordType(n *ast.RecordType) {
	for _, v := range n.Fields {
		v.Accept(w)
	}
}

func (w *walker) VisitUnionType(n *ast.UnionType) {
	for _, t := range n.Types {
		t.Accept(w)
	}
}

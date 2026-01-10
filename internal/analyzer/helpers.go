package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
	"sort"
	"unicode"
)

// TypeToAST converts a typesystem.Type back to an AST representation.
// This is used for reconstructing type signatures for instance methods.
func TypeToAST(t typesystem.Type) ast.Type {
	if t == nil {
		return nil
	}

	switch typ := t.(type) {
	case typesystem.TCon:
		return &ast.NamedType{
			Name: &ast.Identifier{Value: typ.Name},
		}

	case typesystem.TVar:
		return &ast.NamedType{
			Name: &ast.Identifier{Value: typ.Name},
		}

	case typesystem.TApp:
		constructor := TypeToAST(typ.Constructor)
		var args []ast.Type
		for _, arg := range typ.Args {
			args = append(args, TypeToAST(arg))
		}

		// Handle nested apps flattening if needed, or just append args
		// If constructor is already a NamedType, we append args to it.
		if nt, ok := constructor.(*ast.NamedType); ok {
			nt.Args = append(nt.Args, args...)
			return nt
		}
		// If constructor is not NamedType (e.g. FunctionType), this is weird for AST
		// but we return something reasonable or fallback
		return &ast.NamedType{
			Name: &ast.Identifier{Value: "APP_CONSTRUCTOR_UNKNOWN"}, // Should not happen for standard types
			Args: args,
		}

	case typesystem.TFunc:
		var params []ast.Type
		for _, p := range typ.Params {
			params = append(params, TypeToAST(p))
		}
		return &ast.FunctionType{
			Parameters: params,
			ReturnType: TypeToAST(typ.ReturnType),
		}

	case typesystem.TTuple:
		var elems []ast.Type
		for _, e := range typ.Elements {
			elems = append(elems, TypeToAST(e))
		}
		return &ast.TupleType{
			Types: elems,
		}

	case typesystem.TRecord:
		fields := make(map[string]ast.Type)
		for k, v := range typ.Fields {
			fields[k] = TypeToAST(v)
		}
		return &ast.RecordType{
			Fields: fields,
		}

	case typesystem.TUnion:
		var types []ast.Type
		for _, t := range typ.Types {
			types = append(types, TypeToAST(t))
		}
		return &ast.UnionType{
			Types: types,
		}

	default:
		// Fallback
		return &ast.NamedType{
			Name: &ast.Identifier{Value: "UNKNOWN_TYPE"},
		}
	}
}

// CollectImplicitGenerics traverses the AST node to find implicit type variables.
// An implicit type variable is a NamedType starting with a lowercase letter
// that is NOT defined in the symbol table.
func CollectImplicitGenerics(node ast.Node, table *symbols.SymbolTable) []string {
	v := &implicitGenericVisitor{
		table: table,
		found: make(map[string]bool),
	}
	v.visit(node)

	var result []string
	for name := range v.found {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

type implicitGenericVisitor struct {
	table *symbols.SymbolTable
	found map[string]bool
}

func (v *implicitGenericVisitor) visit(node interface{}) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *ast.NamedType:
		name := n.Name.Value
		// Check for lowercase and not defined
		if len(name) > 0 && unicode.IsLower(rune(name[0])) {
			// Check if defined in table (including outer scopes)
			if !v.table.IsDefined(name) {
				v.found[name] = true
			}
		}
		// Recurse into arguments (e.g. List<a>)
		for _, arg := range n.Args {
			v.visit(arg)
		}

	case *ast.FunctionType:
		for _, p := range n.Parameters {
			v.visit(p)
		}
		v.visit(n.ReturnType)

	case *ast.TupleType:
		for _, el := range n.Types {
			v.visit(el)
		}

	case *ast.RecordType:
		for _, f := range n.Fields {
			v.visit(f)
		}

	case *ast.UnionType:
		for _, t := range n.Types {
			v.visit(t)
		}

	// For Type Declarations
	case *ast.TypeDeclarationStatement:
		v.visit(n.TargetType)
		for _, c := range n.Constructors {
			v.visit(c)
		}

	case *ast.DataConstructor:
		for _, p := range n.Parameters {
			v.visit(p)
		}

	// For Function Statements
	case *ast.FunctionStatement:
		for _, p := range n.Parameters {
			v.visit(p.Type)
		}
		v.visit(n.ReturnType)
		if n.Receiver != nil {
			v.visit(n.Receiver.Type)
		}

	// For Instance Declarations
	case *ast.InstanceDeclaration:
		for _, arg := range n.Args {
			v.visit(arg)
		}
	}
}

// CheckTraitImplementation checks if a type implements a trait, handling HKT logic.
// If the trait expects a higher kind (e.g. * -> *) but the type is fully applied (kind *),
// it attempts to find an implementation for the type constructor.
func CheckTraitImplementation(t typesystem.Type, traitName string, table *symbols.SymbolTable) bool {
	checkType := typesystem.UnwrapUnderlying(t)
	if checkType == nil {
		checkType = t
	}

	// 1. Direct check (for simple traits or exact matches)
	if table.IsImplementationExists(traitName, []typesystem.Type{checkType}) {
		return true
	}

	// 2. Kind-directed check for HKT
	// Get the kind expected by the trait
	traitKind, ok := table.GetKind(traitName)
	if !ok {
		return false
	}

	// Helper to reduce type to match kind
	var reduceToKind func(typesystem.Type, typesystem.Kind) typesystem.Type
	reduceToKind = func(ty typesystem.Type, targetKind typesystem.Kind) typesystem.Type {
		currentKind := ty.Kind()

		// If current kind matches target, we are done
		if currentKind.Equal(targetKind) {
			return ty
		}

		if tApp, ok := ty.(typesystem.TApp); ok {
			// Try peeling off last argument
			if len(tApp.Args) > 0 {
				var reduced typesystem.Type
				if len(tApp.Args) == 1 {
					reduced = tApp.Constructor
				} else {
					reduced = typesystem.TApp{
						Constructor: tApp.Constructor,
						Args:        tApp.Args[:len(tApp.Args)-1],
					}
				}
				// Recurse
				return reduceToKind(reduced, targetKind)
			}
		}
		return nil
	}

	candidate := reduceToKind(checkType, traitKind)
	if candidate != nil {
		if table.IsImplementationExists(traitName, []typesystem.Type{candidate}) {
			return true
		}
	}

	return false
}

// isTraitSubclass checks if subTrait implies superTrait (recursively)
func isTraitSubclass(subTrait, superTrait string, table *symbols.SymbolTable) bool {
	if subTrait == superTrait {
		return true
	}
	supers, _ := table.GetTraitSuperTraits(subTrait)
	for _, s := range supers {
		if isTraitSubclass(s, superTrait, table) {
			return true
		}
	}
	return false
}

// OpenRecords recursively opens all closed records in the type by assigning a fresh row variable.
// freshVar is a function that returns a new unique TVar.
func OpenRecords(t typesystem.Type, freshVar func() typesystem.TVar) typesystem.Type {
	if t == nil {
		return nil
	}

	switch t := t.(type) {
	case typesystem.TRecord:
		newFields := make(map[string]typesystem.Type)
		for k, v := range t.Fields {
			newFields[k] = OpenRecords(v, freshVar)
		}

		var newRow typesystem.Type = t.Row
		if newRow == nil {
			// Open closed record with fresh variable
			newRow = freshVar()
		} else {
			newRow = OpenRecords(t.Row, freshVar)
		}

		return typesystem.TRecord{
			Fields: newFields,
			IsOpen: true,
			Row:    newRow,
		}

	case typesystem.TApp:
		newArgs := make([]typesystem.Type, len(t.Args))
		for i, arg := range t.Args {
			newArgs[i] = OpenRecords(arg, freshVar)
		}
		newConstructor := OpenRecords(t.Constructor, freshVar)
		return typesystem.TApp{Constructor: newConstructor, Args: newArgs}

	case typesystem.TFunc:
		newParams := make([]typesystem.Type, len(t.Params))
		for i, p := range t.Params {
			newParams[i] = OpenRecords(p, freshVar)
		}
		newReturn := OpenRecords(t.ReturnType, freshVar)
		return typesystem.TFunc{
			Params:       newParams,
			ReturnType:   newReturn,
			IsVariadic:   t.IsVariadic,
			DefaultCount: t.DefaultCount,
			Constraints:  t.Constraints,
		}

	case typesystem.TTuple:
		newElems := make([]typesystem.Type, len(t.Elements))
		for i, e := range t.Elements {
			newElems[i] = OpenRecords(e, freshVar)
		}
		return typesystem.TTuple{Elements: newElems}

	case typesystem.TUnion:
		newTypes := make([]typesystem.Type, len(t.Types))
		for i, el := range t.Types {
			newTypes[i] = OpenRecords(el, freshVar)
		}
		return typesystem.NormalizeUnion(newTypes)

	case typesystem.TForall:
		// We don't recurse into quantified vars, but we recurse into body
		return typesystem.TForall{
			Vars: t.Vars,
			Type: OpenRecords(t.Type, freshVar),
		}

	case typesystem.TType:
		return typesystem.TType{Type: OpenRecords(t.Type, freshVar)}

	default:
		return t
	}
}

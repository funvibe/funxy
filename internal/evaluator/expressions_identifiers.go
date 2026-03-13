package evaluator

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/typesystem"
	"strings"
)

func (e *Evaluator) evalIdentifier(node *ast.Identifier, env *Environment) Object {
	// Special handling for placeholder (Tree mode compatibility with Analyzer)
	if node.Value == "$placeholder" {
		return &Dictionary{TraitName: "$placeholder"}
	}
	if val, ok := env.Get(node.Value); ok {
		return val
	}
	// Fallback for missing witnesses in Tree mode
	// Analyzer generates code that expects dictionaries ($dict_...), but if they weren't bound
	// (e.g. because of mismatch between instance definition and call site in Tree mode),
	// we return a placeholder to allow execution to proceed to Global Lookup.
	if strings.HasPrefix(node.Value, "$dict_") || strings.HasPrefix(node.Value, "$impl_") {
		return &Dictionary{TraitName: "$placeholder"}
	}

	if builtin, ok := Builtins[node.Value]; ok {
		return builtin
	}
	// Check if it's a trait method (e.g., fmap from Functor)
	if traitMethod := e.lookupTraitMethodByName(node.Value); traitMethod != nil {
		return traitMethod
	}
	return newError("undefined variable: %s", node.Value)
}

func (e *Evaluator) evalAssignExpression(node *ast.AssignExpression, env *Environment) Object {
	// Set type context BEFORE evaluating value, so nullary ClassMethod calls can dispatch
	oldCallNode := e.CurrentCallNode
	var pushedTypeName string
	var pushedWitness bool

	if node.AnnotatedType != nil {
		e.CurrentCallNode = node
		// Push type name to stack to guide dispatch for inner calls
		pushedTypeName = extractTypeNameFromAST(node.AnnotatedType)
		if pushedTypeName != "" {
			e.TypeContextStack = append(e.TypeContextStack, pushedTypeName)
		}

		// Proactive Witness Push
		// If AST has explicit type (e.g. x: String), resolve it and push to WitnessStack
		// This ensures init() sees the correct context "String" even if TypeMap is missing
		sysType := astTypeToTypesystem(node.AnnotatedType)
		// Resolve generics using Env if possible (though top-level assigns might not have generics)
		resolvedType := e.resolveTypeFromEnv(sysType, env)

		// Create witness map for generic context dispatch
		witness := make(map[string][]typesystem.Type)
		// Generic context dispatch: pass expected result type
		witness["$ContextType"] = []typesystem.Type{resolvedType}
		// Also push generic return context for backward compatibility
		witness["$Return"] = []typesystem.Type{resolvedType}

		// Push Applicative/Monad witness if applicable (for pure, return, etc.)
		if _, ok := resolvedType.(typesystem.TApp); ok {
			witness["Applicative"] = []typesystem.Type{resolvedType}
			witness["Monad"] = []typesystem.Type{resolvedType}
		} else if _, ok := resolvedType.(typesystem.TCon); ok {
			witness["Applicative"] = []typesystem.Type{resolvedType}
			witness["Monad"] = []typesystem.Type{resolvedType}
		}

		e.PushWitness(witness)
		pushedWitness = true
	}

	val := e.Eval(node.Value, env)

	// Restore previous call node and stack
	if pushedTypeName != "" && len(e.TypeContextStack) > 0 {
		e.TypeContextStack = e.TypeContextStack[:len(e.TypeContextStack)-1]
	}
	if pushedWitness {
		e.PopWitness()
	}
	e.CurrentCallNode = oldCallNode

	if isError(val) {
		return val
	}

	// If there's a type annotation and value is a nullary ClassMethod (Arity == 0),
	// auto-call it with type context for proper dispatch
	if node.AnnotatedType != nil {
		if cm, ok := val.(*ClassMethod); ok && cm.Arity == 0 {
			// Ensure context is set for the call
			prevCallNode := e.CurrentCallNode
			e.CurrentCallNode = node
			result := e.ApplyFunction(cm, []Object{})
			e.CurrentCallNode = prevCallNode
			if !isError(result) {
				val = result
			}
		}
	}

	// If there's a type annotation and value is a List, propagate element type
	if node.AnnotatedType != nil {
		if list, ok := val.(*List); ok {
			if elemType := extractListElementType(node.AnnotatedType); elemType != "" {
				list.ElementType = elemType
			}
		}
		// If value is a RecordInstance and type annotation is a named type, set TypeName
		if record, ok := val.(*RecordInstance); ok {
			// Handle simple named type (e.g. Point)
			if namedType, ok := node.AnnotatedType.(*ast.NamedType); ok {
				record.TypeName = namedType.Name.Value
			}
			// Handle generic named type (e.g. Box<Int>)
			// We only set the base TypeName ("Box") because runtime erasure
			// TApp also has Constructor which is usually NamedType or TCon
			// AST node for Box<Int> is NamedType with Args
			// No change needed for AST NamedType structure (it includes Args)
		}
	}

	// Actually we need to implement deep update for the tree-walk evaluator
	// otherwise tests will fail in tree-walk mode.
	if _, isMember := node.Left.(*ast.MemberExpression); isMember {
		return e.evalDeepUpdate(node.Left, val, env)
	} else if _, isIndex := node.Left.(*ast.IndexExpression); isIndex {
		return e.evalDeepUpdate(node.Left, val, env)
	}

	if ident, ok := node.Left.(*ast.Identifier); ok {
		if !env.Update(ident.Value, val) {
			env.Set(ident.Value, val)
		}
		return val
	}
	return newError("invalid assignment target")
}

// evalDeepUpdate implements recursive path updating for tree-walk evaluator (similar to OP_UPDATE_PATH)
func (e *Evaluator) evalDeepUpdate(pathExpr ast.Expression, val Object, env *Environment) Object {
	switch n := pathExpr.(type) {
	case *ast.MemberExpression:
		// e.g. r.b.c = val -> base = r.b, member = c
		base := e.Eval(n.Left, env)
		if isError(base) {
			return base
		}
		if record, ok := base.(*RecordInstance); ok {
			newRec := record.Put(n.Member.Value, val)
			// Now we need to update the parent if n.Left is also a Member/Index
			if isPathNode(n.Left) {
				return e.evalDeepUpdate(n.Left, newRec, env)
			}
			return newRec
		}
		return newError("assignment to member expects Record, got %s", base.Type())

	case *ast.IndexExpression:
		base := e.Eval(n.Left, env)
		if isError(base) {
			return base
		}
		indexObj := e.Eval(n.Index, env)
		if isError(indexObj) {
			return indexObj
		}

		var newContainer Object
		switch coll := base.(type) {
		case *List:
			if idx, ok := indexObj.(*Integer); ok {
				newContainer = coll.Set(int(idx.Value), val)
			} else {
				return newError("list index must be integer, got %s", indexObj.Type())
			}
		case *Map:
			newContainer = coll.Put(indexObj, val)
		default:
			return newError("index assignment expects List or Map, got %s", base.Type())
		}

		if isPathNode(n.Left) {
			return e.evalDeepUpdate(n.Left, newContainer, env)
		}
		return newContainer
	}
	return newError("invalid path expression")
}

func isPathNode(node ast.Expression) bool {
	switch node.(type) {
	case *ast.MemberExpression, *ast.IndexExpression:
		return true
	}
	return false
}

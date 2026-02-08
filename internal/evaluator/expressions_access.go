package evaluator

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/utils"
)

func (e *Evaluator) evalMemberExpression(node *ast.MemberExpression, env *Environment) Object {
	// New logic for dictionary passing
	if node.Dictionary != nil {
		dictObj := e.Eval(node.Dictionary, env)
		if isError(dictObj) {
			return dictObj
		}
		dict, ok := dictObj.(*Dictionary)
		if !ok {
			return newError("expected Dictionary, got %s", dictObj.Type())
		}

		// Fallback to name lookup if index is invalid (e.g. -1 from analyzer)
		idx := node.MethodIndex
		if idx == -1 {
			if methodNames, ok := TraitMethods[dict.TraitName]; ok {
				for i, name := range methodNames {
					if name == node.Member.Value {
						idx = i
						break
					}
				}
			}
		}

		if idx >= 0 && idx < len(dict.Methods) {
			method := dict.Methods[idx]
			// If we have a receiver (Left), bind the method to it
			if node.Left != nil {
				left := e.Eval(node.Left, env)
				if !isError(left) {
					return &BoundMethod{Receiver: left, Function: method}
				}
				// If Left eval failed, should we return error?
				// Maybe ignore binding if Left is not a value (e.g. Type reference)?
				// But MemberExpression Left is usually an expression.
				return left // Return the error
			}
			return method
		}
		return newError("method index %d out of bounds for dictionary %s (method %s)", node.MethodIndex, dict.TraitName, node.Member.Value)
	}

	left := e.Eval(node.Left, env)
	if isError(left) {
		return left
	}

	// Handle optional chaining (?.)
	if node.IsOptional {
		return e.evalOptionalChain(left, node, env)
	}

	if record, ok := left.(*RecordInstance); ok {
		if val := record.Get(node.Member.Value); val != nil {
			return val
		}
		if record.ModuleName != "" {
			if altName := utils.ModuleMemberFallbackName(record.ModuleName, node.Member.Value); altName != "" {
				if val := record.Get(altName); val != nil {
					return val
				}
			}
		}
	}

	// Try Extension Method lookup
	typeName := getRuntimeTypeName(left)

	if methods, ok := e.ExtensionMethods[typeName]; ok {
		if fn, ok := methods[node.Member.Value]; ok {
			return &BoundMethod{Receiver: left, Function: fn}
		}
	}

	if _, ok := left.(*RecordInstance); ok {
		return newError("field '%s' not found in record", node.Member.Value)
	}

	return newError("dot access expects Record or Extension Method, got %s", left.Type())
}

// evalOptionalChain handles the ?. operator using Optional trait
// F<A>?.field -> F<B> where F implements Optional
func (e *Evaluator) evalOptionalChain(left Object, node *ast.MemberExpression, env *Environment) Object {
	// Nullable chaining: if left is Nil, short-circuit to Nil.
	if _, ok := left.(*Nil); ok {
		return left
	}

	// Get the type name for trait dispatch
	typeName := getRuntimeTypeName(left)

	// Find isEmpty (in Optional or its super trait Empty)
	isEmptyMethod, hasIsEmpty := e.lookupTraitMethod("Optional", "isEmpty", typeName)
	if !hasIsEmpty {
		// Fallback for nullable types without Optional: access member directly.
		return e.accessMember(left, node, env)
	}

	isEmpty := e.ApplyFunction(isEmptyMethod, []Object{left})
	if isError(isEmpty) {
		return isEmpty
	}

	// If empty, return as is (short-circuit)
	if isEmpty == TRUE {
		return left
	}

	// Not empty - unwrap, access member, wrap
	unwrapMethod, hasUnwrap := e.lookupTraitMethod("Optional", "unwrap", typeName)
	if !hasUnwrap {
		return newError("type %s does not implement Optional trait (missing unwrap)", typeName)
	}

	inner := e.ApplyFunction(unwrapMethod, []Object{left})
	if isError(inner) {
		return inner
	}

	// Access the member on the inner value
	result := e.accessMember(inner, node, env)
	if isError(result) {
		return result
	}

	// Wrap the result back
	wrapMethod, hasWrap := e.lookupTraitMethod("Optional", "wrap", typeName)
	if !hasWrap {
		return newError("type %s does not implement Optional trait (missing wrap)", typeName)
	}

	return e.ApplyFunction(wrapMethod, []Object{result})
}

// accessMember performs the actual member access on an object
func (e *Evaluator) accessMember(obj Object, node *ast.MemberExpression, env *Environment) Object {
	if record, ok := obj.(*RecordInstance); ok {
		if val := record.Get(node.Member.Value); val != nil {
			return val
		}
		if record.ModuleName != "" {
			if altName := utils.ModuleMemberFallbackName(record.ModuleName, node.Member.Value); altName != "" {
				if val := record.Get(altName); val != nil {
					return val
				}
			}
		}
		return newError("field '%s' not found in record", node.Member.Value)
	}

	// Handle HostObject access via reflection
	if hostObj, ok := obj.(*HostObject); ok {
		return e.AccessHostMember(hostObj, node.Member.Value)
	}

	// Try Extension Method lookup
	typeName := getRuntimeTypeName(obj)
	if methods, ok := e.ExtensionMethods[typeName]; ok {
		if fn, ok := methods[node.Member.Value]; ok {
			return &BoundMethod{Receiver: obj, Function: fn}
		}
	}

	return newError("cannot access member '%s' on %s", node.Member.Value, obj.Type())
}

func (e *Evaluator) evalIndexExpression(node *ast.IndexExpression, env *Environment) Object {
	left := e.Eval(node.Left, env)
	if isError(left) {
		return left
	}

	index := e.Eval(node.Index, env)
	if isError(index) {
		return index
	}

	// Map indexing: m[key] -> Option<V>
	if mapObj, ok := left.(*Map); ok {
		val := mapObj.get(index)
		if val == nil {
			return makeNone() // None
		}
		return makeSome(val) // Some(value)
	}

	// For List/Tuple/Bytes, index must be integer
	idxObj, ok := index.(*Integer)
	if !ok {
		return newError("index must be integer")
	}
	idx := int(idxObj.Value)

	switch obj := left.(type) {
	case *Bytes:
		// Bytes indexing: b[i] -> Option<Int>
		max := obj.Len()
		if idx < 0 {
			idx = max + idx
		}
		if idx < 0 || idx >= max {
			return makeNone() // Out of bounds returns None
		}
		return makeSome(&Integer{Value: int64(obj.get(idx))})

	case *List:
		max := obj.Len()
		if idx < 0 {
			idx = max + idx
		}
		if idx < 0 || idx >= max {
			return newError("index out of bounds")
		}
		return obj.get(idx)

	case *Tuple:
		max := len(obj.Elements)
		if idx < 0 {
			idx = max + idx
		}
		if idx < 0 || idx >= max {
			return newError("tuple index out of bounds: %d (tuple has %d elements)", idxObj.Value, max)
		}
		return obj.Elements[idx]

	default:
		return newError("index operator not supported: %s", left.Type())
	}
}

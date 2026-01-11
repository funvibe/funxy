package evaluator

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/typesystem"
)

func (e *Evaluator) evalCallExpression(node *ast.CallExpression, env *Environment) Object {
	// Special handling for default() to avoid init cycle
	if ident, ok := node.Function.(*ast.Identifier); ok && ident.Value == "default" {
		args := e.evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		if len(args) != 1 {
			return newError("wrong number of arguments to default. got=%d, want=1", len(args))
		}
		typeObj, ok := args[0].(*TypeObject)
		if !ok {
			return newError("argument to default must be a Type, got %s", args[0].Type())
		}
		return e.GetDefaultForType(typeObj.TypeVal)
	}

	// Push Witness if present in AST
	var pushedWitness bool
	if node.Witness != nil {
		if witnesses, ok := node.Witness.(map[string][]typesystem.Type); ok {
			e.PushWitness(witnesses)
			pushedWitness = true
		}
	}

	// Fix: Hide TypeContextStack for argument evaluation
	// Arguments should not inherit the expected type of the function call result
	oldTypeContextStack := e.TypeContextStack
	e.TypeContextStack = nil

	// Helper to restore context on error/return
	restoreContext := func() {
		e.TypeContextStack = oldTypeContextStack
	}

	// Calculate Witnesses (Explicit Dictionary Passing)
	var witnessArgs []Object
	for _, witnessExpr := range node.Witnesses {
		wVal := e.Eval(witnessExpr, env)
		if isError(wVal) {
			restoreContext()
			if pushedWitness {
				e.PopWitness()
			}
			return wVal
		}
		// Skip $placeholder dictionaries - they are markers for dynamic dispatch in tree mode
		// and should not be passed as actual arguments to functions
		if dict, ok := wVal.(*Dictionary); ok && dict.TraitName == "$placeholder" {
			continue
		}
		// Only add Dictionary objects as witness args - other types (like Builtin functions)
		// are resolved implementations that should not be passed as extra arguments.
		// They are used via dynamic dispatch in the operator evaluation.
		if _, ok := wVal.(*Dictionary); ok {
			witnessArgs = append(witnessArgs, wVal)
		}
		// Non-dictionary witnesses (like resolved builtins) are ignored here
		// because they represent already-resolved trait methods that will be
		// dispatched dynamically when the operator is evaluated.
	}

	if node.IsTail {
		// Set CurrentCallNode for tail calls too (needed for ClassMethod dispatch)
		oldCallNode := e.CurrentCallNode
		e.CurrentCallNode = node

		function := e.Eval(node.Function, env)
		if isError(function) {
			restoreContext()
			e.CurrentCallNode = oldCallNode
			if pushedWitness {
				e.PopWitness()
			}
			return function
		}
		args := e.evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			restoreContext()
			e.CurrentCallNode = oldCallNode
			if pushedWitness {
				e.PopWitness()
			}
			return args[0]
		}

		// Prepend witnesses to args, but not for ClassMethod (it handles witnesses itself)
		if len(witnessArgs) > 0 {
			if _, ok := function.(*ClassMethod); !ok {
				args = append(witnessArgs, args...)
			}
		}

		// For tail call, we need to preserve the witness context.
		// However, TCO means we replace the stack frame.
		// The next iteration will execute this call.
		// If we pop now, the witness is lost for the execution of the tail call?
		// No, the tail call object itself captures the intent to call.
		// But where is the witness stored? In the TailCall object.

		tc := &TailCall{Func: function, Args: args, CallNode: node}
		if pushedWitness {
			// Copy witnesses to TailCall object
			if witnesses, ok := node.Witness.(map[string][]typesystem.Type); ok {
				tc.Witness = witnesses
			}
			// We can pop from stack now because the loop will re-push (or should handle it)
			// Wait, the main Eval loop doesn't handle TailCall witness pushing.
			// It just calls ApplyFunction.
			// ApplyFunction handles TailCall execution? No, Eval loop does.
			// Let's check Eval loop/trampoline.
			// Usually Eval returns TailCall, and the caller (if it supports TCO) handles it.
			// The caller is often inside a loop in ApplyFunction for user functions.

			// We pop here because we are returning from this frame.
			e.PopWitness()
		}

		if tok := node.GetToken(); tok.Type != "" {
			tc.Line = tok.Line
			tc.Column = tok.Column
		}
		// Store call info for stack trace (even though it's a tail call)
		tc.Name = getFunctionName(function)
		tc.File = e.CurrentFile
		e.CurrentCallNode = oldCallNode
		restoreContext()
		return tc
	}

	function := e.Eval(node.Function, env)
	if isError(function) {
		restoreContext()
		if pushedWitness {
			e.PopWitness()
		}
		return function
	}
	args := e.evalExpressions(node.Arguments, env)
	if len(args) == 1 && isError(args[0]) {
		restoreContext()
		if pushedWitness {
			e.PopWitness()
		}
		return args[0]
	}

	// Prepend witnesses to args, but not for ClassMethod (it handles witnesses itself)
	if len(witnessArgs) > 0 {
		if _, ok := function.(*ClassMethod); !ok {
			args = append(witnessArgs, args...)
		}
	}

	// FIX: Tree mode functions (user defined) might not expect the witness if they weren't compiled/transformed
	// to include the witness parameter. We need to check if the function expects it.
	// But evalCallExpression doesn't know about Function parameters yet (function is just an Object).
	// ApplyFunction handles this stripping logic now.

	// Push call frame with call site info (where the call is made from)
	funcName := getFunctionName(function)
	tok := node.GetToken()
	e.PushCall(funcName, e.CurrentFile, tok.Line, tok.Column)

	// Store current call node for type-based dispatch (pure/mempty)
	// Always update to current node so ApplyFunction checks the specific call's type
	oldCallNode := e.CurrentCallNode
	e.CurrentCallNode = node

	// Handle TypeArgs for data constructors (Reified Generics)
	// If this call has TypeArgs, prepend them as TypeObject arguments
	if node.TypeArgs != nil {
		typeArgObjects := make([]Object, len(node.TypeArgs))
		for i, typeArg := range node.TypeArgs {
			typeArgObjects[i] = &TypeObject{TypeVal: typeArg}
		}
		args = append(typeArgObjects, args...)
	}

	restoreContext() // Restore context for the actual function application (dispatch)
	result := e.ApplyFunction(function, args)
	e.CurrentCallNode = oldCallNode

	// Add stack trace to errors if not already present
	if err, ok := result.(*Error); ok {
		if len(err.StackTrace) == 0 && len(e.CallStack) > 0 {
			err.StackTrace = make([]StackFrame, len(e.CallStack))
			for i, frame := range e.CallStack {
				err.StackTrace[i] = StackFrame{
					Name:   frame.Name,
					File:   frame.File,
					Line:   frame.Line,
					Column: frame.Column,
				}
			}
		}
	}

	e.PopCall()
	if pushedWitness {
		e.PopWitness()
	}
	return result
}

// getFunctionName extracts the name from a function object
func getFunctionName(fn Object) string {
	switch f := fn.(type) {
	case *Function:
		if f.Name != "" {
			return f.Name
		}
		return "<lambda>"
	case *Builtin:
		return f.Name
	case *BoundMethod:
		if f.Function != nil {
			return getFunctionName(f.Function)
		}
		return "<method>"
	case *Constructor:
		return f.TypeName
	case *PartialApplication:
		if f.Function != nil {
			return f.Function.Name + " (partial)"
		}
		if f.Builtin != nil {
			return f.Builtin.Name + " (partial)"
		}
		return "<partial>"
	default:
		return "<unknown>"
	}
}

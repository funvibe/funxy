package evaluator

import (
	"context"
	"io"
	"os"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/typesystem"
	"reflect"
	"strings"
)

// CallFrame represents a single frame in the call stack
type CallFrame struct {
	Name   string // Function name
	File   string // Source file
	Line   int    // Line number
	Column int    // Column number
}

// VMCallHandler is a callback for calling VM closures from builtins
type VMCallHandler func(closure Object, args []Object) Object

// AsyncHandler is a callback for handling async function execution
type AsyncHandler func(fn Object, args []Object) Object

type Evaluator struct {
	// Context for cancellation
	Context context.Context

	Out io.Writer
	// Registry for class implementations.
	// Map: ClassName -> TypeName -> FunctionObject
	ClassImplementations map[string]map[string]Object
	// Registry for extension methods.
	// Map: TypeName -> MethodName -> Object (Function or Builtin)
	ExtensionMethods map[string]map[string]Object
	// Loader for modules
	Loader ModuleLoader
	// BaseDir for import resolution (optional)
	BaseDir string
	// ModuleCache to avoid re-evaluating modules
	ModuleCache map[string]Object
	// Trait default implementations: "TraitName.methodName" -> FunctionStatement
	TraitDefaults map[string]*ast.FunctionStatement
	// Builtin trait default implementations: "TraitName.methodName" -> Builtin
	// Used for built-in traits like Show that don't have AST
	BuiltinTraitDefaults map[string]*Builtin
	// Global environment (for default implementations to access trait methods)
	GlobalEnv *Environment
	// Operator -> Trait mapping for dispatch: "+" -> "Add", "==" -> "Equal"
	OperatorTraits map[string]string
	// Registry for Trait SuperTraits: TraitName -> [SuperTraitName]
	TraitSuperTraits map[string][]string
	// TypeMap from analyzer - maps AST nodes to their inferred types
	TypeMap map[ast.Node]typesystem.Type
	// CurrentCallNode - temporarily stores the AST node being evaluated
	// Used for type-based dispatch (e.g., pure/mempty)
	CurrentCallNode ast.Node
	// TypeContextStack stores the stack of expected types from AnnotatedExpressions
	TypeContextStack []string
	// CallStack for stack traces on errors
	CallStack []CallFrame
	// CurrentFile being evaluated
	CurrentFile string
	// ContainerContext tracks the expected container type for `pure` when inside >>=
	// e.g., when evaluating `Some(42) >>= pure`, this is set to "Option"
	ContainerContext string
	// WitnessStack stores the stack of type witnesses (dictionaries) for generic calls.
	// Each frame maps "TraitName" -> ConcreteTypes and "TypeVarName" -> ConcreteTypes.
	WitnessStack []map[string][]typesystem.Type
	// TypeAliases stores underlying types for type aliases
	// e.g., "Point" -> TRecord{Fields: {x: Int, y: Int}}
	// Used by default() to create default values for alias types
	TypeAliases map[string]typesystem.Type
	// VMCallHandler is set by VM to allow builtins to call VM closures
	VMCallHandler VMCallHandler
	// AsyncHandler is a callback for handling async function execution (used by VM)
	AsyncHandler AsyncHandler
	// CaptureHandler is a callback for safe capturing of closures for async execution
	CaptureHandler func(Object) Object

	// HostCallHandler handles calling reflection methods (injected from embed)
	HostCallHandler func(reflect.Value, []Object) (Object, error)
	// HostToValueHandler handles converting Go values to Objects (injected from embed)
	HostToValueHandler func(interface{}) (Object, error)

	// CurrentEnv stores the current environment being evaluated (for witness lookup)
	CurrentEnv *Environment

	// Fork creates a thread-safe copy of the evaluator for background execution
	Fork func() *Evaluator

	// EmbeddedResources holds static files embedded via --embed during build.
	// File I/O builtins check this map before falling back to the filesystem.
	// Key is relative path from source directory, value is file contents.
	EmbeddedResources map[string][]byte

	// IsBundleMode is true when running from a compiled binary (bundle).
	// Affects sysScriptDir() which returns "" in bundle mode (no script on disk).
	IsBundleMode bool

	// evalDepth tracks the current nesting depth of Eval calls to prevent stack overflow
	evalDepth int
}

// ModuleLoader interface (same as in Analyzer, should probably be in a common package)
type ModuleLoader interface {
	GetModule(path string) (interface{}, error)
}

type LoadedModule interface {
	GetExports() map[string]Object
}

func New() *Evaluator {
	e := &Evaluator{
		Out:                  os.Stdout,
		ClassImplementations: make(map[string]map[string]Object),
		ExtensionMethods:     make(map[string]map[string]Object),
		ModuleCache:          make(map[string]Object),
		TraitSuperTraits:     make(map[string][]string),
		TypeMap:              make(map[ast.Node]typesystem.Type),
		TypeAliases:          make(map[string]typesystem.Type),
	}

	// Pre-populate standard aliases
	// String = List<Char>
	e.TypeAliases["String"] = typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.TCon{Name: "Char"}},
	}

	return e
}

// lookupTraitMethod looks up a method for a type (or types) in a trait, including super traits.
// Supports Multi-Parameter Type Classes (MPTC) via variadic typeNames.
func (e *Evaluator) lookupTraitMethod(traitName, methodName string, typeNames ...string) (Object, bool) {
	if len(typeNames) == 0 {
		return nil, false
	}

	// Form key: "Type" or "Type1_Type2"
	var key string
	if len(typeNames) == 1 {
		key = typeNames[0]
	} else {
		key = strings.Join(typeNames, "_")
	}

	// Check the trait itself
	if typesMap, ok := e.ClassImplementations[traitName]; ok {
		if methodTableObj, ok := typesMap[key]; ok {
			if methodTable, ok := methodTableObj.(*MethodTable); ok {
				if method, ok := methodTable.Methods[methodName]; ok {
					return method, true
				}
			}
		}
	}

	// Check super traits using config
	if traitInfo := config.GetTraitInfo(traitName); traitInfo != nil {
		for _, superTrait := range traitInfo.SuperTraits {
			if method, ok := e.lookupTraitMethod(superTrait, methodName, typeNames...); ok {
				return method, true
			}
		}
	}

	// Fallback to trait default implementation
	if e.TraitDefaults != nil {
		key := traitName + "." + methodName
		if fnStmt, ok := e.TraitDefaults[key]; ok {
			return &Function{
				Name:       methodName,
				Parameters: fnStmt.Parameters,
				Body:       fnStmt.Body,
				Env:        NewEnvironment(),
				Line:       fnStmt.Token.Line,
				Column:     fnStmt.Token.Column,
			}, true
		}
	}

	return nil, false
}

// GetNodeType returns the inferred type for an AST node from the TypeMap.
// Returns nil if the type is not found.
func (e *Evaluator) GetNodeType(node ast.Node) typesystem.Type {
	if e.TypeMap == nil {
		return nil
	}
	return e.TypeMap[node]
}

// GetExpectedReturnType extracts the return type from a function call's context.
// Useful for dispatching pure/mempty based on expected type.
func (e *Evaluator) GetExpectedReturnType(node ast.Node) typesystem.Type {
	t := e.GetNodeType(node)
	if t == nil {
		return nil
	}
	return t
}

func (e *Evaluator) SetLoader(l ModuleLoader) {
	e.Loader = l
}

// Clone creates a copy of the evaluator for use in a goroutine
// Shares immutable state but creates new mutable state
func (e *Evaluator) Clone() *Evaluator {
	return &Evaluator{
		Context:              e.Context,
		Out:                  e.Out,
		ClassImplementations: e.ClassImplementations, // shared, read-only in tasks
		ExtensionMethods:     e.ExtensionMethods,     // shared, read-only in tasks
		Loader:               e.Loader,
		BaseDir:              e.BaseDir,
		ModuleCache:          e.ModuleCache, // shared
		TraitDefaults:        e.TraitDefaults,
		BuiltinTraitDefaults: e.BuiltinTraitDefaults,
		GlobalEnv:            e.GlobalEnv,
		OperatorTraits:       e.OperatorTraits,
		TypeMap:              e.TypeMap,
		CurrentCallNode:      nil,                  // new per goroutine
		CallStack:            make([]CallFrame, 0), // new per goroutine
		CurrentFile:          e.CurrentFile,
		ContainerContext:     "",
		WitnessStack:         make([]map[string][]typesystem.Type, 0), // new per goroutine
		TypeAliases:          e.TypeAliases,                           // shared, read-only
		VMCallHandler:        e.VMCallHandler,                         // shared, for calling VM closures from builtins
		CaptureHandler:       e.CaptureHandler,                        // shared
		HostCallHandler:      e.HostCallHandler,                       // shared
		HostToValueHandler:   e.HostToValueHandler,                    // shared
		EmbeddedResources:    e.EmbeddedResources,                     // shared, read-only
		IsBundleMode:         e.IsBundleMode,                          // shared
	}
}

// maxEvalDepth is the maximum nesting depth of Eval calls.
// Prevents stack overflow from infinite recursion in user programs.
const maxEvalDepth = 10000

func (e *Evaluator) Eval(node ast.Node, env *Environment) Object {
	// Check recursion depth to prevent Go stack overflow
	e.evalDepth++
	if e.evalDepth > maxEvalDepth {
		e.evalDepth--
		return newError("maximum recursion depth exceeded")
	}

	// Check for cancellation
	if e.Context != nil {
		select {
		case <-e.Context.Done():
			e.evalDepth--
			return newError("execution cancelled: %v", e.Context.Err())
		default:
		}
	}

	// Update CurrentEnv for access in ApplyFunction (Witness Passing)
	oldEnv := e.CurrentEnv
	e.CurrentEnv = env
	// Restore on return (defer is slightly expensive but necessary for correctness here)
	defer func() {
		e.CurrentEnv = oldEnv
		e.evalDepth--
	}()

	obj := e.evalCore(node, env)
	if err, ok := obj.(*Error); ok {
		if err.Line == 0 && node != nil {
			if provider, ok := node.(ast.TokenProvider); ok {
				tok := provider.GetToken()
				err.Line = tok.Line
				err.Column = tok.Column
			}
		}
	}
	return obj
}

func (e *Evaluator) evalCore(node ast.Node, env *Environment) Object {
	switch node := node.(type) {
	// Statements
	case *ast.Program:
		return e.evalProgram(node, env)
	case *ast.PackageDeclaration:
		return &Nil{}
	case *ast.ImportStatement:
		return e.evalImportStatement(node, env)
	case *ast.ExpressionStatement:
		return e.Eval(node.Expression, env)
	case *ast.TypeDeclarationStatement:
		return e.evalTypeDeclaration(node, env)
	case *ast.TraitDeclaration:
		return e.evalTraitDeclaration(node, env)
	case *ast.InstanceDeclaration:
		return e.evalInstanceDeclaration(node, env)
	case *ast.ConstantDeclaration:
		return e.evalConstantDeclaration(node, env)
	case *ast.FunctionStatement:
		// Check if it's an extension method
		if node.Receiver != nil {
			return e.evalExtensionMethod(node, env)
		}

		// Register function in current environment
		fn := &Function{
			Name:          node.Name.Value,
			Parameters:    node.Parameters,
			WitnessParams: node.WitnessParams,
			ReturnType:    node.ReturnType,
			Body:          node.Body,
			Env:           env, // Closure
			Line:          node.Token.Line,
			Column:        node.Token.Column,
		}
		env.Set(node.Name.Value, fn)
		return &Nil{}
	case *ast.BlockStatement:
		return e.evalBlockStatement(node, env)

	// Expressions
	case *ast.IntegerLiteral:
		return &Integer{Value: node.Value}
	case *ast.FloatLiteral:
		return &Float{Value: node.Value}
	case *ast.BigIntLiteral:
		return &BigInt{Value: node.Value}
	case *ast.RationalLiteral:
		return &Rational{Value: node.Value}
	case *ast.BooleanLiteral:
		return e.nativeBoolToBooleanObject(node.Value)
	case *ast.NilLiteral:
		return &Nil{}
	case *ast.TupleLiteral:
		return e.evalTupleLiteral(node, env)
	case *ast.ListLiteral:
		return e.evalListLiteral(node, env)
	case *ast.ListComprehension:
		return e.evalListComprehension(node, env)
	case *ast.MapLiteral:
		return e.evalMapLiteral(node, env)
	case *ast.RecordLiteral:
		return e.evalRecordLiteral(node, env)
	case *ast.MemberExpression:
		return e.evalMemberExpression(node, env)
	case *ast.IndexExpression:
		return e.evalIndexExpression(node, env)
	case *ast.StringLiteral:
		return e.evalStringLiteral(node, env)
	case *ast.FormatStringLiteral:
		return e.evalFormatStringLiteral(node, env)
	case *ast.InterpolatedString:
		return e.evalInterpolatedString(node, env)
	case *ast.CharLiteral:
		return e.evalCharLiteral(node, env)
	case *ast.BytesLiteral:
		return e.evalBytesLiteral(node, env)
	case *ast.BitsLiteral:
		return e.evalBitsLiteral(node, env)
	case *ast.PrefixExpression:
		right := e.Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return e.evalPrefixExpression(node.Operator, right)
	case *ast.OperatorAsFunction:
		// Create a function that applies the operator
		return &OperatorFunction{Operator: node.Operator, Evaluator: e}

	case *ast.InfixExpression:
		// Short-circuit evaluation for && and ||
		if node.Operator == "&&" {
			left := e.Eval(node.Left, env)
			if isError(left) {
				return left
			}
			if !e.isTruthy(left) {
				return left
			}
			right := e.Eval(node.Right, env)
			if isError(right) {
				return right
			}
			return right
		}
		if node.Operator == "||" {
			left := e.Eval(node.Left, env)
			if isError(left) {
				return left
			}
			if e.isTruthy(left) {
				return left
			}
			right := e.Eval(node.Right, env)
			if isError(right) {
				return right
			}
			return right
		}

		// Null coalescing: x ?? default (via Optional trait)
		// Some(x) ?? y = x, None ?? y = y
		// Ok(x) ?? y = x, Fail(_) ?? y = y
		if node.Operator == "??" {
			left := e.Eval(node.Left, env)
			if isError(left) {
				return left
			}

			// Nullable coalescing: if left is Nil, return right.
			if _, ok := left.(*Nil); ok {
				return e.Eval(node.Right, env)
			}

			// Use Optional trait for dispatch (includes super traits like Empty)
			typeName := getRuntimeTypeName(left)

			// Find isEmpty (in Optional or its super trait Empty)
			isEmptyMethod, hasIsEmpty := e.lookupTraitMethod("Optional", "isEmpty", typeName)
			if hasIsEmpty {
				isEmpty := e.ApplyFunction(isEmptyMethod, []Object{left})
				if isError(isEmpty) {
					return isEmpty
				}
				if boolVal, ok := isEmpty.(*Boolean); ok && boolVal.Value {
					// Empty: evaluate and return right (short-circuit)
					return e.Eval(node.Right, env)
				}
			}

			// Not empty: call unwrap
			if unwrapMethod, hasUnwrap := e.lookupTraitMethod("Optional", "unwrap", typeName); hasUnwrap {
				return e.ApplyFunction(unwrapMethod, []Object{left})
			}

			// No Optional instance: return left as-is
			return left
		}

		// Pipe operator: x |> f  is equivalent to f(x)
		// x |> f(a) is equivalent to f(a, x)
		// x |> f(_, a) is equivalent to f(x, a)
		if node.Operator == "|>" {
			left := e.Eval(node.Left, env)
			if isError(left) {
				return left
			}

			// Check if right side is a call expression for special handling
			if callExpr, ok := node.Right.(*ast.CallExpression); ok {
				// Evaluate function
				fn := e.Eval(callExpr.Function, env)
				if isError(fn) {
					return fn
				}

				// Collect arguments, handling placeholder
				args := make([]Object, 0, len(callExpr.Arguments)+1)
				placeholderFound := false

				for _, argExpr := range callExpr.Arguments {
					// Check for placeholder
					if ident, ok := argExpr.(*ast.Identifier); ok && ident.Value == "_" {
						if !placeholderFound {
							args = append(args, left)
							placeholderFound = true
							continue
						} else {
							return newError("multiple placeholders in pipe expression not supported")
						}
					}

					val := e.Eval(argExpr, env)
					if isError(val) {
						return val
					}
					args = append(args, val)
				}

				// If no placeholder, append piped value to end
				if !placeholderFound {
					args = append(args, left)
				}

				// Push call frame
				funcName := getFunctionName(fn)
				tok := callExpr.GetToken()
				e.PushCall(funcName, e.CurrentFile, tok.Line, tok.Column)
				result := e.ApplyFunction(fn, args)
				e.PopCall()
				return result
			}

			// Standard pipe: x |> f -> f(x)
			fn := e.Eval(node.Right, env)
			if isError(fn) {
				return fn
			}
			// Push call frame for proper stack trace in debug/trace
			funcName := getFunctionName(fn)
			tok := node.GetToken()
			e.PushCall(funcName, e.CurrentFile, tok.Line, tok.Column)
			result := e.ApplyFunction(fn, []Object{left})
			e.PopCall()
			return result
		}

		// Pipe + unwrap operator: x |>> f  is equivalent to unwrap(f(x))
		// For Result: panics on Fail, returns Ok value
		// For Option: panics on None, returns Some value
		// For other types: pass through (acts like |>)
		if node.Operator == "|>>" {
			left := e.Eval(node.Left, env)
			if isError(left) {
				return left
			}

			var result Object

			// Check if right side is a call expression for special handling
			if callExpr, ok := node.Right.(*ast.CallExpression); ok {
				fn := e.Eval(callExpr.Function, env)
				if isError(fn) {
					return fn
				}

				args := make([]Object, 0, len(callExpr.Arguments)+1)
				placeholderFound := false

				for _, argExpr := range callExpr.Arguments {
					if ident, ok := argExpr.(*ast.Identifier); ok && ident.Value == "_" {
						if !placeholderFound {
							args = append(args, left)
							placeholderFound = true
							continue
						} else {
							return newError("multiple placeholders in pipe expression not supported")
						}
					}

					val := e.Eval(argExpr, env)
					if isError(val) {
						return val
					}
					args = append(args, val)
				}

				if !placeholderFound {
					args = append(args, left)
				}

				funcName := getFunctionName(fn)
				tok := callExpr.GetToken()
				e.PushCall(funcName, e.CurrentFile, tok.Line, tok.Column)
				result = e.ApplyFunction(fn, args)
				e.PopCall()
			} else {
				fn := e.Eval(node.Right, env)
				if isError(fn) {
					return fn
				}
				funcName := getFunctionName(fn)
				tok := node.GetToken()
				e.PushCall(funcName, e.CurrentFile, tok.Line, tok.Column)
				result = e.ApplyFunction(fn, []Object{left})
				e.PopCall()
			}

			if isError(result) {
				return result
			}

			// Unwrap Result/Option
			return pipeUnwrap(result)
		}

		// Composition operator: f ,, g creates a new function (x) -> f(g(x))
		if node.Operator == ",," {
			f := e.Eval(node.Left, env)
			if isError(f) {
				return f
			}
			g := e.Eval(node.Right, env)
			if isError(g) {
				return g
			}
			return &ComposedFunction{F: f, G: g, Evaluator: e}
		}

		// Function application operator: f $ x is equivalent to f(x)
		if node.Operator == "$" {
			fn := e.Eval(node.Left, env)
			if isError(fn) {
				return fn
			}
			arg := e.Eval(node.Right, env)
			if isError(arg) {
				return arg
			}
			return e.ApplyFunction(fn, []Object{arg})
		}

		// Standard evaluation for other operators
		left := e.Eval(node.Left, env)
		if isError(left) {
			return left
		}
		right := e.Eval(node.Right, env)
		if isError(right) {
			return right
		}
		res := e.EvalInfixExpression(node.Operator, left, right)

		// Improve error location for runtime errors (like division by zero)
		// If the error originated here (not deeper in the stack), update line/col
		if err, ok := res.(*Error); ok {
			// If stack trace length matches current stack length, it means no new frames were pushed
			// (or they were popped), implying the error occurred in this context (e.g. primitive op).
			// We compare <= because deeper calls would have LARGER stack trace.
			if len(err.StackTrace) <= len(e.CallStack) {
				err.Line = node.Token.Line
				// VM runtime errors often report column 0. Match this behavior for compatibility.
				err.Column = 0
				// Also update the top frame of the stack trace to reflect the instruction location
				if len(err.StackTrace) > 0 {
					err.StackTrace[len(err.StackTrace)-1].Line = node.Token.Line
					err.StackTrace[len(err.StackTrace)-1].Column = 0
				}
			}
		}
		return res
	case *ast.PostfixExpression:
		left := e.Eval(node.Left, env)
		if isError(left) {
			return left
		}
		return e.evalPostfixExpression(node.Operator, left)
	case *ast.IfExpression:
		return e.evalIfExpression(node, env)
	case *ast.MatchExpression:
		return e.evalMatchExpression(node, env)
	case *ast.Identifier:
		return e.evalIdentifier(node, env)
	case *ast.AssignExpression:
		return e.evalAssignExpression(node, env)
	case *ast.PatternAssignExpression:
		return e.evalPatternAssignExpression(node, env)
	case *ast.CallExpression:
		return e.evalCallExpression(node, env)
	case *ast.TypeApplicationExpression:
		fnObj := e.Eval(node.Expression, env)
		if isError(fnObj) {
			return fnObj
		}

		// Evaluate witnesses if present (resolved by Analyzer)
		if len(node.Witnesses) > 0 {
			var witnesses []Object
			for _, w := range node.Witnesses {
				wObj := e.Eval(w, env)
				if isError(wObj) {
					return wObj
				}
				witnesses = append(witnesses, wObj)
			}

			// Return PartialApplication binding the witnesses
			pa := &PartialApplication{
				AppliedArgs: witnesses,
			}

			switch f := fnObj.(type) {
			case *Function:
				pa.Function = f
			case *Builtin:
				pa.Builtin = f
			case *ClassMethod:
				pa.ClassMethod = f
			case *Constructor:
				pa.Constructor = f
			default:
				return newError("cannot apply types (witnesses) to %s", fnObj.Type())
			}
			return pa
		}

		// If no witnesses, just return the function (generics erased)
		return fnObj
	case *ast.AnnotatedExpression:
		// AnnotatedExpression is a wrapper for type checking
		// Set type context BEFORE evaluating, so ClassMethod calls can dispatch by annotation type
		oldCallNode := e.CurrentCallNode
		e.CurrentCallNode = node

		// Push expected type name to stack for nested calls (e.g. CallExpression inside AnnotatedExpression)
		// This fixes dispatch for (mempty : Attempt<Int>) where mempty is parsed as CallExpression
		if node.TypeAnnotation != nil {
			typeName := extractTypeNameFromAST(node.TypeAnnotation)
			if typeName != "" {
				e.TypeContextStack = append(e.TypeContextStack, typeName)
			}
		}

		// Extract witness from TypeMap if available
		// This enables pure(10) inside w: Writer<MySum, Int> = pure(10) to get the correct witness
		var pushedWitness bool

		// Proactive Push: Use AST annotation directly if available (Robustness)
		if node.TypeAnnotation != nil {
			sysType := astTypeToTypesystem(node.TypeAnnotation)
			// Resolve generics using Env (e.g. t -> Int)
			resolvedType := e.resolveTypeFromEnv(sysType, env)

			witness := make(map[string][]typesystem.Type)
			// Generic context dispatch: pass expected result type
			witness["$ContextType"] = []typesystem.Type{resolvedType}
			// Also push general return context for backward compatibility
			witness["$Return"] = []typesystem.Type{resolvedType}

			e.PushWitness(witness)
			pushedWitness = true
		} else if e.TypeMap != nil {
			// Fallback to TypeMap if no explicit annotation in AST (inferred types)
			if annotatedType := e.TypeMap[node]; annotatedType != nil {
				// Check if annotated type implements Applicative (for pure, etc.)
				// We'll create a witness map for Applicative trait
				witnesses := make(map[string][]typesystem.Type)
				// For now, assume if it's a generic type, it might implement Applicative
				if _, ok := annotatedType.(typesystem.TApp); ok {
					witnesses["Applicative"] = []typesystem.Type{annotatedType}
					e.PushWitness(witnesses)
					pushedWitness = true
				} else if _, ok := annotatedType.(typesystem.TCon); ok {
					// For simple types, check if they're known Applicative instances
					witnesses["Applicative"] = []typesystem.Type{annotatedType}
					e.PushWitness(witnesses)
					pushedWitness = true
				}
			}
		}

		val := e.Eval(node.Expression, env)

		// Pop expected type from stack
		if node.TypeAnnotation != nil {
			typeName := extractTypeNameFromAST(node.TypeAnnotation)
			if typeName != "" && len(e.TypeContextStack) > 0 {
				e.TypeContextStack = e.TypeContextStack[:len(e.TypeContextStack)-1]
			}
		}

		// Pop witness if we pushed it
		if pushedWitness {
			e.PopWitness()
		}

		// Restore previous call node
		e.CurrentCallNode = oldCallNode

		if isError(val) {
			return val
		}

		// If value is a nullary ClassMethod (Arity == 0), auto-call with type context
		if cm, ok := val.(*ClassMethod); ok && cm.Arity == 0 {
			e.CurrentCallNode = node
			// Push witness again for the auto-call
			if e.TypeMap != nil {
				if annotatedType := e.TypeMap[node]; annotatedType != nil {
					witnesses := make(map[string][]typesystem.Type)
					witnesses["Applicative"] = []typesystem.Type{annotatedType}
					e.PushWitness(witnesses)
					defer e.PopWitness()
				}
			}
			result := e.ApplyFunction(cm, []Object{})
			if !isError(result) {
				val = result
			}
		}

		// For Lists, preserve the element type from annotation
		if list, ok := val.(*List); ok {
			if elemType := extractListElementType(node.TypeAnnotation); elemType != "" {
				list.ElementType = elemType
			}
		}
		// If value is a RecordInstance and type annotation is a named type, set TypeName
		if record, ok := val.(*RecordInstance); ok {
			// Handle simple named type (e.g. Point)
			if namedType, ok := node.TypeAnnotation.(*ast.NamedType); ok {
				record.TypeName = namedType.Name.Value
			}
			// Handle generic named type (e.g. Box<Int>)
			// We only set the base TypeName ("Box") because runtime erasure
			// TApp also has Constructor which is usually NamedType or TCon
			// AST node for Box<Int> is NamedType with Args
			// No change needed for AST NamedType structure (it includes Args)
		}
		return val
	case *ast.SpreadExpression:
		// SpreadExpression evaluated in isolation just evaluates its inner expression
		// This allows it to be used, but typically it's handled by evalExpressions contextually.
		// If called directly, just unwrap.
		return e.Eval(node.Expression, env)
	case *ast.FunctionLiteral:
		// Capture current WitnessStack
		// We must copy it to prevent mutation of the captured stack
		var capturedStack []map[string][]typesystem.Type
		if len(e.WitnessStack) > 0 {
			capturedStack = make([]map[string][]typesystem.Type, len(e.WitnessStack))
			copy(capturedStack, e.WitnessStack)
		}

		return &Function{
			Name:                 "", // Lambda has no name
			Parameters:           node.Parameters,
			WitnessParams:        node.WitnessParams,
			ReturnType:           node.ReturnType,
			Body:                 node.Body,
			Env:                  env,           // Capture closure
			CapturedWitnessStack: capturedStack, // Capture witness stack
			Line:                 node.Token.Line,
			Column:               node.Token.Column,
		}
	case *ast.ForExpression:
		return e.evalForExpression(node, env)
	case *ast.RangeExpression:
		return e.evalRangeExpression(node, env)
	case *ast.BreakStatement:
		return e.evalBreakStatement(node, env)
	case *ast.ContinueStatement:
		return e.evalContinueStatement(node, env)
	case *ast.ReturnStatement:
		return e.evalReturnStatement(node, env)
	}

	return nil
}

// getDispatchTypeName returns the base type name for dispatch.
// For List<Int>, returns "List". For Option<String>, returns "Option".
// This matches how keys are stored in ClassImplementations for generic types.
func (e *Evaluator) getDispatchTypeName(obj Object) string {
	typeName := getRuntimeTypeName(obj)
	// Check if it's a generic type (e.g. List<Int>) and extract the base
	if idx := strings.Index(typeName, "<"); idx > 0 {
		return typeName[:idx]
	}
	return typeName
}

// getDefaultForType returns the default value for a type
// First tries built-in defaults (fast path for primitives), then user-defined getDefault
// GetDefaultForType returns the default value for a type (exported for VM)
func (e *Evaluator) GetDefaultForType(t typesystem.Type) Object {
	// For type aliases, resolve to underlying type first
	if tcon, ok := t.(typesystem.TCon); ok {
		if e.TypeAliases != nil {
			if underlying, exists := e.TypeAliases[tcon.Name]; exists {
				// Get default for underlying type but wrap in RecordInstance with TypeName
				result := getDefaultValue(underlying)
				if ri, ok := result.(*RecordInstance); ok {
					ri.TypeName = tcon.Name // Preserve the alias name
				}
				if _, isError := result.(*Error); !isError {
					return result
				}
			}
		}
	}

	// Try hardcoded defaults first (fast path for primitives)
	result := getDefaultValue(t)
	if _, isError := result.(*Error); !isError {
		return result
	}

	// Fallback to user-defined getDefault via trait system
	typeName := e.getTypeNameForDefault(t)
	if typeName != "" {
		if traitResult := e.tryDefaultMethod(typeName); traitResult != nil {
			return traitResult
		}
	}

	// Return original error
	return result
}

func (e *Evaluator) getTypeNameForDefault(t typesystem.Type) string {
	switch typ := t.(type) {
	case typesystem.TCon:
		return typ.Name
	case typesystem.TApp:
		if con, ok := typ.Constructor.(typesystem.TCon); ok {
			return con.Name
		}
	}
	return ""
}

func (e *Evaluator) tryDefaultMethod(typeName string) Object {
	// Look for Default trait implementation with getDefault method
	if typesMap, ok := e.ClassImplementations["Default"]; ok {
		if methodTableObj, ok := typesMap[typeName]; ok {
			if methodTable, ok := methodTableObj.(*MethodTable); ok {
				if method, ok := methodTable.Methods["getDefault"]; ok {
					// getDefault needs a dummy argument - create nil as placeholder
					return e.ApplyFunction(method, []Object{&Nil{}})
				}
			}
		}
	}
	return nil
}

// CompareValues compares two values using the given operator (exported for VM)
func (e *Evaluator) CompareValues(a, b Object, operator string) Object {
	return e.EvalInfixExpression(operator, a, b)
}

// ApplyFunction applies a function to arguments (exported for VM)

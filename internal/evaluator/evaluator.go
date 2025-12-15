package evaluator

import (
	"io"
	"os"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/typesystem"
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
	Out io.Writer
	// Registry for class implementations.
	// Map: ClassName -> TypeName -> FunctionObject
	ClassImplementations map[string]map[string]Object
	// Registry for extension methods.
	// Map: TypeName -> MethodName -> FunctionObject
	ExtensionMethods map[string]map[string]*Function
	// Loader for modules
	Loader ModuleLoader
	// BaseDir for import resolution (optional)
	BaseDir string
	// ModuleCache to avoid re-evaluating modules
	ModuleCache map[string]Object
	// Trait default implementations: "TraitName.methodName" -> FunctionStatement
	TraitDefaults map[string]*ast.FunctionStatement
	// Global environment (for default implementations to access trait methods)
	GlobalEnv *Environment
	// Operator -> Trait mapping for dispatch: "+" -> "Add", "==" -> "Equal"
	OperatorTraits map[string]string
	// TypeMap from analyzer - maps AST nodes to their inferred types
	TypeMap map[ast.Node]typesystem.Type
	// CurrentCallNode - temporarily stores the AST node being evaluated
	// Used for type-based dispatch (e.g., pure/mempty)
	CurrentCallNode ast.Node
	// CallStack for stack traces on errors
	CallStack []CallFrame
	// CurrentFile being evaluated
	CurrentFile string
	// ContainerContext tracks the expected container type for `pure` when inside >>=
	// e.g., when evaluating `Some(42) >>= pure`, this is set to "Option"
	ContainerContext string
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

	// Fork creates a thread-safe copy of the evaluator for background execution
	Fork func() *Evaluator
}

// ModuleLoader interface (same as in Analyzer, should probably be in a common package)
type ModuleLoader interface {
	GetModule(path string) (interface{}, error)
}

type LoadedModule interface {
	GetExports() map[string]Object
}

func New() *Evaluator {
	return &Evaluator{
		Out:                  os.Stdout,
		ClassImplementations: make(map[string]map[string]Object),
		ExtensionMethods:     make(map[string]map[string]*Function),
		ModuleCache:          make(map[string]Object),
		TypeMap:              make(map[ast.Node]typesystem.Type),
		TypeAliases:          make(map[string]typesystem.Type),
	}
}

// lookupTraitMethod looks up a method for a type in a trait, including super traits.
// Returns the method and true if found, nil and false otherwise.
func (e *Evaluator) lookupTraitMethod(traitName, typeName, methodName string) (Object, bool) {
	// Check the trait itself
	if typesMap, ok := e.ClassImplementations[traitName]; ok {
		if methodTableObj, ok := typesMap[typeName]; ok {
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
			if method, ok := e.lookupTraitMethod(superTrait, typeName, methodName); ok {
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
		Out:                  e.Out,
		ClassImplementations: e.ClassImplementations, // shared, read-only in tasks
		ExtensionMethods:     e.ExtensionMethods,     // shared, read-only in tasks
		Loader:               e.Loader,
		BaseDir:              e.BaseDir,
		ModuleCache:          e.ModuleCache, // shared
		TraitDefaults:        e.TraitDefaults,
		GlobalEnv:            e.GlobalEnv,
		OperatorTraits:       e.OperatorTraits,
		TypeMap:              e.TypeMap,
		CurrentCallNode:      nil,                  // new per goroutine
		CallStack:            make([]CallFrame, 0), // new per goroutine
		CurrentFile:          e.CurrentFile,
		ContainerContext:     "",
		TypeAliases:          e.TypeAliases,   // shared, read-only
		VMCallHandler:        e.VMCallHandler, // shared, for calling VM closures from builtins
		CaptureHandler:       e.CaptureHandler, // shared
	}
}

func (e *Evaluator) Eval(node ast.Node, env *Environment) Object {
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
			Name:       node.Name.Value,
			Parameters: node.Parameters,
			ReturnType: node.ReturnType,
			Body:       node.Body,
			Env:        env, // Closure
			Line:       node.Token.Line,
			Column:     node.Token.Column,
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
				return FALSE // Short-circuit: false && _ = false
			}
			right := e.Eval(node.Right, env)
			if isError(right) {
				return right
			}
			return e.nativeBoolToBooleanObject(e.isTruthy(right))
		}
		if node.Operator == "||" {
			left := e.Eval(node.Left, env)
			if isError(left) {
				return left
			}
			if e.isTruthy(left) {
				return TRUE // Short-circuit: true || _ = true
			}
			right := e.Eval(node.Right, env)
			if isError(right) {
				return right
			}
			return e.nativeBoolToBooleanObject(e.isTruthy(right))
		}

		// Null coalescing: x ?? default (via Optional trait)
		// Some(x) ?? y = x, Zero ?? y = y
		// Ok(x) ?? y = x, Fail(_) ?? y = y
		if node.Operator == "??" {
			left := e.Eval(node.Left, env)
			if isError(left) {
				return left
			}

			// Use Optional trait for dispatch (includes super traits like Empty)
			typeName := getRuntimeTypeName(left)

			// Find isEmpty (in Optional or its super trait Empty)
			isEmptyMethod, hasIsEmpty := e.lookupTraitMethod("Optional", typeName, "isEmpty")
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
			if unwrapMethod, hasUnwrap := e.lookupTraitMethod("Optional", typeName, "unwrap"); hasUnwrap {
				return e.ApplyFunction(unwrapMethod, []Object{left})
			}

			// No Optional instance: return left as-is
			return left
		}

		// Pipe operator: x |> f  is equivalent to f(x)
		if node.Operator == "|>" {
			left := e.Eval(node.Left, env)
			if isError(left) {
				return left
			}
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
		return e.EvalInfixExpression(node.Operator, left, right)
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
		// Evaluator treats generics as erased or pass-through
		// Just evaluate the inner expression.
		return e.Eval(node.Expression, env)
	case *ast.AnnotatedExpression:
		// AnnotatedExpression is a wrapper for type checking
		// Set type context BEFORE evaluating, so ClassMethod calls can dispatch by annotation type
		oldCallNode := e.CurrentCallNode
		e.CurrentCallNode = node

		val := e.Eval(node.Expression, env)

		// Restore previous call node
		e.CurrentCallNode = oldCallNode

		if isError(val) {
			return val
		}

		// If value is a nullary ClassMethod (Arity == 0), auto-call with type context
		if cm, ok := val.(*ClassMethod); ok && cm.Arity == 0 {
			e.CurrentCallNode = node
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
		return val
	case *ast.SpreadExpression:
		// SpreadExpression evaluated in isolation just evaluates its inner expression
		// This allows it to be used, but typically it's handled by evalExpressions contextually.
		// If called directly, just unwrap.
		return e.Eval(node.Expression, env)
	case *ast.FunctionLiteral:
		return &Function{
			Name:       "", // Lambda has no name
			Parameters: node.Parameters,
			ReturnType: node.ReturnType,
			Body:       node.Body,
			Env:        env, // Capture closure
			Line:       node.Token.Line,
			Column:     node.Token.Column,
		}
	case *ast.ForExpression:
		return e.evalForExpression(node, env)
	case *ast.BreakStatement:
		return e.evalBreakStatement(node, env)
	case *ast.ContinueStatement:
		return e.evalContinueStatement(node, env)
	}

	return nil
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

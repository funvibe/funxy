package analyzer

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

// InferenceContext holds the state for a type inference pass.
// Using a context instead of global state ensures predictable type variable names
// and allows for proper scoping in tests and parallel compilation.
type InferenceContext struct {
	counter int
	TypeMap map[ast.Node]typesystem.Type
	// ActiveConstraints maps type variable names to their constraints
	// e.g. {"T": ["Order", "Equal"]} means T: Order, T: Equal
	ActiveConstraints map[string][]string
	// Loader for looking up extension methods and traits in source modules
	Loader ModuleLoader
}

// NewInferenceContext creates a new inference context.
func NewInferenceContext() *InferenceContext {
	return &InferenceContext{
		counter:           0,
		TypeMap:           make(map[ast.Node]typesystem.Type),
		ActiveConstraints: make(map[string][]string),
	}
}

// NewInferenceContextWithLoader creates a new inference context with module loader.
func NewInferenceContextWithLoader(loader ModuleLoader) *InferenceContext {
	return &InferenceContext{
		counter:           0,
		TypeMap:           make(map[ast.Node]typesystem.Type),
		ActiveConstraints: make(map[string][]string),
		Loader:            loader,
	}
}

// HasConstraint checks if a type variable has a specific constraint
func (ctx *InferenceContext) HasConstraint(typeVarName, traitName string) bool {
	if constraints, ok := ctx.ActiveConstraints[typeVarName]; ok {
		for _, c := range constraints {
			if c == traitName {
				return true
			}
		}
	}
	return false
}

// AddConstraint adds a constraint for a type variable
func (ctx *InferenceContext) AddConstraint(typeVarName, traitName string) {
	ctx.ActiveConstraints[typeVarName] = append(ctx.ActiveConstraints[typeVarName], traitName)
}

// NewInferenceContextWithTypeMap creates a context with an existing TypeMap.
// It scans the TypeMap for existing type variables and sets the counter
// to avoid collisions with existing names.
func NewInferenceContextWithTypeMap(typeMap map[ast.Node]typesystem.Type) *InferenceContext {
	if typeMap == nil {
		typeMap = make(map[ast.Node]typesystem.Type)
	}

	// Find the highest existing type variable number to avoid collisions
	maxCounter := 0
	for _, t := range typeMap {
		maxCounter = maxInt(maxCounter, findMaxTVarNumber(t))
	}

	return &InferenceContext{
		counter: maxCounter,
		TypeMap: typeMap,
	}
}

// findMaxTVarNumber finds the highest tN number in a type
func findMaxTVarNumber(t typesystem.Type) int {
	if t == nil {
		return 0
	}
	max := 0
	for _, tv := range t.FreeTypeVariables() {
		var num int
		if _, err := fmt.Sscanf(tv.Name, "t%d", &num); err == nil {
			if num > max {
				max = num
			}
		}
	}
	return max
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// FreshVar generates a fresh type variable with a unique name.
func (ctx *InferenceContext) FreshVar() typesystem.TVar {
	ctx.counter++
	name := fmt.Sprintf("t%d", ctx.counter)
	return typesystem.TVar{Name: name}
}

// Reset resets the counter (useful for testing).
func (ctx *InferenceContext) Reset() {
	ctx.counter = 0
}

// standaloneContext is used by external callers (like the compiler) that don't
// participate in the main analysis pass but need to instantiate generic types.
// Note: This is a fresh context per call to avoid non-determinism issues.
func getStandaloneContext() *InferenceContext {
	return NewInferenceContext()
}

// Instantiate replaces all free type variables in t with fresh type variables.
// This is primarily for external callers like the compiler.
// Within the analyzer, use InstantiateWithContext instead.
func Instantiate(t typesystem.Type) typesystem.Type {
	return InstantiateWithContext(getStandaloneContext(), t)
}

// Infer computes the type of an expression and returns a substitution map.
func Infer(node ast.Node, table *symbols.SymbolTable, typeMap map[ast.Node]typesystem.Type) (typesystem.Type, typesystem.Subst, error) {
	ctx := NewInferenceContextWithTypeMap(typeMap)
	return InferWithContext(ctx, node, table)
}

// InferWithContext computes the type using an explicit inference context.
func InferWithContext(ctx *InferenceContext, node ast.Node, table *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error) {
	// Helper to wrap recursive calls
	recursiveInfer := func(n ast.Node, t *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error) {
		return InferWithContext(ctx, n, t)
	}

	var resultType typesystem.Type
	var subst typesystem.Subst
	var err error

	switch n := node.(type) {
	case *ast.AnnotatedExpression:
		resultType, subst, err = inferAnnotatedExpression(ctx, n, table, recursiveInfer)

	case *ast.IntegerLiteral, *ast.FloatLiteral, *ast.BigIntLiteral, *ast.RationalLiteral,
		*ast.TupleLiteral, *ast.RecordLiteral, *ast.ListLiteral, *ast.MapLiteral, *ast.StringLiteral, *ast.FormatStringLiteral, *ast.InterpolatedString, *ast.CharLiteral, *ast.BytesLiteral, *ast.BitsLiteral, *ast.BooleanLiteral, *ast.NilLiteral:
		resultType, subst, err = inferLiteral(ctx, n, table, recursiveInfer)

	case *ast.Identifier:
		resultType, subst, err = inferIdentifier(ctx, n, table)

	case *ast.IfExpression:
		resultType, subst, err = inferIfExpression(ctx, n, table, recursiveInfer)

	case *ast.FunctionLiteral:
		resultType, subst, err = inferFunctionLiteral(ctx, n, table, recursiveInfer)

	case *ast.MatchExpression:
		resultType, subst, err = inferMatchExpression(ctx, n, table, recursiveInfer, ctx.TypeMap)

	case *ast.AssignExpression:
		resultType, subst, err = inferAssignExpression(ctx, n, table, recursiveInfer, ctx.TypeMap)

	case *ast.PatternAssignExpression:
		resultType, subst, err = inferPatternAssignExpression(ctx, n, table, recursiveInfer)

	case *ast.BlockStatement:
		resultType, subst, err = inferBlockStatement(ctx, n, table, recursiveInfer)

	case *ast.SpreadExpression:
		resultType, subst, err = inferSpreadExpression(ctx, n, table, recursiveInfer)

	case *ast.MemberExpression:
		resultType, subst, err = inferMemberExpression(ctx, n, table, recursiveInfer)

	case *ast.IndexExpression:
		resultType, subst, err = inferIndexExpression(ctx, n, table, recursiveInfer)

	case *ast.CallExpression:
		resultType, subst, err = inferCallExpression(ctx, n, table, recursiveInfer)

	case *ast.PrefixExpression:
		resultType, subst, err = inferPrefixExpression(ctx, n, table, recursiveInfer)

	case *ast.InfixExpression:
		resultType, subst, err = inferInfixExpression(ctx, n, table, recursiveInfer)

	case *ast.OperatorAsFunction:
		resultType, subst, err = inferOperatorAsFunction(ctx, n, table)

	case *ast.PostfixExpression:
		resultType, subst, err = inferPostfixExpression(ctx, n, table, recursiveInfer)

	case *ast.ForExpression:
		resultType, subst, err = inferForExpression(ctx, n, table, recursiveInfer)

	case *ast.BreakStatement:
		resultType, subst, err = inferBreakStatement(ctx, n, table, recursiveInfer)

	case *ast.ContinueStatement:
		resultType, subst, err = inferContinueStatement(ctx, n)
	}

	if resultType != nil {
		if ctx.TypeMap != nil {
			// Only store if not already present - avoid overwriting resolved types
			if _, exists := ctx.TypeMap[node]; !exists {
				ctx.TypeMap[node] = resultType
			}
		}
		if subst == nil {
			subst = typesystem.Subst{}
		}
		return resultType, subst, nil
	}

	if err != nil {
		return nil, nil, err
	}

	// Handle nil node case (can happen when parser creates nil due to errors)
	if node == nil {
		return nil, nil, fmt.Errorf("[analyzer] error [A003]: type error: unknown node type for inference: <nil>")
	}

	return nil, nil, inferErrorf(node, "unknown node type for inference: %T", node)
}

// InstantiateWithContext replaces all free type variables with fresh ones using the given context.
func InstantiateWithContext(ctx *InferenceContext, t typesystem.Type) typesystem.Type {
	if t == nil {
		return nil
	}
	vars := t.FreeTypeVariables()
	if len(vars) == 0 {
		return t
	}

	subst := typesystem.Subst{}
	for _, v := range vars {
		subst[v.Name] = ctx.FreshVar()
	}
	return t.Apply(subst)
}

func getCanonicalTypeName(t typesystem.Type) string {
	switch t := t.(type) {
	case typesystem.TCon:
		return t.Name
	case typesystem.TApp:
		if tCon, ok := t.Constructor.(typesystem.TCon); ok {
			return tCon.Name
		}
		return getCanonicalTypeName(t.Constructor)
	case typesystem.TRecord:
		return "RECORD"
	case typesystem.TTuple:
		return "TUPLE"
	default:
		return ""
	}
}

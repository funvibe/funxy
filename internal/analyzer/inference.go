package analyzer

// Force update
import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
	"sort"
)

// InferenceContext holds the state for a type inference pass.
// Using a context instead of global state ensures predictable type variable names
// and allows for proper scoping in tests and parallel compilation.
type InferenceContext struct {
	counter int
	TypeMap map[ast.Node]typesystem.Type
	// ActiveConstraints maps type variable names to their constraints
	// e.g. {"T": [Constraint{Trait: "Order"}, Constraint{Trait: "Equal"}]}
	ActiveConstraints map[string][]Constraint
	// ExpectedTypes maps variable definition nodes (ast.Identifier) to their expected types from usage context
	// Used for look-ahead type inference (e.g., p = pure(5) where p is later used as Reader<Int, Int>)
	ExpectedTypes map[ast.Node]typesystem.Type
	// ExpectedReturnTypes maps call expression nodes to their expected return types
	// Used for inferring trait method return types (e.g., pure(x+y) in >>= context)
	ExpectedReturnTypes map[ast.Node]typesystem.Type
	// Loader for looking up extension methods and traits in source modules
	Loader ModuleLoader
	// CurrentModuleName for checking imported symbol reassignment
	CurrentModuleName string
	// GlobalSubst stores the accumulated substitution for the entire inference pass
	GlobalSubst typesystem.Subst
	// PendingWitnesses stores witnesses that need to be resolved after inference
	PendingWitnesses []PendingWitness
	// Constraints stores accumulated type constraints to be solved later
	Constraints []Constraint
	// InferredConstraints stores constraints inferred from usage of rigid type variables
	// These should be added to the function signature.
	InferredConstraints []Constraint
	// BaseCounter tracks the counter start value for this context
	// Used to distinguish generic parameters (created before) from inference variables (created during this session)
	BaseCounter int
}

// PendingWitness represents a trait constraint that needs a witness
type PendingWitness struct {
	Node    *ast.CallExpression
	Trait   string
	TypeVar string            // The name of the Type Variable (e.g. "t1") that needs to be resolved
	Args    []typesystem.Type // Arguments for the trait (including the one corresponding to TypeVar)
	Index   int               // Index in the Witnesses slice
}

// NewInferenceContext creates a new inference context.
func NewInferenceContext() *InferenceContext {
	return &InferenceContext{
		counter:             0,
		BaseCounter:         0,
		TypeMap:             make(map[ast.Node]typesystem.Type),
		ActiveConstraints:   make(map[string][]Constraint),
		ExpectedTypes:       make(map[ast.Node]typesystem.Type),
		ExpectedReturnTypes: make(map[ast.Node]typesystem.Type),
		GlobalSubst:         make(typesystem.Subst),
		PendingWitnesses:    make([]PendingWitness, 0),
		Constraints:         make([]Constraint, 0),
	}
}

// NewInferenceContextWithLoader creates a new inference context with module loader.
func NewInferenceContextWithLoader(loader ModuleLoader) *InferenceContext {
	return &InferenceContext{
		counter:             0,
		BaseCounter:         0,
		TypeMap:             make(map[ast.Node]typesystem.Type),
		ActiveConstraints:   make(map[string][]Constraint),
		ExpectedTypes:       make(map[ast.Node]typesystem.Type),
		ExpectedReturnTypes: make(map[ast.Node]typesystem.Type),
		Loader:              loader,
		GlobalSubst:         make(typesystem.Subst),
		PendingWitnesses:    make([]PendingWitness, 0),
		Constraints:         make([]Constraint, 0),
	}
}

// HasConstraint checks if a type variable has a specific constraint (ignoring args check for back-compat)
func (ctx *InferenceContext) HasConstraint(typeVarName, traitName string) bool {
	if constraints, ok := ctx.ActiveConstraints[typeVarName]; ok {
		for _, c := range constraints {
			if c.Trait == traitName {
				return true
			}
		}
	}
	return false
}

// HasMPTCConstraint checks if a type variable has a specific constraint with matching arguments
func (ctx *InferenceContext) HasMPTCConstraint(traitName string, args []typesystem.Type) bool {
	// We check constraints on the first argument (by convention)
	if len(args) == 0 {
		return false
	}

	// Unwrap first arg to get TVar name
	firstArg := typesystem.UnwrapUnderlying(args[0])
	if firstArg == nil {
		firstArg = args[0]
	}

	var name string
	if tv, ok := firstArg.(typesystem.TVar); ok {
		name = tv.Name
	} else if tc, ok := firstArg.(typesystem.TCon); ok {
		name = tc.Name
	} else {
		return false
	}

	if constraints, ok := ctx.ActiveConstraints[name]; ok {
		for _, c := range constraints {
			if c.Trait != traitName {
				continue
			}
			// Check args match
			if len(c.Args) != len(args) {
				continue
			}

			match := true
			for i, cArg := range c.Args {
				// Here we compare types. Since these are in inference context,
				// we assume exact match or unification?
				// For checking "Does T have constraint Convert<U>", we expect
				// types to match exactly (as they are in the context).
				// Or unify?
				// For now, strict equality (String representation or strict check)
				if cArg.String() != args[i].String() {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

// AddConstraint adds a constraint for a type variable
func (ctx *InferenceContext) AddConstraint(typeVarName, traitName string) {
	c := Constraint{
		Kind:  ConstraintImplements,
		Trait: traitName,
		Args:  []typesystem.Type{typesystem.TVar{Name: typeVarName}},
	}
	ctx.ActiveConstraints[typeVarName] = append(ctx.ActiveConstraints[typeVarName], c)
}

// AddMPTCConstraint adds a multi-parameter constraint for a type variable
func (ctx *InferenceContext) AddMPTCConstraint(typeVarName, traitName string, args []typesystem.Type) {
	c := Constraint{
		Kind:  ConstraintImplements,
		Trait: traitName,
		Args:  append([]typesystem.Type{typesystem.TVar{Name: typeVarName}}, args...),
	}
	ctx.ActiveConstraints[typeVarName] = append(ctx.ActiveConstraints[typeVarName], c)
}

// AddDeferredConstraint adds a constraint to be solved later
func (ctx *InferenceContext) AddDeferredConstraint(c Constraint) {
	// Migration: Ensure Args is populated if Left is set
	if c.Kind == ConstraintImplements && len(c.Args) == 0 && c.Left != nil {
		c.Args = []typesystem.Type{c.Left}
	}
	ctx.Constraints = append(ctx.Constraints, c)
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
		counter:             maxCounter,
		BaseCounter:         maxCounter,
		TypeMap:             typeMap,
		ExpectedTypes:       make(map[ast.Node]typesystem.Type),
		ExpectedReturnTypes: make(map[ast.Node]typesystem.Type),
		ActiveConstraints:   make(map[string][]Constraint),
		GlobalSubst:         make(typesystem.Subst),
		PendingWitnesses:    make([]PendingWitness, 0),
		Constraints:         make([]Constraint, 0),
	}
}

// RegisterPendingWitness registers a witness that needs to be resolved later.
func (ctx *InferenceContext) RegisterPendingWitness(node *ast.CallExpression, trait, typeVar string, args []typesystem.Type, index int) {
	ctx.PendingWitnesses = append(ctx.PendingWitnesses, PendingWitness{
		Node:    node,
		Trait:   trait,
		TypeVar: typeVar,
		Args:    args,
		Index:   index,
	})
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

// FreshVar generates a fresh type variable with a unique name and default kind Star.
func (ctx *InferenceContext) FreshVar() typesystem.TVar {
	return ctx.FreshVarWithKind(typesystem.Star)
}

// FreshVarWithKind generates a fresh type variable with a unique name and specific kind.
func (ctx *InferenceContext) FreshVarWithKind(k typesystem.Kind) typesystem.TVar {
	ctx.counter++
	name := fmt.Sprintf("t%d", ctx.counter)
	return typesystem.TVar{Name: name, KindVal: k}
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

// Generalize generalizes the type t by replacing free type variables
// that are not bound in the environment with generic variables.
// It also moves relevant constraints from the context to the type scheme.
func (ctx *InferenceContext) Generalize(t typesystem.Type, table *symbols.SymbolTable, ignoredSymbol string) typesystem.Type {
	// Apply current substitution
	t = t.Apply(ctx.GlobalSubst)

	envVars := getEnvFreeVars(table, ctx.GlobalSubst, ignoredSymbol)
	freeVars := t.FreeTypeVariables()

	subst := make(typesystem.Subst)
	for _, v := range freeVars {
		// If variable is NOT in environment, generalize it
		if !envVars[v.Name] {
			// Create a generic variable name (e.g. "gen_t1")
			var num int
			if _, err := fmt.Sscanf(v.Name, "t%d", &num); err == nil {
				genName := "gen_" + v.Name
				subst[v.Name] = typesystem.TVar{Name: genName, KindVal: v.KindVal}
			}
		}
	}

	if len(subst) == 0 {
		return t
	}

	resType := t.Apply(subst)

	// Move constraints involving generalized variables to the type
	var movedConstraints []typesystem.Constraint
	var remainingConstraints []Constraint

	for _, c := range ctx.Constraints {
		// Only move trait implementation constraints
		if c.Kind != ConstraintImplements {
			remainingConstraints = append(remainingConstraints, c)
			continue
		}

		// Apply global subst to constraint type
		left := c.Left.Apply(ctx.GlobalSubst)

		// Check if constraint depends on any generalized variable
		depends := false
		freeInC := left.FreeTypeVariables()
		for _, fv := range freeInC {
			if _, isGen := subst[fv.Name]; isGen {
				depends = true
				break
			}
		}

		if depends {
			// Apply generalization substitution
			newLeft := left.Apply(subst)

			// Check if we can attach this constraint to TFunc
			if tv, ok := newLeft.(typesystem.TVar); ok {
				movedConstraints = append(movedConstraints, typesystem.Constraint{
					TypeVar: tv.Name,
					Trait:   c.Trait,
				})
			} else {
				// Constraint on complex type (e.g. List<gen_T>: Show)
				// Cannot be stored in simple TFunc.Constraints yet.
				// Keep it in context (will likely fail ambiguity check, but it's the best we can do without TScheme support for complex constraints)
				remainingConstraints = append(remainingConstraints, c)
			}
		} else {
			remainingConstraints = append(remainingConstraints, c)
		}
	}

	ctx.Constraints = remainingConstraints

	if len(movedConstraints) > 0 {
		if tFunc, ok := resType.(typesystem.TFunc); ok {
			tFunc.Constraints = append(tFunc.Constraints, movedConstraints...)
			resType = tFunc
		}

		// Also remove PendingWitnesses that are now covered by the generalized constraints
		var remainingWitnesses []PendingWitness
		for _, pw := range ctx.PendingWitnesses {
			// Check if the witness depends on a generalized variable
			// pw.TypeVar is the name of the variable needing a witness
			if _, isGen := subst[pw.TypeVar]; isGen {
				// The variable is generalized. The witness requirement is now part of the function signature constraints.
				// So we don't need to resolve it globally anymore.
				continue
			}
			remainingWitnesses = append(remainingWitnesses, pw)
		}
		ctx.PendingWitnesses = remainingWitnesses
	}

	// Wrap in TForall if generalized variables exist
	var quantifiedVars []typesystem.TVar
	seenVars := make(map[string]bool)
	for _, val := range subst {
		if tv, ok := val.(typesystem.TVar); ok {
			if !seenVars[tv.Name] {
				quantifiedVars = append(quantifiedVars, tv)
				seenVars[tv.Name] = true
			}
		}
	}

	if len(quantifiedVars) > 0 {
		// Sort for deterministic output
		sort.Slice(quantifiedVars, func(i, j int) bool {
			return quantifiedVars[i].Name < quantifiedVars[j].Name
		})
		return typesystem.TForall{
			Vars: quantifiedVars,
			Type: resType,
		}
	}

	return resType
}

func getEnvFreeVars(table *symbols.SymbolTable, subst typesystem.Subst, ignoredSymbol string) map[string]bool {
	vars := make(map[string]bool)
	current := table

	for current != nil {
		for _, sym := range current.All() {
			if sym.Name == ignoredSymbol {
				continue
			}
			if sym.Kind == symbols.VariableSymbol && sym.Type != nil {
				// Apply current substitution to environment types before checking free vars
				// CRITICAL for Value Restriction / Escape Check:
				// If a variable 'x' in env has type 't1', and 't1' is unified with 't2' (local),
				// then 't2' is effectively bound to 'x' and MUST NOT be generalized.
				t := sym.Type.Apply(subst)
				for _, tv := range t.FreeTypeVariables() {
					vars[tv.Name] = true
				}
			}
		}
		current = current.Parent()
	}
	return vars
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

	case *ast.ListComprehension:
		resultType, subst, err = inferListComprehension(ctx, n, table, recursiveInfer)

	case *ast.RangeExpression:
		resultType, subst, err = inferRangeExpression(ctx, n, table, recursiveInfer)
	}

	if resultType != nil {
		if subst == nil {
			subst = typesystem.Subst{}
		}

		// Update GlobalSubst with the returned substitution
		if ctx.GlobalSubst == nil {
			ctx.GlobalSubst = make(typesystem.Subst)
		}
		// Compose the new substitution with the global one: Global = subst ∘ Global
		// This ensures that existing mappings in Global are updated by new refinements in subst
		ctx.GlobalSubst = subst.Compose(ctx.GlobalSubst)

		if ctx.TypeMap != nil {
			// Always update TypeMap with the latest inferred type
			// This is important because look-ahead might populate TypeMap with preliminary types,
			// but main inference produces more accurate types (and fresh type variables that are actually used/bound).
			ctx.TypeMap[node] = resultType
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
	res, _ := InstantiateGenericsWithSubst(ctx, t)
	return res
}

// InstantiateGenericsWithSubst replaces generic type variables with fresh ones
// and returns the substitution map used.
// It handles both explicit Forall types and legacy implicit generics.
func InstantiateGenericsWithSubst(ctx *InferenceContext, t typesystem.Type) (typesystem.Type, typesystem.Subst) {
	if t == nil {
		return nil, nil
	}

	// Handle explicit Forall types
	if forall, ok := t.(typesystem.TForall); ok {
		return InstantiateForallWithSubst(ctx, forall)
	}

	vars := t.FreeTypeVariables()
	if len(vars) == 0 {
		return t, nil
	}

	subst := typesystem.Subst{}
	for _, v := range vars {
		// Check if it's a generic parameter (old) or inference variable (new)
		var num int
		isGeneric := true
		if _, err := fmt.Sscanf(v.Name, "t%d", &num); err == nil {
			if num > ctx.BaseCounter {
				isGeneric = false
			}
		} else {
			// Non-tN names (e.g. "T", "A") are always considered generic/rigid
			isGeneric = true
		}

		if isGeneric {
			subst[v.Name] = ctx.FreshVarWithKind(v.Kind())
		}
	}

	if len(subst) == 0 {
		return t, nil
	}
	return t.Apply(subst), subst
}

// InstantiateGenerics replaces only generic type variables (those created before the current inference session)
// with fresh type variables. Inference variables created during this session are preserved.
func InstantiateGenerics(ctx *InferenceContext, t typesystem.Type) typesystem.Type {
	// First handle explicit Forall
	if forall, ok := t.(typesystem.TForall); ok {
		return InstantiateForall(ctx, forall)
	}
	// Fallback to old implicit generic handling
	res, _ := InstantiateGenericsWithSubst(ctx, t)
	return res
}

// InstantiateForall instantiates a polytype (forall a. T) with fresh type variables.
// It peels off the quantifier and replaces the bound variables in the body.
func InstantiateForall(ctx *InferenceContext, t typesystem.TForall) typesystem.Type {
	subst := make(typesystem.Subst)
	for _, v := range t.Vars {
		subst[v.Name] = ctx.FreshVarWithKind(v.Kind())
	}

	// Add constraints for fresh variables
	for _, c := range t.Constraints {
		if freshVar, ok := subst[c.TypeVar]; ok {
			if tv, ok := freshVar.(typesystem.TVar); ok {
				ctx.AddConstraint(tv.Name, c.Trait)
			}
		}
	}

	// Apply subst to body
	instantiatedBody := t.Type.Apply(subst)

	// Recurse if body is also a Forall (though usually flattened)
	if nextForall, ok := instantiatedBody.(typesystem.TForall); ok {
		return InstantiateForall(ctx, nextForall)
	}
	return instantiatedBody
}

// InstantiateForallWithSubst instantiates a polytype (forall a. T) with fresh type variables
// and returns the instantiated type and the substitution used.
func InstantiateForallWithSubst(ctx *InferenceContext, t typesystem.TForall) (typesystem.Type, typesystem.Subst) {
	subst := make(typesystem.Subst)
	for _, v := range t.Vars {
		subst[v.Name] = ctx.FreshVarWithKind(v.Kind())
	}

	// Apply subst to body
	instantiatedBody := t.Type.Apply(subst)

	// Recurse if body is also a Forall (though usually flattened)
	if nextForall, ok := instantiatedBody.(typesystem.TForall); ok {
		tNext, sNext := InstantiateForallWithSubst(ctx, nextForall)
		// Compose substitutions (subst first, then sNext)
		// But sNext keys are fresh vars from nextForall, which shouldn't overlap with subst values unless we are careful.
		// Actually, we just need to return the cumulative mapping for monomorphization.
		// However, for recursion, it's complex to merge substs.
		// Since flattened foralls are preferred, this case should be rare or handled by flattening.
		// If we do recurse, we should combine the substitutions.
		// But sNext applies to the RESULT of applying subst.
		// So total subst = subst . sNext?
		// No, subst maps OriginalVar -> FreshVar1
		// sNext maps OriginalVar2 (inside body) -> FreshVar2
		// If nested, they are distinct scopes.
		for k, v := range sNext {
			subst[k] = v
		}
		return tNext, subst
	}
	return instantiatedBody, subst
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

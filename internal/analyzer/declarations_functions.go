package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

func inferKindFromFunction(n *ast.FunctionStatement, tpName string, table *symbols.SymbolTable) typesystem.Kind {
	maxArgs := 0
	for _, p := range n.Parameters {
		if p.Type != nil {
			if c := findMaxTypeArgs(tpName, p.Type); c > maxArgs {
				maxArgs = c
			}
		}
	}
	if n.ReturnType != nil {
		if c := findMaxTypeArgs(tpName, n.ReturnType); c > maxArgs {
			maxArgs = c
		}
	}
	if n.Receiver != nil && n.Receiver.Type != nil {
		if c := findMaxTypeArgs(tpName, n.Receiver.Type); c > maxArgs {
			maxArgs = c
		}
	}

	var kindFromUsage typesystem.Kind
	if maxArgs == 0 {
		kindFromUsage = typesystem.Star
	} else {
		kinds := make([]typesystem.Kind, maxArgs+1)
		for i := 0; i <= maxArgs; i++ {
			kinds[i] = typesystem.Star
		}
		kindFromUsage = typesystem.MakeArrow(kinds...)
	}

	// Infer from Constraints
	var kindFromConstraints typesystem.Kind

	for _, c := range n.Constraints {
		paramIndex := -1

		// Check if the constraint matches our type parameter
		if c.TypeVar == tpName {
			paramIndex = 0
		} else {
			// Check arguments (for MPTC)
			for i, arg := range c.Args {
				// We simple-mindedly look for the type parameter name in arguments.
				// This assumes arguments are simple type variables in this context,
				// which is true for class definitions / generic function constraints usually.
				if nt, ok := arg.(*ast.NamedType); ok && nt.Name.Value == tpName {
					paramIndex = i + 1
					break
				}
			}
		}

		if paramIndex != -1 {
			// Look up the trait
			if typeParams, ok := table.GetTraitTypeParams(c.Trait); ok {
				if paramIndex < len(typeParams) {
					traitParamName := typeParams[paramIndex]
					if k, ok := table.GetTraitTypeParamKind(c.Trait, traitParamName); ok {
						// Found a kind from constraint!
						// If we have multiple constraints, prefer the one with higher complexity (Arrow over Star)
						if kindFromConstraints == nil {
							kindFromConstraints = k
						} else {
							// Simple heuristic: Arrow kind is "more information" than Star
							if _, isArrow := k.(typesystem.KArrow); isArrow {
								kindFromConstraints = k
							}
						}
					}
				}
			}
		}
	}

	// Determine Kind based on constraints and usage
	// If we have explicit constraints, they are usually the source of truth for Kinds (e.g. T: Functor => *->*).
	// However, if usage implies a different structure, we might have a mismatch.
	// For now, we trust the constraint if it provides a higher-kinded type.
	if kindFromConstraints != nil {
		// Optimization: If constraint says Star, but usage says Arrow, usage wins (because T<A> means T is *->*)
		// But if constraint says Arrow, and usage says Star (e.g. passed to function), Arrow wins.
		_, usageIsArrow := kindFromUsage.(typesystem.KArrow)
		_, constraintIsArrow := kindFromConstraints.(typesystem.KArrow)

		if usageIsArrow && !constraintIsArrow {
			return kindFromUsage
		}
		return kindFromConstraints
	}

	return kindFromUsage
}

func (w *walker) VisitFunctionStatement(n *ast.FunctionStatement) {
	// Skip if function was not properly parsed
	if n == nil || n.Name == nil {
		return
	}

	// Mode Checks
	if w.mode == ModeNaming || (w.mode == ModeInstances && !w.inInstance) {
		return
	}

	// Register Generic Constraints / Type Params (Temporarily in scope for signature building)
	// Create a temporary scope for building the signature.
	// This ensures type params (both explicit and implicit) are captured and resolved correctly.
	sigScope := symbols.NewEnclosedSymbolTable(w.symbolTable, symbols.ScopeFunction) // Function signature scope

	// Register explicit type params first
	for _, tp := range n.TypeParams {
		// Infer Kind from usage in signature AND constraints
		kind := inferKindFromFunction(n, tp.Value, w.symbolTable)
		sigScope.DefineType(tp.Value, typesystem.TVar{Name: tp.Value, KindVal: kind}, "")
		sigScope.RegisterKind(tp.Value, kind)
	}

	// Prepare types for Signature
	// Implicit Row Polymorphism: Open all closed records in signature
	var retType typesystem.Type
	if n.ReturnType != nil {
		retType = BuildType(n.ReturnType, sigScope, &w.errors)
		retType = OpenRecords(retType, w.freshVar)
	} else {
		retType = w.freshVar()
	}

	var params []typesystem.Type

	// If extension, add receiver to params first
	if n.Receiver != nil && n.Receiver.Type != nil {
		t := BuildType(n.Receiver.Type, sigScope, &w.errors)
		t = OpenRecords(t, w.freshVar)
		params = append(params, t)
	}

	var isVariadic bool
	var defaultCount int
	for _, p := range n.Parameters {
		if p.IsVariadic {
			isVariadic = true
		}
		if p.Default != nil {
			defaultCount++
		}
		if p.Type != nil {
			t := BuildType(p.Type, sigScope, &w.errors)
			t = OpenRecords(t, w.freshVar)
			params = append(params, t)
		} else {
			tv := w.freshVar()
			params = append(params, tv)
		}
	}

	// Collect Implicit Generics found during BuildType
	// We iterate over the local store of sigScope to find any TVars that were created.
	// These are our implicit generics (e.g. "a" in "map(f: a->b)").
	// We need to add them to n.TypeParams so that later phases (Body Analysis) treat them as Rigid Type Params.

	// Use a map to avoid duplicates and check against explicit params
	existingParams := make(map[string]bool)
	for _, tp := range n.TypeParams {
		existingParams[tp.Value] = true
	}

	// We need deterministic order for type params, so we collect and sort
	var implicitParams []string
	for name, sym := range sigScope.All() {
		if sym.Kind == symbols.TypeSymbol {
			if _, isTVar := sym.Type.(typesystem.TVar); isTVar {
				if !existingParams[name] {
					implicitParams = append(implicitParams, name)
				}
			}
		}
	}

	if len(implicitParams) > 0 {
		// Sort implicit params for stability
		// (Optional, but good for tests)
		// sort.Strings(implicitParams) // Need to import sort

		for _, name := range implicitParams {
			// Add to AST as if they were explicit
			n.TypeParams = append(n.TypeParams, &ast.Identifier{
				Token: n.Name.Token, // Use function token as fallback
				Value: name,
			})
		}
	}

	// Build constraints for TFunc
	var fnConstraints []typesystem.Constraint

	// Reset WitnessParams to avoid duplication if visited multiple times
	n.WitnessParams = nil

	// Track already added witness params to avoid duplicates
	addedWitnesses := make(map[string]bool)

	for _, c := range n.Constraints {
		var args []typesystem.Type
		witnessName := "$dict_" + c.TypeVar + "_" + c.Trait

		if len(c.Args) > 0 {
			for _, argNode := range c.Args {
				t := BuildType(argNode, sigScope, &w.errors)
				args = append(args, t)

				// Append arg type name to witness name for MPTC disambiguation
				// e.g. $dict_A_Convert_B
				// Simple heuristic: use string representation of type
				// For TVar "B", it is "B".
				witnessName += "_" + t.String()
			}
		}
		fnConstraints = append(fnConstraints, typesystem.Constraint{TypeVar: c.TypeVar, Trait: c.Trait, Args: args})

		// Add implicit dictionary parameter for the trait itself
		if !addedWitnesses[witnessName] {
			n.WitnessParams = append(n.WitnessParams, witnessName)
			addedWitnesses[witnessName] = true
		}

		// Also add witness params for all super traits
		// This ensures that when a function has constraint `t: SubTrait` where `SubTrait : BaseTrait`,
		// we also have a witness param for `BaseTrait`
		superTraits, _ := w.symbolTable.GetTraitSuperTraits(c.Trait)
		for _, superTrait := range superTraits {
			superWitnessName := "$dict_" + c.TypeVar + "_" + superTrait
			if !addedWitnesses[superWitnessName] {
				n.WitnessParams = append(n.WitnessParams, superWitnessName)
				addedWitnesses[superWitnessName] = true
				// Also add constraint for the super trait
				fnConstraints = append(fnConstraints, typesystem.Constraint{TypeVar: c.TypeVar, Trait: superTrait, Args: nil})
			}
		}
	}

	fnType := typesystem.TFunc{
		Params:       params,
		ReturnType:   retType,
		IsVariadic:   isVariadic,
		DefaultCount: defaultCount,
		Constraints:  fnConstraints,
	}

	// Kind Check: Ensure function signature uses proper types (Kind *)
	if _, err := typesystem.KindCheck(fnType); err != nil {
		w.addError(diagnostics.NewError(
			diagnostics.ErrA003,
			n.Name.GetToken(),
			"invalid function signature: "+err.Error(),
		))
	}

	// Define in Symbol Table
	// In ModeHeaders: We are defining top-level functions.
	// In ModeBodies: We are defining nested functions (since top-level uses analyzeFunctionBody).
	// In both cases, we want to define the symbol in the current scope.

	// Check for redefinition
	// - Skip for instance methods (they implement trait methods that exist globally)
	// - For top-level (ModeHeaders): block redefinition of builtins
	// - For nested (ModeBodies): allow shadowing of builtins (only check local scope)
	if !w.inInstance {
		if w.mode == ModeHeaders || w.mode == ModeFull {
			// Top-level: check full scope chain
			if sym, ok := w.symbolTable.Find(n.Name.Value); ok {
				if sym.IsPending {
					// OK to overwrite Pending (forward declaration)
				} else {
					// Implicitly skip redefinition check for internal symbols (constructors)
					if len(n.Name.Value) > 0 && n.Name.Value[0] == '$' {
						// proceed to overwrite
					} else {
						// Allow shadowing trait methods
						if _, isTraitMethod := w.symbolTable.GetTraitForMethod(n.Name.Value); !isTraitMethod {
							w.addError(diagnostics.NewError(diagnostics.ErrA004, n.Name.GetToken(), n.Name.Value))
							return
						}
					}
				}
			}
		} else if w.mode == ModeBodies {
			// Nested: only check local scope (allow shadowing of builtins)
			if w.symbolTable.IsDefinedLocally(n.Name.Value) {
				sym, ok := w.symbolTable.Find(n.Name.Value)
				if ok && !sym.IsPending {
					w.addError(diagnostics.NewError(diagnostics.ErrA004, n.Name.GetToken(), n.Name.Value))
					return
				}
			}
		}
	}

	if n.Receiver != nil {
		typeName := resolveReceiverTypeName(n.Receiver.Type, w.symbolTable)
		if typeName == "" {
			w.addError(diagnostics.NewError(
				diagnostics.ErrA003,
				n.Receiver.Token,
				"invalid receiver type for extension method",
			))
		} else {
			w.symbolTable.RegisterExtensionMethod(typeName, n.Name.Value, fnType)
		}
	} else {
		// Functions are immutable by default
		// We use DefineConstant. For top-level, module is current. For nested, empty?
		module := w.currentModuleName
		if w.mode == ModeBodies {
			module = "" // Nested functions don't belong to module exports usually?
		}
		w.symbolTable.DefineConstant(n.Name.Value, fnType, module)
	}

	// Store Function Type in TypeMap
	w.TypeMap[n] = fnType

	// Analyze Body
	// If ModeHeaders: Skip body.
	// If ModeBodies: Analyze body (this is a nested function).
	// If ModeFull: Analyze body.

	if w.mode == ModeHeaders || w.mode == ModeInstances {
		return
	}

	// If we are already inside a function body (nested function),
	// we SKIP the walker analysis here because:
	// 1. The walker scope does not contain local variables (assignments are not processed by walker).
	// 2. The inference pass (InferWithContext) will encounter this FunctionStatement
	//    and recursively analyze it with the correct populated scope.
	if w.inFunctionBody {
		return
	}

	// For nested functions, we use the shared analyzeFunctionBody logic?
	// analyzeFunctionBody expects FunctionStatement.
	// But analyzeFunctionBody creates a NEW scope.
	// Yes, nested functions need a new scope.
	w.analyzeFunctionBody(n)
}

// applySubstToNode recursively applies a type substitution to all nodes in the AST.
// This ensures that type variables resolved during inference are propagated to all
// sub-expressions in the TypeMap.
func (w *walker) applySubstToNode(node ast.Node, subst typesystem.Subst) {
	if node == nil || len(subst) == 0 {
		return
	}

	// Update type in TypeMap if present
	if t, ok := w.TypeMap[node]; ok {
		w.TypeMap[node] = t.Apply(subst)
	}

	// Recursively traverse children based on node type
	switch n := node.(type) {
	// ==================== Statements ====================
	case *ast.Program:
		for _, stmt := range n.Statements {
			w.applySubstToNode(stmt, subst)
		}

	case *ast.BlockStatement:
		for _, stmt := range n.Statements {
			w.applySubstToNode(stmt, subst)
		}

	case *ast.ExpressionStatement:
		w.applySubstToNode(n.Expression, subst)

	case *ast.FunctionStatement:
		w.applySubstToNode(n.Body, subst)

	case *ast.ConstantDeclaration:
		w.applySubstToNode(n.Value, subst)

	case *ast.BreakStatement:
		if n.Value != nil {
			w.applySubstToNode(n.Value, subst)
		}

	case *ast.ContinueStatement:
		// No expression children

	case *ast.InstanceDeclaration:
		for _, method := range n.Methods {
			w.applySubstToNode(method.Body, subst)
		}

	// ==================== Expressions ====================
	// --- Operators ---
	case *ast.AssignExpression:
		w.applySubstToNode(n.Left, subst)
		w.applySubstToNode(n.Value, subst)

	case *ast.InfixExpression:
		w.applySubstToNode(n.Left, subst)
		w.applySubstToNode(n.Right, subst)

	case *ast.PrefixExpression:
		w.applySubstToNode(n.Right, subst)

	case *ast.PostfixExpression:
		w.applySubstToNode(n.Left, subst)

	// --- Function calls ---
	case *ast.CallExpression:
		w.applySubstToNode(n.Function, subst)
		for _, arg := range n.Arguments {
			w.applySubstToNode(arg, subst)
		}

	case *ast.TypeApplicationExpression:
		w.applySubstToNode(n.Expression, subst)

	case *ast.FunctionLiteral:
		w.applySubstToNode(n.Body, subst)

	// --- Control flow ---
	case *ast.IfExpression:
		w.applySubstToNode(n.Condition, subst)
		w.applySubstToNode(n.Consequence, subst)
		if n.Alternative != nil {
			w.applySubstToNode(n.Alternative, subst)
		}

	case *ast.ForExpression:
		if n.Condition != nil {
			w.applySubstToNode(n.Condition, subst)
		}
		if n.Iterable != nil {
			w.applySubstToNode(n.Iterable, subst)
		}
		if n.Body != nil {
			w.applySubstToNode(n.Body, subst)
		}

	case *ast.MatchExpression:
		w.applySubstToNode(n.Expression, subst)
		for _, arm := range n.Arms {
			w.applySubstToNode(arm.Expression, subst)
			// Also traverse patterns for any nested expressions
			w.applySubstToPattern(arm.Pattern, subst)
		}

	// --- Collection literals ---
	case *ast.TupleLiteral:
		for _, elem := range n.Elements {
			w.applySubstToNode(elem, subst)
		}

	case *ast.ListLiteral:
		for _, elem := range n.Elements {
			w.applySubstToNode(elem, subst)
		}

	case *ast.RecordLiteral:
		if n.Spread != nil {
			w.applySubstToNode(n.Spread, subst)
		}
		for _, val := range n.Fields {
			w.applySubstToNode(val, subst)
		}

	// --- Access expressions ---
	case *ast.IndexExpression:
		w.applySubstToNode(n.Left, subst)
		w.applySubstToNode(n.Index, subst)

	case *ast.MemberExpression:
		w.applySubstToNode(n.Left, subst)
		w.applySubstToNode(n.Member, subst)

	// --- Other expressions ---
	case *ast.SpreadExpression:
		w.applySubstToNode(n.Expression, subst)

	case *ast.AnnotatedExpression:
		w.applySubstToNode(n.Expression, subst)

	// --- Literals (no children, but may have TypeMap entries) ---
	case *ast.Identifier,
		*ast.IntegerLiteral,
		*ast.FloatLiteral,
		*ast.BooleanLiteral,
		*ast.NilLiteral,
		*ast.BigIntLiteral,
		*ast.RationalLiteral,
		*ast.StringLiteral,
		*ast.InterpolatedString,
		*ast.CharLiteral:
		// No children to traverse, TypeMap already updated above
	}
}

// applySubstToPattern applies substitution to patterns that may contain sub-patterns.
func (w *walker) applySubstToPattern(pattern ast.Pattern, subst typesystem.Subst) {
	if pattern == nil {
		return
	}

	// Update type in TypeMap if present
	if t, ok := w.TypeMap[pattern]; ok {
		w.TypeMap[pattern] = t.Apply(subst)
	}

	switch p := pattern.(type) {
	case *ast.ConstructorPattern:
		for _, elem := range p.Elements {
			w.applySubstToPattern(elem, subst)
		}

	case *ast.TuplePattern:
		for _, elem := range p.Elements {
			w.applySubstToPattern(elem, subst)
		}

	case *ast.ListPattern:
		for _, elem := range p.Elements {
			w.applySubstToPattern(elem, subst)
		}

	case *ast.SpreadPattern:
		if p.Pattern != nil {
			w.applySubstToPattern(p.Pattern, subst)
		}

	case *ast.RecordPattern:
		for _, pat := range p.Fields {
			w.applySubstToPattern(pat, subst)
		}

	case *ast.LiteralPattern, *ast.WildcardPattern, *ast.IdentifierPattern:
		// No children to traverse
	}
}

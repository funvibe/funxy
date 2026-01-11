package analyzer

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/token"
	"github.com/funvibe/funxy/internal/typesystem"
	"sort"
)

// Analyzer performs semantic analysis on the AST.
type Analyzer struct {
	symbolTable   *symbols.SymbolTable
	inLoop        bool // Track if we are inside a loop
	loader        ModuleLoader
	BaseDir       string
	TypeMap       map[ast.Node]typesystem.Type      // Stores inferred types
	inferCtx      *InferenceContext                 // Shared inference context for consistent TVar naming
	TraitDefaults map[string]*ast.FunctionStatement // "TraitName.methodName" -> FunctionStatement
}

// ModuleLoader interface to break dependency cycle
type ModuleLoader interface {
	GetModule(path string) (interface{}, error)     // Returns *modules.Module (which implements LoadedModule)
	GetModuleByPackageName(name string) interface{} // Returns module by package name (for extension methods/traits lookup)
}

// LoadedModule interface representing a fully loaded AND analyzed module
type LoadedModule interface {
	GetName() string                                // Returns package name
	GetExportedSymbols() map[string]typesystem.Type // Deprecated: Use GetExports()
	GetExports() map[string]symbols.Symbol

	IsHeadersAnalyzed() bool
	SetHeadersAnalyzed(bool)
	IsHeadersAnalyzing() bool
	SetHeadersAnalyzing(bool)

	IsBodiesAnalyzed() bool
	SetBodiesAnalyzed(bool)
	IsBodiesAnalyzing() bool
	SetBodiesAnalyzing(bool)

	GetFiles() []*ast.Program
	GetSymbolTable() *symbols.SymbolTable
	SetTypeMap(map[ast.Node]typesystem.Type)

	// Package group support
	IsPackageGroupModule() bool
	GetSubModulesRaw() map[string]interface{} // Returns sub-modules (cast to LoadedModule)

	// Re-export support
	GetReexportSpecs() []*ast.ExportSpec
	AddExport(name string)

	// Trait defaults
	SetTraitDefaults(map[string]*ast.FunctionStatement)
	GetTraitDefaults() map[string]*ast.FunctionStatement
}

// New creates a new Analyzer with a given symbol table.
func New(symbolTable *symbols.SymbolTable) *Analyzer {
	return &Analyzer{
		symbolTable:   symbolTable,
		inLoop:        false,
		BaseDir:       ".", // Default to CWD
		TraitDefaults: make(map[string]*ast.FunctionStatement),
	}
}

func (a *Analyzer) SetLoader(l ModuleLoader) {
	a.loader = l
}

// SetInferenceContext sets the shared inference context.
// This is useful for sharing context between parent and imported modules
// to ensure unique type variable naming across the entire analysis.
func (a *Analyzer) SetInferenceContext(ctx *InferenceContext) {
	a.inferCtx = ctx
}

func (a *Analyzer) RegisterBuiltins() {
	RegisterBuiltins(a.symbolTable)
}

type walker struct {
	symbolTable       *symbols.SymbolTable
	errorSet          map[string]*diagnostics.DiagnosticError // Key: "line:col:code" for deduplication
	errors            []*diagnostics.DiagnosticError          // Temporary slice for compatibility with BuildType etc.
	inLoop            bool                                    // Track if we are inside a loop
	inInstance        bool                                    // Track if we are inside an instance declaration
	loader            ModuleLoader
	BaseDir           string
	TypeMap           map[ast.Node]typesystem.Type
	inferCtx          *InferenceContext // Context for type inference
	mode              AnalysisMode
	TraitDefaults     map[string]*ast.FunctionStatement // "TraitName.methodName" -> FunctionStatement
	currentModuleName string                            // Name of the module being analyzed (for OriginModule tracking)
	currentFile       string                            // Current file being analyzed (for error reporting)
	inFunctionBody    bool                              // Track if we are inside a function body (to skip redundant expression inference)
	Program           *ast.Program                      // Reference to the program being analyzed (for AST injection)
	importedModules   map[string]bool                   // Track imported modules by absolute path to detect duplicates
	injectedStmts     []ast.Statement                   // Statements queued for injection (e.g. dictionaries)
	aborted           bool                              // Flag to abort analysis immediately (e.g. on duplicate import)
}

// addError adds an error to the walker, deduplicating by position and message
func (w *walker) addError(err *diagnostics.DiagnosticError) {
	if err.File == "" && w.currentFile != "" {
		err.File = w.currentFile
	}
	key := fmt.Sprintf("%d:%d:%s", err.Token.Line, err.Token.Column, err.Code)
	if w.errorSet == nil {
		w.errorSet = make(map[string]*diagnostics.DiagnosticError)
	}
	w.errorSet[key] = err
}

// addErrors adds multiple errors to the walker
func (w *walker) addErrors(errs []*diagnostics.DiagnosticError) {
	for _, err := range errs {
		w.addError(err)
	}
}

// getErrors returns all unique errors as a slice, sorted by position
func (w *walker) getErrors() []*diagnostics.DiagnosticError {
	// First, merge any errors from the compatibility slice into errorSet
	for _, err := range w.errors {
		w.addError(err)
	}

	result := make([]*diagnostics.DiagnosticError, 0, len(w.errorSet))
	for _, err := range w.errorSet {
		result = append(result, err)
	}

	// Sort by line, then column for deterministic output
	sort.Slice(result, func(i, j int) bool {
		if result[i].Token.Line != result[j].Token.Line {
			return result[i].Token.Line < result[j].Token.Line
		}
		return result[i].Token.Column < result[j].Token.Column
	})

	return result
}

type AnalysisMode int

const (
	ModeFull      AnalysisMode = iota // Legacy/Single file
	ModeNaming                        // Pass 1: Name Discovery (Pending Symbols)
	ModeHeaders                       // Pass 2: Imports and Signature Resolution
	ModeInstances                     // Pass 3: Trait Instances
	ModeBodies                        // Pass 4: Bodies and Expressions
)

// AnalyzeNaming runs the naming pass (discovery)
func (a *Analyzer) AnalyzeNaming(node ast.Node) []*diagnostics.DiagnosticError {
	// Simple walker for naming only
	w := &walker{
		symbolTable: a.symbolTable,
		errorSet:    make(map[string]*diagnostics.DiagnosticError),
		errors:      []*diagnostics.DiagnosticError{},
		loader:      a.loader,
		BaseDir:     a.BaseDir,
		mode:        ModeNaming,
	}
	node.Accept(w)
	return w.getErrors()
}

func (a *Analyzer) AnalyzeHeaders(node ast.Node) []*diagnostics.DiagnosticError {
	typeMap := make(map[ast.Node]typesystem.Type)

	// Create shared InferenceContext if not exists
	if a.inferCtx == nil {
		a.inferCtx = NewInferenceContextWithLoader(a.loader)
	}
	a.inferCtx.TypeMap = typeMap

	w := &walker{
		symbolTable:     a.symbolTable,
		errorSet:        make(map[string]*diagnostics.DiagnosticError),
		errors:          []*diagnostics.DiagnosticError{},
		inLoop:          false,
		loader:          a.loader,
		BaseDir:         a.BaseDir,
		TypeMap:         typeMap,
		inferCtx:        a.inferCtx, // Use shared context
		mode:            ModeHeaders,
		TraitDefaults:   a.TraitDefaults,
		importedModules: make(map[string]bool),
	}
	if p, ok := node.(*ast.Program); ok {
		w.Program = p
	}
	node.Accept(w)

	// Merge TypeMap
	if a.TypeMap == nil {
		a.TypeMap = make(map[ast.Node]typesystem.Type)
	}
	for k, v := range w.TypeMap {
		a.TypeMap[k] = v
	}

	return w.getErrors()
}

func (a *Analyzer) AnalyzeInstances(node ast.Node) []*diagnostics.DiagnosticError {
	// Reuse existing TypeMap and InferenceContext from Headers pass
	if a.TypeMap == nil {
		a.TypeMap = make(map[ast.Node]typesystem.Type)
	}
	if a.inferCtx == nil {
		a.inferCtx = NewInferenceContextWithLoader(a.loader)
	}
	a.inferCtx.TypeMap = a.TypeMap

	w := &walker{
		symbolTable:   a.symbolTable,
		errorSet:      make(map[string]*diagnostics.DiagnosticError),
		errors:        []*diagnostics.DiagnosticError{},
		inLoop:        false,
		loader:        a.loader,
		BaseDir:       a.BaseDir,
		TypeMap:       a.TypeMap,
		inferCtx:      a.inferCtx,
		mode:          ModeInstances,
		TraitDefaults: a.TraitDefaults,
	}
	if p, ok := node.(*ast.Program); ok {
		w.Program = p
	}
	node.Accept(w)

	// Inject statements generated during instance analysis (dictionaries)
	// Prepend to ensure they are available before use
	if p, ok := node.(*ast.Program); ok && len(w.injectedStmts) > 0 {
		p.Statements = append(w.injectedStmts, p.Statements...)

		// Run Naming and Headers passes on injected statements to register them
		// This ensures they are visible to the subsequent Bodies pass and Evaluator
		for _, stmt := range w.injectedStmts {
			// Pass 1: Naming (Register symbol)
			errs := a.AnalyzeNaming(stmt)
			if len(errs) > 0 {
				w.addErrors(errs)
			}

			// Pass 2: Headers (Resolve signature/type)
			errs = a.AnalyzeHeaders(stmt)
			if len(errs) > 0 {
				w.addErrors(errs)
			}
		}
	}

	return w.getErrors()
}

func (a *Analyzer) AnalyzeBodies(node ast.Node) []*diagnostics.DiagnosticError {
	// Reuse existing TypeMap if possible
	if a.TypeMap == nil {
		a.TypeMap = make(map[ast.Node]typesystem.Type)
	}

	// Reuse shared InferenceContext (counter continues from Headers pass)
	if a.inferCtx == nil {
		a.inferCtx = NewInferenceContextWithLoader(a.loader)
	}
	a.inferCtx.TypeMap = a.TypeMap

	// Pass 2: Main analysis with expected types available
	// Ensure expected return types are propagated to TypeMap if possible,
	// so that Evaluator can see them.

	w := &walker{
		symbolTable:     a.symbolTable,
		errorSet:        make(map[string]*diagnostics.DiagnosticError),
		errors:          []*diagnostics.DiagnosticError{},
		inLoop:          false,
		loader:          a.loader,
		BaseDir:         a.BaseDir,
		TypeMap:         a.TypeMap,
		inferCtx:        a.inferCtx, // Use shared context
		mode:            ModeBodies,
		TraitDefaults:   a.TraitDefaults,
		importedModules: make(map[string]bool),
	}
	node.Accept(w)

	// Propagate expected return types to TypeMap for Evaluator
	for node, expectedType := range a.inferCtx.ExpectedReturnTypes {
		if _, exists := w.TypeMap[node]; !exists {
			w.TypeMap[node] = expectedType
		}
	}

	// Apply global substitution to all types in TypeMap to ensure all type variables are resolved
	if len(a.inferCtx.GlobalSubst) > 0 {
		for node, typ := range w.TypeMap {
			w.TypeMap[node] = typ.Apply(a.inferCtx.GlobalSubst)
		}
		// Finalize Instantiations in CallExpressions
		a.finalizeInstantiations(node, a.inferCtx.GlobalSubst)
	}

	// Resolve Pending Witnesses (global pass)
	ResolvePendingWitnesses(a.inferCtx, nil, a.symbolTable, func(n ast.Node, err error) {
		w.addError(diagnostics.NewError(diagnostics.ErrA003, getNodeToken(n), "GLOBAL RESOLVE: "+err.Error()))
	})

	// Solve Deferred Constraints
	constraintErrors := a.inferCtx.SolveConstraints(a.symbolTable)
	for _, err := range constraintErrors {
		// If the error is already a DiagnosticError, it has location info.
		// appendError will use it. If not, we might lose location unless SolveConstraints puts it in.
		// SolveConstraints creates DiagnosticErrors using inferErrorf which puts location.
		w.appendError(nil, err)
	}

	return w.getErrors()
}

// ResolvePendingWitnesses resolves deferred type class constraints
func ResolvePendingWitnesses(ctx *InferenceContext, subst typesystem.Subst, table *symbols.SymbolTable, errorHandler func(ast.Node, error)) {
	// Filter pending witnesses relevant to this scope (or all, since it's single-pass per function body mostly)
	var remaining []PendingWitness
	for _, pw := range ctx.PendingWitnesses {
		// Resolve args
		var resolvedArgs []typesystem.Type
		if len(pw.Args) > 0 {
			for _, arg := range pw.Args {
				resolved := arg
				// Apply local subst
				if subst != nil {
					resolved = resolved.Apply(subst)
				}
				// Apply global subst
				resolved = resolved.Apply(ctx.GlobalSubst)
				resolvedArgs = append(resolvedArgs, resolved)
			}
		} else {
			// Fallback: Resolve TypeVar
			tvar := typesystem.TVar{Name: pw.TypeVar}
			var concrete typesystem.Type
			if subst != nil {
				concrete = tvar.Apply(subst)
			} else {
				concrete = tvar.Apply(ctx.GlobalSubst)
			}
			// Apply global again?
			concrete = concrete.Apply(ctx.GlobalSubst)
			resolvedArgs = []typesystem.Type{concrete}
		}

		concreteType := resolvedArgs[0] // Main type for generic checks

		// Check if any arg is still a variable
		hasVar := false
		for _, arg := range resolvedArgs {
			if _, ok := arg.(typesystem.TVar); ok {
				hasVar = true
				break
			}
		}

		isGenericWitness := false
		debugLog := ""

		if hasVar {
			// If it's a TVar, check if it has the required constraint in the current context.
			// This handles cases where we are in a generic function body, and the type variable
			// is a generic parameter that satisfies the constraint. In this case, we pass the
			// type variable itself as the witness (Generic Witness).

			// Also check Deferred Constraints (ctx.Constraints) for pending generalizations
			hasDeferred := false
			for _, dc := range ctx.Constraints {
				if dc.Kind == ConstraintImplements {
					// Check if dc.Left matches concreteType (after subst)
					left := dc.Left.Apply(ctx.GlobalSubst)
					if subst != nil {
						left = left.Apply(subst)
					}

					debugLog += fmt.Sprintf("[%s vs %s] ", left, concreteType)

					// Check if variable matches
					if tv, ok := left.(typesystem.TVar); ok {
						if tvName, ok := concreteType.(typesystem.TVar); ok && tv.Name == tvName.Name {
							// Check if dc.Trait implies pw.Trait (e.g. Monad implies Applicative)
							if dc.Trait == pw.Trait || isTraitSubclass(dc.Trait, pw.Trait, table) {
								hasDeferred = true
								break
							}
						}
					}
				}
			}

			// Check if we have a constraint matching all args (MPTC support)
			hasConstraint := false
			if len(resolvedArgs) > 0 {
				hasConstraint = ctx.HasMPTCConstraint(pw.Trait, resolvedArgs)
			} else {
				// Fallback
				hasConstraint = typeHasConstraint(ctx, concreteType, pw.Trait, table)
			}

			if hasConstraint || hasDeferred {
				isGenericWitness = true
			} else {
				// Try looking up the type in the AST node's TypeMap?
				// The node (CallExpression) should have an inferred type.
				if ctx.TypeMap != nil {
					if nodeType, ok := ctx.TypeMap[pw.Node]; ok {
						// Apply substitutions to nodeType
						resolvedNodeType := nodeType
						if subst != nil {
							resolvedNodeType = resolvedNodeType.Apply(subst)
						}
						resolvedNodeType = resolvedNodeType.Apply(ctx.GlobalSubst)

						// Check if resolvedNodeType reveals the type constructor
						if tApp, ok := resolvedNodeType.(typesystem.TApp); ok {
							concreteType = tApp.Constructor
							resolvedArgs[0] = concreteType
						}
					}
				}

				isRigid := false
				if tCon, ok := concreteType.(typesystem.TCon); ok && len(tCon.Name) > 0 && tCon.Name[0] >= 'a' && tCon.Name[0] <= 'z' {
					isRigid = true
				}

				if _, stillVar := concreteType.(typesystem.TVar); stillVar || isRigid {
					// Still unresolved and not a valid generic witness.
					// Keep it pending.
					remaining = append(remaining, pw)
					continue
				}
			}
		}

		if !isGenericWitness {
			// Check implementation
			unwrappedArgs := make([]typesystem.Type, len(resolvedArgs))
			for i, arg := range resolvedArgs {
				u := typesystem.UnwrapUnderlying(arg)
				if u == nil {
					u = arg
				}
				unwrappedArgs[i] = u
			}

			// We use SolveWitness to get the dictionary expression.
			// SolveWitness internally checks EvidenceTable (for global/concrete instances)
			// and handles constructors.
			// It basically replaces the old check table.IsImplementationExists(pw.Trait, unwrappedArgs)
			// because if evidence exists, implementation exists.

			// However, SolveWitness might fail if evidence is not found.
			// IsImplementationExists checked the *list* of implementations.
			// SolveWitness checks the *EvidenceTable* (map key -> name).
			// They should be in sync if declarations_instances.go registers both.

			witnessExpr, err := ctx.SolveWitness(pw.Node, pw.Trait, resolvedArgs, table)
			if err != nil {
				// Fallback to IsImplementationExists check for error reporting?
				// If SolveWitness failed, it means we don't have a way to construct it.
				// But IsImplementationExists might say "yes" (e.g. builtins not fully migrated).
				// If so, we shouldn't fail hard if we are in transition?
				// But Architect says we need to fix the gap.

				// If SolveWitness fails, it is a legitimate error.
				debugConstraints := ""
				for _, c := range ctx.Constraints {
					l := c.Left.Apply(ctx.GlobalSubst)
					if subst != nil {
						l = l.Apply(subst)
					}
					debugConstraints += fmt.Sprintf("[%s %s: %s] ", c.Kind, l.String(), c.Trait)
				}

				if len(unwrappedArgs) == 1 {
					errorHandler(pw.Node, fmt.Errorf("evidence for class %s for type %s not found (SolveWitness failed): %v [Constraints: %s]", pw.Trait, unwrappedArgs[0], err, debugConstraints))
				} else {
					errorHandler(pw.Node, fmt.Errorf("evidence for class %s for types %v not found (SolveWitness failed): %v [Constraints: %s]", pw.Trait, unwrappedArgs, err, debugConstraints))
				}
				continue
			}

			// Store witness expression in AST (Witnesses list)
			if pw.Node.Witnesses == nil {
				pw.Node.Witnesses = make([]ast.Expression, 0)
			}
			// We need to place it at the correct index.
			// The witnesses list might be sparse or unordered during accumulation?
			// pw.Index tells us where it should be.
			// We need to ensure the slice is big enough.
			if len(pw.Node.Witnesses) <= pw.Index {
				// Grow slice
				newSlice := make([]ast.Expression, pw.Index+1)
				copy(newSlice, pw.Node.Witnesses)
				pw.Node.Witnesses = newSlice
			}
			pw.Node.Witnesses[pw.Index] = witnessExpr
		}

		// Store witness in AST (Legacy map support - keep for now)
		if pw.Node.Witness == nil {
			pw.Node.Witness = make(map[string][]typesystem.Type)
		}
		if witnesses, ok := pw.Node.Witness.(map[string][]typesystem.Type); ok {
			witnesses[pw.Trait] = resolvedArgs
		}
	}
	ctx.PendingWitnesses = remaining
}

// Analyze performs semantic analysis on the given node.
func (a *Analyzer) Analyze(node ast.Node) []*diagnostics.DiagnosticError {
	// If node is Program, use multi-pass analysis
	if prog, ok := node.(*ast.Program); ok {
		// Pass 1: Naming
		errs := a.AnalyzeNaming(prog)
		if len(errs) > 0 {
			return errs
		}
		// Pass 2: Headers
		errs = a.AnalyzeHeaders(prog)
		if len(errs) > 0 {
			return errs
		}
		// Pass 3: Instances
		errs = a.AnalyzeInstances(prog)
		if len(errs) > 0 {
			return errs
		}
		// Pass 4: Bodies
		return a.AnalyzeBodies(prog)
	}

	// Fallback for partial nodes (Expressions, etc.) - ModeFull
	typeMap := make(map[ast.Node]typesystem.Type)
	w := &walker{
		symbolTable:     a.symbolTable,
		errorSet:        make(map[string]*diagnostics.DiagnosticError),
		errors:          []*diagnostics.DiagnosticError{},
		inLoop:          a.inLoop, // Inherit loop state from Analyzer
		loader:          a.loader,
		BaseDir:         a.BaseDir,
		TypeMap:         typeMap,
		inferCtx:        NewInferenceContextWithTypeMap(typeMap),
		mode:            ModeFull,
		TraitDefaults:   a.TraitDefaults,
		importedModules: make(map[string]bool),
	}
	node.Accept(w)

	a.TypeMap = w.TypeMap

	// Validate exports after processing the whole program
	if prog, ok := node.(*ast.Program); ok {
		for _, stmt := range prog.Statements {
			if pkg, ok := stmt.(*ast.PackageDeclaration); ok {
				if !pkg.ExportAll {
					for _, exp := range pkg.Exports {
						// Only validate local exports, not re-exports
						// Re-exports are validated in resolveReexports() during module analysis
						if !exp.IsReexport() && exp.Symbol != nil {
							if !w.symbolTable.IsDefined(exp.Symbol.Value) {
								w.addError(diagnostics.NewError(
									diagnostics.ErrA001,
									exp.GetToken(),
									"exported symbol not defined: "+exp.Symbol.Value,
								))
							}
						}
					}
				}
			}
		}
	}

	return w.getErrors()
}

// freshVar generates a fresh type variable using the walker's inference context.
func (w *walker) freshVar() typesystem.TVar {
	return w.inferCtx.FreshVar()
}

// MarkTailCalls recursively marks tail calls in the AST.
func MarkTailCalls(node ast.Node) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *ast.BlockStatement:
		if len(n.Statements) > 0 {
			lastStmt := n.Statements[len(n.Statements)-1]
			MarkTailCalls(lastStmt)
		}
	case *ast.ExpressionStatement:
		MarkTailCalls(n.Expression)
	case *ast.CallExpression:
		n.IsTail = true
	case *ast.IfExpression:
		MarkTailCalls(n.Consequence)
		if n.Alternative != nil {
			MarkTailCalls(n.Alternative)
		}
	case *ast.MatchExpression:
		for _, arm := range n.Arms {
			MarkTailCalls(arm.Expression)
		}
	}
}

func (w *walker) markTailCalls(node ast.Node) {
	MarkTailCalls(node)
}

// getNodeToken extracts token from AST node if possible
func getNodeToken(node ast.Node) token.Token {
	if node == nil {
		return token.Token{}
	}
	if getter, ok := node.(interface{ GetToken() token.Token }); ok {
		return getter.GetToken()
	}
	return token.Token{}
}

// appendError adds an error to the walker's error list.
// If the error is already a DiagnosticError, it's added directly.
// Otherwise, it's wrapped with the given node's location.
// Combined errors are unpacked into individual errors.
func (w *walker) appendError(node ast.Node, err error) {
	// Handle combined errors by unpacking them
	if ce, ok := err.(*combinedError); ok {
		for _, e := range ce.errors {
			w.appendError(node, e)
		}
		return
	}
	if ce, ok := err.(*diagnostics.DiagnosticError); ok {
		w.addError(ce)
	} else {
		w.addError(diagnostics.NewError(diagnostics.ErrA003, getNodeToken(node), err.Error()))
	}
}

// finalizeInstantiations traverses the AST and applies the global substitution
// to any Instantiation maps in CallExpressions.
func (a *Analyzer) finalizeInstantiations(node ast.Node, subst typesystem.Subst) {
	if node == nil {
		return
	}

	// Process CallExpression
	if call, ok := node.(*ast.CallExpression); ok {
		if call.Instantiation != nil {
			for k, v := range call.Instantiation {
				call.Instantiation[k] = v.Apply(subst)
			}
		}
		if call.TypeArgs != nil {
			for i, arg := range call.TypeArgs {
				call.TypeArgs[i] = arg.Apply(subst)
			}
		}
	}

	// Recurse into children
	switch n := node.(type) {
	case *ast.Program:
		for _, stmt := range n.Statements {
			a.finalizeInstantiations(stmt, subst)
		}
	case *ast.BlockStatement:
		for _, stmt := range n.Statements {
			a.finalizeInstantiations(stmt, subst)
		}
	case *ast.ExpressionStatement:
		a.finalizeInstantiations(n.Expression, subst)
	case *ast.ConstantDeclaration:
		a.finalizeInstantiations(n.Value, subst)
	case *ast.FunctionStatement:
		if n.Body != nil {
			a.finalizeInstantiations(n.Body, subst)
		}
	case *ast.FunctionLiteral:
		if n.Body != nil {
			a.finalizeInstantiations(n.Body, subst)
		}
	case *ast.CallExpression:
		a.finalizeInstantiations(n.Function, subst)
		for _, arg := range n.Arguments {
			a.finalizeInstantiations(arg, subst)
		}
	case *ast.InfixExpression:
		a.finalizeInstantiations(n.Left, subst)
		a.finalizeInstantiations(n.Right, subst)
	case *ast.PrefixExpression:
		a.finalizeInstantiations(n.Right, subst)
	case *ast.PostfixExpression:
		a.finalizeInstantiations(n.Left, subst)
	case *ast.IfExpression:
		a.finalizeInstantiations(n.Condition, subst)
		if n.Consequence != nil {
			a.finalizeInstantiations(n.Consequence, subst)
		}
		if n.Alternative != nil {
			a.finalizeInstantiations(n.Alternative, subst)
		}
	case *ast.MatchExpression:
		a.finalizeInstantiations(n.Expression, subst)
		for _, arm := range n.Arms {
			if arm.Guard != nil {
				a.finalizeInstantiations(arm.Guard, subst)
			}
			a.finalizeInstantiations(arm.Expression, subst)
		}
	case *ast.AssignExpression:
		a.finalizeInstantiations(n.Left, subst)
		a.finalizeInstantiations(n.Value, subst)
	case *ast.AnnotatedExpression:
		a.finalizeInstantiations(n.Expression, subst)
	case *ast.TupleLiteral:
		for _, elem := range n.Elements {
			a.finalizeInstantiations(elem, subst)
		}
	case *ast.ListLiteral:
		for _, elem := range n.Elements {
			a.finalizeInstantiations(elem, subst)
		}
	case *ast.MapLiteral:
		for _, pair := range n.Pairs {
			a.finalizeInstantiations(pair.Key, subst)
			a.finalizeInstantiations(pair.Value, subst)
		}
	case *ast.RecordLiteral:
		for _, val := range n.Fields {
			a.finalizeInstantiations(val, subst)
		}
		a.finalizeInstantiations(n.Spread, subst)
	case *ast.ForExpression:
		a.finalizeInstantiations(n.Initializer, subst)
		a.finalizeInstantiations(n.Condition, subst)
		a.finalizeInstantiations(n.Iterable, subst)
		a.finalizeInstantiations(n.Body, subst)
	case *ast.SpreadExpression:
		a.finalizeInstantiations(n.Expression, subst)
	case *ast.MemberExpression:
		a.finalizeInstantiations(n.Left, subst)
	case *ast.IndexExpression:
		a.finalizeInstantiations(n.Left, subst)
		a.finalizeInstantiations(n.Index, subst)
	case *ast.TypeApplicationExpression:
		a.finalizeInstantiations(n.Expression, subst)
	}
}

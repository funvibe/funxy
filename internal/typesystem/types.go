package typesystem

import (
	"fmt"
	"github.com/funvibe/funxy/internal/config"
	"sort"
	"strconv"
	"strings"
)

// Type is the interface for all types in our system.
type Type interface {
	String() string
	Apply(Subst) Type
	FreeTypeVariables() []TVar
	Kind() Kind
}

// TVar represents a type variable (e.g. 'a', 'b', 't1').
type TVar struct {
	Name    string
	KindVal Kind // Renamed from Kind to KindVal to avoid collision with method
}

func (t TVar) String() string {
	// Normalize auto-generated type variables (t1, t2, t14, etc.) to t?
	// This normalization happens in tests (for determinism) and LSP (for clean UI)
	if config.IsTestMode || config.IsLSPMode {
		// Handle "t" prefix (standard inference vars)
		if strings.HasPrefix(t.Name, "t") {
			rest := t.Name[1:]
			if _, err := strconv.Atoi(rest); err == nil {
				return "t?"
			}
		}
		// Handle "gen_t" prefix (generalized type vars)
		if strings.HasPrefix(t.Name, "gen_t") {
			rest := t.Name[5:] // len("gen_t") == 5
			if _, err := strconv.Atoi(rest); err == nil {
				return "t" + rest
			}
		}
		// Handle "_pending_" prefix
		if strings.HasPrefix(t.Name, "_pending_") {
			return "pending?"
		}
	}
	return t.Name
}

func (t TVar) Kind() Kind {
	if t.KindVal == nil {
		return Star
	}
	return t.KindVal
}

func (t TVar) Apply(s Subst) Type {
	return ApplyWithCycleCheck(t, s, make(map[string]bool))
}

// ApplyWithCycleCheck applies substitution with cycle detection.
// This is the main entry point for substitution application.
func ApplyWithCycleCheck(t Type, s Subst, visited map[string]bool) Type {
	if t == nil {
		return nil
	}

	switch typ := t.(type) {
	case TVar:
		// Check for cycle
		if visited[typ.Name] {
			return typ // Break cycle - return the variable as-is
		}

		if replacement, ok := s[typ.Name]; ok {
			// Check for direct self-reference
			if tv, ok := replacement.(TVar); ok && tv.Name == typ.Name {
				return typ
			}
			// Mark as visited and recursively apply
			newVisited := copyVisited(visited)
			newVisited[typ.Name] = true
			return ApplyWithCycleCheck(replacement, s, newVisited)
		}
		return typ

	case TApp:
		newArgs := make([]Type, len(typ.Args))
		for i, arg := range typ.Args {
			newArgs[i] = ApplyWithCycleCheck(arg, s, visited)
		}
		newCtor := ApplyWithCycleCheck(typ.Constructor, s, visited)

		// Flatten nested TApp: if constructor is TApp, merge args
		// e.g., (Result<String>)<B> becomes Result<String, B>
		if ctorApp, ok := newCtor.(TApp); ok {
			// Merge: ctorApp.Args ++ newArgs under ctorApp.Constructor
			mergedArgs := make([]Type, 0, len(ctorApp.Args)+len(newArgs))
			mergedArgs = append(mergedArgs, ctorApp.Args...)
			mergedArgs = append(mergedArgs, newArgs...)
			return TApp{
				Constructor: ctorApp.Constructor,
				Args:        mergedArgs,
			}
		}

		return TApp{
			Constructor: newCtor,
			Args:        newArgs,
		}

	case TCon:
		// Allow substitution of TCon if it matches a key in the substitution map
		// This is required for Monomorphization where generic type parameters
		// are represented as Rigid TCons in the function body.
		if replacement, ok := s[typ.Name]; ok {
			// Check for direct self-reference to avoid infinite loop
			if tCon, ok := replacement.(TCon); ok && tCon.Name == typ.Name {
				return typ
			}
			// Check cycle with visited map
			if visited[typ.Name] {
				return typ
			}
			// Add to visited
			newVisited := copyVisited(visited)
			newVisited[typ.Name] = true
			return ApplyWithCycleCheck(replacement, s, newVisited)
		}
		return typ // Constants don't change

	case TFunc:
		newParams := make([]Type, len(typ.Params))
		for i, p := range typ.Params {
			newParams[i] = ApplyWithCycleCheck(p, s, visited)
		}
		// Apply substitution to constraints - update type variable names
		newConstraints := make([]Constraint, len(typ.Constraints))
		for i, c := range typ.Constraints {
			newTypeVar := c.TypeVar
			// If the type variable was substituted to a fresh var, update the constraint
			if subst, ok := s[c.TypeVar]; ok {
				if tv, ok := subst.(TVar); ok {
					newTypeVar = tv.Name
				}
			}
			newArgs := make([]Type, len(c.Args))
			for j, arg := range c.Args {
				newArgs[j] = ApplyWithCycleCheck(arg, s, visited)
			}
			newConstraints[i] = Constraint{TypeVar: newTypeVar, Trait: c.Trait, Args: newArgs}
		}
		return TFunc{
			Params:       newParams,
			ReturnType:   ApplyWithCycleCheck(typ.ReturnType, s, visited),
			IsVariadic:   typ.IsVariadic,
			DefaultCount: typ.DefaultCount,
			Constraints:  newConstraints,
		}

	case TTuple:
		newElems := make([]Type, len(typ.Elements))
		for i, e := range typ.Elements {
			newElems[i] = ApplyWithCycleCheck(e, s, visited)
		}
		return TTuple{Elements: newElems}

	case TRecord:
		newFields := make(map[string]Type, len(typ.Fields))
		for k, v := range typ.Fields {
			newFields[k] = ApplyWithCycleCheck(v, s, visited)
		}
		var newRow Type
		if typ.Row != nil {
			newRow = ApplyWithCycleCheck(typ.Row, s, visited)
		}
		return TRecord{Fields: newFields, Row: newRow, IsOpen: typ.IsOpen}

	case TUnion:
		newTypes := make([]Type, len(typ.Types))
		for i, t := range typ.Types {
			newTypes[i] = ApplyWithCycleCheck(t, s, visited)
		}
		return NormalizeUnion(newTypes)

	case TForall:
		// Filter substitution to exclude quantified variables
		newSubst := make(Subst)
		boundVars := make(map[string]bool)
		for _, v := range typ.Vars {
			boundVars[v.Name] = true
		}

		for k, v := range s {
			if !boundVars[k] {
				newSubst[k] = v
			}
		}

		// Apply to constraints
		newConstraints := make([]Constraint, len(typ.Constraints))
		for i, c := range typ.Constraints {
			newArgs := make([]Type, len(c.Args))
			for j, arg := range c.Args {
				newArgs[j] = ApplyWithCycleCheck(arg, newSubst, visited)
			}
			newConstraints[i] = Constraint{
				TypeVar: c.TypeVar,
				Trait:   c.Trait,
				Args:    newArgs,
			}
		}

		// Apply to body
		return TForall{
			Vars:        typ.Vars,
			Constraints: newConstraints,
			Type:        ApplyWithCycleCheck(typ.Type, newSubst, visited),
		}

	case TType:
		return TType{Type: ApplyWithCycleCheck(typ.Type, s, visited)}

	default:
		// Fallback for any other types
		return t.Apply(s)
	}
}

func copyVisited(m map[string]bool) map[string]bool {
	newMap := make(map[string]bool, len(m))
	for k, v := range m {
		newMap[k] = v
	}
	return newMap
}

func (t TVar) FreeTypeVariables() []TVar {
	return []TVar{t}
}

// TCon represents a type constant/constructor (e.g. Int, Bool, List).
type TCon struct {
	Name           string
	Module         string    // Optional module path for imported types
	UnderlyingType Type      // For type aliases: the underlying type (nil for regular types)
	TypeParams     *[]string // Names of type parameters for parameterized aliases
	KindVal        Kind      // Rename to KindVal to avoid method name conflict if embedded? No, struct field.
	// But let's call it KindVal just to be safe or just explicit. Or just 'Kind'.
	// In Go, field 'Kind' and method 'Kind()' on *TCon or TCon is fine if TCon is struct.
	// But TCon is a struct, not interface.
}

var builtinKinds map[string]Kind

func init() {
	builtinKinds = make(map[string]Kind)
	arrow1 := MakeArrow(Star, Star)       // * -> *
	arrow2 := MakeArrow(Star, Star, Star) // * -> * -> *

	// Register known builtins (sync with config/builtins.go)
	// * -> *
	builtinKinds["List"] = arrow1
	builtinKinds["Option"] = arrow1
	builtinKinds["Identity"] = arrow1
	builtinKinds["Functor"] = arrow1
	builtinKinds["Applicative"] = arrow1
	builtinKinds["Monad"] = arrow1
	builtinKinds["Empty"] = arrow1
	builtinKinds["Optional"] = arrow1

	// * -> * -> *
	builtinKinds["Map"] = arrow2
	builtinKinds["Result"] = arrow2
	builtinKinds["State"] = arrow2
	builtinKinds["Reader"] = arrow2
	builtinKinds["Writer"] = arrow2

	// OptionT :: (*->*) -> * -> *
	builtinKinds["OptionT"] = MakeArrow(arrow1, Star, Star)

	// ResultT :: (*->*) -> * -> * -> *
	builtinKinds["ResultT"] = MakeArrow(arrow1, Star, Star, Star)
}

func (t TCon) Kind() Kind {
	// If explicit kind is set, return it
	if t.KindVal != nil {
		return t.KindVal
	}
	// Fallback: lookup builtins
	if k, ok := builtinKinds[t.Name]; ok {
		return k
	}
	return Star
}

func (t TCon) String() string {
	name := t.Name
	if (config.IsTestMode || config.IsLSPMode) && strings.HasPrefix(name, "$skolem_") {
		// Normalize skolem constants (e.g. $skolem_x_8 -> $skolem_x_?)
		// This ensures deterministic output in tests and clean UI
		lastIdx := strings.LastIndex(name, "_")
		if lastIdx > 0 {
			suffix := name[lastIdx+1:]
			if _, err := strconv.Atoi(suffix); err == nil {
				name = name[:lastIdx] + "_?"
			}
		}
	}

	if t.Module != "" {
		return t.Module + "." + name
	}
	return name
}

func (t TCon) Apply(s Subst) Type {
	return ApplyWithCycleCheck(t, s, make(map[string]bool))
}

func (t TCon) FreeTypeVariables() []TVar {
	return []TVar{}
}

// UnwrapUnderlying recursively unwraps TCon.UnderlyingType until reaching a non-TCon type.
// Returns the innermost underlying type, or the original type if no UnderlyingType.
func UnwrapUnderlying(t Type) Type {
	for {
		tCon, ok := t.(TCon)
		if !ok || tCon.UnderlyingType == nil {
			return t
		}
		t = tCon.UnderlyingType
	}
}

// ExpandTypeAlias expands a type alias TApp by substituting type arguments into the underlying type.
// For example: StringResult<Int> where StringResult<t> = Result<String, t> becomes Result<String, Int>.
// It also handles higher-kinded aliases and partial applications correctly.
// Returns the original type if it's not an alias or cannot be expanded.
func ExpandTypeAlias(t Type) Type {
	tApp, ok := t.(TApp)
	if !ok {
		return t
	}

	tCon, ok := tApp.Constructor.(TCon)
	if !ok || tCon.UnderlyingType == nil {
		return t
	}

	numParams := 0
	if tCon.TypeParams != nil {
		numParams = len(*tCon.TypeParams)
	}

	// If we don't have enough arguments to satisfy the alias parameters,
	// we cannot fully expand it (Partial Alias Application).
	if len(tApp.Args) < numParams {
		return t
	}

	// 1. Expand the alias using the required number of arguments
	var expanded Type
	if numParams > 0 {
		subst := make(Subst)
		for i, paramName := range *tCon.TypeParams {
			subst[paramName] = tApp.Args[i]
		}
		expanded = tCon.UnderlyingType.Apply(subst)
	} else {
		// No parameters (e.g. type MyExpr = HFix<ExprF>)
		expanded = tCon.UnderlyingType
	}

	// 2. Apply any remaining arguments to the expanded type
	// e.g. MyExpr<Int> -> (HFix<ExprF>)<Int> -> HFix<ExprF, Int>
	remainingArgs := tApp.Args[numParams:]
	if len(remainingArgs) > 0 {
		// If expanded type is TApp, flatten it
		if expandedApp, ok := expanded.(TApp); ok {
			mergedArgs := append([]Type{}, expandedApp.Args...)
			mergedArgs = append(mergedArgs, remainingArgs...)
			expanded = TApp{Constructor: expandedApp.Constructor, Args: mergedArgs}
		} else {
			expanded = TApp{Constructor: expanded, Args: remainingArgs}
		}
	}

	return expanded
}

// TApp represents a type application (e.g. List Int).
type TApp struct {
	Constructor Type
	Args        []Type
	KindVal     Kind // Cache the kind
}

func (t TApp) Kind() Kind {
	if t.KindVal != nil {
		return t.KindVal
	}
	k := t.Constructor.Kind()
	for range t.Args {
		if arrow, ok := k.(KArrow); ok {
			k = arrow.Right
		} else {
			// Applying to non-arrow kind? Should be error or Star?
			// For now, assume Star
			return Star
		}
	}
	return k
}

func (t TApp) String() string {
	// Special case: List<Char> is displayed as String
	if tCon, ok := t.Constructor.(TCon); ok && tCon.Name == config.ListTypeName && len(t.Args) == 1 {
		if argCon, ok := t.Args[0].(TCon); ok && argCon.Name == "Char" {
			return "String"
		}
	}

	args := []string{}
	for _, arg := range t.Args {
		args = append(args, arg.String())
	}
	if len(args) == 0 {
		return t.Constructor.String()
	}
	return fmt.Sprintf("(%s %s)", t.Constructor.String(), strings.Join(args, " "))
}

func (t TApp) Apply(s Subst) Type {
	return ApplyWithCycleCheck(t, s, make(map[string]bool))
}

func (t TApp) FreeTypeVariables() []TVar {
	vars := []TVar{}
	vars = append(vars, t.Constructor.FreeTypeVariables()...)
	for _, arg := range t.Args {
		vars = append(vars, arg.FreeTypeVariables()...)
	}
	return uniqueTVars(vars)
}

// TTuple represents a tuple type (e.g. (Int, Bool)).
type TTuple struct {
	Elements []Type
}

func (t TTuple) Kind() Kind { return Star }

func (t TTuple) String() string {
	args := []string{}
	for _, el := range t.Elements {
		args = append(args, el.String())
	}
	return fmt.Sprintf("(%s)", strings.Join(args, ", "))
}

func (t TTuple) Apply(s Subst) Type {
	return ApplyWithCycleCheck(t, s, make(map[string]bool))
}

func (t TTuple) FreeTypeVariables() []TVar {
	vars := []TVar{}
	for _, el := range t.Elements {
		vars = append(vars, el.FreeTypeVariables()...)
	}
	return uniqueTVars(vars)
}

// TRecord represents a record type (e.g. { x: Int, y: Bool }).
type TRecord struct {
	Fields map[string]Type
	IsOpen bool // If true, this record can be extended (Row Polymorphism inference)
	Row    Type // Row variable for row polymorphism (usually TVar). If non-nil, IsOpen should ideally be true.
}

func (t TRecord) Kind() Kind { return Star }

func (t TRecord) String() string {
	fields := []string{}
	// Sort keys for deterministic output
	keys := []string{}
	for k := range t.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fields = append(fields, fmt.Sprintf("%s: %s", k, t.Fields[k].String()))
	}

	suffix := ""
	if t.Row != nil {
		suffix = " | " + t.Row.String()
	} else if t.IsOpen {
		suffix = ", ..."
	}

	if len(fields) == 0 && suffix != "" && strings.HasPrefix(suffix, " | ") {
		// Just row: { | r } -> { r }? No, usually { | r } or { r } syntax?
		// Funxy doesn't have syntax yet, so format as { | r }
		return fmt.Sprintf("{ %s }", suffix[3:])
	}

	return fmt.Sprintf("{ %s%s }", strings.Join(fields, ", "), suffix)
}

func (t TRecord) Apply(s Subst) Type {
	return ApplyWithCycleCheck(t, s, make(map[string]bool))
}

func (t TRecord) FreeTypeVariables() []TVar {
	vars := []TVar{}
	// Sort field names for deterministic order
	keys := make([]string, 0, len(t.Fields))
	for k := range t.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		vars = append(vars, t.Fields[k].FreeTypeVariables()...)
	}
	if t.Row != nil {
		vars = append(vars, t.Row.FreeTypeVariables()...)
	}
	return uniqueTVars(vars)
}

// TUnion represents a union type (e.g. Int | String | Nil).
// Types are normalized: flattened, deduplicated, and sorted for comparison.
type TUnion struct {
	Types []Type // At least 2 types
}

func (t TUnion) Kind() Kind { return Star }

func (t TUnion) String() string {
	parts := []string{}
	for _, typ := range t.Types {
		parts = append(parts, typ.String())
	}
	return strings.Join(parts, " | ")
}

func (t TUnion) Apply(s Subst) Type {
	return ApplyWithCycleCheck(t, s, make(map[string]bool))
}

func (t TUnion) FreeTypeVariables() []TVar {
	vars := []TVar{}
	for _, typ := range t.Types {
		vars = append(vars, typ.FreeTypeVariables()...)
	}
	return uniqueTVars(vars)
}

// NormalizeUnion creates a normalized union type.
// It flattens nested unions, removes duplicates, and sorts types.
func NormalizeUnion(types []Type) Type {
	// Flatten nested unions
	flat := []Type{}
	for _, t := range types {
		if u, ok := t.(TUnion); ok {
			flat = append(flat, u.Types...)
		} else {
			flat = append(flat, t)
		}
	}

	// Remove duplicates (using string representation for simplicity)
	seen := make(map[string]bool)
	unique := []Type{}
	for _, t := range flat {
		s := t.String()
		if !seen[s] {
			seen[s] = true
			unique = append(unique, t)
		}
	}

	// If only one type remains, return it directly
	if len(unique) == 1 {
		return unique[0]
	}

	// Sort for deterministic comparison
	sort.Slice(unique, func(i, j int) bool {
		return unique[i].String() < unique[j].String()
	})

	return TUnion{Types: unique}
}

// Constraint represents a type constraint (e.g. T: Show or T: Convert<U>)
type Constraint struct {
	TypeVar string
	Trait   string
	Args    []Type // Arguments for MPTC
}

// TFunc represents a function type (e.g. (Int, Int) -> Bool).
type TFunc struct {
	Params       []Type
	ReturnType   Type
	IsVariadic   bool
	DefaultCount int          // Number of parameters with default values (from the end)
	Constraints  []Constraint // Generic constraints (e.g. T: Show)
}

func (t TFunc) Kind() Kind { return Star }

func (t TFunc) String() string {
	params := []string{}
	defaultStart := len(t.Params) - t.DefaultCount
	if defaultStart < 0 {
		defaultStart = 0
	}

	for i, p := range t.Params {
		s := p.String()
		if i >= defaultStart {
			s += "?"
		}
		params = append(params, s)
	}
	if t.IsVariadic {
		if len(params) > 0 {
			params[len(params)-1] = "..." + params[len(params)-1]
		} else {
			params = append(params, "...")
		}
	}
	return fmt.Sprintf("(%s) -> %s", strings.Join(params, ", "), t.ReturnType.String())
}

func (t TFunc) Apply(s Subst) Type {
	return ApplyWithCycleCheck(t, s, make(map[string]bool))
}

func (t TFunc) FreeTypeVariables() []TVar {
	vars := []TVar{}
	for _, p := range t.Params {
		vars = append(vars, p.FreeTypeVariables()...)
	}
	vars = append(vars, t.ReturnType.FreeTypeVariables()...)

	// Include variables from constraints (they are implicit parameters)
	for _, c := range t.Constraints {
		vars = append(vars, TVar{Name: c.TypeVar})
		for _, arg := range c.Args {
			vars = append(vars, arg.FreeTypeVariables()...)
		}
	}

	return uniqueTVars(vars)
}

// TForall represents a universally quantified type (Rank-N).
// e.g. forall T. T -> T
type TForall struct {
	Vars        []TVar
	Constraints []Constraint
	Type        Type
}

func (t TForall) Kind() Kind { return Star }

func (t TForall) String() string {
	// In LSP mode, hide the explicit quantifier if it's just implicit polymorphism
	// to avoid cluttering the view with "forall t? t? ...".
	if config.IsLSPMode {
		return t.Type.String()
	}

	vars := []string{}
	for _, v := range t.Vars {
		varStr := v.String()

		// Collect all constraints for this variable
		var constraints []string
		for _, c := range t.Constraints {
			if c.TypeVar == v.Name {
				constraints = append(constraints, c.Trait)
			}
		}

		// Add constraints to variable string
		if len(constraints) > 0 {
			varStr += ": " + strings.Join(constraints, ", ")
		}

		vars = append(vars, varStr)
	}
	return fmt.Sprintf("forall %s. %s", strings.Join(vars, " "), t.Type.String())
}

func (t TForall) Apply(s Subst) Type {
	return ApplyWithCycleCheck(t, s, make(map[string]bool))
}

func (t TForall) FreeTypeVariables() []TVar {
	bound := make(map[string]bool)
	for _, v := range t.Vars {
		bound[v.Name] = true
	}

	free := t.Type.FreeTypeVariables()

	// Also collect free variables from constraints
	for _, c := range t.Constraints {
		for _, arg := range c.Args {
			constraintFree := arg.FreeTypeVariables()
			free = append(free, constraintFree...)
		}
	}

	result := []TVar{}
	for _, v := range free {
		if !bound[v.Name] {
			result = append(result, v)
		}
	}
	return uniqueTVars(result)
}

// TType represents the type of a Type (Meta-type).
// e.g. Int (the value) has type TType{Type: Int}.
type TType struct {
	Type Type
}

func (t TType) Kind() Kind { return Star }

func (t TType) String() string { return fmt.Sprintf("Type<%s>", t.Type.String()) }

func (t TType) Apply(s Subst) Type {
	return TType{Type: t.Type.Apply(s)}
}

func (t TType) FreeTypeVariables() []TVar {
	return t.Type.FreeTypeVariables()
}

// Subst is a mapping from Type Variables to Types.
type Subst map[string]Type

// Compose combines two substitutions.
func (s1 Subst) Compose(s2 Subst) Subst {
	subst := Subst{}
	for k, v := range s2 {
		subst[k] = v
	}
	for k, v := range s1 {
		subst[k] = v.Apply(s2)
	}
	return subst
}

func uniqueTVars(vars []TVar) []TVar {
	unique := []TVar{}
	seen := map[string]bool{}
	for _, v := range vars {
		if !seen[v.Name] {
			seen[v.Name] = true
			unique = append(unique, v)
		}
	}
	return unique
}

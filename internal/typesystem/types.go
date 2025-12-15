package typesystem

import (
	"fmt"
	"github.com/funvibe/funxy/internal/config"
	"sort"
	"strings"
)

// Type is the interface for all types in our system.
type Type interface {
	String() string
	Apply(Subst) Type
	FreeTypeVariables() []TVar
}

// TVar represents a type variable (e.g. 'a', 'b', 't1').
type TVar struct {
	Name string
}

func (t TVar) String() string { return t.Name }

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
			newConstraints[i] = Constraint{TypeVar: newTypeVar, Trait: c.Trait}
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
		return TRecord{Fields: newFields, IsOpen: typ.IsOpen}

	case TUnion:
		newTypes := make([]Type, len(typ.Types))
		for i, t := range typ.Types {
			newTypes[i] = ApplyWithCycleCheck(t, s, visited)
		}
		return NormalizeUnion(newTypes)

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
	Module         string // Optional module path for imported types
	UnderlyingType Type   // For type aliases: the underlying type (nil for regular types)
}

func (t TCon) String() string {
	if t.Module != "" {
		return t.Module + "." + t.Name
	}
	return t.Name
}

func (t TCon) Apply(s Subst) Type {
	return t
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

// TApp represents a type application (e.g. List Int).
type TApp struct {
	Constructor Type
	Args        []Type
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
}

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
	if t.IsOpen {
		return fmt.Sprintf("{ %s, ... }", strings.Join(fields, ", "))
	}
	return fmt.Sprintf("{ %s }", strings.Join(fields, ", "))
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
	return uniqueTVars(vars)
}

// TUnion represents a union type (e.g. Int | String | Nil).
// Types are normalized: flattened, deduplicated, and sorted for comparison.
type TUnion struct {
	Types []Type // At least 2 types
}

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

// Constraint represents a type constraint (e.g. T: Show)
type Constraint struct {
	TypeVar string
	Trait   string
}

// TFunc represents a function type (e.g. (Int, Int) -> Bool).
type TFunc struct {
	Params       []Type
	ReturnType   Type
	IsVariadic   bool
	DefaultCount int          // Number of parameters with default values (from the end)
	Constraints  []Constraint // Generic constraints (e.g. T: Show)
}

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
			params[len(params)-1] += "..."
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
	return uniqueTVars(vars)
}

// TType represents the type of a Type (Meta-type).
// e.g. Int (the value) has type TType{Type: Int}.
type TType struct {
	Type Type
}

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

package analyzer

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

// CheckExhaustiveness verifies that a match expression covers all possible cases.
// It returns an error if not exhaustive.
func CheckExhaustiveness(n *ast.MatchExpression, targetType typesystem.Type, table *symbols.SymbolTable) error {
	// 1. Get all patterns from arms
	patterns := make([]ast.Pattern, len(n.Arms))
	for i, c := range n.Arms {
		patterns[i] = c.Pattern
	}

	// 2. Check if exhaustive
	if isExhaustive(targetType, patterns, table) {
		return nil
	}

	// 3. If not, generate error message
	missing := getMissingPatterns(targetType, patterns, table)
	return notExhaustive(n, missing)
}

// isExhaustive checks if the given set of patterns covers the target type.
func isExhaustive(t typesystem.Type, patterns []ast.Pattern, table *symbols.SymbolTable) bool {
	// 1. Check for Wildcard or Variable patterns (catch-all) at top level
	for _, p := range patterns {
		if isCatchAll(p) {
			return true
		}
	}

	// Resolve type aliases
	realType := resolveType(t)

	// Handle TVar (Type Variable) by deducing type from patterns
	if _, ok := realType.(typesystem.TVar); ok {
		deducedType := deduceTypeFromPatterns(patterns, table)
		if deducedType != nil {
			realType = resolveType(deducedType)
		} else {
			// Fallback: check if patterns themselves imply exhaustiveness (e.g. RecordPattern catch-all)
			// This is handled in case typesystem.TVar below.
		}
	}

	// Special handling for List (built-in or treated as such)
	isList := false
	if tCon, ok := realType.(typesystem.TCon); ok && tCon.Name == config.ListTypeName {
		isList = true
	} else if tApp, ok := realType.(typesystem.TApp); ok {
		if tCon, ok := tApp.Constructor.(typesystem.TCon); ok && tCon.Name == config.ListTypeName {
			isList = true
		} else if tCon, ok := tApp.Constructor.(*typesystem.TCon); ok && tCon.Name == config.ListTypeName {
			isList = true
		}
	}

	if isList {
		hasEmpty := false
		hasCons := false // Covers [h, t...] (length >= 1)

		for _, p := range patterns {
			// Handle ListPattern AND TuplePattern (some tests use (x, ...xs) for variadic args which are Lists)
			var elements []ast.Pattern
			if lp, ok := p.(*ast.ListPattern); ok {
				elements = lp.Elements
			} else if tp, ok := p.(*ast.TuplePattern); ok {
				elements = tp.Elements
			} else {
				continue
			}

			if len(elements) == 0 {
				hasEmpty = true
			} else {
				// Check for spread
				lastIdx := len(elements) - 1
				if _, ok := elements[lastIdx].(*ast.SpreadPattern); ok {
					// [x...] covers everything?
					if len(elements) == 1 {
						return true
					}
					// [head, tail...] covers >= 1
					if len(elements) >= 2 {
						hasCons = true
					}
				}
			}
		}
		if hasEmpty && hasCons {
			return true
		}
	}

	switch rt := realType.(type) {
	case typesystem.TCon:
		// Boolean
		if rt.Name == "Bool" {
			hasTrue := false
			hasFalse := false
			for _, p := range patterns {
				if lp, ok := p.(*ast.LiteralPattern); ok {
					if val, ok := lp.Value.(bool); ok {
						if val {
							hasTrue = true
						} else {
							hasFalse = true
						}
					}
				}
			}
			return hasTrue && hasFalse
		}

		// ADTs (Custom Types)
		// Get variants for this type
		variants, ok := table.GetVariants(rt.Name)
		if ok && len(variants) > 0 {
			// Check if all constructors are covered
			coveredVariants := make(map[string]bool)

			// Collect covered constructors
			for _, p := range patterns {
				if cp, ok := p.(*ast.ConstructorPattern); ok {
					coveredVariants[cp.Name.Value] = true
				}
			}

			for _, v := range variants {
				if !coveredVariants[v] {
					return false
				}
			}

			return checkAdtExhaustiveness(rt.Name, variants, patterns, table)
		}

		// Other TCons (Int, String, Char, etc.)
		return false

	case typesystem.TApp:
		// Probably an ADT with generics (e.g. Option[Int], List[String], Result[String, (Int, Task)])
		if constructor, ok := rt.Constructor.(typesystem.TCon); ok {
			variants, ok := table.GetVariants(constructor.Name)
			if ok && len(variants) > 0 {
				// Pass type args for proper type resolution in nested patterns
				return checkAdtExhaustivenessWithTypeArgs(constructor.Name, variants, patterns, table, rt.Args)
			}
		}
		return false // Default to non-exhaustive if no catch-all

	case typesystem.TTuple:
		// For tuples, we need to check if all element positions are exhaustive.
		// Collect all tuple patterns with matching arity.
		var tuplePatterns []*ast.TuplePattern
		for _, p := range patterns {
			if tp, ok := p.(*ast.TuplePattern); ok {
				hasSpread := false
				if len(tp.Elements) > 0 {
					if _, ok := tp.Elements[len(tp.Elements)-1].(*ast.SpreadPattern); ok {
						hasSpread = true
					}
				}

				if hasSpread {
					// Fixed elements count must be <= tuple length
					if len(tp.Elements)-1 > len(rt.Elements) {
						continue
					}
				} else {
					if len(tp.Elements) != len(rt.Elements) {
						continue
					}
				}
				tuplePatterns = append(tuplePatterns, tp)
			}
		}

		if len(tuplePatterns) == 0 {
			return false
		}

		// Quick check: if any pattern has all catch-alls, it's exhaustive
		for _, tp := range tuplePatterns {
			allCatchAll := true
			for _, el := range tp.Elements {
				if _, ok := el.(*ast.SpreadPattern); ok {
					continue
				}
				if !isCatchAll(el) {
					allCatchAll = false
					break
				}
			}
			if allCatchAll {
				return true
			}
		}

		// Check each tuple position independently for exhaustiveness
		// This handles cases like (MkUserId x, MkUserId y) where MkUserId is the only constructor
		for col := 0; col < len(rt.Elements); col++ {
			columnPatterns := make([]ast.Pattern, 0, len(tuplePatterns))
			for _, tp := range tuplePatterns {
				if col < len(tp.Elements) {
					columnPatterns = append(columnPatterns, tp.Elements[col])
				}
			}
			if !isExhaustive(rt.Elements[col], columnPatterns, table) {
				return false
			}
		}
		return true

	case typesystem.TVar:
		// Should have been handled by deduceTypeFromPatterns, but if deduction failed:
		return false

	case typesystem.TRecord:
		// Check if the ORIGINAL type (before unwrapping) is an ADT-as-Alias
		// If t is TCon or TApp(TCon), and it has variants, we should check ADT patterns
		if tCon, ok := t.(typesystem.TCon); ok {
			variants, ok := table.GetVariants(tCon.Name)
			if ok && len(variants) > 0 {
				if checkAdtExhaustiveness(tCon.Name, variants, patterns, table) {
					return true
				}
			}
		} else if tApp, ok := t.(typesystem.TApp); ok {
			if constructor, ok := tApp.Constructor.(typesystem.TCon); ok {
				variants, ok := table.GetVariants(constructor.Name)
				if ok && len(variants) > 0 {
					if checkAdtExhaustivenessWithTypeArgs(constructor.Name, variants, patterns, table, tApp.Args) {
						return true
					}
				}
			}
		}

		// For records, check for catch-all record pattern
		for _, p := range patterns {
			if rp, ok := p.(*ast.RecordPattern); ok {
				// We don't enforce that pattern has all fields of t.
				// Missing fields in pattern are treated as wildcards for those fields.
				// We only check if the fields PRESENT in pattern are catch-alls.

				allFieldsCatchAll := true
				for _, subPat := range rp.Fields {
					if !isCatchAll(subPat) {
						allFieldsCatchAll = false
						break
					}
				}
				if allFieldsCatchAll {
					return true
				}
			}
		}
		return false

	case typesystem.TUnion:
		// For union types, check if all members are covered by type patterns
		// Each member of the union must be covered
		for _, member := range rt.Types {
			memberCovered := false
			for _, p := range patterns {
				if tp, ok := p.(*ast.TypePattern); ok {
					// Check if this type pattern covers this member
					patType := buildTypeFromAst(tp.Type, table)
					if typesMatch(patType, member) {
						memberCovered = true
						break
					}
				}
				// Catch-all patterns cover everything
				if isCatchAll(p) {
					memberCovered = true
					break
				}
				// Constructor patterns for Nil
				if cp, ok := p.(*ast.ConstructorPattern); ok {
					if cp.Name.Value == "Nil" && isNilType(member) {
						memberCovered = true
						break
					}
				}
			}
			if !memberCovered {
				return false
			}
		}
		return true

	default:
		return false
	}
}

func isCatchAll(p ast.Pattern) bool {
	switch p.(type) {
	case *ast.WildcardPattern:
		return true
	case *ast.IdentifierPattern:
		return true
	case *ast.SpreadPattern:
		return true
	default:
		return false
	}
}

// buildTypeFromAst converts an AST type to a typesystem type for exhaustiveness checking
func buildTypeFromAst(t ast.Type, table *symbols.SymbolTable) typesystem.Type {
	var errs []*diagnostics.DiagnosticError
	return BuildType(t, table, &errs)
}

// typesMatch checks if two types are equivalent for exhaustiveness purposes
func typesMatch(t1, t2 typesystem.Type) bool {
	// Simple string comparison for now
	return t1.String() == t2.String()
}

// isNilType checks if a type is the Nil type
func isNilType(t typesystem.Type) bool {
	if t == typesystem.Nil {
		return true
	}
	if tcon, ok := t.(typesystem.TCon); ok {
		return tcon.Name == "Nil"
	}
	return false
}

// deduceTypeFromPatterns attempts to infer the type from the patterns themselves.
// This is useful when the subject type is a Type Variable (unknown).
func deduceTypeFromPatterns(patterns []ast.Pattern, table *symbols.SymbolTable) typesystem.Type {
	for _, p := range patterns {
		switch p := p.(type) {
		case *ast.LiteralPattern:
			if _, ok := p.Value.(bool); ok {
				return typesystem.TCon{Name: "Bool"}
			}
		case *ast.ListPattern:
			// Assume List
			return typesystem.TCon{Name: config.ListTypeName}
		case *ast.ConstructorPattern:
			// Look up constructor to find type
			sym, ok := table.Find(p.Name.Value)
			if ok {
				// sym.Type is typically TFunc or TCon (for 0-ary)
				// We want the return type of the constructor
				if tFunc, ok := sym.Type.(typesystem.TFunc); ok {
					return tFunc.ReturnType
				}
				if tCon, ok := sym.Type.(typesystem.TCon); ok {
					return tCon
				}
				// If it's a value (0-ary constructor defined as value?), might be TApp or TCon
				return sym.Type
			}
		case *ast.TuplePattern:
			// Deduce Tuple type
			// We try to deduce element types from sub-patterns
			elements := make([]typesystem.Type, len(p.Elements))
			for i, el := range p.Elements {
				deduced := deduceTypeFromPatterns([]ast.Pattern{el}, table)
				if deduced != nil {
					elements[i] = deduced
				} else {
					elements[i] = typesystem.TVar{Name: "unknown"}
				}
			}
			return typesystem.TTuple{Elements: elements}
		case *ast.RecordPattern:
			// Deduce Record type with field names
			// Types of fields are unknown (TVar), but presence of fields is known.
			fields := make(map[string]typesystem.Type)
			for name := range p.Fields {
				fields[name] = typesystem.TVar{Name: "unknown"}
			}
			return typesystem.TRecord{Fields: fields}
		}
	}
	return nil
}

// checkAdtExhaustiveness checks if all variants of an ADT are covered.
// It also checks recursively if arguments of constructors are covered.
func checkAdtExhaustiveness(typeName string, variants []string, patterns []ast.Pattern, table *symbols.SymbolTable) bool {
	return checkAdtExhaustivenessWithTypeArgs(typeName, variants, patterns, table, nil)
}

func checkAdtExhaustivenessWithTypeArgs(typeName string, variants []string, patterns []ast.Pattern, table *symbols.SymbolTable, typeArgs []typesystem.Type) bool {
	for _, variant := range variants {
		// Find patterns that match this variant
		var matchingPatterns []ast.Pattern
		for _, p := range patterns {
			if cp, ok := p.(*ast.ConstructorPattern); ok {
				if cp.Name.Value == variant {
					matchingPatterns = append(matchingPatterns, cp)
				}
			}
		}

		if len(matchingPatterns) == 0 {
			return false // Variant not covered at all
		}

		// If variant has arguments, check if they are covered.
		ctorSym, ok := table.Find(variant)
		if !ok {
			continue
		}

		// Determine arity
		arity := 0
		var paramTypes []typesystem.Type

		if tFunc, ok := ctorSym.Type.(typesystem.TFunc); ok {
			arity = len(tFunc.Params)
			paramTypes = tFunc.Params

			// If we have type args, substitute type variables in paramTypes
			// This resolves e.g. T in Ok(T) to the actual type like (Int, Task)
			if len(typeArgs) > 0 {
				// Build substitution from return type's type vars to typeArgs
				// e.g., Result<e, t> with typeArgs [String, (Int, Task)] -> {e: String, t: (Int, Task)}
				subst := buildTypeVarSubst(tFunc.ReturnType, typeArgs)

				// Apply substitution to param types
				resolvedParams := make([]typesystem.Type, len(paramTypes))
				for i, pt := range paramTypes {
					resolvedParams[i] = pt.Apply(subst)
				}
				paramTypes = resolvedParams
			}
		}

		if arity == 0 {
			continue
		}

		// Collect argument pattern rows
		argumentPatternRows := make([][]ast.Pattern, 0, len(matchingPatterns))
		for _, mp := range matchingPatterns {
			cp := mp.(*ast.ConstructorPattern)
			if len(cp.Elements) != arity {
				continue
			}
			argumentPatternRows = append(argumentPatternRows, cp.Elements)
		}

		// Quick check: if any row is all catch-alls, this variant is fully covered
		fullyCovered := false
		for _, row := range argumentPatternRows {
			allCatchAll := true
			for _, pat := range row {
				if !isCatchAll(pat) {
					allCatchAll = false
					break
				}
			}
			if allCatchAll {
				fullyCovered = true
				break
			}
		}

		if !fullyCovered {
			// Check each column (argument position) for exhaustiveness
			// All columns must be exhaustive for the variant to be covered
			for col := 0; col < arity; col++ {
				columnPatterns := make([]ast.Pattern, len(argumentPatternRows))
				for i, row := range argumentPatternRows {
					columnPatterns[i] = row[col]
				}
				if !isExhaustive(paramTypes[col], columnPatterns, table) {
					return false
				}
			}
		}
	}
	return true
}

func getMissingPatterns(t typesystem.Type, patterns []ast.Pattern, table *symbols.SymbolTable) string {
	// Simplified reporting
	realType := resolveType(t)

	// Try to deduce type if TVar
	if _, ok := realType.(typesystem.TVar); ok {
		deducedType := deduceTypeFromPatterns(patterns, table)
		if deducedType != nil {
			realType = resolveType(deducedType)
		}
	}

	switch t := realType.(type) {
	case typesystem.TCon:
		if t.Name == "Bool" {
			hasTrue := false
			hasFalse := false
			for _, p := range patterns {
				if lp, ok := p.(*ast.LiteralPattern); ok {
					if val, ok := lp.Value.(bool); ok {
						if val {
							hasTrue = true
						} else {
							hasFalse = true
						}
					}
				}
			}
			if !hasTrue {
				return "true"
			}
			if !hasFalse {
				return "false"
			}
		}

		// Check for List type
		if t.Name == config.ListTypeName {
			return getMissingListPatterns(patterns)
		}

		variants, ok := table.GetVariants(t.Name)
		if ok {
			missing := []string{}
			covered := make(map[string]bool)
			for _, p := range patterns {
				if cp, ok := p.(*ast.ConstructorPattern); ok {
					covered[cp.Name.Value] = true
				}
			}
			for _, v := range variants {
				if !covered[v] {
					missing = append(missing, v)
				}
			}
			if len(missing) > 0 {
				return fmt.Sprintf("%v", missing)
			}
			return "some pattern combinations"
		}

		// Int, String, etc - infinite cases
		return fmt.Sprintf("other %s values (add _ or default case)", t.Name)

	case typesystem.TApp:
		// Check for List<T>
		if constructor, ok := t.Constructor.(typesystem.TCon); ok {
			if constructor.Name == config.ListTypeName {
				return getMissingListPatterns(patterns)
			}

			variants, ok := table.GetVariants(constructor.Name)
			if ok {
				missing := []string{}
				covered := make(map[string]bool)
				for _, p := range patterns {
					if cp, ok := p.(*ast.ConstructorPattern); ok {
						covered[cp.Name.Value] = true
					}
				}
				for _, v := range variants {
					if !covered[v] {
						missing = append(missing, v)
					}
				}
				if len(missing) > 0 {
					return fmt.Sprintf("%v", missing)
				}
			}
		}
		return "_ (catch-all pattern)"

	case typesystem.TRecord:
		// For records, suggest a record pattern
		fields := make([]string, 0, len(t.Fields))
		for name := range t.Fields {
			fields = append(fields, name)
		}
		if len(fields) > 0 {
			return fmt.Sprintf("{ %s: _ } or _ (catch-all)", fields[0])
		}
		return "{ } or _ (catch-all)"

	case typesystem.TTuple:
		// For tuples, suggest tuple pattern
		return fmt.Sprintf("(%d-element tuple pattern) or _", len(t.Elements))

	case typesystem.TUnion:
		// For unions, list uncovered members
		var missing []string
		for _, member := range t.Types {
			memberCovered := false
			for _, p := range patterns {
				if tp, ok := p.(*ast.TypePattern); ok {
					patType := buildTypeFromAst(tp.Type, table)
					if typesMatch(patType, member) {
						memberCovered = true
						break
					}
				}
				if isCatchAll(p) {
					memberCovered = true
					break
				}
				// Constructor patterns for Nil
				if cp, ok := p.(*ast.ConstructorPattern); ok {
					if cp.Name.Value == "Nil" && isNilType(member) {
						memberCovered = true
						break
					}
				}
			}
			if !memberCovered {
				missing = append(missing, member.String())
			}
		}
		if len(missing) > 0 {
			return fmt.Sprintf("type patterns for: %v", missing)
		}
		return "_ (catch-all pattern)"
	}
	return "_ (catch-all pattern)"
}

// getMissingListPatterns returns helpful message about missing list patterns
func getMissingListPatterns(patterns []ast.Pattern) string {
	hasEmpty := false
	hasNonEmpty := false

	for _, p := range patterns {
		var elements []ast.Pattern
		if lp, ok := p.(*ast.ListPattern); ok {
			elements = lp.Elements
		} else if tp, ok := p.(*ast.TuplePattern); ok {
			elements = tp.Elements
		} else {
			continue
		}

		if len(elements) == 0 {
			hasEmpty = true
		} else {
			// Check if it covers non-empty lists
			lastIdx := len(elements) - 1
			if _, ok := elements[lastIdx].(*ast.SpreadPattern); ok {
				hasNonEmpty = true
			}
		}
	}

	if !hasEmpty && !hasNonEmpty {
		return "[] (empty list), [x, ...xs] (non-empty list)"
	}
	if !hasEmpty {
		return "[] (empty list)"
	}
	if !hasNonEmpty {
		return "[x, ...xs] (non-empty list)"
	}
	return "_ (catch-all pattern)"
}

func resolveType(t typesystem.Type) typesystem.Type {
	switch t := t.(type) {
	case typesystem.TType:
		return resolveType(t.Type)
	default:
		// Use UnwrapUnderlying for TCon with UnderlyingType
		return typesystem.UnwrapUnderlying(t)
	}
}

// buildTypeVarSubst extracts type variables from returnType and maps them to typeArgs.
// e.g., Result<e, t> with typeArgs [String, (Int, Task)] -> {e: String, t: (Int, Task)}
func buildTypeVarSubst(returnType typesystem.Type, typeArgs []typesystem.Type) typesystem.Subst {
	subst := typesystem.Subst{}

	// Extract type vars from TApp structure
	if tApp, ok := returnType.(typesystem.TApp); ok {
		for i, arg := range tApp.Args {
			if i < len(typeArgs) {
				if tVar, ok := arg.(typesystem.TVar); ok {
					subst[tVar.Name] = typeArgs[i]
				}
			}
		}
	}

	return subst
}

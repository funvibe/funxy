package analyzer

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
	"sort"
	"strings"
	"unicode"
)

// BuildType converts an AST Type node into a typesystem.Type.
func BuildType(t ast.Type, table *symbols.SymbolTable, errs *[]*diagnostics.DiagnosticError) typesystem.Type {
	if t == nil {
		return typesystem.TCon{Name: "Unknown"}
	}
	switch t := t.(type) {
	case *ast.NamedType:
		name := t.Name.Value

		// 0. Check for qualified type names (e.g., "module.Type")
		// These should NOT be treated as type variables even if they start with lowercase
		isQualified := strings.Contains(name, ".")

		// For qualified names, return TCon with module info AND underlying type
		// This preserves nominal type for extension method lookup while allowing unification
		if isQualified {
			parts := strings.SplitN(name, ".", 2)
			if len(parts) == 2 {
				moduleName := parts[0]
				typeName := parts[1]

				// Resolve underlying type via symbol table
				var underlyingType typesystem.Type
				if table != nil {
					if resolved, ok := table.ResolveType(name); ok {
						underlyingType = resolved
					}
				}

				tBase := typesystem.TCon{Name: typeName, Module: moduleName, UnderlyingType: underlyingType}

				// Handle generic arguments
				if len(t.Args) > 0 {
					args := []typesystem.Type{}
					for _, arg := range t.Args {
						args = append(args, BuildType(arg, table, errs))
					}
					return typesystem.TApp{Constructor: tBase, Args: args}
				}
				return tBase
			}
		}

		// 1. Check if Type Variable or Rigid Type Parameter
		isTypeParam := false
		var typeParamType typesystem.Type
		if table != nil && !isQualified {
			// Check if defined in symbol table
			if sym, ok := table.Find(name); ok && sym.Kind == symbols.TypeSymbol {
				// Skip type parameter detection for type aliases - they should resolve to underlying type
				if !sym.IsTypeAlias() {
					switch symType := sym.Type.(type) {
					case typesystem.TVar:
						isTypeParam = true
						typeParamType = symType
					case typesystem.TCon:
						// Check if this is a rigid type parameter (TCon with same name)
						// This happens during body analysis where type params are registered as TCon
						if symType.Name == name && symType.Module == "" {
							isTypeParam = true
							typeParamType = symType
						}
					}
				}
			} else if len(name) > 0 && unicode.IsLower(rune(name[0])) {
				// Implicit Generic Discovery (Case Sensitivity Rule)
				// If it starts with lowercase and is NOT in the symbol table,
				// treat it as a new Type Variable and register it in the current scope.
				isTypeParam = true
				tVar := typesystem.TVar{Name: name}
				typeParamType = tVar

				// Register in table to reuse in this scope (e.g. map(a -> b, List<a>))
				table.DefineType(name, tVar, "")
			}
		}

		// If uppercase name is NOT found in symbol table:
		// Strictly enforce "Uppercase = Concrete Type" rule.
		// It MUST be defined (or be a built-in).
		if !isTypeParam && !isQualified && len(name) > 0 && unicode.IsUpper(rune(name[0])) && table != nil {
			// Check if this type is defined anywhere
			_, isDefined := table.Find(name)
			_, isType := table.ResolveType(name)

			if !isDefined && !isType {
				// Types that require import (not in prelude)
				requiresImport := map[string]string{
					"Uuid":     "lib/uuid",
					"Logger":   "lib/log",
					"Task":     "lib/task",
					"SqlValue": "lib/sql",
					"SqlDB":    "lib/sql",
					"SqlTx":    "lib/sql",
					"Date":     "lib/date",
					"Json":     "lib/json",
				}
				if pkg, needsImport := requiresImport[name]; needsImport {
					if errs != nil {
						*errs = append(*errs, diagnostics.NewError(
							diagnostics.ErrA006,
							t.GetToken(),
							fmt.Sprintf("type '%s' requires import \"%s\"", name, pkg),
						))
					}
				} else {
					// Error: Undeclared type
					if errs != nil {
						*errs = append(*errs, diagnostics.NewError(
							diagnostics.ErrA002,
							t.GetToken(),
							name,
						))
					}
					// Fallback to unknown TCon to avoid cascading panics
					return typesystem.TCon{Name: name}
				}
			}
		}

		if isTypeParam {
			// If it has arguments, it's Higher-Kinded Type application (e.g. F<A>)
			if len(t.Args) > 0 {
				args := []typesystem.Type{}
				for _, arg := range t.Args {
					args = append(args, BuildType(arg, table, errs))
				}
				return typesystem.TApp{Constructor: typeParamType, Args: args}
			}
			return typeParamType
		}

		// 2. Built-in types check (if not shadowed)
		// ...

		// 3. Resolve Type Alias
		if table != nil {
			if resolved, ok := table.ResolveType(name); ok {
				// Check if it's a TCon with the same name (not an alias, but the type itself)
				isAlias := true
				if tCon, ok := resolved.(typesystem.TCon); ok && tCon.Name == name {
					isAlias = false
				}

				if isAlias {
					// It's an Alias.
					// We must validate kinds BEFORE substitution to catch bad applications.
					// Retrieve Kind of the Alias itself from table.
					aliasKind := typesystem.Star
					if k, ok := table.GetKind(name); ok {
						aliasKind = k
					}

					args := []typesystem.Type{}
					for _, arg := range t.Args {
						args = append(args, BuildType(arg, table, errs))
					}

					// Kind Validation Logic (same as TCon)
					currentKind := aliasKind
					for i, arg := range args {
						arrow, ok := currentKind.(typesystem.KArrow)
						if !ok {
							*errs = append(*errs, diagnostics.NewError(
								diagnostics.ErrA003, // Type Error
								t.GetToken(),
								fmt.Sprintf("Type %s has kind %s, cannot be applied to argument %d", name, aliasKind, i+1),
							))
							break
						}

						argKind := GetKind(arg, table)
						if !arrow.Left.Equal(argKind) {
							*errs = append(*errs, diagnostics.NewError(
								diagnostics.ErrA003,
								t.Args[i].GetToken(),
								fmt.Sprintf("Type argument mismatch: expected kind %s, got %s", arrow.Left, argKind),
							))
						}
						currentKind = arrow.Right
					}

					// If it has arguments, return TApp instead of substituted TCon
					// This ensures correct recursive resolution using ResolveTypeAlias
					if len(args) > 0 {
						// Look up the actual TCon definition to preserve TypeParams and UnderlyingType
						tCon := typesystem.TCon{Name: name}
						if sym, ok := table.Find(name); ok && sym.Kind == symbols.TypeSymbol {
							if realTCon, ok := sym.Type.(typesystem.TCon); ok {
								tCon = realTCon
							}
						}

						if params, ok := table.GetTypeParams(name); ok {
							if len(params) == len(args) {
								return typesystem.TApp{Constructor: tCon, Args: args}
							}
						}
						// If params mismatch, fall through or return TApp anyway?
						// Return TApp so Unify can handle it (and maybe fail on arity)
						return typesystem.TApp{Constructor: tCon, Args: args}
					}

					// Return TCon with underlying type for nominal type preservation (non-generic alias)
					return typesystem.TCon{Name: name, UnderlyingType: resolved}
				}
			}
		}

		// 4. Default: TCon
		var tBase typesystem.Type = typesystem.TCon{Name: name}

		// Attempt to preserve TCon info from Symbol Table (Module, UnderlyingType)
		if table != nil {
			if sym, ok := table.Find(name); ok && sym.Kind == symbols.TypeSymbol {
				tBase = sym.Type
				// Handle case where TypeSymbol holds TType wrapper (e.g. for types used as values)
				// We want the underlying type definition
				if tType, ok := tBase.(typesystem.TType); ok {
					tBase = tType.Type
				}
			} else if resolved, ok := table.ResolveType(name); ok {
				// Fallback: Check ResolveType (handles types shadowed by constructors)
				if tCon, ok := resolved.(typesystem.TCon); ok && tCon.Name == name {
					tBase = tCon
				}
			}

			// Ensure Kind is set correctly (crucial for KindCheck)
			if tCon, ok := tBase.(typesystem.TCon); ok {
				// Always update Kind from registry if available, as TCon might be stale or missing it
				if k, ok := table.GetKind(name); ok {
					tCon.KindVal = k
					tBase = tCon
				}
			}
		}

		// Validate Kind if arguments are present
		if len(t.Args) > 0 {
			args := []typesystem.Type{}
			for _, arg := range t.Args {
				args = append(args, BuildType(arg, table, errs))
			}

			if table != nil && errs != nil {
				// Check Kind
				conKind := typesystem.Star
				if k, ok := table.GetKind(name); ok {
					conKind = k
				}

				currentKind := conKind
				for i, arg := range args {
					arrow, ok := currentKind.(typesystem.KArrow)
					if !ok {
						*errs = append(*errs, diagnostics.NewError(
							diagnostics.ErrA003, // Type Error
							t.GetToken(),
							fmt.Sprintf("Type %s has kind %s, cannot be applied to argument %d", name, conKind, i+1),
						))
						break
					}

					argKind := GetKind(arg, table)
					if !arrow.Left.Equal(argKind) {
						*errs = append(*errs, diagnostics.NewError(
							diagnostics.ErrA003,
							t.Args[i].GetToken(),
							fmt.Sprintf("Type argument mismatch: expected kind %s, got %s", arrow.Left, argKind),
						))
					}
					currentKind = arrow.Right
				}
			}

			return typesystem.TApp{Constructor: tBase, Args: args}
		}
		return tBase

	case *ast.TupleType:
		elements := []typesystem.Type{}
		for _, el := range t.Types {
			elements = append(elements, BuildType(el, table, errs))
		}
		return typesystem.TTuple{Elements: elements}

	case *ast.RecordType:
		fields := make(map[string]typesystem.Type)
		// Sort keys for deterministic processing
		keys := make([]string, 0, len(t.Fields))
		for k := range t.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fields[k] = BuildType(t.Fields[k], table, errs)
		}
		return typesystem.TRecord{Fields: fields}

	case *ast.FunctionType:
		params := []typesystem.Type{}
		for _, p := range t.Parameters {
			params = append(params, BuildType(p, table, errs))
		}
		return typesystem.TFunc{
			Params:     params,
			ReturnType: BuildType(t.ReturnType, table, errs),
			IsVariadic: false,
		}

	case *ast.UnionType:
		types := []typesystem.Type{}
		for _, ut := range t.Types {
			types = append(types, BuildType(ut, table, errs))
		}
		return typesystem.NormalizeUnion(types)

	case *ast.ForallType:
		var vars []typesystem.TVar
		var innerTable *symbols.SymbolTable
		if table != nil {
			innerTable = symbols.NewEnclosedSymbolTable(table, symbols.ScopeFunction)
		}

		var constraints []typesystem.Constraint
		for _, p := range t.Vars {
			tv := typesystem.TVar{Name: p.Value}
			vars = append(vars, tv)
			if innerTable != nil {
				innerTable.DefineType(p.Value, tv, "")
			}

			for _, c := range p.Constraints {
				constraints = append(constraints, typesystem.Constraint{
					TypeVar: p.Value,
					Trait:   c.Trait,
					Args:    []typesystem.Type{},
				})
			}
		}

		return typesystem.TForall{
			Vars:        vars,
			Constraints: constraints,
			Type:        BuildType(t.Type, innerTable, errs),
		}

	default:
		return typesystem.TCon{Name: "Unknown"}
	}
}

func GetKind(t typesystem.Type, table *symbols.SymbolTable) typesystem.Kind {
	if table == nil {
		return typesystem.Star
	}
	switch t := t.(type) {
	case typesystem.TCon:
		if k, ok := table.GetKind(t.Name); ok {
			return k
		}
		// Maybe it's an alias?
		if res, ok := table.ResolveType(t.Name); ok {
			if _, ok := res.(typesystem.TCon); !ok { // Avoid infinite loop if resolves to self
				return GetKind(res, table)
			}
		}
		return typesystem.Star
	case typesystem.TVar:
		if k, ok := table.GetKind(t.Name); ok {
			return k
		}
		return typesystem.Star
	case typesystem.TApp:
		k := GetKind(t.Constructor, table)
		for range t.Args {
			if arrow, ok := k.(typesystem.KArrow); ok {
				k = arrow.Right
			} else {
				return typesystem.Star
			}
		}
		return k
	case typesystem.TFunc:
		return typesystem.Star
	case typesystem.TRecord:
		return typesystem.Star
	case typesystem.TTuple:
		return typesystem.Star
	case typesystem.TForall:
		return typesystem.Star // ? or Kind of body?
	}
	return typesystem.Star
}

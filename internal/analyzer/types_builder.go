package analyzer

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
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
						// Could be a rigid type parameter - check if it looks like a type param name
						// (single uppercase letter or short uppercase name)
						if len(name) <= 2 || (len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z') {
							isTypeParam = true
							typeParamType = symType
						}
					}
				}
			}
		}
	}
	// If uppercase name is NOT found in symbol table:
	// - Short names (1-2 chars like T, A, E) -> type variable (generic parameter)
	// - Long names (3+ chars) that are NOT in symbol table -> error (undefined type)
	// This prevents using types like Uuid without importing lib/uuid
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
				"Date":     "lib/sql",
			}

			if pkg, needsImport := requiresImport[name]; needsImport {
				// This type requires import
				*errs = append(*errs, diagnostics.NewError(
					diagnostics.ErrA006, // Undefined symbol
					t.GetToken(),
					fmt.Sprintf("type '%s' requires import \"%s\"", name, pkg),
				))
				return typesystem.TCon{Name: name} // Return TCon anyway to continue checking
			} else if len(name) <= 2 {
				// Short name not found -> treat as type variable (T, A, E, etc.)
				isTypeParam = true
				typeParamType = typesystem.TVar{Name: name}
			}
			// Else: unknown long type name - let it through, might be defined elsewhere
		}
	}

	if isTypeParam {
		// HKT: If type parameter has arguments, create TApp with type param as constructor
		// Example: F<A> where F is a type parameter -> TApp{Constructor: F, Args: [A]}
		if len(t.Args) > 0 {
			args := []typesystem.Type{}
			for _, arg := range t.Args {
				args = append(args, BuildType(arg, table, errs))
			}
			return typesystem.TApp{
				Constructor: typeParamType,
				Args:        args,
			}
		}
		return typeParamType
	}

		// 2. Check if "String" (special case alias)
		if name == "String" {
			return typesystem.TApp{
				Constructor: typesystem.TCon{Name: config.ListTypeName},
				Args:        []typesystem.Type{typesystem.TCon{Name: "Char"}},
			}
		}

		// 3. Check Symbol Table for Alias Resolution
		if table != nil {
			if resolved, ok := table.ResolveType(name); ok {
				// Check if it is an Alias (resolved != TCon(name))
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

				// If it has arguments, perform substitution.
				if len(args) > 0 {
					if params, ok := table.GetTypeParams(name); ok {
						if len(params) == len(args) {
							subst := typesystem.Subst{}
							for i, p := range params {
								subst[p] = args[i]
							}
							appliedResolved := resolved.Apply(subst)
							return typesystem.TCon{Name: name, UnderlyingType: appliedResolved}
						}
					}
					// If generic params missing or count mismatch, return TCon with underlying
					return typesystem.TCon{Name: name, UnderlyingType: resolved}
				}
				// Return TCon with underlying type for nominal type preservation
				return typesystem.TCon{Name: name, UnderlyingType: resolved}
				}
			}
		}

		// 4. Default: TCon
		tBase := typesystem.TCon{Name: name}

		// Attempt to preserve TCon info from Symbol Table (Module, UnderlyingType)
		if table != nil {
			if resolved, ok := table.ResolveType(name); ok {
				if tCon, ok := resolved.(typesystem.TCon); ok && tCon.Name == name {
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
	}
	return typesystem.TCon{Name: "Unknown"}
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
	}
	return typesystem.Star
}

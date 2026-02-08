package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
	"github.com/funvibe/funxy/internal/utils"
)

// resolveTypeAlias resolves a type alias to its underlying type.
// If the type is a TCon and has a registered alias, returns the underlying type.
// Otherwise returns the original type unchanged.
func resolveTypeAlias(t typesystem.Type, table *symbols.SymbolTable) typesystem.Type {
	if table != nil {
		return table.ResolveTypeAlias(t)
	}
	// Fallback logic if table is nil
	// First try to unwrap TCon.UnderlyingType
	if tCon, ok := t.(typesystem.TCon); ok {
		if tCon.UnderlyingType != nil {
			return typesystem.UnwrapUnderlying(t)
		}
	}
	return t
}

func isVirtualLibModule(ctx *InferenceContext, packageName string) bool {
	if ctx == nil || ctx.Loader == nil || packageName == "" || packageName == "lib" {
		return false
	}
	modInterface := ctx.Loader.GetModuleByPackageName(packageName)
	if mod, ok := modInterface.(*modules.Module); ok {
		return mod.IsVirtual
	}
	return false
}

func lookupModuleMemberFallback(ctx *InferenceContext, n *ast.MemberExpression, table *symbols.SymbolTable, tRec typesystem.TRecord) (typesystem.Type, bool) {
	ident, ok := n.Left.(*ast.Identifier)
	if !ok {
		return nil, false
	}
	sym, ok := table.Find(ident.Value)
	if !ok || sym.Kind != symbols.ModuleSymbol {
		return nil, false
	}
	packageName, ok := table.GetPackageNameByAlias(ident.Value)
	if !ok {
		packageName = ident.Value
	}
	if !isVirtualLibModule(ctx, packageName) {
		return nil, false
	}
	altName := utils.ModuleMemberFallbackName(packageName, n.Member.Value)
	if altName == "" || altName == n.Member.Value {
		return nil, false
	}
	if fieldType, ok := tRec.Fields[altName]; ok {
		instType, _ := instantiateMemberType(ctx, n, fieldType, table)
		if ctx.TypeMap != nil {
			ctx.TypeMap[n.Member] = instType
		}
		return instType, true
	}
	return nil, false
}

// isImplementationInSourceModule checks if a type implements a trait in its source module.
// This is used when the type has a module prefix (e.g., s.Circle).
func isImplementationInSourceModule(ctx *InferenceContext, traitName string, t typesystem.Type, table *symbols.SymbolTable) bool {
	if ctx.Loader == nil {
		return false
	}

	// Get module alias from type
	moduleAlias := ""
	typeName := ""
	switch tt := t.(type) {
	case typesystem.TCon:
		moduleAlias = tt.Module
		typeName = tt.Name
	case typesystem.TApp:
		if tCon, ok := tt.Constructor.(typesystem.TCon); ok {
			moduleAlias = tCon.Module
			typeName = tCon.Name
		}
	}

	if moduleAlias == "" {
		return false
	}

	// Convert alias to package name
	packageName, ok := table.GetPackageNameByAlias(moduleAlias)
	if !ok {
		packageName = moduleAlias
	}

	// Find the source module
	modInterface := ctx.Loader.GetModuleByPackageName(packageName)
	if modInterface == nil {
		return false
	}

	loadedMod, ok := modInterface.(LoadedModule)
	if !ok {
		return false
	}

	// Check in source module's symbol table
	modTable := loadedMod.GetSymbolTable()
	if modTable == nil {
		return false
	}

	// Create type without module prefix for lookup
	localType := typesystem.TCon{Name: typeName}
	if tApp, ok := t.(typesystem.TApp); ok {
		localType = typesystem.TCon{Name: typeName}
		// Reconstruct TApp with local constructor
		t = typesystem.TApp{Constructor: localType, Args: tApp.Args}
	} else {
		t = localType
	}

	return modTable.IsImplementationExists(traitName, []typesystem.Type{t})
}

// lookupExtensionMethodInSourceModule looks up an extension method in the module
// where the type is defined (via TCon.Module field).
// This is the primary lookup mechanism - extension methods live in their source module.
func lookupExtensionMethodInSourceModule(ctx *InferenceContext, t typesystem.Type, methodName string, table *symbols.SymbolTable) (typesystem.Type, bool) {
	if ctx.Loader == nil {
		return nil, false
	}

	// Get module alias from type (TCon.Module contains the alias used during import)
	moduleAlias := ""
	switch tt := t.(type) {
	case typesystem.TCon:
		moduleAlias = tt.Module
	case typesystem.TApp:
		if tCon, ok := tt.Constructor.(typesystem.TCon); ok {
			moduleAlias = tCon.Module
		}
	}

	if moduleAlias == "" {
		return nil, false
	}

	// Convert alias to package name
	packageName, ok := table.GetPackageNameByAlias(moduleAlias)
	if !ok {
		// Alias not found, try using moduleAlias as package name directly
		packageName = moduleAlias
	}

	// Find the source module by package name
	modInterface := ctx.Loader.GetModuleByPackageName(packageName)
	if modInterface == nil {
		return nil, false
	}

	loadedMod, ok := modInterface.(LoadedModule)
	if !ok {
		return nil, false
	}

	// Look up extension method in source module's symbol table
	modTable := loadedMod.GetSymbolTable()
	if modTable == nil {
		return nil, false
	}

	// Get type name without module prefix
	typeName := ""
	switch tt := t.(type) {
	case typesystem.TCon:
		typeName = tt.Name
	case typesystem.TApp:
		if tCon, ok := tt.Constructor.(typesystem.TCon); ok {
			typeName = tCon.Name
		}
	}

	if typeName == "" {
		return nil, false
	}

	return modTable.GetExtensionMethod(typeName, methodName)
}

func isString(t typesystem.Type) bool {
	if tCon, ok := t.(typesystem.TCon); ok && tCon.Name == "String" {
		return true
	}
	if tApp, ok := t.(typesystem.TApp); ok {
		if tCon, ok := tApp.Constructor.(typesystem.TCon); ok && tCon.Name == config.ListTypeName {
			if len(tApp.Args) == 1 {
				if tArg, ok := tApp.Args[0].(typesystem.TCon); ok && tArg.Name == "Char" {
					return true
				}
			}
		}
	}
	return false
}

func inferMemberExpression(ctx *InferenceContext, n *ast.MemberExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	leftType, s1, err := inferFn(n.Left, table)
	if err != nil {
		return nil, nil, err
	}

	totalSubst := s1

	// Keep original type for extension method lookup (before alias resolution)
	originalType := leftType

	// Resolve type aliases: if leftType is a TCon, check if it's an alias
	leftType = resolveTypeAlias(leftType, table)

	// If leftType is still a TVar, check ExpectedTypes context for potential resolution
	// This helps with code like: r = pure(5); r >>= ...
	if tv, ok := leftType.(typesystem.TVar); ok {
		// Only check if left is an identifier
		if ident, ok := n.Left.(*ast.Identifier); ok {
			// Find symbol to get the definition node
			sym, found := table.Find(ident.Value)
			if found && sym.DefinitionNode != nil {
				// Check ExpectedTypes using the definition node
				if expectedType, hasExpected := ctx.ExpectedTypes[sym.DefinitionNode]; hasExpected {
					// Unify the TVar with the expected type
					subst, err := typesystem.Unify(tv, expectedType)
					if err == nil {
						totalSubst = subst.Compose(totalSubst)
						leftType = expectedType.Apply(totalSubst)
						originalType = leftType
					}
				}
			}
		}
	}

	// Handle optional chaining (?.)
	if n.IsOptional {
		return inferOptionalChain(ctx, n, leftType, totalSubst, table, inferFn)
	}

	// 1. Check Record Field
	if tRec, ok := leftType.(typesystem.TRecord); ok {
		// Populate ResolutionMap for the member if it exists
		if ctx.ResolutionMap != nil {
			// For records, we don't have a specific DefinitionNode for the field unless we track it during record literal creation or type definition.
			// If TRecord comes from a type definition (unwrapped TCon), we could find the field definition.
			// But TRecord structure doesn't store source info.
			// We can at least resolve to something if possible, but skipping for now.
		}

		if fieldType, ok := tRec.Fields[n.Member.Value]; ok {
			instType, _ := instantiateMemberType(ctx, n, fieldType, table)
			// Explicitly set type for the member identifier for LSP hover
			if ctx.TypeMap != nil {
				ctx.TypeMap[n.Member] = instType
			}
			return instType, totalSubst, nil
		}
		if fallbackType, ok := lookupModuleMemberFallback(ctx, n, table, tRec); ok {
			return fallbackType, totalSubst, nil
		} else if tRec.IsOpen {
			// Row Polymorphism: Inferred from TVar usage.
			// We generate a substitution to refine the record type.
			newFieldType := ctx.FreshVar()
			newFields := make(map[string]typesystem.Type)
			for k, v := range tRec.Fields {
				newFields[k] = v
			}
			newFields[n.Member.Value] = newFieldType

			// Refine the record type to include the new field
			// We need to find the original TVar that resolved to this TRecord?
			// Not necessarily. If leftType is TRecord, it might be a literal or already resolved.
			// If it's a literal, we can't extend it unless it's marked Open (which literals are not by default).
			// Only records coming from TVar unification are Open.
			// But wait, if leftType IS TRecord, where is the TVar?
			// The TVar was replaced by TRecord in a previous step.
			// If we want to further refine it, we are mutating the TRecord?
			// No, types are immutable-ish.
			// If we return a new TRecord, does it propagate back?
			// Only if we have a handle to the original TVar.
			// But if we are here, `leftType` IS TRecord.
			// This logic seems to rely on `table.Update` to retroactive change type of `ident`.

			// For now, let's keep the logic but use Substitution if possible.
			// If leftType is TRecord, we can't substitute TRecord with TRecord in Subst (Subst maps Name -> Type).
			// Unless we have the Name.

			// Previous logic used table.Update on `ident.Value`.
			// If `n.Left` is Identifier, we can find its TVar name?
			// `inferFn` resolved it to `leftType`.
			// If `leftType` came from `table.Find`, and it was `TRecord` in the table.

			if ident, ok := n.Left.(*ast.Identifier); ok {
				newRecType := typesystem.TRecord{Fields: newFields, IsOpen: true}
				// We want to update ident's type in the context.
				// Since we can't put TRecord -> TRecord in Subst,
				// and we returned TRecord from inferFn (meaning table has TRecord),
				// we probably need to Update table or return a "constraint" that TRecordA = TRecordB.
				// But Unify(TRecord, TRecord) will unify fields.

				// If we use table.Update here, it's a side effect.
				// But we are moving to Subst.
				// Ideally we should have `TVar` here, but `inferFn` returned resolved `TRecord`.
				// This implies that earlier inference already fixed it to a Record.

				// Let's stick to `table.Update` for this specific case as it mimics "mutation" of the open record in the environment,
				// OR (better) assume we can't change it if it's already a Record in the table?
				// The previous code allowed it.

				if err := table.Update(ident.Value, newRecType); err != nil {
					return nil, nil, inferErrorf(n, "failed to refine type of %s: %v", ident.Value, err)
				}
				return newFieldType, totalSubst, nil
			} else {
				return ctx.FreshVar(), totalSubst, nil
			}
		}
	}

	// 1.5 Check Trait Method (Dictionary Passing)
	// Check if the member name corresponds to a method of a trait that the type implements.
	if traitName, ok := table.GetTraitForMethod(n.Member.Value); ok {
		// Get trait method signature (Generic)
		if methodType, ok := table.GetTraitMethodType(n.Member.Value); ok {
			// Check if leftType implements traitName
			// We need to resolve the witness to be sure (and to get the dictionary)
			// SolveWitness checks implementation and ambiguity.

			// We need the concrete type for witness resolution
			concreteType := leftType.Apply(totalSubst)

			// For MPTC, we need to know WHICH argument this type corresponds to?
			// Usually dot notation is for the FIRST argument (Receiver).
			// So we assume Left is Arg[0].

			// Resolving Witness
			resolvedArgs := []typesystem.Type{concreteType}
			// Check if trait has > 1 param.
			typeParams, _ := table.GetTraitTypeParams(traitName)
			if len(typeParams) > 1 {
				// MPTC: We can't solve witness with just Receiver yet (unless other params are fixed/default).
				// We skip VTable optimization for MPTC member access for now?
				// Or we report error "MPTC method syntax x.method() not supported, use Trait.method(x, ...)"?
				// Haskell doesn't support dot notation for MPTC easily either.
				// Let's assume single param for dot notation for now.
			} else {
				// Solve Witness
				witness, err := ctx.SolveWitness(n, traitName, resolvedArgs, table)
				if err == nil {
					// Found Witness!
					n.Dictionary = witness
					idx, _ := table.GetTraitMethodIndex(traitName, n.Member.Value)
					n.MethodIndex = idx

					// Return type: The method type with receiver applied (Extension method style)
					// Instantiate generics
					freshType := InstantiateWithContext(ctx, methodType)
					if tFunc, ok := freshType.(typesystem.TFunc); ok && len(tFunc.Params) > 0 {
						recvParam := tFunc.Params[0]
						subst, err := typesystem.Unify(recvParam, leftType)
						if err == nil {
							totalSubst = subst.Compose(totalSubst)
							newParams := make([]typesystem.Type, len(tFunc.Params)-1)
							for i, p := range tFunc.Params[1:] {
								newParams[i] = p.Apply(subst)
							}
							return typesystem.TFunc{
								Params:     newParams,
								ReturnType: tFunc.ReturnType.Apply(subst),
								IsVariadic: tFunc.IsVariadic,
							}, totalSubst, nil
						}
					}
				}
			}
		}
	}

	// 2. Check Extension Method
	// First try to find in source module (where the type is defined)
	if extMethodType, ok := lookupExtensionMethodInSourceModule(ctx, originalType, n.Member.Value, table); ok {
		freshType := InstantiateWithContext(ctx, extMethodType)
		if tFunc, ok := freshType.(typesystem.TFunc); ok && len(tFunc.Params) > 0 {
			recvParam := tFunc.Params[0]
			subst, err := typesystem.Unify(recvParam, leftType)
			if err != nil {
				return nil, nil, inferErrorf(n, "extension method %s receiver mismatch: expected %s, got %s", n.Member.Value, recvParam, leftType)
			}
			totalSubst = subst.Compose(totalSubst)
			newParams := make([]typesystem.Type, len(tFunc.Params)-1)
			for i, p := range tFunc.Params[1:] {
				newParams[i] = p.Apply(subst)
			}
			return typesystem.TFunc{
				Params:     newParams,
				ReturnType: tFunc.ReturnType.Apply(subst),
				IsVariadic: tFunc.IsVariadic,
			}, totalSubst, nil
		}
	}

	// Check extension method by type alias name (for imported type aliases)
	if tRec, ok := leftType.(typesystem.TRecord); ok {
		if aliasName, ok := table.FindTypeAliasForRecord(tRec); ok && aliasName != "" {
			if extMethodType, ok := table.GetExtensionMethod(aliasName, n.Member.Value); ok {
				freshType := InstantiateWithContext(ctx, extMethodType)
				if tFunc, ok := freshType.(typesystem.TFunc); ok && len(tFunc.Params) > 0 {
					recvParam := tFunc.Params[0]
					subst, err := typesystem.Unify(recvParam, leftType)
					if err != nil {
						return nil, nil, inferErrorf(n, "extension method %s receiver mismatch: expected %s, got %s", n.Member.Value, recvParam, leftType)
					}
					totalSubst = subst.Compose(totalSubst)
					newParams := make([]typesystem.Type, len(tFunc.Params)-1)
					for i, p := range tFunc.Params[1:] {
						newParams[i] = p.Apply(subst)
					}
					return typesystem.TFunc{
						Params:     newParams,
						ReturnType: tFunc.ReturnType.Apply(subst),
						IsVariadic: tFunc.IsVariadic,
					}, totalSubst, nil
				}
			}
		}
	}

	// Fallback: check local symbol table (for locally defined types)
	typeName := getCanonicalTypeName(leftType)
	if typeName != "" {
		if typeName == "Module" {
			return ctx.FreshVar(), totalSubst, nil
		} else {
			if extMethodType, ok := table.GetExtensionMethod(typeName, n.Member.Value); ok {
				freshType := InstantiateWithContext(ctx, extMethodType)

				if tFunc, ok := freshType.(typesystem.TFunc); ok {
					if len(tFunc.Params) > 0 {
						recvParam := tFunc.Params[0]
						subst, err := typesystem.Unify(recvParam, leftType)
						if err != nil {
							return nil, nil, inferErrorf(n, "extension method %s receiver mismatch: expected %s, got %s", n.Member.Value, recvParam, leftType)
						}
						totalSubst = subst.Compose(totalSubst)

						newParams := make([]typesystem.Type, len(tFunc.Params)-1)
						for i, p := range tFunc.Params[1:] {
							newParams[i] = p.Apply(subst)
						}

						return typesystem.TFunc{
							Params:     newParams,
							ReturnType: tFunc.ReturnType.Apply(subst),
							IsVariadic: tFunc.IsVariadic,
						}, totalSubst, nil
					}
				}
			}
		}
	}

	// 3. Named Type / Alias
	if tCon, ok := leftType.(typesystem.TCon); ok {
		if resolvedType, ok := table.ResolveType(tCon.Name); ok {
			if tRec, ok := resolvedType.(typesystem.TRecord); ok {
				if fieldType, ok := tRec.Fields[n.Member.Value]; ok {
					instType, _ := instantiateMemberType(ctx, n, fieldType, table)
					return instType, totalSubst, nil
				}
			}

			if extMethodType, ok := table.GetExtensionMethod(tCon.Name, n.Member.Value); ok {
				freshType := InstantiateWithContext(ctx, extMethodType)
				if tFunc, ok := freshType.(typesystem.TFunc); ok && len(tFunc.Params) > 0 {
					recvParam := tFunc.Params[0]
					subst, err := typesystem.Unify(recvParam, leftType)
					if err != nil {
						return nil, nil, inferErrorf(n, "extension method %s receiver mismatch: expected %s, got %s", n.Member.Value, recvParam, leftType)
					}
					totalSubst = subst.Compose(totalSubst)
					newParams := make([]typesystem.Type, len(tFunc.Params)-1)
					for i, p := range tFunc.Params[1:] {
						newParams[i] = p.Apply(subst)
					}
					return typesystem.TFunc{
						Params:     newParams,
						ReturnType: tFunc.ReturnType.Apply(subst),
						IsVariadic: tFunc.IsVariadic,
					}, totalSubst, nil
				}
			}

			resolvedTypeName := getCanonicalTypeName(resolvedType)
			if resolvedTypeName != "" {
				if extMethodType, ok := table.GetExtensionMethod(resolvedTypeName, n.Member.Value); ok {
					freshType := InstantiateWithContext(ctx, extMethodType)
					if tFunc, ok := freshType.(typesystem.TFunc); ok && len(tFunc.Params) > 0 {
						recvParam := tFunc.Params[0]
						subst, err := typesystem.Unify(recvParam, resolvedType)
						if err != nil {
							return nil, nil, inferErrorf(n, "extension method %s receiver mismatch: expected %s, got %s", n.Member.Value, recvParam, resolvedType)
						}
						totalSubst = subst.Compose(totalSubst)
						newParams := make([]typesystem.Type, len(tFunc.Params)-1)
						for i, p := range tFunc.Params[1:] {
							newParams[i] = p.Apply(subst)
						}
						return typesystem.TFunc{
							Params:     newParams,
							ReturnType: tFunc.ReturnType.Apply(subst),
							IsVariadic: tFunc.IsVariadic,
						}, totalSubst, nil
					}
				}
			}
		}
	}

	if tApp, ok := leftType.(typesystem.TApp); ok {
		constructorName := getCanonicalTypeName(tApp.Constructor)
		if constructorName != "" {
			if extMethodType, ok := table.GetExtensionMethod(constructorName, n.Member.Value); ok {
				freshType := InstantiateWithContext(ctx, extMethodType)
				if tFunc, ok := freshType.(typesystem.TFunc); ok && len(tFunc.Params) > 0 {
					recvParam := tFunc.Params[0]
					subst, err := typesystem.Unify(recvParam, leftType)
					if err != nil {
						return nil, nil, inferErrorf(n, "extension method %s receiver mismatch: expected %s, got %s", n.Member.Value, recvParam, leftType)
					}
					totalSubst = subst.Compose(totalSubst)
					newParams := make([]typesystem.Type, len(tFunc.Params)-1)
					for i, p := range tFunc.Params[1:] {
						newParams[i] = p.Apply(subst)
					}
					return typesystem.TFunc{
						Params:     newParams,
						ReturnType: tFunc.ReturnType.Apply(subst),
						IsVariadic: tFunc.IsVariadic,
					}, totalSubst, nil
				}
			}
		}
	}

	if isString(leftType) {
		if extMethodType, ok := table.GetExtensionMethod("String", n.Member.Value); ok {
			freshType := InstantiateWithContext(ctx, extMethodType)
			if tFunc, ok := freshType.(typesystem.TFunc); ok && len(tFunc.Params) > 0 {
				recvParam := tFunc.Params[0]
				subst, err := typesystem.Unify(recvParam, leftType)
				if err != nil {
					return nil, nil, inferErrorf(n, "extension method %s receiver mismatch: expected %s, got %s", n.Member.Value, recvParam, leftType)
				}
				totalSubst = subst.Compose(totalSubst)
				newParams := make([]typesystem.Type, len(tFunc.Params)-1)
				for i, p := range tFunc.Params[1:] {
					newParams[i] = p.Apply(subst)
				}
				return typesystem.TFunc{
					Params:     newParams,
					ReturnType: tFunc.ReturnType.Apply(subst),
					IsVariadic: tFunc.IsVariadic,
				}, totalSubst, nil
			}
		}
	}

	// 4. Handle TVar (Row Polymorphism Inference)
	if tv, ok := leftType.(typesystem.TVar); ok {
		if ident, ok := n.Left.(*ast.Identifier); ok {
			// Check if the identifier matches the TVar logic.
			// If leftType IS a TVar, we can return a Substitution!

			newFieldType := ctx.FreshVar()
			openRecord := typesystem.TRecord{
				Fields: map[string]typesystem.Type{
					n.Member.Value: newFieldType,
				},
				IsOpen: true,
			}

			// Substitute tv with openRecord
			subst := typesystem.Subst{tv.Name: openRecord}
			totalSubst = subst.Compose(totalSubst)

			// Also update table for legacy consistency?
			// Update table IS needed if we rely on table lookups elsewhere not using Subst.
			// But ideally we rely on Subst.
			// Let's update table too to be safe for now.
			if err := table.Update(ident.Value, openRecord); err != nil {
				return nil, nil, inferErrorf(n, "failed to refine type of %s (TVar): %v", ident.Value, err)
			}

			return newFieldType, totalSubst, nil
		} else {
			return ctx.FreshVar(), totalSubst, nil
		}
	}

	// 5. HostObject (Embed API)
	if tCon, ok := leftType.(typesystem.TCon); ok && tCon.Name == "HostObject" {
		// Allow any member access on HostObject, return a fresh type variable (dynamic)
		return ctx.FreshVar(), totalSubst, nil
	}

	if _, ok := leftType.(typesystem.TRecord); ok {
		return nil, nil, inferErrorf(n, "record %s has no field or extension method '%s'", leftType, n.Member.Value)
	}
	return nil, nil, inferErrorf(n, "dot access expects Record or Extension Method, got %s", leftType)
}

func inferIndexExpression(ctx *InferenceContext, n *ast.IndexExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	leftType, s1, err := inferFn(n.Left, table)
	if err != nil {
		return nil, nil, err
	}

	totalSubst := s1

	indexType, s2, err := inferFn(n.Index, table)
	if err != nil {
		return nil, nil, err
	}
	totalSubst = s2.Compose(totalSubst)

	// Apply accumulated subst
	leftType = leftType.Apply(totalSubst)
	indexType = indexType.Apply(totalSubst)

	// Expand type aliases (e.g., IntMap<String> -> Map<Int, String>)
	leftType = typesystem.ExpandTypeAlias(leftType)

	// Handle Map indexing: map[key] -> Option<V>
	if tApp, ok := leftType.(typesystem.TApp); ok {
		if tCon, ok := tApp.Constructor.(typesystem.TCon); ok && tCon.Name == config.MapTypeName && len(tApp.Args) == 2 {
			keyType := tApp.Args[0]
			valType := tApp.Args[1]

			// Unify index with key type
			subst, err := typesystem.Unify(keyType, indexType)
			if err != nil {
				return nil, nil, inferErrorf(n, "map key type mismatch: expected %s, got %s", keyType, indexType)
			}
			totalSubst = subst.Compose(totalSubst)

			// Return Option<V>
			resultType := typesystem.TApp{
				Constructor: typesystem.TCon{Name: config.OptionTypeName},
				Args:        []typesystem.Type{valType.Apply(totalSubst)},
			}
			return resultType, totalSubst, nil
		}
	}

	// For List/Tuple/Bytes, index must be Int
	if subst, err := typesystem.Unify(typesystem.Int, indexType); err != nil {
		return nil, nil, inferErrorf(n, "index must be Int, got %s", indexType)
	} else {
		totalSubst = subst.Compose(totalSubst)
	}

	// Handle Bytes indexing: bytes[i] -> Option<Int>
	if tCon, ok := leftType.(typesystem.TCon); ok && tCon.Name == config.BytesTypeName {
		resultType := typesystem.TApp{
			Constructor: typesystem.TCon{Name: config.OptionTypeName},
			Args:        []typesystem.Type{typesystem.Int},
		}
		return resultType, totalSubst, nil
	}

	// Handle Tuple indexing
	if tupleType, ok := leftType.(typesystem.TTuple); ok {
		// Try to extract index value from AST for constant folding
		if intLit, ok := n.Index.(*ast.IntegerLiteral); ok {
			idx := int(intLit.Value)
			if idx < 0 {
				idx = len(tupleType.Elements) + idx
			}
			if idx < 0 || idx >= len(tupleType.Elements) {
				return nil, nil, inferErrorf(n, "tuple index %d out of bounds (tuple has %d elements)", intLit.Value, len(tupleType.Elements))
			}
			return tupleType.Elements[idx].Apply(totalSubst), totalSubst, nil
		}
		// For non-literal index, return a fresh type variable
		// The actual type will be determined at runtime
		return ctx.FreshVar(), totalSubst, nil
	}

	// If the left side is still unknown, don't force it to List here.
	// Defer until other constraints (e.g., assignments, returns) clarify.
	if _, ok := leftType.(typesystem.TVar); ok {
		return ctx.FreshVar(), totalSubst, nil
	}

	// Handle List indexing
	itemType := ctx.FreshVar()
	expectedListType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{itemType},
	}

	subst, err := typesystem.Unify(expectedListType, leftType)
	if err != nil {
		return nil, nil, inferErrorf(n, "index operator expects List or Tuple, got %s", leftType)
	}
	totalSubst = subst.Compose(totalSubst)

	resultType := itemType.Apply(totalSubst)
	// Debug: print result type for list indexing
	// fmt.Printf("[DEBUG inferIndexExpression] itemType=%v, totalSubst=%v, resultType=%v\n", itemType, totalSubst, resultType)
	return resultType, totalSubst, nil
}

// inferOptionalChain handles the ?. operator for optional chaining
// Uses the Optional trait - works with any type implementing Optional<F>
// F<A>?.field -> F<B> where B is the type of the field
func inferOptionalChain(ctx *InferenceContext, n *ast.MemberExpression, leftType typesystem.Type, totalSubst typesystem.Subst, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	// Allow nullable unions (T | Nil) to use optional chaining.
	if union, ok := leftType.(typesystem.TUnion); ok {
		nonNil, hasNil := splitNilUnion(union)
		if hasNil {
			if len(nonNil) == 0 {
				return typesystem.Nil, totalSubst, nil
			}

			fieldTypes := make([]typesystem.Type, 0, len(nonNil)+1)
			for _, baseType := range nonNil {
				ft, fieldSubst, err := inferMemberOnType(ctx, n, baseType, totalSubst, table, inferFn)
				if err != nil {
					return nil, nil, err
				}
				totalSubst = fieldSubst.Compose(totalSubst)
				fieldTypes = append(fieldTypes, ft)
			}

			fieldTypes = append(fieldTypes, typesystem.Nil)
			return typesystem.NormalizeUnion(fieldTypes), totalSubst, nil
		}
	}

	// Check that leftType implements Optional trait
	if !CheckTraitImplementation(leftType, "Optional", table) {
		return nil, nil, inferErrorf(n, "optional chaining (?.) requires type implementing Optional, got %s", leftType)
	}

	// leftType must be F<A, ...> - extract constructor and type args
	tApp, ok := leftType.(typesystem.TApp)
	if !ok {
		return nil, nil, inferErrorf(n, "optional chaining (?.) requires a type constructor F<A>, got %s", leftType)
	}

	// Extract inner type A using trait method signature
	innerType, ok := getOptionalInnerType(leftType, table)
	if !ok {
		return nil, nil, inferErrorf(n, "type must have at least one type argument for optional chaining")
	}

	// Create a temporary MemberExpression with IsOptional=false to infer the field access
	tempNode := &ast.MemberExpression{
		Token:      n.Token,
		Left:       n.Left,
		Member:     n.Member,
		IsOptional: false,
	}

	// Infer field access on the inner type: A.field -> B
	fieldType, fieldSubst, err := inferMemberOnType(ctx, tempNode, innerType, totalSubst, table, inferFn)
	if err != nil {
		return nil, nil, err
	}
	totalSubst = fieldSubst

	// Find which argument position the inner type is at by checking where innerType appears
	innerTypePos := findInnerTypePosition(tApp, innerType)

	// Wrap result in the same container, replacing only the inner type position
	newArgs := make([]typesystem.Type, len(tApp.Args))
	for i := range tApp.Args {
		if i == innerTypePos {
			newArgs[i] = fieldType
		} else {
			newArgs[i] = tApp.Args[i]
		}
	}

	return typesystem.TApp{
		Constructor: tApp.Constructor,
		Args:        newArgs,
	}, totalSubst, nil
}

func splitNilUnion(u typesystem.TUnion) ([]typesystem.Type, bool) {
	nonNil := make([]typesystem.Type, 0, len(u.Types))
	hasNil := false
	for _, t := range u.Types {
		if isNilType(t) {
			hasNil = true
			continue
		}
		nonNil = append(nonNil, t)
	}
	return nonNil, hasNil
}

// findInnerTypePosition finds which argument position matches the inner type
// by comparing structural equality. Falls back to 0 if not found.
func findInnerTypePosition(tApp typesystem.TApp, innerType typesystem.Type) int {
	for i, arg := range tApp.Args {
		if typesEqual(arg, innerType) {
			return i
		}
	}
	// Fallback to first position
	return 0
}

// typesEqual checks structural equality of types
func typesEqual(a, b typesystem.Type) bool {
	// Try unification - if it succeeds with empty subst, they're equal
	// For simple comparison, just use string representation
	return a.String() == b.String()
}

// inferMemberOnType infers the type of accessing a member on a specific type
func inferMemberOnType(ctx *InferenceContext, n *ast.MemberExpression, baseType typesystem.Type, totalSubst typesystem.Subst, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	// Resolve TCon with UnderlyingType (type aliases)
	resolvedType := typesystem.UnwrapUnderlying(baseType)

	// Check Record Field
	if tRec, ok := resolvedType.(typesystem.TRecord); ok {
		if fieldType, ok := tRec.Fields[n.Member.Value]; ok {
			instType, _ := instantiateMemberType(ctx, n, fieldType, table)
			return instType, totalSubst, nil
		}
	}

	// Check Extension Method
	typeName := getCanonicalTypeName(baseType)
	if typeName != "" {
		if extMethodType, ok := table.GetExtensionMethod(typeName, n.Member.Value); ok {
			freshType := InstantiateWithContext(ctx, extMethodType)
			if tFunc, ok := freshType.(typesystem.TFunc); ok && len(tFunc.Params) > 0 {
				recvParam := tFunc.Params[0]
				subst, err := typesystem.Unify(recvParam, baseType)
				if err != nil {
					return nil, nil, inferErrorf(n, "extension method %s receiver mismatch: expected %s, got %s", n.Member.Value, recvParam, baseType)
				}
				totalSubst = subst.Compose(totalSubst)
				newParams := make([]typesystem.Type, len(tFunc.Params)-1)
				for i, p := range tFunc.Params[1:] {
					newParams[i] = p.Apply(subst)
				}
				return typesystem.TFunc{
					Params:     newParams,
					ReturnType: tFunc.ReturnType.Apply(subst),
					IsVariadic: tFunc.IsVariadic,
				}, totalSubst, nil
			}
		}
	}

	// Check TApp for extension methods
	if tApp, ok := baseType.(typesystem.TApp); ok {
		constructorName := getCanonicalTypeName(tApp.Constructor)
		if constructorName != "" {
			if extMethodType, ok := table.GetExtensionMethod(constructorName, n.Member.Value); ok {
				freshType := InstantiateWithContext(ctx, extMethodType)
				if tFunc, ok := freshType.(typesystem.TFunc); ok && len(tFunc.Params) > 0 {
					recvParam := tFunc.Params[0]
					subst, err := typesystem.Unify(recvParam, baseType)
					if err != nil {
						return nil, nil, inferErrorf(n, "extension method %s receiver mismatch: expected %s, got %s", n.Member.Value, recvParam, baseType)
					}
					totalSubst = subst.Compose(totalSubst)
					newParams := make([]typesystem.Type, len(tFunc.Params)-1)
					for i, p := range tFunc.Params[1:] {
						newParams[i] = p.Apply(subst)
					}
					return typesystem.TFunc{
						Params:     newParams,
						ReturnType: tFunc.ReturnType.Apply(subst),
						IsVariadic: tFunc.IsVariadic,
					}, totalSubst, nil
				}
			}
		}
	}

	return nil, nil, inferErrorf(n, "type %s has no field or method '%s'", baseType, n.Member.Value)
}

func instantiateMemberType(ctx *InferenceContext, n *ast.MemberExpression, memberType typesystem.Type, table *symbols.SymbolTable) (typesystem.Type, typesystem.Subst) {
	// Check context for Rank-N expectation
	// If the context expects a polytype (Forall), and the member is a polytype,
	// do NOT instantiate it. Pass it as is.
	if expectedType, ok := ctx.ExpectedTypes[n]; ok {
		// Resolve alias if any
		resolved := resolveTypeAlias(expectedType, table)
		if _, isExpectedForall := resolved.(typesystem.TForall); isExpectedForall {
			// Also check if the member itself is a Forall
			if _, isMemForall := memberType.(typesystem.TForall); isMemForall {
				return memberType, nil
			}
		}
	}
	return InstantiateGenericsWithSubst(ctx, memberType)
}

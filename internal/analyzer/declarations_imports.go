package analyzer

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
	"github.com/funvibe/funxy/internal/utils"
	"sort"
)

func (w *walker) VisitImportStatement(n *ast.ImportStatement) {
	if w.loader == nil {
		return
	}

	// Only process imports in ModeHeaders (or ModeFull)
	// We ALSO need to process in ModeBodies to:
	// 1. Ensure dependency bodies are analyzed (via analyzeRegularModule)
	// 2. Refresh imported symbol types (resolved from Pending to Actual)
	if w.mode == ModeNaming || w.mode == ModeInstances {
		return
	}

	// Resolve absolute path from ImportStatement
	importPath := n.Path.Value
	pathToCheck := utils.ResolveImportPath(w.BaseDir, importPath)

	// Check for duplicate import
	if w.importedModules[pathToCheck] {
		w.addError(diagnostics.NewError(
			diagnostics.ErrA004,
			n.Path.GetToken(),
			fmt.Sprintf("duplicate import: module '%s' already imported", n.Path.Value),
		))
		return
	}
	w.importedModules[pathToCheck] = true

	modInterface, err := w.loader.GetModule(pathToCheck)
	if err != nil {
		// Check if it's a DiagnosticError (syntax error in module)
		if compileErr, ok := err.(*diagnostics.DiagnosticError); ok {
			w.addError(compileErr)
			return
		}

		w.addError(diagnostics.NewError(
			diagnostics.ErrA001,
			n.Path.GetToken(),
			fmt.Sprintf("module not found: %s (%s)", n.Path.Value, err.Error()),
		))
		return
	}

	// Define Module Symbol
	name := ""
	if n.Alias != nil {
		// Explicit alias provided
		name = n.Alias.Value
	} else {
		// Use base name by default
		name = utils.ExtractModuleName(n.Path.Value)
	}

	if loadedMod, ok := modInterface.(LoadedModule); ok {
		// Handle package groups by analyzing all sub-packages
		if loadedMod.IsPackageGroupModule() {
			w.analyzePackageGroup(loadedMod, pathToCheck)
		} else {
			// Recursive Analysis based on Mode for regular modules
			w.analyzeRegularModule(loadedMod, pathToCheck)
		}

		// Store mapping alias -> packageName for extension method/trait lookup
		packageName := loadedMod.GetName()
		// If alias provided, register it
		if n.Alias != nil {
			w.symbolTable.RegisterModuleAlias(n.Alias.Value, packageName)
		} else {
			// Default name is also an alias
			w.symbolTable.RegisterModuleAlias(name, packageName)
		}

		// Get Exports (Symbols with Kinds)
		exportSymbols := loadedMod.GetExports()

		// Identify exported Types to tag TCon (sorted for deterministic order)
		exportedTypes := make(map[string]bool)
		exportKeys := make([]string, 0, len(exportSymbols))
		for k := range exportSymbols {
			exportKeys = append(exportKeys, k)
		}
		sort.Strings(exportKeys)

		for _, expName := range exportKeys {
			sym := exportSymbols[expName]
			if sym.Kind == symbols.TypeSymbol || sym.Kind == symbols.ConstructorSymbol {
				// Only tag TypeSymbol? ConstructorSymbol usually not TCon (TFunc or TApp)
				// But if we export Type, we want TCon{Name: Type} to be tagged.
				if sym.Kind == symbols.TypeSymbol {
					exportedTypes[expName] = true
				}
			}
		}

		// Handle selective imports
		if n.ImportAll || len(n.Symbols) > 0 || len(n.Exclude) > 0 {
			// Import symbols directly into current scope
			symbolsToImport := make(map[string]bool)

			// Track which traits need to be implicitly imported when their methods are imported
			traitsToImport := make(map[string]bool)

			if n.ImportAll {
				// Import all (using already sorted exportKeys)
				for _, expName := range exportKeys {
					symbolsToImport[expName] = true
				}
			} else if len(n.Symbols) > 0 {
				// Import only specified
				for _, sym := range n.Symbols {
					if _, ok := exportSymbols[sym.Value]; ok {
						symbolsToImport[sym.Value] = true

						// If this symbol is a trait method, automatically import the trait too
						if modSymTable := loadedMod.GetSymbolTable(); modSymTable != nil {
							if traitName, ok := modSymTable.GetTraitForMethod(sym.Value); ok {
								traitsToImport[traitName] = true
							}
						}
					} else {
						w.addError(diagnostics.NewError(
							diagnostics.ErrA006,
							sym.GetToken(),
							sym.Value,
						))
					}
				}

				// Add traits to import list
				for traitName := range traitsToImport {
					if !symbolsToImport[traitName] {
						symbolsToImport[traitName] = true
					}
				}
			} else if len(n.Exclude) > 0 {
				// Import all except specified (using already sorted exportKeys)
				excludeSet := make(map[string]bool)
				for _, sym := range n.Exclude {
					excludeSet[sym.Value] = true
				}
				for _, expName := range exportKeys {
					if !excludeSet[expName] {
						symbolsToImport[expName] = true
					}
				}
			}

			// Register each symbol directly (sorted for deterministic order)
			importKeys := make([]string, 0, len(symbolsToImport))
			for k := range symbolsToImport {
				importKeys = append(importKeys, k)
			}
			sort.Strings(importKeys)

			for _, symName := range importKeys {
				sym := exportSymbols[symName]
				// For selective imports, don't tag types with module name
				// Types are imported into local scope without qualification
				// taggedType := tagModule(sym.Type, packageName, exportedTypes)
				taggedType := sym.Type

				// Use OriginModule from source symbol, or packageName if not set
				origin := sym.OriginModule
				if origin == "" {
					origin = packageName
				}

				// Check for duplicate import - allow if same origin (same symbol re-exported via different paths)
				if existing, exists := w.symbolTable.Find(symName); exists {
					if existing.OriginModule == origin {
						// Same symbol from same origin.
						// If current mode is Bodies, we might want to update the type (from Pending to Actual).
						// If existing is Pending, definitely update.
						if existing.IsPending || w.mode == ModeBodies {
							// Proceed to overwrite
						} else {
							// Same symbol, already defined, no update needed
							continue
						}
					} else {
						// Different origins - conflict
						w.addError(diagnostics.NewError(
							diagnostics.ErrA004,
							n.GetToken(),
							fmt.Sprintf("%s' (already defined from %s, cannot import from %s)",
								symName, existing.OriginModule, origin),
						))
						continue
					}
				}

				if sym.Kind == symbols.TypeSymbol {
					// Check if it's a type alias with underlying type
					if sym.IsTypeAlias() {
						// Copy both nominal type and underlying type
						// Extract tagged underlying type from taggedType (if it's a TCon)
						taggedUnderlying := sym.UnderlyingType
						if tCon, ok := taggedType.(typesystem.TCon); ok && tCon.UnderlyingType != nil {
							taggedUnderlying = tCon.UnderlyingType
						}
						w.symbolTable.DefineTypeAlias(symName, taggedType, taggedUnderlying, origin)
					} else {
						w.symbolTable.DefineType(symName, taggedType, origin)
					}

					// Automatically import constructors for ADTs
					if loadedModSymTable := loadedMod.GetSymbolTable(); loadedModSymTable != nil {
						if variants, ok := loadedModSymTable.GetVariants(symName); ok {
							for _, variantName := range variants {
								// Only import if the variant is actually exported by the module
								if variantSym, ok := exportSymbols[variantName]; ok {
									// Check for conflict/redefinition logic similar to main loop?
									// Since it's implicit, maybe we should be softer or just overwrite?
									// Main loop checks "existing.OriginModule == origin".
									// Let's do the same check.
									if existing, exists := w.symbolTable.Find(variantName); exists {
										if existing.OriginModule != origin {
											// Conflict - but since this is implicit, maybe we just skip importing the constructor?
											// Or should we error?
											// If I import "Shape", I expect "Circle". If "Circle" is taken, that's an error.
											w.addError(diagnostics.NewError(
												diagnostics.ErrA004,
												n.GetToken(),
												fmt.Sprintf("implicit import of '%s' (constructor of %s) conflicts with existing symbol from %s",
													variantName, symName, existing.OriginModule),
											))
										}
										continue
									}

									variantTaggedType := tagModule(variantSym.Type, packageName, exportedTypes)
									w.symbolTable.DefineConstructor(variantName, variantTaggedType, origin)
								}
							}
						}
					}
				} else if sym.Kind == symbols.ConstructorSymbol {
					w.symbolTable.DefineConstructor(symName, taggedType, origin)
				} else if sym.Kind == symbols.TraitSymbol {
					// Import Trait definition
					var typeParams, superTraits []string
					if modSymTable := loadedMod.GetSymbolTable(); modSymTable != nil {
						typeParams, _ = modSymTable.GetTraitTypeParams(symName)
						superTraits, _ = modSymTable.GetTraitSuperTraits(symName)
					}
					w.symbolTable.DefineTrait(symName, typeParams, superTraits, origin)

					// Import trait parameter kinds
					if modSymTable := loadedMod.GetSymbolTable(); modSymTable != nil {
						for _, param := range typeParams {
							if kind, ok := modSymTable.GetTraitTypeParamKind(symName, param); ok {
								w.symbolTable.RegisterTraitTypeParamKind(symName, param, kind)
							}
						}
					}

					// Import trait methods linkage
					if modSymTable := loadedMod.GetSymbolTable(); modSymTable != nil {
						methods := modSymTable.GetTraitAllMethods(symName)
						for _, methodName := range methods {
							w.symbolTable.RegisterTraitMethod2(symName, methodName)

							// Register method linkage (methodName -> traitName)
							if methodSym, ok := modSymTable.Find(methodName); ok {
								// Tag method type with module for consistency
								taggedMethodType := tagModule(methodSym.Type, packageName, exportedTypes)
								w.symbolTable.RegisterTraitMethod(methodName, symName, taggedMethodType, origin)
							}
						}
					}
				} else {
					w.symbolTable.Define(symName, taggedType, origin)
				}
			}

			// Copy trait implementations and extension methods for imported types
			if loadedMod, ok := modInterface.(LoadedModule); ok {
				if modSymTable := loadedMod.GetSymbolTable(); modSymTable != nil {
					importedTypes := make(map[string]bool)
					for _, symName := range importKeys {
						if exportedTypes[symName] {
							importedTypes[symName] = true
						}
					}
					w.importTraitImplementations(modSymTable, importedTypes, "")
					w.importExtensionMethods(modSymTable, importedTypes)
				}
			}
		} else {
			// Default: Create TRecord for the module (using already sorted exportKeys)
			fields := make(map[string]typesystem.Type)
			for _, expName := range exportKeys {
				sym := exportSymbols[expName]

				// For traits, create a special marker type since they have Type=nil
				if sym.Kind == symbols.TraitSymbol {
					// Use a TCon to represent the trait in the module record
					fields[expName] = typesystem.TCon{Name: expName}
					continue
				}

				if sym.Type == nil {
					// Skip symbols with nil types (other than traits)
					continue
				}
				// Tag with alias for namespaced access (e.g., m.Vector)
				taggedType := tagModule(sym.Type, name, exportedTypes)
				if taggedType == nil {
					continue
				}
				fields[expName] = taggedType
			}

			moduleType := typesystem.TRecord{Fields: fields}
			w.symbolTable.DefineModule(name, moduleType)

			// Copy trait definitions for exported traits (qualified access like m.Trait)
			if modSymTable := loadedMod.GetSymbolTable(); modSymTable != nil {
				for _, expName := range exportKeys {
					sym := exportSymbols[expName]
					if sym.Kind == symbols.TraitSymbol {
						// Copy trait definition with qualified name
						typeParams, _ := modSymTable.GetTraitTypeParams(expName)
						superTraits, _ := modSymTable.GetTraitSuperTraits(expName)
						qualifiedTraitName := name + "." + expName

						// Define the trait with qualified name
						w.symbolTable.DefineTrait(qualifiedTraitName, typeParams, superTraits, sym.OriginModule)

						// Copy trait parameter kinds for qualified name
						for _, param := range typeParams {
							if kind, ok := modSymTable.GetTraitTypeParamKind(expName, param); ok {
								w.symbolTable.RegisterTraitTypeParamKind(qualifiedTraitName, param, kind)
							}
						}

						// Copy trait methods
						// Trait methods need to be available as functions when trait is imported
						methods := modSymTable.GetTraitAllMethods(expName)
						for _, methodName := range methods {
							w.symbolTable.RegisterTraitMethod2(qualifiedTraitName, methodName)

							// Try to get method type from module's symbol table
							var methodType typesystem.Type
							if methodSym, ok := modSymTable.Find(methodName); ok && methodSym.Type != nil {
								methodType = methodSym.Type
							} else {
								// Method type not found in module symbol table
								// This can happen if module wasn't fully analyzed
								// Create a generic placeholder type that will be resolved via trait dispatch
								methodType = typesystem.TVar{Name: "method_" + methodName}
							}

							taggedMethodType := tagModule(methodType, name, exportedTypes)
							w.symbolTable.RegisterTraitMethod(methodName, qualifiedTraitName, taggedMethodType, sym.OriginModule)
						}
					}
				}
			}

			// Copy type aliases for exported types (qualified names like m.Vector)
			if modSymTable := loadedMod.GetSymbolTable(); modSymTable != nil {
				for typeName := range exportedTypes {
					if aliasType, ok := modSymTable.GetTypeAlias(typeName); ok {
						qualifiedName := name + "." + typeName
						// Register with qualified name (e.g., "m.Vector" -> TRecord{...})
						w.symbolTable.RegisterTypeAlias(qualifiedName, aliasType)
					}
				}
			}

			// Copy trait implementations for exported types
			// This is still needed for types that don't have Module field (like record aliases)
			// For types with Module field, we also look up in source module via isImplementationInSourceModule
			if modSymTable := loadedMod.GetSymbolTable(); modSymTable != nil {
				w.importTraitImplementations(modSymTable, exportedTypes, name)
			}
		}
	} else {
		// Fallback if casting fails (shouldn't happen if wired correctly)
		w.symbolTable.Define(name, typesystem.TCon{Name: "Module"}, name)
	}
}

// analyzePackageGroup analyzes all sub-packages in a package group
func (w *walker) analyzePackageGroup(loadedMod LoadedModule, pathToCheck string) {
	// Analyze each sub-package
	for _, subModRaw := range loadedMod.GetSubModulesRaw() {
		subMod, ok := subModRaw.(LoadedModule)
		if !ok {
			continue
		}
		w.analyzeRegularModule(subMod, pathToCheck)
	}

	// Mark the package group as analyzed
	loadedMod.SetHeadersAnalyzed(true)
	loadedMod.SetBodiesAnalyzed(true)
}

// resolveReexports resolves re-export specifications and adds symbols to module exports.
// Called after module analysis when all imports have been processed.
// Returns errors if re-export references a module that wasn't imported or
// if trying to re-export symbols that aren't exported by the source module.
func resolveReexports(mod LoadedModule, loader ModuleLoader) []*diagnostics.DiagnosticError {
	var errors []*diagnostics.DiagnosticError

	specs := mod.GetReexportSpecs()
	if len(specs) == 0 {
		return errors
	}

	symbolTable := mod.GetSymbolTable()

	for _, spec := range specs {
		if spec.ModuleName == nil {
			continue
		}

		moduleName := spec.ModuleName.Value

		// Validate that the module was actually imported
		// Check if moduleName is a registered module alias
		packageName, isImported := symbolTable.GetPackageNameByAlias(moduleName)
		if !isImported {
			errors = append(errors, diagnostics.NewError(
				diagnostics.ErrA006,
				spec.GetToken(),
				fmt.Sprintf("cannot re-export from '%s': module not imported", moduleName),
			))
			continue
		}

		// Get source module exports for validation
		var sourceExports map[string]bool
		if loader != nil {
			if sourceMod := loader.GetModuleByPackageName(packageName); sourceMod != nil {
				if lm, ok := sourceMod.(LoadedModule); ok {
					sourceExports = make(map[string]bool)
					for name := range lm.GetExports() {
						sourceExports[name] = true
					}
				}
			}
		}

		if spec.ReexportAll {
			// shapes(*) — re-export all symbols with OriginModule == moduleName
			// We iterate through all symbols and find those that came from this module
			for name, sym := range symbolTable.All() {
				if sym.OriginModule == moduleName || sym.OriginModule == packageName {
					mod.AddExport(name)
				}
			}
		} else {
			// shapes(foo, bar) — re-export specific symbols
			for _, symIdent := range spec.Symbols {
				name := symIdent.Value

				// Validate that the symbol is exported by the source module
				if sourceExports != nil {
					if !sourceExports[name] {
						errors = append(errors, diagnostics.NewError(
							diagnostics.ErrA006,
							symIdent.GetToken(),
							fmt.Sprintf("cannot re-export '%s' from '%s': symbol not exported by source module", name, moduleName),
						))
						continue
					}
				}

				// Verify symbol exists in current symbol table (was successfully imported)
				if _, ok := symbolTable.Find(name); ok {
					mod.AddExport(name)
				} else {
					// Symbol not found in current scope - maybe wasn't imported
					errors = append(errors, diagnostics.NewError(
						diagnostics.ErrA006,
						symIdent.GetToken(),
						fmt.Sprintf("cannot re-export '%s': symbol not found in current scope", name),
					))
				}
			}
		}
	}

	return errors
}

// preRegisterModuleNames removed

// analyzeRegularModule handles analysis for a regular (non-package-group) module
func (w *walker) analyzeRegularModule(loadedMod LoadedModule, pathToCheck string) {
	if w.mode == ModeHeaders {
		if !loadedMod.IsHeadersAnalyzed() {
			if loadedMod.IsHeadersAnalyzing() {
				// Circular dependency in headers - skip
				return
			}
			loadedMod.SetHeadersAnalyzing(true)

			modAnalyzer := New(loadedMod.GetSymbolTable())
			modAnalyzer.SetLoader(w.loader)
			// Share inference context to ensure unique TVar names across modules
			modAnalyzer.SetInferenceContext(w.inferCtx)
			modAnalyzer.RegisterBuiltins()
			modAnalyzer.BaseDir = utils.GetModuleDir(pathToCheck)

			// Pass 1: Naming (Discovery) - Register all names as Pending
			// This replaces preRegisterModuleNames
			for _, file := range loadedMod.GetFiles() {
				// Use AnalyzeNaming (ModeNaming)
				errs := modAnalyzer.AnalyzeNaming(file)
				// Naming errors (e.g. invalid names) should be reported
				w.addErrors(errs)
			}

			// Pass 2: Declarations (Headers) - Resolve Types and Signatures
			for _, file := range loadedMod.GetFiles() {
				errs := modAnalyzer.AnalyzeHeaders(file)
				w.addErrors(errs)
			}

			// Pass 3: Instances - Check trait implementations
			// This is done after all headers (types/traits/signatures) are known
			for _, file := range loadedMod.GetFiles() {
				errs := modAnalyzer.AnalyzeInstances(file)
				w.addErrors(errs)
			}

			loadedMod.SetTypeMap(modAnalyzer.TypeMap)
			loadedMod.SetTraitDefaults(modAnalyzer.TraitDefaults)
			loadedMod.SetHeadersAnalyzing(false)
			loadedMod.SetHeadersAnalyzed(true)

			// Resolve re-exports after headers analysis (imports are processed)
			reexportErrs := resolveReexports(loadedMod, w.loader)
			w.addErrors(reexportErrs)
		}
	} else if w.mode == ModeBodies {
		if !loadedMod.IsBodiesAnalyzed() {
			if loadedMod.IsBodiesAnalyzing() {
				// Cycle in bodies is allowed - skip
				return
			}
			loadedMod.SetBodiesAnalyzing(true)

			modAnalyzer := New(loadedMod.GetSymbolTable())
			modAnalyzer.SetLoader(w.loader)
			// Share inference context to ensure unique TVar names across modules
			modAnalyzer.SetInferenceContext(w.inferCtx)
			modAnalyzer.RegisterBuiltins()
			modAnalyzer.BaseDir = utils.GetModuleDir(pathToCheck)

			for _, file := range loadedMod.GetFiles() {
				errs := modAnalyzer.AnalyzeBodies(file)
				w.addErrors(errs)
			}

			loadedMod.SetTypeMap(modAnalyzer.TypeMap)
			// loadedMod.SetTraitDefaults(modAnalyzer.TraitDefaults)
			loadedMod.SetBodiesAnalyzing(false)
			loadedMod.SetBodiesAnalyzed(true)
		}
	} else {
		// Legacy / Full Mode
		if !loadedMod.IsBodiesAnalyzed() {
			if loadedMod.IsBodiesAnalyzing() {
				return
			}
			loadedMod.SetBodiesAnalyzing(true)

			modAnalyzer := New(loadedMod.GetSymbolTable())
			modAnalyzer.SetLoader(w.loader)
			// Share inference context to ensure unique TVar names across modules
			modAnalyzer.SetInferenceContext(w.inferCtx)
			modAnalyzer.RegisterBuiltins()
			modAnalyzer.BaseDir = utils.GetModuleDir(pathToCheck)

			for _, file := range loadedMod.GetFiles() {
				errs := modAnalyzer.Analyze(file)
				w.addErrors(errs)
			}

			loadedMod.SetTypeMap(modAnalyzer.TypeMap)
			loadedMod.SetTraitDefaults(modAnalyzer.TraitDefaults)
			loadedMod.SetBodiesAnalyzing(false)
			loadedMod.SetBodiesAnalyzed(true)

			// Resolve re-exports after full analysis
			reexportErrs := resolveReexports(loadedMod, w.loader)
			w.addErrors(reexportErrs)
		}
	}
}

package vm

import (
	"fmt"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
	"path/filepath"
)

// ProcessImports processes all pending imports
func (vm *VM) ProcessImports(imports []PendingImport) error {
	for _, imp := range imports {
		if err := vm.processOneImport(imp); err != nil {
			return err
		}
	}
	return nil
}

// processOneImport handles a single import
func (vm *VM) processOneImport(imp PendingImport) error {
	// Check if it's a virtual module (lib/*)
	if isVirtualModule(imp.Path) {
		return vm.importVirtualModule(imp)
	}

	// User module - need loader
	if vm.loader == nil {
		return fmt.Errorf("cannot load user module %s: no module loader configured", imp.Path)
	}

	return vm.importUserModule(imp)
}

// importUserModule loads, compiles, and executes a user-defined module
func (vm *VM) importUserModule(imp PendingImport) error {
	// Resolve import path
	importPath := imp.Path
	if len(importPath) > 0 && importPath[0] == '.' {
		importPath = filepath.Join(vm.baseDir, importPath)
	}

	// Normalize to absolute path for caching
	absPath, err := filepath.Abs(importPath)
	if err != nil {
		return fmt.Errorf("failed to resolve module path %s: %v", imp.Path, err)
	}

	// Check cache first
	if cachedObj := vm.moduleCache.Get(absPath); cachedObj != nil {
		if cached, ok := cachedObj.(*evaluator.RecordInstance); ok {
			return vm.applyModuleImport(imp, cached)
		}
	}

	// Check if module is being loaded (cyclic import detection)
	if vm.loadingModules == nil {
		vm.loadingModules = make(map[string]bool)
	}
	if vm.loadingModules[absPath] {
		// Cyclic import - create placeholder and continue
		// The actual module will be populated when it finishes loading
		placeholder := evaluator.NewRecord(nil)
		placeholder.TypeName = imp.Path
		vm.moduleCache = vm.moduleCache.Put(absPath, placeholder)
		return vm.applyModuleImport(imp, placeholder)
	}
	vm.loadingModules[absPath] = true

	// Load module through loader
	modInterface, err := vm.loader.GetModule(absPath)
	if err != nil {
		delete(vm.loadingModules, absPath)
		return fmt.Errorf("failed to load module %s: %v", imp.Path, err)
	}

	mod, ok := modInterface.(*modules.Module)
	if !ok {
		delete(vm.loadingModules, absPath)
		return fmt.Errorf("invalid module type for %s", imp.Path)
	}

	// Compile and execute the module
	modObj, err := vm.compileAndExecuteModule(mod)
	if err != nil {
		delete(vm.loadingModules, absPath)
		return fmt.Errorf("failed to execute module %s: %v", imp.Path, err)
	}

	// Update cache with actual module (in case placeholder was used)
	if placeholderObj := vm.moduleCache.Get(absPath); placeholderObj != nil {
		if placeholder, ok := placeholderObj.(*evaluator.RecordInstance); ok {
			// Copy fields from actual module to placeholder for cyclic refs
			placeholder.Fields = make([]evaluator.RecordField, len(modObj.Fields))
			copy(placeholder.Fields, modObj.Fields)
		}
	}
	vm.moduleCache = vm.moduleCache.Put(absPath, modObj)
	delete(vm.loadingModules, absPath)

	return vm.applyModuleImport(imp, modObj)
}

// compileAndExecuteModule compiles a module's files and executes them
func (vm *VM) compileAndExecuteModule(mod *modules.Module) (*evaluator.RecordInstance, error) {
	// Handle package groups specially
	if mod.IsPackageGroup {
		exports := make(map[string]evaluator.Object)

		// Load and execute each sub-package
		for subName, subMod := range mod.Imports {
			// Compile and execute sub-module
			subObj, err := vm.compileAndExecuteModule(subMod)
			if err != nil {
				return nil, fmt.Errorf("failed to execute sub-package %s: %v", subName, err)
			}

			// Add all exports from sub-module to combined exports
			for _, field := range subObj.Fields {
				exports[field.Key] = field.Value
			}
		}

		return evaluator.NewRecord(exports), nil
	}

	// Regular module compilation
	compiler := NewCompiler()
	compiler.SetBaseDir(mod.Dir)

	for _, file := range mod.Files {
		if err := compiler.compileProgram(file); err != nil {
			return nil, fmt.Errorf("compilation error in %s: %v", mod.Name, err)
		}
	}

	compiler.emit(OP_HALT, 0)
	chunk := compiler.currentChunk()

	modVM := New()
	modVM.loader = vm.loader
	modVM.baseDir = mod.Dir
	// Share module cache and loading state for cyclic import detection
	modVM.moduleCache = vm.moduleCache
	modVM.loadingModules = vm.loadingModules
	modVM.RegisterBuiltins()

	// Initialize trait defaults from analysis results
	if mod.TraitDefaults != nil {
		modVM.traitDefaults = mod.TraitDefaults
	}



	pendingImports := compiler.GetPendingImports()
	if err := modVM.ProcessImports(pendingImports); err != nil {
		return nil, fmt.Errorf("import error in module %s: %v", mod.Name, err)
	}

	_, err := modVM.Run(chunk)
	if err != nil {
		return nil, fmt.Errorf("runtime error in %s: %v", mod.Name, err)
	}

	exports := make(map[string]evaluator.Object)
	for name := range mod.Exports {
		if val := modVM.globals.Get(name); val != nil {
			exports[name] = val
		}
	}

	// Attach globals to exported closures so they can access module scope
	for _, val := range exports {
		if closure, ok := val.(*ObjClosure); ok {
			closure.Globals = modVM.globals
		}
	}

	// Copy trait implementations from module VM to parent VM
	modVM.traitMethods.Range(func(traitName string, typeMapObj evaluator.Object) bool {
		modTypeMap := typeMapObj.(*PersistentMap)

		// Get or create parent trait map
		var parentTypeMap *PersistentMap
		if val := vm.traitMethods.Get(traitName); val != nil {
			parentTypeMap = val.(*PersistentMap)
		} else {
			parentTypeMap = EmptyMap()
		}

		modTypeMap.Range(func(typeName string, methodMapObj evaluator.Object) bool {
			modMethodMap := methodMapObj.(*PersistentMap)

			// Get or create parent type map
			var parentMethodMap *PersistentMap
			if val := parentTypeMap.Get(typeName); val != nil {
				parentMethodMap = val.(*PersistentMap)
			} else {
				parentMethodMap = EmptyMap()
			}

			modMethodMap.Range(func(methodName string, closureObj evaluator.Object) bool {
				closure := closureObj.(*ObjClosure)
				// Attach module globals to trait methods
				// ObjClosure.Globals is *PersistentMap, modVM.globals is *PersistentMap
				closure.Globals = modVM.globals

				parentMethodMap = parentMethodMap.Put(methodName, closure)
				return true
			})
			parentTypeMap = parentTypeMap.Put(typeName, parentMethodMap)
			return true
		})
		vm.traitMethods = vm.traitMethods.Put(traitName, parentTypeMap)
		return true
	})

	// Copy extension methods from module VM to parent VM
	modVM.extensionMethods.Range(func(typeName string, methodMapObj evaluator.Object) bool {
		modMethodMap := methodMapObj.(*PersistentMap)

		// Get or create parent extension map
		var parentMethodMap *PersistentMap
		if val := vm.extensionMethods.Get(typeName); val != nil {
			parentMethodMap = val.(*PersistentMap)
		} else {
			parentMethodMap = EmptyMap()
		}

		modMethodMap.Range(func(methodName string, closureObj evaluator.Object) bool {
			closure := closureObj.(*ObjClosure)
			closure.Globals = modVM.globals
			parentMethodMap = parentMethodMap.Put(methodName, closure)
			return true
		})
		vm.extensionMethods = vm.extensionMethods.Put(typeName, parentMethodMap)
		return true
	})

	// Copy trait defaults from module VM to parent VM
	for key, fn := range modVM.traitDefaults {
		vm.traitDefaults[key] = fn
	}

	return evaluator.NewRecord(exports), nil
}

// Helper to check if a value is a constructor for a type
func isConstructorForType(val evaluator.Object, typeName string) bool {
	switch c := val.(type) {
	case *evaluator.Constructor:
		return c.TypeName == typeName
	case *evaluator.DataInstance:
		return c.TypeName == typeName
	}
	return false
}

// applyModuleImport applies the import specification to globals
func (vm *VM) applyModuleImport(imp PendingImport, modObj *evaluator.RecordInstance) error {
	// Try to get the module for trait checking
	var mod *modules.Module
	if vm.loader != nil {
		importPath := imp.Path
		if len(importPath) > 0 && importPath[0] == '.' {
			importPath = filepath.Join(vm.baseDir, importPath)
		}
		absPath, err := filepath.Abs(importPath)
		if err == nil {
			if modInterface, err := vm.loader.GetModule(absPath); err == nil {
				if m, ok := modInterface.(*modules.Module); ok {
					mod = m
				}
			}
		}
	}

	if imp.Alias != "" {
		vm.globals = vm.globals.Put(imp.Alias, modObj)
	} else if imp.ImportAll {
		// Create a set of excluded symbols for efficient lookup
		excluded := make(map[string]bool)
		for _, sym := range imp.ExcludeSymbols {
			excluded[sym] = true
		}

		// Import all symbols except excluded ones
		for _, field := range modObj.Fields {
			if !excluded[field.Key] {
				vm.globals = vm.globals.Put(field.Key, field.Value)
			}
		}
	} else if len(imp.ExcludeSymbols) > 0 {
		// import "path" !(a, b, c) - import all except specified
		excluded := make(map[string]bool)
		for _, sym := range imp.ExcludeSymbols {
			excluded[sym] = true
		}

		for _, field := range modObj.Fields {
			if !excluded[field.Key] {
				vm.globals = vm.globals.Put(field.Key, field.Value)
			}
		}
	} else if len(imp.Symbols) > 0 {
		for _, sym := range imp.Symbols {
			if val := modObj.Get(sym); val != nil {
				vm.globals = vm.globals.Put(sym, val)

				// Implicit import of constructors for ADTs
				if typeObj, ok := val.(*evaluator.TypeObject); ok {
					typeName := ""
					if tCon, ok := typeObj.TypeVal.(typesystem.TCon); ok {
						typeName = tCon.Name
					} else if tApp, ok := typeObj.TypeVal.(typesystem.TApp); ok {
						if tCon, ok := tApp.Constructor.(typesystem.TCon); ok {
							typeName = tCon.Name
						}
					}

					if typeName != "" {
						// Scan module exports for constructors of this type
						for _, field := range modObj.Fields {
							if isConstructorForType(field.Value, typeName) {
								vm.globals = vm.globals.Put(field.Key, field.Value)
							}
						}
					}
				}
			} else {
				// Check if it's a trait - traits don't have runtime values but should be importable
				if mod != nil && mod.SymbolTable != nil {
					if symbolVal, ok := mod.SymbolTable.Find(sym); ok && symbolVal.Kind == symbols.TraitSymbol {
						// Trait found - now import all its methods
						traitMethodNames := mod.SymbolTable.GetTraitAllMethods(sym)
						for _, methodName := range traitMethodNames {
							// Try to get the method from modObj
							if methodVal := modObj.Get(methodName); methodVal != nil {
								vm.globals = vm.globals.Put(methodName, methodVal)
							}
						}
						continue
					}
				}
				return fmt.Errorf("symbol '%s' not found in module", sym)
			}
		}
	} else {
		modName := filepath.Base(imp.Path)
		if ext := filepath.Ext(modName); ext != "" {
			modName = modName[:len(modName)-len(ext)]
		}
		vm.globals = vm.globals.Put(modName, modObj)
	}
	return nil
}

// isVirtualModule checks if path refers to a built-in virtual module
func isVirtualModule(path string) bool {
	return path == "lib" ||
		len(path) > 4 && path[:4] == "lib/" ||
		isKnownVirtualPackage(path)
}

// isKnownVirtualPackage checks if name is a known virtual package
func isKnownVirtualPackage(name string) bool {
	for _, pkg := range modules.GetLibSubPackages() {
		if name == pkg {
			return true
		}
	}
	return false
}

// importVirtualModule imports a built-in virtual module
func (vm *VM) importVirtualModule(imp PendingImport) error {
	pkgName := imp.Path
	if len(pkgName) > 4 && pkgName[:4] == "lib/" {
		pkgName = pkgName[4:]
	}

	if imp.Path == "lib" {
		return vm.importAllLibPackages(imp)
	}

	builtins := evaluator.GetVirtualModuleBuiltins(pkgName)
	if builtins == nil {
		return fmt.Errorf("unknown virtual module: %s", pkgName)
	}

	if imp.Alias != "" {
		fields := make(map[string]evaluator.Object)
		for name, fn := range builtins {
			fields[name] = fn
		}
		vm.globals = vm.globals.Put(imp.Alias, evaluator.NewRecord(fields))
	} else if imp.ImportAll {
		// Create a set of excluded symbols for efficient lookup
		excluded := make(map[string]bool)
		for _, sym := range imp.ExcludeSymbols {
			excluded[sym] = true
		}

		// Import all symbols except excluded ones
		for name, fn := range builtins {
			if !excluded[name] {
				vm.globals = vm.globals.Put(name, fn)
			}
		}
	} else if len(imp.ExcludeSymbols) > 0 {
		// import "lib/module" !(a, b, c) - import all except specified
		excluded := make(map[string]bool)
		for _, sym := range imp.ExcludeSymbols {
			excluded[sym] = true
		}

		for name, fn := range builtins {
			if !excluded[name] {
				vm.globals = vm.globals.Put(name, fn)
			}
		}
	} else if len(imp.Symbols) > 0 {
		for _, sym := range imp.Symbols {
			if fn, ok := builtins[sym]; ok {
				vm.globals = vm.globals.Put(sym, fn)

				// Auto-import ADT constructors if present
				if pkg := modules.GetVirtualPackage("lib/" + pkgName); pkg != nil {
					if variants, ok := pkg.Variants[sym]; ok {
						for _, variantName := range variants {
							if variantFn, exists := builtins[variantName]; exists {
								vm.globals = vm.globals.Put(variantName, variantFn)
							}
						}
					}
				}
			} else {
				return fmt.Errorf("symbol '%s' not found in module %s", sym, pkgName)
			}
		}
	} else {
		fields := make(map[string]evaluator.Object)
		for name, fn := range builtins {
			fields[name] = fn
		}
		vm.globals = vm.globals.Put(pkgName, evaluator.NewRecord(fields))
	}

	return nil
}

// importAllLibPackages imports all lib/* packages
func (vm *VM) importAllLibPackages(imp PendingImport) error {
	packages := modules.GetLibSubPackages()

	if imp.ImportAll {
		// Create a set of excluded symbols for efficient lookup
		excluded := make(map[string]bool)
		for _, sym := range imp.ExcludeSymbols {
			excluded[sym] = true
		}

		for _, pkg := range packages {
			builtins := evaluator.GetVirtualModuleBuiltins(pkg)
			for name, fn := range builtins {
				if !excluded[name] {
					vm.globals = vm.globals.Put(name, fn)
				}
			}
		}
	} else if len(imp.ExcludeSymbols) > 0 {
		// import "lib" !(a, b, c) - import all except specified
		excluded := make(map[string]bool)
		for _, sym := range imp.ExcludeSymbols {
			excluded[sym] = true
		}

		for _, pkg := range packages {
			builtins := evaluator.GetVirtualModuleBuiltins(pkg)
			for name, fn := range builtins {
				if !excluded[name] {
					vm.globals = vm.globals.Put(name, fn)
				}
			}
		}
	} else if imp.Alias != "" {
		libFields := make(map[string]evaluator.Object)
		for _, pkg := range packages {
			builtins := evaluator.GetVirtualModuleBuiltins(pkg)
			if builtins != nil {
				pkgFields := make(map[string]evaluator.Object)
				for name, fn := range builtins {
					pkgFields[name] = fn
				}
				libFields[pkg] = evaluator.NewRecord(pkgFields)
			}
		}
		vm.globals = vm.globals.Put(imp.Alias, evaluator.NewRecord(libFields))
	} else {
		libFields := make(map[string]evaluator.Object)
		for _, pkg := range packages {
			builtins := evaluator.GetVirtualModuleBuiltins(pkg)
			if builtins != nil {
				pkgFields := make(map[string]evaluator.Object)
				for name, fn := range builtins {
					pkgFields[name] = fn
				}
				libFields[pkg] = evaluator.NewRecord(pkgFields)
			}
		}
		vm.globals = vm.globals.Put("lib", evaluator.NewRecord(libFields))
	}

	return nil
}

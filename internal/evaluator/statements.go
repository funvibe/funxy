package evaluator

import (
	"fmt"
	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
	"github.com/funvibe/funxy/internal/utils"
	"strings"
)

func (e *Evaluator) evalProgram(program *ast.Program, env *Environment) Object {
	var result Object
	for _, stmt := range program.Statements {
		result = e.Eval(stmt, env)
		switch result := result.(type) {
		case *Error:
			return result
		}
	}
	return result
}

func (e *Evaluator) evalImportStatement(node *ast.ImportStatement, env *Environment) Object {
	if e.Loader == nil {
		return &Nil{}
	}

	importPath := node.Path.Value
	pathToCheck := utils.ResolveImportPath(e.BaseDir, importPath)

	modInterface, err := e.Loader.GetModule(pathToCheck)
	if err != nil {
		return newError("failed to load module: %s (%s)", node.Path.Value, err.Error())
	}

	if mod, ok := modInterface.(*modules.Module); ok {
		// Handle virtual modules (lib/list, etc.)
		if mod.IsVirtual {
			return e.importVirtualModule(node, mod, env)
		}

		// Handle package groups (import "dir" imports all sub-packages)
		if mod.IsPackageGroup {
			return e.importPackageGroup(node, mod, env)
		}

		modObj, err := e.EvaluateModule(mod)
		if err != nil {
			return newError("failed to evaluate module %s: %s", mod.Name, err.Error())
		}

		// Import trait implementations from the module
		for traitName, typeImpls := range mod.ClassImplementations {
			if e.ClassImplementations[traitName] == nil {
				e.ClassImplementations[traitName] = make(map[string]Object)
			}
			for typeName, impl := range typeImpls {
				if obj, ok := impl.(Object); ok {
					e.ClassImplementations[traitName][typeName] = obj
				}
			}
		}

		record, ok := modObj.(*RecordInstance)
		if !ok {
			return newError("module %s is not a record", mod.Name)
		}

		// Handle import specifications
		if node.ImportAll {
			// import "path" (*) - import all symbols into current scope
			for _, f := range record.Fields {
				env.Set(f.Key, f.Value)
			}
			return &Nil{}
		}

		if len(node.Symbols) > 0 {
			// import "path" (a, b, c) - import only specified symbols
			for _, sym := range node.Symbols {
				if val := record.Get(sym.Value); val != nil {
					env.Set(sym.Value, val)

					// Auto-import ADT constructors if present
					if mod.SymbolTable != nil {
						if variants, ok := mod.SymbolTable.GetVariants(sym.Value); ok {
							for _, variantName := range variants {
								// Only import if the variant is actually exported by the module
								if variantVal := record.Get(variantName); variantVal != nil {
									env.Set(variantName, variantVal)
								}
							}
						}
					}
				} else {
					// Check if it's a trait - traits don't have runtime values but should be importable
					if mod.SymbolTable != nil {
						if symVal, ok := mod.SymbolTable.Find(sym.Value); ok && symVal.Kind == symbols.TraitSymbol {
							// Trait found - it's OK to import even though there's no runtime value
							continue
						}
					}
					// Extension methods might not be in the record yet but could be in exports
					return newError("symbol '%s' not found in module %s", sym.Value, mod.Name)
				}
			}
			return &Nil{}
		}

		if len(node.Exclude) > 0 {
			// import "path" !(a, b, c) - import all except specified
			excludeSet := make(map[string]bool)
			for _, sym := range node.Exclude {
				excludeSet[sym.Value] = true
			}
			for _, f := range record.Fields {
				if !excludeSet[f.Key] {
					env.Set(f.Key, f.Value)
				}
			}
			return &Nil{}
		}

		// Default: import as module object with alias
		name := ""
		if node.Alias != nil {
			name = node.Alias.Value
		} else {
			name = utils.ExtractModuleName(node.Path.Value)
		}

		env.Set(name, modObj)
		return &Nil{}
	}

	return newError("invalid module type loaded")
}

// importVirtualModule handles importing built-in virtual modules
func (e *Evaluator) importVirtualModule(node *ast.ImportStatement, mod *modules.Module, env *Environment) Object {
	// Special case: import "lib" imports all lib/* packages
	if mod.Name == "lib" {
		return e.importAllLibPackages(node, env)
	}

	// Get builtins for this virtual module
	builtins := e.getVirtualModuleBuiltins(mod.Name)
	if builtins == nil {
		return newError("unknown virtual module: %s", mod.Name)
	}

	// Handle import specifications
	if node.ImportAll {
		for name, fn := range builtins {
			env.Set(name, fn)
		}
		return &Nil{}
	}

	if len(node.Symbols) > 0 {
		for _, sym := range node.Symbols {
			if fn, ok := builtins[sym.Value]; ok {
				env.Set(sym.Value, fn)

				// Auto-import ADT constructors if present
				// For virtual modules, we need to check the VirtualPackage definition
				if pkg := modules.GetVirtualPackage("lib/" + mod.Name); pkg != nil {
					if variants, ok := pkg.Variants[sym.Value]; ok {
						for _, variantName := range variants {
							if variantFn, exists := builtins[variantName]; exists {
								env.Set(variantName, variantFn)
							}
						}
					}
				}
			} else {
				return newError("symbol '%s' not found in module %s", sym.Value, mod.Name)
			}
		}
		return &Nil{}
	}

	if len(node.Exclude) > 0 {
		excludeSet := make(map[string]bool)
		for _, sym := range node.Exclude {
			excludeSet[sym.Value] = true
		}
		for name, fn := range builtins {
			if !excludeSet[name] {
				env.Set(name, fn)
			}
		}
		return &Nil{}
	}

	// Default: import as module object
	fields := make(map[string]Object)
	for name, fn := range builtins {
		fields[name] = fn
	}
	modObj := NewRecord(fields)

	name := ""
	if node.Alias != nil {
		name = node.Alias.Value
	} else {
		name = mod.Name
	}

	env.Set(name, modObj)
	return &Nil{}
}

// getVirtualModuleBuiltins returns builtins for a virtual module by name
func (e *Evaluator) getVirtualModuleBuiltins(name string) map[string]Object {
	return GetVirtualModuleBuiltins(name)
}

// GetVirtualModuleBuiltins returns builtins for a virtual module by name (exported for VM use)
func GetVirtualModuleBuiltins(name string) map[string]Object {
	env := NewEnvironment()
	var builtins map[string]*Builtin

	switch name {
	case "list":
		builtins = ListBuiltins()
		SetListBuiltinTypes(builtins)
	case "map":
		builtins = GetMapBuiltins()
		SetMapBuiltinTypes(builtins)
	case "bytes":
		builtins = BytesBuiltins()
		SetBytesBuiltinTypes(builtins)
	case "bits":
		builtins = BitsBuiltins()
		SetBitsBuiltinTypes(builtins)
	case "time":
		builtins = TimeBuiltins()
		SetTimeBuiltinTypes(builtins)
	case "io":
		builtins = IOBuiltins()
		SetIOBuiltinTypes(builtins)
	case "sys":
		builtins = SysBuiltins()
		SetSysBuiltinTypes(builtins)
	case "tuple":
		builtins = TupleBuiltins()
		SetTupleBuiltinTypes(builtins)
	case "string":
		builtins = StringBuiltins()
		SetStringBuiltinTypes(builtins)
	case "math":
		builtins = MathBuiltins()
		SetMathBuiltinTypes(builtins)
	case "bignum":
		builtins = BignumBuiltins()
		SetBignumBuiltinTypes(builtins)
	case "char":
		builtins = CharBuiltins()
		SetCharBuiltinTypes(builtins)
	case "json":
		RegisterJsonBuiltins(env)
		return env.GetStore()
	case "crypto":
		builtins = CryptoBuiltins()
		SetCryptoBuiltinTypes(builtins)
	case "regex":
		builtins = RegexBuiltins()
		SetRegexBuiltinTypes(builtins)
	case "http":
		builtins = HttpBuiltins()
		SetHttpBuiltinTypes(builtins)
	case "test":
		builtins = TestBuiltins()
		SetTestBuiltinTypes(builtins)
	case "rand":
		builtins = RandBuiltins()
		SetRandBuiltinTypes(builtins)
	case "date":
		RegisterDateBuiltins(env)
		return env.GetStore()
	case "ws":
		builtins = WsBuiltins()
		SetWsBuiltinTypes(builtins)
	case "sql":
		RegisterSqlBuiltins(env)
		return env.GetStore()
	case "url":
		builtins = UrlBuiltins()
		SetUrlBuiltinTypes(builtins)
	case "path":
		builtins = PathBuiltins()
		SetPathBuiltinTypes(builtins)
	case "uuid":
		builtins = UuidBuiltins()
		SetUuidBuiltinTypes(builtins)
	case "log":
		builtins = LogBuiltins()
		SetLogBuiltinTypes(builtins)
	case "task":
		builtins = TaskBuiltins()
		SetTaskBuiltinTypes(builtins)
	case "csv":
		builtins = CsvBuiltins()
		SetCsvBuiltinTypes(builtins)
	case "flag":
		builtins = FlagBuiltins()
		SetFlagBuiltinTypes(builtins)
	case "option":
		builtins = OptionBuiltins()
	case "result":
		builtins = ResultBuiltins()
	default:
		return nil
	}

	if builtins != nil {
		for name, fn := range builtins {
			env.Set(name, fn)
		}

		// Inject exported types for specific modules that don't use Register...Builtins
		if name == "task" {
			env.Set("Task", &TypeObject{TypeVal: typesystem.TCon{Name: "Task"}})
		} else if name == "log" {
			env.Set("Logger", &TypeObject{TypeVal: typesystem.TCon{Name: "Logger"}})
		} else if name == "uuid" {
			env.Set("Uuid", &TypeObject{TypeVal: typesystem.TCon{Name: "Uuid"}})
		}

		return env.GetStore()
	}

	return nil
}

// importAllLibPackages handles import "lib" - imports all lib/* packages
func (e *Evaluator) importAllLibPackages(node *ast.ImportStatement, env *Environment) Object {
	// Collect all builtins from all lib/* packages
	allBuiltins := make(map[string]Object)

	for _, pkgName := range modules.GetLibSubPackages() {
		builtins := e.getVirtualModuleBuiltins(pkgName)
		for name, fn := range builtins {
			allBuiltins[name] = fn
		}
	}

	// Handle import specifications
	if node.ImportAll {
		// import "lib" (*) - import all symbols from all packages
		for name, fn := range allBuiltins {
			env.Set(name, fn)
		}
		return &Nil{}
	}

	if len(node.Symbols) > 0 {
		// import "lib" (symbol1, symbol2) - import specific symbols
		for _, sym := range node.Symbols {
			if fn, ok := allBuiltins[sym.Value]; ok {
				env.Set(sym.Value, fn)
			} else {
				return newError("symbol '%s' not found in lib packages", sym.Value)
			}
		}
		return &Nil{}
	}

	if len(node.Exclude) > 0 {
		// import "lib" !(symbol1, symbol2) - import all except specified
		excludeSet := make(map[string]bool)
		for _, sym := range node.Exclude {
			excludeSet[sym.Value] = true
		}
		for name, fn := range allBuiltins {
			if !excludeSet[name] {
				env.Set(name, fn)
			}
		}
		return &Nil{}
	}

	// Default: import "lib" as module object containing subpackages
	libFields := make(map[string]Object)
	for _, pkgName := range modules.GetLibSubPackages() {
		builtins := e.getVirtualModuleBuiltins(pkgName)
		if builtins != nil {
			pkgFields := make(map[string]Object)
			for name, fn := range builtins {
				pkgFields[name] = fn
			}
			libFields[pkgName] = NewRecord(pkgFields)
		}
	}

	libObj := NewRecord(libFields)

	name := "lib"
	if node.Alias != nil {
		name = node.Alias.Value
	}

	env.Set(name, libObj)
	return &Nil{}
}

// importPackageGroup handles importing from a directory containing sub-packages
func (e *Evaluator) importPackageGroup(node *ast.ImportStatement, mod *modules.Module, env *Environment) Object {
	// Collect all exported symbols from all sub-packages
	allFields := make(map[string]Object)

	for _, subMod := range mod.Imports {
		// Evaluate each sub-package
		subModObj, err := e.EvaluateModule(subMod)
		if err != nil {
			return newError("failed to evaluate sub-package %s: %s", subMod.Name, err.Error())
		}

		// Import trait implementations from sub-module
		for traitName, typeImpls := range subMod.ClassImplementations {
			if e.ClassImplementations[traitName] == nil {
				e.ClassImplementations[traitName] = make(map[string]Object)
			}
			for typeName, impl := range typeImpls {
				if obj, ok := impl.(Object); ok {
					e.ClassImplementations[traitName][typeName] = obj
				}
			}
		}

		subRecord, ok := subModObj.(*RecordInstance)
		if !ok {
			return newError("sub-package %s is not a record", subMod.Name)
		}

		// Collect exported fields
		for _, f := range subRecord.Fields {
			if subMod.Exports[f.Key] {
				allFields[f.Key] = f.Value
			}
		}
	}

	// Handle import specifications
	if node.ImportAll {
		for name, val := range allFields {
			env.Set(name, val)
		}
		return &Nil{}
	}

	if len(node.Symbols) > 0 {
		for _, sym := range node.Symbols {
			if val, ok := allFields[sym.Value]; ok {
				env.Set(sym.Value, val)
			} else {
				return newError("symbol '%s' not found in package group %s", sym.Value, mod.Name)
			}
		}
		return &Nil{}
	}

	if len(node.Exclude) > 0 {
		excludeSet := make(map[string]bool)
		for _, sym := range node.Exclude {
			excludeSet[sym.Value] = true
		}
		for name, val := range allFields {
			if !excludeSet[name] {
				env.Set(name, val)
			}
		}
		return &Nil{}
	}

	// Default: import as module object containing all sub-packages
	modObj := NewRecord(allFields)

	name := ""
	if node.Alias != nil {
		name = node.Alias.Value
	} else {
		name = mod.Name
	}

	env.Set(name, modObj)
	return &Nil{}
}

func (e *Evaluator) EvaluateModule(mod *modules.Module) (Object, error) {
	if cached, ok := e.ModuleCache[mod.Dir]; ok {
		return cached, nil
	}

	// Save current ClassImplementations to detect what was added by this module
	oldClassImpls := make(map[string]map[string]Object)
	for trait, impls := range e.ClassImplementations {
		oldClassImpls[trait] = make(map[string]Object)
		for typeName, impl := range impls {
			oldClassImpls[trait][typeName] = impl
		}
	}

	env := NewEnvironment()
	RegisterBuiltins(env)
	RegisterFPTraits(e, env) // Register FP traits for this module

	// Pre-create module object to handle cycles (empty for now)
	modObj := NewRecord(nil)
	e.ModuleCache[mod.Dir] = modObj

	oldBaseDir := e.BaseDir
	e.BaseDir = mod.Dir
	defer func() { e.BaseDir = oldBaseDir }()

	for _, file := range mod.Files {
		res := e.Eval(file, env)
		if isError(res) {
			return nil, fmt.Errorf("runtime error in %s: %s", mod.Name, res.Inspect())
		}
	}

	// Populate exports from environment
	exports := make(map[string]Object)
	for name := range mod.Exports {
		if val, ok := env.Get(name); ok {
			exports[name] = val
		} else {
			// Check if it's an extension method
			// Extension methods are stored in e.ExtensionMethods[typeName][methodName]
			// We need to find them and export them as regular functions
			for _, methods := range e.ExtensionMethods {
				if fn, ok := methods[name]; ok {
					// Found the extension method, export it as a regular function
					exports[name] = fn
					break
				}
			}
		}
	}

	// Update the existing modObj fields (in-place to preserve reference for cycles)
	// Create a temporary record to get sorted fields, then copy to modObj
	tempRec := NewRecord(exports)
	modObj.Fields = tempRec.Fields

	// Copy newly added ClassImplementations to module
	mod.ClassImplementations = make(map[string]map[string]interface{})
	for trait, impls := range e.ClassImplementations {
		if oldImpls, had := oldClassImpls[trait]; had {
			// Copy only new implementations for this trait
			for typeName, impl := range impls {
				if _, wasOld := oldImpls[typeName]; !wasOld {
					if mod.ClassImplementations[trait] == nil {
						mod.ClassImplementations[trait] = make(map[string]interface{})
					}
					mod.ClassImplementations[trait][typeName] = impl
				}
			}
		} else {
			// Copy all implementations for new trait
			mod.ClassImplementations[trait] = make(map[string]interface{})
			for typeName, impl := range impls {
				mod.ClassImplementations[trait][typeName] = impl
			}
		}
	}

	return modObj, nil
}

func (e *Evaluator) evalBlockStatement(block *ast.BlockStatement, env *Environment) Object {
	var result Object
	blockEnv := NewEnclosedEnvironment(env)

	for _, stmt := range block.Statements {
		result = e.Eval(stmt, blockEnv)
		if result != nil {
			rt := result.Type()
			if rt == ERROR_OBJ {
				return result
			}
			if rt == RETURN_VALUE_OBJ {
				return result
			}
			if rt == BREAK_SIGNAL_OBJ || rt == CONTINUE_SIGNAL_OBJ {
				return result
			}
		}
	}

	if result == nil {
		return &Nil{}
	}
	return result
}

func (e *Evaluator) evalTypeDeclaration(node *ast.TypeDeclarationStatement, env *Environment) Object {
	tCon := typesystem.TCon{Name: node.Name.Value}
	env.Set(node.Name.Value, &TypeObject{TypeVal: tCon})

	if node.IsAlias {
		// For type aliases, store TCon with the alias name (not the expanded type)
		// This ensures getType(Point) returns type(Point), not type({ x: Int, y: Int })
		// Also store the underlying type in TypeAliases for default() to work
		if e.TypeAliases == nil {
			e.TypeAliases = make(map[string]typesystem.Type)
		}
		underlyingType := analyzer.BuildType(node.TargetType, nil, nil)
		e.TypeAliases[node.Name.Value] = underlyingType
		return &Nil{}
	}

	for _, c := range node.Constructors {
		if len(c.Parameters) == 0 {
			env.Set(c.Name.Value, &DataInstance{Name: c.Name.Value, Fields: []Object{}, TypeName: node.Name.Value})
		} else {
			env.Set(c.Name.Value, &Constructor{Name: c.Name.Value, TypeName: node.Name.Value, Arity: len(c.Parameters)})
		}
	}
	return &Nil{}
}

func (e *Evaluator) evalTraitDeclaration(node *ast.TraitDeclaration, env *Environment) Object {
	// Register SuperTraits
	var superTraits []string
	for _, st := range node.SuperTraits {
		name := extractTypeNameFromAST(st)
		if name != "" {
			superTraits = append(superTraits, name)
		}
	}
	e.TraitSuperTraits[node.Name.Value] = superTraits

	// Update global TraitMethods map for dictionary lookups
	// This is required for FindMethodInDictionary to work with user-defined traits
	var methodNames []string
	for _, sig := range node.Signatures {
		name := sig.Name.Value
		if sig.Operator != "" {
			name = "(" + sig.Operator + ")"
		}
		methodNames = append(methodNames, name)
	}
	TraitMethods[node.Name.Value] = methodNames

	for _, sig := range node.Signatures {
		methodName := sig.Name.Value
		// For operator methods, use the synthetic name "(+)" etc.
		// sig.Operator is non-empty for operator methods
		if sig.Operator != "" {
			methodName = "(" + sig.Operator + ")"
		}
		// Arity from parameter count - used to determine if auto-call in type context
		arity := len(sig.Parameters)
		env.Set(methodName, &ClassMethod{Name: methodName, ClassName: node.Name.Value, Arity: arity})

		// Register default implementation if body exists
		if sig.Body != nil {
			key := node.Name.Value + "." + methodName
			if e.TraitDefaults == nil {
				e.TraitDefaults = make(map[string]*ast.FunctionStatement)
			}
			e.TraitDefaults[key] = sig
		}
	}

	if _, ok := e.ClassImplementations[node.Name.Value]; !ok {
		e.ClassImplementations[node.Name.Value] = make(map[string]Object)
	}

	return &Nil{}
}

func (e *Evaluator) evalInstanceDeclaration(node *ast.InstanceDeclaration, env *Environment) Object {
	className := node.TraitName.Value

	var typeKey string
	if len(node.Args) > 0 {
		// MPTC: Combine all arg types into a key "Type1_Type2"
		// This matches the lookup strategy we will implement
		var typeNames []string
		for _, arg := range node.Args {
			typeName, err := e.resolveCanonicalTypeName(arg, env)
			if err != nil {
				return newError("%s", err.Error())
			}
			typeNames = append(typeNames, typeName)
		}
		typeKey = strings.Join(typeNames, "_")
	} else {
		// Should not happen if parser validates args
		return newError("instance declaration missing arguments")
	}

	if _, ok := e.ClassImplementations[className]; !ok {
		e.ClassImplementations[className] = make(map[string]Object)
	}

	methods := make(map[string]Object)
	for _, method := range node.Methods {
		fn := &Function{
			Name:          method.Name.Value,
			Parameters:    method.Parameters,
			WitnessParams: method.WitnessParams,
			ReturnType:    method.ReturnType,
			Body:          method.Body,
			Env:           env,
			Line:          method.Token.Line,
			Column:        method.Token.Column,
		}
		methods[method.Name.Value] = fn
	}

	table := &MethodTable{Methods: methods}
	e.ClassImplementations[className][typeKey] = table

	// Register constructor for generic instances (Tree-walk mode support for dictionary passing)
	if len(node.AnalyzedRequirements) > 0 || len(node.TypeParams) > 0 {
		typeName := typeKey // Use typeKey as typeName (e.g. Expr_t)
		if len(node.Args) > 0 {
			// Try to get simple name if possible, or use typeKey
			simpleName, _ := e.resolveCanonicalTypeName(node.Args[0], env)
			if simpleName != "" {
				// Use the base name (e.g. Expr) if it matches the start of typeKey?
				// analyzer uses TypeName from TCon.
				// Here we approximate.
				// If typeKey is "Expr_t", we want "Expr".
				if parts := strings.Split(typeKey, "_"); len(parts) > 0 {
					typeName = parts[0]
				}
			}
		}

		evidenceName := analyzer.GetDictionaryConstructorName(className, typeName)

		ctor := &Builtin{
			Name: evidenceName,
			Fn: func(ev *Evaluator, args ...Object) Object {
				if len(args) != len(node.AnalyzedRequirements) {
					return newError("constructor %s expects %d arguments, got %d", evidenceName, len(node.AnalyzedRequirements), len(args))
				}

				// Create Dictionary
				dict := &Dictionary{
					TraitName: className,
					Methods:   nil, // Populated below
					Supers:    []*Dictionary{},
				}

				// Create closure environment
				closureEnv := NewEnclosedEnvironment(env)

				// Bind witnesses
				for i, req := range node.AnalyzedRequirements {
					// Construct param name: $w_TypeVar_Trait_Args...
					paramName := analyzer.GetWitnessParamName(req.TypeVar, req.Trait)
					if len(req.Args) > 0 {
						for _, arg := range req.Args {
							paramName += "_" + arg.String()
						}
					}
					closureEnv.Set(paramName, args[i])
				}

				// Populate Supers
				if superTraits, ok := e.TraitSuperTraits[className]; ok {
					for _, stName := range superTraits {
						for i, req := range node.AnalyzedRequirements {
							if req.Trait == stName {
								if dictArg, ok := args[i].(*Dictionary); ok {
									dict.Supers = append(dict.Supers, dictArg)
								} else {
									return newError("witness for super trait %s must be a dictionary", stName)
								}
								break
							}
						}
					}
				}

				// Create methods
				methodsMap := make(map[string]Object)
				for _, method := range node.Methods {
					methodFn := &Function{
						Name:          method.Name.Value,
						Parameters:    method.Parameters,
						WitnessParams: method.WitnessParams,
						ReturnType:    method.ReturnType,
						Body:          method.Body,
						Env:           closureEnv, // Capture witnesses
						Line:          method.Token.Line,
						Column:        method.Token.Column,
					}
					methodsMap[method.Name.Value] = methodFn
				}

				// Map to slice based on Trait definition order
				if methodNames, ok := TraitMethods[className]; ok {
					dict.Methods = make([]Object, len(methodNames))
					for i, name := range methodNames {
						if fn, ok := methodsMap[name]; ok {
							dict.Methods[i] = fn
						} else {
							// Should we check default implementation?
							// For instance declaration, if method is missing, it should use default.
							// But here we are constructing the dictionary.
							// If `node.Methods` is incomplete, we should check `e.TraitDefaults` or global defaults.
							// But usually analyzer ensures completeness or defaults are injected?
							// If missing, we might leave it nil or error?
							// For now, leave nil (or panic if accessed).
						}
					}
				} else {
					// Fallback if TraitMethods not found (e.g. built-in trait?)
					// Built-in traits like Show/Equal might not be in TraitMethods map if not declared in source?
					// But user instances implement them.
					// We need to ensure built-in traits are in TraitMethods.
					// Builtins are registered elsewhere.
				}

				return dict
			},
		}
		env.Set(evidenceName, ctor)
	}

	return &Nil{}
}

func (e *Evaluator) evalConstantDeclaration(node *ast.ConstantDeclaration, env *Environment) Object {
	// Set type context BEFORE evaluating value, so nullary ClassMethod calls can dispatch
	// This mirrors evalAssignExpression behavior
	oldCallNode := e.CurrentCallNode
	var pushedTypeName string
	var pushedWitness bool

	if node.TypeAnnotation != nil {
		e.CurrentCallNode = node
		// Push type name to stack to guide dispatch for inner calls
		pushedTypeName = extractTypeNameFromAST(node.TypeAnnotation)
		if pushedTypeName != "" {
			e.TypeContextStack = append(e.TypeContextStack, pushedTypeName)
		}
	}

	val := e.Eval(node.Value, env)

	// Restore previous call node and stack
	if pushedTypeName != "" && len(e.TypeContextStack) > 0 {
		e.TypeContextStack = e.TypeContextStack[:len(e.TypeContextStack)-1]
	}
	if pushedWitness {
		e.PopWitness()
	}
	e.CurrentCallNode = oldCallNode

	if isError(val) {
		return val
	}

	// Propagate TypeName from annotation if value is a record
	if node.TypeAnnotation != nil {
		// If value is a RecordInstance and type annotation is a named type, set TypeName
		if record, ok := val.(*RecordInstance); ok {
			// Handle simple named type (e.g. Point)
			if namedType, ok := node.TypeAnnotation.(*ast.NamedType); ok {
				record.TypeName = namedType.Name.Value
			}
		}
	}

	// Handle pattern destructuring
	if node.Pattern != nil {
		return e.bindPatternToValue(node.Pattern, val, env)
	}

	// Simple binding
	env.Set(node.Name.Value, val)
	return &Nil{}
}

func (e *Evaluator) evalPatternAssignExpression(node *ast.PatternAssignExpression, env *Environment) Object {
	val := e.Eval(node.Value, env)
	if isError(val) {
		return val
	}
	return e.bindPatternToValue(node.Pattern, val, env)
}

// bindPatternToValue binds variables from a pattern to their values
func (e *Evaluator) bindPatternToValue(pat ast.Pattern, val Object, env *Environment) Object {
	switch p := pat.(type) {
	case *ast.IdentifierPattern:
		env.Set(p.Value, val)
		return &Nil{}

	case *ast.TuplePattern:
		tuple, ok := val.(*Tuple)
		if !ok {
			return newError("cannot destructure non-tuple value with tuple pattern")
		}
		if len(tuple.Elements) != len(p.Elements) {
			return newError("tuple pattern has %d elements but value has %d", len(p.Elements), len(tuple.Elements))
		}
		for i, elem := range p.Elements {
			result := e.bindPatternToValue(elem, tuple.Elements[i], env)
			if isError(result) {
				return result
			}
		}
		return &Nil{}

	case *ast.ListPattern:
		list, ok := val.(*List)
		if !ok {
			return newError("cannot destructure non-list value with list pattern")
		}
		if list.len() < len(p.Elements) {
			return newError("list pattern has %d elements but value has %d", len(p.Elements), list.len())
		}
		for i, elem := range p.Elements {
			result := e.bindPatternToValue(elem, list.get(i), env)
			if isError(result) {
				return result
			}
		}
		return &Nil{}

	case *ast.WildcardPattern:
		// Ignore - don't bind anything
		return &Nil{}

	case *ast.RecordPattern:
		record, ok := val.(*RecordInstance)
		if !ok {
			return newError("cannot destructure non-record value with record pattern")
		}
		for fieldName, fieldPat := range p.Fields {
			fieldVal := record.Get(fieldName)
			if fieldVal == nil {
				return newError("record does not have field '%s'", fieldName)
			}
			result := e.bindPatternToValue(fieldPat, fieldVal, env)
			if isError(result) {
				return result
			}
		}
		return &Nil{}

	default:
		return newError("unsupported pattern in destructuring")
	}
}

func (e *Evaluator) evalExtensionMethod(node *ast.FunctionStatement, env *Environment) Object {
	typeName, err := e.resolveCanonicalTypeName(node.Receiver.Type, env)
	if err != nil {
		return newError("%s", err.Error())
	}

	methodName := node.Name.Value

	fn := &Function{
		Name:          node.Name.Value,
		Parameters:    node.Parameters,
		WitnessParams: node.WitnessParams,
		ReturnType:    node.ReturnType,
		Body:          node.Body,
		Env:           env,
		Line:          node.Token.Line,
		Column:        node.Token.Column,
	}
	newParams := append([]*ast.Parameter{node.Receiver}, node.Parameters...)
	fn.Parameters = newParams

	if _, ok := e.ExtensionMethods[typeName]; !ok {
		e.ExtensionMethods[typeName] = make(map[string]Object)
	}
	e.ExtensionMethods[typeName][methodName] = fn

	return &Nil{}
}

func (e *Evaluator) evalForExpression(node *ast.ForExpression, env *Environment) Object {
	loopEnv := NewEnclosedEnvironment(env)

	runBody := func() (Object, bool) {
		res := e.evalBlockStatement(node.Body, loopEnv)

		if isError(res) {
			return res, true
		}

		if breakSig, ok := res.(*BreakSignal); ok {
			return breakSig.Value, true
		}

		if _, ok := res.(*ContinueSignal); ok {
			return nil, false
		}

		if rt := res.Type(); rt == RETURN_VALUE_OBJ {
			return res, true
		}

		return res, false
	}

	var lastResult Object = &Nil{}

	if node.Iterable != nil {
		iterable := e.Eval(node.Iterable, env)
		if isError(iterable) {
			return iterable
		}

		var iteratorFn Object

		// Look up iter method from Iter trait implementation for this type
		iterableTypeName := getRuntimeTypeName(iterable)
		if iterMethod, found := e.lookupTraitMethod(config.IterTraitName, config.IterMethodName, iterableTypeName); found {
			res := e.ApplyFunction(iterMethod, []Object{iterable})
			if !isError(res) {
				iteratorFn = res
			}
		}

		// Fallback: try direct environment lookup (for backward compatibility)
		if iteratorFn == nil {
			if iterSym, ok := env.Get(config.IterMethodName); ok {
				res := e.ApplyFunction(iterSym, []Object{iterable})
				if !isError(res) {
					iteratorFn = res
				}
			}
		}

		if iteratorFn != nil {
			itemName := node.ItemName.Value

			for {
				stepRes := e.ApplyFunction(iteratorFn, []Object{})
				if isError(stepRes) {
					return stepRes
				}

				if data, ok := stepRes.(*DataInstance); ok && data.TypeName == config.OptionTypeName {
					if data.Name == config.SomeCtorName {
						val := data.Fields[0]
						loopEnv.Set(itemName, val)

						res, shouldBreak := runBody()
						if shouldBreak {
							if isError(res) || res.Type() == RETURN_VALUE_OBJ {
								return res
							}
							return res
						}
						if res != nil {
							lastResult = res
						}
					} else if data.Name == config.ZeroCtorName {
						break
					} else {
						return newError("iterator returned unexpected Option variant: %s", data.Name)
					}
				} else {
					return newError("iterator must return Option, got %s", stepRes.Type())
				}
			}

		} else {
			var items []Object
			if list, ok := iterable.(*List); ok {
				items = list.ToSlice()
			} else if str, ok := iterable.(*List); ok {
				items = str.ToSlice()
			} else if rng, ok := iterable.(*Range); ok {
				// Expand range to items
				// We support Int and Char ranges for now, similar to VM
				start := rng.Start
				end := rng.End
				next := rng.Next

				var current int64
				var endVal int64
				var step int64 = 1
				isChar := false
				isNumeric := false

				if sInt, ok := start.(*Integer); ok {
					if eInt, ok := end.(*Integer); ok {
						current = sInt.Value
						endVal = eInt.Value
						isNumeric = true
						if nInt, ok := next.(*Integer); ok {
							step = nInt.Value - current
						}
					}
				} else if sChar, ok := start.(*Char); ok {
					if eChar, ok := end.(*Char); ok {
						current = sChar.Value
						endVal = eChar.Value
						isChar = true
						isNumeric = true
						if nChar, ok := next.(*Char); ok {
							step = nChar.Value - current
						}
					}
				}

				if isNumeric {
					// Generate items
					// Inclusive end
					for {
						if step > 0 {
							if current > endVal {
								break
							}
						} else {
							if current < endVal {
								break
							}
						}

						var item Object
						if isChar {
							item = &Char{Value: current}
						} else {
							item = &Integer{Value: current}
						}
						items = append(items, item)

						current += step
					}
				} else {
					return newError("range iteration only supported for Int and Char in interpreter mode")
				}
			} else {
				return newError("iterable must be List or implement Iter trait, got %s", iterable.Type())
			}

			itemName := node.ItemName.Value

			for _, item := range items {
				loopEnv.Set(itemName, item)

				res, shouldBreak := runBody()
				if shouldBreak {
					if isError(res) || res.Type() == RETURN_VALUE_OBJ {
						return res
					}
					return res
				}
				if res != nil {
					lastResult = res
				}
			}
		}

	} else {
		for {
			cond := e.Eval(node.Condition, loopEnv)
			if isError(cond) {
				return cond
			}

			if !e.isTruthy(cond) {
				break
			}

			res, shouldBreak := runBody()
			if shouldBreak {
				if isError(res) || res.Type() == RETURN_VALUE_OBJ {
					return res
				}
				return res
			}
			if res != nil {
				lastResult = res
			}
		}
	}

	return lastResult
}

func (e *Evaluator) evalBreakStatement(node *ast.BreakStatement, env *Environment) Object {
	var val Object
	if node.Value != nil {
		val = e.Eval(node.Value, env)
		if isError(val) {
			return val
		}
	} else {
		val = &Nil{}
	}
	return &BreakSignal{Value: val}
}

func (e *Evaluator) evalContinueStatement(node *ast.ContinueStatement, env *Environment) Object {
	return &ContinueSignal{}
}

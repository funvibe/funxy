package evaluator

import (
	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/typesystem"
	"strings"
)

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

		// Retrieve Dispatch Strategy from SymbolTable (if available)
		var dispatchSources []typesystem.DispatchSource
		if env.SymbolTable != nil {
			if sources, ok := env.SymbolTable.GetTraitMethodDispatch(node.Name.Value, methodName); ok {
				dispatchSources = sources
			}
		}

		env.Set(methodName, &ClassMethod{
			Name:            methodName,
			ClassName:       node.Name.Value,
			Arity:           arity,
			DispatchSources: dispatchSources,
		})

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
	return val
}

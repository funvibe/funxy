package ext

import (
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/typesystem"
)

// RegisterVirtualPackages registers ext/* virtual packages from the inspection
// result so that the Funxy analyzer can resolve "import ext/name" during
// compilation and type-check the imported symbols.
//
// This is called during "funxy build" when funxy.yaml is present.
func RegisterVirtualPackages(cfg *Config, result *InspectResult) {
	// Group bindings by ext module name
	groups := make(map[string][]*ResolvedBinding)
	for _, b := range result.Bindings {
		modName := b.Dep.ExtModuleName()
		groups[modName] = append(groups[modName], b)
	}

	for modName, bindings := range groups {
		pkg := buildVirtualPackage(modName, bindings)
		modules.RegisterVirtualPackage("ext/"+modName, pkg)
	}
}

// RegisterMinimalVirtualPackages registers minimal ext/* virtual packages
// (without detailed type info) for cases when inspection results are not
// available. This is sufficient for the analyzer to not reject ext/* imports.
func RegisterMinimalVirtualPackages(cfg *Config) {
	for _, dep := range cfg.Deps {
		modName := dep.ExtModuleName()
		pkg := &modules.VirtualPackage{
			Name:    modName,
			Symbols: make(map[string]typesystem.Type),
		}

		// Add symbols with generic variadic function types.
		// Using TVar ensures the analyzer won't reject calls due to type mismatch.
		// These are permissive placeholders — real types come from inspection.
		for _, bind := range dep.Bind {
			if bind.Type != "" {
				// For type bindings with explicit methods, register <as><Method> symbols.
				// E.g. type: UUID, as: uuid, methods: [String, Version]
				// → uuidString, uuidVersion
				for _, methodName := range bind.Methods {
					funxyName := bind.As + methodName
					pkg.Symbols[funxyName] = typesystem.TFunc{
						Params:     []typesystem.Type{typesystem.TVar{Name: "_ext_arg_" + funxyName}},
						ReturnType: typesystem.TVar{Name: "_ext_ret_" + funxyName},
						IsVariadic: true,
					}
				}
				// Register constructor if enabled: bare <as> prefix is the function name
				if bind.Constructor {
					pkg.Symbols[bind.As] = typesystem.TFunc{
						Params:     []typesystem.Type{typesystem.TVar{Name: "_ext_arg_" + bind.As}},
						ReturnType: typesystem.TVar{Name: "_ext_ret_" + bind.As},
						IsVariadic: true,
					}
				}
				// Note: bind_all or type bindings without explicit methods list
				// cannot be enumerated without inspection. The full inspection path
				// (RegisterVirtualPackages) handles this case.
			}
			if bind.Func != "" {
				pkg.Symbols[bind.As] = typesystem.TFunc{
					Params:     []typesystem.Type{typesystem.TVar{Name: "_ext_arg_" + bind.As}},
					ReturnType: typesystem.TVar{Name: "_ext_ret_" + bind.As},
					IsVariadic: true,
				}
			}
			if bind.Const != "" {
				// Constants are values, use a permissive type placeholder
				pkg.Symbols[bind.As] = typesystem.TVar{Name: "_ext_const_" + bind.As}
			}
		}

		modules.RegisterVirtualPackage("ext/"+modName, pkg)
	}
}

// RegisterVirtualPackagesFromRegistry registers ext/* virtual packages for all
// ext modules that were registered at startup via evaluator.RegisterExtBuiltins().
//
// This is used when the ext build binary runs scripts directly — there's no
// funxy.yaml to parse, but the ext builtins are already compiled in. The
// analyzer needs virtual packages to resolve "import ext/name" statements.
//
// Each registered ext builtin function gets a permissive variadic type signature,
// allowing the analyzer to accept any call without exact type checking.
func RegisterVirtualPackagesFromRegistry() {
	for _, modName := range evaluator.GetAllExtModules() {
		// Skip if already registered (e.g., via RegisterMinimalVirtualPackages)
		if modules.IsVirtualPackage("ext/" + modName) {
			continue
		}

		builtins := evaluator.GetExtBuiltins(modName)
		pkg := &modules.VirtualPackage{
			Name:    modName,
			Symbols: make(map[string]typesystem.Type),
		}

		for funcName := range builtins {
			pkg.Symbols[funcName] = typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.TVar{Name: "_ext_arg_" + funcName}},
				ReturnType: typesystem.TVar{Name: "_ext_ret_" + funcName},
				IsVariadic: true,
			}
		}

		modules.RegisterVirtualPackage("ext/"+modName, pkg)
	}
}

// buildVirtualPackage creates a VirtualPackage with proper types from inspection results.
func buildVirtualPackage(modName string, bindings []*ResolvedBinding) *modules.VirtualPackage {
	pkg := &modules.VirtualPackage{
		Name:    modName,
		Symbols: make(map[string]typesystem.Type),
	}

	for _, b := range bindings {
		if b.TypeBinding != nil {
			for _, method := range b.TypeBinding.Methods {
				// Build function type: (args...) -> returnType
				funcType := buildFunxyFuncType(method.Signature, true)
				pkg.Symbols[method.FunxyName] = funcType
			}
			// Field getters: <prefix><FieldName>(HostObject) -> fieldType
			if b.TypeBinding.IsStruct {
				for _, field := range b.TypeBinding.Fields {
					funxyName := b.Spec.As + ucFirst(field.GoName)
					pkg.Symbols[funxyName] = typesystem.TFunc{
						Params:     []typesystem.Type{typesystem.TCon{Name: "HostObject"}},
						ReturnType: goTypeRefToFunxyType(field.Type),
					}
				}
			}
			// Constructor: <prefix>(Record) -> HostObject
			if b.Spec.Constructor && b.TypeBinding.IsStruct {
				pkg.Symbols[b.Spec.As] = typesystem.TFunc{
					Params:     []typesystem.Type{typesystem.TVar{Name: "_ext_record_" + b.Spec.As}},
					ReturnType: typesystem.TCon{Name: "HostObject"},
				}
			}
		}

		if b.FuncBinding != nil {
			funcType := buildFunxyFuncType(b.FuncBinding.Signature, false)
			pkg.Symbols[b.Spec.As] = funcType
		}

		if b.ConstBinding != nil {
			pkg.Symbols[b.Spec.As] = goTypeRefToFunxyType(b.ConstBinding.Type)
		}
	}

	return pkg
}

// buildFunxyFuncType constructs a Funxy function type from a Go signature.
// If hasReceiver is true, the function takes an additional HostObject as the first arg.
func buildFunxyFuncType(sig *FuncSignature, hasReceiver bool) typesystem.Type {
	// Build parameter types
	var paramTypes []typesystem.Type

	if hasReceiver {
		paramTypes = append(paramTypes, typesystem.TCon{Name: "HostObject"})
	}

	for _, p := range sig.Params {
		paramTypes = append(paramTypes, goTypeRefToFunxyType(p.Type))
	}

	// Build return type
	var returnType typesystem.Type
	if len(sig.Results) == 0 {
		returnType = typesystem.TCon{Name: "Nil"}
	} else if sig.HasErrorReturn && len(sig.Results) == 2 {
		// (T, error) → Result<String, T>
		innerType := goTypeRefToFunxyType(sig.Results[0].Type)
		returnType = typesystem.TApp{
			Constructor: typesystem.TCon{Name: "Result"},
			Args:        []typesystem.Type{typesystem.TCon{Name: "String"}, innerType},
		}
	} else if len(sig.Results) == 1 {
		returnType = goTypeRefToFunxyType(sig.Results[0].Type)
	} else {
		returnType = typesystem.TCon{Name: "HostObject"} // Tuple fallback
	}

	// Build function type: (params...) -> returnType
	return typesystem.TFunc{
		Params:     paramTypes,
		ReturnType: returnType,
		IsVariadic: sig.IsVariadic,
	}
}

// goTypeRefToFunxyType converts a GoTypeRef to a Funxy typesystem.Type.
// Dispatches on ref.Kind (not FunxyType string) for reliability.
func goTypeRefToFunxyType(ref GoTypeRef) typesystem.Type {
	switch ref.Kind {
	case GoTypeBasic:
		switch ref.FunxyType {
		case "Int":
			return typesystem.TCon{Name: "Int"}
		case "Float":
			return typesystem.TCon{Name: "Float"}
		case "Bool":
			return typesystem.TCon{Name: "Bool"}
		case "String":
			return typesystem.TCon{Name: "String"}
		default:
			return typesystem.TCon{Name: "HostObject"}
		}

	case GoTypeByteSlice:
		return typesystem.TCon{Name: "Bytes"}

	case GoTypeFunc:
		if ref.FuncSig != nil {
			var params []typesystem.Type
			for _, p := range ref.FuncSig.Params {
				params = append(params, goTypeRefToFunxyType(p.Type))
			}
			var returnType typesystem.Type
			if len(ref.FuncSig.Results) == 0 {
				returnType = typesystem.TCon{Name: "Nil"}
			} else if len(ref.FuncSig.Results) == 1 {
				returnType = goTypeRefToFunxyType(ref.FuncSig.Results[0].Type)
			} else {
				returnType = typesystem.TCon{Name: "HostObject"}
			}
			return typesystem.TFunc{
				Params:     params,
				ReturnType: returnType,
			}
		}
		return typesystem.TCon{Name: "HostObject"}

	case GoTypeSlice, GoTypeArray:
		if ref.ElemType != nil {
			elemType := goTypeRefToFunxyType(*ref.ElemType)
			return typesystem.TApp{
				Constructor: typesystem.TCon{Name: "List"},
				Args:        []typesystem.Type{elemType},
			}
		}
		return typesystem.TCon{Name: "HostObject"}

	case GoTypeMap:
		if ref.KeyType != nil && ref.ElemType != nil {
			keyType := goTypeRefToFunxyType(*ref.KeyType)
			valType := goTypeRefToFunxyType(*ref.ElemType)
			return typesystem.TApp{
				Constructor: typesystem.TCon{Name: "Map"},
				Args:        []typesystem.Type{keyType, valType},
			}
		}
		return typesystem.TCon{Name: "HostObject"}

	case GoTypeError:
		return typesystem.TCon{Name: "String"}

	default:
		// GoTypePtr, GoTypeStruct, GoTypeInterface, GoTypeNamed, GoTypeChan,
		// GoTypeContext, GoTypeTypeParam — all wrap as HostObject
		return typesystem.TCon{Name: "HostObject"}
	}
}

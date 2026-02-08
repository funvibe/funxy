package vm

import "github.com/funvibe/funxy/internal/evaluator"

// SetEvaluator sets the evaluator to use for builtin calls
func (vm *VM) SetEvaluator(eval *evaluator.Evaluator) {
	vm.eval = eval
}

// RegisterBuiltins registers all standard builtins in VM globals
func (vm *VM) RegisterBuiltins() {
	// Create a temp environment and register builtins there
	env := evaluator.NewEnvironment()
	evaluator.RegisterBuiltins(env)

	// Get builtins using the accessor
	vm.importBuiltinsFromEnv(env)

	// Register FP trait methods (fmap, pure, mempty, etc.)
	vm.registerFPTraitMethods()
}

// registerFPTraitMethods registers FP trait class methods as globals
func (vm *VM) registerFPTraitMethods() {
	// Semigroup
	vm.globals.Globals = vm.globals.Globals.Put("(< >)", &evaluator.ClassMethod{Name: "(<>)", ClassName: "Semigroup", Arity: 2})

	// Monoid
	vm.globals.Globals = vm.globals.Globals.Put("mempty", &evaluator.ClassMethod{Name: "mempty", ClassName: "Monoid", Arity: 0})

	// Functor
	vm.globals.Globals = vm.globals.Globals.Put("fmap", &evaluator.ClassMethod{Name: "fmap", ClassName: "Functor", Arity: 2})

	// Applicative
	vm.globals.Globals = vm.globals.Globals.Put("pure", &evaluator.ClassMethod{Name: "pure", ClassName: "Applicative", Arity: 1})
	vm.globals.Globals = vm.globals.Globals.Put("(<*>)", &evaluator.ClassMethod{Name: "(<*>)", ClassName: "Applicative", Arity: 2})

	// Monad
	vm.globals.Globals = vm.globals.Globals.Put("(>>=)", &evaluator.ClassMethod{Name: "(>>=)", ClassName: "Monad", Arity: 2})

	// Optional
	vm.globals.Globals = vm.globals.Globals.Put("(??)", &evaluator.ClassMethod{Name: "(??)", ClassName: "Optional", Arity: 2})

	// Empty
	vm.globals.Globals = vm.globals.Globals.Put("isEmpty", &evaluator.ClassMethod{Name: "isEmpty", ClassName: "Empty", Arity: 1})

	// Show trait
	vm.globals.Globals = vm.globals.Globals.Put("show", &evaluator.ClassMethod{Name: "show", ClassName: "Show", Arity: 1})

	// Equal trait
	vm.globals.Globals = vm.globals.Globals.Put("(==)", &evaluator.ClassMethod{Name: "(==)", ClassName: "Equal", Arity: 2})
	vm.globals.Globals = vm.globals.Globals.Put("(/=)", &evaluator.ClassMethod{Name: "(/=)", ClassName: "Equal", Arity: 2})

	// Ord trait
	vm.globals.Globals = vm.globals.Globals.Put("(<)", &evaluator.ClassMethod{Name: "(<)", ClassName: "Ord", Arity: 2})
	vm.globals.Globals = vm.globals.Globals.Put("(<=)", &evaluator.ClassMethod{Name: "(<=)", ClassName: "Ord", Arity: 2})
	vm.globals.Globals = vm.globals.Globals.Put("(>)", &evaluator.ClassMethod{Name: "(>)", ClassName: "Ord", Arity: 2})
	vm.globals.Globals = vm.globals.Globals.Put("(>=)", &evaluator.ClassMethod{Name: "(>=)", ClassName: "Ord", Arity: 2})

	// _bindWitness internal builtin
	// _bindWitness(fn, witness1, witness2, ...) -> PartialApplication
	vm.globals.Globals = vm.globals.Globals.Put("_bindWitness", &evaluator.Builtin{
		Name: "_bindWitness",
		Fn: func(e *evaluator.Evaluator, args ...evaluator.Object) evaluator.Object {
			if len(args) < 2 {
				return &evaluator.Error{Message: "_bindWitness expects at least 2 arguments (fn, witness)"}
			}
			fn := args[0]
			witnesses := args[1:]

			// Check if fn is already a PartialApplication
			if existingPa, ok := fn.(*evaluator.PartialApplication); ok {
				// Append witnesses to existing applied args
				newArgs := append(existingPa.AppliedArgs, witnesses...)
				return &evaluator.PartialApplication{
					Function:        existingPa.Function,
					Builtin:         existingPa.Builtin,
					Constructor:     existingPa.Constructor,
					ClassMethod:     existingPa.ClassMethod,
					VMClosure:       existingPa.VMClosure,
					AppliedArgs:     newArgs,
					RemainingParams: existingPa.RemainingParams,
				}
			}

			// Create new PartialApplication
			res := &evaluator.PartialApplication{
				AppliedArgs: witnesses,
			}
			switch f := fn.(type) {
			case *evaluator.Function:
				res.Function = f
			case *evaluator.Builtin:
				res.Builtin = f
			case *evaluator.ClassMethod:
				res.ClassMethod = f
			case *evaluator.Constructor:
				res.Constructor = f
			default:
				// Assume it's a VM Closure or compatible object
				res.VMClosure = f
			}
			return res
		},
	})

	// Register builtin trait implementations for primitive types
	vm.registerBuiltinTraitImpls()
}

// registerBuiltinTraitImpls registers native implementations of traits for builtin types
func (vm *VM) registerBuiltinTraitImpls() {
	// Show for basic types
	vm.registerBuiltinTraitMethod("Show", "Int", "show", func(args []evaluator.Object) evaluator.Object {
		return evaluator.StringToList(args[0].Inspect())
	})
	vm.registerBuiltinTraitMethod("Show", "Float", "show", func(args []evaluator.Object) evaluator.Object {
		return evaluator.StringToList(args[0].Inspect())
	})
	vm.registerBuiltinTraitMethod("Show", "Bool", "show", func(args []evaluator.Object) evaluator.Object {
		return evaluator.StringToList(args[0].Inspect())
	})
	vm.registerBuiltinTraitMethod("Show", "String", "show", func(args []evaluator.Object) evaluator.Object {
		return evaluator.StringToList(args[0].Inspect())
	})
	vm.registerBuiltinTraitMethod("Show", "Char", "show", func(args []evaluator.Object) evaluator.Object {
		if c, ok := args[0].(*evaluator.Char); ok {
			return evaluator.StringToList("'" + string(rune(c.Value)) + "'")
		}
		return evaluator.StringToList(args[0].Inspect())
	})
	vm.registerBuiltinTraitMethod("Show", "List", "show", func(args []evaluator.Object) evaluator.Object {
		return evaluator.StringToList(args[0].Inspect())
	})
	vm.registerBuiltinTraitMethod("Show", "Tuple", "show", func(args []evaluator.Object) evaluator.Object {
		return evaluator.StringToList(args[0].Inspect())
	})
	vm.registerBuiltinTraitMethod("Show", "Record", "show", func(args []evaluator.Object) evaluator.Object {
		return evaluator.StringToList(args[0].Inspect())
	})
	vm.registerBuiltinTraitMethod("Show", "Map", "show", func(args []evaluator.Object) evaluator.Object {
		return evaluator.StringToList(args[0].Inspect())
	})
	vm.registerBuiltinTraitMethod("Show", "Option", "show", func(args []evaluator.Object) evaluator.Object {
		return evaluator.StringToList(args[0].Inspect())
	})
	vm.registerBuiltinTraitMethod("Show", "Result", "show", func(args []evaluator.Object) evaluator.Object {
		return evaluator.StringToList(args[0].Inspect())
	})
	vm.registerBuiltinTraitMethod("Show", "BigInt", "show", func(args []evaluator.Object) evaluator.Object {
		return evaluator.StringToList(args[0].Inspect())
	})
	vm.registerBuiltinTraitMethod("Show", "Rational", "show", func(args []evaluator.Object) evaluator.Object {
		return evaluator.StringToList(args[0].Inspect())
	})
	vm.registerBuiltinTraitMethod("Show", "Bytes", "show", func(args []evaluator.Object) evaluator.Object {
		return evaluator.StringToList(args[0].Inspect())
	})
	vm.registerBuiltinTraitMethod("Show", "Bits", "show", func(args []evaluator.Object) evaluator.Object {
		return evaluator.StringToList(args[0].Inspect())
	})
	vm.registerBuiltinTraitMethod("Show", "Nil", "show", func(args []evaluator.Object) evaluator.Object {
		return evaluator.StringToList("Nil")
	})
	vm.registerBuiltinTraitMethod("Show", "Function", "show", func(args []evaluator.Object) evaluator.Object {
		return evaluator.StringToList("<function>")
	})

	vm.registerBuiltinTraitMethod("Show", "Type", "show", func(args []evaluator.Object) evaluator.Object {
		return evaluator.StringToList(args[0].Inspect())
	})

	// Semigroup for List
	vm.registerBuiltinTraitMethod("Semigroup", "List", "(<>)", func(args []evaluator.Object) evaluator.Object {
		a, ok1 := args[0].(*evaluator.List)
		b, ok2 := args[1].(*evaluator.List)
		if ok1 && ok2 {
			return a.Concat(b)
		}
		return &evaluator.Error{Message: "Semigroup (<>) expects two Lists"}
	})

	// Semigroup for String (List Char concatenation)
	vm.registerBuiltinTraitMethod("Semigroup", "String", "(<>)", func(args []evaluator.Object) evaluator.Object {
		a, ok1 := args[0].(*evaluator.List)
		b, ok2 := args[1].(*evaluator.List)
		if ok1 && ok2 {
			return a.Concat(b)
		}
		return &evaluator.Error{Message: "Semigroup (<>) expects two Strings"}
	})

	// Semigroup for Option
	vm.registerBuiltinTraitMethod("Semigroup", "Option", "(<>)", func(args []evaluator.Object) evaluator.Object {
		a, ok1 := args[0].(*evaluator.DataInstance)
		b, ok2 := args[1].(*evaluator.DataInstance)
		if !ok1 || !ok2 {
			return &evaluator.Error{Message: "Semigroup (<>) expects Options"}
		}
		if a.Name == "Some" {
			return a
		}
		return b
	})

	// Monoid for List
	vm.registerBuiltinTraitMethod("Monoid", "List", "mempty", func(args []evaluator.Object) evaluator.Object {
		return evaluator.NewList([]evaluator.Object{})
	})

	// Monoid for Option
	vm.registerBuiltinTraitMethod("Monoid", "Option", "mempty", func(args []evaluator.Object) evaluator.Object {
		return &evaluator.DataInstance{Name: "None", TypeName: "Option"}
	})

	// Empty for Option
	vm.registerBuiltinTraitMethod("Empty", "Option", "isEmpty", func(args []evaluator.Object) evaluator.Object {
		if di, ok := args[0].(*evaluator.DataInstance); ok {
			return &evaluator.Boolean{Value: di.Name == "None"}
		}
		return &evaluator.Boolean{Value: false}
	})

	// Empty for Result
	vm.registerBuiltinTraitMethod("Empty", "Result", "isEmpty", func(args []evaluator.Object) evaluator.Object {
		if di, ok := args[0].(*evaluator.DataInstance); ok {
			return &evaluator.Boolean{Value: di.Name == "Fail"}
		}
		return &evaluator.Boolean{Value: false}
	})

	// Empty for List
	vm.registerBuiltinTraitMethod("Empty", "List", "isEmpty", func(args []evaluator.Object) evaluator.Object {
		if l, ok := args[0].(*evaluator.List); ok {
			return &evaluator.Boolean{Value: l.Len() == 0}
		}
		return &evaluator.Boolean{Value: false}
	})
}

// registerBuiltinTraitMethod registers a single builtin trait method
func (vm *VM) registerBuiltinTraitMethod(traitName, typeName, methodName string, fn func([]evaluator.Object) evaluator.Object) {
	bc := &BuiltinClosure{Name: traitName + "." + typeName + "." + methodName, Fn: fn}

	// traitMethods[traitName]
	var typeMap *PersistentMap
	if val := vm.traitMethods.Get(traitName); val != nil {
		typeMap = val.(*PersistentMap)
	} else {
		typeMap = EmptyMap()
	}

	// traitMethods[traitName][typeName]
	var methodMap *PersistentMap
	if val := typeMap.Get(typeName); val != nil {
		methodMap = val.(*PersistentMap)
	} else {
		methodMap = EmptyMap()
	}

	// We don't store the BuiltinClosure in traitMethods map structure (it stores ObjClosure)
	// But we need the map structure to exist so lookups can traverse it?
	// Actually LookupTraitMethod only looks for ObjClosure.
	// LookupTraitMethodAny checks traitMethods first, then builtinTraitMethods.
	// So we DO need to ensure the map structure exists in traitMethods if we want it to be "known"
	// that the trait exists for the type?
	// The original code did:
	// if vm.traitMethods[traitName] == nil { ... make map ... }
	// This was just to initialize the maps. It didn't put the 'bc' into it.
	// It put 'bc' into vm.builtinTraitMethods.
	// So we just need to ensure the nested PersistentMaps exist.

	typeMap = typeMap.Put(typeName, methodMap)
	vm.traitMethods = vm.traitMethods.Put(traitName, typeMap)

	vm.builtinTraitMethods = vm.builtinTraitMethods.Put(traitName+"."+typeName+"."+methodName, bc)
}

// importBuiltinsFromEnv imports builtins from an Environment
func (vm *VM) importBuiltinsFromEnv(env *evaluator.Environment) {
	builtins := evaluator.GetBuiltinsList()
	for name, obj := range builtins {
		vm.globals.Globals = vm.globals.Globals.Put(name, obj)
	}
}

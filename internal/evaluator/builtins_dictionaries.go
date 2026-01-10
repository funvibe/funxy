package evaluator

import (
	"github.com/funvibe/funxy/internal/config"
)

// TraitMethods defines the standard methods for built-in traits.
// This is used for dictionary construction and dynamic dispatch.
var TraitMethods = map[string][]string{
	"Show":        {"show"},
	"Equal":       {"(==)", "(!=)"},
	"Order":       {"(<)", "(>)", "(<=)", "(>=)"},
	"Numeric":     {"(+)", "(-)", "(*)", "(/)", "(%)", "(**)"},
	"Bitwise":     {"(&)", "(|)", "(^)", "(<<)", "(>>)"},
	"Concat":      {"(++)"},
	"Default":     {"default", "getDefault"}, // Both registered in analyzer
	"Functor":     {"fmap"},
	"Applicative": {"pure", "(<*>)"},
	"Monad":       {"(>>=)"},
	"Semigroup":   {"(<>)"},
	"Monoid":      {"mempty"},
	"Empty":       {"isEmpty"},
	"Optional":    {"unwrap", "wrap"},
	"Iter":        {"iter"},
}

// RegisterDictionaryGlobals populates the environment with dictionary objects
// ($impl_Trait_Type or $ctor_Trait_Type) for all registered class implementations.
// This bridges the gap between the legacy ClassImplementations map and the new
// dictionary-passing runtime.
func RegisterDictionaryGlobals(e *Evaluator, env *Environment) {
	// 1. Map of Trait -> Ordered Methods (Names) matching analyzer/builtins.go
	// Used from package-level variable TraitMethods

	// 2. Iterate over all registered implementations
	for traitName, types := range e.ClassImplementations {
		// Get expected method order
		expectedMethods, ok := TraitMethods[traitName]
		if !ok {
			// Skip user-defined traits not in our hardcoded list?
			// Or should we try to infer? For now, skip to avoid crashes.
			// fmt.Printf("Warning: Unknown trait %s in ClassImplementations\n", traitName)
			continue
		}

		// Get trait info for Kind check
		traitInfo := config.GetTraitInfo(traitName)
		isHKT := false
		if traitInfo != nil && traitInfo.Kind != "*" {
			isHKT = true
		}

		for typeName, methodTable := range types {
			// Determine if we need a Constructor ($ctor) or Constant ($impl)
			// Rule:
			// - If Trait is *, and Type is a Container (Generic), use Constructor.
			// - Otherwise (Trait is * and Type is Primitive, OR Trait is HKT), use Constant.

			// We identify "Container" by checking if typeName is in our known HKT list
			isContainer := false
			switch typeName {
			case "List", "Option", "Result", "Map", "State", "Reader", "Writer", "Identity", "OptionT", "ResultT":
				isContainer = true
			}

			// Special case for String (List<Char>): it acts like a primitive for * traits
			if typeName == "String" {
				isContainer = false
			}

			// Special case for Tuple, Bytes, Bits: they are * types but can be treated as primitives here
			// Tuple is generic though?
			// Analyzer registers Tuple as constant for fixed arity?
			// "reg(table, "Show", tupleType)" in analyzer/builtins.go
			// tupleType is TTuple.
			// Check reg(): if typesystem.TTuple, it uses t.String() as key -> "Tuple" is not the key, it's "(...)"
			// But ClassImplementations uses keys like "List", "Int".
			// For Tuple, we might have multiple implementations?
			// Evaluator doesn't really implement Tuple explicitly in ClassImplementations yet?
			// Wait, builtins_std.go:
			// "Tuple" is in builtinTypes list for Show!
			// So typeName is "Tuple".
			// But Tuple is generic. Show Tuple depends on Show components.
			// So it should be a $ctor_Show_Tuple.
			// Currently evaluator registers it as "Tuple" and uses defaultShowMethod which uses Inspect.
			// Inspect handles recursion.
			// So for Evaluator purposes, we can treat it as a Constant $impl_Show_Tuple that uses runtime reflection (Inspect).
			// This matches what we have: a single MethodTable for "Tuple".

			shouldBeConstructor := !isHKT && isContainer

			// However, if we already have a fully resolved implementation (MethodTable)
			// that works for ANY instance of the type (using reflection or generic logic),
			// we can expose it as a CONSTANT dictionary even if it "should" be a constructor.
			// The only reason to be a constructor is if we need sub-dictionaries (witnesses).

			// If our implementation uses Inspect/reflection (like Show List), it doesn't need sub-dictionaries.
			// It just works.
			// So we can expose it as $impl_Show_List directly?
			// BUT the Analyzer expects $ctor_Show_List for List<T>.
			// Rewrite: $ctor_Show_List(dict_T_Show).
			// If we provide a constant $impl_Show_List, the runtime lookups will fail if code calls $ctor.

			// So we MUST provide $ctor_Show_List.
			// This constructor will take 1 argument (the inner dict), ignore it (since we use reflection),
			// and return the Dictionary.

			var globalName string
			if shouldBeConstructor {
				globalName = "$ctor_" + traitName + "_" + typeName
			} else {
				globalName = "$impl_" + traitName + "_" + typeName
			}

			// Build the Dictionary Object
			// 1. Methods
			methods := make([]Object, len(expectedMethods))
			if methodTable, ok := methodTable.(*MethodTable); ok {
				for i, mName := range expectedMethods {
					if m, found := methodTable.Methods[mName]; found {
						methods[i] = m
					} else {
						// Missing method? Check operator mapping?
						// expectedMethods includes operators like "(==)".
						// ClassImplementations should have them keyed by "(==)".
						// If missing, maybe fall back to "equal"? No, builtins_std uses correct names.
						// panic(fmt.Sprintf("Missing method %s for %s %s", mName, traitName, typeName))
						// Just fill with Error to avoid panic during registration?
						methods[i] = newError("Method %s not implemented for %s %s", mName, traitName, typeName)
					}
				}
			} else {
				// Should not happen if registered correctly using MethodTable
				for i := range methods {
					methods[i] = newError("Invalid method table for %s %s", traitName, typeName)
				}
			}

			// 2. Supers
			// We need to construct super dictionaries.
			// This is hard because we don't have them easily available in ClassImplementations.
			// But for built-ins, we can rely on `lookupTraitMethod` to find them if we leave Supers empty?
			// No, the dictionary object MUST contain supers if the trait has supertraits.
			// The `lookupTraitMethod` logic in `evaluator.go` iterates super traits from config.
			// BUT `SolveWitness` generates code that accesses `.Supers[i]`.
			// So the Dictionary object MUST have `Supers` populated.

			// Example: Order implies Equal.
			// $impl_Order_Int must have Supers: [$impl_Equal_Int].

			// We can lookup the super dictionary from `env`!
			// Since we register in a loop, we might have order dependencies.
			// But "Int" implementations are likely registered all at once.
			// Or we can lazy load? No, objects are values.

			// We can resolve supers dynamically or do multiple passes.
			// Or simpler: We know standard types have all standard traits.
			// We can fetch the super implementation from e.ClassImplementations!

			supers := resolveSupers(e, traitName, typeName, isHKT)

			dict := &Dictionary{
				TraitName: traitName,
				Methods:   methods,
				Supers:    supers,
			}

			// Register in Env
			if shouldBeConstructor {
				// Create a constructor function that returns this dictionary
				// It ignores arguments because our built-in implementations use reflection/builtins
				// and don't rely on the passed inner dictionaries.
				ctor := &Builtin{
					Name: globalName,
					Fn: func(ev *Evaluator, args ...Object) Object {
						// Return the pre-built dictionary
						// Note: for some cases (like Tuple?), we might want to compose dictionaries?
						// But for List, Show uses Inspect which doesn't need inner dict.
						// Equal for List DOES need inner Equal?
						// `builtins_std.go`: `equal` for List calls `EvalInfixExpression("==", ...)`
						// which recurses via `CompareValues`.
						// `CompareValues` uses `EvalInfixExpression` -> type based dispatch.
						// So it implicitly uses the global/default resolution, NOT the passed dictionary.
						// This is a hybrid approach:
						// - Dictionary passing at top level.
						// - Reflection/Global dispatch at recursion level (in builtins).
						// This works for now.
						return dict
					},
				}
				env.Set(globalName, ctor)
			} else {
				env.Set(globalName, dict)
			}
		}
	}
}

func resolveSupers(e *Evaluator, traitName, typeName string, isHKT bool) []*Dictionary {
	traitInfo := config.GetTraitInfo(traitName)
	if traitInfo == nil || len(traitInfo.SuperTraits) == 0 {
		return []*Dictionary{}
	}

	supers := make([]*Dictionary, len(traitInfo.SuperTraits))
	for i, superName := range traitInfo.SuperTraits {
		// Find implementation for super trait
		// We can reuse the same logic to find/construct the dictionary
		// Since we are inside the loop, we might need to construct it on demand or look it up.

		// Ideally we look up in e.ClassImplementations again.
		if types, ok := e.ClassImplementations[superName]; ok {
			if _, ok := types[typeName]; ok {
				// Found it. But we need the Dictionary object.
				// Since we haven't finished registering, it might not be in env.
				// But we can RECURSIVELY call a helper to build the dictionary.
				// To avoid infinite recursion, we assume DAG hierarchy (valid traits).

				// But wait, we need methods and supers for the super trait.
				// Let's refactor `buildDictionary` into a helper.
				supers[i] = buildDictionary(e, superName, typeName)
			}
		}
	}
	return supers
}

func buildDictionary(e *Evaluator, traitName, typeName string) *Dictionary {
	// Re-implement the construction logic for a single dictionary
	// (Simplified copy of the loop body)

	// 1. Methods
	expectedMethods, ok := TraitMethods[traitName]
	if !ok {
		return &Dictionary{TraitName: traitName} // Empty
	}

	types, ok := e.ClassImplementations[traitName]
	if !ok {
		return &Dictionary{TraitName: traitName}
	}
	methodTableObj, ok := types[typeName]
	if !ok {
		return &Dictionary{TraitName: traitName}
	}

	methodTable, ok := methodTableObj.(*MethodTable)
	if !ok {
		return &Dictionary{TraitName: traitName}
	}

	methods := make([]Object, len(expectedMethods))
	for i, mName := range expectedMethods {
		if m, found := methodTable.Methods[mName]; found {
			methods[i] = m
		} else {
			methods[i] = newError("Method %s not implemented", mName)
		}
	}

	// 2. Supers
	// Recurse
	traitInfo := config.GetTraitInfo(traitName)
	var supers []*Dictionary
	if traitInfo != nil {
		supers = make([]*Dictionary, len(traitInfo.SuperTraits))
		for i, superName := range traitInfo.SuperTraits {
			supers[i] = buildDictionary(e, superName, typeName)
		}
	}

	return &Dictionary{
		TraitName: traitName,
		Methods:   methods,
		Supers:    supers,
	}
}

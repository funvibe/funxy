package analyzer

import (
	"fmt"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/token"
	"github.com/funvibe/funxy/internal/typesystem"
	"strings"
	"unicode"
)

// GetEvidenceKey generates a canonical key for a trait instance (concrete or generic).
// It encodes the trait name and the head constructors of all type arguments.
// Examples:
// - Convert<Int, String> -> Convert[Int,String]
// - Show List<T> -> Show[List]
// - Functor Map<K, V> -> Functor[Map]
func GetEvidenceKey(traitName string, args []typesystem.Type) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = getHeadConstructorName(arg)
	}
	return fmt.Sprintf("%s[%s]", traitName, strings.Join(parts, ","))
}

func getHeadConstructorName(t typesystem.Type) string {
	t = typesystem.UnwrapUnderlying(t) // Handle aliases

	switch typ := t.(type) {
	case typesystem.TCon:
		return typ.Name
	case typesystem.TApp:
		return getHeadConstructorName(typ.Constructor)
	case typesystem.TTuple:
		return "TUPLE"
	case typesystem.TRecord:
		return "RECORD"
	case typesystem.TFunc:
		return "FUNCTION"
	case typesystem.TVar:
		return typ.Name
	default:
		// Best effort for other types
		return typ.String()
	}
}

// isValueName checks if a name follows value naming convention (starts with lowercase or _)
// Also allows operator methods like (==), (+), etc.
func isValueName(name string) bool {
	if len(name) == 0 {
		return false
	}
	first := rune(name[0])
	// Allow operators in parens: (==), (+), etc.
	if first == '(' {
		return true
	}
	return unicode.IsLower(first) || first == '_'
}

// isTypeName checks if a name follows type naming convention (starts with uppercase)
func isTypeName(name string) bool {
	if len(name) == 0 {
		return false
	}
	return unicode.IsUpper(rune(name[0]))
}

// checkValueName validates that a name follows value naming convention
// and appends an error if not
func checkValueName(name string, tok token.Token, errors *[]*diagnostics.DiagnosticError) bool {
	if !isValueName(name) {
		*errors = append(*errors, diagnostics.NewAnalyzerError(
			diagnostics.ErrA008, tok,
			"value '"+name+"' must start with lowercase letter or underscore",
		))
		return false
	}
	return true
}

// checkTypeName validates that a name follows type naming convention
// and appends an error if not
func checkTypeName(name string, tok token.Token, errors *[]*diagnostics.DiagnosticError) bool {
	if !isTypeName(name) {
		*errors = append(*errors, diagnostics.NewAnalyzerError(
			diagnostics.ErrA008, tok,
			"type '"+name+"' must start with uppercase letter",
		))
		return false
	}
	return true
}

// GetDictionaryName generates the global name for a dictionary constant.
// Format: $impl_Trait_Type
func GetDictionaryName(traitName, typeName string) string {
	return "$impl_" + traitName + "_" + typeName
}

// GetDictionaryConstructorName generates the global name for a dictionary constructor function.
// Format: $ctor_Trait_Type
func GetDictionaryConstructorName(traitName, typeName string) string {
	return "$ctor_" + traitName + "_" + typeName
}

// GetWitnessParamName generates the name for a hidden dictionary parameter.
// Format: $dict_T_Trait
func GetWitnessParamName(typeVar, traitName string) string {
	return "$dict_" + typeVar + "_" + traitName
}

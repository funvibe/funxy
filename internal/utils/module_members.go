package utils

import (
	"unicode"
	"unicode/utf8"
)

// ModuleMemberFallbackName maps a short member name to a verbose stdlib name.
// Example: moduleName="string", member="toUpper" -> "stringToUpper".
func ModuleMemberFallbackName(moduleName, member string) string {
	if moduleName == "" || member == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(member)
	if r == utf8.RuneError && size == 0 {
		return ""
	}
	upper := unicode.ToUpper(r)
	if upper == r {
		return moduleName + member
	}
	return moduleName + string(upper) + member[size:]
}

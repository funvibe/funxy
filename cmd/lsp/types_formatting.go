package main

import (
	"fmt"
	"github.com/funvibe/funxy/internal/typesystem"
	"sort"
	"strconv"
	"strings"
)

func isInternalName(name string) bool {
	if strings.HasPrefix(name, "t") {
		if _, err := strconv.Atoi(name[1:]); err == nil {
			return true
		}
	}
	if strings.HasPrefix(name, "gen_t") {
		if _, err := strconv.Atoi(name[5:]); err == nil {
			return true
		}
	}
	if strings.HasPrefix(name, "$skolem_") {
		return true
	}
	if strings.HasPrefix(name, "_pending_") {
		return true
	}
	return false
}

// PrettifyType normalizes type variables for display in LSP hover.
// It renames variables to a, b, c... z, t1, t2... based on order of appearance.
// It also unwraps top-level TForall since LSP mode hides the quantifier anyway.
func PrettifyType(t typesystem.Type) string {
	if t == nil {
		return ""
	}

	// Unwrap TForall to access inner type and variables
	// We want to rename bound variables too.
	// Note: We currently lose constraints attached to TForall in LSP mode (same as existing behavior)
	current := t
	for {
		if forall, ok := current.(typesystem.TForall); ok {
			current = forall.Type
		} else {
			break
		}
	}

	// 1. Collect all TVars in order of appearance
	vars := make([]string, 0)
	seen := make(map[string]bool)

	var collect func(typesystem.Type)
	collect = func(t typesystem.Type) {
		if t == nil {
			return
		}
		switch typ := t.(type) {
		case typesystem.TCon:
			// Treat TCons that look like auto-generated variables (t123, gen_t..., $skolem...) as variables to be prettified
			if isInternalName(typ.Name) {
				if !seen[typ.Name] {
					seen[typ.Name] = true
					vars = append(vars, typ.Name)
				}
			}
		case typesystem.TVar:
			if !seen[typ.Name] {
				seen[typ.Name] = true
				vars = append(vars, typ.Name)
			}
		case typesystem.TApp:
			collect(typ.Constructor)
			for _, arg := range typ.Args {
				collect(arg)
			}
		case typesystem.TFunc:
			for _, p := range typ.Params {
				collect(p)
			}
			collect(typ.ReturnType)
		case typesystem.TRecord:
			// Sort keys for deterministic order
			keys := make([]string, 0, len(typ.Fields))
			for k := range typ.Fields {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				collect(typ.Fields[k])
			}
			if typ.Row != nil {
				collect(typ.Row)
			}
		case typesystem.TForall:
			collect(typ.Type)
		case typesystem.TUnion:
			for _, sub := range typ.Types {
				collect(sub)
			}
		case typesystem.TTuple:
			for _, sub := range typ.Elements {
				collect(sub)
			}
		case typesystem.TType:
			collect(typ.Type)
		}
	}

	collect(current)

	// 2. Generate mappings
	subst := make(typesystem.Subst)

	nextName := func(idx int) string {
		if idx < 26 {
			return string('a' + byte(idx))
		}
		return fmt.Sprintf("t%d", idx-26+1)
	}

	currentIdx := 0
	for _, originalName := range vars {
		newName := nextName(currentIdx)
		currentIdx++
		// Map to TVar regardless of whether source was TVar or TCon
		subst[originalName] = typesystem.TVar{Name: newName}
	}

	// 3. Apply substitution
	newType := current.Apply(subst)
	return newType.String()
}

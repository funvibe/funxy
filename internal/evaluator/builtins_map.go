package evaluator

import (
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/typesystem"
)

// GetMapBuiltins returns the map of map-related built-in functions
func GetMapBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"mapNew":        {Fn: builtinMapNew, Name: "mapNew"},
		"mapFromRecord": {Fn: builtinMapFromRecord, Name: "mapFromRecord"},
		"mapGet":        {Fn: builtinMapGet, Name: "mapGet"},
		"mapGetOr":      {Fn: builtinMapGetOr, Name: "mapGetOr"},
		"mapPut":        {Fn: builtinMapPut, Name: "mapPut"},
		"mapRemove":     {Fn: builtinMapRemove, Name: "mapRemove"},
		"mapKeys":       {Fn: builtinMapKeys, Name: "mapKeys"},
		"mapValues":     {Fn: builtinMapValues, Name: "mapValues"},
		"mapItems":      {Fn: builtinMapItems, Name: "mapItems"},
		"mapContains":   {Fn: builtinMapContains, Name: "mapContains"},
		"mapSize":       {Fn: builtinMapSize, Name: "mapSize"},
		"mapMerge":      {Fn: builtinMapMerge, Name: "mapMerge"},
	}
}

// mapNew: () -> Map<K, V>
func builtinMapNew(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("mapNew expects 0 arguments")
	}
	return newMap()
}

// mapFromRecord: (Record) -> Map<String, Any>
func builtinMapFromRecord(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("mapFromRecord expects 1 argument")
	}
	rec, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("mapFromRecord expects a Record, got %s", args[0].Type())
	}

	m := newMap()
	for _, field := range rec.Fields {
		// Key is string, convert to List<Char>
		key := stringToList(field.Key)
		m = m.put(key, field.Value)
	}
	return m
}

// mapGet: (Map<K, V>, K) -> Option<V>
func builtinMapGet(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("mapGet expects 2 arguments, got %d", len(args))
	}
	m, ok := args[0].(*Map)
	if !ok {
		return newError("mapGet expects a Map as first argument, got %s", args[0].Type())
	}
	key := args[1]
	val := m.get(key)
	if val == nil {
		return makeNone() // None
	}
	return makeSome(val) // Some(value)
}

// mapGetOr: (Map<K, V>, K, V) -> V
func builtinMapGetOr(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("mapGetOr expects 3 arguments, got %d", len(args))
	}
	m, ok := args[0].(*Map)
	if !ok {
		return newError("mapGetOr expects a Map as first argument, got %s", args[0].Type())
	}
	key := args[1]
	defaultVal := args[2]
	val := m.get(key)
	if val == nil {
		return defaultVal
	}
	return val
}

// mapPut: (Map<K, V>, K, V) -> Map<K, V>
func builtinMapPut(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("mapPut expects 3 arguments, got %d", len(args))
	}
	m, ok := args[0].(*Map)
	if !ok {
		return newError("mapPut expects a Map as first argument, got %s", args[0].Type())
	}
	key := args[1]
	value := args[2]
	return m.put(key, value)
}

// mapRemove: (Map<K, V>, K) -> Map<K, V>
func builtinMapRemove(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("mapRemove expects 2 arguments, got %d", len(args))
	}
	m, ok := args[0].(*Map)
	if !ok {
		return newError("mapRemove expects a Map as first argument, got %s", args[0].Type())
	}
	key := args[1]
	return m.remove(key)
}

// mapKeys: (Map<K, V>) -> List<K>
func builtinMapKeys(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("mapKeys expects 1 argument, got %d", len(args))
	}
	m, ok := args[0].(*Map)
	if !ok {
		return newError("mapKeys expects a Map, got %s", args[0].Type())
	}
	return m.keys()
}

// mapValues: (Map<K, V>) -> List<V>
func builtinMapValues(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("mapValues expects 1 argument, got %d", len(args))
	}
	m, ok := args[0].(*Map)
	if !ok {
		return newError("mapValues expects a Map, got %s", args[0].Type())
	}
	return m.values()
}

// mapItems: (Map<K, V>) -> List<(K, V)>
func builtinMapItems(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("mapItems expects 1 argument, got %d", len(args))
	}
	m, ok := args[0].(*Map)
	if !ok {
		return newError("mapItems expects a Map, got %s", args[0].Type())
	}
	return m.items()
}

// mapContains: (Map<K, V>, K) -> Bool
func builtinMapContains(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("mapContains expects 2 arguments, got %d", len(args))
	}
	m, ok := args[0].(*Map)
	if !ok {
		return newError("mapContains expects a Map as first argument, got %s", args[0].Type())
	}
	key := args[1]
	if m.contains(key) {
		return TRUE
	}
	return FALSE
}

// mapSize: (Map<K, V>) -> Int
func builtinMapSize(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("mapSize expects 1 argument, got %d", len(args))
	}
	m, ok := args[0].(*Map)
	if !ok {
		return newError("mapSize expects a Map, got %s", args[0].Type())
	}
	return &Integer{Value: int64(m.len())}
}

// mapMerge: (Map<K, V>, Map<K, V>) -> Map<K, V>
func builtinMapMerge(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("mapMerge expects 2 arguments, got %d", len(args))
	}
	m1, ok := args[0].(*Map)
	if !ok {
		return newError("mapMerge expects a Map as first argument, got %s", args[0].Type())
	}
	m2, ok := args[1].(*Map)
	if !ok {
		return newError("mapMerge expects a Map as second argument, got %s", args[1].Type())
	}
	return m1.merge(m2)
}

// SetMapBuiltinTypes sets type information for map builtins
func SetMapBuiltinTypes(builtins map[string]*Builtin) {
	K := typesystem.TVar{Name: "K"}
	V := typesystem.TVar{Name: "V"}

	mapKV := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.MapTypeName},
		Args:        []typesystem.Type{K, V},
	}

	// For mapFromRecord: Map<String, V>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{typesystem.Char},
	}
	mapStringV := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.MapTypeName},
		Args:        []typesystem.Type{stringType, V},
	}
	// We don't have a specific Record type in Type system that matches "any record",
	// so we might use a generic or a special check. For now, use Any or specific Record logic in analyzer.
	// Using TVar "R" for record.
	recordType := typesystem.TVar{Name: "R"}

	optionV := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.OptionTypeName},
		Args:        []typesystem.Type{V},
	}

	listK := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{K},
	}

	listV := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{V},
	}

	pairKV := typesystem.TTuple{Elements: []typesystem.Type{K, V}}
	listPairs := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{pairKV},
	}

	types := map[string]typesystem.Type{
		"mapNew":        typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: mapKV},
		"mapFromRecord": typesystem.TFunc{Params: []typesystem.Type{recordType}, ReturnType: mapStringV},
		"mapGet":        typesystem.TFunc{Params: []typesystem.Type{mapKV, K}, ReturnType: optionV},
		"mapGetOr":      typesystem.TFunc{Params: []typesystem.Type{mapKV, K, V}, ReturnType: V},
		"mapPut":        typesystem.TFunc{Params: []typesystem.Type{mapKV, K, V}, ReturnType: mapKV},
		"mapRemove":     typesystem.TFunc{Params: []typesystem.Type{mapKV, K}, ReturnType: mapKV},
		"mapKeys":       typesystem.TFunc{Params: []typesystem.Type{mapKV}, ReturnType: listK},
		"mapValues":     typesystem.TFunc{Params: []typesystem.Type{mapKV}, ReturnType: listV},
		"mapItems":      typesystem.TFunc{Params: []typesystem.Type{mapKV}, ReturnType: listPairs},
		"mapContains":   typesystem.TFunc{Params: []typesystem.Type{mapKV, K}, ReturnType: typesystem.Bool},
		"mapSize":       typesystem.TFunc{Params: []typesystem.Type{mapKV}, ReturnType: typesystem.Int},
		"mapMerge":      typesystem.TFunc{Params: []typesystem.Type{mapKV, mapKV}, ReturnType: mapKV},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

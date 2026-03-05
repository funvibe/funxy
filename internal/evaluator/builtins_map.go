package evaluator

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
		"mapFold":       {Fn: builtinMapFold, Name: "mapFold"},
		"mapMap":        {Fn: builtinMapMap, Name: "mapMap"},
		"mapFilter":     {Fn: builtinMapFilter, Name: "mapFilter"},
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

// mapFold: ((U, K, V) -> U, U, Map<K, V>) -> U
func builtinMapFold(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("mapFold expects 3 arguments, got %d", len(args))
	}
	foldFn := args[0]
	acc := args[1]
	m, ok := args[2].(*Map)
	if !ok {
		return newError("mapFold expects a Map as third argument, got %s", args[2].Type())
	}

	for _, item := range m.Items() {
		result := e.ApplyFunction(foldFn, []Object{acc, item.Key, item.Value})
		if isError(result) {
			return result
		}
		acc = result
	}
	return acc
}

// mapMap: ((K, V) -> V2, Map<K, V>) -> Map<K, V2>
func builtinMapMap(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("mapMap expects 2 arguments, got %d", len(args))
	}
	mapFn := args[0]
	m, ok := args[1].(*Map)
	if !ok {
		return newError("mapMap expects a Map as second argument, got %s", args[1].Type())
	}

	newM := newMap()
	for _, item := range m.Items() {
		mappedVal := e.ApplyFunction(mapFn, []Object{item.Key, item.Value})
		if isError(mappedVal) {
			return mappedVal
		}
		// Since we're building a new map internally in Go, using put is very efficient
		newM = newM.put(item.Key, mappedVal)
	}
	return newM
}

// mapFilter: ((K, V) -> Bool, Map<K, V>) -> Map<K, V>
func builtinMapFilter(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("mapFilter expects 2 arguments, got %d", len(args))
	}
	predFn := args[0]
	m, ok := args[1].(*Map)
	if !ok {
		return newError("mapFilter expects a Map as second argument, got %s", args[1].Type())
	}

	newM := newMap()
	for _, item := range m.Items() {
		predResult := e.ApplyFunction(predFn, []Object{item.Key, item.Value})
		if isError(predResult) {
			return predResult
		}
		if boolResult, isBool := predResult.(*Boolean); isBool && boolResult.Value {
			newM = newM.put(item.Key, item.Value)
		}
	}
	return newM
}

// SetMapBuiltinTypes sets type information for map builtins

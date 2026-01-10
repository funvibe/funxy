package evaluator

import (
	"github.com/funvibe/funxy/internal/typesystem"
	"sort"
)

// ListBuiltins returns built-in functions for lib/list virtual package
func ListBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"head":      {Fn: builtinHead, Name: "head"},
		"headOr":    {Fn: builtinHeadOr, Name: "headOr"},
		"last":      {Fn: builtinLast, Name: "last"},
		"lastOr":    {Fn: builtinLastOr, Name: "lastOr"},
		"nth":       {Fn: builtinNth, Name: "nth"},
		"nthOr":     {Fn: builtinNthOr, Name: "nthOr"},
		"tail":      {Fn: builtinTail, Name: "tail"},
		"init":      {Fn: builtinInit, Name: "init"},
		"take":      {Fn: builtinTake, Name: "take"},
		"drop":      {Fn: builtinDrop, Name: "drop"},
		"slice":     {Fn: builtinSlice, Name: "slice"},
		"length":    {Fn: builtinLength, Name: "length"},
		"contains":  {Fn: builtinContains, Name: "contains"},
		"filter":    {Fn: builtinFilter, Name: "filter"},
		"map":       {Fn: builtinMap, Name: "map"},
		"foldl":     {Fn: builtinFoldl, Name: "foldl"},
		"foldr":     {Fn: builtinFoldr, Name: "foldr"},
		"indexOf":   {Fn: builtinIndexOf, Name: "indexOf"},
		"reverse":   {Fn: builtinReverse, Name: "reverse"},
		"concat":    {Fn: builtinConcat, Name: "concat"},
		"flatten":   {Fn: builtinFlatten, Name: "flatten"},
		"unique":    {Fn: builtinUnique, Name: "unique"},
		"zip":       {Fn: builtinZip, Name: "zip"},
		"unzip":     {Fn: builtinUnzip, Name: "unzip"},
		"sort":      {Fn: builtinSort, Name: "sort"},
		"sortBy":    {Fn: builtinSortBy, Name: "sortBy"},
		"range":     {Fn: builtinRange, Name: "range"},
		"find":      {Fn: builtinFind, Name: "find"},
		"findIndex": {Fn: builtinFindIndex, Name: "findIndex"},
		"any":       {Fn: builtinAny, Name: "any"},
		"all":       {Fn: builtinAll, Name: "all"},
		"takeWhile": {Fn: builtinTakeWhile, Name: "takeWhile"},
		"dropWhile": {Fn: builtinDropWhile, Name: "dropWhile"},
		"partition": {Fn: builtinPartition, Name: "partition"},
		"forEach":   {Fn: builtinForEach, Name: "forEach"},
		"append":    {Fn: builtinAppend, Name: "append"},
	}
}

// head: List<T> -> T (panics if empty)
func builtinHead(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("head expects 1 argument, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("head expects a list, got %s", args[0].Type())
	}
	if list.len() == 0 {
		return newError("head: empty list")
	}
	return list.get(0)
}

// headOr: (List<T>, T) -> T
func builtinHeadOr(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("headOr expects 2 arguments, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("headOr expects a list as first argument, got %s", args[0].Type())
	}
	if list.len() == 0 {
		return args[1]
	}
	return list.get(0)
}

// last: List<T> -> T (panics if empty)
func builtinLast(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("last expects 1 argument, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("last expects a list, got %s", args[0].Type())
	}
	if list.len() == 0 {
		return newError("last: empty list")
	}
	return list.get(list.len() - 1)
}

// lastOr: (List<T>, T) -> T
func builtinLastOr(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("lastOr expects 2 arguments, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("lastOr expects a list as first argument, got %s", args[0].Type())
	}
	if list.len() == 0 {
		return args[1]
	}
	return list.get(list.len() - 1)
}

// nth: (List<T>, Int) -> T (panics if out of bounds)
func builtinNth(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("nth expects 2 arguments, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("nth expects a list as first argument, got %s", args[0].Type())
	}
	idx, ok := args[1].(*Integer)
	if !ok {
		return newError("nth expects an integer as second argument, got %s", args[1].Type())
	}
	n := int(idx.Value)
	if n < 0 || n >= list.len() {
		return newError("nth: index %d out of bounds for list of length %d", n, list.len())
	}
	return list.get(n)
}

// nthOr: (List<T>, Int, T) -> T
func builtinNthOr(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("nthOr expects 3 arguments, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("nthOr expects a list as first argument, got %s", args[0].Type())
	}
	idx, ok := args[1].(*Integer)
	if !ok {
		return newError("nthOr expects an integer as second argument, got %s", args[1].Type())
	}
	n := int(idx.Value)
	if n < 0 || n >= list.len() {
		return args[2]
	}
	return list.get(n)
}

// tail: List<T> -> List<T> (panics if empty)
func builtinTail(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("tail expects 1 argument, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("tail expects a list, got %s", args[0].Type())
	}
	if list.len() == 0 {
		return newError("tail: empty list")
	}
	return list.Slice(1, list.len())
}

// init: List<T> -> List<T> (panics if empty)
func builtinInit(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("init expects 1 argument, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("init expects a list, got %s", args[0].Type())
	}
	if list.len() == 0 {
		return newError("init: empty list")
	}
	return list.Slice(0, list.len()-1)
}

// take: (List<T>, Int) -> List<T>
func builtinTake(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("take expects 2 arguments, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("take expects a list as first argument, got %s", args[0].Type())
	}
	n, ok := args[1].(*Integer)
	if !ok {
		return newError("take expects an integer as second argument, got %s", args[1].Type())
	}
	count := int(n.Value)
	if count < 0 {
		count = 0
	}
	if count > list.len() {
		count = list.len()
	}
	return list.Slice(0, count)
}

// drop: (List<T>, Int) -> List<T>
func builtinDrop(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("drop expects 2 arguments, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("drop expects a list as first argument, got %s", args[0].Type())
	}
	n, ok := args[1].(*Integer)
	if !ok {
		return newError("drop expects an integer as second argument, got %s", args[1].Type())
	}
	count := int(n.Value)
	if count < 0 {
		count = 0
	}
	if count > list.len() {
		count = list.len()
	}
	return list.Slice(count, list.len())
}

// slice: (List<T>, Int, Int) -> List<T> (panics if out of bounds)
func builtinSlice(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("slice expects 3 arguments, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("slice expects a list as first argument, got %s", args[0].Type())
	}
	fromArg, ok := args[1].(*Integer)
	if !ok {
		return newError("slice expects an integer as second argument, got %s", args[1].Type())
	}
	toArg, ok := args[2].(*Integer)
	if !ok {
		return newError("slice expects an integer as third argument, got %s", args[2].Type())
	}
	from := int(fromArg.Value)
	to := int(toArg.Value)
	length := list.len()

	if from < 0 || to > length || from > to {
		return newError("slice: indices [%d, %d) out of bounds for list of length %d", from, to, length)
	}
	return list.Slice(from, to)
}

// length: List<T> -> Int
func builtinLength(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("length expects 1 argument, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("length expects a list, got %s", args[0].Type())
	}
	return &Integer{Value: int64(list.len())}
}

// contains: (List<T>, T) -> Bool
func builtinContains(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("contains expects 2 arguments, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("contains expects a list as first argument, got %s", args[0].Type())
	}
	elem := args[1]
	for _, item := range list.ToSlice() {
		if objectsEqual(item, elem) {
			return TRUE
		}
	}
	return FALSE
}

// filter: ((T) -> Bool, List<T>) -> List<T>
func builtinFilter(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("filter expects 2 arguments, got %d", len(args))
	}
	predFn := args[0]
	list, ok := args[1].(*List)
	if !ok {
		return newError("filter expects a list as second argument, got %s", args[1].Type())
	}

	var result []Object
	for _, item := range list.ToSlice() {
		predicateResult := e.ApplyFunction(predFn, []Object{item})
		if isError(predicateResult) {
			return predicateResult
		}
		if boolResult, ok := predicateResult.(*Boolean); ok && boolResult.Value {
			result = append(result, item)
		}
	}
	return newList(result)
}

// map: ((T) -> U, List<T>) -> List<U>
func builtinMap(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("map expects 2 arguments, got %d", len(args))
	}
	mapFn := args[0]
	list, ok := args[1].(*List)
	if !ok {
		return newError("map expects a list as second argument, got %s", args[1].Type())
	}

	result := make([]Object, list.len())
	for i, item := range list.ToSlice() {
		mapped := e.ApplyFunction(mapFn, []Object{item})
		if isError(mapped) {
			return mapped
		}
		result[i] = mapped
	}
	return newList(result)
}

// foldl: ((U, T) -> U, U, List<T>) -> U
// Left fold: foldl((+), 0, [1,2,3]) = ((0+1)+2)+3
func builtinFoldl(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("foldl expects 3 arguments, got %d", len(args))
	}
	foldFn := args[0]
	acc := args[1]
	list, ok := args[2].(*List)
	if !ok {
		return newError("foldl expects a list as third argument, got %s", args[2].Type())
	}

	for _, item := range list.ToSlice() {
		result := e.ApplyFunction(foldFn, []Object{acc, item})
		if isError(result) {
			return result
		}
		acc = result
	}
	return acc
}

// foldr: ((T, U) -> U, U, List<T>) -> U
// Right fold: foldr((+), 0, [1,2,3]) = 1+(2+(3+0))
func builtinFoldr(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("foldr expects 3 arguments, got %d", len(args))
	}
	foldFn := args[0]
	acc := args[1]
	list, ok := args[2].(*List)
	if !ok {
		return newError("foldr expects a list as third argument, got %s", args[2].Type())
	}

	// Iterate from right to left
	for i := list.len() - 1; i >= 0; i-- {
		result := e.ApplyFunction(foldFn, []Object{list.get(i), acc})
		if isError(result) {
			return result
		}
		acc = result
	}
	return acc
}

// indexOf: (List<T>, T) -> Option<Int>
func builtinIndexOf(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("indexOf expects 2 arguments, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("indexOf expects a list as first argument, got %s", args[0].Type())
	}
	elem := args[1]
	for i, item := range list.ToSlice() {
		if objectsEqual(item, elem) {
			// Return Some(index)
			return &DataInstance{
				Name:     "Some",
				Fields:   []Object{&Integer{Value: int64(i)}},
				TypeName: "Option",
			}
		}
	}
	// Return Zero (not found)
	return &DataInstance{
		Name:     "Zero",
		Fields:   []Object{},
		TypeName: "Option",
	}
}

// reverse: List<T> -> List<T>
func builtinReverse(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("reverse expects 1 argument, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("reverse expects a list, got %s", args[0].Type())
	}
	n := list.len()
	result := make([]Object, n)
	for i, elem := range list.ToSlice() {
		result[n-1-i] = elem
	}
	return newList(result)
}

// concat: (List<T>, List<T>) -> List<T>
func builtinConcat(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("concat expects 2 arguments, got %d", len(args))
	}
	list1, ok := args[0].(*List)
	if !ok {
		return newError("concat expects a list as first argument, got %s", args[0].Type())
	}
	list2, ok := args[1].(*List)
	if !ok {
		return newError("concat expects a list as second argument, got %s", args[1].Type())
	}
	result := make([]Object, 0, list1.len()+list2.len())
	result = append(result, list1.ToSlice()...)
	result = append(result, list2.ToSlice()...)
	return newList(result)
}

// flatten: List<List<T>> -> List<T>
func builtinFlatten(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("flatten expects 1 argument, got %d", len(args))
	}
	listOfLists, ok := args[0].(*List)
	if !ok {
		return newError("flatten expects a list, got %s", args[0].Type())
	}
	var result []Object
	for _, item := range listOfLists.ToSlice() {
		innerList, ok := item.(*List)
		if !ok {
			return newError("flatten expects a list of lists, got list containing %s", item.Type())
		}
		result = append(result, innerList.ToSlice()...)
	}
	return newList(result)
}

// unique: List<T> -> List<T>
func builtinUnique(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("unique expects 1 argument, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("unique expects a list, got %s", args[0].Type())
	}
	var result []Object
	seen := make(map[string]bool)
	for _, elem := range list.ToSlice() {
		key := elem.Inspect()
		if !seen[key] {
			seen[key] = true
			result = append(result, elem)
		}
	}
	return newList(result)
}

// zip: (List<A>, List<B>) -> List<(A, B)>
func builtinZip(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("zip expects 2 arguments, got %d", len(args))
	}
	list1, ok := args[0].(*List)
	if !ok {
		return newError("zip expects a list as first argument, got %s", args[0].Type())
	}
	list2, ok := args[1].(*List)
	if !ok {
		return newError("zip expects a list as second argument, got %s", args[1].Type())
	}
	minLen := list1.len()
	if list2.len() < minLen {
		minLen = list2.len()
	}
	result := make([]Object, minLen)
	for i := 0; i < minLen; i++ {
		result[i] = &Tuple{Elements: []Object{list1.get(i), list2.get(i)}}
	}
	return newList(result)
}

// unzip: List<(A, B)> -> (List<A>, List<B>)
func builtinUnzip(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("unzip expects 1 argument, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("unzip expects a list, got %s", args[0].Type())
	}
	var listA, listB []Object
	for _, item := range list.ToSlice() {
		tuple, ok := item.(*Tuple)
		if !ok || len(tuple.Elements) != 2 {
			return newError("unzip expects a list of 2-tuples, got %s", item.Inspect())
		}
		listA = append(listA, tuple.Elements[0])
		listB = append(listB, tuple.Elements[1])
	}
	return &Tuple{Elements: []Object{newList(listA), newList(listB)}}
}

// sort: List<T> -> List<T> (for Order types)
func builtinSort(e *Evaluator, args ...Object) Object {
	// FIX: Strip witness if present (parity with VM/Compiler which passes dictionaries)
	if len(args) > 1 {
		if _, ok := args[0].(*Dictionary); ok {
			args = args[1:]
		}
	}

	if len(args) != 1 {
		return newError("sort expects 1 argument, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("sort expects a list, got %s", args[0].Type())
	}
	if list.len() == 0 {
		return newList([]Object{})
	}

	// Copy elements
	result := make([]Object, list.len())
	copy(result, list.ToSlice())

	// Sort using comparison
	sort.SliceStable(result, func(i, j int) bool {
		cmp := compareObjects(result[i], result[j])
		return cmp < 0
	})

	return newList(result)
}

// sortBy: (List<T>, (T, T) -> Int) -> List<T>
func builtinSortBy(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("sortBy expects 2 arguments, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("sortBy expects a list as first argument, got %s", args[0].Type())
	}
	cmpFn := args[1]

	if list.len() == 0 {
		return newList([]Object{})
	}

	// Copy elements
	result := make([]Object, list.len())
	copy(result, list.ToSlice())

	// Sort using custom comparator
	sort.SliceStable(result, func(i, j int) bool {
		cmpResult := e.ApplyFunction(cmpFn, []Object{result[i], result[j]})
		if intResult, ok := cmpResult.(*Integer); ok {
			return intResult.Value < 0
		}
		return false
	})

	return newList(result)
}

// range: (Int, Int) -> List<Int>
// Generates list [start..end-1]
func builtinRange(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("range expects 2 arguments, got %d", len(args))
	}
	startArg, ok := args[0].(*Integer)
	if !ok {
		return newError("range expects integers, got %s", args[0].Type())
	}
	endArg, ok := args[1].(*Integer)
	if !ok {
		return newError("range expects integers, got %s", args[1].Type())
	}

	start := int(startArg.Value)
	end := int(endArg.Value)

	if start >= end {
		return newList([]Object{})
	}

	result := make([]Object, end-start)
	for i := start; i < end; i++ {
		result[i-start] = &Integer{Value: int64(i)}
	}
	return newList(result)
}

// find: ((T) -> Bool, List<T>) -> Option<T>
func builtinFind(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("find expects 2 arguments, got %d", len(args))
	}
	predFn := args[0]
	list, ok := args[1].(*List)
	if !ok {
		return newError("find expects a list as second argument, got %s", args[1].Type())
	}

	for _, item := range list.ToSlice() {
		result := e.ApplyFunction(predFn, []Object{item})
		if isError(result) {
			return result
		}
		if boolResult, ok := result.(*Boolean); ok && boolResult.Value {
			return &DataInstance{
				Name:     "Some",
				Fields:   []Object{item},
				TypeName: "Option",
			}
		}
	}
	return &DataInstance{
		Name:     "Zero",
		Fields:   []Object{},
		TypeName: "Option",
	}
}

// findIndex: ((T) -> Bool, List<T>) -> Option<Int>
func builtinFindIndex(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("findIndex expects 2 arguments, got %d", len(args))
	}
	predFn := args[0]
	list, ok := args[1].(*List)
	if !ok {
		return newError("findIndex expects a list as second argument, got %s", args[1].Type())
	}

	for i, item := range list.ToSlice() {
		result := e.ApplyFunction(predFn, []Object{item})
		if isError(result) {
			return result
		}
		if boolResult, ok := result.(*Boolean); ok && boolResult.Value {
			return &DataInstance{
				Name:     "Some",
				Fields:   []Object{&Integer{Value: int64(i)}},
				TypeName: "Option",
			}
		}
	}
	return &DataInstance{
		Name:     "Zero",
		Fields:   []Object{},
		TypeName: "Option",
	}
}

// any: ((T) -> Bool, List<T>) -> Bool
func builtinAny(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("any expects 2 arguments, got %d", len(args))
	}
	predFn := args[0]
	list, ok := args[1].(*List)
	if !ok {
		return newError("any expects a list as second argument, got %s", args[1].Type())
	}

	for _, item := range list.ToSlice() {
		result := e.ApplyFunction(predFn, []Object{item})
		if isError(result) {
			return result
		}
		if boolResult, ok := result.(*Boolean); ok && boolResult.Value {
			return TRUE
		}
	}
	return FALSE
}

// all: ((T) -> Bool, List<T>) -> Bool
func builtinAll(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("all expects 2 arguments, got %d", len(args))
	}
	predFn := args[0]
	list, ok := args[1].(*List)
	if !ok {
		return newError("all expects a list as second argument, got %s", args[1].Type())
	}

	for _, item := range list.ToSlice() {
		result := e.ApplyFunction(predFn, []Object{item})
		if isError(result) {
			return result
		}
		if boolResult, ok := result.(*Boolean); ok && !boolResult.Value {
			return FALSE
		}
	}
	return TRUE
}

// takeWhile: ((T) -> Bool, List<T>) -> List<T>
func builtinTakeWhile(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("takeWhile expects 2 arguments, got %d", len(args))
	}
	predFn := args[0]
	list, ok := args[1].(*List)
	if !ok {
		return newError("takeWhile expects a list as second argument, got %s", args[1].Type())
	}

	var result []Object
	for _, item := range list.ToSlice() {
		predResult := e.ApplyFunction(predFn, []Object{item})
		if isError(predResult) {
			return predResult
		}
		if boolResult, ok := predResult.(*Boolean); ok && boolResult.Value {
			result = append(result, item)
		} else {
			break
		}
	}
	return newList(result)
}

// dropWhile: ((T) -> Bool, List<T>) -> List<T>
func builtinDropWhile(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("dropWhile expects 2 arguments, got %d", len(args))
	}
	predFn := args[0]
	list, ok := args[1].(*List)
	if !ok {
		return newError("dropWhile expects a list as second argument, got %s", args[1].Type())
	}

	startIdx := list.len()
	for i, item := range list.ToSlice() {
		predResult := e.ApplyFunction(predFn, []Object{item})
		if isError(predResult) {
			return predResult
		}
		if boolResult, ok := predResult.(*Boolean); ok && !boolResult.Value {
			startIdx = i
			break
		}
	}
	return list.Slice(startIdx, list.len())
}

// partition: ((T) -> Bool, List<T>) -> (List<T>, List<T>)
func builtinPartition(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("partition expects 2 arguments, got %d", len(args))
	}
	predFn := args[0]
	list, ok := args[1].(*List)
	if !ok {
		return newError("partition expects a list as second argument, got %s", args[1].Type())
	}

	var trueList, falseList []Object
	for _, item := range list.ToSlice() {
		predResult := e.ApplyFunction(predFn, []Object{item})
		if isError(predResult) {
			return predResult
		}
		if boolResult, ok := predResult.(*Boolean); ok && boolResult.Value {
			trueList = append(trueList, item)
		} else {
			falseList = append(falseList, item)
		}
	}
	return &Tuple{Elements: []Object{newList(trueList), newList(falseList)}}
}

// forEach: ((T) -> Nil, List<T>) -> Nil
func builtinForEach(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("forEach expects 2 arguments, got %d", len(args))
	}
	fn := args[0]
	list, ok := args[1].(*List)
	if !ok {
		return newError("forEach expects a list as second argument, got %s", args[1].Type())
	}

	for _, item := range list.ToSlice() {
		result := e.ApplyFunction(fn, []Object{item})
		if isError(result) {
			return result
		}
		// result is ignored - forEach is for side effects only
	}
	return &Nil{}
}

// append: (List<T>, T) -> List<T>
func builtinAppend(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("append expects 2 arguments, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("append expects a list as first argument, got %s", args[0].Type())
	}
	elem := args[1]

	// Create new list with appended element
	// We must copy because lists are immutable
	original := list.ToSlice()
	result := make([]Object, len(original)+1)
	copy(result, original)
	result[len(original)] = elem

	return newList(result)
}

// objectsEqual compares two objects for equality
func objectsEqual(a, b Object) bool {
	switch av := a.(type) {
	case *Integer:
		if bv, ok := b.(*Integer); ok {
			return av.Value == bv.Value
		}
	case *Float:
		if bv, ok := b.(*Float); ok {
			return av.Value == bv.Value
		}
	case *Boolean:
		if bv, ok := b.(*Boolean); ok {
			return av.Value == bv.Value
		}
	case *Char:
		if bv, ok := b.(*Char); ok {
			return av.Value == bv.Value
		}
	}
	return a.Inspect() == b.Inspect()
}

// compareObjects compares two objects, returns -1, 0, or 1
func compareObjects(a, b Object) int {
	switch av := a.(type) {
	case *Integer:
		if bv, ok := b.(*Integer); ok {
			if av.Value < bv.Value {
				return -1
			} else if av.Value > bv.Value {
				return 1
			}
			return 0
		}
	case *Float:
		if bv, ok := b.(*Float); ok {
			if av.Value < bv.Value {
				return -1
			} else if av.Value > bv.Value {
				return 1
			}
			return 0
		}
	case *Char:
		if bv, ok := b.(*Char); ok {
			if av.Value < bv.Value {
				return -1
			} else if av.Value > bv.Value {
				return 1
			}
			return 0
		}
	case *Boolean:
		if bv, ok := b.(*Boolean); ok {
			if !av.Value && bv.Value {
				return -1
			} else if av.Value && !bv.Value {
				return 1
			}
			return 0
		}
	}
	// Fallback: compare string representations
	aStr := a.Inspect()
	bStr := b.Inspect()
	if aStr < bStr {
		return -1
	} else if aStr > bStr {
		return 1
	}
	return 0
}

// SetListBuiltinTypes sets type info for list builtins
func SetListBuiltinTypes(builtins map[string]*Builtin) {
	T := typesystem.TVar{Name: "T"}
	U := typesystem.TVar{Name: "U"}
	A := typesystem.TVar{Name: "A"}
	B := typesystem.TVar{Name: "B"}

	listT := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{T}}
	listU := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{U}}
	listA := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{A}}
	listB := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{B}}
	listListT := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{listT}}
	tupleAB := typesystem.TTuple{Elements: []typesystem.Type{A, B}}
	listTupleAB := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{tupleAB}}
	tupleListAListB := typesystem.TTuple{Elements: []typesystem.Type{listA, listB}}
	optionInt := typesystem.TApp{Constructor: typesystem.TCon{Name: "Option"}, Args: []typesystem.Type{typesystem.Int}}

	optionT := typesystem.TApp{Constructor: typesystem.TCon{Name: "Option"}, Args: []typesystem.Type{T}}
	tupleListTListT := typesystem.TTuple{Elements: []typesystem.Type{listT, listT}}
	predicateT := typesystem.TFunc{Params: []typesystem.Type{T}, ReturnType: typesystem.Bool}

	types := map[string]typesystem.Type{
		"head":     typesystem.TFunc{Params: []typesystem.Type{listT}, ReturnType: T},
		"headOr":   typesystem.TFunc{Params: []typesystem.Type{listT, T}, ReturnType: T},
		"last":     typesystem.TFunc{Params: []typesystem.Type{listT}, ReturnType: T},
		"lastOr":   typesystem.TFunc{Params: []typesystem.Type{listT, T}, ReturnType: T},
		"nth":      typesystem.TFunc{Params: []typesystem.Type{listT, typesystem.Int}, ReturnType: T},
		"nthOr":    typesystem.TFunc{Params: []typesystem.Type{listT, typesystem.Int, T}, ReturnType: T},
		"tail":     typesystem.TFunc{Params: []typesystem.Type{listT}, ReturnType: listT},
		"init":     typesystem.TFunc{Params: []typesystem.Type{listT}, ReturnType: listT},
		"take":     typesystem.TFunc{Params: []typesystem.Type{listT, typesystem.Int}, ReturnType: listT},
		"drop":     typesystem.TFunc{Params: []typesystem.Type{listT, typesystem.Int}, ReturnType: listT},
		"slice":    typesystem.TFunc{Params: []typesystem.Type{listT, typesystem.Int, typesystem.Int}, ReturnType: listT},
		"length":   typesystem.TFunc{Params: []typesystem.Type{listT}, ReturnType: typesystem.Int},
		"contains": typesystem.TFunc{Params: []typesystem.Type{listT, T}, ReturnType: typesystem.Bool},
		// Function-first for higher-order functions
		"filter":    typesystem.TFunc{Params: []typesystem.Type{predicateT, listT}, ReturnType: listT},
		"map":       typesystem.TFunc{Params: []typesystem.Type{typesystem.TFunc{Params: []typesystem.Type{T}, ReturnType: U}, listT}, ReturnType: listU},
		"foldl":     typesystem.TFunc{Params: []typesystem.Type{typesystem.TFunc{Params: []typesystem.Type{U, T}, ReturnType: U}, U, listT}, ReturnType: U},
		"foldr":     typesystem.TFunc{Params: []typesystem.Type{typesystem.TFunc{Params: []typesystem.Type{T, U}, ReturnType: U}, U, listT}, ReturnType: U},
		"indexOf":   typesystem.TFunc{Params: []typesystem.Type{listT, T}, ReturnType: optionInt},
		"find":      typesystem.TFunc{Params: []typesystem.Type{predicateT, listT}, ReturnType: optionT},
		"findIndex": typesystem.TFunc{Params: []typesystem.Type{predicateT, listT}, ReturnType: optionInt},
		"any":       typesystem.TFunc{Params: []typesystem.Type{predicateT, listT}, ReturnType: typesystem.Bool},
		"all":       typesystem.TFunc{Params: []typesystem.Type{predicateT, listT}, ReturnType: typesystem.Bool},
		"takeWhile": typesystem.TFunc{Params: []typesystem.Type{predicateT, listT}, ReturnType: listT},
		"dropWhile": typesystem.TFunc{Params: []typesystem.Type{predicateT, listT}, ReturnType: listT},
		"partition": typesystem.TFunc{Params: []typesystem.Type{predicateT, listT}, ReturnType: tupleListTListT},
		"forEach":   typesystem.TFunc{Params: []typesystem.Type{typesystem.TFunc{Params: []typesystem.Type{T}, ReturnType: typesystem.Nil}, listT}, ReturnType: typesystem.Nil},
		"reverse":   typesystem.TFunc{Params: []typesystem.Type{listT}, ReturnType: listT},
		"concat":    typesystem.TFunc{Params: []typesystem.Type{listT, listT}, ReturnType: listT},
		"flatten":   typesystem.TFunc{Params: []typesystem.Type{listListT}, ReturnType: listT},
		"unique":    typesystem.TFunc{Params: []typesystem.Type{listT}, ReturnType: listT},
		"zip":       typesystem.TFunc{Params: []typesystem.Type{listA, listB}, ReturnType: listTupleAB},
		"unzip":     typesystem.TFunc{Params: []typesystem.Type{listTupleAB}, ReturnType: tupleListAListB},
		"sort":      typesystem.TFunc{Params: []typesystem.Type{listT}, ReturnType: listT},
		"sortBy":    typesystem.TFunc{Params: []typesystem.Type{listT, typesystem.TFunc{Params: []typesystem.Type{T, T}, ReturnType: typesystem.Int}}, ReturnType: listT},
		"range":     typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, typesystem.Int}, ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{typesystem.Int}}},
		"append":    typesystem.TFunc{Params: []typesystem.Type{listT, T}, ReturnType: listT},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

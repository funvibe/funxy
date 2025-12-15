package evaluator

import (
	"math/rand"
	"github.com/funvibe/funxy/internal/typesystem"
	"sync"
	"time"
)

// Global random source with mutex for thread safety
var (
	randSource *rand.Rand
	randMutex  sync.Mutex
)

func init() {
	// Initialize with current time
	randSource = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// RandBuiltins returns built-in functions for lib/rand virtual package
func RandBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"randomInt":        {Fn: builtinRandomInt, Name: "randomInt"},
		"randomIntRange":   {Fn: builtinRandomIntRange, Name: "randomIntRange"},
		"randomFloat":      {Fn: builtinRandomFloat, Name: "randomFloat"},
		"randomFloatRange": {Fn: builtinRandomFloatRange, Name: "randomFloatRange"},
		"randomBool":       {Fn: builtinRandomBool, Name: "randomBool"},
		"randomChoice":     {Fn: builtinRandomChoice, Name: "randomChoice"},
		"randomShuffle":    {Fn: builtinRandomShuffle, Name: "randomShuffle"},
		"randomSample":     {Fn: builtinRandomSample, Name: "randomSample"},
		"randomSeed":       {Fn: builtinRandomSeed, Name: "randomSeed"},
	}
}

// randomInt: () -> Int
func builtinRandomInt(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("randomInt expects 0 arguments, got %d", len(args))
	}
	randMutex.Lock()
	defer randMutex.Unlock()
	return &Integer{Value: randSource.Int63()}
}

// randomIntRange: (Int, Int) -> Int
// Returns random int in [min, max]
func builtinRandomIntRange(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("randomIntRange expects 2 arguments, got %d", len(args))
	}

	minInt, ok := args[0].(*Integer)
	if !ok {
		return newError("randomIntRange expects integer min, got %s", args[0].Type())
	}

	maxInt, ok := args[1].(*Integer)
	if !ok {
		return newError("randomIntRange expects integer max, got %s", args[1].Type())
	}

	min := minInt.Value
	max := maxInt.Value

	if min > max {
		return newError("randomIntRange: min (%d) cannot be greater than max (%d)", min, max)
	}

	if min == max {
		return &Integer{Value: min}
	}

	randMutex.Lock()
	defer randMutex.Unlock()
	return &Integer{Value: min + randSource.Int63n(max-min+1)}
}

// randomFloat: () -> Float
// Returns random float in [0.0, 1.0)
func builtinRandomFloat(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("randomFloat expects 0 arguments, got %d", len(args))
	}
	randMutex.Lock()
	defer randMutex.Unlock()
	return &Float{Value: randSource.Float64()}
}

// randomFloatRange: (Float, Float) -> Float
// Returns random float in [min, max)
func builtinRandomFloatRange(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("randomFloatRange expects 2 arguments, got %d", len(args))
	}

	minFloat, ok := args[0].(*Float)
	if !ok {
		return newError("randomFloatRange expects float min, got %s", args[0].Type())
	}

	maxFloat, ok := args[1].(*Float)
	if !ok {
		return newError("randomFloatRange expects float max, got %s", args[1].Type())
	}

	min := minFloat.Value
	max := maxFloat.Value

	if min > max {
		return newError("randomFloatRange: min cannot be greater than max")
	}

	randMutex.Lock()
	defer randMutex.Unlock()
	return &Float{Value: min + randSource.Float64()*(max-min)}
}

// randomBool: () -> Bool
func builtinRandomBool(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("randomBool expects 0 arguments, got %d", len(args))
	}
	randMutex.Lock()
	defer randMutex.Unlock()
	if randSource.Intn(2) == 1 {
		return TRUE
	}
	return FALSE
}

// randomChoice: List<A> -> Option<A>
func builtinRandomChoice(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("randomChoice expects 1 argument, got %d", len(args))
	}

	list, ok := args[0].(*List)
	if !ok {
		return newError("randomChoice expects a list, got %s", args[0].Type())
	}

	if list.len() == 0 {
		return makeZero()
	}

	randMutex.Lock()
	idx := randSource.Intn(list.len())
	randMutex.Unlock()
	return makeSome(list.get(idx))
}

// randomShuffle: List<A> -> List<A>
func builtinRandomShuffle(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("randomShuffle expects 1 argument, got %d", len(args))
	}

	list, ok := args[0].(*List)
	if !ok {
		return newError("randomShuffle expects a list, got %s", args[0].Type())
	}

	// Create a copy
	shuffled := make([]Object, list.len())
	copy(shuffled, list.ToSlice())

	// Fisher-Yates shuffle
	randMutex.Lock()
	randSource.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	randMutex.Unlock()

	return newList(shuffled)
}

// randomSample: (List<A>, Int) -> List<A>
func builtinRandomSample(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("randomSample expects 2 arguments, got %d", len(args))
	}

	list, ok := args[0].(*List)
	if !ok {
		return newError("randomSample expects a list, got %s", args[0].Type())
	}

	nInt, ok := args[1].(*Integer)
	if !ok {
		return newError("randomSample expects integer n, got %s", args[1].Type())
	}

	n := int(nInt.Value)
	if n < 0 {
		return newError("randomSample: n cannot be negative")
	}

	randMutex.Lock()
	defer randMutex.Unlock()

	if n >= list.len() {
		// Return shuffled copy of entire list
		shuffled := make([]Object, list.len())
		copy(shuffled, list.ToSlice())
		randSource.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})
		return newList(shuffled)
	}

	// Reservoir sampling for efficiency
	result := make([]Object, n)
	copy(result, list.ToSlice()[:n])

	for i := n; i < list.len(); i++ {
		j := randSource.Intn(i + 1)
		if j < n {
			result[j] = list.get(i)
		}
	}

	return newList(result)
}

// randomSeed: Int -> Nil
func builtinRandomSeed(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("randomSeed expects 1 argument, got %d", len(args))
	}

	seedInt, ok := args[0].(*Integer)
	if !ok {
		return newError("randomSeed expects integer seed, got %s", args[0].Type())
	}

	randMutex.Lock()
	randSource = rand.New(rand.NewSource(seedInt.Value))
	randMutex.Unlock()
	return &Nil{}
}

// SetRandBuiltinTypes sets type info for rand builtins
func SetRandBuiltinTypes(builtins map[string]*Builtin) {
	// Generic type variable
	typeA := typesystem.TVar{Name: "A"}

	// List<A>
	listA := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typeA},
	}

	// Option<A>
	optionA := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{typeA},
	}

	types := map[string]typesystem.Type{
		"randomInt":        typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Int},
		"randomIntRange":   typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, typesystem.Int}, ReturnType: typesystem.Int},
		"randomFloat":      typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Float},
		"randomFloatRange": typesystem.TFunc{Params: []typesystem.Type{typesystem.Float, typesystem.Float}, ReturnType: typesystem.Float},
		"randomBool":       typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Bool},
		"randomChoice":     typesystem.TFunc{Params: []typesystem.Type{listA}, ReturnType: optionA},
		"randomShuffle":    typesystem.TFunc{Params: []typesystem.Type{listA}, ReturnType: listA},
		"randomSample":     typesystem.TFunc{Params: []typesystem.Type{listA, typesystem.Int}, ReturnType: listA},
		"randomSeed":       typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

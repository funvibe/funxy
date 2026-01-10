package evaluator

import (
	"github.com/funvibe/funxy/internal/typesystem"
	"time"
)

// TimeBuiltins returns built-in functions for lib/time virtual package
func TimeBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"timeNow": {Fn: builtinTime, Name: "timeNow"},
		"clockNs": {Fn: builtinClockNs, Name: "clockNs"},
		"clockMs": {Fn: builtinClockMs, Name: "clockMs"},
		"sleep":   {Fn: builtinSleep, Name: "sleep"},
		"sleepMs": {Fn: builtinSleepMs, Name: "sleepMs"},
	}
}

// time: () -> Int
// Returns Unix timestamp in seconds
func builtinTime(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("time expects 0 arguments, got %d", len(args))
	}
	return &Integer{Value: time.Now().Unix()}
}

// clockNs: () -> Int
// Returns monotonic nanoseconds (for benchmarking)
func builtinClockNs(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("clockNs expects 0 arguments, got %d", len(args))
	}
	return &Integer{Value: time.Now().UnixNano()}
}

// clockMs: () -> Int
// Returns monotonic milliseconds (for benchmarking)
func builtinClockMs(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("clockMs expects 0 arguments, got %d", len(args))
	}
	return &Integer{Value: time.Now().UnixNano() / 1_000_000}
}

// sleep: (Int) -> Nil
// Pauses execution for specified seconds
func builtinSleep(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("sleep expects 1 argument, got %d", len(args))
	}
	seconds, ok := args[0].(*Integer)
	if !ok {
		return newError("sleep expects an integer argument, got %s", args[0].Type())
	}
	if seconds.Value < 0 {
		return newError("sleep: duration cannot be negative")
	}
	time.Sleep(time.Duration(seconds.Value) * time.Second)
	return &Nil{}
}

// sleepMs: (Int) -> Nil
// Pauses execution for specified milliseconds
func builtinSleepMs(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("sleepMs expects 1 argument, got %d", len(args))
	}
	ms, ok := args[0].(*Integer)
	if !ok {
		return newError("sleepMs expects an integer argument, got %s", args[0].Type())
	}
	if ms.Value < 0 {
		return newError("sleepMs: duration cannot be negative")
	}
	time.Sleep(time.Duration(ms.Value) * time.Millisecond)
	return &Nil{}
}

// SetTimeBuiltinTypes sets type info for time builtins
func SetTimeBuiltinTypes(builtins map[string]*Builtin) {
	types := map[string]typesystem.Type{
		"timeNow": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Int},
		"clockNs": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Int},
		"clockMs": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Int},
		"sleep":   typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},
		"sleepMs": typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

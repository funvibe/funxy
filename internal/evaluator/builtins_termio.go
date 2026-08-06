package evaluator

import (
	"context"
	"fmt"
	"github.com/funvibe/funxy/internal/typesystem"
	"time"
)

type termInputEventKind uint8

const (
	termInputKey termInputEventKind = iota
	termInputText
	termInputError
)

type termInputEvent struct {
	kind  termInputEventKind
	value string
}

func keyInputEvent(value string) termInputEvent {
	return termInputEvent{kind: termInputKey, value: value}
}

func textInputEvent(value string) termInputEvent {
	return termInputEvent{kind: termInputText, value: value}
}

func errorInputEvent(value string) termInputEvent {
	return termInputEvent{kind: termInputError, value: value}
}

// RegisterTermIOBuiltins registers the InputEvent ADT and lib/termio functions.
func RegisterTermIOBuiltins(env *Environment) {
	env.Set("InputEvent", &TypeObject{TypeVal: typesystem.TCon{Name: "InputEvent"}})
	env.Set("KeyEvent", &Constructor{Name: "KeyEvent", TypeName: "InputEvent", Arity: 1})
	env.Set("TextEvent", &Constructor{Name: "TextEvent", TypeName: "InputEvent", Arity: 1})

	for name, fn := range TermIOBuiltins() {
		env.Set(name, fn)
	}
}

// TermIOBuiltins returns built-in functions for lib/termio.
func TermIOBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"withTerminalInput": {Fn: builtinWithTerminalInput, Name: "withTerminalInput"},
		"readInputEvent":    {Fn: builtinReadInputEvent, Name: "readInputEvent"},
	}
}

// builtinWithTerminalInput owns the complete terminal input lifecycle. The
// terminal is restored when the callback returns, returns an evaluator error,
// or panics. sysExit and supported Unix termination signals also run cleanup.
func builtinWithTerminalInput(e *Evaluator, args ...Object) (result Object) {
	if len(args) != 1 {
		return newError("withTerminalInput expects 1 argument, got %d", len(args))
	}

	if err := startTermInputSession(); err != nil {
		return newError("withTerminalInput: %s", err)
	}

	defer func() {
		if err := stopTermInputSession(); err != nil {
			if evaluatorErr, ok := result.(*Error); ok {
				combined := *evaluatorErr
				combined.Message += fmt.Sprintf("; terminal cleanup failed: %s", err)
				result = &combined
			} else {
				result = newError("withTerminalInput cleanup: %s", err)
			}
		}
	}()

	return e.ApplyFunction(args[0], []Object{})
}

func builtinReadInputEvent(e *Evaluator, args ...Object) Object {
	if len(args) > 1 {
		return newError("readInputEvent expects 0 or 1 arguments, got %d", len(args))
	}

	timeoutMs := int64(0)
	if len(args) == 1 {
		timeout, ok := args[0].(*Integer)
		if !ok {
			return newError("readInputEvent: expected Int for timeout, got %s", args[0].Type())
		}
		if timeout.Value < 0 {
			return newError("readInputEvent: timeout must be non-negative")
		}
		timeoutMs = timeout.Value
	}

	ctx := e.Context
	if ctx == nil {
		ctx = context.Background()
	}
	event, ok, err := waitTermInputEvent(ctx, time.Duration(timeoutMs)*time.Millisecond)
	if err != nil {
		return newError("readInputEvent: %s", err)
	}
	if !ok {
		return makeNone()
	}

	value := stringToList(event.value)
	switch event.kind {
	case termInputKey:
		return makeSome(&DataInstance{Name: "KeyEvent", TypeName: "InputEvent", Fields: []Object{value}})
	case termInputText:
		return makeSome(&DataInstance{Name: "TextEvent", TypeName: "InputEvent", Fields: []Object{value}})
	default:
		return newError("readInputEvent: %s", fmt.Sprintf("unknown event kind %d", event.kind))
	}
}

package evaluator

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/typesystem"
	"strings"
)

// TailCall represents a function call that should be executed via trampoline.
type TailCall struct {
	Func     Object
	Args     []Object
	Line     int
	Column   int
	Name     string                       // Function name for stack trace
	File     string                       // File name for stack trace
	Witness  map[string][]typesystem.Type // Witnesses for this call
	CallNode ast.Node                     // Original call node for type dispatch
}

func (tc *TailCall) Type() ObjectType { return TAIL_CALL_OBJ }
func (tc *TailCall) Inspect() string  { return "TailCall" }
func (tc *TailCall) RuntimeType() typesystem.Type {
	if tc == nil {
		return typesystem.TCon{Name: "TailCall"}
	}
	return typesystem.TCon{Name: "TailCall"}
}
func (tc *TailCall) Hash() uint32 { return 0 }

// Error
type Error struct {
	Message    string
	Line       int
	Column     int
	StackTrace []StackFrame
}

// StackFrame for error stack traces
type StackFrame struct {
	Name   string
	File   string
	Line   int
	Column int
}

func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) Inspect() string {
	var result string
	if e.Line > 0 {
		result = fmt.Sprintf("ERROR at %d:%d: %s", e.Line, e.Column, e.Message)
	} else {
		result = "ERROR: " + e.Message
	}

	// Add stack trace if available
	// Shows call chain from innermost (most recent) to outermost
	// Format: at <caller>:<line> (called <callee>)
	if len(e.StackTrace) > 0 {
		result += "\nStack trace:"
		for i := len(e.StackTrace) - 1; i >= 0; i-- {
			frame := e.StackTrace[i]
			// The caller is the NEXT frame (outer), or filename for the outermost
			var callerName string
			if i > 0 {
				callerName = e.StackTrace[i-1].Name
			} else {
				// Use filename without extension for top-level calls
				callerName = frame.File
				if idx := strings.LastIndex(callerName, "."); idx > 0 {
					callerName = callerName[:idx]
				}
			}
			result += fmt.Sprintf("\n  at %s:%d (called %s)", callerName, frame.Line, frame.Name)
		}
	}

	return result
}
func (e *Error) RuntimeType() typesystem.Type {
	if e == nil {
		return typesystem.TCon{Name: "Error"}
	}
	return typesystem.TCon{Name: "Error"}
}
func (e *Error) Hash() uint32 {
	return hashString(e.Message)
}

// ReturnValue wraps a value that is being returned prematurely
type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() ObjectType { return RETURN_VALUE_OBJ }
func (rv *ReturnValue) Inspect() string  { return rv.Value.Inspect() }
func (rv *ReturnValue) RuntimeType() typesystem.Type {
	if rv == nil {
		return typesystem.TCon{Name: "ReturnValue"}
	}
	return rv.Value.RuntimeType()
}
func (rv *ReturnValue) Hash() uint32 { return rv.Value.Hash() }

// BreakSignal is an internal object used to signal a break from a loop.
type BreakSignal struct {
	Value Object // Optional value to return from loop (default None)
}

func (bs *BreakSignal) Type() ObjectType { return BREAK_SIGNAL_OBJ }
func (bs *BreakSignal) Inspect() string  { return "Break" }
func (bs *BreakSignal) RuntimeType() typesystem.Type {
	if bs == nil {
		return typesystem.TCon{Name: "Break"}
	}
	return typesystem.TCon{Name: "Break"}
}
func (bs *BreakSignal) Hash() uint32 { return 0 }

// ContinueSignal is an internal object used to signal a continue in a loop.
type ContinueSignal struct{}

func (cs *ContinueSignal) Type() ObjectType { return CONTINUE_SIGNAL_OBJ }
func (cs *ContinueSignal) Inspect() string  { return "Continue" }
func (cs *ContinueSignal) RuntimeType() typesystem.Type {
	if cs == nil {
		return typesystem.TCon{Name: "Continue"}
	}
	return typesystem.TCon{Name: "Continue"}
}
func (cs *ContinueSignal) Hash() uint32 { return 0 }

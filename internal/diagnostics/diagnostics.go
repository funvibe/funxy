package diagnostics

import (
	"fmt"
	"github.com/funvibe/funxy/internal/token"
)

// Phase represents the processing phase where an error occurred
type Phase string

const (
	PhaseLexer    Phase = "lexer"
	PhaseParser   Phase = "parser"
	PhaseAnalyzer Phase = "analyzer"
	PhaseRuntime  Phase = "runtime"
)

type ErrorCode string

const (
	// Lexer Errors
	ErrL001 ErrorCode = "L001" // Invalid character

	// Parser Errors
	ErrP001 ErrorCode = "P001" // Unexpected token
	ErrP002 ErrorCode = "P002" // Expected identifier on left side of assignment
	ErrP003 ErrorCode = "P003" // Could not parse as integer
	ErrP004 ErrorCode = "P004" // No prefix parse function found
	ErrP005 ErrorCode = "P005" // Expected closing parenthesis
	ErrP006 ErrorCode = "P006" // Invalid import syntax
	ErrP007 ErrorCode = "P007" // Index assignment not supported

	// Analyzer Errors
	ErrA001 ErrorCode = "A001" // Undeclared variable
	ErrA002 ErrorCode = "A002" // Undeclared type
	ErrA003 ErrorCode = "A003" // Type error
	ErrA004 ErrorCode = "A004" // Redefinition error
	ErrA005 ErrorCode = "A005" // Type mismatch in assignment
	ErrA006 ErrorCode = "A006" // Undefined symbol
	ErrA007 ErrorCode = "A007" // Match not exhaustive
	ErrA008 ErrorCode = "A008" // Naming convention error

	// Runtime Errors
	ErrR001 ErrorCode = "R001" // Runtime error

	// Compilation Errors
	ErrC001 ErrorCode = "C001" // Compilation error
)

var errorTemplates = map[ErrorCode]string{
	ErrL001: "invalid character: '%s'",
	ErrP001: "unexpected token: expected '%s', but got '%s'",
	ErrP002: "expected an identifier on the left side of an assignment",
	ErrP003: "could not parse '%s' as an integer",
	ErrP004: "cannot parse expression starting with '%s'",
	ErrP005: "expected next token to be '%s', but got '%s' instead",
	ErrP006: "%s",
	ErrP007: "index assignment is not supported. Use 'lib/list' update/insert functions for immutable list modification",
	ErrA001: "undeclared variable: '%s'",
	ErrA002: "undeclared type: '%s'",
	ErrA003: "type error: %s",
	ErrA004: "redefinition of symbol: '%s'",
	ErrA005: "type mismatch in assignment: expected %s, got %s",
	ErrA006: "undefined symbol: '%s'",
	ErrA007: "match expression is not exhaustive. Missing cases: %s",
	ErrA008: "naming convention: %s",
	ErrR001: "runtime error: %s",
	ErrC001: "compilation error: %s",
}

type DiagnosticError struct {
	Code  ErrorCode
	Phase Phase
	Args  []interface{}
	Token token.Token
	File  string
	Hint  string // Optional hint for fixing the error
}

func (e *DiagnosticError) Error() string {
	template, ok := errorTemplates[e.Code]
	if !ok {
		return fmt.Sprintf("unknown error code: %s", e.Code)
	}

	message := fmt.Sprintf(template, e.Args...)

	prefix := ""
	if e.File != "" {
		prefix = fmt.Sprintf("%s: ", e.File)
	}

	/*
		phaseStr := ""
		if e.Phase != "" {
			phaseStr = fmt.Sprintf("[%s] ", e.Phase)
		}
	*/
	phaseStr := ""

	var result string
	if e.Token.Line > 0 {
		result = fmt.Sprintf("%s%serror at %d:%d [%s]: %s", prefix, phaseStr, e.Token.Line, e.Token.Column, e.Code, message)
	} else {
		result = fmt.Sprintf("%s%serror [%s]: %s", prefix, phaseStr, e.Code, message)
	}

	// Hints disabled - they're unstable and break tests
	// if e.Hint != "" {
	// 	result += "\n  hint: " + e.Hint
	// }
	return result
}

// NewError creates an error with just code and token (legacy compatibility)
func NewError(code ErrorCode, tok token.Token, args ...interface{}) *DiagnosticError {
	return &DiagnosticError{
		Code:  code,
		Token: tok,
		Args:  args,
	}
}

// NewPhaseError creates an error with phase information
func NewPhaseError(phase Phase, code ErrorCode, tok token.Token, args ...interface{}) *DiagnosticError {
	return &DiagnosticError{
		Code:  code,
		Phase: phase,
		Token: tok,
		Args:  args,
	}
}

// NewAnalyzerError creates an analyzer phase error
func NewAnalyzerError(code ErrorCode, tok token.Token, args ...interface{}) *DiagnosticError {
	return NewPhaseError(PhaseAnalyzer, code, tok, args...)
}

// InternalError creates an internal error (for "should never happen" cases)
func InternalError(tok token.Token, message string) *DiagnosticError {
	return NewAnalyzerError(ErrA003, tok, "internal error: "+message)
}

// WrapError wraps an existing error with phase and location info
func WrapError(phase Phase, tok token.Token, err error) *DiagnosticError {
	if ce, ok := err.(*DiagnosticError); ok {
		// Already a DiagnosticError, just add phase if missing
		if ce.Phase == "" {
			ce.Phase = phase
		}
		if ce.Token.Line == 0 && tok.Line > 0 {
			ce.Token = tok
		}
		return ce
	}
	// Wrap generic error
	code := ErrA003 // Default to type error for analyzer
	return NewPhaseError(phase, code, tok, err.Error())
}

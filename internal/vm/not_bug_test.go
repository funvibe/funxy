package vm

import (
	"testing"

	"github.com/funvibe/funxy/internal/evaluator"
)

// TestPrefixNotInTailPosition verifies that the ! (NOT) operator works
// correctly when the operand is a function call in tail position.
// This was a bug where the compiler emitted TAIL_CALL for the operand
// of a prefix expression, causing the prefix operator to be skipped.
// See: compilePrefixExpression must set inTailPosition=false before
// compiling the operand.
func TestPrefixNotInTailPosition(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name: "! with function call in lambda",
			input: `
				isEven = fun(x) { x % 2 == 0 }
				negate = fun(x) { !isEven(x) }
				negate(4)
			`,
			expected: false,
		},
		{
			name: "! with function call returns true",
			input: `
				isEven = fun(x) { x % 2 == 0 }
				negate = fun(x) { !isEven(x) }
				negate(3)
			`,
			expected: true,
		},
		{
			name: "! in higher-order function",
			input: `
				isEven = fun(x) { x % 2 == 0 }
				apply = fun(f, x) { f(x) }
				apply(fun(x) { !isEven(x) }, 4)
			`,
			expected: false,
		},
		{
			name: "negation of function call in tail position",
			input: `
				getNum = fun(x) { x + 10 }
				neg = fun(x) { -getNum(x) }
				neg(5)
			`,
			expected: int64(-15),
		},
		{
			name: "bitwise NOT of function call in tail position",
			input: `
				getNum = fun(x) { x + 1 }
				bnot = fun(x) { ~getNum(x) }
				bnot(0)
			`,
			expected: int64(-2),
		},
		{
			name: "! at top level (should still work)",
			input: `
				isEven = fun(x) { x % 2 == 0 }
				!isEven(4)
			`,
			expected: false,
		},
		{
			name: "chained prefix in tail position",
			input: `
				isOdd = fun(x) { x % 2 != 0 }
				doubleNeg = fun(x) { !!isOdd(x) }
				doubleNeg(3)
			`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVM(t, tt.input)

			switch exp := tt.expected.(type) {
			case bool:
				b, ok := result.(*evaluator.Boolean)
				if !ok {
					t.Fatalf("expected Boolean, got %T (%s)", result, result.Inspect())
				}
				if b.Value != exp {
					t.Errorf("expected %v, got %v", exp, b.Value)
				}
			case int64:
				i, ok := result.(*evaluator.Integer)
				if !ok {
					t.Fatalf("expected Integer, got %T (%s)", result, result.Inspect())
				}
				if i.Value != exp {
					t.Errorf("expected %d, got %d", exp, i.Value)
				}
			case string:
				if result.Inspect() != exp {
					t.Errorf("expected %s, got %s", exp, result.Inspect())
				}
			}
		})
	}
}

package vm

import (
	"testing"

	"github.com/funvibe/funxy/internal/evaluator"
)

// TestIndexAndMemberNotInTailPosition verifies that a function call used as the
// index of an index expression (xs[f(i)]) or as the receiver of a member
// access (f(x).field) is NOT compiled as a tail call.
//
// This was a bug where, inside a function/closure body in tail position, the
// inner call reused the current call frame and returned directly, so:
//   - xs[f(i)] returned f(i) instead of xs[f(i)]
//   - f(x).field returned f(x) instead of f(x).field
//
// See: compileIndexExpression / compileMemberExpression must set
// inTailPosition=false before compiling their sub-expressions.
func TestIndexAndMemberNotInTailPosition(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name: "index with call result inside closure (tail position)",
			input: `
				pick = fun(i) { i }
				xs = [30, 0, 40]
				get = fun(i) { xs[pick(i)] }
				get(2)
			`,
			expected: int64(40),
		},
		{
			name: "index with call result, first element",
			input: `
				pick = fun(i) { i }
				xs = [30, 0, 40]
				get = fun(i) { xs[pick(i)] }
				get(0)
			`,
			expected: int64(30),
		},
		{
			name: "index where the indexed object is itself a call",
			input: `
				mkList = fun(x) { [x, x + 1, x + 2] }
				at1 = fun(x) { mkList(x)[1] }
				at1(5)
			`,
			expected: int64(6),
		},
		{
			name: "member access on a call result (tail position)",
			input: `
				mkRec = fun(x) { { val: x * 10 } }
				getVal = fun(x) { mkRec(x).val }
				getVal(3)
			`,
			expected: int64(30),
		},
		{
			name: "index with call result summed in a loop (closure capture)",
			input: `
				pick = fun(i) { i }
				sumViaClosure = fun(xs) {
					get = fun(i) { xs[pick(i)] }
					get(0) + get(1) + get(2)
				}
				sumViaClosure([30, 0, 30])
			`,
			expected: int64(60),
		},
		{
			name: "index at top level still works",
			input: `
				pick = fun(i) { i }
				xs = [7, 8, 9]
				xs[pick(2)]
			`,
			expected: int64(9),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVM(t, tt.input)

			i, ok := result.(*evaluator.Integer)
			if !ok {
				t.Fatalf("expected Integer, got %T (%s)", result, result.Inspect())
			}
			if i.Value != tt.expected.(int64) {
				t.Errorf("expected %d, got %d", tt.expected, i.Value)
			}
		})
	}
}

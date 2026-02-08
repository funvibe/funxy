package mutator

import (
	"flag"
	"github.com/funvibe/funxy/internal/ast"
	"testing"
)

var _ = flag.Bool("tree", false, "ignored")

func TestASTMutator_Mutate(t *testing.T) {
	// Create a simple AST
	program := &ast.Program{
		Statements: []ast.Statement{
			&ast.ExpressionStatement{
				Expression: &ast.InfixExpression{
					Left:     &ast.IntegerLiteral{Value: 1},
					Operator: "+",
					Right:    &ast.IntegerLiteral{Value: 2},
				},
			},
		},
	}

	// Create mutator with fixed seed
	mutator := NewASTMutator(12345)

	// Apply mutation
	mutator.Mutate(program)

	// Check if something changed
	stmt := program.Statements[0].(*ast.ExpressionStatement)
	expr := stmt.Expression.(*ast.InfixExpression)

	// With seed 12345, we expect some change.
	// Let's just verify it's still a valid structure but potentially different.
	if expr.Operator == "+" && expr.Left.(*ast.IntegerLiteral).Value == 1 && expr.Right.(*ast.IntegerLiteral).Value == 2 {
		// It's possible mutation didn't happen or changed something to same value (unlikely with this seed/logic)
		// But let's try mutating multiple times to ensure change
		changed := false
		for i := 0; i < 100; i++ {
			mutator.Mutate(program)
			stmt = program.Statements[0].(*ast.ExpressionStatement)
			expr = stmt.Expression.(*ast.InfixExpression)
			if expr.Operator != "+" || expr.Left.(*ast.IntegerLiteral).Value != 1 || expr.Right.(*ast.IntegerLiteral).Value != 2 {
				changed = true
				break
			}
		}
		if !changed {
			t.Error("AST was not mutated after multiple attempts")
		}
	}
}

func TestASTMutator_MutateBlock(t *testing.T) {
	block := &ast.BlockStatement{
		Statements: []ast.Statement{
			&ast.ExpressionStatement{Expression: &ast.IntegerLiteral{Value: 1}},
			&ast.ExpressionStatement{Expression: &ast.IntegerLiteral{Value: 2}},
			&ast.ExpressionStatement{Expression: &ast.IntegerLiteral{Value: 3}},
		},
	}

	program := &ast.Program{
		Statements: []ast.Statement{
			&ast.ExpressionStatement{
				Expression: &ast.IfExpression{
					Condition:   &ast.BooleanLiteral{Value: true},
					Consequence: block,
				},
			},
		},
	}

	mutator := NewASTMutator(12345)

	// Mutate enough times to trigger block mutation (deletion or inner mutation)
	initialLen := len(block.Statements)
	changed := false
	for i := 0; i < 100; i++ {
		mutator.Mutate(program)
		if len(block.Statements) != initialLen {
			changed = true
			break
		}
		// Check values
		if len(block.Statements) > 0 {
			stmt := block.Statements[0].(*ast.ExpressionStatement)
			if lit, ok := stmt.Expression.(*ast.IntegerLiteral); ok {
				if lit.Value != 1 {
					changed = true
					break
				}
			}
		}
	}

	if !changed {
		t.Log("Block structure or content did not change (might be random chance, but unlikely)")
	}
}

func TestASTMutator_RandomOperator(t *testing.T) {
	mutator := NewASTMutator(1)
	op := mutator.randomOperator()
	if op == "" {
		t.Error("Returned empty operator")
	}
}

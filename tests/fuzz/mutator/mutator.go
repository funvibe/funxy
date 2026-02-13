package mutator

import (
	"math/rand"
	"github.com/funvibe/funxy/internal/ast"
)

// ASTMutator applies random mutations to an AST.
type ASTMutator struct {
	rnd *rand.Rand
}

// NewASTMutator creates a new ASTMutator with the given seed.
func NewASTMutator(seed int64) *ASTMutator {
	return &ASTMutator{
		rnd: rand.New(rand.NewSource(seed)),
	}
}

// Mutate applies a random mutation to the program.
// It modifies the AST in place.
func (m *ASTMutator) Mutate(program *ast.Program) {
	if len(program.Statements) == 0 {
		return
	}

	// Pick a random statement to mutate
	idx := m.rnd.Intn(len(program.Statements))
	stmt := program.Statements[idx]

	// Apply a mutation based on the statement type
	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		if s == nil || s.Expression == nil {
			return
		}
		m.mutateExpression(s.Expression)
	case *ast.BlockStatement:
		if s == nil {
			return
		}
		m.mutateBlock(s)
	case *ast.FunctionStatement:
		if s == nil {
			return
		}
		m.mutateFunction(s)
	case *ast.ConstantDeclaration:
		if s == nil {
			return
		}
		m.mutateConstantDeclaration(s)
	}
}

func (m *ASTMutator) mutateExpression(expr ast.Expression) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.InfixExpression:
		r := m.rnd.Float32()
		if r < 0.33 {
			e.Operator = m.randomOperator()
		} else if r < 0.66 {
			m.mutateExpression(e.Left)
		} else {
			m.mutateExpression(e.Right)
		}
	case *ast.PrefixExpression:
		if m.rnd.Float32() < 0.5 {
			e.Operator = m.randomPrefixOperator()
		} else {
			m.mutateExpression(e.Right)
		}
	case *ast.IntegerLiteral:
		// Change value
		e.Value += m.rnd.Int63n(21) - 10 // -10 to +10
	case *ast.BooleanLiteral:
		// Flip boolean
		e.Value = !e.Value
	case *ast.StringLiteral:
		// Mutate string content
		if len(e.Value) > 0 {
			runes := []rune(e.Value)
			idx := m.rnd.Intn(len(runes))
			runes[idx] = rune(m.rnd.Intn(128)) // Random ASCII char
			e.Value = string(runes)
		}
	case *ast.IfExpression:
		if m.rnd.Float32() < 0.33 {
			m.mutateExpression(e.Condition)
		} else if m.rnd.Float32() < 0.66 {
			m.mutateBlock(e.Consequence)
		} else if e.Alternative != nil {
			m.mutateBlock(e.Alternative)
		}
	case *ast.CallExpression:
		if len(e.Arguments) > 0 {
			idx := m.rnd.Intn(len(e.Arguments))
			m.mutateExpression(e.Arguments[idx])
		}
	}
}

func (m *ASTMutator) mutateBlock(block *ast.BlockStatement) {
	if block == nil {
		return
	}
	if len(block.Statements) == 0 {
		return
	}
	// Delete a random statement
	if m.rnd.Float32() < 0.1 {
		idx := m.rnd.Intn(len(block.Statements))
		block.Statements = append(block.Statements[:idx], block.Statements[idx+1:]...)
		return
	}

	// Mutate a random statement inside the block
	idx := m.rnd.Intn(len(block.Statements))
	stmt := block.Statements[idx]

	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		if s == nil || s.Expression == nil {
			return
		}
		m.mutateExpression(s.Expression)
	case *ast.BlockStatement:
		if s == nil {
			return
		}
		m.mutateBlock(s)
	case *ast.FunctionStatement:
		if s == nil {
			return
		}
		m.mutateFunction(s)
	case *ast.ConstantDeclaration:
		if s == nil {
			return
		}
		m.mutateConstantDeclaration(s)
	}
}

func (m *ASTMutator) mutateFunction(fn *ast.FunctionStatement) {
	if fn.Body != nil {
		m.mutateBlock(fn.Body)
	}
}

func (m *ASTMutator) randomOperator() string {
	ops := []string{"+", "-", "*", "/", "==", "!=", "<", ">", "<=", ">=", "&&", "||"}
	return ops[m.rnd.Intn(len(ops))]
}

func (m *ASTMutator) randomPrefixOperator() string {
	ops := []string{"-", "!"}
	return ops[m.rnd.Intn(len(ops))]
}

func (m *ASTMutator) mutateConstantDeclaration(stmt *ast.ConstantDeclaration) {
	if stmt.Value != nil {
		m.mutateExpression(stmt.Value)
	}
}

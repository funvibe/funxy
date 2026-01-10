package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/typesystem"
)

// ConstraintType represents the kind of constraint
type ConstraintType string

const (
	ConstraintUnify      ConstraintType = "Unify"      // T1 ~ T2
	ConstraintImplements ConstraintType = "Implements" // Trait(T1, T2...)
)

// Constraint represents a type constraint to be solved later
type Constraint struct {
	Kind  ConstraintType
	Left  typesystem.Type
	Right typesystem.Type   // For Unify: Right type.
	Trait string            // For Implements: Trait name
	Args  []typesystem.Type // For Implements: [T1, T2...]
	Node  ast.Node          // Source node for error reporting
}

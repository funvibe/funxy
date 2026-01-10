package evaluator

import (
	"github.com/funvibe/funxy/internal/typesystem"
)

// PushWitness pushes a witness dictionary onto the stack.
func (e *Evaluator) PushWitness(w map[string][]typesystem.Type) {
	e.WitnessStack = append(e.WitnessStack, w)
}

// PopWitness pops the top witness dictionary from the stack.
func (e *Evaluator) PopWitness() {
	if len(e.WitnessStack) > 0 {
		e.WitnessStack = e.WitnessStack[:len(e.WitnessStack)-1]
	}
}

// RestoreWitnessStack restores the witness stack to a specific depth.
func (e *Evaluator) RestoreWitnessStack(depth int) {
	if len(e.WitnessStack) > depth {
		e.WitnessStack = e.WitnessStack[:depth]
	}
}

// GetWitness looks up a witness type by key (Trait name or TypeVar name).
// Searches from the top of the stack down.
func (e *Evaluator) GetWitness(key string) []typesystem.Type {
	for i := len(e.WitnessStack) - 1; i >= 0; i-- {
		if t, ok := e.WitnessStack[i][key]; ok {
			return t
		}
	}
	return nil
}

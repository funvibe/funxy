package typesystem

import "fmt"

// SymbolNotFoundError indicates a symbol was not found
type SymbolNotFoundError struct {
	Name string
}

func (e *SymbolNotFoundError) Error() string {
	return fmt.Sprintf("symbol not found: %s", e.Name)
}

func NewSymbolNotFoundError(name string) *SymbolNotFoundError {
	return &SymbolNotFoundError{Name: name}
}

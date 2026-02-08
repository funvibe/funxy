package typesystem

type DispatchKind int

const (
	DispatchArg     DispatchKind = 0 // Argument index
	DispatchReturn  DispatchKind = 1 // Return context (outer type constructor)
	DispatchWitness DispatchKind = 2 // Witness stack (generic var)
	DispatchHint    DispatchKind = 3 // Explicit type hint required (phantom types)
)

type DispatchSource struct {
	Kind  DispatchKind
	Index int // Argument index (for DispatchArg)
}

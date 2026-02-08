package evaluator

import (
	"github.com/funvibe/funxy/internal/typesystem"
	"hash/fnv"
)

type ObjectType string

const (
	INTEGER_OBJ         = "INTEGER"
	FLOAT_OBJ           = "FLOAT"
	NIL_OBJ             = "NIL"
	ERROR_OBJ           = "ERROR"
	FUNCTION_OBJ        = "FUNCTION"
	BUILTIN_OBJ         = "BUILTIN"
	DATA_INSTANCE_OBJ   = "DATA_INSTANCE"
	CONSTRUCTOR_OBJ     = "CONSTRUCTOR"
	BOOLEAN_OBJ         = "BOOLEAN"
	TUPLE_OBJ           = "TUPLE"
	TYPE_OBJ            = "TYPE"
	LIST_OBJ            = "LIST"
	CHAR_OBJ            = "CHAR"
	RETURN_VALUE_OBJ    = "RETURN_VALUE"
	CLASS_METHOD_OBJ    = "CLASS_METHOD"    // New
	RECORD_OBJ          = "RECORD"          // New
	BREAK_SIGNAL_OBJ    = "BREAK_SIGNAL"    // New
	CONTINUE_SIGNAL_OBJ = "CONTINUE_SIGNAL" // New
	MAP_OBJ             = "MAP"             // Immutable hash map
	BYTES_OBJ           = "BYTES"           // Byte sequence
	BITS_OBJ            = "BITS"            // Bit sequence

	// Runtime Type Names (Canonical)
	RUNTIME_TYPE_INT        = "Int"
	RUNTIME_TYPE_FLOAT      = "Float"
	RUNTIME_TYPE_BOOL       = "Bool"
	RUNTIME_TYPE_CHAR       = "Char"
	RUNTIME_TYPE_STRING     = "String" // Not used directly as type of object, but conceptually
	RUNTIME_TYPE_LIST       = "List"
	RUNTIME_TYPE_TUPLE      = "TUPLE"
	RUNTIME_TYPE_RECORD     = "RECORD"
	RUNTIME_TYPE_FUNCTION   = "FUNCTION"
	RUNTIME_TYPE_RANGE      = "Range"        // New
	RUNTIME_TYPE_BIGINT     = "BigInt"       // New
	RUNTIME_TYPE_RATIONAL   = "Rational"     // New
	BOUND_METHOD_OBJ        = "BOUND_METHOD" // New
	TAIL_CALL_OBJ           = "TAIL_CALL"    // New for TCO
	BIG_INT_OBJ             = "BIG_INT"
	RATIONAL_OBJ            = "RATIONAL"
	COMPOSED_FUNC_OBJ       = "COMPOSED_FUNC"
	PARTIAL_APPLICATION_OBJ = "PARTIAL_APPLICATION"
	DICTIONARY_OBJ          = "DICTIONARY" // New: VTable for Type Classes
	RANGE_OBJ               = "RANGE"
	HOST_OBJ                = "HOST" // New: Host Object
)

type Object interface {
	Type() ObjectType
	Inspect() string
	RuntimeType() typesystem.Type // Returns the type system representation
	Hash() uint32
}

// Helper for hashing strings
func hashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

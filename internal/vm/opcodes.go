// Package vm implements a bytecode virtual machine for Funxy
package vm

// Opcode represents a single VM instruction
type Opcode byte

const (
	// Stack manipulation
	OP_CONST     Opcode = iota // Push constant from pool
	OP_POP                     // Discard top of stack
	OP_POP_BELOW               // Discard item below top N items: [..., val, a, b] -> [..., a, b]
	OP_DUP                     // Duplicate top of stack

	// Arithmetic
	OP_ADD // +
	OP_SUB // -
	OP_MUL // *
	OP_DIV // /
	OP_MOD // %
	OP_POW // **
	OP_NEG // Unary minus

	// Bitwise operations
	OP_BAND          // &
	OP_BOR           // |
	OP_BXOR          // ^
	OP_BNOT          // ~ (unary)
	OP_LEN           // len (unary)
	OP_INTERP_CONCAT // string interpolation concat
	OP_LSHIFT        // <<
	OP_RSHIFT        // >>

	// List/String operations
	OP_CONCAT // ++
	OP_CONS   // ::

	// Comparison
	OP_EQ // ==
	OP_NE // !=
	OP_LT // <
	OP_LE // <=
	OP_GT // >
	OP_GE // >=

	// Logic
	OP_NOT // !
	OP_AND // &&
	OP_OR  // ||

	// Variables (Phase 2)
	OP_GET_LOCAL  // Get local variable by index
	OP_SET_LOCAL  // Set local variable by index
	OP_GET_GLOBAL // Get global variable by name
	OP_SET_GLOBAL // Set global variable by name

	// Control flow (Phase 3)
	OP_JUMP          // Unconditional jump
	OP_JUMP_IF_FALSE // Jump if top of stack is false
	OP_LOOP          // Jump backward (for loops)

	// Functions (Phase 4)
	OP_CALL      // Call function
	OP_TAIL_CALL // Tail call optimization - reuse current frame
	OP_RETURN    // Return from function

	// Closures (Phase 6)
	OP_CLOSURE       // Create closure
	OP_GET_UPVALUE   // Get captured variable
	OP_SET_UPVALUE   // Set captured variable
	OP_CLOSE_UPVALUE // Close upvalue when leaving scope

	// Data structures (Phase 7)
	OP_MAKE_LIST            // Create list
	OP_MAKE_RECORD          // Create record
	OP_MAKE_TUPLE           // Create tuple
	OP_MAKE_MAP             // Create map
	OP_SPREAD               // Spread list/tuple elements
	OP_UNWRAP_OR_RETURN     // Unwrap Option/Result or early return
	OP_MATCH_STRING_PATTERN // Match string pattern with captures (legacy)
	OP_MATCH_STRING_EXTRACT // Match string, pop input, push bool + captures
	OP_TRAIT_OP             // Trait-based operator dispatch
	OP_EVAL_STMT            // Evaluate AST statement via evaluator
	OP_TUPLE_SLICE          // Get slice of tuple: [tuple, start] -> [slice]
	OP_LIST_SLICE           // Get slice of list: [list, start] -> [slice]
	OP_CHECK_TUPLE_LEN_GE   // Check tuple length >= N (for spread patterns)
	OP_SPREAD_ARG           // Mark argument as spread (to be unpacked)
	OP_CALL_SPREAD          // Call with spread arguments
	OP_COMPOSE              // Function composition: f ,, g
	OP_REGISTER_TRAIT       // Register trait method: [closure] traitIdx typeIdx methodIdx
	OP_CALL_TRAIT           // Call trait method for operator
	OP_DEFAULT              // Get default value for type
	OP_GET_FIELD            // Get record field
	OP_GET_INDEX            // Get list/map element
	OP_OPTIONAL_CHAIN_FIELD // Optional chaining: obj?.field
	OP_UNWRAP_OR_PANIC      // Unwrap Option/Result or panic (for |>> operator)

	// Pattern matching (Phase 7)
	OP_CHECK_TAG          // Check DataInstance.Name == constant, push bool
	OP_GET_DATA_FIELD     // Get DataInstance.Fields[index], push value
	OP_CHECK_LIST_LEN     // Check list length (==, >=), push bool
	OP_GET_LIST_REST      // Get rest of list from index
	OP_CHECK_TYPE         // Check if value is of given type
	OP_SET_FIELD          // Set field in record
	OP_SET_INDEX          // Set element in list/map
	OP_CALL_METHOD        // Call method on object (extension methods or field access + call)
	OP_COALESCE           // Null coalescing: push (unwrapped, true) or (original, false)
	OP_MAKE_ITER          // Convert iterable to iterator (handles Iter trait)
	OP_SET_TYPE_NAME      // Set TypeName on RecordInstance (for type annotations)
	OP_SET_LIST_ELEM_TYPE // Set ElementType on List (for List<T> annotations)
	OP_ITER_NEXT          // Get next item from iterator (handles both index-based and lazy)
	OP_GET_LIST_ELEM      // Get list element by index
	OP_CHECK_TUPLE_LEN    // Check tuple length, push bool
	OP_GET_TUPLE_ELEM     // Get tuple element by index
	OP_RANGE              // Create range object: [start, next?, end] -> [range]

	// Special
	OP_NIL   // Push nil
	OP_TRUE  // Push true
	OP_FALSE // Push false

	// Scope management
	OP_CLOSE_SCOPE // Close scope: pop n locals but keep result

	// Type context for ClassMethod dispatch
	OP_SET_TYPE_CONTEXT   // Set expected type for ClassMethod dispatch
	OP_CLEAR_TYPE_CONTEXT // Clear type context

	OP_EXTEND_RECORD // Extend record with new fields: [base, key, val, ...] -> [new_record]

	OP_REGISTER_EXTENSION  // Register extension method: [closure] typeNameIdx methodNameIdx
	OP_REGISTER_TYPE_ALIAS // Register type alias: [typeObject] nameIdx

	// Halt
	OP_HALT // Stop execution

	OP_AUTO_CALL // Auto-call nullary method if type context is set
	OP_FORMATTER // Create format string function: [constant_index] -> [closure]
)

// OpcodeNames maps opcodes to their string names (for debugging)
var OpcodeNames = map[Opcode]string{
	OP_CONST:     "CONST",
	OP_POP:       "POP",
	OP_POP_BELOW: "POP_BELOW",
	OP_DUP:       "DUP",

	OP_ADD: "ADD",
	OP_SUB: "SUB",
	OP_MUL: "MUL",
	OP_DIV: "DIV",
	OP_MOD: "MOD",
	OP_POW: "POW",
	OP_NEG: "NEG",

	OP_BAND:          "BAND",
	OP_BOR:           "BOR",
	OP_BXOR:          "BXOR",
	OP_BNOT:          "BNOT",
	OP_LEN:           "LEN",
	OP_INTERP_CONCAT: "INTERP_CONCAT",
	OP_LSHIFT:        "LSHIFT",
	OP_RSHIFT:        "RSHIFT",

	OP_CONCAT: "CONCAT",
	OP_CONS:   "CONS",

	OP_EQ: "EQ",
	OP_NE: "NE",
	OP_LT: "LT",
	OP_LE: "LE",
	OP_GT: "GT",
	OP_GE: "GE",

	OP_NOT: "NOT",
	OP_AND: "AND",
	OP_OR:  "OR",

	OP_GET_LOCAL:  "GET_LOCAL",
	OP_SET_LOCAL:  "SET_LOCAL",
	OP_GET_GLOBAL: "GET_GLOBAL",
	OP_SET_GLOBAL: "SET_GLOBAL",

	OP_JUMP:          "JUMP",
	OP_JUMP_IF_FALSE: "JUMP_IF_FALSE",
	OP_LOOP:          "LOOP",

	OP_CALL:      "CALL",
	OP_TAIL_CALL: "TAIL_CALL",
	OP_RETURN:    "RETURN",

	OP_CLOSURE:       "CLOSURE",
	OP_GET_UPVALUE:   "GET_UPVALUE",
	OP_SET_UPVALUE:   "SET_UPVALUE",
	OP_CLOSE_UPVALUE: "CLOSE_UPVALUE",

	OP_MAKE_LIST:            "MAKE_LIST",
	OP_MAKE_RECORD:          "MAKE_RECORD",
	OP_MAKE_TUPLE:           "MAKE_TUPLE",
	OP_MAKE_MAP:             "MAKE_MAP",
	OP_SPREAD:               "SPREAD",
	OP_UNWRAP_OR_RETURN:     "UNWRAP_OR_RETURN",
	OP_MATCH_STRING_PATTERN: "MATCH_STRING_PATTERN",
	OP_MATCH_STRING_EXTRACT: "MATCH_STRING_EXTRACT",
	OP_TRAIT_OP:             "TRAIT_OP",
	OP_EVAL_STMT:            "EVAL_STMT",
	OP_TUPLE_SLICE:          "TUPLE_SLICE",
	OP_LIST_SLICE:           "LIST_SLICE",
	OP_CHECK_TUPLE_LEN_GE:   "CHECK_TUPLE_LEN_GE",
	OP_SPREAD_ARG:           "SPREAD_ARG",
	OP_CALL_SPREAD:          "CALL_SPREAD",
	OP_COMPOSE:              "COMPOSE",
	OP_REGISTER_TRAIT:       "REGISTER_TRAIT",
	OP_CALL_TRAIT:           "CALL_TRAIT",
	OP_DEFAULT:              "DEFAULT",
	OP_GET_FIELD:            "GET_FIELD",
	OP_GET_INDEX:            "GET_INDEX",
	OP_OPTIONAL_CHAIN_FIELD: "OPTIONAL_CHAIN_FIELD",
	OP_UNWRAP_OR_PANIC:      "UNWRAP_OR_PANIC",

	OP_CHECK_TAG:          "CHECK_TAG",
	OP_GET_DATA_FIELD:     "GET_DATA_FIELD",
	OP_CHECK_LIST_LEN:     "CHECK_LIST_LEN",
	OP_GET_LIST_REST:      "GET_LIST_REST",
	OP_CHECK_TYPE:         "CHECK_TYPE",
	OP_SET_FIELD:          "SET_FIELD",
	OP_SET_INDEX:          "SET_INDEX",
	OP_CALL_METHOD:        "CALL_METHOD",
	OP_COALESCE:           "COALESCE",
	OP_SET_TYPE_NAME:      "SET_TYPE_NAME",
	OP_SET_LIST_ELEM_TYPE: "SET_LIST_ELEM_TYPE",
	OP_GET_LIST_ELEM:      "GET_LIST_ELEM",
	OP_CHECK_TUPLE_LEN:    "CHECK_TUPLE_LEN",
	OP_GET_TUPLE_ELEM:     "GET_TUPLE_ELEM",
	OP_RANGE:              "RANGE",

	OP_NIL:   "NIL",
	OP_TRUE:  "TRUE",
	OP_FALSE: "FALSE",

	OP_CLOSE_SCOPE:        "CLOSE_SCOPE",
	OP_SET_TYPE_CONTEXT:   "SET_TYPE_CONTEXT",
	OP_CLEAR_TYPE_CONTEXT: "CLEAR_TYPE_CONTEXT",

	OP_EXTEND_RECORD:       "EXTEND_RECORD",
	OP_REGISTER_EXTENSION:  "REGISTER_EXTENSION",
	OP_REGISTER_TYPE_ALIAS: "REGISTER_TYPE_ALIAS",

	OP_HALT:      "HALT",
	OP_AUTO_CALL: "AUTO_CALL",
	OP_FORMATTER: "FORMATTER",
}

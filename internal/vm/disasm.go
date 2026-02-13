package vm

import (
	"fmt"
	"strings"
)

// Disassemble returns a human-readable representation of the bytecode
func Disassemble(chunk *Chunk, name string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("== %s ==\n", name))

	offset := 0
	for offset < len(chunk.Code) {
		offset = disassembleInstruction(&sb, chunk, offset)
	}

	return sb.String()
}

// DisassembleInstruction disassembles a single instruction
func disassembleInstruction(sb *strings.Builder, chunk *Chunk, offset int) int {
	sb.WriteString(fmt.Sprintf("%04d ", offset))

	// Print line number
	if offset > 0 && chunk.Lines[offset] == chunk.Lines[offset-1] {
		sb.WriteString("   | ")
	} else {
		sb.WriteString(fmt.Sprintf("%4d ", chunk.Lines[offset]))
	}

	op := Opcode(chunk.Code[offset])

	switch op {
	case OP_CONST:
		return constantInstruction(sb, "CONST", chunk, offset)

	case OP_NIL:
		return simpleInstruction(sb, "NIL", offset)
	case OP_TRUE:
		return simpleInstruction(sb, "TRUE", offset)
	case OP_FALSE:
		return simpleInstruction(sb, "FALSE", offset)

	case OP_POP:
		return simpleInstruction(sb, "POP", offset)
	case OP_DUP:
		return simpleInstruction(sb, "DUP", offset)

	case OP_ADD:
		return simpleInstruction(sb, "ADD", offset)
	case OP_SUB:
		return simpleInstruction(sb, "SUB", offset)
	case OP_MUL:
		return simpleInstruction(sb, "MUL", offset)
	case OP_DIV:
		return simpleInstruction(sb, "DIV", offset)
	case OP_MOD:
		return simpleInstruction(sb, "MOD", offset)
	case OP_NEG:
		return simpleInstruction(sb, "NEG", offset)

	case OP_EQ:
		return simpleInstruction(sb, "EQ", offset)
	case OP_NE:
		return simpleInstruction(sb, "NE", offset)
	case OP_LT:
		return simpleInstruction(sb, "LT", offset)
	case OP_LE:
		return simpleInstruction(sb, "LE", offset)
	case OP_GT:
		return simpleInstruction(sb, "GT", offset)
	case OP_GE:
		return simpleInstruction(sb, "GE", offset)

	case OP_NOT:
		return simpleInstruction(sb, "NOT", offset)
	case OP_AND:
		return simpleInstruction(sb, "AND", offset)
	case OP_OR:
		return simpleInstruction(sb, "OR", offset)

	case OP_GET_LOCAL:
		return byteInstruction(sb, "GET_LOCAL", chunk, offset)
	case OP_SET_LOCAL:
		return byteInstruction(sb, "SET_LOCAL", chunk, offset)
	case OP_GET_GLOBAL:
		return constantInstruction(sb, "GET_GLOBAL", chunk, offset)
	case OP_SET_GLOBAL:
		return constantInstruction(sb, "SET_GLOBAL", chunk, offset)

	case OP_JUMP:
		return jumpInstruction(sb, "JUMP", 1, chunk, offset)
	case OP_JUMP_IF_FALSE:
		return jumpInstruction(sb, "JUMP_IF_FALSE", 1, chunk, offset)
	case OP_LOOP:
		return jumpInstruction(sb, "LOOP", -1, chunk, offset)

	case OP_CALL:
		return byteInstruction(sb, "CALL", chunk, offset)
	case OP_RETURN:
		return simpleInstruction(sb, "RETURN", offset)
	case OP_CLOSURE:
		return closureInstruction(sb, "CLOSURE", chunk, offset)
	case OP_GET_UPVALUE:
		return byteInstruction(sb, "GET_UPVALUE", chunk, offset)
	case OP_SET_UPVALUE:
		return byteInstruction(sb, "SET_UPVALUE", chunk, offset)
	case OP_CLOSE_UPVALUE:
		return simpleInstruction(sb, "CLOSE_UPVALUE", offset)

	case OP_CLOSE_SCOPE:
		return byteInstruction(sb, "CLOSE_SCOPE", chunk, offset)

	case OP_MAKE_LIST:
		return constantInstruction(sb, "MAKE_LIST", chunk, offset)

	case OP_CHECK_TAG:
		return constantInstruction(sb, "CHECK_TAG", chunk, offset)
	case OP_GET_DATA_FIELD:
		return byteInstruction(sb, "GET_DATA_FIELD", chunk, offset)
	case OP_CHECK_LIST_LEN:
		// op byte + 2-byte length
		opByte := chunk.Code[offset+1]
		length := int(chunk.Code[offset+2])<<8 | int(chunk.Code[offset+3])
		opStr := "=="
		if opByte == 1 {
			opStr = ">="
		}
		sb.WriteString(fmt.Sprintf("%-16s %s %d\n", "CHECK_LIST_LEN", opStr, length))
		return offset + 4
	case OP_GET_LIST_ELEM:
		return constantInstruction(sb, "GET_LIST_ELEM", chunk, offset)
	case OP_CHECK_TUPLE_LEN:
		return byteInstruction(sb, "CHECK_TUPLE_LEN", chunk, offset)
	case OP_GET_TUPLE_ELEM:
		return byteInstruction(sb, "GET_TUPLE_ELEM", chunk, offset)

	case OP_CALL_SPREAD:
		return byteInstruction(sb, "CALL_SPREAD", chunk, offset)
	case OP_COMPOSE:
		return simpleInstruction(sb, "COMPOSE", offset)
	case OP_SPREAD_ARG:
		return simpleInstruction(sb, "SPREAD_ARG", offset)
	case OP_MAKE_RECORD:
		// byte count + 2-byte typeNameIdx
		count := int(chunk.Code[offset+1])
		typeIdx := int(chunk.Code[offset+2])<<8 | int(chunk.Code[offset+3])
		sb.WriteString(fmt.Sprintf("%-16s %d (type %d)\n", "MAKE_RECORD", count, typeIdx))
		return offset + 4
	case OP_MAKE_TUPLE:
		return byteInstruction(sb, "MAKE_TUPLE", chunk, offset)
	case OP_MAKE_MAP:
		return byteInstruction(sb, "MAKE_MAP", chunk, offset)
	case OP_UNWRAP_OR_RETURN:
		return simpleInstruction(sb, "UNWRAP_OR_RETURN", offset)
	case OP_TRAIT_OP:
		return constantInstruction(sb, "TRAIT_OP", chunk, offset)
	case OP_CHECK_TUPLE_LEN_GE:
		return byteInstruction(sb, "CHECK_TUPLE_LEN_GE", chunk, offset)
	case OP_REGISTER_TRAIT:
		// [closure] traitIdx typeIdx methodIdx
		traitIdx := int(chunk.Code[offset+1])<<8 | int(chunk.Code[offset+2])
		typeIdx := int(chunk.Code[offset+3])<<8 | int(chunk.Code[offset+4])
		methodIdx := int(chunk.Code[offset+5])<<8 | int(chunk.Code[offset+6])
		sb.WriteString(fmt.Sprintf("%-16s %4d %4d %4d\n", "REGISTER_TRAIT", traitIdx, typeIdx, methodIdx))
		return offset + 7
	case OP_DEFAULT:
		return simpleInstruction(sb, "DEFAULT", offset)
	case OP_GET_FIELD:
		return constantInstruction(sb, "GET_FIELD", chunk, offset)
	case OP_GET_INDEX:
		return simpleInstruction(sb, "GET_INDEX", offset)
	case OP_OPTIONAL_CHAIN_FIELD:
		return constantInstruction(sb, "OPTIONAL_CHAIN_FIELD", chunk, offset)
	case OP_UNWRAP_OR_PANIC:
		return simpleInstruction(sb, "UNWRAP_OR_PANIC", offset)
	case OP_CHECK_TYPE:
		return constantInstruction(sb, "CHECK_TYPE", chunk, offset)
	case OP_SET_FIELD:
		return constantInstruction(sb, "SET_FIELD", chunk, offset)
	case OP_SET_INDEX:
		return simpleInstruction(sb, "SET_INDEX", offset)
	case OP_CALL_METHOD:
		// nameIdx (2) + argCount (1)
		nameIdx := int(chunk.Code[offset+1])<<8 | int(chunk.Code[offset+2])
		argCount := int(chunk.Code[offset+3])
		sb.WriteString(fmt.Sprintf("%-16s %4d (args: %d)\n", "CALL_METHOD", nameIdx, argCount))
		return offset + 4
	case OP_COALESCE:
		return simpleInstruction(sb, "COALESCE", offset)
	case OP_SET_TYPE_NAME:
		return constantInstruction(sb, "SET_TYPE_NAME", chunk, offset)
	case OP_SET_LIST_ELEM_TYPE:
		return constantInstruction(sb, "SET_LIST_ELEM_TYPE", chunk, offset)
	case OP_RANGE:
		return simpleInstruction(sb, "RANGE", offset)
	case OP_SET_TYPE_CONTEXT:
		return constantInstruction(sb, "SET_TYPE_CONTEXT", chunk, offset)
	case OP_CLEAR_TYPE_CONTEXT:
		return simpleInstruction(sb, "CLEAR_TYPE_CONTEXT", offset)
	case OP_EXTEND_RECORD:
		return byteInstruction(sb, "EXTEND_RECORD", chunk, offset)
	case OP_REGISTER_EXTENSION:
		// [closure] typeNameIdx methodNameIdx
		typeIdx := int(chunk.Code[offset+1])<<8 | int(chunk.Code[offset+2])
		methodIdx := int(chunk.Code[offset+3])<<8 | int(chunk.Code[offset+4])
		sb.WriteString(fmt.Sprintf("%-16s %4d %4d\n", "REGISTER_EXTENSION", typeIdx, methodIdx))
		return offset + 5
	case OP_REGISTER_TYPE_ALIAS:
		return constantInstruction(sb, "REGISTER_TYPE_ALIAS", chunk, offset)
	case OP_AUTO_CALL:
		return simpleInstruction(sb, "AUTO_CALL", offset)
	case OP_FORMATTER:
		return constantInstruction(sb, "FORMATTER", chunk, offset)
	case OP_TAIL_CALL:
		return byteInstruction(sb, "TAIL_CALL", chunk, offset)
	case OP_HALT:
		return simpleInstruction(sb, "HALT", offset)
	case OP_POP_BELOW:
		return byteInstruction(sb, "POP_BELOW", chunk, offset)
	default:
		sb.WriteString(fmt.Sprintf("Unknown opcode %d\n", op))
		return offset + 1
	}
}

func simpleInstruction(sb *strings.Builder, name string, offset int) int {
	sb.WriteString(fmt.Sprintf("%s\n", name))
	return offset + 1
}

func constantInstruction(sb *strings.Builder, name string, chunk *Chunk, offset int) int {
	idx := int(chunk.Code[offset+1])<<8 | int(chunk.Code[offset+2])

	if idx < len(chunk.Constants) {
		sb.WriteString(fmt.Sprintf("%-16s %4d '%s'\n", name, idx, chunk.Constants[idx].Inspect()))
	} else {
		sb.WriteString(fmt.Sprintf("%-16s %4d (invalid)\n", name, idx))
	}

	return offset + 3
}

func byteInstruction(sb *strings.Builder, name string, chunk *Chunk, offset int) int {
	slot := chunk.Code[offset+1]
	sb.WriteString(fmt.Sprintf("%-16s %4d\n", name, slot))
	return offset + 2
}

func jumpInstruction(sb *strings.Builder, name string, sign int, chunk *Chunk, offset int) int {
	jump := int(chunk.Code[offset+1])<<8 | int(chunk.Code[offset+2])
	target := offset + 3 + sign*jump
	sb.WriteString(fmt.Sprintf("%-16s %4d -> %d\n", name, jump, target))
	return offset + 3
}

func closureInstruction(sb *strings.Builder, name string, chunk *Chunk, offset int) int {
	idx := int(chunk.Code[offset+1])<<8 | int(chunk.Code[offset+2])
	offset += 3

	if idx >= len(chunk.Constants) {
		sb.WriteString(fmt.Sprintf("%-16s %4d (invalid)\n", name, idx))
		return offset
	}

	fn, ok := chunk.Constants[idx].(*CompiledFunction)
	if !ok {
		sb.WriteString(fmt.Sprintf("%-16s %4d (not a function)\n", name, idx))
		return offset
	}

	sb.WriteString(fmt.Sprintf("%-16s %4d '%s'\n", name, idx, fn.Inspect()))

	// Recursively disassemble the function chunk
	funcDisasm := Disassemble(fn.Chunk, fn.Name)
	// Indent the function disassembly
	indented := strings.ReplaceAll(funcDisasm, "\n", "\n    | ")
	sb.WriteString("    | " + indented + "\n")

	// Print upvalue info
	for i := 0; i < fn.UpvalueCount; i++ {
		isLocal := chunk.Code[offset]
		index := chunk.Code[offset+1]
		offset += 2

		var localStr string
		if isLocal == 1 {
			localStr = "local"
		} else {
			localStr = "upvalue"
		}
		sb.WriteString(fmt.Sprintf("%04d    |                     %s %d\n", offset-2, localStr, index))
	}

	return offset
}

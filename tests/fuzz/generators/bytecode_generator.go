package generators

import (
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/vm"
)

// BytecodeGenerator generates random VM bytecode.
type BytecodeGenerator struct {
	*Generator
}

// NewBytecodeGenerator creates a new bytecode generator.
func NewBytecodeGenerator(data []byte) *BytecodeGenerator {
	return &BytecodeGenerator{
		Generator: NewFromData(data),
	}
}

// GenerateChunk creates a random VM chunk.
func (g *BytecodeGenerator) GenerateChunk() *vm.Chunk {
	chunk := vm.NewChunk()

	// 1. Generate Constants Pool
	constCount := g.Src().Intn(20) + 1
	for i := 0; i < constCount; i++ {
		chunk.AddConstant(g.GenerateConstant())
	}

	// 2. Generate Bytecode
	codeLen := g.Src().Intn(100) + 10
	for i := 0; i < codeLen; i++ {
		// Pick a random opcode
		op := vm.Opcode(g.Src().Intn(256))
		line := g.Src().Intn(100) + 1

		// Write opcode
		chunk.WriteOp(op, line)

		// Handle operands based on opcode (to make it semi-valid)
		// or just write random bytes (to test robustness against malformed bytecode)
		// We'll do a mix: 80% semi-valid, 20% pure random
		if g.Src().Intn(5) != 0 {
			g.writeOperands(chunk, op, line, constCount)
		} else {
			// Random extra bytes
			extra := g.Src().Intn(3)
			for j := 0; j < extra; j++ {
				chunk.Write(byte(g.Src().Intn(256)), line)
			}
		}
	}

	return chunk
}

func (g *BytecodeGenerator) GenerateConstant() evaluator.Object {
	switch g.Src().Intn(6) {
	case 0:
		return &evaluator.Integer{Value: int64(g.Src().Intn(1000))}
	case 1:
		return &evaluator.Float{Value: g.Src().Float64() * 1000}
	case 2:
		return &evaluator.Boolean{Value: g.Src().Intn(2) == 0}
	case 3:
		return evaluator.StringToList(g.GenerateIdentifier()) // Reuse identifier gen for strings
	case 4:
		return &evaluator.Nil{}
	default:
		return &evaluator.Integer{Value: 0}
	}
}

func (g *BytecodeGenerator) writeOperands(chunk *vm.Chunk, op vm.Opcode, line int, constCount int) {
	switch op {
	case vm.OP_CONST, vm.OP_GET_GLOBAL, vm.OP_SET_GLOBAL, vm.OP_MAKE_RECORD, vm.OP_GET_FIELD, vm.OP_SET_FIELD, vm.OP_CALL_METHOD:
		// 2-byte constant index
		idx := g.Src().Intn(constCount)
		chunk.Write(byte(idx>>8), line)
		chunk.Write(byte(idx), line)

	case vm.OP_JUMP, vm.OP_JUMP_IF_FALSE, vm.OP_LOOP:
		// 2-byte jump offset
		offset := g.Src().Intn(100) // Short jumps
		chunk.Write(byte(offset>>8), line)
		chunk.Write(byte(offset), line)

	case vm.OP_GET_LOCAL, vm.OP_SET_LOCAL, vm.OP_CALL, vm.OP_MAKE_LIST, vm.OP_MAKE_TUPLE, vm.OP_MAKE_MAP:
		// 1-byte operand
		chunk.Write(byte(g.Src().Intn(256)), line)

	case vm.OP_CLOSURE:
		// Constant index for function + upvalues
		idx := g.Src().Intn(constCount)
		chunk.Write(byte(idx>>8), line)
		chunk.Write(byte(idx), line)
		// We don't know how many upvalues, so just write a few random bytes
		// This will likely cause the VM to misinterpret subsequent opcodes, which is good for fuzzing
		count := g.Src().Intn(5)
		for i := 0; i < count; i++ {
			chunk.Write(byte(g.Src().Intn(256)), line)
		}
	}
}

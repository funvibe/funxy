package vm

import "github.com/funvibe/funxy/internal/evaluator"

// Chunk represents a sequence of bytecode instructions
type Chunk struct {
	// Code is the bytecode instructions
	Code []byte
	
	// Constants pool - literals, function names, etc.
	Constants []evaluator.Object
	
	// Lines maps bytecode offset to source line number (for errors)
	Lines []int
	
	// Columns maps bytecode offset to source column number (for errors)
	Columns []int
	
	// File is the source file name
	File string
}

// NewChunk creates a new empty chunk
func NewChunk() *Chunk {
	return &Chunk{
		Code:      make([]byte, 0, 256),
		Constants: make([]evaluator.Object, 0, 64),
		Lines:     make([]int, 0, 256),
		Columns:   make([]int, 0, 256),
	}
}

// Write adds a byte to the chunk with line info (column defaults to 0)
func (c *Chunk) Write(b byte, line int) {
	c.Code = append(c.Code, b)
	c.Lines = append(c.Lines, line)
	c.Columns = append(c.Columns, 0)
}

// WriteWithCol adds a byte to the chunk with line and column info
func (c *Chunk) WriteWithCol(b byte, line, col int) {
	c.Code = append(c.Code, b)
	c.Lines = append(c.Lines, line)
	c.Columns = append(c.Columns, col)
}

// WriteOp writes an opcode to the chunk
func (c *Chunk) WriteOp(op Opcode, line int) {
	c.Write(byte(op), line)
}

// WriteOpWithCol writes an opcode to the chunk with column info
func (c *Chunk) WriteOpWithCol(op Opcode, line, col int) {
	c.WriteWithCol(byte(op), line, col)
}

// AddConstant adds a constant to the pool and returns its index
func (c *Chunk) AddConstant(value evaluator.Object) int {
	c.Constants = append(c.Constants, value)
	return len(c.Constants) - 1
}

// WriteConstant writes OP_CONST followed by the constant index
func (c *Chunk) WriteConstant(value evaluator.Object, line int) {
	idx := c.AddConstant(value)
	c.WriteOp(OP_CONST, line)
	// Write index as 2 bytes (allows up to 65535 constants)
	c.Write(byte(idx>>8), line)
	c.Write(byte(idx), line)
}

// ReadConstantIndex reads a 2-byte constant index at offset
func (c *Chunk) ReadConstantIndex(offset int) int {
	return int(c.Code[offset])<<8 | int(c.Code[offset+1])
}

// Len returns the number of bytes in the chunk
func (c *Chunk) Len() int {
	return len(c.Code)
}



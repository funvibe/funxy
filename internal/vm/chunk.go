package vm

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/typesystem"
)

func init() {
	// Register types with gob for serialization
	gob.Register(&CompiledFunction{})
	gob.Register(&Chunk{})

	// Register evaluator types that are serializable (no Environment/Evaluator refs)
	gob.Register(&evaluator.Integer{})
	gob.Register(&evaluator.Float{})
	gob.Register(&evaluator.Boolean{})
	gob.Register(&evaluator.Nil{})
	gob.Register(&evaluator.Char{})
	gob.Register(&evaluator.Bytes{})
	gob.Register(&evaluator.Bits{})
	gob.Register(&evaluator.List{})
	gob.Register(&evaluator.Tuple{})
	gob.Register(&evaluator.RecordInstance{})
	gob.Register(&evaluator.Map{})
	gob.Register(&evaluator.BigInt{})
	gob.Register(&evaluator.Rational{})
	gob.Register(&evaluator.DataInstance{})
	gob.Register(&evaluator.Constructor{})
	gob.Register(&evaluator.ClassMethod{})
	gob.Register(&evaluator.TypeObject{})
	// Custom serialized types (implement GobEncode/GobDecode):
	gob.Register(&evaluator.OperatorFunction{}) // Only serializes Operator field
	// NOT registered (contain Environment/Evaluator and not used in bytecode):
	// - evaluator.Function (has Env *Environment)
	// - evaluator.ComposedFunction (has Evaluator *Evaluator)
	// - evaluator.PartialApplication (has *Function)
	// - evaluator.BoundMethod (has *Function)

	// Note: PersistentVector is NOT registered here because it implements
	// GobEncode/GobDecode methods for custom serialization

	// Register typesystem types
	gob.Register(typesystem.TCon{})
	gob.Register(typesystem.TVar{})
	gob.Register(typesystem.TApp{})
	gob.Register(typesystem.TFunc{})
	gob.Register(typesystem.TRecord{})
	gob.Register(typesystem.TTuple{})
	gob.Register(typesystem.TUnion{})
	gob.Register(typesystem.TType{})
	gob.Register(typesystem.Constraint{})

	// Register stringConstant - it's defined in compiler_expressions.go
	// but needs to be registered here for gob serialization
	gob.Register(&stringConstant{})
}

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

	// PendingImports stores imports needed by this chunk (for compiled bytecode)
	PendingImports []PendingImport
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

// BytecodeFile represents the complete serialized bytecode file
type BytecodeFile struct {
	Magic   [4]byte // "FXYB"
	Version byte    // 0x01
	Chunk   *Chunk
}

// Serialize converts a Chunk to a binary format using gob encoding
// Format:
// - Magic number (4 bytes): 0x46585942 ("FXYB")
// - Version (1 byte): 0x01
// - Gob-encoded Chunk data
func (c *Chunk) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)

	// Magic number
	buf.Write([]byte{0x46, 0x58, 0x59, 0x42}) // "FXYB"

	// Version
	buf.WriteByte(0x01)

	// Encode the chunk using gob
	enc := gob.NewEncoder(buf)
	if err := enc.Encode(c); err != nil {
		return nil, fmt.Errorf("gob encoding failed: %w", err)
	}

	return buf.Bytes(), nil
}

// Deserialize reconstructs a Chunk from binary format
func Deserialize(data []byte) (*Chunk, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("data too short")
	}

	// Check magic number
	if data[0] != 0x46 || data[1] != 0x58 || data[2] != 0x59 || data[3] != 0x42 {
		return nil, fmt.Errorf("invalid magic number, expected FXYB")
	}

	// Check version
	if data[4] != 0x01 {
		return nil, fmt.Errorf("unsupported bytecode version: %d", data[4])
	}

	// Decode the chunk
	buf := bytes.NewReader(data[5:])
	dec := gob.NewDecoder(buf)
	var chunk Chunk
	if err := dec.Decode(&chunk); err != nil {
		return nil, fmt.Errorf("gob decoding failed: %w", err)
	}

	return &chunk, nil
}

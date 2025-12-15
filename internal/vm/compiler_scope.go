package vm

import "github.com/funvibe/funxy/internal/evaluator"

// beginScope starts a new scope
func (c *Compiler) beginScope() {
	c.scopeDepth++
}

// endScope ends the current scope and emits cleanup code
func (c *Compiler) endScope(line int) {
	c.scopeDepth--

	for c.localCount > 0 && c.locals[c.localCount-1].Depth > c.scopeDepth {
		if c.locals[c.localCount-1].IsCaptured {
			c.emit(OP_CLOSE_UPVALUE, line)
		} else {
			c.emit(OP_POP, line)
		}
		c.slotCount--
		c.localCount--
	}
}

// endScopeNoEmit closes scope without emitting POP instructions
func (c *Compiler) endScopeNoEmit() {
	c.scopeDepth--
	for c.localCount > 0 && c.locals[c.localCount-1].Depth > c.scopeDepth {
		c.localCount--
	}
}

// addLocal adds a local variable to the current scope
func (c *Compiler) addLocal(name string, slot int) {
	if c.localCount >= 256 {
		panic("too many local variables in function")
	}
	c.locals[c.localCount] = Local{
		Name:  name,
		Depth: c.scopeDepth,
		Slot:  slot,
	}
	c.localCount++
}

// removeSlotFromStack decreases slotCount and shifts down slot indices of all locals above the removed slot
func (c *Compiler) removeSlotFromStack(removedSlot int) {
	c.slotCount--
	for i := 0; i < c.localCount; i++ {
		if c.locals[i].Slot > removedSlot {
			c.locals[i].Slot--
		}
	}
}

// resolveLocal looks up a local variable by name
func (c *Compiler) resolveLocal(name string) int {
	for i := c.localCount - 1; i >= 0; i-- {
		if c.locals[i].Name == name {
			return c.locals[i].Slot
		}
	}
	return -1
}

// resolveLocalIndex returns both the slot AND the local's index
func (c *Compiler) resolveLocalIndex(name string) (slot int, localIdx int) {
	for i := c.localCount - 1; i >= 0; i-- {
		if c.locals[i].Name == name {
			return c.locals[i].Slot, i
		}
	}
	return -1, -1
}

// resolveUpvalue looks for a variable in enclosing scopes
func (c *Compiler) resolveUpvalue(name string) int {
	if c.enclosing == nil {
		return -1
	}

	slot, localIdx := c.enclosing.resolveLocalIndex(name)
	if slot != -1 {
		c.enclosing.locals[localIdx].IsCaptured = true
		return c.addUpvalue(uint8(slot), true)
	}

	upvalue := c.enclosing.resolveUpvalue(name)
	if upvalue != -1 {
		return c.addUpvalue(uint8(upvalue), false)
	}

	return -1
}

// addUpvalue adds an upvalue to this function's upvalue list
func (c *Compiler) addUpvalue(index uint8, isLocal bool) int {
	for i := 0; i < c.upvalueCount; i++ {
		if c.upvalues[i].Index == index && c.upvalues[i].IsLocal == isLocal {
			return i
		}
	}

	if c.upvalueCount >= 256 {
		panic("too many closure variables in function")
	}

	c.upvalues[c.upvalueCount] = Upvalue{
		Index:   index,
		IsLocal: isLocal,
	}
	c.upvalueCount++
	return c.upvalueCount - 1
}

// emit helpers

func (c *Compiler) emit(op Opcode, line int) {
	c.currentChunk().WriteOp(op, line)
}

func (c *Compiler) emitWithCol(op Opcode, line, col int) {
	c.currentChunk().WriteOpWithCol(op, line, col)
}

func (c *Compiler) emitConstant(value evaluator.Object, line int) {
	c.currentChunk().WriteConstant(value, line)
}

func (c *Compiler) emitJump(op Opcode, line int) int {
	c.emit(op, line)
	c.currentChunk().Write(0xff, line)
	c.currentChunk().Write(0xff, line)
	return c.currentChunk().Len() - 2
}

func (c *Compiler) patchJump(offset int) {
	jump := c.currentChunk().Len() - offset - 2

	if jump > 0xffff {
		panic("jump too far")
	}

	c.currentChunk().Code[offset] = byte(jump >> 8)
	c.currentChunk().Code[offset+1] = byte(jump)
}

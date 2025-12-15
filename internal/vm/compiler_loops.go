package vm

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/evaluator"
)

// emitLoop emits a backward jump to loopStart
func (c *Compiler) emitLoop(loopStart int, line int) {
	c.emit(OP_LOOP, line)

	offset := c.currentChunk().Len() - loopStart + 2
	if offset > 0xffff {
		panic("loop body too large")
	}

	c.currentChunk().Write(byte(offset>>8), line)
	c.currentChunk().Write(byte(offset), line)
}

// compileForExpression compiles for/while expression
func (c *Compiler) compileForExpression(expr *ast.ForExpression) error {
	line := expr.Token.Line

	// While-style loop: for condition { body }
	if expr.Condition != nil {
		return c.compileWhileLoop(expr)
	}

	// For-in style loop: for item in iterable { body }
	if expr.ItemName != nil && expr.Iterable != nil {
		return c.compileForInLoop(expr)
	}

	return fmt.Errorf("invalid for expression at line %d", line)
}

// compileWhileLoop compiles a while-style loop
func (c *Compiler) compileWhileLoop(expr *ast.ForExpression) error {
	line := expr.Token.Line

	// Save slot count before loop
	slotCountBeforeLoop := c.slotCount

	// Push nil as the initial result (loop result)
	c.emit(OP_NIL, line)
	c.slotCount++

	// Remember loop start for continue and loop back
	loopStart := c.currentChunk().Len()

	// Push loop context for break/continue
	c.loopStack = append(c.loopStack, LoopContext{
		loopStart:  loopStart,
		breakJumps: nil,
		scopeDepth: c.scopeDepth,
		localCount: c.localCount,
		slotCount:  slotCountBeforeLoop,
	})

	// Compile condition
	if err := c.compileExpression(expr.Condition); err != nil {
		return err
	}

	// Jump out of loop if condition is false
	exitJump := c.emitJump(OP_JUMP_IF_FALSE, line)
	c.emit(OP_POP, line) // Pop condition
	c.slotCount--

	// Pop previous result (we'll compute new one)
	c.emit(OP_POP, line)
	c.slotCount--

	// Compile body
	if err := c.compileBlockExpression(expr.Body); err != nil {
		return err
	}

	// Loop back to condition
	c.emitLoop(loopStart, line)

	// Patch exit jump (condition was false)
	c.patchJump(exitJump)
	c.emit(OP_POP, line) // Pop condition (it was false)
	c.slotCount--
	// Now loop_result is on top of stack

	// Patch all break jumps to here (after condition pop)
	loopCtx := c.loopStack[len(c.loopStack)-1]
	for _, breakJump := range loopCtx.breakJumps {
		c.patchJump(breakJump)
	}
	c.loopStack = c.loopStack[:len(c.loopStack)-1]

	// Result is on the stack (either nil, last body value, or break value)
	c.slotCount++
	return nil
}

// compileForInLoop compiles: for item in iterable { body }
func (c *Compiler) compileForInLoop(expr *ast.ForExpression) error {
	line := expr.Token.Line

	// Save slot count before loop vars for break cleanup
	slotCountBeforeLoop := c.slotCount

	// Begin scope for loop variables
	c.beginScope()

	// Compile iterable and store in local
	if err := c.compileExpression(expr.Iterable); err != nil {
		return err
	}
	// OP_MAKE_ITER: converts iterable to [iterable/iterator, length/-1]
	c.emit(OP_MAKE_ITER, line)
	iterableSlot := c.slotCount - 1
	c.addLocal("$iterable", iterableSlot)

	// OP_MAKE_ITER pushes length (or -1 for iterators)
	c.slotCount++
	lenSlot := c.slotCount - 1
	c.addLocal("$len", lenSlot)

	// Create index variable starting at 0
	c.emitConstant(&evaluator.Integer{Value: 0}, line)
	c.slotCount++
	indexSlot := c.slotCount - 1
	c.addLocal("$index", indexSlot)

	// Push nil as initial result
	c.emit(OP_NIL, line)
	c.slotCount++
	resultSlot := c.slotCount - 1

	// Create item slot (initially nil, will be updated each iteration)
	c.emit(OP_NIL, line)
	c.slotCount++
	itemSlot := c.slotCount - 1
	c.addLocal(expr.ItemName.Value, itemSlot)

	// Loop start
	loopStart := c.currentChunk().Len()

	// Push loop context
	c.loopStack = append(c.loopStack, LoopContext{
		loopStart:  loopStart,
		breakJumps: nil,
		scopeDepth: c.scopeDepth,
		localCount: c.localCount,
		slotCount:  slotCountBeforeLoop,
	})

	// OP_ITER_NEXT: gets next item or signals end
	c.emit(OP_ITER_NEXT, line)
	c.currentChunk().Write(byte(iterableSlot), line)
	c.currentChunk().Write(byte(lenSlot), line)
	c.currentChunk().Write(byte(indexSlot), line)
	c.slotCount += 2

	// Save slotCount after ITER_NEXT for exit label
	slotCountAfterIterNext := c.slotCount

	// Exit if continue_flag is false
	exitJump := c.emitJump(OP_JUMP_IF_FALSE, line)
	c.emit(OP_POP, line) // Pop continue_flag (true)
	c.slotCount--

	// Store item in itemSlot
	c.emit(OP_SET_LOCAL, line)
	c.currentChunk().Write(byte(itemSlot), line)
	c.emit(OP_POP, line)
	c.slotCount--

	// Compile body
	if err := c.compileBlockExpression(expr.Body); err != nil {
		return err
	}

	// Store body result
	c.emit(OP_SET_LOCAL, line)
	c.currentChunk().Write(byte(resultSlot), line)
	c.emit(OP_POP, line)
	c.slotCount--

	// Loop back
	c.emitLoop(loopStart, line)

	// Exit label
	c.patchJump(exitJump)
	c.slotCount = slotCountAfterIterNext
	c.emit(OP_POP, line) // Pop continue_flag (false)
	c.slotCount--
	c.emit(OP_POP, line) // Pop item (nil)
	c.slotCount--

	// Pop loop context before patching
	loopCtx := c.loopStack[len(c.loopStack)-1]
	c.loopStack = c.loopStack[:len(c.loopStack)-1]

	// Get result before ending scope
	c.emit(OP_GET_LOCAL, line)
	c.currentChunk().Write(byte(resultSlot), line)
	c.slotCount++

	// End scope - pop loop variables
	c.emit(OP_CLOSE_SCOPE, line)
	c.currentChunk().Write(byte(5), line)
	c.slotCount -= 5
	c.endScopeNoEmit()

	// Patch break jumps AFTER cleanup
	for _, jump := range loopCtx.breakJumps {
		c.patchJump(jump)
	}

	return nil
}

// compileBreakStatement compiles break statement
func (c *Compiler) compileBreakStatement(stmt *ast.BreakStatement) error {
	if len(c.loopStack) == 0 {
		return fmt.Errorf("break outside of loop")
	}

	line := stmt.Token.Line
	loopCtx := &c.loopStack[len(c.loopStack)-1]

	// Push break value (or nil if none)
	if stmt.Value != nil {
		if err := c.compileExpression(stmt.Value); err != nil {
			return err
		}
	} else {
		c.emit(OP_NIL, line)
		c.slotCount++
	}

	// Calculate slots to close
	slotsToClose := c.slotCount - loopCtx.slotCount - 1
	if slotsToClose > 0 {
		c.emit(OP_CLOSE_SCOPE, line)
		c.currentChunk().Write(byte(slotsToClose), line)
	}

	// Jump to after loop (will be patched)
	jumpOffset := c.emitJump(OP_JUMP, line)
	loopCtx.breakJumps = append(loopCtx.breakJumps, jumpOffset)

	// Reset slotCount for unreachable code path
	c.slotCount = loopCtx.slotCount + 1
	return nil
}

// compileContinueStatement compiles continue statement
func (c *Compiler) compileContinueStatement(stmt *ast.ContinueStatement) error {
	if len(c.loopStack) == 0 {
		return fmt.Errorf("continue outside of loop")
	}

	line := stmt.Token.Line
	loopCtx := &c.loopStack[len(c.loopStack)-1]

	// Push nil as "result" for this iteration
	c.emit(OP_NIL, line)
	c.slotCount++

	// Close any locals defined since loop started
	localsToClose := c.localCount - loopCtx.localCount
	if localsToClose > 0 {
		c.emit(OP_CLOSE_SCOPE, line)
		c.currentChunk().Write(byte(localsToClose), line)
	}

	// Jump back to loop start
	c.emitLoop(loopCtx.loopStart, line)

	// Code after continue is unreachable
	c.emit(OP_NIL, line)
	c.slotCount++
	return nil
}


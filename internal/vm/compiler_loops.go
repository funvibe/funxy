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

// compileListComprehension compiles a list comprehension expression
// [output | clause, clause, ...]
// This is compiled as nested loops that build a result list
func (c *Compiler) compileListComprehension(expr *ast.ListComprehension) error {
	line := expr.Token.Line

	// Create empty result list using OP_MAKE_LIST with 0 elements
	c.emit(OP_MAKE_LIST, line)
	c.currentChunk().Write(byte(0), line) // high byte of count
	c.currentChunk().Write(byte(0), line) // low byte of count
	c.slotCount++
	resultSlot := c.slotCount - 1

	// Add local for result list so we can update it
	c.beginScope()
	c.addLocal("$comp_result", resultSlot)

	// Compile clauses recursively
	if err := c.compileCompClauses(expr.Clauses, 0, expr.Output, resultSlot, line); err != nil {
		return err
	}

	// Get result list back on top of stack
	c.emit(OP_GET_LOCAL, line)
	c.currentChunk().Write(byte(resultSlot), line)
	c.slotCount++

	// End scope - clean up but keep result on stack
	c.emit(OP_CLOSE_SCOPE, line)
	c.currentChunk().Write(byte(1), line) // 1 local to close (the result list slot)
	c.slotCount--
	c.endScopeNoEmit()

	return nil
}

// compileCompClauses recursively compiles comprehension clauses
func (c *Compiler) compileCompClauses(clauses []ast.CompClause, idx int, output ast.Expression, resultSlot int, line int) error {
	if idx >= len(clauses) {
		// All clauses processed, compile output and append to result
		// Strategy: get result list, compile output, make single-element list, concat

		// Get result list first
		c.emit(OP_GET_LOCAL, line)
		c.currentChunk().Write(byte(resultSlot), line)
		c.slotCount++
		// Stack: [..., result_list]

		// Compile output expression
		if err := c.compileExpression(output); err != nil {
			return err
		}
		// Stack: [..., result_list, output_value]

		// Create single-element list from output
		c.emit(OP_MAKE_LIST, line)
		c.currentChunk().Write(byte(0), line) // high byte
		c.currentChunk().Write(byte(1), line) // low byte = 1 element
		// Stack: [..., result_list, [output_value]]

		// Concat: result_list ++ [output_value]
		c.emit(OP_CONCAT, line)
		c.slotCount-- // consumes 2, pushes 1
		// Stack: [..., new_result_list]

		// Store updated list back
		c.emit(OP_SET_LOCAL, line)
		c.currentChunk().Write(byte(resultSlot), line)
		c.emit(OP_POP, line)
		c.slotCount--
		// Stack: [...] (back to state before this clause)

		return nil
	}

	clause := clauses[idx]

	switch cl := clause.(type) {
	case *ast.CompGenerator:
		// Compile generator: pattern <- iterable
		return c.compileCompGenerator(cl, clauses, idx, output, resultSlot, line)

	case *ast.CompFilter:
		// Compile filter: boolean expression
		return c.compileCompFilter(cl, clauses, idx, output, resultSlot, line)

	default:
		return fmt.Errorf("unknown comprehension clause type: %T", clause)
	}
}

// compileCompGenerator compiles a generator clause (pattern <- iterable)
func (c *Compiler) compileCompGenerator(gen *ast.CompGenerator, clauses []ast.CompClause, idx int, output ast.Expression, resultSlot int, line int) error {
	// Begin scope for generator variables
	c.beginScope()

	// Compile iterable
	if err := c.compileExpression(gen.Iterable); err != nil {
		return err
	}

	// Convert to iterator
	// OP_MAKE_ITER pops iterable, pushes (iter, len) - net +1 on stack
	c.emit(OP_MAKE_ITER, line)
	// After MAKE_ITER: stack has iter at slotCount-1, but we need to account for len too
	// The iter replaces the iterable, and len is pushed on top
	// So: iter is at slotCount-1, len will be at slotCount
	iterableSlot := c.slotCount - 1
	c.addLocal("$comp_iter", iterableSlot)

	// Length slot - MAKE_ITER pushed this
	c.slotCount++ // Account for the length that MAKE_ITER pushed
	lenSlot := c.slotCount - 1
	c.addLocal("$comp_len", lenSlot)

	// Index variable
	c.emitConstant(&evaluator.Integer{Value: 0}, line)
	c.slotCount++
	indexSlot := c.slotCount - 1
	c.addLocal("$comp_idx", indexSlot)

	// Item slot
	c.emit(OP_NIL, line)
	c.slotCount++
	itemSlot := c.slotCount - 1

	// For complex patterns, pre-allocate slots for pattern variables BEFORE the loop
	// This ensures the slots are stable across iterations
	var patternSlots map[string]int
	if _, ok := gen.Pattern.(*ast.IdentifierPattern); !ok {
		patternSlots = make(map[string]int)
		c.allocatePatternSlots(gen.Pattern, patternSlots, line)
	}

	// Loop start
	loopStart := c.currentChunk().Len()

	// Get next item
	c.emit(OP_ITER_NEXT, line)
	c.currentChunk().Write(byte(iterableSlot), line)
	c.currentChunk().Write(byte(lenSlot), line)
	c.currentChunk().Write(byte(indexSlot), line)
	c.slotCount += 2

	// Save slotCount after ITER_NEXT for exit label
	slotCountAfterIterNext := c.slotCount

	// Exit if done
	exitJump := c.emitJump(OP_JUMP_IF_FALSE, line)
	c.emit(OP_POP, line) // Pop continue flag
	c.slotCount--

	// Store item
	c.emit(OP_SET_LOCAL, line)
	c.currentChunk().Write(byte(itemSlot), line)
	c.emit(OP_POP, line)
	c.slotCount--

	// Bind pattern variables
	// For simple identifier pattern, just add local pointing to itemSlot
	if identPat, ok := gen.Pattern.(*ast.IdentifierPattern); ok {
		if identPat.Value != "_" {
			c.addLocal(identPat.Value, itemSlot)
		}
	} else {
		// For complex patterns, extract values into pre-allocated slots
		if err := c.extractPatternValues(gen.Pattern, itemSlot, patternSlots, line); err != nil {
			return err
		}
	}

	// Compile remaining clauses
	if err := c.compileCompClauses(clauses, idx+1, output, resultSlot, line); err != nil {
		return err
	}

	// Loop back
	c.emitLoop(loopStart, line)

	// Exit label
	c.patchJump(exitJump)
	c.slotCount = slotCountAfterIterNext // Reset slotCount for exit path
	c.emit(OP_POP, line)                 // Pop continue flag (false)
	c.slotCount--
	c.emit(OP_POP, line) // Pop item from ITER_NEXT
	c.slotCount--

	// Clean up generator variables: item slot, index, len, iter + pattern slots
	// We can't use OP_CLOSE_SCOPE because it expects a result on top of stack
	// Just pop them manually

	// Pop pattern variable slots (if any)
	for range patternSlots {
		c.emit(OP_POP, line)
		c.slotCount--
	}

	c.emit(OP_POP, line) // Pop item slot
	c.slotCount--
	c.emit(OP_POP, line) // Pop index
	c.slotCount--
	c.emit(OP_POP, line) // Pop len
	c.slotCount--
	c.emit(OP_POP, line) // Pop iter
	c.slotCount--
	c.endScopeNoEmit()

	return nil
}

// allocatePatternSlots pre-allocates stack slots for pattern variables
// This is called BEFORE the loop to ensure stable slot positions
func (c *Compiler) allocatePatternSlots(pattern ast.Pattern, slots map[string]int, line int) {
	switch p := pattern.(type) {
	case *ast.TuplePattern:
		for _, elem := range p.Elements {
			c.allocatePatternSlots(elem, slots, line)
		}
	case *ast.RecordPattern:
		for _, fieldPattern := range p.Fields {
			c.allocatePatternSlots(fieldPattern, slots, line)
		}
	case *ast.ListPattern:
		for _, elem := range p.Elements {
			c.allocatePatternSlots(elem, slots, line)
		}
	case *ast.IdentifierPattern:
		if p.Value != "_" {
			// Allocate a slot for this variable
			c.emit(OP_NIL, line)
			c.slotCount++
			slot := c.slotCount - 1
			slots[p.Value] = slot
			c.addLocal(p.Value, slot)
		}
	case *ast.WildcardPattern:
		// No slot needed
	}
}

// extractPatternValues extracts values from sourceSlot into pre-allocated pattern slots
// This is called on EACH iteration to update the pattern variable values
func (c *Compiler) extractPatternValues(pattern ast.Pattern, sourceSlot int, slots map[string]int, line int) error {
	switch p := pattern.(type) {
	case *ast.TuplePattern:
		for i, elem := range p.Elements {
			// Get source tuple
			c.emit(OP_GET_LOCAL, line)
			c.currentChunk().Write(byte(sourceSlot), line)
			c.slotCount++

			// Get element i from tuple
			c.emitConstant(&evaluator.Integer{Value: int64(i)}, line)
			c.slotCount++
			c.emit(OP_GET_TUPLE_ELEM, line)
			c.slotCount-- // index consumed, element on stack

			// Handle the element
			if ident, ok := elem.(*ast.IdentifierPattern); ok {
				if ident.Value != "_" {
					// Store in pre-allocated slot
					slot := slots[ident.Value]
					c.emit(OP_SET_LOCAL, line)
					c.currentChunk().Write(byte(slot), line)
				}
				c.emit(OP_POP, line)
				c.slotCount--
			} else {
				// Nested pattern - create temp slot and recurse
				tempSlot := c.slotCount - 1
				if err := c.extractPatternValues(elem, tempSlot, slots, line); err != nil {
					return err
				}
				c.emit(OP_POP, line)
				c.slotCount--
			}
		}
		return nil

	case *ast.RecordPattern:
		for fieldName, fieldPattern := range p.Fields {
			// Get source record
			c.emit(OP_GET_LOCAL, line)
			c.currentChunk().Write(byte(sourceSlot), line)
			c.slotCount++

			// Get field from record
			nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: fieldName})
			c.emit(OP_GET_FIELD, line)
			c.currentChunk().Write(byte(nameIdx>>8), line)
			c.currentChunk().Write(byte(nameIdx), line)

			if ident, ok := fieldPattern.(*ast.IdentifierPattern); ok {
				if ident.Value != "_" {
					slot := slots[ident.Value]
					c.emit(OP_SET_LOCAL, line)
					c.currentChunk().Write(byte(slot), line)
				}
				c.emit(OP_POP, line)
				c.slotCount--
			} else {
				tempSlot := c.slotCount - 1
				if err := c.extractPatternValues(fieldPattern, tempSlot, slots, line); err != nil {
					return err
				}
				c.emit(OP_POP, line)
				c.slotCount--
			}
		}
		return nil

	case *ast.ListPattern:
		for i, elem := range p.Elements {
			// Get source list
			c.emit(OP_GET_LOCAL, line)
			c.currentChunk().Write(byte(sourceSlot), line)
			c.slotCount++

			// Get element i from list
			idxConst := c.currentChunk().AddConstant(&evaluator.Integer{Value: int64(i)})
			c.emit(OP_CONST, line)
			c.currentChunk().Write(byte(idxConst>>8), line)
			c.currentChunk().Write(byte(idxConst), line)
			c.slotCount++
			c.emit(OP_GET_INDEX, line)
			c.slotCount-- // index consumed

			if ident, ok := elem.(*ast.IdentifierPattern); ok {
				if ident.Value != "_" {
					slot := slots[ident.Value]
					c.emit(OP_SET_LOCAL, line)
					c.currentChunk().Write(byte(slot), line)
				}
				c.emit(OP_POP, line)
				c.slotCount--
			} else {
				tempSlot := c.slotCount - 1
				if err := c.extractPatternValues(elem, tempSlot, slots, line); err != nil {
					return err
				}
				c.emit(OP_POP, line)
				c.slotCount--
			}
		}
		return nil

	case *ast.IdentifierPattern:
		if p.Value != "_" {
			// Get value from source and store in slot
			c.emit(OP_GET_LOCAL, line)
			c.currentChunk().Write(byte(sourceSlot), line)
			c.slotCount++
			slot := slots[p.Value]
			c.emit(OP_SET_LOCAL, line)
			c.currentChunk().Write(byte(slot), line)
			c.emit(OP_POP, line)
			c.slotCount--
		}
		return nil

	case *ast.WildcardPattern:
		return nil

	default:
		return fmt.Errorf("unsupported pattern type in extraction: %T", pattern)
	}
}

// compileCompFilter compiles a filter clause (boolean expression)
func (c *Compiler) compileCompFilter(filter *ast.CompFilter, clauses []ast.CompClause, idx int, output ast.Expression, resultSlot int, line int) error {
	// Compile condition
	if err := c.compileExpression(filter.Condition); err != nil {
		return err
	}

	// Skip if false
	skipJump := c.emitJump(OP_JUMP_IF_FALSE, line)
	c.emit(OP_POP, line) // Pop condition (true)
	c.slotCount--

	// Compile remaining clauses
	if err := c.compileCompClauses(clauses, idx+1, output, resultSlot, line); err != nil {
		return err
	}

	// Skip label
	skipEnd := c.emitJump(OP_JUMP, line)
	c.patchJump(skipJump)
	c.emit(OP_POP, line) // Pop condition (false)
	c.slotCount--
	c.patchJump(skipEnd)

	return nil
}

package vm

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/evaluator"
	"sort"
)

// compilePatternCheck compiles pattern matching checks
// Stack: [matched_value] -> [matched_value] (value stays on stack)
// Returns failJump offset if pattern can fail, -1 otherwise
func (c *Compiler) compilePatternCheck(pattern ast.Pattern, line int) (int, error) {
	switch p := pattern.(type) {
	case *ast.WildcardPattern:
		// Always matches, no code needed
		return -1, nil

	case *ast.IdentifierPattern:
		// Bind variable to matched value (top of stack)
		// The value stays on stack and becomes the binding
		// No DUP needed - the element IS the binding
		c.addLocal(p.Value, c.slotCount-1)
		// slotCount unchanged - element stays on stack as binding
		return -1, nil

	case *ast.LiteralPattern:
		return c.compileLiteralPattern(p, line)

	case *ast.ConstructorPattern:
		return c.compileConstructorPattern(p, line)

	case *ast.StringPattern:
		return c.compileStringPattern(p, line)

	case *ast.TuplePattern:
		return c.compileTuplePattern(p, line)

	case *ast.RecordPattern:
		return c.compileRecordPattern(p, line)

	case *ast.TypePattern:
		return c.compileTypePattern(p, line)

	case *ast.SpreadPattern:
		return c.compileSpreadPattern(p, line)

	case *ast.ListPattern:
		return c.compileListPattern(p, line)

	case *ast.PinPattern:
		return c.compilePinPattern(p, line)

	default:
		return -1, fmt.Errorf("unsupported pattern type: %T", pattern)
	}
}

func (c *Compiler) compileLiteralPattern(p *ast.LiteralPattern, line int) (int, error) {
	// DUP matched value, push literal, compare
	c.emit(OP_DUP, line)
	c.slotCount++
	litObj := c.literalToObject(p.Value)
	c.emitConstant(litObj, line)
	c.slotCount++
	c.emit(OP_EQ, line)
	c.slotCount-- // EQ pops dup and literal, pushes bool

	checkJump := c.emitJump(OP_JUMP_IF_FALSE, line)
	c.emit(OP_POP, line) // pop comparison result (true)
	c.slotCount--
	successJump := c.emitJump(OP_JUMP, line)

	// Failure path
	c.patchJump(checkJump)
	c.emit(OP_POP, line) // pop comparison result (false)
	failJump := c.emitJump(OP_JUMP, line)

	c.patchJump(successJump)
	return failJump, nil
}

func (c *Compiler) compileConstructorPattern(p *ast.ConstructorPattern, line int) (int, error) {
	// Stack at entry: [..., value_to_match]
	// slotCount includes value_to_match
	slotAtEntry := c.slotCount - 1 // slot of value_to_match

	c.emit(OP_DUP, line)
	c.slotCount++
	// Stack: [..., value, value_copy]

	tagIdx := c.currentChunk().AddConstant(&stringConstant{Value: p.Name.Value})
	c.emit(OP_CHECK_TAG, line)
	c.currentChunk().Write(byte(tagIdx>>8), line)
	c.currentChunk().Write(byte(tagIdx), line)
	c.slotCount++ // CHECK_TAG pushes bool
	// Stack: [..., value, value_copy, bool]

	tagJump := c.emitJump(OP_JUMP_IF_FALSE, line)
	c.emit(OP_POP, line) // pop check result (true)
	c.slotCount--
	// Stack: [..., value, value_copy]

	dataSlot := slotAtEntry + 1 // value_copy is at slotAtEntry + 1

	// For no elements (like Zero), just succeed after tag check
	if len(p.Elements) == 0 {
		successJump := c.emitJump(OP_JUMP, line)

		// Tag failure path - stack has: [..., value, value_copy, false]
		c.patchJump(tagJump)
		c.emit(OP_POP, line) // pop false
		c.emit(OP_POP, line) // pop value_copy
		// Stack: [..., value] - back to entry state
		failJump := c.emitJump(OP_JUMP, line)

		c.patchJump(successJump)
		// slotCount stays at entry + 2 (value + copy on stack)
		return failJump, nil
	}

	var allFailJumps []int

	// Extract and match each field
	for i, elem := range p.Elements {
		// Stack before field: [..., value, value_copy, ...bindings_so_far]
		slotsBeforeExtract := c.slotCount

		c.emit(OP_GET_LOCAL, line)
		c.currentChunk().Write(byte(dataSlot), line)
		c.slotCount++
		c.emit(OP_GET_DATA_FIELD, line)
		c.currentChunk().Write(byte(i), line)
		// Stack: [..., value, value_copy, ...bindings, field_i]
		// slotCount now includes field_i

		slotWithField := c.slotCount

		fieldJump, err := c.compilePatternCheck(elem, line)
		if err != nil {
			return -1, err
		}
		// After nested pattern:
		// - Success: stack may have additional bindings, slotCount updated
		// - Failure (if fieldJump >= 0): stack returned to state BEFORE compilePatternCheck
		//   which means field_i is still on stack

		slotsAfterSuccess := c.slotCount

		if fieldJump >= 0 {
			// Nested pattern can fail
			successCont := c.emitJump(OP_JUMP, line)

			c.patchJump(fieldJump)
			// At failure: stack is [..., value, value_copy, ...bindings_before_this_field, field_i]
			// Need to pop: field_i + bindings_before_this_field + value_copy
			// bindings_before_this_field = slotsBeforeExtract - (slotAtEntry + 2)
			bindingsSoFar := slotsBeforeExtract - (slotAtEntry + 2)
			// Pop field_i
			c.emit(OP_POP, line)
			// Pop bindings
			for j := 0; j < bindingsSoFar; j++ {
				c.emit(OP_POP, line)
			}
			// Pop value_copy
			c.emit(OP_POP, line)
			// Stack: [..., value] - back to entry state
			allFailJumps = append(allFailJumps, c.emitJump(OP_JUMP, line))

			c.patchJump(successCont)
			c.slotCount = slotsAfterSuccess
		}

		// If not identifier pattern, field stays on stack but we don't need it as binding
		if _, isIdent := elem.(*ast.IdentifierPattern); !isIdent {
			// Field was used for pattern check but not bound
			if slotsAfterSuccess == slotWithField {
				// No bindings added, just pop the field
				c.emit(OP_POP, line)
				c.slotCount--
			} else {
				// Bindings were added, so field is buried under them.
				// We need to remove the field (at slotWithField-1) but keep bindings.
				keepCount := slotsAfterSuccess - slotWithField
				c.emit(OP_POP_BELOW, line)
				c.currentChunk().Write(byte(keepCount), line)
				c.removeSlotFromStack(slotWithField - 1)
			}
		}
	}

	slotsAtSuccess := c.slotCount

	// Cleanup value_copy before success jump
	// Stack: [..., value, value_copy, ...bindings]
	// We want to pop value_copy (at slotAtEntry + 1)
	// Bindings are above it.
	bindingsCount := slotsAtSuccess - (slotAtEntry + 2)
	if bindingsCount > 0 {
		c.emit(OP_POP_BELOW, line)
		c.currentChunk().Write(byte(bindingsCount), line)
		c.removeSlotFromStack(slotAtEntry + 1)
	} else {
		// No bindings, just pop value_copy
		c.emit(OP_POP, line)
		c.slotCount--
	}
	slotsAtSuccess = c.slotCount // Update slotsAtSuccess after cleanup

	successJump := c.emitJump(OP_JUMP, line)

	// Tag failure path - stack has: [..., value, value_copy, false]
	c.patchJump(tagJump)
	c.emit(OP_POP, line) // pop false
	c.emit(OP_POP, line) // pop value_copy
	// Stack: [..., value] - back to entry state
	allFailJumps = append(allFailJumps, c.emitJump(OP_JUMP, line))

	// Common failure point - all paths have stack at entry state
	for _, fj := range allFailJumps {
		c.patchJump(fj)
	}
	failJump := c.emitJump(OP_JUMP, line)

	c.patchJump(successJump)
	c.slotCount = slotsAtSuccess

	return failJump, nil
}

func (c *Compiler) compileStringPattern(p *ast.StringPattern, line int) (int, error) {
	c.emit(OP_DUP, line)
	c.slotCount++

	partsIdx := c.currentChunk().AddConstant(&StringPatternParts{Parts: p.Parts})

	// Count captures
	captureCount := 0
	var captureNames []string
	for _, part := range p.Parts {
		if part.IsCapture {
			captureCount++
			captureNames = append(captureNames, part.Value)
		}
	}

	c.emit(OP_MATCH_STRING_EXTRACT, line)
	c.currentChunk().Write(byte(partsIdx>>8), line)
	c.currentChunk().Write(byte(partsIdx), line)
	c.currentChunk().Write(byte(captureCount), line)

	// OP_MATCH_STRING_EXTRACT pops string (1), pushes captures (N) + bool (1)
	// Net stack change: -1 + N + 1 = +N
	c.slotCount += captureCount

	checkJump := c.emitJump(OP_JUMP_IF_FALSE, line)
	c.emit(OP_POP, line) // pop true
	c.slotCount--

	// Bind captures (they are on stack below true, now exposed)
	for i, name := range captureNames {
		// capture 0 is deepest. capture N-1 is top.
		// Stack: [mv, cap0, cap1...capN-1]
		// We want to bind them.
		// capN-1 is at slotCount-1.
		slot := c.slotCount - captureCount + i
		c.addLocal(name, slot)
	}

	successJump := c.emitJump(OP_JUMP, line)

	// Failure path
	c.patchJump(checkJump)
	c.emit(OP_POP, line) // pop false
	// Pop dummy captures
	for i := 0; i < captureCount; i++ {
		c.emit(OP_POP, line)
	}

	failJump := c.emitJump(OP_JUMP, line)

	c.patchJump(successJump)
	return failJump, nil
}

func (c *Compiler) compileTuplePattern(p *ast.TuplePattern, line int) (int, error) {
	tupleSlot := c.slotCount - 1

	hasSpread := false
	var spreadIdx int
	if len(p.Elements) > 0 {
		if _, ok := p.Elements[len(p.Elements)-1].(*ast.SpreadPattern); ok {
			hasSpread = true
			spreadIdx = len(p.Elements) - 1
		}
	}

	c.emit(OP_DUP, line)
	c.slotCount++
	if hasSpread {
		c.emit(OP_CHECK_TUPLE_LEN_GE, line)
		c.currentChunk().Write(byte(spreadIdx), line)
	} else {
		c.emit(OP_CHECK_TUPLE_LEN, line)
		c.currentChunk().Write(byte(len(p.Elements)), line)
	}
	c.slotCount++

	lengthJump := c.emitJump(OP_JUMP_IF_FALSE, line)
	c.emit(OP_POP, line)
	c.slotCount--
	c.emit(OP_POP, line)
	c.slotCount--

	bindingsStart := c.slotCount

	// Track slots after each element for proper cleanup
	type elemInfo struct {
		failJump     int
		slotsAtStart int // slots before getting this element
		slotsAfterOk int // slots after successful match of this element
	}
	var elemInfos []elemInfo

	fixedCount := len(p.Elements)
	if hasSpread {
		fixedCount = spreadIdx
	}

	for i := 0; i < fixedCount; i++ {
		elemPat := p.Elements[i]
		slotsAtStart := c.slotCount

		c.emit(OP_GET_LOCAL, line)
		c.currentChunk().Write(byte(tupleSlot), line)
		c.slotCount++
		c.emitConstant(&evaluator.Integer{Value: int64(i)}, line)
		c.slotCount++
		c.emit(OP_GET_TUPLE_ELEM, line)
		c.slotCount -= 2
		c.slotCount++

		slotsBeforeCheck := c.slotCount
		elemJump, err := c.compilePatternCheck(elemPat, line)
		if err != nil {
			return -1, err
		}
		slotsAfterCheck := c.slotCount

		if _, isIdent := elemPat.(*ast.IdentifierPattern); !isIdent {
			if _, isSpread := elemPat.(*ast.SpreadPattern); !isSpread {
				if slotsAfterCheck == slotsBeforeCheck {
					c.emit(OP_POP, line)
					c.slotCount--
				} else {
					// Bindings added. Pop element below bindings.
					keepCount := slotsAfterCheck - slotsBeforeCheck
					c.emit(OP_POP_BELOW, line)
					c.currentChunk().Write(byte(keepCount), line)
					c.removeSlotFromStack(slotsBeforeCheck - 1)
				}
			}
		}

		elemInfos = append(elemInfos, elemInfo{
			failJump:     elemJump,
			slotsAtStart: slotsAtStart,
			slotsAfterOk: c.slotCount,
		})
	}

	if hasSpread {
		spreadPat := p.Elements[spreadIdx].(*ast.SpreadPattern)
		slotsAtStart := c.slotCount

		c.emit(OP_GET_LOCAL, line)
		c.currentChunk().Write(byte(tupleSlot), line)
		c.slotCount++
		c.emitConstant(&evaluator.Integer{Value: int64(spreadIdx)}, line)
		c.slotCount++
		c.emit(OP_TUPLE_SLICE, line)
		c.slotCount -= 2
		c.slotCount++

		elemJump, err := c.compilePatternCheck(spreadPat, line)
		if err != nil {
			return -1, err
		}

		elemInfos = append(elemInfos, elemInfo{
			failJump:     elemJump,
			slotsAtStart: slotsAtStart,
			slotsAfterOk: c.slotCount,
		})
	}

	slotsAtSuccess := c.slotCount
	successJump := c.emitJump(OP_JUMP, line)

	// Length failure path: stack has [tuple, dup_tuple, bool]
	c.patchJump(lengthJump)
	c.emit(OP_POP, line) // pop bool
	c.emit(OP_POP, line) // pop dup tuple
	failFromLength := c.emitJump(OP_JUMP, line)

	// Element failure paths: each element failure needs cleanup of previous bindings + current element
	var elemFailJumps []int
	for i, info := range elemInfos {
		if info.failJump < 0 {
			continue
		}
		c.patchJump(info.failJump)
		// At failure, we have: [tuple, ...bindings_0_to_i-1, elem_i_value]
		// Previous elements' bindings are at slots bindingsStart to info.slotsAtStart
		// Current element value is at top of stack
		// Need to pop from current stack (which has elem_i) down to bindingsStart

		// Stack at failure from element i: bindingsStart + bindings_from_0_to_i-1 + elem_i
		// We need to pop: elem_i + all previous bindings
		var prevBindings int
		if i > 0 {
			prevBindings = elemInfos[i-1].slotsAfterOk - bindingsStart
		}
		// Pop elem_i (1 slot) + previous bindings
		for j := 0; j < prevBindings+1; j++ {
			c.emit(OP_POP, line)
		}
		elemFailJumps = append(elemFailJumps, c.emitJump(OP_JUMP, line))
	}

	// All failure paths converge here with just [tuple] on stack
	c.patchJump(failFromLength)
	for _, fj := range elemFailJumps {
		c.patchJump(fj)
	}
	c.slotCount = bindingsStart // reset to state with just tuple
	failJump := c.emitJump(OP_JUMP, line)

	c.patchJump(successJump)
	c.slotCount = slotsAtSuccess

	return failJump, nil
}

func (c *Compiler) compileRecordPattern(p *ast.RecordPattern, line int) (int, error) {
	// Record is at slotCount-1, we reference it directly via GET_LOCAL
	recordSlot := c.slotCount - 1

	bindingsStart := c.slotCount

	// Sort keys for deterministic compilation
	keys := make([]string, 0, len(p.Fields))
	for k := range p.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	type fieldInfo struct {
		failJump     int
		slotsAfterOk int
	}
	var fieldInfos []fieldInfo

	for _, fieldName := range keys {
		fieldPattern := p.Fields[fieldName]
		// Get record from its slot (not DUP from stack top which may be wrong)
		c.emit(OP_GET_LOCAL, line)
		c.currentChunk().Write(byte(recordSlot), line)
		c.slotCount++

		// Get field from record
		nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: fieldName})
		c.emit(OP_GET_FIELD, line)
		c.currentChunk().Write(byte(nameIdx>>8), line)
		c.currentChunk().Write(byte(nameIdx), line)
		// GET_FIELD pops record, pushes field value, net 0

		slotsBeforeCheck := c.slotCount
		fieldJump, err := c.compilePatternCheck(fieldPattern, line)
		if err != nil {
			return -1, err
		}
		slotsAfterCheck := c.slotCount

		// For non-binding patterns, pop the field
		if _, isIdent := fieldPattern.(*ast.IdentifierPattern); !isIdent {
			if slotsAfterCheck == slotsBeforeCheck {
			c.emit(OP_POP, line)
			c.slotCount--
			} else {
				// Bindings added. Pop field below bindings.
				keepCount := slotsAfterCheck - slotsBeforeCheck
				c.emit(OP_POP_BELOW, line)
				c.currentChunk().Write(byte(keepCount), line)
				c.removeSlotFromStack(slotsBeforeCheck - 1)
			}
		}

		fieldInfos = append(fieldInfos, fieldInfo{
			failJump:     fieldJump,
			slotsAfterOk: c.slotCount,
		})
	}

	slotsAtSuccess := c.slotCount
	successJump := c.emitJump(OP_JUMP, line)

	// Failure paths
	var allFailJumps []int
	for i, info := range fieldInfos {
		if info.failJump < 0 {
			continue
		}
		c.patchJump(info.failJump)

		// At failure of field i: stack has [..., bindings_0_to_i-1, field_i_value]
		// We need to pop field_i_value AND all previous bindings

		// 1. Pop field_i_value (always present at failure point)
		c.emit(OP_POP, line)

		// 2. Pop previous bindings
		var bindingsSoFar int
		if i > 0 {
			bindingsSoFar = fieldInfos[i-1].slotsAfterOk - bindingsStart
		}
		for j := 0; j < bindingsSoFar; j++ {
			c.emit(OP_POP, line)
		}

		allFailJumps = append(allFailJumps, c.emitJump(OP_JUMP, line))
	}

	// Common failure point
	for _, fj := range allFailJumps {
		c.patchJump(fj)
	}
	// Reset slot count to state before bindings (but matched value is still on stack)
	c.slotCount = bindingsStart

	failJump := c.emitJump(OP_JUMP, line)

	c.patchJump(successJump)
	c.slotCount = slotsAtSuccess
	return failJump, nil
}

func (c *Compiler) compileTypePattern(p *ast.TypePattern, line int) (int, error) {
	typeName := ""
	if p.Type != nil {
		switch t := p.Type.(type) {
		case *ast.NamedType:
			typeName = t.Name.Value
		}
	}
	if typeName == "" {
		return -1, fmt.Errorf("unsupported type in TypePattern")
	}

	c.emit(OP_DUP, line)
	c.slotCount++
	typeIdx := c.currentChunk().AddConstant(&stringConstant{Value: typeName})
	c.emit(OP_CHECK_TYPE, line)
	c.currentChunk().Write(byte(typeIdx>>8), line)
	c.currentChunk().Write(byte(typeIdx), line)
	c.slotCount++

	typeJump := c.emitJump(OP_JUMP_IF_FALSE, line)
	c.emit(OP_POP, line)
	c.slotCount--
	c.emit(OP_POP, line)
	c.slotCount--

	if p.Name != "" && p.Name != "_" {
		slot := c.slotCount - 1
		c.addLocal(p.Name, slot)
	}

	successJump := c.emitJump(OP_JUMP, line)

	c.patchJump(typeJump)
	c.emit(OP_POP, line)
	c.emit(OP_POP, line)
	c.slotCount -= 2

	failJump := c.emitJump(OP_JUMP, line)
	c.patchJump(successJump)

	return failJump, nil
}

func (c *Compiler) compileSpreadPattern(p *ast.SpreadPattern, line int) (int, error) {
	if idPat, ok := p.Pattern.(*ast.IdentifierPattern); ok {
		// Simple identifier - bind spread value directly
		slot := c.slotCount - 1
		c.addLocal(idPat.Value, slot)
		return -1, nil
	}
	if tuplePat, ok := p.Pattern.(*ast.TuplePattern); ok {
		// Nested tuple pattern - match spread value against it
		// The spread value is already on stack, compile nested pattern
		return c.compileTuplePattern(tuplePat, line)
	}
	return -1, fmt.Errorf("unsupported nested pattern in spread: %T", p.Pattern)
}

func (c *Compiler) compileListPattern(p *ast.ListPattern, line int) (int, error) {
	listSlot := c.slotCount - 1

	hasSpread := false
	var spreadIdx int
	if len(p.Elements) > 0 {
		if _, ok := p.Elements[len(p.Elements)-1].(*ast.SpreadPattern); ok {
			hasSpread = true
			spreadIdx = len(p.Elements) - 1
		}
	}

	c.emit(OP_DUP, line)
	c.slotCount++
	c.emit(OP_CHECK_LIST_LEN, line)
	if hasSpread {
		c.currentChunk().Write(byte(1), line)
	} else {
		c.currentChunk().Write(byte(0), line)
	}
	if hasSpread {
		c.currentChunk().Write(byte(spreadIdx>>8), line)
		c.currentChunk().Write(byte(spreadIdx), line)
	} else {
		c.currentChunk().Write(byte(len(p.Elements)>>8), line)
		c.currentChunk().Write(byte(len(p.Elements)), line)
	}
	c.slotCount++

	lengthJump := c.emitJump(OP_JUMP_IF_FALSE, line)
	c.emit(OP_POP, line)
	c.slotCount--
	c.emit(OP_POP, line)
	c.slotCount--

	bindingsStart := c.slotCount

	// Track slots after each element for proper cleanup
	type elemInfo struct {
		failJump     int
		slotsAfterOk int // slots after successful match of this element
	}
	var elemInfos []elemInfo

	fixedCount := len(p.Elements)
	if hasSpread {
		fixedCount = spreadIdx
	}

	for i := 0; i < fixedCount; i++ {
		elemPat := p.Elements[i]
		c.emit(OP_GET_LOCAL, line)
		c.currentChunk().Write(byte(listSlot), line)
		c.slotCount++
		c.emitConstant(&evaluator.Integer{Value: int64(i)}, line)
		c.slotCount++
		c.emit(OP_GET_LIST_ELEM, line)
		c.slotCount--

		slotsBeforeCheck := c.slotCount
		fj, err := c.compilePatternCheck(elemPat, line)
		if err != nil {
			return -1, err
		}
		slotsAfterCheck := c.slotCount

		// Optimization: if pattern is not a binding (e.g. literal), pop the element
		// to keep stack size low and consistent with failure cleanup logic.
		if _, isIdent := elemPat.(*ast.IdentifierPattern); !isIdent {
			if _, isSpread := elemPat.(*ast.SpreadPattern); !isSpread {
				if slotsAfterCheck == slotsBeforeCheck {
			c.emit(OP_POP, line)
			c.slotCount--
				} else {
					// Bindings added. Pop element below bindings.
					keepCount := slotsAfterCheck - slotsBeforeCheck
					c.emit(OP_POP_BELOW, line)
					c.currentChunk().Write(byte(keepCount), line)
					c.removeSlotFromStack(slotsBeforeCheck - 1)
				}
			}
		}

		elemInfos = append(elemInfos, elemInfo{
			failJump:     fj,
			slotsAfterOk: c.slotCount,
		})
	}

	if hasSpread {
		spreadPat := p.Elements[spreadIdx].(*ast.SpreadPattern)
		c.emit(OP_GET_LOCAL, line)
		c.currentChunk().Write(byte(listSlot), line)
		c.slotCount++
		c.emit(OP_GET_LIST_REST, line)
		c.currentChunk().Write(byte(fixedCount), line)

		if idPat, ok := spreadPat.Pattern.(*ast.IdentifierPattern); ok {
			c.addLocal(idPat.Value, c.slotCount-1)
		}
	}

	slotsAtSuccess := c.slotCount
	successJump := c.emitJump(OP_JUMP, line)

	// Length failure path: stack has [list, dup_list, bool_result]
	c.patchJump(lengthJump)
	c.emit(OP_POP, line) // pop bool result
	c.emit(OP_POP, line) // pop dup list
	lengthFailJump := c.emitJump(OP_JUMP, line)

	// Element failure paths: each element failure needs cleanup of previous bindings + current element
	var elemFailJumps []int
	for i, info := range elemInfos {
		if info.failJump < 0 {
			continue
		}
		c.patchJump(info.failJump)
		// At failure, we have: [tuple, ...bindings_0_to_i-1, elem_i_value]
		// Previous elements' bindings are at slots bindingsStart to info.slotsAtStart
		// Current element value is at top of stack
		// Need to pop from current stack (which has elem_i) down to bindingsStart

		// Stack at failure from element i: bindingsStart + bindings_from_0_to_i-1 + elem_i
		// We need to pop: elem_i + all previous bindings
		var prevBindings int
		if i > 0 {
			prevBindings = elemInfos[i-1].slotsAfterOk - bindingsStart
		}
		// Pop elem_i (1 slot) + previous bindings
		for j := 0; j < prevBindings+1; j++ {
			c.emit(OP_POP, line)
		}
		elemFailJumps = append(elemFailJumps, c.emitJump(OP_JUMP, line))
	}

	// Both failure paths converge here with just [list] on stack
	c.patchJump(lengthFailJump)
	for _, fj := range elemFailJumps {
		c.patchJump(fj)
	}
	c.slotCount = bindingsStart

	failJump := c.emitJump(OP_JUMP, line)

	c.patchJump(successJump)
	c.slotCount = slotsAtSuccess

	return failJump, nil
}

func (c *Compiler) compilePinPattern(p *ast.PinPattern, line int) (int, error) {
	// Stack: [..., match_val]

	// 1. DUP match_val for comparison
	c.emit(OP_DUP, line)
	c.slotCount++

	// 2. Resolve variable and push its value to stack
	// Look for local variable
	if slot := c.resolveLocal(p.Name); slot != -1 {
		c.emit(OP_GET_LOCAL, line)
		c.currentChunk().Write(byte(slot), line)
		c.slotCount++
	} else if upvalue := c.resolveUpvalue(p.Name); upvalue != -1 {
		// Look for upvalue
		c.emit(OP_GET_UPVALUE, line)
		c.currentChunk().Write(byte(upvalue), line)
		c.slotCount++
	} else {
		// Global variable
		nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: p.Name})
		c.emit(OP_GET_GLOBAL, line)
		c.currentChunk().Write(byte(nameIdx>>8), line)
		c.currentChunk().Write(byte(nameIdx), line)
		c.slotCount++
	}

	// Stack: [..., match_val, match_val, pinned_val]

	// 3. Equality check
	c.emit(OP_EQ, line)
	c.slotCount-- // EQ pops 2 (match_val, pinned_val), pushes 1 (bool)
	// c.slotCount-- // Removed incorrect decrement. Stack is [..., match_val, bool] (N+1)

	// Stack: [..., match_val, bool]

	// 4. Conditional jump
	checkJump := c.emitJump(OP_JUMP_IF_FALSE, line)
	c.emit(OP_POP, line) // pop comparison result (true)
	c.slotCount--
	successJump := c.emitJump(OP_JUMP, line)

	// Failure path
	c.patchJump(checkJump)
	c.emit(OP_POP, line) // pop comparison result (false)
	failJump := c.emitJump(OP_JUMP, line)

	c.patchJump(successJump)
	return failJump, nil
}

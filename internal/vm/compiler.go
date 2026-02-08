package vm

import (
	"encoding/hex"
	"fmt"
	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
	"sort"
	"strconv"
	"strings"
)

// Local represents a local variable during compilation
type Local struct {
	Name       string
	Depth      int  // Scope depth where this local was declared
	Slot       int  // Stack slot relative to frame.base
	IsCaptured bool // True if captured by a nested function (needs to become upvalue)
}

// Upvalue represents a captured variable from an enclosing scope
type Upvalue struct {
	Index   uint8 // Index of the local/upvalue in enclosing scope
	IsLocal bool  // True if captures a local, false if captures another upvalue
}

// FunctionType distinguishes top-level code from functions
type FunctionType int

const (
	TYPE_SCRIPT FunctionType = iota
	TYPE_FUNCTION
)

// LoopContext tracks loop information for break/continue
type LoopContext struct {
	loopStart  int   // Offset of loop start (for continue)
	breakJumps []int // Offsets of break jumps to patch
	scopeDepth int   // Scope depth when loop started
	localCount int   // Local count when loop started
	slotCount  int   // Slot count when loop started (before loop vars)
}

// Compiler compiles AST to bytecode
type Compiler struct {
	// Current function being compiled
	function *CompiledFunction
	funcType FunctionType

	locals     []Local
	localCount int // Number of locals defined
	scopeDepth int // Current scope depth (0 = global for script, 1 = function body)
	slotCount  int // Current stack slot count (relative to frame.base)

	// Upvalues captured by this function
	upvalues     []Upvalue
	upvalueCount int

	// Enclosing compiler (for nested functions)
	enclosing *Compiler

	// Loop context stack for break/continue
	loopStack []LoopContext

	// Global variables defined in this script (for top-level code only)
	globals map[string]bool

	// Module loading support
	baseDir         string          // Base directory for resolving imports
	importedModules map[string]bool // Track already imported modules to avoid duplicates

	// Collected imports to be processed before VM runs
	pendingImports []PendingImport

	// Tail call optimization
	inTailPosition bool // true when compiling expression in tail position

	// Type aliases collected during compilation for default() support
	typeAliases map[string]typesystem.Type

	// Type map from static analyzer (optional) to support type-based dispatch
	typeMap map[ast.Node]typesystem.Type

	// Context for type expectations (propagated from assignments/annotations)
	typeContext string

	// Function Registry for monomorphization (maps function name to AST)
	functionRegistry map[string]*ast.FunctionStatement

	// Type substitution for monomorphized functions
	subst typesystem.Subst

	// Symbol Table for looking up trait methods and dispatch strategies
	symbolTable *symbols.SymbolTable

	// Resolution Map from Analyzer for looking up symbols by AST node
	resolutionMap map[ast.Node]symbols.Symbol

	// Specialization depth counter (tracked on root compiler to detect infinite
	// monomorphization recursion, e.g. f<T> calling f<Wrapper<T>>)
	specializeDepth int
}

// PendingImport represents an import that needs to be processed before VM runs
type PendingImport struct {
	Path           string   // Module path (e.g., "lib/test")
	ImportAll      bool     // import (*)
	Symbols        []string // Specific symbols to import
	ExcludeSymbols []string // Symbols to exclude when ImportAll is true
	Alias          string   // Module alias for "import as" syntax
}

// NewCompiler creates a new compiler for top-level code
func NewCompiler() *Compiler {
	c := &Compiler{
		function: &CompiledFunction{
			Chunk: NewChunk(),
			Name:  "<script>",
		},
		funcType:         TYPE_SCRIPT,
		locals:           make([]Local, 256),
		upvalues:         make([]Upvalue, 256),
		globals:          make(map[string]bool),
		importedModules:  make(map[string]bool),
		typeAliases:      make(map[string]typesystem.Type),
		functionRegistry: make(map[string]*ast.FunctionStatement),
		subst:            make(typesystem.Subst),
	}
	return c
}

// SetSymbolTable sets the symbol table
func (c *Compiler) SetSymbolTable(st *symbols.SymbolTable) {
	c.symbolTable = st
}

// SetResolutionMap sets the resolution map from analyzer
func (c *Compiler) SetResolutionMap(resMap map[ast.Node]symbols.Symbol) {
	c.resolutionMap = resMap
}

// SetTypeMap sets the type map from static analyzer
func (c *Compiler) SetTypeMap(typeMap map[ast.Node]typesystem.Type) {
	c.typeMap = typeMap
}

// GetTypeAliases returns collected type aliases for default() support
func (c *Compiler) GetTypeAliases() map[string]typesystem.Type {
	return c.typeAliases
}

// extractTypeConstructorName extracts the type constructor name from a type.
// e.g., Option<Int> → "Option", List<String> → "List", Result<String, Int> → "Result"
func extractTypeConstructorName(t typesystem.Type) string {
	switch typ := t.(type) {
	case typesystem.TCon:
		return typ.Name
	case typesystem.TApp:
		// For TApp, recursively get the constructor
		return extractTypeConstructorName(typ.Constructor)
	default:
		return ""
	}
}

// SetBaseDir sets the base directory for resolving imports
func (c *Compiler) SetBaseDir(dir string) {
	c.baseDir = dir
}

// astTypeToTypesystemType converts AST type to typesystem.Type for type aliases
func (c *Compiler) astTypeToTypesystemType(t ast.Type) typesystem.Type {
	if t == nil {
		return nil
	}
	switch node := t.(type) {
	case *ast.NamedType:
		if node.Name != nil {
			baseName := node.Name.Value
			// If has type arguments, create TApp
			if len(node.Args) > 0 {
				args := make([]typesystem.Type, len(node.Args))
				for i, arg := range node.Args {
					args[i] = c.astTypeToTypesystemType(arg)
				}
				return typesystem.TApp{
					Constructor: typesystem.TCon{Name: baseName},
					Args:        args,
				}
			}
			return typesystem.TCon{Name: baseName}
		}
	case *ast.RecordType:
		fields := make(map[string]typesystem.Type)
		for name, fieldType := range node.Fields {
			fields[name] = c.astTypeToTypesystemType(fieldType)
		}
		return typesystem.TRecord{Fields: fields}
	case *ast.FunctionType:
		var params []typesystem.Type
		for _, p := range node.Parameters {
			params = append(params, c.astTypeToTypesystemType(p))
		}
		ret := c.astTypeToTypesystemType(node.ReturnType)
		return typesystem.TFunc{
			Params:     params,
			ReturnType: ret,
		}
	case *ast.TupleType:
		var elems []typesystem.Type
		for _, e := range node.Types {
			elems = append(elems, c.astTypeToTypesystemType(e))
		}
		return typesystem.TTuple{Elements: elems}
	case *ast.UnionType:
		var types []typesystem.Type
		for _, t := range node.Types {
			types = append(types, c.astTypeToTypesystemType(t))
		}
		return typesystem.TUnion{Types: types}
	}
	return nil
}

// newFunctionCompiler creates a compiler for a function
func newFunctionCompiler(enclosing *Compiler, name string, arity int) *Compiler {
	c := &Compiler{
		function: &CompiledFunction{
			Chunk: NewChunk(),
			Name:  name,
			Arity: arity,
		},
		funcType:         TYPE_FUNCTION,
		locals:           make([]Local, 256),
		upvalues:         make([]Upvalue, 256),
		scopeDepth:       1, // Function body starts at depth 1
		enclosing:        enclosing,
		typeMap:          enclosing.typeMap,          // Inherit type map
		typeAliases:      enclosing.typeAliases,      // Inherit type aliases map reference
		functionRegistry: enclosing.functionRegistry, // Inherit registry
		subst:            enclosing.subst,            // Inherit substitution
		symbolTable:      enclosing.symbolTable,      // Inherit symbol table
		resolutionMap:    enclosing.resolutionMap,    // Inherit resolution map
	}
	return c
}

// currentChunk returns the chunk being compiled
func (c *Compiler) currentChunk() *Chunk {
	return c.function.Chunk
}

// withTypeContext sets the type context for the duration of the function
func (c *Compiler) withTypeContext(context string, fn func() error) error {
	oldContext := c.typeContext
	c.typeContext = context
	err := fn()
	c.typeContext = oldContext
	return err
}

// Compile compiles a program to bytecode
func (c *Compiler) Compile(program *ast.Program) (*Chunk, error) {
	if err := c.compileProgram(program); err != nil {
		return nil, err
	}

	c.emit(OP_HALT, 0)
	c.function.LocalCount = c.localCount

	// Save local variable names for debugging (top-level script)
	c.function.LocalNames = make([]string, c.localCount)
	for i := 0; i < c.localCount; i++ {
		c.function.LocalNames[i] = c.locals[i].Name
	}

	// Copy pending imports to the chunk for serialization
	chunk := c.currentChunk()
	chunk.PendingImports = c.pendingImports

	return chunk, nil
}

// compileProgram compiles a program's statements without emitting HALT
// Used for compiling module files that are then combined
func (c *Compiler) compileProgram(program *ast.Program) error {
	for i, stmt := range program.Statements {
		// Track slotCount before compiling statement
		slotsBefore := c.slotCount

		if err := c.compileStatement(stmt); err != nil {
			return err
		}

		// Pop intermediate values only if statement added something to stack
		// and it's not the last statement
		if i < len(program.Statements)-1 && c.slotCount > slotsBefore {
			c.emit(OP_POP, 0)
			c.slotCount--
		}
	}
	return nil
}

// Statement compilation
// Expression compilation - each expression pushes exactly ONE value onto the stack
func (c *Compiler) literalToObject(val interface{}) evaluator.Object {
	switch v := val.(type) {
	case int64:
		return &evaluator.Integer{Value: v}
	case int:
		return &evaluator.Integer{Value: int64(v)}
	case float64:
		return &evaluator.Float{Value: v}
	case bool:
		return &evaluator.Boolean{Value: v}
	case string:
		return evaluator.StringToList(v)
	case rune:
		return &evaluator.Char{Value: int64(v)}
	default:
		return &evaluator.Nil{}
	}
}

// countLocalsInScope returns number of locals in current scope
// compileListLiteral compiles a list literal [a, b, c]
func (c *Compiler) compileListLiteral(lit *ast.ListLiteral) error {
	line := lit.Token.Line

	// Elements of a list literal are never in tail position
	// (we need their results to build the list)
	wasTail := c.inTailPosition
	c.inTailPosition = false
	defer func() { c.inTailPosition = wasTail }()

	// Check if any element is a spread
	hasSpread := false
	for _, elem := range lit.Elements {
		if _, ok := elem.(*ast.SpreadExpression); ok {
			hasSpread = true
			break
		}
	}

	if !hasSpread {
		// Simple case: no spreads
		for _, elem := range lit.Elements {
			if err := c.withTypeContext("", func() error {
				return c.compileExpression(elem)
			}); err != nil {
				return err
			}
		}

		// Emit MAKE_LIST with count
		c.emit(OP_MAKE_LIST, line)
		count := len(lit.Elements)
		c.currentChunk().Write(byte(count>>8), line)
		c.currentChunk().Write(byte(count), line)

		c.slotCount -= len(lit.Elements)
		c.slotCount++
	} else {
		// Complex case: has spreads - build incrementally
		// Start with empty list
		c.emit(OP_MAKE_LIST, line)
		c.currentChunk().Write(byte(0), line)
		c.currentChunk().Write(byte(0), line)
		c.slotCount++

		for _, elem := range lit.Elements {
			if spread, ok := elem.(*ast.SpreadExpression); ok {
				// Compile the spread expression
				if err := c.withTypeContext("", func() error {
					return c.compileExpression(spread.Expression)
				}); err != nil {
					return err
				}
				// Emit CONCAT to merge with current list
				c.emit(OP_CONCAT, line)
				c.slotCount--
			} else {
				// Compile single element and cons it onto the front
				// We'll build list in reverse then... no, let's build properly
				// Actually: use CONS to prepend element to list
				// Wait, that builds in wrong order. Let's emit each element,
				// then concat one at a time
				if err := c.withTypeContext("", func() error {
					return c.compileExpression(elem)
				}); err != nil {
					return err
				}
				// compileExpression increased slotCount by 1 for the element
				// Create a 1-element list (consumes 1, pushes 1 - net 0 on stack)
				c.emit(OP_MAKE_LIST, line)
				c.currentChunk().Write(byte(1>>8), line) // high byte (0)
				c.currentChunk().Write(byte(1), line)    // low byte (1)
				// slotCount unchanged: element consumed, list pushed
				// Now concat with accumulator (consumes 2, pushes 1)
				c.emit(OP_CONCAT, line)
				c.slotCount-- // net effect of this iteration: element compiled (+1) -> CONCAT (-1) = 0 extra
			}
		}
	}

	return nil
}

// Compile tuple literal
func (c *Compiler) compileTupleLiteral(lit *ast.TupleLiteral) error {
	line := lit.Token.Line

	// Elements of a tuple literal are never in tail position
	wasTail := c.inTailPosition
	c.inTailPosition = false
	defer func() { c.inTailPosition = wasTail }()

	// Compile each element
	for _, elem := range lit.Elements {
		if err := c.withTypeContext("", func() error {
			return c.compileExpression(elem)
		}); err != nil {
			return err
		}
	}

	// Emit MAKE_TUPLE with count
	c.emit(OP_MAKE_TUPLE, line)
	c.currentChunk().Write(byte(len(lit.Elements)), line)

	c.slotCount -= len(lit.Elements)
	c.slotCount++

	return nil
}

// Compile record literal
func (c *Compiler) compileRecordLiteral(lit *ast.RecordLiteral) error {
	line := lit.Token.Line

	// Elements of a record literal are never in tail position
	wasTail := c.inTailPosition
	c.inTailPosition = false
	defer func() { c.inTailPosition = wasTail }()

	// If spread is present, compile it first
	hasSpread := lit.Spread != nil
	if hasSpread {
		if err := c.withTypeContext("", func() error {
			return c.compileExpression(lit.Spread)
		}); err != nil {
			return err
		}
	}

	fieldCount := 0
	// Compile each field value and emit field name
	for name, value := range lit.Fields {
		// Push field name as constant
		nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: name})
		c.emit(OP_CONST, line)
		c.currentChunk().Write(byte(nameIdx>>8), line)
		c.currentChunk().Write(byte(nameIdx), line)
		c.slotCount++

		// Push field value
		if err := c.withTypeContext("", func() error {
			return c.compileExpression(value)
		}); err != nil {
			return err
		}
		fieldCount++
	}

	if hasSpread {
		// Emit EXTEND_RECORD with field count
		c.emit(OP_EXTEND_RECORD, line)
		c.currentChunk().Write(byte(fieldCount), line)

		// Stack: [base, name1, val1, name2, val2...]
		// Base consumes 1, pairs consume 2*N
		// Result consumes -1 (base) - 2*N + 1 (result) = -2*N
		// So total change: -2*N
		c.slotCount -= fieldCount * 2
		// Base is replaced by result, so slotCount stays same for base
	} else {
		// Emit MAKE_RECORD with field count
		c.emit(OP_MAKE_RECORD, line)
		c.currentChunk().Write(byte(fieldCount), line)

		// Check if there's a nominal type name from TypeMap
		var typeNameIdx int = 0xFFFF // Use 0xFFFF to indicate "no type name"
		if c.typeMap != nil {
			if inferredType := c.typeMap[lit]; inferredType != nil {
				typeName := extractTypeConstructorName(inferredType)
				if typeName != "" {
					typeNameIdx = c.currentChunk().AddConstant(&stringConstant{Value: typeName})
				}
			}
		}

		// Emit type name index (2 bytes)
		c.currentChunk().Write(byte(typeNameIdx>>8), line)
		c.currentChunk().Write(byte(typeNameIdx), line)

		c.slotCount -= fieldCount * 2 // name+value pairs
		c.slotCount++
	}

	return nil
}

// Compile index expression: list[i], map[k]
func (c *Compiler) compileIndexExpression(expr *ast.IndexExpression) error {
	if err := c.withTypeContext("", func() error {
		return c.compileExpression(expr.Left)
	}); err != nil {
		return err
	}

	if err := c.withTypeContext("", func() error {
		return c.compileExpression(expr.Index)
	}); err != nil {
		return err
	}

	c.emit(OP_GET_INDEX, expr.Token.Line)
	c.slotCount-- // consumes 2, pushes 1

	return nil
}

// Compile postfix expression: x?
func (c *Compiler) compilePostfixExpression(expr *ast.PostfixExpression) error {
	line := expr.Token.Line

	// Compile the left operand
	if err := c.compileExpression(expr.Left); err != nil {
		return err
	}

	switch expr.Operator {
	case "?":
		// Unwrap Option/Result or early return
		c.emit(OP_UNWRAP_OR_RETURN, line)
	default:
		return fmt.Errorf("unknown postfix operator: %s", expr.Operator)
	}

	return nil
}

// Compile pattern assignment: (a, b) = tuple
func (c *Compiler) compilePatternAssignExpression(expr *ast.PatternAssignExpression) error {
	line := expr.Token.Line

	// Compile the value
	if err := c.compileExpression(expr.Value); err != nil {
		return err
	}

	// Bind pattern to value
	if err := c.bindPattern(expr.Pattern, line); err != nil {
		return err
	}

	// Push nil as result
	c.emitConstant(&evaluator.Nil{}, line)
	c.slotCount++
	return nil
}

// bindPattern extracts values from pattern and binds to variables
func (c *Compiler) bindPattern(pat ast.Pattern, line int) error {
	switch p := pat.(type) {
	case *ast.IdentifierPattern:
		// Set variable - value is on stack
		name := p.Value
		if slot := c.resolveLocal(name); slot != -1 {
			c.emit(OP_SET_LOCAL, line)
			c.currentChunk().Write(byte(slot), line)
		} else if upvalue := c.resolveUpvalue(name); upvalue != -1 {
			c.emit(OP_SET_UPVALUE, line)
			c.currentChunk().Write(byte(upvalue), line)
		} else {
			nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: name})
			c.emit(OP_SET_GLOBAL, line)
			c.currentChunk().Write(byte(nameIdx>>8), line)
			c.currentChunk().Write(byte(nameIdx), line)
		}
		c.emit(OP_POP, line)
		c.slotCount--
		return nil

	case *ast.TuplePattern:
		// Value is a tuple - extract each element
		// Save tuple slot since bindings may change stack layout
		tupleSlot := c.slotCount - 1
		slotsBeforeBindings := c.slotCount

		for i, elemPat := range p.Elements {
			// Get tuple from its slot (not DUP, since stack may have grown with bindings)
			c.emit(OP_GET_LOCAL, line)
			c.currentChunk().Write(byte(tupleSlot), line)
			c.slotCount++

			// Get element at index i
			c.emitConstant(&evaluator.Integer{Value: int64(i)}, line)
			c.slotCount++
			c.emit(OP_GET_TUPLE_ELEM, line)
			c.slotCount -= 2 // tuple and index consumed
			c.slotCount++    // element pushed

			// Now we have element on stack, bind it
			if err := c.bindPatternElement(elemPat, line); err != nil {
				return err
			}
		}

		// Pop original tuple (at tupleSlot)
		if c.slotCount > slotsBeforeBindings {
			// Bindings were added, tuple is buried.
			count := c.slotCount - slotsBeforeBindings
			c.emit(OP_POP_BELOW, line)
			c.currentChunk().Write(byte(count), line)
			c.removeSlotFromStack(tupleSlot)
		} else {
			// No bindings, just pop tuple
			c.emit(OP_POP, line)
			c.slotCount--
		}
		return nil

	case *ast.ListPattern:
		// Value is a list - extract each element
		listSlot := c.slotCount - 1
		slotsBeforeBindings := c.slotCount

		for i, elemPat := range p.Elements {
			// Get list from its slot
			c.emit(OP_GET_LOCAL, line)
			c.currentChunk().Write(byte(listSlot), line)
			c.slotCount++

			// Get element at index i
			c.emitConstant(&evaluator.Integer{Value: int64(i)}, line)
			c.slotCount++
			c.emit(OP_GET_LIST_ELEM, line)
			c.slotCount -= 2 // index consumed
			c.slotCount++    // element pushed

			// Now we have element on stack, bind it
			if err := c.bindPatternElement(elemPat, line); err != nil {
				return err
			}
		}

		if c.slotCount > slotsBeforeBindings {
			count := c.slotCount - slotsBeforeBindings
			c.emit(OP_POP_BELOW, line)
			c.currentChunk().Write(byte(count), line)
			c.removeSlotFromStack(listSlot)
		} else {
			c.emit(OP_POP, line)
			c.slotCount--
		}
		return nil

	case *ast.RecordPattern:
		// Value is a record - extract each field
		// Save record slot since bindings may change stack layout
		recordSlot := c.slotCount - 1
		slotsBeforeBindings := c.slotCount

		// Sort keys for deterministic compilation
		keys := make([]string, 0, len(p.Fields))
		for k := range p.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, fieldName := range keys {
			fieldPattern := p.Fields[fieldName]

			// Get record from its slot
			c.emit(OP_GET_LOCAL, line)
			c.currentChunk().Write(byte(recordSlot), line)
			c.slotCount++

			// Get field
			nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: fieldName})
			c.emit(OP_GET_FIELD, line)
			c.currentChunk().Write(byte(nameIdx>>8), line)
			c.currentChunk().Write(byte(nameIdx), line)

			// Now we have element on stack, bind it
			if err := c.bindPatternElement(fieldPattern, line); err != nil {
				return err
			}
		}

		if c.slotCount > slotsBeforeBindings {
			count := c.slotCount - slotsBeforeBindings
			c.emit(OP_POP_BELOW, line)
			c.currentChunk().Write(byte(count), line)
			c.removeSlotFromStack(recordSlot)
		} else {
			c.emit(OP_POP, line)
			c.slotCount--
		}
		return nil

	case *ast.WildcardPattern:
		// Pop the value - it's not bound
		c.emit(OP_POP, line)
		c.slotCount--
		return nil

	default:
		return fmt.Errorf("unsupported pattern type in assignment: %T", pat)
	}
}

// bindPatternElement binds a single extracted element to a pattern
// The element is on top of stack and will be consumed (popped)
func (c *Compiler) bindPatternElement(pat ast.Pattern, line int) error {
	switch p := pat.(type) {
	case *ast.IdentifierPattern:
		// Set variable - value is on stack
		name := p.Value
		if slot := c.resolveLocal(name); slot != -1 {
			c.emit(OP_SET_LOCAL, line)
			c.currentChunk().Write(byte(slot), line)
		} else if upvalue := c.resolveUpvalue(name); upvalue != -1 {
			c.emit(OP_SET_UPVALUE, line)
			c.currentChunk().Write(byte(upvalue), line)
		} else if c.scopeDepth > 0 {
			// Inside a function - create a new local variable
			// Reserve a slot by pushing nil, then set it with actual value
			slot := c.slotCount - 1 // element is at top
			c.addLocal(name, slot)
			// Value stays on stack as the local, don't pop
			return nil
		} else {
			nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: name})
			c.emit(OP_SET_GLOBAL, line)
			c.currentChunk().Write(byte(nameIdx>>8), line)
			c.currentChunk().Write(byte(nameIdx), line)
		}
		c.emit(OP_POP, line)
		c.slotCount--
		return nil

	case *ast.WildcardPattern:
		// Discard - just pop
		c.emit(OP_POP, line)
		c.slotCount--
		return nil

	case *ast.TuplePattern:
		// Recursively bind nested tuple
		tupleSlot := c.slotCount - 1
		bindingsStart := c.slotCount

		for i, elemPat := range p.Elements {
			c.emit(OP_GET_LOCAL, line)
			c.currentChunk().Write(byte(tupleSlot), line)
			c.slotCount++
			c.emitConstant(&evaluator.Integer{Value: int64(i)}, line)
			c.slotCount++
			c.emit(OP_GET_TUPLE_ELEM, line)
			c.slotCount -= 2
			c.slotCount++

			if err := c.bindPatternElement(elemPat, line); err != nil {
				return err
			}
		}

		if c.slotCount > bindingsStart {
			count := c.slotCount - bindingsStart
			c.emit(OP_POP_BELOW, line)
			c.currentChunk().Write(byte(count), line)
			c.removeSlotFromStack(tupleSlot)
		} else {
			c.emit(OP_POP, line)
			c.slotCount--
		}
		return nil

	case *ast.ListPattern:
		// Recursively bind nested list
		listSlot := c.slotCount - 1
		bindingsStart := c.slotCount

		for i, elemPat := range p.Elements {
			c.emit(OP_GET_LOCAL, line)
			c.currentChunk().Write(byte(listSlot), line)
			c.slotCount++
			c.emitConstant(&evaluator.Integer{Value: int64(i)}, line)
			c.slotCount++
			c.emit(OP_GET_LIST_ELEM, line)
			c.slotCount--

			if err := c.bindPatternElement(elemPat, line); err != nil {
				return err
			}
		}

		if c.slotCount > bindingsStart {
			count := c.slotCount - bindingsStart
			c.emit(OP_POP_BELOW, line)
			c.currentChunk().Write(byte(count), line)
			c.removeSlotFromStack(listSlot)
		} else {
			c.emit(OP_POP, line)
			c.slotCount--
		}
		return nil

	case *ast.RecordPattern:
		// Value is a record - extract each field
		// Save record slot since bindings may change stack layout
		recordSlot := c.slotCount - 1
		bindingsStart := c.slotCount

		keys := make([]string, 0, len(p.Fields))
		for k := range p.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, fieldName := range keys {
			fieldPattern := p.Fields[fieldName]

			c.emit(OP_GET_LOCAL, line)
			c.currentChunk().Write(byte(recordSlot), line)
			c.slotCount++

			nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: fieldName})
			c.emit(OP_GET_FIELD, line)
			c.currentChunk().Write(byte(nameIdx>>8), line)
			c.currentChunk().Write(byte(nameIdx), line)

			if err := c.bindPatternElement(fieldPattern, line); err != nil {
				return err
			}
		}

		if c.slotCount > bindingsStart {
			count := c.slotCount - bindingsStart
			c.emit(OP_POP_BELOW, line)
			c.currentChunk().Write(byte(count), line)
			c.removeSlotFromStack(recordSlot)
		} else {
			c.emit(OP_POP, line)
			c.slotCount--
		}
		return nil

	default:
		return fmt.Errorf("unsupported pattern element type: %T", pat)
	}
}

// Compile map literal: %{ key => value }
func (c *Compiler) compileMapLiteral(lit *ast.MapLiteral) error {
	line := lit.Token.Line

	// Elements of a map literal are never in tail position
	wasTail := c.inTailPosition
	c.inTailPosition = false
	defer func() { c.inTailPosition = wasTail }()

	// Compile each key-value pair
	for _, pair := range lit.Pairs {
		if err := c.withTypeContext("", func() error {
			return c.compileExpression(pair.Key)
		}); err != nil {
			return err
		}
		if err := c.withTypeContext("", func() error {
			return c.compileExpression(pair.Value)
		}); err != nil {
			return err
		}
	}

	// Emit MAKE_MAP with pair count
	c.emit(OP_MAKE_MAP, line)
	c.currentChunk().Write(byte(len(lit.Pairs)), line)

	c.slotCount -= len(lit.Pairs) * 2 // key+value pairs
	c.slotCount++

	return nil
}

// Compile anonymous function: fun(x) { x * 2 }
func (c *Compiler) compileFunctionLiteral(lit *ast.FunctionLiteral) error {
	arity := len(lit.Parameters)
	line := lit.Token.Line

	// Check for variadic parameter
	isVariadic := false
	if len(lit.Parameters) > 0 && lit.Parameters[len(lit.Parameters)-1].IsVariadic {
		isVariadic = true
		arity-- // Variadic param doesn't count toward fixed arity
	}

	// Count required params (those without defaults)
	requiredArity := 0
	for _, param := range lit.Parameters {
		if param.Default == nil && !param.IsVariadic {
			requiredArity++
		}
	}

	// Create a new compiler for this function
	funcCompiler := newFunctionCompiler(c, "<lambda>", arity)
	funcCompiler.function.RequiredArity = requiredArity
	funcCompiler.function.IsVariadic = isVariadic

	// Parameters become the first locals
	for i, param := range lit.Parameters {
		funcCompiler.addLocal(param.Name.Value, i)
	}
	funcCompiler.slotCount = len(lit.Parameters) // Include variadic param as local

	// Compile default values
	numDefaults := arity - requiredArity
	if numDefaults > 0 {
		funcCompiler.function.Defaults = make([]int, numDefaults)
		funcCompiler.function.DefaultChunks = make([]*Chunk, numDefaults)
		defaultIdx := 0
		for _, param := range lit.Parameters {
			if param.Default != nil {
				var constVal evaluator.Object
				switch d := param.Default.(type) {
				case *ast.IntegerLiteral:
					constVal = &evaluator.Integer{Value: d.Value}
				case *ast.FloatLiteral:
					constVal = &evaluator.Float{Value: d.Value}
				case *ast.StringLiteral:
					constVal = evaluator.StringToList(d.Value)
				case *ast.BooleanLiteral:
					constVal = &evaluator.Boolean{Value: d.Value}
				case *ast.ListLiteral:
					if len(d.Elements) == 0 {
						constVal = evaluator.NewList([]evaluator.Object{})
					}
				case *ast.NilLiteral:
					constVal = &evaluator.Nil{}
				}

				if constVal != nil {
					constIdx := funcCompiler.function.Chunk.AddConstant(constVal)
					funcCompiler.function.Defaults[defaultIdx] = constIdx
				} else {
					funcCompiler.function.Defaults[defaultIdx] = -1
					defaultCompiler := newFunctionCompiler(c, "<default>", 0)
					if err := defaultCompiler.compileExpression(param.Default); err != nil {
						return err
					}
					defaultCompiler.emit(OP_RETURN, line)
					funcCompiler.function.DefaultChunks[defaultIdx] = defaultCompiler.function.Chunk
				}
				defaultIdx++
			}
		}
	}

	// Compile the function body
	if err := funcCompiler.compileFunctionBody(lit.Body); err != nil {
		return err
	}

	// Get the compiled function
	fn := funcCompiler.function
	fn.LocalCount = funcCompiler.localCount
	fn.UpvalueCount = funcCompiler.upvalueCount

	// Build TypeInfo for getType()
	fn.TypeInfo = buildFunctionType(lit)

	// Add function as a constant and emit OP_CLOSURE
	fnIdx := c.currentChunk().AddConstant(fn)
	c.emit(OP_CLOSURE, line)
	c.currentChunk().Write(byte(fnIdx>>8), line)
	c.currentChunk().Write(byte(fnIdx), line)

	// Emit upvalue info
	for i := 0; i < funcCompiler.upvalueCount; i++ {
		if funcCompiler.upvalues[i].IsLocal {
			c.currentChunk().Write(1, line)
		} else {
			c.currentChunk().Write(0, line)
		}
		c.currentChunk().Write(funcCompiler.upvalues[i].Index, line)
	}
	c.slotCount++

	return nil
}

// Compile bytes literal: @"hello", @x"48656C6C6F", @b"01001000"
func (c *Compiler) compileBytesLiteral(lit *ast.BytesLiteral) error {
	line := lit.Token.Line
	var bytes *evaluator.Bytes

	switch lit.Kind {
	case "string":
		bytes = evaluator.BytesFromString(lit.Content)
	case "hex":
		data, err := hex.DecodeString(lit.Content)
		if err != nil {
			return fmt.Errorf("invalid hex string in bytes literal: %s", err.Error())
		}
		bytes = evaluator.BytesFromSlice(data)
	case "bin":
		if len(lit.Content)%8 != 0 {
			return fmt.Errorf("binary bytes literal must be a multiple of 8 bits, got %d bits", len(lit.Content))
		}
		data := make([]byte, len(lit.Content)/8)
		for i := 0; i < len(data); i++ {
			byteStr := lit.Content[i*8 : (i+1)*8]
			val, _ := strconv.ParseUint(byteStr, 2, 8)
			data[i] = byte(val)
		}
		bytes = evaluator.BytesFromSlice(data)
	}

	c.emitConstant(bytes, line)
	c.slotCount++
	return nil
}

// Compile bits literal: #b"101", #x"F"
func (c *Compiler) compileBitsLiteral(lit *ast.BitsLiteral) error {
	line := lit.Token.Line
	var bits *evaluator.Bits
	var err error

	switch lit.Kind {
	case "bin":
		bits, err = evaluator.BitsFromBinary(lit.Content)
	case "hex":
		bits, err = evaluator.BitsFromHex(lit.Content)
	case "oct":
		bits, err = evaluator.BitsFromOctal(lit.Content)
	}

	if err != nil {
		return fmt.Errorf("invalid bits literal at line %d: %s", line, err.Error())
	}

	c.emitConstant(bits, line)
	c.slotCount++
	return nil
}

// Compile interpolated string: "Hello {name}"
func (c *Compiler) compileInterpolatedString(expr *ast.InterpolatedString) error {
	line := expr.Token.Line

	if len(expr.Parts) == 0 {
		c.emitConstant(evaluator.StringToList(""), line)
		c.slotCount++
		return nil
	}

	// Compile first part
	if err := c.compileExpression(expr.Parts[0]); err != nil {
		return err
	}

	// If only one part, we must ensure it's a string
	if len(expr.Parts) == 1 {
		// Force conversion to string by concatenating with empty string
		c.emitConstant(evaluator.StringToList(""), line)
		c.slotCount++
		c.emit(OP_INTERP_CONCAT, line)
		c.slotCount--
		return nil
	}

	// Compile remaining parts and concatenate
	for i := 1; i < len(expr.Parts); i++ {
		if err := c.compileExpression(expr.Parts[i]); err != nil {
			return err
		}
		c.emit(OP_INTERP_CONCAT, line)
		c.slotCount--
	}

	return nil
}

// Compile format string literal
func (c *Compiler) compileFormatStringLiteral(lit *ast.FormatStringLiteral) error {
	line := lit.Token.Line

	// Store format string in constants
	constIdx := c.currentChunk().AddConstant(&stringConstant{Value: lit.Value})

	// Emit OP_FORMATTER
	c.emit(OP_FORMATTER, line)
	c.currentChunk().Write(byte(constIdx>>8), line)
	c.currentChunk().Write(byte(constIdx), line)

	c.slotCount++ // Pushes the formatter function
	return nil
}

// Compile member expression: record.field
func (c *Compiler) compileMemberExpression(expr *ast.MemberExpression) error {
	if err := c.withTypeContext("", func() error {
		return c.compileExpression(expr.Left)
	}); err != nil {
		return err
	}

	line := expr.Token.Line
	nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: expr.Member.Value})

	if expr.IsOptional {
		// Optional chaining: obj?.field
		// Check if obj is None/Fail, if so return it unchanged
		// Otherwise extract inner value, get field, wrap result
		c.emit(OP_OPTIONAL_CHAIN_FIELD, line)
		c.currentChunk().Write(byte(nameIdx>>8), line)
		c.currentChunk().Write(byte(nameIdx), line)
	} else {
		c.emit(OP_GET_FIELD, line)
		c.currentChunk().Write(byte(nameIdx>>8), line)
		c.currentChunk().Write(byte(nameIdx), line)
	}

	// Consumes 1, pushes 1 - net 0
	return nil
}

// buildFunctionType builds a typesystem.TFunc from a FunctionLiteral
func buildFunctionType(lit *ast.FunctionLiteral) typesystem.Type {
	var params []typesystem.Type
	for _, param := range lit.Parameters {
		if param.Type != nil {
			params = append(params, analyzer.BuildType(param.Type, nil, nil))
		} else {
			// Unknown parameter type
			params = append(params, typesystem.TVar{Name: "?"})
		}
	}

	var retType typesystem.Type
	if lit.ReturnType != nil {
		retType = analyzer.BuildType(lit.ReturnType, nil, nil)
	} else {
		retType = typesystem.TVar{Name: "?"}
	}

	return typesystem.TFunc{
		Params:     params,
		ReturnType: retType,
	}
}

// buildFunctionTypeFromStatement builds a typesystem.TFunc from a FunctionStatement
func buildFunctionTypeFromStatement(stmt *ast.FunctionStatement) typesystem.Type {
	var params []typesystem.Type
	for _, param := range stmt.Parameters {
		if param.Type != nil {
			params = append(params, analyzer.BuildType(param.Type, nil, nil))
		} else {
			// Unknown parameter type
			params = append(params, typesystem.TVar{Name: "?"})
		}
	}

	var retType typesystem.Type
	if stmt.ReturnType != nil {
		retType = analyzer.BuildType(stmt.ReturnType, nil, nil)
	} else {
		retType = typesystem.TVar{Name: "?"}
	}

	return typesystem.TFunc{
		Params:     params,
		ReturnType: retType,
	}
}

// maxSpecializeDepth limits monomorphization recursion to prevent infinite
// expansion (e.g. f<T> → f<Wrapper<T>> → f<Wrapper<Wrapper<T>>> → …).
// Kept low because types can grow exponentially (2^depth characters).
const maxSpecializeDepth = 8

// specialize generates a specialized version of a generic function
func (c *Compiler) specialize(name string, instantiation map[string]typesystem.Type) (string, error) {
	stmt, ok := c.functionRegistry[name]
	if !ok {
		return name, nil // Not found in registry, maybe imported or built-in
	}

	// Find root compiler early — we need it for the depth check BEFORE
	// generating specName (mangleTypeName calls t.String() which can itself
	// recurse exponentially on deeply-nested types).
	root := c
	for root.enclosing != nil {
		root = root.enclosing
	}

	// Guard against infinite monomorphization recursion.
	// Must check BEFORE generating specName, because mangleTypeName → t.String()
	// is itself O(2^depth) on exponentially-growing types.
	root.specializeDepth++
	defer func() { root.specializeDepth-- }()
	if root.specializeDepth > maxSpecializeDepth {
		return "", fmt.Errorf("specialization depth exceeded for %s (possible infinite monomorphization)", name)
	}

	// Generate specialized name
	var keys []string
	for k := range instantiation {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	specName := name
	for _, k := range keys {
		t := instantiation[k]
		specName += "$" + mangleTypeName(t)
	}

	// Check if already compiled
	if root.globals[specName] {
		return specName, nil
	}
	// Mark as compiled to break recursion
	root.registerGlobal(specName)

	// Create compiler for specialized function attached to ROOT
	arity := len(stmt.Parameters)
	specCompiler := newFunctionCompiler(root, specName, arity)

	// Setup substitution
	specCompiler.subst = make(typesystem.Subst)
	for k, v := range instantiation {
		specCompiler.subst[k] = v
	}

	// Setup locals
	for i, param := range stmt.Parameters {
		specCompiler.addLocal(param.Name.Value, i)
	}
	specCompiler.slotCount = arity

	// Compile Body
	if err := specCompiler.compileFunctionBody(stmt.Body); err != nil {
		return "", err
	}

	// Finalize function object
	fn := specCompiler.function
	fn.LocalCount = specCompiler.localCount
	fn.UpvalueCount = specCompiler.upvalueCount
	// fn.TypeInfo = ... (skip for now)

	// Emit OP_CLOSURE to ROOT
	fnIdx := root.currentChunk().AddConstant(fn)
	root.emit(OP_CLOSURE, stmt.Token.Line)
	root.currentChunk().Write(byte(fnIdx>>8), stmt.Token.Line)
	root.currentChunk().Write(byte(fnIdx), stmt.Token.Line)

	// Emit Upvalues
	for i := 0; i < specCompiler.upvalueCount; i++ {
		if specCompiler.upvalues[i].IsLocal {
			root.currentChunk().Write(1, stmt.Token.Line)
		} else {
			root.currentChunk().Write(0, stmt.Token.Line)
		}
		root.currentChunk().Write(specCompiler.upvalues[i].Index, stmt.Token.Line)
	}
	root.slotCount++

	// Emit OP_SET_GLOBAL to ROOT
	nameIdx := root.currentChunk().AddConstant(&stringConstant{Value: specName})
	root.emit(OP_SET_GLOBAL, stmt.Token.Line)
	root.currentChunk().Write(byte(nameIdx>>8), stmt.Token.Line)
	root.currentChunk().Write(byte(nameIdx), stmt.Token.Line)
	root.emit(OP_POP, stmt.Token.Line) // consume value
	root.slotCount--

	return specName, nil
}

func mangleTypeName(t typesystem.Type) string {
	s := t.String()
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "<", "$")
	s = strings.ReplaceAll(s, ">", "$")
	s = strings.ReplaceAll(s, ",", "_")
	s = strings.ReplaceAll(s, ".", "_")
	return s
}

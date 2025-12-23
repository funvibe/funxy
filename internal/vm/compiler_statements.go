package vm

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/typesystem"
)

// compileStatement compiles a statement
func (c *Compiler) compileStatement(stmt ast.Statement) error {
	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		return c.compileExpression(s.Expression)

	case *ast.ConstantDeclaration:
		return c.compileConstantDeclaration(s)

	case *ast.BlockStatement:
		return c.compileBlockStatement(s)

	case *ast.FunctionStatement:
		return c.compileFunctionStatement(s)

	case *ast.ImportStatement:
		return c.compileImportStatement(s)

	case *ast.TypeDeclarationStatement:
		return c.compileTypeDeclaration(s)

	case *ast.PackageDeclaration:
		return nil

	case *ast.BreakStatement:
		return c.compileBreakStatement(s)

	case *ast.ContinueStatement:
		return c.compileContinueStatement(s)

	case *ast.TraitDeclaration:
		return c.compileTraitDeclaration(s)

	case *ast.InstanceDeclaration:
		return c.compileInstanceDeclaration(s)

	default:
		return fmt.Errorf("unknown statement type: %T", stmt)
	}
}

// compileFunctionStatement compiles a function definition
func (c *Compiler) compileFunctionStatement(stmt *ast.FunctionStatement) error {
	name := stmt.Name.Value
	line := stmt.Token.Line

	hasReceiver := stmt.Receiver != nil

	allParams := stmt.Parameters
	if hasReceiver {
		allParams = append([]*ast.Parameter{stmt.Receiver}, stmt.Parameters...)
	}

	arity := len(allParams)

	requiredArity := 0
	for _, param := range allParams {
		if param.Default == nil {
			requiredArity++
		}
	}

	isVariadic := false
	if len(stmt.Parameters) > 0 && stmt.Parameters[len(stmt.Parameters)-1].IsVariadic {
		isVariadic = true
		arity--
	}

	funcCompiler := newFunctionCompiler(c, name, arity)
	funcCompiler.function.IsVariadic = isVariadic
	funcCompiler.function.RequiredArity = requiredArity

	for i, param := range allParams {
		funcCompiler.addLocal(param.Name.Value, i)
	}
	funcCompiler.slotCount = len(allParams)

	numDefaults := arity - requiredArity
	if numDefaults > 0 {
		funcCompiler.function.Defaults = make([]int, numDefaults)
		funcCompiler.function.DefaultChunks = make([]*Chunk, numDefaults)
		defaultIdx := 0
		for _, param := range allParams {
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

	if err := funcCompiler.compileFunctionBody(stmt.Body); err != nil {
		return err
	}

	fn := funcCompiler.function
	fn.LocalCount = funcCompiler.localCount
	fn.UpvalueCount = funcCompiler.upvalueCount
	fn.TypeInfo = buildFunctionTypeFromStatement(stmt)

	fnIdx := c.currentChunk().AddConstant(fn)
	c.emit(OP_CLOSURE, line)
	c.currentChunk().Write(byte(fnIdx>>8), line)
	c.currentChunk().Write(byte(fnIdx), line)

	for i := 0; i < funcCompiler.upvalueCount; i++ {
		if funcCompiler.upvalues[i].IsLocal {
			c.currentChunk().Write(1, line)
		} else {
			c.currentChunk().Write(0, line)
		}
		c.currentChunk().Write(funcCompiler.upvalues[i].Index, line)
	}
	c.slotCount++

	c.emit(OP_DUP, line)
	c.slotCount++
	nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: name})
	c.emit(OP_SET_GLOBAL, line)
	c.currentChunk().Write(byte(nameIdx>>8), line)
	c.currentChunk().Write(byte(nameIdx), line)

	if hasReceiver {
		// Register as extension method
		// Stack: ... [closure] ... (closure is still on stack because SET_GLOBAL peeked or we DUPed)
		// Wait, OP_SET_GLOBAL pops? Yes, it consumes value.
		// But we did OP_DUP before SET_GLOBAL.
		// So stack has: [..., closure]

		// Need to resolve receiver type name
		receiverType := stmt.Receiver.Type
		typeName := ""
		switch t := receiverType.(type) {
		case *ast.NamedType:
			typeName = t.Name.Value
		case *ast.FunctionType:
			typeName = "Function"
		case *ast.TupleType:
			typeName = "Tuple"
		case *ast.RecordType:
			typeName = "Record"
		default:
			// Fallback or error?
			// Just skip registration if type is complex/unknown
		}

		if typeName != "" {
			c.emit(OP_DUP, line) // Duplicate closure again for registration
			c.slotCount++

			typeNameIdx := c.currentChunk().AddConstant(&stringConstant{Value: typeName})
			methodNameIdx := c.currentChunk().AddConstant(&stringConstant{Value: name})

			c.emit(OP_REGISTER_EXTENSION, line)
			c.currentChunk().Write(byte(typeNameIdx>>8), line)
			c.currentChunk().Write(byte(typeNameIdx), line)
			c.currentChunk().Write(byte(methodNameIdx>>8), line)
			c.currentChunk().Write(byte(methodNameIdx), line)
			c.slotCount-- // Register consumes closure
		}
	}

	return nil
}

// compileTypeDeclaration compiles type declarations
func (c *Compiler) compileTypeDeclaration(stmt *ast.TypeDeclarationStatement) error {
	line := stmt.Token.Line

	if c.funcType == TYPE_FUNCTION {
		c.emit(OP_NIL, line)
		c.slotCount++
		return nil
	}

	typeName := stmt.Name.Value

	typeObj := &evaluator.TypeObject{TypeVal: typesystem.TCon{Name: typeName}}
	c.emitConstant(typeObj, line)
	c.slotCount++
	nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: typeName})
	c.emit(OP_SET_GLOBAL, line)
	c.currentChunk().Write(byte(nameIdx>>8), line)
	c.currentChunk().Write(byte(nameIdx), line)
	c.registerGlobal(typeName)
	c.emit(OP_POP, line)
	c.slotCount--

	if stmt.IsAlias {
		if stmt.TargetType != nil {
			underlyingType := c.astTypeToTypesystemType(stmt.TargetType)
			if underlyingType != nil {
				c.typeAliases[typeName] = underlyingType
			}
		}
		return nil
	}

	for _, ctor := range stmt.Constructors {
		ctorName := ctor.Name.Value
		if len(ctor.Parameters) == 0 {
			dataInst := &evaluator.DataInstance{
				Name:     ctorName,
				Fields:   []evaluator.Object{},
				TypeName: typeName,
			}
			c.emitConstant(dataInst, line)
		} else {
			ctorObj := &evaluator.Constructor{
				Name:     ctorName,
				TypeName: typeName,
				Arity:    len(ctor.Parameters),
			}
			c.emitConstant(ctorObj, line)
		}
		c.slotCount++
		ctorNameIdx := c.currentChunk().AddConstant(&stringConstant{Value: ctorName})
		c.emit(OP_SET_GLOBAL, line)
		c.currentChunk().Write(byte(ctorNameIdx>>8), line)
		c.currentChunk().Write(byte(ctorNameIdx), line)
		c.registerGlobal(ctorName)
		c.emit(OP_POP, line)
		c.slotCount--
	}

	return nil
}

// compileTraitDeclaration compiles trait declarations
func (c *Compiler) compileTraitDeclaration(stmt *ast.TraitDeclaration) error {
	line := stmt.Token.Line
	traitName := stmt.Name.Value

	for _, method := range stmt.Signatures {
		methodName := method.Name.Value
		arity := len(method.Parameters)

		cm := &evaluator.ClassMethod{
			Name:      methodName,
			ClassName: traitName,
			Arity:     arity,
		}
		c.emitConstant(cm, line)
		c.slotCount++

		nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: methodName})
		c.emit(OP_SET_GLOBAL, line)
		c.currentChunk().Write(byte(nameIdx>>8), line)
		c.currentChunk().Write(byte(nameIdx), line)
		c.registerGlobal(methodName)
		c.emit(OP_POP, line)
		c.slotCount--
	}

	c.emit(OP_NIL, line)
	c.slotCount++
	return nil
}

// compileInstanceDeclaration compiles instance declarations
func (c *Compiler) compileInstanceDeclaration(stmt *ast.InstanceDeclaration) error {
	line := stmt.Token.Line
	traitName := stmt.TraitName.Value

	typeName := ""
	switch t := stmt.Target.(type) {
	case *ast.NamedType:
		typeName = t.Name.Value
	case *ast.FunctionType:
		typeName = "Function"
	case *ast.TupleType:
		typeName = "Tuple"
	case *ast.RecordType:
		typeName = "Record"
	default:
		return fmt.Errorf("unsupported type in instance declaration: %T", stmt.Target)
	}

	for _, method := range stmt.Methods {
		methodName := method.Name.Value
		if method.Operator != "" {
			methodName = "(" + method.Operator + ")"
		}

		arity := len(method.Parameters)
		funcCompiler := newFunctionCompiler(c, methodName, arity)

		for i, param := range method.Parameters {
			funcCompiler.addLocal(param.Name.Value, i)
		}
		funcCompiler.slotCount = arity

		if err := funcCompiler.compileFunctionBody(method.Body); err != nil {
			return err
		}

		fn := funcCompiler.function
		fn.LocalCount = funcCompiler.localCount
		fn.UpvalueCount = funcCompiler.upvalueCount

		fnIdx := c.currentChunk().AddConstant(fn)
		c.emit(OP_CLOSURE, line)
		c.currentChunk().Write(byte(fnIdx>>8), line)
		c.currentChunk().Write(byte(fnIdx), line)

		for i := 0; i < funcCompiler.upvalueCount; i++ {
			if funcCompiler.upvalues[i].IsLocal {
				c.currentChunk().Write(1, line)
			} else {
				c.currentChunk().Write(0, line)
			}
			c.currentChunk().Write(byte(funcCompiler.upvalues[i].Index), line)
		}
		c.slotCount++

		traitIdx := c.currentChunk().AddConstant(&stringConstant{Value: traitName})
		typeIdx := c.currentChunk().AddConstant(&stringConstant{Value: typeName})
		methodIdx := c.currentChunk().AddConstant(&stringConstant{Value: methodName})

		c.emit(OP_REGISTER_TRAIT, line)
		c.currentChunk().Write(byte(traitIdx>>8), line)
		c.currentChunk().Write(byte(traitIdx), line)
		c.currentChunk().Write(byte(typeIdx>>8), line)
		c.currentChunk().Write(byte(typeIdx), line)
		c.currentChunk().Write(byte(methodIdx>>8), line)
		c.currentChunk().Write(byte(methodIdx), line)
		c.slotCount--
	}

	c.emit(OP_NIL, line)
	c.slotCount++
	return nil
}

// compileImportStatement compiles import statements
func (c *Compiler) compileImportStatement(stmt *ast.ImportStatement) error {
	path := stmt.Path.Value
	line := stmt.Token.Line

	// We allow multiple imports of the same path because they might have different aliases or symbol lists.
	// The VM is responsible for caching loaded modules to avoid re-execution.
	// c.importedModules[path] = true

	var symbols []string
	for _, sym := range stmt.Symbols {
		symbols = append(symbols, sym.Value)
	}

	var excludeSymbols []string
	for _, sym := range stmt.Exclude {
		excludeSymbols = append(excludeSymbols, sym.Value)
	}

	pending := PendingImport{
		Path:           path,
		ImportAll:      stmt.ImportAll,
		Symbols:        symbols,
		ExcludeSymbols: excludeSymbols,
	}
	if stmt.Alias != nil {
		pending.Alias = stmt.Alias.Value
	}

	root := c
	for root.enclosing != nil {
		root = root.enclosing
	}
	root.pendingImports = append(root.pendingImports, pending)

	c.emit(OP_NIL, line)
	c.slotCount++

	return nil
}

// GetPendingImports returns the list of imports that need to be processed
func (c *Compiler) GetPendingImports() []PendingImport {
	return c.pendingImports
}

// compileFunctionBody compiles the body of a function
func (c *Compiler) compileFunctionBody(body *ast.BlockStatement) error {
	for i, stmt := range body.Statements {
		isLast := i == len(body.Statements)-1
		if isLast {
			c.inTailPosition = true
		}

		if err := c.compileStatement(stmt); err != nil {
			return err
		}

		if isLast {
			c.inTailPosition = false
		}

		if !isLast {
			// Only pop if the statement produced a value that needs to be discarded.
			// Declarations (ConstantDeclaration, FunctionStatement) leave locals on the stack
			// that must NOT be popped.
			// ExpressionStatements produce a value that should be discarded if not last.
			shouldPop := false
			switch stmt.(type) {
			case *ast.ExpressionStatement:
				shouldPop = true
			case *ast.ImportStatement:
				shouldPop = true // pushes nil
			case *ast.TraitDeclaration:
				shouldPop = true // pushes nil
			}

			if shouldPop {
				c.emit(OP_POP, 0)
				c.slotCount--
			}
		}
	}

	if len(body.Statements) == 0 {
		c.emit(OP_NIL, body.Token.Line)
		c.slotCount++
	}

	c.emit(OP_RETURN, body.Token.Line)
	return nil
}

package funxy

import (
	"fmt"
	"io/ioutil"
	"os"
	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/token"
	"github.com/funvibe/funxy/internal/typesystem"
	"github.com/funvibe/funxy/internal/vm"
	"path/filepath"
	"reflect"
	"sync/atomic"
)

// VM wraps the underlying Funxy VM and provides a high-level embedding API.
type VM struct {
	machine      *vm.VM
	marshaller   *Marshaller
	bindings     map[string]Binding
	initialState []byte
}

// Binding represents a bound Go value or function.
type Binding struct {
	Value interface{}
	Type  typesystem.Type
}

// New creates a new Funxy VM instance.
func New() *VM {
	v := vm.New()

	// Create a loader in SandboxMode by default
	loader := modules.NewLoader()
	loader.SandboxMode = true
	v.SetLoader(loader)

	v.RegisterBuiltins()

	funxyVM := &VM{
		machine:  v,
		bindings: make(map[string]Binding),
	}
	funxyVM.marshaller = NewMarshaller(v)
	v.SetHostHandlers(funxyVM.hostCallHandler, funxyVM.hostToValueHandler)
	return funxyVM
}

// AllowModule explicitly allows loading a dirty virtual package
func (v *VM) AllowModule(path string) {
	if loader := v.machine.GetLoader(); loader != nil {
		loader.AllowedModules[path] = true
	}
}

// SetInitialState sets the initial state data for the VM.
// Must be called before LoadFile or Eval.
func (v *VM) SetInitialState(data []byte) {
	v.initialState = data
}

// GetMetrics returns VM monitoring statistics (CPU instructions, memory allocations).
// GetMetrics returns memory, instruction, and rate metrics for this VM instance
func (v *VM) GetMetrics() map[string]uint64 {
	evalMetrics := v.machine.GetEvaluatorMetrics()
	vmInstr := atomic.LoadUint64(&v.machine.InstructionCount)
	vmAlloc := atomic.LoadUint64(&v.machine.AllocatedBytes)

	metrics := map[string]uint64{
		"instructions": evalMetrics["instructions"] + vmInstr,
		"allocations":  evalMetrics["allocations"] + vmAlloc,
	}

	// Add rates
	metrics["current_instructions_per_sec"] = atomic.LoadUint64(&v.machine.CurrentInstructionsPerSec)
	metrics["current_allocations_bytes_per_sec"] = atomic.LoadUint64(&v.machine.CurrentAllocationsPerSec)

	// Add limits if they are set
	if v.machine.MaxInstructions > 0 {
		metrics["limit_instructions"] = v.machine.MaxInstructions
	}
	if v.machine.MaxInstructionsPerSec > 0 {
		metrics["limit_instructions_per_sec"] = v.machine.MaxInstructionsPerSec
	}
	if v.machine.MaxAllocBytes > 0 {
		metrics["limit_allocations_bytes"] = v.machine.MaxAllocBytes
	}
	if v.machine.MaxAllocBytesPerSecond > 0 {
		metrics["limit_allocations_bytes_per_sec"] = v.machine.MaxAllocBytesPerSecond
	}
	if v.machine.MaxStackDepth > 0 {
		metrics["limit_stack_depth"] = uint64(v.machine.MaxStackDepth)
	}

	return metrics
}

// RegisterSupervisor enables Supervisor API capabilities in this VM.
func (v *VM) RegisterSupervisor(h *Hypervisor) {
	// Let the evaluator know about the supervisor handler so builtins can use it
	eval := v.machine.GetEvaluator()
	if eval != nil {
		eval.SupervisorHandler = h.SupervisorHandler()
	}
}

// InjectStateHandler sets up state handoff mechanisms
func (v *VM) InjectStateHandler() {
	eval := v.machine.GetEvaluator()
	if eval != nil {
		var currentState evaluator.Object = &evaluator.Nil{}

		eval.StateHandler = &evaluator.StateHandler{
			GetState: func() evaluator.Object {
				return currentState
			},
			SetState: func(state evaluator.Object) {
				currentState = state
			},
		}

		// Pre-populate with initialState if it's set and valid
		if len(v.initialState) > 0 {
			if decoded, err := evaluator.DeserializeValue(v.initialState); err == nil {
				currentState = decoded
			}
		}
	}
}

func (v *VM) hostCallHandler(fn reflect.Value, args []evaluator.Object) (evaluator.Object, error) {
	// Convert args from Funxy to Go
	fnType := fn.Type()
	numIn := fnType.NumIn()
	isVariadic := fnType.IsVariadic()

	// Check arg count
	if isVariadic {
		if len(args) < numIn-1 {
			return nil, fmt.Errorf("expected at least %d arguments, got %d", numIn-1, len(args))
		}
	} else {
		if len(args) != numIn {
			return nil, fmt.Errorf("expected %d arguments, got %d", numIn, len(args))
		}
	}

	goArgs := make([]reflect.Value, len(args))
	for i, arg := range args {
		// Determine target type
		var targetType reflect.Type
		if isVariadic && i >= numIn-1 {
			targetType = fnType.In(numIn - 1).Elem()
		} else if i < numIn {
			targetType = fnType.In(i)
		}

		val, err := v.marshaller.FromValue(arg, targetType)
		if err != nil {
			return nil, fmt.Errorf("argument %d conversion failed: %w", i, err)
		}
		// Handle nil interface
		if val == nil {
			// reflect.ValueOf(nil) is invalid — must create a typed zero value.
			// Only nilable kinds (pointer, interface, map, slice, chan, func) accept nil;
			// for value types (int, bool, struct, etc.) passing nil is a type error.
			if targetType != nil {
				switch targetType.Kind() {
				case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
					goArgs[i] = reflect.Zero(targetType)
				default:
					return nil, fmt.Errorf("argument %d: cannot convert nil to non-nullable type %s", i, targetType)
				}
			} else {
				goArgs[i] = reflect.Zero(targetType)
			}
		} else {
			goArgs[i] = reflect.ValueOf(val)
		}
	}

	// Call
	results := fn.Call(goArgs)

	// Convert results back to Funxy
	if len(results) == 0 {
		return &evaluator.Nil{}, nil
	}
	if len(results) == 1 {
		return v.marshaller.ToValue(results[0].Interface())
	}
	// Multiple returns -> Tuple?
	// Funxy supports tuples.
	elements := make([]evaluator.Object, len(results))
	for i, res := range results {
		val, err := v.marshaller.ToValue(res.Interface())
		if err != nil {
			return nil, fmt.Errorf("return value [%d] conversion failed: %w", i, err)
		}
		elements[i] = val
	}
	return &evaluator.Tuple{Elements: elements}, nil
}

func (v *VM) hostToValueHandler(val interface{}) (evaluator.Object, error) {
	return v.marshaller.ToValue(val)
}

// Bind registers a Go function or value with the VM.
// It effectively makes it available in the global scope of scripts.
func (v *VM) Bind(name string, val interface{}) {
	// 1. Generate type signature for static analysis
	typ := inferType(val)
	v.bindings[name] = Binding{Value: val, Type: typ}

	// 2. Register in runtime globals
	// For functions, we might need to wrap them?
	// The Marshaller converts Go funcs to HostObject.
	// The runtime (evaluator/vm) will handle calling HostObjects.
	obj, _ := v.marshaller.ToValue(val)

	// Access the underlying PersistentMap of globals
	// We use Put to update the immutable map reference in ModuleScope
	currentGlobals := v.machine.GetGlobals()
	newGlobals := currentGlobals.Put(name, obj)
	v.machine.SetGlobals(newGlobals)
}

// Set sets a global variable in the VM.
// Use this for data objects. For functions, prefer Bind.
func (v *VM) Set(name string, val interface{}) {
	obj, _ := v.marshaller.ToValue(val)
	currentGlobals := v.machine.GetGlobals()
	newGlobals := currentGlobals.Put(name, obj)
	v.machine.SetGlobals(newGlobals)
}

// Get retrieves a global variable from the VM.
func (v *VM) Get(name string) (interface{}, error) {
	currentGlobals := v.machine.GetGlobals()
	obj := currentGlobals.Get(name)
	if obj == nil {
		return nil, fmt.Errorf("variable '%s' not found", name)
	}
	return v.marshaller.FromValue(obj, nil)
}

// Call calls a function defined in Funxy (or bound from Go) by name.
func (v *VM) Call(funcName string, args ...interface{}) (interface{}, error) {
	currentGlobals := v.machine.GetGlobals()
	fnObj := currentGlobals.Get(funcName)
	if fnObj == nil {
		return nil, fmt.Errorf("function '%s' not found", funcName)
	}

	funxyArgs := make([]evaluator.Object, len(args))
	for i, arg := range args {
		obj, err := v.marshaller.ToValue(arg)
		if err != nil {
			return nil, err
		}
		funxyArgs[i] = obj
	}

	// Use VM's internal evaluator to apply the function
	result, err := v.machine.CallFunction(fnObj, funxyArgs)
	if err != nil {
		return nil, err
	}

	return v.marshaller.FromValue(result, nil)
}

// Eval executes Funxy code string.
func (v *VM) Eval(code string) (interface{}, error) {
	// Create pipeline context
	ctx := pipeline.NewPipelineContext(code)
	ctx.FilePath = "<eval>"

	// Inject bindings
	for name, binding := range v.bindings {
		ctx.SymbolTable.Define(name, binding.Type, "embed")
	}

	// Run pipeline
	p := pipeline.New(
		&lexer.LexerProcessor{},
		&parser.ParserProcessor{},
		&analyzer.SemanticAnalyzerProcessor{},
		&CompilerProcessor{},
	)

	ctx = p.Run(ctx)

	if len(ctx.Errors) > 0 {
		errMsg := "Errors during compilation:\n"
		for _, e := range ctx.Errors {
			errMsg += fmt.Sprintf("%s\n", e.Error())
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	if ctx.BytecodeChunk == nil {
		return nil, nil
	}

	chunk, ok := ctx.BytecodeChunk.(*vm.Chunk)
	if !ok {
		return nil, fmt.Errorf("invalid bytecode chunk type")
	}

	// Process imports
	if err := v.machine.ProcessImports(chunk.PendingImports); err != nil {
		return nil, fmt.Errorf("import error: %w", err)
	}

	result, err := v.machine.Run(chunk)
	if err != nil {
		return nil, err
	}

	return v.marshaller.FromValue(result, nil)
}

// CompileFile parses, analyzes, and compiles a file, returning the chunk.
func (v *VM) CompileFile(path string) (*vm.Chunk, error) {
	if path == "" {
		return nil, fmt.Errorf("CompileFile: empty path")
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("CompileFile: file not found: %s", path)
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("CompileFile: permission denied: %s", path)
		}
		return nil, fmt.Errorf("CompileFile: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("CompileFile: expected file, got directory: %s", path)
	}
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("CompileFile: %w", err)
	}
	code := string(content)

	// Create pipeline context
	ctx := pipeline.NewPipelineContext(code)
	ctx.FilePath = path

	// Inject bindings into SymbolTable
	for name, binding := range v.bindings {
		ctx.SymbolTable.Define(name, binding.Type, "embed")
	}

	// Run pipeline
	p := pipeline.New(
		&lexer.LexerProcessor{},
		&parser.ParserProcessor{},
		&analyzer.SemanticAnalyzerProcessor{},
		&CompilerProcessor{},
	)

	ctx = p.Run(ctx)

	if len(ctx.Errors) > 0 {
		// Format errors
		errMsg := "Errors during compilation:\n"
		for _, e := range ctx.Errors {
			errMsg += fmt.Sprintf("%s\n", e.Error())
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	if ctx.BytecodeChunk == nil {
		return nil, nil // No code to run
	}

	chunk, ok := ctx.BytecodeChunk.(*vm.Chunk)
	if !ok {
		return nil, fmt.Errorf("invalid bytecode chunk type")
	}

	return chunk, nil
}

// RunChunk executes a compiled chunk.
func (v *VM) RunChunk(chunk *vm.Chunk, dir string) error {
	v.InjectStateHandler()

	// Pre-initialize evaluator so it's ready for any concurrent RPC calls
	// before the main loop starts executing instructions
	v.machine.GetEvaluator()

	v.machine.SetBaseDir(dir)
	if err := v.machine.ProcessImports(chunk.PendingImports); err != nil {
		return fmt.Errorf("import error: %w", err)
	}
	_, err := v.machine.Run(chunk)
	return err
}

// LoadFile parses, analyzes, compiles, and executes a file.
func (v *VM) LoadFile(path string) error {
	chunk, err := v.CompileFile(path)
	if err != nil {
		return err
	}
	if chunk == nil {
		return nil
	}
	return v.RunChunk(chunk, filepath.Dir(path))
}

// CompilerProcessor compiles AST to bytecode
type CompilerProcessor struct{}

func (cp *CompilerProcessor) Process(ctx *pipeline.PipelineContext) *pipeline.PipelineContext {
	if ctx.AstRoot == nil || len(ctx.Errors) > 0 {
		return ctx
	}

	program, ok := ctx.AstRoot.(*ast.Program)
	if !ok {
		return ctx
	}

	compiler := vm.NewCompiler()
	// Pass TypeMap
	compiler.SetTypeMap(ctx.TypeMap)
	// Set BaseDir for compiler if available
	if ctx.FilePath != "" {
		compiler.SetBaseDir(filepath.Dir(ctx.FilePath))
	}

	chunk, err := compiler.Compile(program)
	if err != nil {
		ctx.Errors = append(ctx.Errors, diagnostics.NewError(
			diagnostics.ErrC001,
			token.Token{},
			fmt.Sprintf("Compilation error: %s", err),
		))
	} else {
		ctx.BytecodeChunk = chunk
	}

	return ctx
}

// inferType generates a Funxy type from a Go value.
func inferType(val interface{}) typesystem.Type {
	t := reflect.TypeOf(val)
	if t == nil {
		return typesystem.TCon{Name: "Nil"}
	}

	switch t.Kind() {
	case reflect.Int, reflect.Int64:
		return typesystem.TCon{Name: "Int"}
	case reflect.Float64:
		return typesystem.TCon{Name: "Float"}
	case reflect.Bool:
		return typesystem.TCon{Name: "Bool"}
	case reflect.String:
		// String is List<Char>
		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: "List"},
			Args:        []typesystem.Type{typesystem.TCon{Name: "Char"}},
		}
	case reflect.Slice:
		elemType := inferType(reflect.Zero(t.Elem()).Interface())
		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: "List"},
			Args:        []typesystem.Type{elemType},
		}
	case reflect.Map:
		keyType := inferType(reflect.Zero(t.Key()).Interface())
		valType := inferType(reflect.Zero(t.Elem()).Interface())
		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: "Map"},
			Args:        []typesystem.Type{keyType, valType},
		}
	case reflect.Func:
		// Generate function type
		numIn := t.NumIn()
		isVariadic := t.IsVariadic()
		params := make([]typesystem.Type, numIn)
		for i := 0; i < numIn; i++ {
			paramType := t.In(i)
			// For variadic functions, the last param is []T — unwrap to T
			if isVariadic && i == numIn-1 {
				paramType = paramType.Elem()
			}
			params[i] = inferType(reflect.Zero(paramType).Interface())
		}

		var retType typesystem.Type
		if t.NumOut() > 0 {
			retType = inferType(reflect.Zero(t.Out(0)).Interface())
		} else {
			retType = typesystem.TCon{Name: "Nil"}
		}

		return typesystem.TFunc{
			Params:     params,
			ReturnType: retType,
			IsVariadic: isVariadic,
		}
	}

	// Default to generic or HostObject
	return typesystem.TCon{Name: "HostObject"}
}

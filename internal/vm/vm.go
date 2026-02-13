package vm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/typesystem"
	"path/filepath"
	"strings"
)

// formatFilePath formats a file path for display in stack traces
func formatFilePath(file string) string {
	if file == "" {
		return file
	}

	// Make path relative if it's absolute
	if filepath.IsAbs(file) {
		if wd, err := os.Getwd(); err == nil {
			if rel, err := filepath.Rel(wd, file); err == nil {
				file = rel
			}
		}
	}

	// Remove source extension for display
	file = config.TrimSourceExt(file)

	return file
}

var errEarlyReturn = errors.New("early return")
var errDebugBreak = errors.New("debug break")
var errTruncatedBytecode = errors.New("truncated bytecode")
var errStackUnderflow = errors.New("stack underflow")
var errStackOverflow = errors.New("stack overflow")
var errInvalidConstantIndex = errors.New("invalid constant index")

// Initial sizes for stack and frames
const InitialStackSize = 2048
const InitialFrameCount = 1024

// Growth increment when stack/frames need to expand
const StackGrowthIncrement = 1024
const FrameGrowthIncrement = 512

// Maximum call stack depth to prevent infinite recursion and stack overflow
const MaxFrameCount = 4096

// Maximum operand stack size to prevent OOM
const MaxStackSize = 1024 * 1024 // 1M elements

// CallFrame represents a single ongoing function call
type CallFrame struct {
	closure *ObjClosure // The closure being executed
	chunk   *Chunk      // The bytecode being executed (shortcut to closure.Function.Chunk)
	ip      int         // Instruction pointer within this frame's chunk
	base    int         // Base pointer: where this frame's locals start in the stack

	// ImplicitTypeContext is set by trait operators to guide dynamic dispatch
	ImplicitTypeContext string

	// ExplicitTypeContextDepth tracks the depth of explicit context stack when frame started
	ExplicitTypeContextDepth int
}

// VM is the virtual machine that executes bytecode
type VM struct {
	stack []Value
	sp    int // Stack pointer (points to next free slot)

	frames     []CallFrame // Call stack (dynamic)
	frameCount int

	// Current frame (for convenience)
	frame *CallFrame

	// Globals are stored in an immutable persistent map for thread safety
	// Wrapped in ModuleScope for shared mutable access within a module
	globals *ModuleScope

	// Linked list of open upvalues, sorted by stack location (highest first)
	openUpvalues *ObjUpvalue

	// Trait method registry: traitMethods[traitName] -> map[typeName] -> map[methodName] -> closure
	// Stored as nested PersistentMaps for efficient copy-on-write
	// Structure: traitName -> *PersistentMap (typeName -> *PersistentMap (methodName -> *ObjClosure))
	traitMethods *PersistentMap

	// Extension methods: extensionMethods[typeName] -> map[methodName] -> closure
	// Structure: typeName -> *PersistentMap (methodName -> *ObjClosure)
	extensionMethods *PersistentMap

	// Builtin trait methods (Go functions wrapped as BuiltinClosure)
	// Structure: key -> *BuiltinClosure
	builtinTraitMethods *PersistentMap

	// Type aliases for default() support
	typeAliases *PersistentMap

	// Trait default implementations for fallback (AST-based, for source execution)
	traitDefaults map[string]*ast.FunctionStatement

	// Pre-compiled trait defaults (bytecode-based, for bundle execution)
	compiledTraitDefaults map[string]*CompiledFunction

	// Embedded resources from bundle (static files: HTML, images, configs, etc.)
	// Key is relative path, value is file contents.
	resources map[string][]byte

	// Evaluator for builtin Go functions only (not for Funxy code!)
	eval *evaluator.Evaluator

	// Module loading support
	loader         *modules.Loader // Module loader for resolving imports
	baseDir        string          // Base directory for resolving relative imports
	currentFile    string          // Current file name for error messages
	moduleCache    *PersistentMap  // Cache of compiled/executed modules
	loadingModules map[string]bool // Modules currently being loaded (for cyclic detection)

	// Bundle for self-contained bytecode execution (nil when running from source)
	bundle *Bundle

	// Type context stack for ClassMethod dispatch
	typeContextStack []string

	// Pending implicit context for next call
	nextImplicitContext string

	// TypeMap from analyzer (optional, for type-based dispatch in evaluator fallback)
	typeMap map[ast.Node]typesystem.Type

	// Output writer (defaults to os.Stdout)
	out io.Writer

	// Debugger for debugging support
	debugger *Debugger

	// Context for cancellation
	Context context.Context
}

// New creates a new VM instance
func New() *VM {
	return &VM{
		stack:               make([]Value, 0, InitialStackSize),      // Dynamic stack with initial capacity
		frames:              make([]CallFrame, 0, InitialFrameCount), // Dynamic frames with initial capacity
		globals:             NewModuleScope(),
		traitMethods:        EmptyMap(),
		extensionMethods:    EmptyMap(),
		builtinTraitMethods: EmptyMap(),
		typeContextStack:    make([]string, 0, 16),
		typeAliases:         EmptyMap(),
		moduleCache:         EmptyMap(),
		out:                 os.Stdout,
		typeMap:             make(map[ast.Node]typesystem.Type),
		debugger:            NewDebugger(),
	}
}

// SetOutput sets the output writer for the VM (and its internal evaluator)
func (vm *VM) SetOutput(w io.Writer) {
	vm.out = w
	if vm.eval != nil {
		vm.eval.Out = w
	}
}

// SetContext sets the context for cancellation
func (vm *VM) SetContext(ctx context.Context) {
	vm.Context = ctx
	if vm.eval != nil {
		vm.eval.Context = ctx
	}
}

// SetTypeMap sets the type map from analyzer
func (vm *VM) SetTypeMap(typeMap map[ast.Node]typesystem.Type) {
	vm.typeMap = typeMap
}

// SetTypeAliases sets the type aliases from compiler
func (vm *VM) SetTypeAliases(aliases map[string]typesystem.Type) {
	for k, v := range aliases {
		vm.typeAliases = vm.typeAliases.Put(k, &evaluator.TypeObject{TypeVal: v})
	}
}

// SetTraitDefaults sets the trait default implementations from analyzer
func (vm *VM) SetTraitDefaults(defaults map[string]*ast.FunctionStatement) {
	vm.traitDefaults = defaults
}

// SetBundle sets the bytecode bundle for self-contained execution.
// When set, the VM resolves user module imports from the bundle
// instead of loading from disk.
func (vm *VM) SetBundle(b *Bundle) {
	vm.bundle = b
	if b != nil && b.Resources != nil {
		vm.resources = b.Resources
		// If evaluator was already created (e.g. by RegisterFPTraits),
		// sync resources to it now.
		if vm.eval != nil {
			vm.eval.EmbeddedResources = vm.resources
		}
	}
}

// SetLoader sets the module loader for resolving imports
func (vm *VM) SetLoader(loader *modules.Loader) {
	vm.loader = loader
}

// RegisterTraitMethod registers a compiled method for a trait/type combination
func (vm *VM) RegisterTraitMethod(traitName, typeName, methodName string, closure *ObjClosure) {
	// traitMethods[traitName]
	var typeMap *PersistentMap
	if val := vm.traitMethods.Get(traitName); val != nil {
		typeMap = val.(*PersistentMap)
	} else {
		typeMap = EmptyMap()
	}

	// traitMethods[traitName][typeName]
	var methodMap *PersistentMap
	if val := typeMap.Get(typeName); val != nil {
		methodMap = val.(*PersistentMap)
	} else {
		methodMap = EmptyMap()
	}

	// traitMethods[traitName][typeName][methodName] = closure
	methodMap = methodMap.Put(methodName, closure)
	typeMap = typeMap.Put(typeName, methodMap)
	vm.traitMethods = vm.traitMethods.Put(traitName, typeMap)
}

// LookupTraitMethod finds a trait method for a given type
func (vm *VM) LookupTraitMethod(traitName, typeName, methodName string) *ObjClosure {
	if typeMapObj := vm.traitMethods.Get(traitName); typeMapObj != nil {
		typeMap := typeMapObj.(*PersistentMap)
		if methodMapObj := typeMap.Get(typeName); methodMapObj != nil {
			methodMap := methodMapObj.(*PersistentMap)
			if closureObj := methodMap.Get(methodName); closureObj != nil {
				return closureObj.(*ObjClosure)
			}
		}
	}
	return nil
}

// LookupTraitMethodFuzzy finds a trait method by matching argument types against instance keys
func (vm *VM) LookupTraitMethodFuzzy(traitName, methodName string, args []evaluator.Object, context string) evaluator.Object {
	typeMapObj := vm.traitMethods.Get(traitName)
	if typeMapObj == nil {
		return nil
	}
	typeMap := typeMapObj.(*PersistentMap)

	var bestMatch evaluator.Object
	bestScore := -1

	// Iterate over all keys in typeMap
	typeMap.Range(func(key string, val evaluator.Object) bool {
		// key is the instance type signature (e.g. "Int_String" or "Int")
		parts := strings.Split(key, "_")

		// Check match with args
		match := true
		score := 0

		// Check args matches (Prefix)
		argCount := len(args)
		// We only check args we have. If key is longer, that's fine (partial match on key).
		// If args are longer than key? Then key is too short, mismatch.
		if argCount > len(parts) {
			match = false
		} else {
			for i, arg := range args {
				argType := vm.getTypeName(ObjectToValue(arg))
				if parts[i] != argType {
					match = false
					break
				}
				score++ // +1 for each arg match
			}
		}

		if match {
			// Boost score if context matches any remaining part of the key
			if context != "" && len(parts) > argCount {
				for i := argCount; i < len(parts); i++ {
					if parts[i] == context {
						score++
						break // Only count once
					}
				}
			}

			if score > bestScore {
				methodMap := val.(*PersistentMap)
				if closureObj := methodMap.Get(methodName); closureObj != nil {
					bestMatch = closureObj
					bestScore = score
				}
			}
		}
		return true
	})

	if bestMatch != nil {
		return bestMatch
	}

	// Fallback: fuzzy search in builtinTraitMethods
	// Key format: Trait.Type.Method
	prefix := traitName + "."
	suffix := "." + methodName

	vm.builtinTraitMethods.Range(func(key string, val evaluator.Object) bool {
		if strings.HasPrefix(key, prefix) && strings.HasSuffix(key, suffix) {
			// Extract Type part: prefix<Type>suffix
			typePart := key[len(prefix) : len(key)-len(suffix)]
			parts := strings.Split(typePart, "_")

			match := true
			score := 0
			argCount := len(args)

			if argCount > len(parts) {
				match = false
			} else {
				for i, arg := range args {
					argType := vm.getTypeName(ObjectToValue(arg))
					if parts[i] != argType {
						match = false
						break
					}
					score++
				}
			}

			if match {
				if context != "" && len(parts) > argCount {
					for i := argCount; i < len(parts); i++ {
						if parts[i] == context {
							score++
							break
						}
					}
				}

				if score > bestScore {
					bestMatch = val
					bestScore = score
				}
			}
		}
		return true
	})

	return bestMatch
}

// LookupTraitMethodAny finds a trait method, returning either ObjClosure or BuiltinClosure
func (vm *VM) LookupTraitMethodAny(traitName, typeName, methodName string) evaluator.Object {
	// First check compiled closures
	if closure := vm.LookupTraitMethod(traitName, typeName, methodName); closure != nil {
		return closure
	}
	// Then check builtin closures
	key := traitName + "." + typeName + "." + methodName
	if bc := vm.builtinTraitMethods.Get(key); bc != nil {
		return bc
	}
	return nil
}

// LookupOperator finds an operator method for a type (searches all traits)
func (vm *VM) LookupOperator(typeName, operator string) *ObjClosure {
	methodName := "(" + operator + ")"
	// Iterate over all traits
	var found *ObjClosure
	vm.traitMethods.Range(func(traitName string, typeMapObj evaluator.Object) bool {
		typeMap := typeMapObj.(*PersistentMap)
		if methodMapObj := typeMap.Get(typeName); methodMapObj != nil {
			methodMap := methodMapObj.(*PersistentMap)
			if closureObj := methodMap.Get(methodName); closureObj != nil {
				found = closureObj.(*ObjClosure)
				return false // Stop iteration
			}
		}
		return true // Continue iteration
	})
	return found
}

// LookupBuiltinOperator finds a builtin operator method for a type
func (vm *VM) LookupBuiltinOperator(typeName, operator string) *BuiltinClosure {
	methodName := "(" + operator + ")"
	// Try to find key ending with .typeName.methodName
	// This is inefficient with HAMT if we don't know the trait name.
	// But builtinTraitMethods keys are "Trait.Type.Method".
	// We need to iterate.
	var found *BuiltinClosure
	suffix := "." + typeName + "." + methodName
	vm.builtinTraitMethods.Range(func(key string, val evaluator.Object) bool {
		if strings.HasSuffix(key, suffix) {
			found = val.(*BuiltinClosure)
			return false
		}
		return true
	})
	return found
}

// getTypeContext returns the current type context for dispatch
func (vm *VM) getTypeContext() string {
	// 1. Check implicit context from frame (from trait operators)
	// Priority: Implicit > Explicit.
	if vm.frame != nil && vm.frame.ImplicitTypeContext != "" {
		return vm.frame.ImplicitTypeContext
	}
	// 2. Check explicit context stack (from compiler)
	if len(vm.typeContextStack) > 0 {
		return vm.typeContextStack[len(vm.typeContextStack)-1]
	}
	return ""
}

// compileTraitDefault JIT-compiles a trait default method for a specific type
func (vm *VM) compileTraitDefault(fn *ast.FunctionStatement, traitName, typeName string) (*ObjClosure, error) {
	// Create a mini-compiler for this function
	compiler := &Compiler{
		function: &CompiledFunction{
			Chunk: NewChunk(),
			Name:  fn.Name.Value,
			Arity: len(fn.Parameters),
		},
		funcType:    TYPE_FUNCTION,
		locals:      make([]Local, 256),
		upvalues:    make([]Upvalue, 256),
		typeAliases: make(map[string]typesystem.Type),
		scopeDepth:  1, // Function body starts at depth 1
	}

	// Copy type aliases from VM to compiler
	vm.typeAliases.Range(func(key string, val evaluator.Object) bool {
		if typeObj, ok := val.(*evaluator.TypeObject); ok {
			compiler.typeAliases[key] = typeObj.TypeVal
		}
		return true
	})

	// Add parameters as locals at depth 1
	for i, param := range fn.Parameters {
		compiler.locals[i] = Local{Name: param.Name.Value, Depth: 1, Slot: i}
	}
	compiler.localCount = len(fn.Parameters)
	compiler.slotCount = len(fn.Parameters)

	// Compile the function body
	if err := compiler.compileFunctionBody(fn.Body); err != nil {
		return nil, err
	}

	compiledFn := compiler.function
	compiledFn.LocalCount = compiler.localCount
	compiledFn.UpvalueCount = compiler.upvalueCount
	compiledFn.RequiredArity = len(fn.Parameters)

	// Create closure
	closure := &ObjClosure{
		Function: compiledFn,
		Upvalues: make([]*ObjUpvalue, 0),
	}

	return closure, nil
}

// SetBaseDir sets the base directory for resolving relative imports
func (vm *VM) SetBaseDir(dir string) {
	vm.baseDir = dir
}

// SetCurrentFile sets the current file name for error messages
func (vm *VM) SetCurrentFile(file string) {
	vm.currentFile = file
}

// GetDebugger returns the debugger instance
func (vm *VM) GetDebugger() *Debugger {
	return vm.debugger
}

// EnableDebugger enables debugging
func (vm *VM) EnableDebugger() {
	if vm.debugger != nil {
		vm.debugger.Enabled = true
	}
}

// DisableDebugger disables debugging
func (vm *VM) DisableDebugger() {
	if vm.debugger != nil {
		vm.debugger.Enabled = false
		vm.debugger.Run()
	}
}

// getEvaluator returns or creates the evaluator for builtin calls
func (vm *VM) getEvaluator() *evaluator.Evaluator {
	if vm.eval == nil {
		vm.eval = evaluator.New()
		vm.eval.Out = vm.out
		vm.eval.Context = vm.Context // Propagate context
		vm.eval.BaseDir = "."
		vm.eval.CurrentFile = "<vm>"
		// Set VMCallHandler to allow builtins to call VM closures
		vm.eval.VMCallHandler = vm.vmCallHandler
		vm.eval.AsyncHandler = vm.asyncHandler
		vm.eval.CaptureHandler = vm.captureHandler
		// Initialize GlobalEnv if not set
		if vm.eval.GlobalEnv == nil {
			vm.eval.GlobalEnv = evaluator.NewEnvironment()
		}
		// Pass type aliases for default() support
		evalAliases := make(map[string]typesystem.Type)
		vm.typeAliases.Range(func(key string, val evaluator.Object) bool {
			if typeObj, ok := val.(*evaluator.TypeObject); ok {
				evalAliases[key] = typeObj.TypeVal
			}
			return true
		})
		vm.eval.TypeAliases = evalAliases
		// Pass trait default implementations
		vm.eval.TraitDefaults = vm.traitDefaults
		// Pass type map for type-based dispatch
		vm.eval.TypeMap = vm.typeMap
		// Pass embedded resources for file I/O builtins
		vm.eval.EmbeddedResources = vm.resources

		// Set Fork function for thread-safe isolation (e.g. for taskMap)
		vm.eval.Fork = func() *evaluator.Evaluator {
			newVM := vm.ForkVM()
			newEval := newVM.getEvaluator()
			// Inherit environment for non-compiled code (if any)
			if vm.eval.GlobalEnv != nil {
				// We don't want to share the exact same env pointer if it's mutable?
				// But GlobalEnv in VM is sync'd from vm.globals.
				// newVM has its own globals map.
				// newEval.GlobalEnv should be initialized from newVM.globals.
				// But what about closures capturing env?
				// VM closures use Upvalues, not env.
				// Tree-walk closures use env.
				// If we have mixed code, we might need env copy.
				// newEval.GlobalEnv is already initialized by getEvaluator().
				// But let's copy local env if we were inside a function?
				// No, Fork() is called from builtinTaskMap which has 'e'.
				// 'e' is the evaluator of the caller.
				// If caller has local env, Clone() copies it.
				// Fork() should probably do the same?
				// But vm.getEvaluator() creates a fresh evaluator with EMPTY local env?
				// Wait. vm.eval is the evaluator for BUILTINS. It doesn't track local env of running VM code.
				// VM tracks locals in stack.
				// So vm.eval usually has only GlobalEnv.
				// So we are good.
			}
			return newEval
		}

		// Register FP traits (Semigroup, Monad, etc.) and operator mappings
		evaluator.RegisterFPTraits(vm.eval, vm.eval.GlobalEnv)
	}
	// Sync VM globals to evaluator's GlobalEnv for trait dispatch
	if vm.eval.GlobalEnv != nil {
		if vm.globals != nil && vm.globals.Globals != nil {
			vm.globals.Globals.Range(func(name string, obj evaluator.Object) bool {
				vm.eval.GlobalEnv.Set(name, obj)
				return true
			})
		}
	}
	// Sync VM trait methods to evaluator's ClassImplementations
	vm.traitMethods.Range(func(traitName string, typeMapObj evaluator.Object) bool {
		if vm.eval.ClassImplementations[traitName] == nil {
			vm.eval.ClassImplementations[traitName] = make(map[string]evaluator.Object)
		}
		typeMap := typeMapObj.(*PersistentMap)
		typeMap.Range(func(typeName string, methodMapObj evaluator.Object) bool {
			methodMap := methodMapObj.(*PersistentMap)
			// Create MethodTable for this type
			methodTable := &evaluator.MethodTable{
				Methods: make(map[string]evaluator.Object),
			}
			methodMap.Range(func(methodName string, closureObj evaluator.Object) bool {
				methodTable.Methods[methodName] = closureObj
				return true
			})

			// Fix: Don't overwrite existing implementation if new table is empty
			// This prevents empty entries in vm.traitMethods (from unknown source)
			// from clobbering builtin instances.
			if len(methodTable.Methods) == 0 {
				if existing, ok := vm.eval.ClassImplementations[traitName][typeName]; ok {
					if mt, ok := existing.(*evaluator.MethodTable); ok && len(mt.Methods) > 0 {
						return true
					}
				}
			}

			vm.eval.ClassImplementations[traitName][typeName] = methodTable
			return true
		})
		return true
	})
	return vm.eval
}

// getDefaultForRecord creates a default value for a record type
func (vm *VM) getDefaultForRecord(t typesystem.Type, typeName string) evaluator.Object {
	rec, ok := t.(typesystem.TRecord)
	if !ok {
		return nil
	}

	fields := make(map[string]evaluator.Object)
	for name, fieldType := range rec.Fields {
		fieldDefault := vm.getDefaultForType(fieldType)
		if fieldDefault == nil {
			return nil
		}
		fields[name] = fieldDefault
	}

	recordInstance := evaluator.NewRecord(fields)
	recordInstance.TypeName = typeName
	return recordInstance
}

// getDefaultForType returns default value for a basic type
func (vm *VM) getDefaultForType(t typesystem.Type) evaluator.Object {
	switch typ := t.(type) {
	case typesystem.TCon:
		switch typ.Name {
		case "Int":
			return &evaluator.Integer{Value: 0}
		case "Float":
			return &evaluator.Float{Value: 0.0}
		case "Bool":
			return &evaluator.Boolean{Value: false}
		case "String":
			return evaluator.NewList([]evaluator.Object{}) // empty string
		case "Char":
			return &evaluator.Char{Value: 0}
		}
		// Check if it's a type alias
		if val := vm.typeAliases.Get(typ.Name); val != nil {
			if typeObj, ok := val.(*evaluator.TypeObject); ok {
				return vm.getDefaultForRecord(typeObj.TypeVal, typ.Name)
			}
		}
	case typesystem.TRecord:
		return vm.getDefaultForRecord(t, "")
	}
	return nil
}

// vmCallHandler handles calls to VM closures from builtins
func (vm *VM) vmCallHandler(closure evaluator.Object, args []evaluator.Object) evaluator.Object {
	// Handle VMComposedFunction
	if composed, ok := closure.(*VMComposedFunction); ok {
		if len(args) != 1 {
			return &evaluator.Error{Message: fmt.Sprintf("composed function expects 1 argument, got %d", len(args))}
		}
		gResult, err := vm.callAndGetResult(ObjectToValue(composed.G), ObjectToValue(args[0]))
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		fResult, err := vm.callAndGetResult(ObjectToValue(composed.F), gResult)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return fResult.AsObject()
	}

	// Check if it's a VM closure
	vmClosure, ok := closure.(*ObjClosure)
	if !ok {
		return nil // Not a VM closure, let default handler report error
	}

	fn := vmClosure.Function

	// Check arity - partial application if not enough args
	if len(args) < fn.Arity {
		return &evaluator.PartialApplication{
			VMClosure:   closure,
			AppliedArgs: args,
		}
	}
	if len(args) > fn.Arity && !fn.IsVariadic {
		return &evaluator.Error{Message: fmt.Sprintf("expected %d arguments but got %d", fn.Arity, len(args))}
	}

	// Save current VM state
	savedFrameCount := vm.frameCount
	savedSp := vm.sp

	// Push arguments onto stack (closure is NOT pushed - callClosureDirect handles it)
	for _, arg := range args {
		vm.push(ObjectToValue(arg))
	}

	// Grow frames array if needed
	if vm.frameCount >= len(vm.frames) {
		// Grow by increment or double, whichever is larger
		growBy := FrameGrowthIncrement
		if len(vm.frames) > growBy {
			growBy = len(vm.frames)
		}
		newFrames := make([]CallFrame, len(vm.frames)+growBy)
		copy(newFrames, vm.frames[:vm.frameCount])
		vm.frames = newFrames
	}

	frame := &vm.frames[vm.frameCount]
	frame.closure = vmClosure
	frame.chunk = fn.Chunk
	frame.ip = 0
	frame.base = vm.sp - len(args)

	// Inherit implicit context from current frame (caller)
	if vm.frame != nil {
		frame.ImplicitTypeContext = vm.frame.ImplicitTypeContext
	} else {
		frame.ImplicitTypeContext = ""
	}
	// Check if evaluator has ContainerContext set (from builtin call like >>=)
	// This allows builtins (like Monad.bind) to propagate context to callbacks
	if vm.eval != nil && vm.eval.ContainerContext != "" {
		frame.ImplicitTypeContext = vm.eval.ContainerContext
	}
	// If nextImplicitContext is set (e.g. by OP_TRAIT_OP), override/use it
	if vm.nextImplicitContext != "" {
		frame.ImplicitTypeContext = vm.nextImplicitContext
		vm.nextImplicitContext = ""
	}

	frame.ExplicitTypeContextDepth = len(vm.typeContextStack)

	vm.frameCount++
	vm.frame = frame

	// Execute until this call returns
	targetFrameCount := savedFrameCount

	for vm.frameCount > targetFrameCount {
		result, done, err := vm.step()
		if err != nil {
			// Check for early return signal
			if errors.Is(err, errEarlyReturn) {
				vm.performReturn()
				if vm.frameCount <= targetFrameCount {
					// We returned from the function called by vmCallHandler
					// Result is on top of stack (pushed by performReturn)
					resultVal := vm.pop()

					// Restore stack pointer to before arguments were pushed
					// performReturn sets sp to frame.base + 1 (result)
					// frame.base is usually savedSp
					// So sp is savedSp + 1. pop() makes it savedSp.
					vm.sp = savedSp
					// Restore frame safely (handle slice growth)
					if savedFrameCount > 0 {
						vm.frame = &vm.frames[savedFrameCount-1]
					} else {
						vm.frame = nil
					}
					return resultVal.AsObject()
				}
				continue
			}

			vm.frameCount = savedFrameCount
			vm.sp = savedSp
			// Restore frame safely
			if savedFrameCount > 0 {
				vm.frame = &vm.frames[savedFrameCount-1]
			} else {
				vm.frame = nil
			}
			return &evaluator.Error{Message: err.Error()}
		}
		if done && vm.frameCount <= targetFrameCount {
			// step() pushed result onto stack, but we return directly to builtin
			// Restore stack pointer to before arguments were pushed
			vm.sp = savedSp
			// Restore frame safely
			if savedFrameCount > 0 {
				vm.frame = &vm.frames[savedFrameCount-1]
			} else {
				vm.frame = nil
			}
			return result.AsObject()
		}
	}

	vm.sp = savedSp
	// Restore frame safely
	if savedFrameCount > 0 {
		vm.frame = &vm.frames[savedFrameCount-1]
	} else {
		vm.frame = nil
	}
	return &evaluator.Nil{}
}

// step executes one instruction and returns (result, done, error)
// done is true if OP_RETURN or OP_HALT was executed
func (vm *VM) step() (res Value, done bool, err error) {
	// Recover from truncated bytecode panic
	defer func() {
		if r := recover(); r != nil {
			if r == errTruncatedBytecode || r == errStackUnderflow || r == errInvalidConstantIndex {
				err = r.(error)
				res = NilVal()
				done = false
			} else {
				panic(r) // Re-panic other errors
			}
		}
	}()

	// Check debugger breakpoint before executing instruction
	if vm.debugger != nil && vm.debugger.Enabled && vm.debugger.ShouldBreak(vm) {
		// Debugger wants to break - call OnStop callback if set
		if vm.debugger.OnStop != nil {
			vm.debugger.OnStop(vm.debugger, vm)
		}
		// Return special error to signal debug break
		return NilVal(), false, errDebugBreak
	}

	if vm.frame.ip >= len(vm.frame.chunk.Code) {
		// End of code reached without explicit return/halt
		// This can happen if control flow falls off the end
		// Implicit return nil
		return NilVal(), true, nil
	}

	op := Opcode(vm.frame.chunk.Code[vm.frame.ip])
	vm.frame.ip++

	switch op {
	case OP_RETURN:
		result := vm.pop() // result is Value

		// If result is a RecordInstance and has no TypeName, try to set it from function's return type
		if result.IsObj() {
			if record, ok := result.Obj.(*evaluator.RecordInstance); ok && record.TypeName == "" {
				fn := vm.frame.closure.Function
				if fn.TypeInfo != nil {
					if tFunc, ok := fn.TypeInfo.(typesystem.TFunc); ok {
						if tCon, ok := tFunc.ReturnType.(typesystem.TCon); ok {
							record.TypeName = tCon.Name
						}
					}
				}
			}
		}

		// Truncate explicit type context stack to restore state
		if len(vm.typeContextStack) > vm.frame.ExplicitTypeContextDepth {
			vm.typeContextStack = vm.typeContextStack[:vm.frame.ExplicitTypeContextDepth]
		}

		vm.closeUpvalues(vm.frame.base)
		vm.frameCount--
		if vm.frameCount == 0 {
			return result, true, nil
		}
		vm.sp = vm.frame.base
		vm.frame = &vm.frames[vm.frameCount-1]
		vm.push(result) // Push Value directly

		return result, true, nil

	case OP_HALT:
		vm.frameCount = 0 // Signal complete termination
		if vm.sp > 0 {
			return vm.pop(), true, nil
		}
		return NilVal(), true, nil

	default:
		err := vm.executeOneOp(op)
		return NilVal(), false, err
	}
}

// executeOneOp executes a single opcode (except RETURN and HALT)
func (vm *VM) Run(chunk *Chunk) (result evaluator.Object, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok && e == errStackOverflow {
				err = e
				return
			}
			panic(r)
		}
	}()

	// Create a "script" function and closure for top-level code
	scriptFn := &CompiledFunction{
		Chunk: chunk,
		Name:  "<script>",
	}
	scriptClosure := &ObjClosure{
		Function: scriptFn,
		Upvalues: nil,
		Globals:  vm.globals,
	}

	// Reset stack and frames - allocate full size initially for compatibility
	vm.stack = make([]Value, InitialStackSize)
	vm.sp = 0
	vm.frames = make([]CallFrame, InitialFrameCount)

	// Set up the initial frame for top-level code
	vm.frameCount = 1
	vm.frames[0] = CallFrame{
		closure:                  scriptClosure,
		chunk:                    chunk,
		ip:                       0,
		base:                     0,                   // Top-level code starts at stack base 0
		ImplicitTypeContext:      vm.getTypeContext(), // Inherit context (if any)
		ExplicitTypeContextDepth: len(vm.typeContextStack),
	}
	vm.frame = &vm.frames[0]
	vm.openUpvalues = nil

	// Execute with debugger support
	resultVal, err := vm.executeWithDebugger()
	if err != nil {
		return nil, err
	}
	return resultVal.AsObject(), nil
}

// execute is the main interpreter loop
func (vm *VM) execute() (Value, error) {
	return vm.executeWithDebugger()
}

// executeWithDebugger is the main interpreter loop with debugger support
func (vm *VM) executeWithDebugger() (Value, error) {
	// Instruction counter for periodic context checks
	opsSinceCheck := 0
	const checkInterval = 1000

	for {
		// Check for cancellation periodically
		opsSinceCheck++
		if opsSinceCheck >= checkInterval {
			opsSinceCheck = 0
			if vm.Context != nil {
				select {
				case <-vm.Context.Done():
					return NilVal(), vm.Context.Err()
				default:
				}
			}
		}

		result, done, err := vm.step()
		if err != nil {
			// Check for early return signal
			if errors.Is(err, errEarlyReturn) {
				// Perform return with value on stack
				vm.performReturn()
				if vm.frameCount == 0 {
					return vm.pop(), nil
				}
				continue
			}
			// Check for debug break
			if errors.Is(err, errDebugBreak) {
				// Execution paused by debugger
				// The debugger's OnStop callback will handle the pause and wait for user input
				// After user enters 'continue', execution will resume from this point
				// So we just continue the loop (don't return)
				continue
			}
			// Add line info and stack trace to error
			return NilVal(), vm.formatError(err)
		}
		if done {
			// HALT always returns, RETURN returns when frameCount becomes 0
			if vm.frameCount == 0 {
				return result, nil
			}
		}
	}
}

// performReturn handles the return mechanics
func (vm *VM) performReturn() {
	result := vm.pop() // result is Value
	vm.returnWithValue(result)
}

// returnWithValue returns from current frame with specified value
func (vm *VM) returnWithValue(result Value) {
	frame := vm.frame

	// If result is a RecordInstance and has no TypeName, try to set it from function's return type
	if result.IsObj() {
		if record, ok := result.Obj.(*evaluator.RecordInstance); ok && record.TypeName == "" {
			fn := frame.closure.Function
			if fn.TypeInfo != nil {
				if tFunc, ok := fn.TypeInfo.(typesystem.TFunc); ok {
					if tCon, ok := tFunc.ReturnType.(typesystem.TCon); ok {
						record.TypeName = tCon.Name
					}
				}
			}
		}
	}

	// Truncate explicit type context stack to restore state
	if len(vm.typeContextStack) > frame.ExplicitTypeContextDepth {
		vm.typeContextStack = vm.typeContextStack[:frame.ExplicitTypeContextDepth]
	}

	// Close all upvalues that point into this frame's stack slots
	vm.closeUpvalues(frame.base)

	vm.frameCount--

	// Restore sp to frame.base (where args started, replacing function slot)
	vm.sp = frame.base

	if vm.frameCount > 0 {
		vm.frame = &vm.frames[vm.frameCount-1]
	}

	// Push result back
	vm.push(result)
}

// callValue handles function calls
// Stack operations
func (vm *VM) push(obj Value) {
	// Grow stack if needed
	if vm.sp >= len(vm.stack) {
		if vm.sp >= MaxStackSize {
			panic(errStackOverflow)
		}
		// Grow by increment or double, whichever is larger
		growBy := StackGrowthIncrement
		if len(vm.stack) > growBy {
			growBy = len(vm.stack)
		}
		newStack := make([]Value, len(vm.stack)+growBy)
		copy(newStack, vm.stack[:vm.sp])
		vm.stack = newStack
	}

	// Safety check for sp
	if vm.sp < 0 {
		panic(fmt.Sprintf("stack underflow in push (sp=%d)", vm.sp))
	}

	vm.stack[vm.sp] = obj
	vm.sp++
}

func (vm *VM) pop() Value {
	if vm.sp <= 0 {
		panic(errStackUnderflow)
	}
	vm.sp--
	return vm.stack[vm.sp]
}

func (vm *VM) peek(distance int) Value {
	idx := vm.sp - 1 - distance
	if idx < 0 {
		panic(errStackUnderflow)
	}
	return vm.stack[idx]
}

// checkStack ensures there are at least n elements on the stack
func (vm *VM) checkStack(n int) {
	if vm.sp < n {
		panic(errStackUnderflow)
	}
}

// Read helpers
func (vm *VM) readByte() byte {
	if vm.frame.ip >= len(vm.frame.chunk.Code) {
		panic(errTruncatedBytecode)
	}
	b := vm.frame.chunk.Code[vm.frame.ip]
	vm.frame.ip++
	return b
}

func (vm *VM) readConstantIndex() int {
	high := vm.readByte()
	low := vm.readByte()
	return int(high)<<8 | int(low)
}

func (vm *VM) readConstant() evaluator.Object {
	idx := vm.readConstantIndex()
	if idx >= len(vm.frame.chunk.Constants) {
		panic(errInvalidConstantIndex)
	}
	return vm.frame.chunk.Constants[idx]
}

func (vm *VM) readJumpOffset() int {
	high := vm.readByte()
	low := vm.readByte()
	return int(high)<<8 | int(low)
}

// Binary operations
func (vm *VM) objectToString(obj evaluator.Object) string {
	switch v := obj.(type) {
	case *evaluator.List:
		// Check if it's a string (List<Char>)
		if evaluator.IsStringList(v) {
			return evaluator.ListToString(v)
		}
		// Otherwise use Inspect for list representation
		return v.Inspect()
	default:
		return obj.Inspect()
	}
}

// getTypeName returns the type name for trait lookup
func (vm *VM) getTypeName(obj Value) string {
	if obj.IsInt() {
		return "Int"
	}
	if obj.IsFloat() {
		return "Float"
	}
	if obj.IsBool() {
		return "Bool"
	}
	if obj.IsNil() {
		return "Nil"
	}

	// It's an object
	switch o := obj.Obj.(type) {
	case *evaluator.DataInstance:
		return o.TypeName
	case *evaluator.Integer:
		return "Int"
	case *evaluator.Float:
		return "Float"
	case *evaluator.Boolean:
		return "Bool"
	case *evaluator.List:
		// Check if it's a string (List Char)
		if evaluator.IsStringList(o) {
			return "String"
		}
		return "List"
	case *evaluator.Tuple:
		return "Tuple"
	case *evaluator.RecordInstance:
		if o.TypeName != "" {
			return o.TypeName
		}
		return "Record"
	case *evaluator.Map:
		return "Map"
	case *evaluator.Bytes:
		return "Bytes"
	case *evaluator.Bits:
		return "Bits"
	case *evaluator.BigInt:
		return "BigInt"
	case *evaluator.Rational:
		return "Rational"
	case *evaluator.Char:
		return "Char"
	case *evaluator.TypeObject:
		return "Type"
	case *evaluator.Nil:
		return "Nil"
	case *ObjClosure:
		return "Function"
	case *evaluator.Function:
		return "Function"
	case *evaluator.Builtin:
		return "Function"
	case *evaluator.Task:
		return "Task"
	default:
		return string(obj.Obj.Type())
	}
}

// Helper functions

func (vm *VM) isTruthy(obj Value) bool {
	if obj.IsBool() {
		return obj.AsBool()
	}
	// Check for boxed Boolean
	if obj.IsObj() {
		if b, ok := obj.Obj.(*evaluator.Boolean); ok {
			return b.Value
		}
	}
	return false
}

func (vm *VM) runtimeError(format string, args ...interface{}) error {
	// Just return the formatted message - formatError will add line info
	return fmt.Errorf(format, args...)
}

// runtimeErrorWithCallee creates an error that includes the name of the called function
func (vm *VM) runtimeErrorWithCallee(callee string, format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return &calleeError{message: msg, callee: callee}
}

// calleeError is an error that includes the name of the called function
type calleeError struct {
	message string
	callee  string
}

func (e *calleeError) Error() string {
	return e.message
}

// captureUpvalue creates or reuses an upvalue pointing to the given stack location
func (vm *VM) captureUpvalue(location int) *ObjUpvalue {
	// Walk through the open upvalue list looking for an existing one
	var prev *ObjUpvalue
	upvalue := vm.openUpvalues

	// The list is sorted by location (highest first)
	for upvalue != nil && upvalue.Location > location {
		prev = upvalue
		upvalue = upvalue.Next
	}

	// If we found an existing upvalue at this location, reuse it
	if upvalue != nil && upvalue.Location == location {
		return upvalue
	}

	// Create a new upvalue
	created := &ObjUpvalue{
		Location: location,
		Next:     upvalue, // Link to the next (lower) upvalue
	}

	// Insert into the list
	if prev == nil {
		vm.openUpvalues = created
	} else {
		prev.Next = created
	}

	return created
}

// closeUpvalues closes all upvalues that point to stack locations >= lastSlot
func (vm *VM) closeUpvalues(lastSlot int) {
	for vm.openUpvalues != nil && vm.openUpvalues.Location >= lastSlot {
		upvalue := vm.openUpvalues

		// "Close" the upvalue: copy value from stack to Closed field
		// Closed field is still Object, so we must box it.
		// But wait, ObjUpvalue expects Object.
		// Let's assume we box it for now.
		upvalue.Closed = vm.stack[upvalue.Location].AsObject()
		upvalue.Location = -1 // Mark as closed

		vm.openUpvalues = upvalue.Next
	}
}

// isEmptyDataInstance checks if DataInstance represents an "empty" value
// Uses Optional trait's isEmpty method if available, otherwise checks builtin types
func (vm *VM) isEmptyDataInstance(data *evaluator.DataInstance) bool {
	// First check builtin Option/Result types for fast path
	switch data.Name {
	case config.NoneCtorName, config.FailCtorName:
		return true
	case config.SomeCtorName, config.OkCtorName:
		return false
	}

	// For user-defined types, check if they implement Optional trait
	typeName := data.TypeName
	if typeName == "" {
		typeName = data.Name
	}

	// Look for isEmpty method in Optional trait
	isEmptyMethod := vm.LookupTraitMethodAny("Optional", typeName, "isEmpty")
	if isEmptyMethod == nil {
		// Also check Empty trait (parent of Optional)
		isEmptyMethod = vm.LookupTraitMethodAny("Empty", typeName, "isEmpty")
	}

	if isEmptyMethod != nil {
		// Call isEmpty method to determine if empty
		eval := vm.getEvaluator()
		result := eval.ApplyFunction(isEmptyMethod, []evaluator.Object{data})
		if boolVal, ok := result.(*evaluator.Boolean); ok {
			return boolVal.Value
		}
	}

	// Default: not empty
	return false
}

// isWrapperDataInstance checks if DataInstance is a wrapper type
// Uses Optional trait's unwrap method if available
func (vm *VM) isWrapperDataInstance(data *evaluator.DataInstance) bool {
	// First check builtin Option/Result types for fast path
	switch data.Name {
	case config.SomeCtorName, config.OkCtorName:
		return true
	case config.NoneCtorName, config.FailCtorName:
		return false
	}

	// For user-defined types, check if they implement Optional trait with unwrap
	typeName := data.TypeName
	if typeName == "" {
		typeName = data.Name
	}

	// Type is a wrapper if it has unwrap method (from Optional trait)
	unwrapMethod := vm.LookupTraitMethodAny("Optional", typeName, "unwrap")
	return unwrapMethod != nil
}

// formatError adds line info and stack trace to VM errors
func (vm *VM) formatError(err error) error {
	// Get current line and column from chunk
	line := 0
	col := 0
	if vm.frame != nil && vm.frame.chunk != nil && vm.frame.ip > 0 {
		ip := vm.frame.ip - 1
		if ip < len(vm.frame.chunk.Lines) {
			line = vm.frame.chunk.Lines[ip]
		}
		if ip < len(vm.frame.chunk.Columns) {
			col = vm.frame.chunk.Columns[ip]
		}
	}

	// For builtin errors like panic, extract function name from error message
	errMsg := err.Error()
	calledFn := ""
	if ce, ok := err.(*calleeError); ok {
		calledFn = ce.callee
	} else if strings.HasPrefix(errMsg, "ERROR: ") {
		errMsg = errMsg[7:]
		calledFn = "panic"
	}

	// Build full stack trace by walking all frames
	var stackTrace strings.Builder
	for i := vm.frameCount - 1; i >= 0; i-- {
		frame := &vm.frames[i]
		if frame.chunk == nil {
			continue
		}
		// Get file name for this frame
		file := frame.chunk.File
		if file == "" {
			file = vm.currentFile
		}
		if file == "" {
			file = "<script>"
		}
		// Remove source extension for stack trace display
		file = config.TrimSourceExt(file)
		// Get line for this frame
		frameLine := 0
		if frame.ip > 0 && frame.ip-1 < len(frame.chunk.Lines) {
			frameLine = frame.chunk.Lines[frame.ip-1]
		}
		// Get the function name we're in (use file name for top-level)
		fnName := file
		if frame.closure != nil && frame.closure.Function != nil {
			if frame.closure.Function.Name != "" && frame.closure.Function.Name != "<script>" {
				fnName = frame.closure.Function.Name
			}
		}
		// Get what was called at this level
		called := calledFn
		if i == vm.frameCount-1 && calledFn != "" {
			// Use the callee from error for innermost frame
		} else if i < vm.frameCount-1 {
			// Called the function in frame above
			nextFrame := &vm.frames[i+1]
			if nextFrame.closure != nil && nextFrame.closure.Function != nil {
				called = nextFrame.closure.Function.Name
				if called == "" {
					called = "<anonymous>"
				}
			}
		} else {
			called = fnName
		}

		// For the outermost frame (i == 0), format the path
		displayName := fnName
		if i == 0 && fnName == file {
			// This is the top-level script frame, format the path
			fullPath := frame.chunk.File
			if fullPath == "" {
				fullPath = vm.currentFile
			}
			displayName = formatFilePath(fullPath)
			if displayName == "" {
				displayName = file
			}
		}

		stackTrace.WriteString(fmt.Sprintf("\n  at %s:%d (called %s)", displayName, frameLine, called))
	}

	return fmt.Errorf("runtime error: ERROR at %d:%d: %s\nStack trace:%s", line, col, errMsg, stackTrace.String())
}

// captureHandler safely snapshots a closure for async execution
func (vm *VM) captureHandler(obj evaluator.Object) evaluator.Object {
	switch obj := obj.(type) {
	case *ObjClosure:
		newClosure := &ObjClosure{
			Function: obj.Function,
			Upvalues: make([]*ObjUpvalue, len(obj.Upvalues)),
			Globals:  obj.Globals,
		}
		for i, upvalue := range obj.Upvalues {
			if upvalue.Location >= 0 {
				// Open upvalue: capture current value from THIS VM's stack
				var val evaluator.Object = &evaluator.Nil{}
				if upvalue.Location < len(vm.stack) {
					val = vm.stack[upvalue.Location].AsObject()
				}
				// Recurse to capture nested closures/partials inside the captured value
				val = vm.captureHandler(val)

				// Create CLOSED upvalue
				newClosure.Upvalues[i] = &ObjUpvalue{
					Location: -1,
					Closed:   val,
				}
			} else {
				// Already closed: reuse/copy but also recurse on the closed value
				// The value might be a closure that was closed earlier but still needs deep capture check?
				// If it's already closed, it's safe from stack access of THIS VM.
				// But if it contains another closure with open upvalues to THIS VM's stack?
				// Yes, we should recurse.

				capturedVal := vm.captureHandler(upvalue.Closed)

				newClosure.Upvalues[i] = &ObjUpvalue{
					Location: -1,
					Closed:   capturedVal,
				}
			}
		}
		return newClosure

	case *evaluator.PartialApplication:
		// Handle PartialApplication: recurse on VMClosure and AppliedArgs
		newPartial := &evaluator.PartialApplication{
			Function:        obj.Function,
			Builtin:         obj.Builtin,
			Constructor:     obj.Constructor,
			VMClosure:       nil, // will set below
			AppliedArgs:     make([]evaluator.Object, len(obj.AppliedArgs)),
			RemainingParams: obj.RemainingParams,
		}

		if obj.VMClosure != nil {
			newPartial.VMClosure = vm.captureHandler(obj.VMClosure)
		}

		for i, arg := range obj.AppliedArgs {
			newPartial.AppliedArgs[i] = vm.captureHandler(arg)
		}
		return newPartial

	case *VMComposedFunction:
		// Handle VMComposedFunction: recurse on F and G
		return &VMComposedFunction{
			F: vm.captureHandler(obj.F),
			G: vm.captureHandler(obj.G),
		}

	case *evaluator.ComposedFunction:
		// Handle Evaluator ComposedFunction
		return &evaluator.ComposedFunction{
			F:         vm.captureHandler(obj.F),
			G:         vm.captureHandler(obj.G),
			Evaluator: obj.Evaluator, // Evaluator reference is usually fine (or needs cloning if context specific)
		}
	}

	// Default: return object as is (primitives, builtins, etc. are safe)
	return obj
}

// ForkVM creates a thread-safe copy of the VM for isolated execution
func (vm *VM) ForkVM() *VM {
	newVM := New()
	newVM.SetOutput(vm.out)

	// Copy state
	// For forked VM (e.g. evaluator fallback), we want to share the current globals
	// But thread safety implies we should probably copy the snapshot?
	// VM.globals is *ModuleScope (mutable).
	// If we share *ModuleScope, changes in newVM affect vm.
	// ForkVM is used for evaluator Fork(), which usually implies isolation.
	// However, PersistentMap is immutable.
	// So we should create a NEW ModuleScope with the SAME PersistentMap root.
	newVM.globals = &ModuleScope{Globals: vm.globals.Globals}

	newVM.loader = vm.loader
	newVM.baseDir = vm.baseDir
	newVM.typeAliases = vm.typeAliases
	newVM.typeMap = vm.typeMap

	// Create safe copies of mutable maps
	newVM.traitMethods = vm.traitMethods               // PersistentMap is safe to share
	newVM.builtinTraitMethods = vm.builtinTraitMethods // PersistentMap is safe to share
	newVM.extensionMethods = vm.extensionMethods       // PersistentMap is safe to share
	newVM.traitDefaults = vm.traitDefaults
	newVM.compiledTraitDefaults = vm.compiledTraitDefaults // Read-only at runtime
	newVM.bundle = vm.bundle                               // Shared bundle ref
	newVM.resources = vm.resources                         // Shared resources ref
	newVM.moduleCache = vm.moduleCache
	newVM.currentFile = vm.currentFile

	// Reset stack and frames
	newVM.stack = make([]Value, InitialStackSize)
	newVM.sp = 0
	newVM.frames = make([]CallFrame, InitialFrameCount)

	// Create dummy halt frame so step() knows when to stop
	// Unlike asyncHandler which creates a specific frame for the task fn,
	// ForkVM just prepares a VM. The caller (via vmCallHandler on the new eval)
	// will push the actual frame.
	// But vmCallHandler expects a running VM or at least initialized?
	// vmCallHandler checks frameCount.
	newVM.frameCount = 0

	return newVM
}

// asyncHandler handles async execution by spawning a new VM
func (vm *VM) asyncHandler(fn evaluator.Object, args []evaluator.Object) evaluator.Object {
	// Capture function and arguments to ensure they are safe for the new VM
	// This closes any open upvalues relative to the current VM's stack
	fn = vm.captureHandler(fn)
	for i, arg := range args {
		args[i] = vm.captureHandler(arg)
	}

	task := evaluator.NewTask()

	// Create new VM for isolation
	newVM := New()
	// Inherit output
	newVM.SetOutput(vm.out)
	// Copy state
	// For async, we want isolation. So we snapshot the globals.
	newVM.globals = &ModuleScope{Globals: vm.globals.Globals}

	newVM.loader = vm.loader
	newVM.baseDir = vm.baseDir
	newVM.typeAliases = vm.typeAliases

	// Create safe copies of mutable maps for async execution
	// This prevents data races when JIT compilation writes to these maps
	newVM.traitMethods = vm.traitMethods                   // PersistentMap is safe to share
	newVM.builtinTraitMethods = vm.builtinTraitMethods     // PersistentMap is safe to share
	newVM.extensionMethods = vm.extensionMethods           // PersistentMap is safe to share
	newVM.traitDefaults = vm.traitDefaults                 // Read-only at runtime
	newVM.compiledTraitDefaults = vm.compiledTraitDefaults // Read-only at runtime
	newVM.bundle = vm.bundle                               // Shared bundle ref
	newVM.moduleCache = vm.moduleCache                     // Shared persistent map cache
	newVM.currentFile = vm.currentFile

	// Reset stack and frames for new VM
	newVM.stack = make([]Value, InitialStackSize)
	// Force sp to 0 explicitly again
	newVM.sp = 0
	newVM.frames = make([]CallFrame, InitialFrameCount)

	// Initialize first frame for top-level script
	// It must have OP_HALT so that when the called function returns,
	// the VM halts and returns the result.
	haltChunk := &Chunk{
		Code: []byte{byte(OP_HALT)},
	}

	newVM.frames[0] = CallFrame{
		closure: &ObjClosure{
			Function: &CompiledFunction{
				Name:  "<async>",
				Chunk: haltChunk,
			},
		},
		chunk: haltChunk, // CRITICAL: step() uses frame.chunk directly
		ip:    0,
		base:  0,
	}
	newVM.frameCount = 1
	newVM.frame = &newVM.frames[0]

	go func() {
		evaluator.AcquirePoolSlot()
		defer evaluator.ReleasePoolSlot()

		// Ensure fresh state
		newVM.sp = 0

		// Push function first (callValue expects [fn, args...] on stack)
		if newVM.sp >= len(newVM.stack) {
			newStack := make([]Value, len(newVM.stack)+StackGrowthIncrement)
			copy(newStack, newVM.stack)
			newVM.stack = newStack
		}
		newVM.stack[newVM.sp] = ObjectToValue(fn) // Push function!
		newVM.sp++

		// Push arguments manually
		for _, arg := range args {
			if newVM.sp >= len(newVM.stack) {
				newStack := make([]Value, len(newVM.stack)+StackGrowthIncrement)
				copy(newStack, newVM.stack)
				newVM.stack = newStack
			}
			newVM.stack[newVM.sp] = ObjectToValue(arg)
			newVM.sp++
		}

		// Setup call
		if err := newVM.callValue(ObjectToValue(fn), len(args)); err != nil {
			task.Complete(&evaluator.Error{Message: err.Error()})
			return
		}

		// Execute
		resultVal, err := newVM.execute()
		if err != nil {
			task.Complete(&evaluator.Error{Message: err.Error()})
		} else {
			task.Complete(resultVal.AsObject())
		}
	}()

	return task
}

// RegisterFPTraits registers Functional Programming traits and operators into VM globals
func (vm *VM) RegisterFPTraits() {
	env := evaluator.NewEnvironment()
	e := vm.getEvaluator()
	evaluator.RegisterBasicTraits(e, env)
	evaluator.RegisterFPTraits(e, env)
	evaluator.RegisterStandardTraits(e, env)
	evaluator.RegisterDictionaryGlobals(e, env)

	// Copy symbols from env to globals
	for name, val := range env.GetStore() {
		vm.globals.Globals = vm.globals.Globals.Put(name, val)
	}

	// Copy trait implementations from evaluator to VM
	for traitName, typesMap := range e.ClassImplementations {
		// traitMethods[traitName]
		var typeMap *PersistentMap
		if val := vm.traitMethods.Get(traitName); val != nil {
			typeMap = val.(*PersistentMap)
		} else {
			typeMap = EmptyMap()
		}

		for typeName, methodTableObj := range typesMap {
			if methodTable, ok := methodTableObj.(*evaluator.MethodTable); ok {
				// traitMethods[traitName][typeName]
				var methodMap *PersistentMap
				if existing := typeMap.Get(typeName); existing != nil {
					methodMap = existing.(*PersistentMap)
				} else {
					methodMap = EmptyMap()
				}

				for methodName, method := range methodTable.Methods {
					method := method // Capture loop var
					key := traitName + "." + typeName + "." + methodName

					vm.builtinTraitMethods = vm.builtinTraitMethods.Put(key, &BuiltinClosure{
						Name: methodName,
						Fn: func(args []evaluator.Object) evaluator.Object {
							return e.ApplyFunction(method, args)
						},
					})
				}
				typeMap = typeMap.Put(typeName, methodMap)
			}
		}

		vm.traitMethods = vm.traitMethods.Put(traitName, typeMap)
	}
}

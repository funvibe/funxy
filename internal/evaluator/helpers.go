package evaluator

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/typesystem"
	"strings"
)

// extractListElementType extracts element type name from List<T> annotation
// Returns empty string if not a List type or no type argument
func extractListElementType(t ast.Type) string {
	nt, ok := t.(*ast.NamedType)
	if !ok {
		return ""
	}
	if nt.Name.Value != config.ListTypeName {
		return ""
	}
	if len(nt.Args) == 0 {
		return ""
	}
	// Get the element type name
	if elemType, ok := nt.Args[0].(*ast.NamedType); ok {
		return elemType.Name.Value
	}
	return ""
}

// Helper for method table
type MethodTable struct {
	Methods map[string]Object
}

func (mt *MethodTable) Type() ObjectType             { return "METHOD_TABLE" }
func (mt *MethodTable) Inspect() string              { return "MethodTable" }
func (mt *MethodTable) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "MethodTable"} }
func (mt *MethodTable) Hash() uint32 {
	// Not hashable/useful to hash method tables
	return 0
}

var (
	TRUE  = &Boolean{Value: true}
	FALSE = &Boolean{Value: false}
)

func intPow(n, m int64) int64 {
	if m < 0 {
		return 0
	}
	if m == 0 {
		return 1
	}
	var result int64 = 1
	for i := int64(0); i < m; i++ {
		result *= n
	}
	return result
}

func newError(format string, a ...interface{}) *Error {
	return &Error{Message: fmt.Sprintf(format, a...)}
}

func newErrorWithLocation(line, column int, format string, a ...interface{}) *Error {
	return &Error{
		Message: fmt.Sprintf(format, a...),
		Line:    line,
		Column:  column,
	}
}

// PushCall adds a call frame to the stack
func (e *Evaluator) PushCall(name string, file string, line, column int) {
	e.CallStack = append(e.CallStack, CallFrame{
		Name:   name,
		File:   file,
		Line:   line,
		Column: column,
	})
}

// PopCall removes the top call frame
func (e *Evaluator) PopCall() {
	if len(e.CallStack) > 0 {
		e.CallStack = e.CallStack[:len(e.CallStack)-1]
	}
}

// newErrorWithStack creates an error with the current stack trace
func (e *Evaluator) newErrorWithStack(format string, a ...interface{}) *Error {
	err := &Error{Message: fmt.Sprintf(format, a...)}

	// Copy stack trace
	if len(e.CallStack) > 0 {
		err.StackTrace = make([]StackFrame, len(e.CallStack))
		for i, frame := range e.CallStack {
			err.StackTrace[i] = StackFrame{
				Name:   frame.Name,
				File:   frame.File,
				Line:   frame.Line,
				Column: frame.Column,
			}
		}
	}

	return err
}

func isError(obj Object) bool {
	if obj != nil {
		return obj.Type() == ERROR_OBJ
	}
	return false
}

func unwrapReturnValue(obj Object) Object {
	if returnValue, ok := obj.(*ReturnValue); ok {
		return returnValue.Value
	}
	return obj
}

// ExtractTypeConstructorName extracts the type constructor name from a type.
// e.g., Option<Int> → "Option", List<String> → "List", Result<String, Int> → "Result"
func ExtractTypeConstructorName(t typesystem.Type) string {
	switch typ := t.(type) {
	case typesystem.TCon:
		return typ.Name
	case typesystem.TApp:
		// For TApp, recursively get the constructor
		return ExtractTypeConstructorName(typ.Constructor)
	default:
		return ""
	}
}

// extractTypeNameFromAST extracts the type name from an AST type annotation.
// Used for dispatch when TypeMap doesn't contain the type.
func extractTypeNameFromAST(t ast.Type) string {
	switch typ := t.(type) {
	case *ast.NamedType:
		name := typ.Name.Value
		return name
	default:
		return ""
	}
}

func (e *Evaluator) resolveCanonicalTypeName(t ast.Type, env *Environment) (string, error) {
	switch t := t.(type) {
	case *ast.NamedType:
		name := t.Name.Value
		// For named types (including record aliases and String), use the name as-is
		// BUT if it's an alias for a structural type (Function/Tuple), resolve to the structural name
		// because runtime objects (Function/Tuple) don't carry the alias name.
		if e.TypeAliases != nil {
			if underlying, ok := e.TypeAliases[name]; ok {
				if _, isFunc := underlying.(typesystem.TFunc); isFunc {
					return RUNTIME_TYPE_FUNCTION, nil
				}
				if _, isTuple := underlying.(typesystem.TTuple); isTuple {
					return RUNTIME_TYPE_TUPLE, nil
				}
			}
		}
		return name, nil
	case *ast.TupleType:
		return RUNTIME_TYPE_TUPLE, nil
	case *ast.RecordType:
		return RUNTIME_TYPE_RECORD, nil
	case *ast.FunctionType:
		return RUNTIME_TYPE_FUNCTION, nil
	default:
		return "", fmt.Errorf("unsupported target type for instance: %T", t)
	}
}

func (e *Evaluator) areObjectsEqual(a, b Object) bool {
	if a.Type() != b.Type() {
		return false
	}

	switch a := a.(type) {
	case *Integer:
		return a.Value == b.(*Integer).Value
	case *Float:
		return a.Value == b.(*Float).Value
	case *BigInt:
		return a.Value.Cmp(b.(*BigInt).Value) == 0
	case *Rational:
		return a.Value.Cmp(b.(*Rational).Value) == 0
	case *Boolean:
		return a.Value == b.(*Boolean).Value
	case *Char:
		return a.Value == b.(*Char).Value
	case *Nil:
		return true
	case *List:
		bList := b.(*List)
		if a.len() != bList.len() {
			return false
		}
		for i, el := range a.ToSlice() {
			if !e.areObjectsEqual(el, bList.get(i)) {
				return false
			}
		}
		return true
	case *Tuple:
		bTuple := b.(*Tuple)
		if len(a.Elements) != len(bTuple.Elements) {
			return false
		}
		for i, el := range a.Elements {
			if !e.areObjectsEqual(el, bTuple.Elements[i]) {
				return false
			}
		}
		return true
	case *RecordInstance:
		bRec := b.(*RecordInstance)
		if len(a.Fields) != len(bRec.Fields) {
			return false
		}
		for i, field := range a.Fields {
			bField := bRec.Fields[i]
			if field.Key != bField.Key {
				return false
			}
			if !e.areObjectsEqual(field.Value, bField.Value) {
				return false
			}
		}
		return true
	case *DataInstance:
		bData := b.(*DataInstance)
		if a.Name != bData.Name || a.TypeName != bData.TypeName {
			return false
		}
		if len(a.Fields) != len(bData.Fields) {
			return false
		}
		for i, field := range a.Fields {
			if !e.areObjectsEqual(field, bData.Fields[i]) {
				return false
			}
		}
		return true
	case *TypeObject:
		bType := b.(*TypeObject)
		return a.TypeVal.String() == bType.TypeVal.String()
	case *Bytes:
		return a.equals(b.(*Bytes))
	case *Bits:
		return a.equals(b.(*Bits))
	case *Map:
		return a.equals(b.(*Map), e)
	case *Uuid:
		return a.Value == b.(*Uuid).Value
	}
	return false
}

// GetContentBytes converts String (*List) or Bytes (*Bytes) object to []byte
func GetContentBytes(obj Object) ([]byte, error) {
	if list, ok := obj.(*List); ok {
		return []byte(ListToString(list)), nil
	} else if b, ok := obj.(*Bytes); ok {
		return b.data, nil
	}
	return nil, fmt.Errorf("expected String or Bytes, got %s", obj.Type())
}

func (e *Evaluator) isTruthy(obj Object) bool {
	switch obj := obj.(type) {
	case *Boolean:
		return obj.Value
	default:
		return false
	}
}

func (e *Evaluator) evalExpressions(exps []ast.Expression, env *Environment) []Object {
	var result []Object
	for _, exp := range exps {
		// Handle spread expression: args...
		if spread, ok := exp.(*ast.SpreadExpression); ok {
			val := e.Eval(spread.Expression, env)
			if isError(val) {
				return []Object{val}
			}
			if tuple, ok := val.(*Tuple); ok {
				result = append(result, tuple.Elements...)
			} else if listObj, ok := val.(*List); ok {
				result = append(result, listObj.ToSlice()...)
			} else {
				return []Object{newError("cannot spread non-sequence type: %s", val.Type())}
			}
			continue
		}

		evaluated := e.Eval(exp, env)
		if isError(evaluated) {
			return []Object{evaluated}
		}
		result = append(result, evaluated)
	}
	return result
}

// lookupTraitMethodByName looks up a trait method by name across all traits.
// Returns a ClassMethod wrapper if found, nil otherwise.
// This is used for identifier lookup (e.g., when calling `fmap(...)` directly).
func (e *Evaluator) lookupTraitMethodByName(methodName string) Object {
	// Check if any trait has this method registered (explicit implementation)
	for traitName, typesMap := range e.ClassImplementations {
		// If at least one type has an implementation, return a ClassMethod dispatcher
		for _, methodTableObj := range typesMap {
			if methodTable, ok := methodTableObj.(*MethodTable); ok {
				if _, ok := methodTable.Methods[methodName]; ok {
					// Found! Return a ClassMethod dispatcher
					// Arity -1 means unknown/don't auto-call
					return &ClassMethod{
						Name:      methodName,
						ClassName: traitName,
						Arity:     -1,
					}
				}
			}
		}
	}

	// Check default implementations (e.g. lte in OrderSemantics)
	// This is necessary if no type explicitly overrides the default method yet
	if e.TraitDefaults != nil {
		suffix := "." + methodName
		for key, fnStmt := range e.TraitDefaults {
			if strings.HasSuffix(key, suffix) {
				traitName := strings.TrimSuffix(key, suffix)
				return &ClassMethod{
					Name:      methodName,
					ClassName: traitName,
					Arity:     len(fnStmt.Parameters),
				}
			}
		}
	}

	return nil
}

// matchStringPattern matches a string against a pattern with captures.
// Pattern parts can be literals or capture groups like {name} or {path...} (greedy).
// Returns (matched, captures) where captures maps variable names to captured values.
// MatchStringPattern matches a string against a pattern with captures (exported for VM)
func MatchStringPattern(parts []ast.StringPatternPart, str string) (bool, map[string]string) {
	captures := make(map[string]string)
	pos := 0

	for i, part := range parts {
		if !part.IsCapture {
			// Literal part - must match exactly
			if !strings.HasPrefix(str[pos:], part.Value) {
				return false, nil
			}
			pos += len(part.Value)
		} else {
			// Capture part
			if part.Greedy {
				// Greedy: capture everything until end or next literal
				if i+1 < len(parts) && !parts[i+1].IsCapture {
					// Find next literal
					nextLit := parts[i+1].Value
					idx := strings.Index(str[pos:], nextLit)
					if idx == -1 {
						return false, nil
					}
					captures[part.Value] = str[pos : pos+idx]
					pos += idx
				} else {
					// No next literal - capture rest of string
					captures[part.Value] = str[pos:]
					pos = len(str)
				}
			} else {
				// Non-greedy: capture until next '/' or end
				end := pos
				for end < len(str) && str[end] != '/' {
					end++
				}
				// Also stop at next literal if present
				if i+1 < len(parts) && !parts[i+1].IsCapture {
					nextLit := parts[i+1].Value
					idx := strings.Index(str[pos:], nextLit)
					if idx != -1 && pos+idx < end {
						end = pos + idx
					}
				}
				captures[part.Value] = str[pos:end]
				pos = end
			}
		}
	}

	// All parts matched and consumed entire string (or at least matched all parts)
	return pos == len(str), captures
}

// checkArgsMatch checks if the arguments match the function parameters
// This is used to disambiguate trait dispatch (Context vs Args)
func (e *Evaluator) checkArgsMatch(fn Object, args []Object) bool {
	switch f := fn.(type) {
	case *Function:
		// Skip witness arguments for validation ONLY if they are actually provided
		witnessCount := len(f.WitnessParams)
		actualArgs := args

		hasWitnesses := false
		if witnessCount > 0 && len(args) >= witnessCount {
			hasWitnesses = true
			for i := 0; i < witnessCount; i++ {
				if _, ok := args[i].(*Dictionary); !ok {
					hasWitnesses = false
					break
				}
			}
		}

		if hasWitnesses {
			actualArgs = args[witnessCount:]
		}

		paramCount := len(f.Parameters)
		isVariadic := false
		if paramCount > 0 && f.Parameters[paramCount-1].IsVariadic {
			isVariadic = true
			paramCount--
		}

		for i, arg := range actualArgs {
			if i >= paramCount {
				if isVariadic {
					// Variadic args match generic list usually, hard to check strict type without detailed info
					return true
				}
				return false // Too many args
			}

			param := f.Parameters[i]
			if !e.fuzzyMatchAstType(arg, param.Type) {
				return false
			}
		}
		return true

	case *Builtin:
		if f.TypeInfo == nil {
			return true // Optimistic
		}
		if tFunc, ok := f.TypeInfo.(typesystem.TFunc); ok {
			params := tFunc.Params
			isVariadic := tFunc.IsVariadic

			for i, arg := range args {
				if i >= len(params) {
					if isVariadic {
						return true
					}
					return false
				}
				if !e.fuzzyMatchSystemType(arg, params[i]) {
					return false
				}
			}
			return true
		}
		return true

	default:
		return true // Optimistic
	}
}

func (f *Function) countDefaults() int {
	count := 0
	for _, p := range f.Parameters {
		if p.Default != nil {
			count++
		}
	}
	return count
}

// fuzzyMatchAstType checks if val matches type, being optimistic about generics
func (e *Evaluator) fuzzyMatchAstType(val Object, t ast.Type) bool {
	// Nil type means Any (untyped parameter) - always match
	if t == nil {
		return true
	}

	// If it's a NamedType and not standard, assume generic and match
	if nt, ok := t.(*ast.NamedType); ok {
		if !isStandardType(nt.Name.Value) {
			return true
		}
	}
	// Use strict check for standard types
	return e.matchesType(val, t)
}

// fuzzyMatchSystemType checks if val matches system type
func (e *Evaluator) fuzzyMatchSystemType(val Object, t typesystem.Type) bool {
	switch typ := t.(type) {
	case typesystem.TCon:
		if !isStandardType(typ.Name) {
			return true
		}
		// Check strict runtime type match
		runtimeType := getRuntimeTypeName(val)
		if runtimeType == typ.Name {
			return true
		}
		// Check aliases systemically
		if e.TypeAliases != nil {
			if alias, ok := e.TypeAliases[typ.Name]; ok {
				aliasName := ExtractTypeConstructorName(alias)
				if aliasName == runtimeType {
					return true
				}
			}
		}
		return false
	case typesystem.TVar:
		return true // Generic matches all
	default:
		return true // Complex types - optimistic
	}
}

func isStandardType(name string) bool {
	switch name {
	case "Int", "Float", "Bool", "Char", "String", "List", "Map", "Bytes", "Bits",
		"Option", "Result", "Nil", "BigInt", "Rational", "Tuple":
		return true
	}
	return false
}

// normalizeTypeName normalizes type names for runtime comparison (e.g. String -> List)
func normalizeTypeName(name string) string {
	// We want to preserve String distinct from List for trait dispatch
	return name
}

// ModuleMemberFallbackName maps a short member name to a verbose stdlib name.
// Example: moduleName="string", member="toUpper" -> "stringToUpper".
// ModuleMemberFallbackName moved to utils to avoid duplication.

// Helper to extract return type name from Function/Builtin
func (e *Evaluator) getReturnTypeName(obj Object) string {
	switch fn := obj.(type) {
	case *Function:
		if fn.ReturnType != nil {
			return extractTypeNameFromAST(fn.ReturnType)
		}
	case *Builtin:
		if fn.TypeInfo != nil {
			if tFunc, ok := fn.TypeInfo.(typesystem.TFunc); ok {
				return ExtractTypeConstructorName(tFunc.ReturnType)
			}
		}
	case *BoundMethod:
		return e.getReturnTypeName(fn.Function)
	}
	return ""
}

// ============================================================================
// Helpers for Dictionary Passing
// ============================================================================

// FindMethodInDictionary searches for a method in a dictionary or its supers.
// Returns nil if not found.
func FindMethodInDictionary(d *Dictionary, methodName string) Object {
	// Check if this dictionary belongs to a trait that has the method
	if methods, ok := TraitMethods[d.TraitName]; ok {
		for i, name := range methods {
			if name == methodName {
				// Found the method name in this trait definition
				if i < len(d.Methods) {
					return d.Methods[i]
				}
				return nil // Should not happen if dictionary is well-formed
			}
		}
	}

	// Recurse into supers
	for _, super := range d.Supers {
		if m := FindMethodInDictionary(super, methodName); m != nil {
			return m
		}
	}
	return nil
}

// extractWitnessMethod tries to extract a method from an explicit witness dictionary
// passed as the first argument.
// Returns: (method, remainingArgs, found)
func extractWitnessMethod(args []Object, methodName string) (Object, []Object, bool) {
	if len(args) < 1 {
		return nil, args, false
	}

	dict, ok := args[0].(*Dictionary)
	if !ok {
		return nil, args, false
	}

	// If it's a placeholder dictionary (from Analyzer), strip it even if method not found
	// This ensures fall-back to dynamic lookup in Tree mode
	// Also strip empty trait name which might happen in some testing scenarios
	if dict.TraitName == "$placeholder" || dict.TraitName == "" {
		return nil, args[1:], true
	}

	if method := FindMethodInDictionary(dict, methodName); method != nil {
		// Found it! Consume the dictionary argument
		return method, args[1:], true
	}

	// Method not found in this dictionary.
	// We do NOT consume the dictionary here, leaving it for subsequent lookups or strategies.
	// Note: Callers might iterate through multiple dictionaries if they expect a specific witness.
	return nil, args, false
}

// resolveTypeFromEnv resolves type variables in a type using the environment.
// This supports explicit witness passing where generic types are resolved to concrete types
// passed as arguments (or inferred and stored in Env).
func (e *Evaluator) resolveTypeFromEnv(t typesystem.Type, env *Environment) typesystem.Type {
	if env == nil {
		return t
	}
	switch typ := t.(type) {
	case typesystem.TVar:
		// Look for type variable in env
		// First check prefixed name (from generic instantiation via $typevar_ prefix).
		// This is set in ApplyFunction when a generic function is called.
		if val, ok := env.Get("$typevar_" + typ.Name); ok {
			if typeObj, ok := val.(*TypeObject); ok {
				if typeObj.Alias != "" {
					return typesystem.TCon{Name: typeObj.Alias}
				}
				return typeObj.TypeVal
			}
		}
		// Fallback: check regular name (for type declarations like Int, String, etc.)
		if val, ok := env.Get(typ.Name); ok {
			if typeObj, ok := val.(*TypeObject); ok {
				if typeObj.Alias != "" {
					return typesystem.TCon{Name: typeObj.Alias}
				}
				return typeObj.TypeVal
			}
		}
		return typ
	case typesystem.TApp:
		newConstructor := e.resolveTypeFromEnv(typ.Constructor, env)
		newArgs := make([]typesystem.Type, len(typ.Args))
		for i, arg := range typ.Args {
			newArgs[i] = e.resolveTypeFromEnv(arg, env)
		}
		// Always return new TApp as comparison of TApp (containing slice) causes panic
		return typesystem.TApp{Constructor: newConstructor, Args: newArgs}
	case typesystem.TFunc:
		newParams := make([]typesystem.Type, len(typ.Params))
		for i, p := range typ.Params {
			newParams[i] = e.resolveTypeFromEnv(p, env)
		}
		newRet := e.resolveTypeFromEnv(typ.ReturnType, env)
		return typesystem.TFunc{
			Params:       newParams,
			ReturnType:   newRet,
			IsVariadic:   typ.IsVariadic,
			DefaultCount: typ.DefaultCount,
		}
	// TTuple, TRecord, etc. should be handled too but TApp/TVar are most important for Witnesses.
	// Adding TTuple/TRecord for completeness.
	case typesystem.TTuple:
		newElements := make([]typesystem.Type, len(typ.Elements))
		for i, el := range typ.Elements {
			newElements[i] = e.resolveTypeFromEnv(el, env)
		}
		return typesystem.TTuple{Elements: newElements}
	default:
		return typ
	}
}

func getKeys(m map[string]Object) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (e *Evaluator) nativeBoolToBooleanObject(input bool) *Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

// getTypeName returns a human-readable type name for debugging
func getTypeName(obj Object) string {
	switch v := obj.(type) {
	case *Integer:
		return "Int"
	case *Float:
		return "Float"
	case *Boolean:
		return "Bool"
	case *Char:
		return "Char"
	case *Nil:
		return "Nil"
	case *List:
		if v.len() == 0 {
			return "List<_>"
		}
		// Try to infer element type from first element
		first := v.get(0)
		return "List<" + getTypeName(first) + ">"
	case *Map:
		return "Map<_, _>"
	case *Tuple:
		if len(v.Elements) == 0 {
			return "()"
		}
		types := make([]string, len(v.Elements))
		for i, el := range v.Elements {
			types[i] = getTypeName(el)
		}
		return "(" + strings.Join(types, ", ") + ")"
	case *Function:
		return "Function"
	case *Builtin:
		return "Builtin"
	case *RecordInstance:
		return v.TypeName
	case *DataInstance:
		return v.TypeName
	case *BigInt:
		return "BigInt"
	case *Rational:
		return "Rational"
	case *Bytes:
		return "Bytes"
	case *Bits:
		return "Bits"
	case *Uuid:
		return "Uuid"
	default:
		return string(obj.Type())
	}
}

package ext

import (
	"fmt"
	"sort"
	"strings"
	"text/template"
	"unicode"
)

// CodeGenerator produces Go source code for ext/* bindings.
type CodeGenerator struct {
	// funxyModulePath is the Go import path of the Funxy project
	// (e.g. "parser" for local dev, "github.com/funvibe/funxy" for published).
	funxyModulePath string
}

// NewCodeGenerator creates a new code generator.
func NewCodeGenerator(funxyModulePath string) *CodeGenerator {
	return &CodeGenerator{funxyModulePath: funxyModulePath}
}

// GeneratedFile represents a generated Go source file.
type GeneratedFile struct {
	// Filename is the relative path within the generated project (e.g. "ext_redis.go").
	Filename string

	// Content is the full Go source code.
	Content string
}

// Generate produces all Go source files for the given inspection result.
// Returns the binding files plus a main.go that registers everything.
func (cg *CodeGenerator) Generate(result *InspectResult) ([]GeneratedFile, error) {
	var files []GeneratedFile

	// Group bindings by alias (ext module name)
	groups := cg.groupBindings(result.Bindings)

	for alias, bindings := range groups {
		file, err := cg.generateBindingFile(alias, bindings)
		if err != nil {
			return nil, fmt.Errorf("generating bindings for %s: %w", alias, err)
		}
		files = append(files, file)
	}

	// Sort files for deterministic output
	sort.Slice(files, func(i, j int) bool {
		return files[i].Filename < files[j].Filename
	})

	// Generate main.go
	mainFile, err := cg.generateMainFile(result)
	if err != nil {
		return nil, fmt.Errorf("generating main.go: %w", err)
	}
	files = append(files, mainFile)

	return files, nil
}

// groupBindings groups bindings by their ext module name (derived from the parent dep).
func (cg *CodeGenerator) groupBindings(bindings []*ResolvedBinding) map[string][]*ResolvedBinding {
	groups := make(map[string][]*ResolvedBinding)
	for _, b := range bindings {
		modName := b.Dep.ExtModuleName()
		groups[modName] = append(groups[modName], b)
	}
	return groups
}

// generateBindingFile generates a Go source file for a single ext module.
func (cg *CodeGenerator) generateBindingFile(alias string, bindings []*ResolvedBinding) (GeneratedFile, error) {
	ctx := &bindingFileContext{
		Alias:           alias,
		FunxyModulePath: cg.funxyModulePath,
		Imports:         make(map[string]string), // path → local alias
	}

	// Always need pkg/ext (aliased as ext)
	ctx.Imports[cg.funxyModulePath+"/pkg/ext"] = "ext"

	// Collect all Go package imports needed
	for _, b := range bindings {
		ctx.addImport(b.GoPackagePath)

		if b.TypeBinding != nil {
			for _, m := range b.TypeBinding.Methods {
				cg.collectTypeImports(ctx, m.Signature)
			}
			for _, f := range b.TypeBinding.Fields {
				// Only add imports for fields that will actually be used in generated code.
				// Field getters use toFunxy() which handles any type via interface{},
				// so we only need imports for constructable fields (used in constructors).
				if isConstructableFieldType(f.Type) {
					cg.collectRefImports(ctx, f.Type)
				}
			}
		}
		if b.FuncBinding != nil {
			cg.collectTypeImports(ctx, b.FuncBinding.Signature)
		}
	}

	// Check if context is needed (for skip_context)
	for _, b := range bindings {
		if b.Spec.SkipContext {
			ctx.NeedContext = true
			ctx.addImport("context")
			break
		}
	}

	// fmt is always imported via the template (for newErr etc.)
	// No need to add it explicitly — it's in the template header

	// Generate function wrappers
	for _, b := range bindings {
		if b.TypeBinding != nil {
			ctx.generateTypeWrappers(b)
			ctx.generateFieldGetters(b)
			if b.Spec.Constructor && b.TypeBinding.IsStruct {
				ctx.generateConstructor(b)
			}
		}
		if b.FuncBinding != nil {
			ctx.generateFuncWrapper(b)
		}
		if b.ConstBinding != nil {
			ctx.generateConstValue(b)
		}
	}

	// Render the file
	content, err := ctx.render()
	if err != nil {
		return GeneratedFile{}, err
	}

	return GeneratedFile{
		Filename: "ext_" + alias + ".go",
		Content:  content,
	}, nil
}

// bindingFileContext holds state while generating a single binding file.
type bindingFileContext struct {
	Alias           string
	FunxyModulePath string
	Imports         map[string]string // path → local alias (empty = default)
	NeedContext     bool
	NeedFmt         bool
	Functions       []generatedFunc
	Values          []generatedValue // Constants and other non-function values
}

// generatedFunc represents a single generated wrapper function.
type generatedFunc struct {
	// FunxyName is the function name in Funxy (e.g. "redisGet").
	FunxyName string
	// GoCode is the Go function body source code.
	GoCode string
}

// generatedValue represents a constant or variable value registered directly.
type generatedValue struct {
	// FunxyName is the value name in Funxy (e.g. "httpStatusOK").
	FunxyName string
	// GoExpr is a Go expression that produces an evaluator.Object.
	GoExpr string
}

// addImport adds a Go import path, assigning a unique local alias if needed.
func (ctx *bindingFileContext) addImport(pkgPath string) {
	if _, exists := ctx.Imports[pkgPath]; exists {
		return
	}
	ctx.Imports[pkgPath] = ImportAlias(pkgPath)
}

// collectTypeImports walks a FuncSignature and adds required imports.
func (cg *CodeGenerator) collectTypeImports(ctx *bindingFileContext, sig *FuncSignature) {
	for _, p := range sig.Params {
		cg.collectRefImports(ctx, p.Type)
	}
	for _, r := range sig.Results {
		cg.collectRefImports(ctx, r.Type)
	}
}

// collectRefImports adds imports needed for a GoTypeRef.
func (cg *CodeGenerator) collectRefImports(ctx *bindingFileContext, ref GoTypeRef) {
	if ref.PkgPath != "" {
		ctx.addImport(ref.PkgPath)
	}
	if ref.ElemType != nil {
		cg.collectRefImports(ctx, *ref.ElemType)
	}
	if ref.KeyType != nil {
		cg.collectRefImports(ctx, *ref.KeyType)
	}
	if ref.FuncSig != nil {
		cg.collectTypeImports(ctx, ref.FuncSig)
	}
}

// generateTypeWrappers generates wrapper functions for all methods of a type binding.
func (ctx *bindingFileContext) generateTypeWrappers(b *ResolvedBinding) {
	tb := b.TypeBinding
	pkgAlias := ImportAlias(b.GoPackagePath)

	for _, method := range tb.Methods {
		var buf strings.Builder

		// Build the wrapper function
		sig := method.Signature
		skipCtx := b.Spec.SkipContext && sig.HasContextParam
		chainResult := b.Spec.ChainResult
		if chainResult == "" {
			chainResult = method.AutoChainMethod // auto-detected from return type
		}
		errorToResult := b.Spec.ErrorToResult

		// Calculate effective parameter count (visible to Funxy)
		funxyParamCount := len(sig.Params)
		if skipCtx {
			funxyParamCount-- // context is hidden
		}
		// +1 for self (the object we're calling on)
		totalFunxyArgs := funxyParamCount + 1

		// Arg count validation (considering variadic)
		if sig.IsVariadic {
			buf.WriteString(fmt.Sprintf("if len(args) < %d {\n", totalFunxyArgs-1))
			buf.WriteString(fmt.Sprintf("\treturn newErr(\"%%s: expected at least %%d arguments, got %%d\", %q, %d, len(args))\n",
				method.FunxyName, totalFunxyArgs-1))
			buf.WriteString("}\n")
		} else {
			buf.WriteString(fmt.Sprintf("if len(args) != %d {\n", totalFunxyArgs))
			buf.WriteString(fmt.Sprintf("\treturn newErr(\"%%s: expected %%d arguments, got %%d\", %q, %d, len(args))\n",
				method.FunxyName, totalFunxyArgs))
			buf.WriteString("}\n")
		}

		// Extract self from first argument
		buf.WriteString("// Extract receiver\n")
		buf.WriteString("selfHost, ok := args[0].(*ext.HostObject)\n")
		buf.WriteString("if !ok {\n")
		buf.WriteString(fmt.Sprintf("\treturn newErr(\"%s: first argument must be a HostObject, got %%s\", args[0].Type())\n", method.FunxyName))
		buf.WriteString("}\n")

		// Type assertion for the receiver — try both pointer and value types.
		// Go methods with value receivers can be called on both T and *T.
		goTypeName := qualifiedGoTypeName(pkgAlias, tb.GoName, tb.TypeArgs)
		ptrType := "*" + goTypeName
		valType := goTypeName
		buf.WriteString(fmt.Sprintf("selfPtr, okPtr := selfHost.Value.(%s)\n", ptrType))
		buf.WriteString(fmt.Sprintf("selfVal, okVal := selfHost.Value.(%s)\n", valType))
		buf.WriteString("if !okPtr && !okVal {\n")
		buf.WriteString(fmt.Sprintf("\treturn newErr(\"%s: receiver type mismatch, expected %s or %s, got %%T\", selfHost.Value)\n", method.FunxyName, ptrType, valType))
		buf.WriteString("}\n")
		buf.WriteString("var self *" + goTypeName + "\n")
		buf.WriteString("if okPtr {\n")
		buf.WriteString("\tself = selfPtr\n")
		buf.WriteString("} else {\n")
		buf.WriteString("\tself = &selfVal\n")
		buf.WriteString("}\n")

		// Convert Funxy args to Go args
		argIdx := 1 // Start after self
		goCallArgs := []string{}

		if skipCtx {
			goCallArgs = append(goCallArgs, "context.Background()")
		}

		for i, param := range sig.Params {
			if skipCtx && i == 0 && sig.HasContextParam {
				continue // Skip context parameter
			}

			if param.IsVariadic {
				// Variadic: collect remaining args
				variadicVarName := fmt.Sprintf("varArgs%d", i)
				elemGoType := qualifiedGoType(param.Type, b.GoPackagePath)
				buf.WriteString("// Convert variadic arguments\n")
				buf.WriteString(fmt.Sprintf("var %s []%s\n", variadicVarName, elemGoType))
				buf.WriteString(fmt.Sprintf("for vi := %d; vi < len(args); vi++ {\n", argIdx))

				needsAssertion := param.Type.Kind == GoTypeFunc || param.Type.Kind == GoTypeInterface
				if needsAssertion {
					buf.WriteString("\tvHost, ok := args[vi].(*ext.HostObject)\n")
					buf.WriteString("\tif !ok {\n")
					buf.WriteString(fmt.Sprintf("\t\treturn newErr(\"%s: variadic arg %%d: expected HostObject, got %%s\", vi-%d, args[vi].Type())\n", method.FunxyName, argIdx))
					buf.WriteString("\t}\n")
					buf.WriteString(fmt.Sprintf("\tvTyped, ok := vHost.Value.(%s)\n", elemGoType))
					buf.WriteString("\tif !ok {\n")
					buf.WriteString(fmt.Sprintf("\t\treturn newErr(\"%s: variadic arg %%d: expected %s, got %%T\", vi-%d, vHost.Value)\n", method.FunxyName, elemGoType, argIdx))
					buf.WriteString("\t}\n")
					buf.WriteString(fmt.Sprintf("\t%s = append(%s, vTyped)\n", variadicVarName, variadicVarName))
				} else {
					convExpr := ctx.genFromFunxy("args[vi]", param.Type, b.GoPackagePath)
					buf.WriteString(fmt.Sprintf("\tvVal, vErr := %s\n", convExpr))
					buf.WriteString("\tif vErr != nil {\n")
					buf.WriteString(fmt.Sprintf("\t\treturn newErr(\"%s: variadic arg %%d: %%v\", vi-%d, vErr)\n", method.FunxyName, argIdx))
					buf.WriteString("\t}\n")
					buf.WriteString(fmt.Sprintf("\t%s = append(%s, vVal)\n", variadicVarName, variadicVarName))
				}

				buf.WriteString("}\n")
				goCallArgs = append(goCallArgs, variadicVarName+"...")
			} else if param.Type.Kind == GoTypeFunc && param.Type.FuncSig != nil {
				// Callback parameter: wrap Funxy closure into Go func
				argVarName := fmt.Sprintf("arg%d", i)
				ctx.genCallbackConversion(&buf, argVarName, fmt.Sprintf("args[%d]", argIdx), param.Type, b.GoPackagePath)
				goCallArgs = append(goCallArgs, argVarName)
				argIdx++
			} else {
				argVarName := fmt.Sprintf("arg%d", i)
				convExpr := ctx.genFromFunxy(fmt.Sprintf("args[%d]", argIdx), param.Type, b.GoPackagePath)
				buf.WriteString(fmt.Sprintf("%s, err%d := %s\n", argVarName, i, convExpr))
				buf.WriteString(fmt.Sprintf("if err%d != nil {\n", i))
				buf.WriteString(fmt.Sprintf("\treturn newErr(\"%s: arg %d: %%v\", err%d)\n", method.FunxyName, i, i))
				buf.WriteString("}\n")
				goCallArgs = append(goCallArgs, argVarName)
				argIdx++
			}
		}

		// Build the method call
		callExpr := fmt.Sprintf("self.%s(%s)", method.GoName, strings.Join(goCallArgs, ", "))

		// Handle chain_result — only apply to methods that return a single
		// non-error value (e.g. *StatusCmd, *StringCmd). Methods that already
		// return error or (T, error) are handled directly without chaining.
		useChain := chainResult != "" && len(sig.Results) == 1 && !sig.HasErrorReturn
		if useChain {
			callExpr = fmt.Sprintf("%s.%s()", callExpr, chainResult)
			// When chaining, the chained method's return type is what we work with
			// The chain method typically returns (T, error)
			errorToResult = true // Chain result implies error handling
		}

		// Handle return values
		ctx.genReturnHandling(&buf, method.FunxyName, callExpr, sig, errorToResult, useChain, b.GoPackagePath)

		ctx.Functions = append(ctx.Functions, generatedFunc{
			FunxyName: method.FunxyName,
			GoCode:    buf.String(),
		})
	}
}

// generateFuncWrapper generates a wrapper for a standalone function binding.
func (ctx *bindingFileContext) generateFuncWrapper(b *ResolvedBinding) {
	fb := b.FuncBinding
	pkgAlias := ImportAlias(b.GoPackagePath)

	var buf strings.Builder
	sig := fb.Signature
	skipCtx := b.Spec.SkipContext && sig.HasContextParam
	errorToResult := b.Spec.ErrorToResult

	// Calculate effective parameter count
	funxyParamCount := len(sig.Params)
	if skipCtx {
		funxyParamCount--
	}

	// Arg count validation
	if sig.IsVariadic {
		buf.WriteString(fmt.Sprintf("if len(args) < %d {\n", funxyParamCount-1))
		buf.WriteString(fmt.Sprintf("\treturn newErr(\"%%s: expected at least %%d arguments, got %%d\", %q, %d, len(args))\n",
			b.Spec.As, funxyParamCount-1))
		buf.WriteString("}\n")
	} else {
		buf.WriteString(fmt.Sprintf("if len(args) != %d {\n", funxyParamCount))
		buf.WriteString(fmt.Sprintf("\treturn newErr(\"%%s: expected %%d arguments, got %%d\", %q, %d, len(args))\n",
			b.Spec.As, funxyParamCount))
		buf.WriteString("}\n")
	}

	// Convert Funxy args to Go args
	argIdx := 0
	goCallArgs := []string{}

	if skipCtx {
		goCallArgs = append(goCallArgs, "context.Background()")
	}

	for i, param := range sig.Params {
		if skipCtx && i == 0 && sig.HasContextParam {
			continue
		}

		if param.IsVariadic {
			variadicVarName := fmt.Sprintf("varArgs%d", i)
			elemGoType := qualifiedGoType(param.Type, b.GoPackagePath)
			buf.WriteString(fmt.Sprintf("var %s []%s\n", variadicVarName, elemGoType))
			buf.WriteString(fmt.Sprintf("for vi := %d; vi < len(args); vi++ {\n", argIdx))

			needsAssertion := param.Type.Kind == GoTypeFunc || param.Type.Kind == GoTypeInterface
			if needsAssertion {
				// Function/interface types: extract from HostObject and type-assert
				buf.WriteString("\tvHost, ok := args[vi].(*ext.HostObject)\n")
				buf.WriteString("\tif !ok {\n")
				buf.WriteString(fmt.Sprintf("\t\treturn newErr(\"%s: variadic arg %%d: expected HostObject, got %%s\", vi-%d, args[vi].Type())\n", b.Spec.As, argIdx))
				buf.WriteString("\t}\n")
				buf.WriteString(fmt.Sprintf("\tvTyped, ok := vHost.Value.(%s)\n", elemGoType))
				buf.WriteString("\tif !ok {\n")
				buf.WriteString(fmt.Sprintf("\t\treturn newErr(\"%s: variadic arg %%d: expected %s, got %%T\", vi-%d, vHost.Value)\n", b.Spec.As, elemGoType, argIdx))
				buf.WriteString("\t}\n")
				buf.WriteString(fmt.Sprintf("\t%s = append(%s, vTyped)\n", variadicVarName, variadicVarName))
			} else {
				convExpr := ctx.genFromFunxy("args[vi]", param.Type, b.GoPackagePath)
				buf.WriteString(fmt.Sprintf("\tvVal, vErr := %s\n", convExpr))
				buf.WriteString("\tif vErr != nil {\n")
				buf.WriteString(fmt.Sprintf("\t\treturn newErr(\"%s: variadic arg %%d: %%v\", vi-%d, vErr)\n", b.Spec.As, argIdx))
				buf.WriteString("\t}\n")
				buf.WriteString(fmt.Sprintf("\t%s = append(%s, vVal)\n", variadicVarName, variadicVarName))
			}

			buf.WriteString("}\n")
			goCallArgs = append(goCallArgs, variadicVarName+"...")
		} else if param.Type.Kind == GoTypeFunc && param.Type.FuncSig != nil {
			// Callback parameter: wrap Funxy closure into Go func
			argVarName := fmt.Sprintf("arg%d", i)
			ctx.genCallbackConversion(&buf, argVarName, fmt.Sprintf("args[%d]", argIdx), param.Type, b.GoPackagePath)
			goCallArgs = append(goCallArgs, argVarName)
			argIdx++
		} else {
			argVarName := fmt.Sprintf("arg%d", i)
			convExpr := ctx.genFromFunxy(fmt.Sprintf("args[%d]", argIdx), param.Type, b.GoPackagePath)
			buf.WriteString(fmt.Sprintf("%s, err%d := %s\n", argVarName, i, convExpr))
			buf.WriteString(fmt.Sprintf("if err%d != nil {\n", i))
			buf.WriteString(fmt.Sprintf("\treturn newErr(\"%s: arg %d: %%v\", err%d)\n", b.Spec.As, i, i))
			buf.WriteString("}\n")
			goCallArgs = append(goCallArgs, argVarName)
			argIdx++
		}
	}

	// Build the function call (with type args for generics)
	typeArgStr := ""
	if len(fb.TypeArgs) > 0 {
		typeArgStr = "[" + strings.Join(fb.TypeArgs, ", ") + "]"
	}
	callExpr := fmt.Sprintf("%s.%s%s(%s)", pkgAlias, fb.GoName, typeArgStr, strings.Join(goCallArgs, ", "))

	// Handle return values
	ctx.genReturnHandling(&buf, b.Spec.As, callExpr, sig, errorToResult, false, b.GoPackagePath)

	ctx.Functions = append(ctx.Functions, generatedFunc{
		FunxyName: b.Spec.As,
		GoCode:    buf.String(),
	})
}

// generateFieldGetters generates getter functions for struct fields of a type binding.
// For a type with as="redis" and field "Addr", generates "redisAddr(obj) -> String".
func (ctx *bindingFileContext) generateFieldGetters(b *ResolvedBinding) {
	tb := b.TypeBinding
	if !tb.IsStruct || len(tb.Fields) == 0 {
		return
	}

	pkgAlias := ImportAlias(b.GoPackagePath)

	for _, field := range tb.Fields {
		var buf strings.Builder
		funxyName := b.Spec.As + ucFirst(field.GoName)

		// Arg count: exactly 1 (the object)
		buf.WriteString(fmt.Sprintf("if len(args) != 1 {\n"))
		buf.WriteString(fmt.Sprintf("\treturn newErr(\"%%s: expected 1 argument, got %%d\", %q, len(args))\n", funxyName))
		buf.WriteString("}\n")

		// Extract self
		buf.WriteString("selfHost, ok := args[0].(*ext.HostObject)\n")
		buf.WriteString("if !ok {\n")
		buf.WriteString(fmt.Sprintf("\treturn newErr(\"%s: argument must be a HostObject, got %%s\", args[0].Type())\n", funxyName))
		buf.WriteString("}\n")

		// Type assertion — try both pointer and value types, with nil check for pointers
		goTypeName := qualifiedGoTypeName(pkgAlias, tb.GoName, tb.TypeArgs)
		ptrType := "*" + goTypeName
		valType := goTypeName
		buf.WriteString(fmt.Sprintf("if selfPtr, ok := selfHost.Value.(%s); ok {\n", ptrType))
		buf.WriteString(fmt.Sprintf("\tif selfPtr == nil {\n"))
		buf.WriteString(fmt.Sprintf("\t\treturn newErr(\"%s: nil pointer dereference on %s\")\n", funxyName, ptrType))
		buf.WriteString(fmt.Sprintf("\t}\n"))
		buf.WriteString(fmt.Sprintf("\treturn toFunxy(selfPtr.%s)\n", field.GoName))
		buf.WriteString("}\n")
		buf.WriteString(fmt.Sprintf("if selfVal, ok := selfHost.Value.(%s); ok {\n", valType))
		buf.WriteString(fmt.Sprintf("\treturn toFunxy(selfVal.%s)\n", field.GoName))
		buf.WriteString("}\n")
		buf.WriteString(fmt.Sprintf("return newErr(\"%s: expected %s or %s, got %%T\", selfHost.Value)\n", funxyName, ptrType, valType))

		ctx.Functions = append(ctx.Functions, generatedFunc{
			FunxyName: funxyName,
			GoCode:    buf.String(),
		})
	}
}

// generateConstructor generates a constructor function that creates a Go struct
// from a Funxy record. The function name is the bare `as` prefix.
// Example: type Options, as: opts → opts({ addr: "localhost", db: 0 }) → *redis.Options
func (ctx *bindingFileContext) generateConstructor(b *ResolvedBinding) {
	tb := b.TypeBinding
	pkgAlias := ImportAlias(b.GoPackagePath)
	funxyName := b.Spec.As // bare prefix is the constructor name

	var buf strings.Builder

	// Accept exactly 1 argument: a record
	buf.WriteString("if len(args) != 1 {\n")
	buf.WriteString(fmt.Sprintf("\treturn newErr(\"%%s: expected 1 argument (record), got %%d\", %q, len(args))\n", funxyName))
	buf.WriteString("}\n")

	// Extract the record
	buf.WriteString("rec, ok := args[0].(*ext.RecordInstance)\n")
	buf.WriteString("if !ok {\n")
	buf.WriteString(fmt.Sprintf("\treturn newErr(\"%s: expected a record, got %%s\", args[0].Type())\n", funxyName))
	buf.WriteString("}\n")

	// Create the Go struct
	goTypeName := qualifiedGoTypeName(pkgAlias, tb.GoName, tb.TypeArgs)
	buf.WriteString(fmt.Sprintf("var obj %s\n", goTypeName))

	// For each exported field, try to extract from the record.
	// Skip fields with types that can't be constructed from Funxy
	// (functions, interfaces, channels, etc.)
	for _, field := range tb.Fields {
		if !isConstructableFieldType(field.Type) {
			continue
		}

		valVar := "fv_" + field.FunxyName
		buf.WriteString(fmt.Sprintf("if %s := rec.Get(%q); %s != nil {\n", valVar, field.FunxyName, valVar))

		if field.Type.Kind == GoTypePtr && field.Type.ElemType != nil && field.Type.ElemType.Kind == GoTypeBasic {
			// Pointer to basic type (*string, *int32, *bool, etc.) — convert basic, then take address.
			// This is very common in AWS SDK and similar APIs.
			elemConv := ctx.genFromFunxy(valVar, *field.Type.ElemType, b.GoPackagePath)
			goElemType := qualifiedGoType(*field.Type.ElemType, b.GoPackagePath)
			buf.WriteString(fmt.Sprintf("\tif v, err := %s; err == nil { tmp := %s(v); obj.%s = &tmp }\n",
				elemConv, goElemType, field.GoName))
		} else {
			conv := ctx.genFromFunxy(valVar, field.Type, b.GoPackagePath)
			if field.Type.Kind == GoTypeBasic {
				// Basic types may need a cast (e.g. int → time.Duration, string → MyString)
				goType := qualifiedGoType(field.Type, b.GoPackagePath)
				buf.WriteString(fmt.Sprintf("\tif v, err := %s; err == nil { obj.%s = %s(v) }\n", conv, field.GoName, goType))
			} else {
				buf.WriteString(fmt.Sprintf("\tif v, err := %s; err == nil { obj.%s = v }\n", conv, field.GoName))
			}
		}

		buf.WriteString("}\n")
	}

	// Return pointer to the struct (most Go APIs expect *T)
	buf.WriteString("return &ext.HostObject{Value: &obj}\n")

	ctx.Functions = append(ctx.Functions, generatedFunc{
		FunxyName: funxyName,
		GoCode:    buf.String(),
	})
}

// isConstructableFieldType returns true if a field type can be populated
// from a Funxy record value. Types that have no Funxy representation
// (functions, interfaces, channels) are skipped in constructors.
func isConstructableFieldType(ref GoTypeRef) bool {
	switch ref.Kind {
	case GoTypeBasic, GoTypeByteSlice:
		return true
	case GoTypePtr:
		if ref.ElemType != nil {
			return isConstructableFieldType(*ref.ElemType)
		}
		return false
	case GoTypeSlice:
		if ref.ElemType != nil {
			return isConstructableFieldType(*ref.ElemType)
		}
		return false
	case GoTypeStruct, GoTypeNamed:
		return true
	case GoTypeFunc, GoTypeInterface, GoTypeContext, GoTypeChan:
		return false
	default:
		return false
	}
}

// generateConstValue generates a constant value entry for the builtins map.
func (ctx *bindingFileContext) generateConstValue(b *ResolvedBinding) {
	cb := b.ConstBinding
	pkgAlias := ImportAlias(b.GoPackagePath)

	goExpr := fmt.Sprintf("toFunxy(%s(%s.%s))", constCast(cb.Type), pkgAlias, cb.GoName)
	ctx.Values = append(ctx.Values, generatedValue{
		FunxyName: b.Spec.As,
		GoExpr:    goExpr,
	})
}

// constCast returns a Go type cast needed for constants (e.g. int, float64, string).
// Go constants may be untyped, so we cast to ensure toFunxy receives the right Go type.
func constCast(ref GoTypeRef) string {
	switch ref.FunxyType {
	case "Int":
		return "int"
	case "Float":
		return "float64"
	case "Bool":
		return "bool"
	case "String":
		return "string"
	default:
		return "interface{}"
	}
}

// genReturnHandling generates code to handle the return values of a Go call.
func (ctx *bindingFileContext) genReturnHandling(buf *strings.Builder, funxyName, callExpr string, sig *FuncSignature, errorToResult, isChained bool, goPkgPath string) {
	results := sig.Results
	if isChained {
		// When chaining (e.g. .Result()), assume the chained method returns (T, error)
		// We generate a simulated two-result handling
		buf.WriteString(fmt.Sprintf("chainVal, chainErr := %s\n", callExpr))
		if errorToResult {
			buf.WriteString("if chainErr != nil {\n")
			buf.WriteString("\treturn makeResultErr(chainErr.Error())\n")
			buf.WriteString("}\n")
			buf.WriteString("return makeResultOk(toFunxy(chainVal))\n")
		} else {
			buf.WriteString("if chainErr != nil {\n")
			buf.WriteString(fmt.Sprintf("\treturn newErr(\"%s: %%v\", chainErr)\n", funxyName))
			buf.WriteString("}\n")
			buf.WriteString("return toFunxy(chainVal)\n")
		}
		return
	}

	numResults := len(results)

	switch {
	case numResults == 0:
		// No return values → call and return nil
		buf.WriteString(fmt.Sprintf("%s\n", callExpr))
		buf.WriteString("return &ext.Nil{}\n")

	case numResults == 1 && !sig.HasErrorReturn:
		// Single non-error return
		buf.WriteString(fmt.Sprintf("result := %s\n", callExpr))
		buf.WriteString("return toFunxy(result)\n")

	case numResults == 1 && sig.HasErrorReturn:
		// Single error return
		buf.WriteString(fmt.Sprintf("err := %s\n", callExpr))
		if errorToResult {
			buf.WriteString("if err != nil {\n")
			buf.WriteString("\treturn makeResultErr(err.Error())\n")
			buf.WriteString("}\n")
			buf.WriteString("return makeResultOk(&ext.Nil{})\n")
		} else {
			buf.WriteString("if err != nil {\n")
			buf.WriteString(fmt.Sprintf("\treturn newErr(\"%s: %%v\", err)\n", funxyName))
			buf.WriteString("}\n")
			buf.WriteString("return &ext.Nil{}\n")
		}

	case numResults == 2 && sig.HasErrorReturn:
		// (T, error) — the most common pattern
		buf.WriteString(fmt.Sprintf("result, err := %s\n", callExpr))
		if errorToResult {
			buf.WriteString("if err != nil {\n")
			buf.WriteString("\treturn makeResultErr(err.Error())\n")
			buf.WriteString("}\n")
			buf.WriteString("return makeResultOk(toFunxy(result))\n")
		} else {
			buf.WriteString("if err != nil {\n")
			buf.WriteString(fmt.Sprintf("\treturn newErr(\"%s: %%v\", err)\n", funxyName))
			buf.WriteString("}\n")
			buf.WriteString("return toFunxy(result)\n")
		}

	default:
		// Multiple returns → Tuple
		vars := make([]string, numResults)
		for i := range results {
			if i == numResults-1 && sig.HasErrorReturn {
				vars[i] = "retErr"
			} else {
				vars[i] = fmt.Sprintf("ret%d", i)
			}
		}
		buf.WriteString(fmt.Sprintf("%s := %s\n", strings.Join(vars, ", "), callExpr))

		if sig.HasErrorReturn {
			if errorToResult {
				buf.WriteString("if retErr != nil {\n")
				buf.WriteString("\treturn makeResultErr(retErr.Error())\n")
				buf.WriteString("}\n")
			} else {
				buf.WriteString("if retErr != nil {\n")
				buf.WriteString(fmt.Sprintf("\treturn newErr(\"%s: %%v\", retErr)\n", funxyName))
				buf.WriteString("}\n")
			}
		}

		// Build tuple from non-error results
		nonErrorVars := vars
		if sig.HasErrorReturn {
			nonErrorVars = vars[:numResults-1]
		}

		if len(nonErrorVars) == 1 {
			if errorToResult {
				buf.WriteString(fmt.Sprintf("return makeResultOk(toFunxy(%s))\n", nonErrorVars[0]))
			} else {
				buf.WriteString(fmt.Sprintf("return toFunxy(%s)\n", nonErrorVars[0]))
			}
		} else {
			buf.WriteString("elements := []ext.Object{\n")
			for _, v := range nonErrorVars {
				buf.WriteString(fmt.Sprintf("\ttoFunxy(%s),\n", v))
			}
			buf.WriteString("}\n")
			if errorToResult {
				buf.WriteString("return makeResultOk(&ext.Tuple{Elements: elements})\n")
			} else {
				buf.WriteString("return &ext.Tuple{Elements: elements}\n")
			}
		}
	}
}

// genFromFunxy generates a Go expression that converts a Funxy Object to the target Go type.
// Returns an expression of type (T, error).
func (ctx *bindingFileContext) genFromFunxy(objExpr string, ref GoTypeRef, goPkgPath string) string {
	switch ref.Kind {
	case GoTypeBasic:
		switch ref.FunxyType {
		case "Int":
			return fmt.Sprintf("toGoInt(%s)", objExpr)
		case "Float":
			return fmt.Sprintf("toGoFloat(%s)", objExpr)
		case "Bool":
			return fmt.Sprintf("toGoBool(%s)", objExpr)
		case "String":
			return fmt.Sprintf("toGoString(%s)", objExpr)
		default:
			return fmt.Sprintf("toGoAny(%s)", objExpr)
		}

	case GoTypeTypeParam:
		// After substitution this should not appear — safety net: treat as any.
		return fmt.Sprintf("toGoAny(%s)", objExpr)

	case GoTypeByteSlice:
		return fmt.Sprintf("toGoBytes(%s)", objExpr)

	case GoTypePtr:
		// Pointer types: extract from HostObject
		goType := qualifiedGoType(ref, goPkgPath)
		return fmt.Sprintf("toGoHost[%s](%s)", goType, objExpr)

	case GoTypeStruct, GoTypeNamed, GoTypeInterface:
		goType := qualifiedGoType(ref, goPkgPath)
		return fmt.Sprintf("toGoHost[%s](%s)", goType, objExpr)

	case GoTypeSlice:
		if ref.ElemType != nil {
			elemGoType := qualifiedGoType(*ref.ElemType, goPkgPath)
			return fmt.Sprintf("toGoSlice[%s](%s)", elemGoType, objExpr)
		}
		return fmt.Sprintf("toGoAny(%s)", objExpr)

	case GoTypeMap:
		if ref.KeyType != nil && ref.ElemType != nil {
			keyGoType := qualifiedGoType(*ref.KeyType, goPkgPath)
			valGoType := qualifiedGoType(*ref.ElemType, goPkgPath)
			return fmt.Sprintf("toGoMap[%s, %s](%s)", keyGoType, valGoType, objExpr)
		}
		return fmt.Sprintf("toGoAny(%s)", objExpr)

	case GoTypeFunc:
		// Callbacks are handled separately via genCallbackConversion.
		// If we reach here, fall through to toGoAny.
		return fmt.Sprintf("toGoAny(%s)", objExpr)

	case GoTypeError:
		return fmt.Sprintf("toGoError(%s)", objExpr)

	case GoTypeContext:
		return fmt.Sprintf("toGoContext(%s)", objExpr)

	default:
		return fmt.Sprintf("toGoAny(%s)", objExpr)
	}
}

// genCallbackConversion generates inline code that wraps a Funxy callable into a Go func.
// It writes the code to buf and returns the variable name holding the Go func.
func (ctx *bindingFileContext) genCallbackConversion(buf *strings.Builder, varName string, argsExpr string, ref GoTypeRef, goPkgPath string) {
	sig := ref.FuncSig
	if sig == nil {
		// Fallback: just use toGoAny
		buf.WriteString(fmt.Sprintf("%s, _ := toGoAny(%s)\n", varName, argsExpr))
		return
	}

	goFuncType := qualifiedGoFuncType(sig, goPkgPath)

	// Capture the Funxy callable
	cbObjName := varName + "Fn"
	buf.WriteString(fmt.Sprintf("%s := %s\n", cbObjName, argsExpr))

	// Generate the Go func literal
	// Build parameter list: p0 type0, p1 type1, ...
	var goParams []string
	for i, p := range sig.Params {
		goParams = append(goParams, fmt.Sprintf("p%d %s", i, qualifiedGoType(p.Type, goPkgPath)))
	}

	// Build return type list
	var goResults []string
	for _, r := range sig.Results {
		goResults = append(goResults, qualifiedGoType(r.Type, goPkgPath))
	}

	// Start the func literal assignment
	buf.WriteString(fmt.Sprintf("var %s %s\n", varName, goFuncType))
	buf.WriteString(fmt.Sprintf("%s = func(%s)", varName, strings.Join(goParams, ", ")))
	switch len(goResults) {
	case 0:
		buf.WriteString(" {\n")
	case 1:
		buf.WriteString(fmt.Sprintf(" %s {\n", goResults[0]))
	default:
		buf.WriteString(fmt.Sprintf(" (%s) {\n", strings.Join(goResults, ", ")))
	}

	// Convert Go args to Funxy args
	buf.WriteString("\tcbArgs := []ext.Object{")
	for i := range sig.Params {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(fmt.Sprintf("toFunxy(p%d)", i))
	}
	buf.WriteString("}\n")

	// Call the Funxy function via evaluator
	buf.WriteString(fmt.Sprintf("\tcbResult := ev.ApplyFunction(%s, cbArgs)\n", cbObjName))

	// Handle results
	numResults := len(sig.Results)

	if numResults == 0 {
		// No return values — just check for error
		buf.WriteString("\tif cbResult != nil && cbResult.Type() == \"ERROR\" {\n")
		buf.WriteString(fmt.Sprintf("\t\tpanic(fmt.Sprintf(\"callback error: %%s\", cbResult.(*ext.Error).Message))\n"))
		buf.WriteString("\t}\n")
	} else if numResults == 1 && sig.HasErrorReturn {
		// Single error return
		buf.WriteString("\tif cbResult != nil && cbResult.Type() == \"ERROR\" {\n")
		buf.WriteString("\t\treturn fmt.Errorf(\"%s\", cbResult.(*ext.Error).Message)\n")
		buf.WriteString("\t}\n")
		buf.WriteString("\treturn nil\n")
	} else if numResults == 1 {
		// Single non-error return
		buf.WriteString("\tif cbResult != nil && cbResult.Type() == \"ERROR\" {\n")
		buf.WriteString(fmt.Sprintf("\t\tpanic(fmt.Sprintf(\"callback error: %%s\", cbResult.(*ext.Error).Message))\n"))
		buf.WriteString("\t}\n")
		goType := qualifiedGoType(sig.Results[0].Type, goPkgPath)
		buf.WriteString(fmt.Sprintf("\tvar r0 %s\n", goType))
		ctx.genCallbackResultConversion(buf, "r0", "cbResult", sig.Results[0].Type, goPkgPath)
		buf.WriteString("\treturn r0\n")
	} else if numResults == 2 && sig.HasErrorReturn {
		// (T, error) — the most common callback pattern
		buf.WriteString("\tif cbResult != nil && cbResult.Type() == \"ERROR\" {\n")
		goType := qualifiedGoType(sig.Results[0].Type, goPkgPath)
		buf.WriteString(fmt.Sprintf("\t\tvar zero %s\n", goType))
		buf.WriteString("\t\treturn zero, fmt.Errorf(\"%s\", cbResult.(*ext.Error).Message)\n")
		buf.WriteString("\t}\n")
		buf.WriteString(fmt.Sprintf("\tvar r0 %s\n", goType))
		ctx.genCallbackResultConversion(buf, "r0", "cbResult", sig.Results[0].Type, goPkgPath)
		buf.WriteString("\treturn r0, nil\n")
	} else {
		// Multiple returns: panic on error, return zero values for unhandled cases
		buf.WriteString("\tif cbResult != nil && cbResult.Type() == \"ERROR\" {\n")
		buf.WriteString(fmt.Sprintf("\t\tpanic(fmt.Sprintf(\"callback error: %%s\", cbResult.(*ext.Error).Message))\n"))
		buf.WriteString("\t}\n")
		// For simplicity, return zero values with first result from cbResult
		for i, r := range sig.Results {
			if sig.HasErrorReturn && i == numResults-1 {
				continue
			}
			goType := qualifiedGoType(r.Type, goPkgPath)
			buf.WriteString(fmt.Sprintf("\tvar r%d %s\n", i, goType))
			if i == 0 {
				ctx.genCallbackResultConversion(buf, fmt.Sprintf("r%d", i), "cbResult", r.Type, goPkgPath)
			}
		}
		buf.WriteString("\treturn ")
		first := true
		for i := range sig.Results {
			if sig.HasErrorReturn && i == numResults-1 {
				if !first {
					buf.WriteString(", ")
				}
				buf.WriteString("nil")
				first = false
				continue
			}
			if !first {
				buf.WriteString(", ")
			}
			buf.WriteString(fmt.Sprintf("r%d", i))
			first = false
		}
		buf.WriteString("\n")
	}

	buf.WriteString("}\n")
}

// genCallbackResultConversion generates a simple conversion from Funxy result to Go type.
func (ctx *bindingFileContext) genCallbackResultConversion(buf *strings.Builder, varName, resultExpr string, ref GoTypeRef, goPkgPath string) {
	switch ref.Kind {
	case GoTypeBasic:
		switch ref.FunxyType {
		case "Int":
			buf.WriteString(fmt.Sprintf("\tif v, err := toGoInt(%s); err == nil { %s = v }\n", resultExpr, varName))
		case "Float":
			buf.WriteString(fmt.Sprintf("\tif v, err := toGoFloat(%s); err == nil { %s = v }\n", resultExpr, varName))
		case "Bool":
			buf.WriteString(fmt.Sprintf("\tif v, err := toGoBool(%s); err == nil { %s = v }\n", resultExpr, varName))
		case "String":
			buf.WriteString(fmt.Sprintf("\tif v, err := toGoString(%s); err == nil { %s = v }\n", resultExpr, varName))
		default:
			buf.WriteString(fmt.Sprintf("\tif v, err := toGoAny(%s); err == nil { %s, _ = v.(%s) }\n", resultExpr, varName, qualifiedGoType(ref, goPkgPath)))
		}
	case GoTypeError:
		buf.WriteString(fmt.Sprintf("\tif v, err := toGoError(%s); err == nil { %s = v }\n", resultExpr, varName))
	default:
		goType := qualifiedGoType(ref, goPkgPath)
		buf.WriteString(fmt.Sprintf("\tif v, err := toGoAny(%s); err == nil { %s, _ = v.(%s) }\n", resultExpr, varName, goType))
	}
}

// render generates the final Go source for this binding file.
func (ctx *bindingFileContext) render() (string, error) {
	// Indent GoCode for each function to fit inside the closure
	indentedFuncs := make([]generatedFunc, len(ctx.Functions))
	for i, f := range ctx.Functions {
		indentedFuncs[i] = generatedFunc{
			FunxyName: f.FunxyName,
			GoCode:    indentCode(f.GoCode, "\t\t\t"),
		}
	}

	tmpl, err := template.New("binding").Parse(bindingFileTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	data := struct {
		Alias      string
		Identifier string
		Imports    []importEntry
		Funcs      []generatedFunc
		Values     []generatedValue
	}{
		Alias:      ctx.Alias,
		Identifier: identifier(ctx.Alias),
		Imports:    ctx.sortedImports(),
		Funcs:      indentedFuncs,
		Values:     ctx.Values,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

// indentCode adds a prefix to each line of code.
func indentCode(code, prefix string) string {
	lines := strings.Split(strings.TrimRight(code, "\n"), "\n")
	var result strings.Builder
	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}
		if line != "" {
			result.WriteString(prefix)
			result.WriteString(line)
		}
	}
	result.WriteString("\n")
	return result.String()
}

type importEntry struct {
	Path  string
	Alias string
}

func (ctx *bindingFileContext) sortedImports() []importEntry {
	// Skip imports that are already in the template header (context, fmt)
	skip := map[string]bool{
		"context": true,
		"fmt":     true,
	}

	var entries []importEntry
	for path, alias := range ctx.Imports {
		if skip[path] {
			continue
		}
		entries = append(entries, importEntry{Path: path, Alias: alias})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries
}

// goReservedWords are Go keywords that cannot be used as import aliases.
var goReservedWords = map[string]bool{
	"break": true, "default": true, "func": true, "interface": true, "select": true,
	"case": true, "defer": true, "go": true, "map": true, "struct": true,
	"chan": true, "else": true, "goto": true, "package": true, "switch": true,
	"const": true, "fallthrough": true, "if": true, "range": true, "type": true,
	"continue": true, "for": true, "import": true, "return": true, "var": true,
	// Generated code uses these identifiers, so avoid them as aliases
	"context": true, "fmt": true, "ext": true, "main": true, "init": true,
}

// ImportAlias returns a valid Go identifier for an import path.
// Handles hyphens (go-redis → goredis), versioned paths (v9 → parent),
// and reserved words (go → pkgGo).
func ImportAlias(pkgPath string) string {
	parts := strings.Split(pkgPath, "/")
	last := parts[len(parts)-1]
	// Handle versioned imports like "v9" → use parent
	if len(last) > 0 && last[0] == 'v' && len(parts) > 1 {
		allDigits := true
		for _, c := range last[1:] {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			last = parts[len(parts)-2]
		}
	}

	// Sanitize identifier: keep only letters, digits, and underscores (whitelist)
	alias := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return r
		}
		return -1
	}, last)

	if alias == "" {
		alias = "pkg"
	}

	if goReservedWords[alias] {
		alias = "pkg" + strings.ToUpper(alias[:1]) + alias[1:]
	}
	return alias
}

// identifier returns a valid Go identifier for a string.
// Replaces invalid characters with underscores.
func identifier(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return r
		}
		return '_'
	}, s)
}

// qualifiedGoType returns the Go type string with package qualifier.
func qualifiedGoType(ref GoTypeRef, contextPkgPath string) string {
	switch ref.Kind {
	case GoTypeBasic:
		return ref.GoString
	case GoTypeTypeParam:
		// After substitution this should not appear — but as a safety net, use "any".
		return "any"
	case GoTypeByteSlice:
		return "[]byte"
	case GoTypePtr:
		if ref.ElemType != nil {
			return "*" + qualifiedGoType(*ref.ElemType, contextPkgPath)
		}
		return ref.GoString
	case GoTypeSlice:
		if ref.ElemType != nil {
			return "[]" + qualifiedGoType(*ref.ElemType, contextPkgPath)
		}
		return ref.GoString
	case GoTypeError:
		return "error"
	case GoTypeContext:
		return "context.Context"
	case GoTypeFunc:
		if ref.FuncSig != nil {
			return qualifiedGoFuncType(ref.FuncSig, contextPkgPath)
		}
		return ref.GoString
	default:
		if ref.PkgPath != "" && ref.TypeName != "" {
			return ImportAlias(ref.PkgPath) + "." + ref.TypeName
		}
		return ref.GoString
	}
}

// qualifiedGoFuncType generates a Go func type string from a FuncSignature.
// E.g. "func(string, int) (bool, error)"
func qualifiedGoFuncType(sig *FuncSignature, contextPkgPath string) string {
	var params []string
	for _, p := range sig.Params {
		params = append(params, qualifiedGoType(p.Type, contextPkgPath))
	}

	var results []string
	for _, r := range sig.Results {
		results = append(results, qualifiedGoType(r.Type, contextPkgPath))
	}

	paramStr := strings.Join(params, ", ")
	switch len(results) {
	case 0:
		return "func(" + paramStr + ")"
	case 1:
		return "func(" + paramStr + ") " + results[0]
	default:
		return "func(" + paramStr + ") (" + strings.Join(results, ", ") + ")"
	}
}

// qualifiedGoTypeName returns the fully qualified Go type name, including type args for generics.
// E.g. qualifiedGoTypeName("slices", "Set", ["string"]) → "slices.Set[string]"
func qualifiedGoTypeName(pkgAlias, goName string, typeArgs []string) string {
	name := pkgAlias + "." + goName
	if len(typeArgs) > 0 {
		name += "[" + strings.Join(typeArgs, ", ") + "]"
	}
	return name
}

// generateMainFile generates the main.go that registers all ext modules and
// replicates the standard funxy main.
func (cg *CodeGenerator) generateMainFile(result *InspectResult) (GeneratedFile, error) {
	// Collect unique ext module names
	aliases := make(map[string]bool)
	for _, b := range result.Bindings {
		aliases[b.Dep.ExtModuleName()] = true
	}

	type modEntry struct {
		Name       string
		Identifier string
	}
	sorted := make([]modEntry, 0, len(aliases))
	for a := range aliases {
		sorted = append(sorted, modEntry{Name: a, Identifier: identifier(a)})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	// Collect unique Go package imports
	goPkgs := make(map[string]bool)
	for _, b := range result.Bindings {
		goPkgs[b.GoPackagePath] = true
	}

	tmpl, err := template.New("main").Parse(mainFileTemplate)
	if err != nil {
		return GeneratedFile{}, fmt.Errorf("parsing main template: %w", err)
	}

	data := struct {
		FunxyModulePath string
		Modules         []modEntry
	}{
		FunxyModulePath: cg.funxyModulePath,
		Modules:         sorted,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return GeneratedFile{}, fmt.Errorf("executing main template: %w", err)
	}

	return GeneratedFile{
		Filename: "ext_init.go",
		Content:  buf.String(),
	}, nil
}

// Templates

const bindingFileTemplate = `// Code generated by funxy ext codegen. DO NOT EDIT.
package main

import (
	"context"
	"fmt"
{{- range .Imports}}
{{- if .Alias}}
	{{.Alias}} "{{.Path}}"
{{- else}}
	"{{.Path}}"
{{- end}}
{{- end}}
)

// Suppress unused import warnings
var _ = context.Background
var _ = fmt.Sprintf

// register_{{.Identifier}} returns the builtins map for the "ext/{{.Alias}}" module.
func register_{{.Identifier}}() map[string]ext.Object {
	builtins := make(map[string]ext.Object)
{{- range .Values}}

	builtins["{{.FunxyName}}"] = {{.GoExpr}}
{{- end}}

{{- range .Funcs}}

	builtins["{{.FunxyName}}"] = &ext.Builtin{
		Name: "{{.FunxyName}}",
		Fn: func(ev *ext.Evaluator, args ...ext.Object) ext.Object {
{{.GoCode}}
		},
	}
{{- end}}

	return builtins
}
`

const mainFileTemplate = `// Code generated by funxy ext codegen. DO NOT EDIT.
package main

import (
	"{{.FunxyModulePath}}/pkg/ext"
)

func init() {
	// Register ext/* builtins before main() runs.
	// The real main.go from the Funxy source tree is copied into this workspace,
	// so this binary is a full-featured funxy interpreter with ext modules compiled in.
{{- range .Modules}}
	ext.RegisterExtBuiltins("{{.Name}}", register_{{.Identifier}}())
{{- end}}
}
`

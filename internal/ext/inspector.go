package ext

import (
	"fmt"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// InspectResult holds all extracted type information for code generation.
type InspectResult struct {
	// Bindings is an ordered list of resolved binding specifications.
	Bindings []*ResolvedBinding
}

// ResolvedBinding represents a fully resolved binding from funxy.yaml
// with all type information extracted from Go source.
type ResolvedBinding struct {
	// Spec is the original bind specification from funxy.yaml.
	Spec BindSpec

	// Dep is the parent dependency.
	Dep Dep

	// GoPackagePath is the full Go import path.
	GoPackagePath string

	// TypeBinding is non-nil for type bindings.
	TypeBinding *TypeBinding

	// FuncBinding is non-nil for func bindings.
	FuncBinding *FuncBinding

	// ConstBinding is non-nil for const bindings.
	ConstBinding *ConstBinding
}

// TypeBinding represents a bound Go type with its methods.
type TypeBinding struct {
	// GoName is the Go type name (e.g. "Client").
	GoName string

	// IsInterface is true if the type is an interface.
	IsInterface bool

	// IsStruct is true if the type is a struct.
	IsStruct bool

	// Methods is the ordered list of bound methods.
	Methods []*MethodInfo

	// Fields holds exported struct fields (if IsStruct and the underlying type is accessible).
	Fields []*FieldInfo

	// TypeParams describes the Go type parameters, if any (for generic types).
	TypeParams []TypeParamInfo

	// TypeArgs are the resolved Go type strings for instantiation (e.g. ["string"]).
	TypeArgs []string
}

// FuncBinding represents a bound standalone Go function.
type FuncBinding struct {
	// GoName is the Go function name (e.g. "New").
	GoName string

	// Signature is the function signature.
	Signature *FuncSignature

	// TypeParams describes the Go type parameters, if any (for generic functions).
	TypeParams []TypeParamInfo

	// TypeArgs are the resolved Go type strings for instantiation (e.g. ["any", "string"]).
	// Set during resolution: auto-inferred for unconstrained params, or from BindSpec.TypeArgs.
	TypeArgs []string
}

// TypeParamInfo describes a single Go type parameter on a generic function or type.
type TypeParamInfo struct {
	// Name is the Go type parameter name (e.g. "T", "K").
	Name string
	// Constraint is the constraint string (e.g. "any", "comparable", "~int | ~float64").
	Constraint string
	// IsAny is true if the constraint is interface{}/any (unconstrained).
	IsAny bool
}

// MethodInfo describes a single method on a Go type.
type MethodInfo struct {
	// GoName is the Go method name (e.g. "Get").
	GoName string

	// FunxyName is the Funxy function name (e.g. "redisGet").
	FunxyName string

	// Signature is the method signature.
	Signature *FuncSignature

	// HasPointerReceiver is true if the method is on *T (not T).
	HasPointerReceiver bool

	// AutoChainMethod is auto-detected when the method returns a single
	// non-error type that has a .Result() (or similar) method returning (T, error).
	// Used to auto-chain fluent APIs (e.g. go-redis *StatusCmd → .Result()).
	// Empty if no chainable method was detected.
	AutoChainMethod string
}

// FieldInfo describes an exported struct field.
type FieldInfo struct {
	// GoName is the Go field name.
	GoName string

	// FunxyName is the Funxy field name (camelCase).
	FunxyName string

	// Type is the Go type of the field.
	Type GoTypeRef
}

// FuncSignature describes a function's parameters and return types.
type FuncSignature struct {
	// Params is the ordered list of parameters (excluding receiver for methods).
	Params []*ParamInfo

	// Results is the ordered list of return values.
	Results []*ParamInfo

	// IsVariadic is true if the last parameter is variadic (...T).
	IsVariadic bool

	// HasContextParam is true if the first parameter is context.Context.
	HasContextParam bool

	// HasErrorReturn is true if the last return value is error.
	HasErrorReturn bool
}

// ParamInfo describes a function parameter or return value.
type ParamInfo struct {
	// Name is the parameter name (may be empty for unnamed params/returns).
	Name string

	// Type is the Go type reference.
	Type GoTypeRef

	// IsVariadic is true if this is the variadic element type (not the slice).
	IsVariadic bool
}

// GoTypeRef represents a reference to a Go type, with enough information
// for code generation.
type GoTypeRef struct {
	// Kind categorizes the type for codegen purposes.
	Kind GoTypeKind

	// GoString is the Go type as a string (e.g. "string", "*redis.Client", "[]byte").
	GoString string

	// PkgPath is the import path for named types (e.g. "github.com/redis/go-redis/v9").
	// Empty for builtin types.
	PkgPath string

	// TypeName is the unqualified type name for named types (e.g. "Client").
	// Empty for builtin and anonymous types.
	TypeName string

	// ElemType is the element type for slices, arrays, maps, pointers, and channels.
	ElemType *GoTypeRef

	// KeyType is the key type for maps.
	KeyType *GoTypeRef

	// FunxyType is the corresponding Funxy type string (e.g. "Int", "String", "List<Int>").
	FunxyType string

	// FuncSig holds the full function signature for GoTypeFunc parameters.
	// This enables generating callback wrappers that bridge Funxy closures to Go func types.
	FuncSig *FuncSignature

	// TypeParamIndex is the index in the parent function/type's type parameter list.
	// Only meaningful when Kind == GoTypeTypeParam.
	TypeParamIndex int
}

// ConstBinding represents a bound package-level constant.
type ConstBinding struct {
	// GoName is the Go constant name (e.g. "StatusOK").
	GoName string

	// Type is the Go type of the constant.
	Type GoTypeRef
}

// GoTypeKind categorizes Go types for code generation.
type GoTypeKind int

const (
	GoTypeBasic     GoTypeKind = iota // bool, int, float64, string, etc.
	GoTypeStruct                      // struct types
	GoTypeInterface                   // interface types
	GoTypePtr                         // *T
	GoTypeSlice                       // []T
	GoTypeArray                       // [N]T
	GoTypeMap                         // map[K]V
	GoTypeFunc                        // func types
	GoTypeChan                        // channel types
	GoTypeError                       // the error interface
	GoTypeContext                     // context.Context
	GoTypeByteSlice                   // []byte (special case, maps to Bytes)
	GoTypeNamed                       // other named types (wrapped as HostObject)
	GoTypeTypeParam                   // type parameter (T, K, etc.) — from Go generics
)

// Inspector loads Go packages and extracts type information for binding.
type Inspector struct {
	// workDir is the temporary Go module directory used for package loading.
	workDir string

	// loadedPkgs caches loaded packages by import path.
	loadedPkgs map[string]*packages.Package

	// goVersion is the Go version to use in the generated go.mod.
	goVersion string

	// configDir is the directory containing funxy.yaml (for local: paths).
	configDir string
}

// NewInspector creates a new Inspector.
// goVersion should match the target Go version (e.g. "1.25.3").
func NewInspector(goVersion string) *Inspector {
	return &Inspector{
		loadedPkgs: make(map[string]*packages.Package),
		goVersion:  goVersion,
	}
}

// SetConfigDir sets the directory containing funxy.yaml (for resolving local: paths).
func (ins *Inspector) SetConfigDir(dir string) {
	ins.configDir = dir
}

// Inspect loads all Go packages referenced in the config and extracts
// type information for code generation.
func (ins *Inspector) Inspect(cfg *Config) (*InspectResult, error) {
	// Step 1: Create temporary Go module for package loading
	if err := ins.setupWorkspace(cfg, ins.configDir); err != nil {
		return nil, fmt.Errorf("setting up workspace: %w", err)
	}

	// Step 2: Load all referenced Go packages
	pkgPaths := collectPackagePaths(cfg)
	if err := ins.loadPackages(pkgPaths); err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}

	// Step 3: Resolve each binding
	var bindings []*ResolvedBinding
	for _, dep := range cfg.Deps {
		if dep.BindAll {
			resolved, err := ins.resolveBindAll(dep)
			if err != nil {
				return nil, fmt.Errorf("resolving bind_all for %s: %w", dep.Pkg, err)
			}
			bindings = append(bindings, resolved...)
		} else {
			for _, bind := range dep.Bind {
				resolved, err := ins.resolveBinding(dep, bind)
				if err != nil {
					return nil, fmt.Errorf("resolving binding %s/%s: %w", dep.Pkg, bindTarget(bind), err)
				}
				bindings = append(bindings, resolved)
			}
		}
	}

	return &InspectResult{Bindings: bindings}, nil
}

// Cleanup removes the temporary workspace.
func (ins *Inspector) Cleanup() {
	if ins.workDir != "" {
		os.RemoveAll(ins.workDir)
		ins.workDir = ""
	}
}

// WorkDir returns the path to the temporary workspace.
// Used by the builder to reuse the workspace for the final go build.
func (ins *Inspector) WorkDir() string {
	return ins.workDir
}

// setupWorkspace creates a temporary Go module with the required dependencies.
// configDir is the directory containing funxy.yaml (for resolving local: paths).
// Pass "" if unknown (local deps will fail to resolve).
func (ins *Inspector) setupWorkspace(cfg *Config, configDir ...string) error {
	dir, err := os.MkdirTemp("", "funxy-ext-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	ins.workDir = dir

	cfgDir := ""
	if len(configDir) > 0 {
		cfgDir = configDir[0]
	}

	// Generate go.mod
	var gomod strings.Builder
	gomod.WriteString("module funxy-ext-build\n\n")
	gomod.WriteString(fmt.Sprintf("go %s\n\n", ins.goVersion))
	gomod.WriteString("require (\n")
	for _, req := range cfg.GoModRequires() {
		parts := strings.SplitN(req, " ", 2)
		if len(parts) == 2 {
			gomod.WriteString(fmt.Sprintf("\t%s %s\n", parts[0], parts[1]))
		} else {
			// Version will be resolved by go mod tidy
			gomod.WriteString(fmt.Sprintf("\t%s v0.0.0\n", parts[0]))
		}
	}
	gomod.WriteString(")\n")

	// Add replace directives for local deps
	replaces := cfg.GoModReplaces(cfgDir)
	for _, replace := range replaces {
		gomod.WriteString(fmt.Sprintf("\nreplace %s\n", replace))
	}

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod.String()), 0o644); err != nil {
		return fmt.Errorf("writing go.mod: %w", err)
	}

	// Generate a dummy .go file that imports all packages
	// (needed for go mod tidy to resolve versions)
	var dummy strings.Builder
	dummy.WriteString("package main\n\nimport (\n")
	for _, dep := range cfg.Deps {
		dummy.WriteString(fmt.Sprintf("\t_ %q\n", dep.Pkg))
	}
	dummy.WriteString(")\n\nfunc main() {}\n")

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(dummy.String()), 0o644); err != nil {
		return fmt.Errorf("writing main.go: %w", err)
	}

	// Run go mod tidy to resolve dependencies
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go mod tidy failed: %s\n%w", string(output), err)
	}

	return nil
}

// loadPackages loads the specified Go packages using go/packages.
func (ins *Inspector) loadPackages(pkgPaths []string) error {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedSyntax |
			packages.NeedImports |
			packages.NeedDeps,
		Dir: ins.workDir,
		Env: append(os.Environ(), "GOWORK=off"),
	}

	pkgs, err := packages.Load(cfg, pkgPaths...)
	if err != nil {
		return fmt.Errorf("loading packages: %w", err)
	}

	// Check for package errors
	var errs []string
	for _, pkg := range pkgs {
		for _, e := range pkg.Errors {
			errs = append(errs, fmt.Sprintf("%s: %s", pkg.PkgPath, e.Msg))
		}
		ins.loadedPkgs[pkg.PkgPath] = pkg
	}

	if len(errs) > 0 {
		return fmt.Errorf("package errors:\n  %s", strings.Join(errs, "\n  "))
	}

	return nil
}

// resolveBinding resolves a single BindSpec to a ResolvedBinding.
func (ins *Inspector) resolveBinding(dep Dep, bind BindSpec) (*ResolvedBinding, error) {
	pkg, ok := ins.loadedPkgs[dep.Pkg]
	if !ok {
		return nil, fmt.Errorf("package %s not loaded", dep.Pkg)
	}

	resolved := &ResolvedBinding{
		Spec:          bind,
		Dep:           dep,
		GoPackagePath: dep.Pkg,
	}

	if bind.Type != "" {
		tb, err := ins.resolveTypeBinding(pkg, bind)
		if err != nil {
			return nil, err
		}
		resolved.TypeBinding = tb
	} else if bind.Const != "" {
		cb, err := ins.resolveConstBinding(pkg, bind)
		if err != nil {
			return nil, err
		}
		resolved.ConstBinding = cb
	} else {
		fb, err := ins.resolveFuncBinding(pkg, bind)
		if err != nil {
			return nil, err
		}
		resolved.FuncBinding = fb
	}

	return resolved, nil
}

// resolveBindAll resolves a bind_all dependency to a list of ResolvedBindings.
func (ins *Inspector) resolveBindAll(dep Dep) ([]*ResolvedBinding, error) {
	pkg, ok := ins.loadedPkgs[dep.Pkg]
	if !ok {
		return nil, fmt.Errorf("package %s not loaded", dep.Pkg)
	}

	scope := pkg.Types.Scope()
	var bindings []*ResolvedBinding

	// Collect all exported types and functions
	names := scope.Names()
	sort.Strings(names)

	for _, name := range names {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}

		switch obj := obj.(type) {
		case *types.TypeName:
			bind := BindSpec{
				Type: name,
				As:   dep.As,
			}
			tb, err := ins.resolveTypeBinding(pkg, bind)
			if err != nil {
				// Skip types we can't bind (e.g. type aliases to basic types)
				continue
			}
			bindings = append(bindings, &ResolvedBinding{
				Spec:          bind,
				Dep:           dep,
				GoPackagePath: dep.Pkg,
				TypeBinding:   tb,
			})

		case *types.Func:
			sig, ok := obj.Type().(*types.Signature)
			if !ok || sig.Recv() != nil {
				continue // Skip methods, only bind package-level functions
			}
			funcName := lcFirst(name) // camelCase for Funxy
			bind := BindSpec{
				Func: name,
				As:   dep.As + ucFirst(funcName),
			}
			fb, err := ins.resolveFuncBinding(pkg, bind)
			if err != nil {
				continue
			}
			bindings = append(bindings, &ResolvedBinding{
				Spec:          bind,
				Dep:           dep,
				GoPackagePath: dep.Pkg,
				FuncBinding:   fb,
			})

		case *types.Const:
			constName := lcFirst(name)
			bind := BindSpec{
				Const: name,
				As:    dep.As + ucFirst(constName),
			}
			cb := resolveConstFromObj(obj)
			bindings = append(bindings, &ResolvedBinding{
				Spec:          bind,
				Dep:           dep,
				GoPackagePath: dep.Pkg,
				ConstBinding:  cb,
			})
		}
	}

	return bindings, nil
}

// resolveTypeBinding extracts method and field information for a type binding.
func (ins *Inspector) resolveTypeBinding(pkg *packages.Package, bind BindSpec) (*TypeBinding, error) {
	scope := pkg.Types.Scope()
	obj := scope.Lookup(bind.Type)
	if obj == nil {
		return nil, fmt.Errorf("type %q not found in package %s", bind.Type, pkg.PkgPath)
	}

	typeName, ok := obj.(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("%q is not a type in package %s", bind.Type, pkg.PkgPath)
	}

	namedType, ok := typeName.Type().(*types.Named)
	if !ok {
		return nil, fmt.Errorf("%q is not a named type in package %s", bind.Type, pkg.PkgPath)
	}

	tb := &TypeBinding{
		GoName: bind.Type,
	}

	// Detect generic type parameters and instantiate if needed
	if tparams := namedType.TypeParams(); tparams != nil && tparams.Len() > 0 {
		for i := 0; i < tparams.Len(); i++ {
			tp := tparams.At(i)
			tb.TypeParams = append(tb.TypeParams, TypeParamInfo{
				Name:       tp.Obj().Name(),
				Constraint: types.TypeString(tp.Constraint(), nil),
				IsAny:      isAnyConstraint(tp),
			})
		}

		typeArgStrs, _, err := resolveTypeArgs(tb.TypeParams, bind.TypeArgs)
		if err != nil {
			return nil, fmt.Errorf("generic type %s: %w", bind.Type, err)
		}
		tb.TypeArgs = typeArgStrs

		// Instantiate the named type with concrete type args
		goTypeArgs := make([]types.Type, len(typeArgStrs))
		for i, s := range typeArgStrs {
			goTypeArgs[i] = parseBasicGoType(s)
		}
		instantiated, err := types.Instantiate(types.NewContext(), namedType, goTypeArgs, false)
		if err != nil {
			return nil, fmt.Errorf("instantiating %s[%s]: %w", bind.Type, strings.Join(typeArgStrs, ", "), err)
		}
		instNamed, ok := instantiated.(*types.Named)
		if !ok {
			return nil, fmt.Errorf("instantiated %s is not a named type", bind.Type)
		}
		namedType = instNamed
	}

	// Determine if struct or interface
	underlying := namedType.Underlying()
	if _, ok := underlying.(*types.Struct); ok {
		tb.IsStruct = true
	}
	if _, ok := underlying.(*types.Interface); ok {
		tb.IsInterface = true
	}

	// Extract methods
	methodFilter := buildMethodFilter(bind)

	// We need methods from both the type itself and the pointer type
	// to get all methods (value receiver + pointer receiver)
	mset := types.NewMethodSet(types.NewPointer(namedType))
	for i := 0; i < mset.Len(); i++ {
		sel := mset.At(i)
		method := sel.Obj().(*types.Func)

		if !method.Exported() {
			continue
		}

		if !methodFilter(method.Name()) {
			continue
		}

		sig := method.Type().(*types.Signature)

		methodInfo := &MethodInfo{
			GoName:    method.Name(),
			FunxyName: makeFunxyMethodName(bind.As, method.Name()),
			Signature: extractSignature(sig),
		}

		// Check if this is a pointer receiver method
		// The method set of *T includes methods of T, but pointer-receiver methods
		// are only in the method set of *T.
		valMset := types.NewMethodSet(namedType)
		if valMset.Lookup(method.Pkg(), method.Name()) == nil {
			methodInfo.HasPointerReceiver = true
		}

		// Auto-detect chainable return type: if the method returns a single
		// non-error type whose pointer has a .Result() → (T, error) method,
		// record it for automatic chaining (fluent API pattern like go-redis).
		if bind.ErrorToResult && len(methodInfo.Signature.Results) == 1 && !methodInfo.Signature.HasErrorReturn {
			retType := sig.Results().At(0).Type()
			if chainMethod := detectChainMethod(retType); chainMethod != "" {
				methodInfo.AutoChainMethod = chainMethod
			}
		}

		tb.Methods = append(tb.Methods, methodInfo)
	}

	// Sort methods for deterministic output
	sort.Slice(tb.Methods, func(i, j int) bool {
		return tb.Methods[i].GoName < tb.Methods[j].GoName
	})

	// Extract struct fields
	if structType, ok := underlying.(*types.Struct); ok {
		for i := 0; i < structType.NumFields(); i++ {
			field := structType.Field(i)
			if !field.Exported() {
				continue
			}
			tb.Fields = append(tb.Fields, &FieldInfo{
				GoName:    field.Name(),
				FunxyName: lcFirst(field.Name()),
				Type:      goTypeToRef(field.Type()),
			})
		}
	}

	return tb, nil
}

// resolveFuncBinding extracts signature information for a function binding.
func (ins *Inspector) resolveFuncBinding(pkg *packages.Package, bind BindSpec) (*FuncBinding, error) {
	scope := pkg.Types.Scope()
	obj := scope.Lookup(bind.Func)
	if obj == nil {
		return nil, fmt.Errorf("function %q not found in package %s", bind.Func, pkg.PkgPath)
	}

	fn, ok := obj.(*types.Func)
	if !ok {
		return nil, fmt.Errorf("%q is not a function in package %s", bind.Func, pkg.PkgPath)
	}

	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return nil, fmt.Errorf("could not get signature for %q", bind.Func)
	}

	fb := &FuncBinding{
		GoName:    bind.Func,
		Signature: extractSignature(sig),
	}

	// Detect generic type parameters
	if tparams := sig.TypeParams(); tparams != nil && tparams.Len() > 0 {
		for i := 0; i < tparams.Len(); i++ {
			tp := tparams.At(i)
			fb.TypeParams = append(fb.TypeParams, TypeParamInfo{
				Name:       tp.Obj().Name(),
				Constraint: types.TypeString(tp.Constraint(), nil),
				IsAny:      isAnyConstraint(tp),
			})
		}

		// Resolve type args: explicit from bind spec, or auto-infer for unconstrained
		typeArgs, subs, err := resolveTypeArgs(fb.TypeParams, bind.TypeArgs)
		if err != nil {
			return nil, fmt.Errorf("generic function %s: %w", bind.Func, err)
		}
		fb.TypeArgs = typeArgs

		// Substitute type params in the signature with concrete types
		fb.Signature = substituteSignature(fb.Signature, subs)
	}

	return fb, nil
}

// resolveConstBinding extracts type information for a constant binding.
func (ins *Inspector) resolveConstBinding(pkg *packages.Package, bind BindSpec) (*ConstBinding, error) {
	scope := pkg.Types.Scope()
	obj := scope.Lookup(bind.Const)
	if obj == nil {
		return nil, fmt.Errorf("constant %q not found in package %s", bind.Const, pkg.PkgPath)
	}

	constObj, ok := obj.(*types.Const)
	if !ok {
		return nil, fmt.Errorf("%q is not a constant in package %s", bind.Const, pkg.PkgPath)
	}

	return resolveConstFromObj(constObj), nil
}

// resolveConstFromObj creates a ConstBinding from a types.Const.
func resolveConstFromObj(obj *types.Const) *ConstBinding {
	return &ConstBinding{
		GoName: obj.Name(),
		Type:   goTypeToRef(obj.Type()),
	}
}

// extractSignature converts a Go types.Signature to our FuncSignature.
func extractSignature(sig *types.Signature) *FuncSignature {
	fs := &FuncSignature{
		IsVariadic: sig.Variadic(),
	}

	// Extract parameters
	params := sig.Params()
	for i := 0; i < params.Len(); i++ {
		param := params.At(i)
		pi := &ParamInfo{
			Name: param.Name(),
			Type: goTypeToRef(param.Type()),
		}

		// Check if first param is context.Context
		if i == 0 && isContextType(param.Type()) {
			fs.HasContextParam = true
		}

		// For variadic, the last param is a slice; extract the element type
		if sig.Variadic() && i == params.Len()-1 {
			pi.IsVariadic = true
			if sliceType, ok := param.Type().(*types.Slice); ok {
				pi.Type = goTypeToRef(sliceType.Elem())
			}
		}

		fs.Params = append(fs.Params, pi)
	}

	// Extract results
	results := sig.Results()
	for i := 0; i < results.Len(); i++ {
		result := results.At(i)
		ri := &ParamInfo{
			Name: result.Name(),
			Type: goTypeToRef(result.Type()),
		}

		// Check if last result is error
		if i == results.Len()-1 && isErrorType(result.Type()) {
			fs.HasErrorReturn = true
		}

		fs.Results = append(fs.Results, ri)
	}

	return fs
}

// goTypeToRef converts a go/types.Type to a GoTypeRef.
func goTypeToRef(t types.Type) GoTypeRef {
	switch t := t.(type) {
	case *types.Basic:
		return basicTypeToRef(t)

	case *types.Named:
		// Special cases
		if isErrorType(t) {
			return GoTypeRef{Kind: GoTypeError, GoString: "error", FunxyType: "String"}
		}
		if isContextType(t) {
			return GoTypeRef{Kind: GoTypeContext, GoString: "context.Context",
				PkgPath: "context", TypeName: "Context", FunxyType: "HostObject"}
		}

		obj := t.Obj()
		pkgPath := ""
		if obj.Pkg() != nil {
			pkgPath = obj.Pkg().Path()
		}

		ref := GoTypeRef{
			Kind:     GoTypeNamed,
			GoString: types.TypeString(t, nil),
			PkgPath:  pkgPath,
			TypeName: obj.Name(),
		}

		// Check underlying type
		switch t.Underlying().(type) {
		case *types.Struct:
			ref.Kind = GoTypeStruct
		case *types.Interface:
			ref.Kind = GoTypeInterface
		}

		ref.FunxyType = "HostObject"
		return ref

	case *types.Pointer:
		elem := goTypeToRef(t.Elem())
		return GoTypeRef{
			Kind:      GoTypePtr,
			GoString:  "*" + elem.GoString,
			ElemType:  &elem,
			FunxyType: elem.FunxyType,
		}

	case *types.Slice:
		// Special case: []byte → Bytes
		if basic, ok := t.Elem().(*types.Basic); ok && basic.Kind() == types.Byte {
			return GoTypeRef{Kind: GoTypeByteSlice, GoString: "[]byte", FunxyType: "Bytes"}
		}
		elem := goTypeToRef(t.Elem())
		funxyElem := elem.FunxyType
		if funxyElem == "" {
			funxyElem = "HostObject"
		}
		return GoTypeRef{
			Kind:      GoTypeSlice,
			GoString:  "[]" + elem.GoString,
			ElemType:  &elem,
			FunxyType: "List<" + funxyElem + ">",
		}

	case *types.Array:
		elem := goTypeToRef(t.Elem())
		funxyElem := elem.FunxyType
		if funxyElem == "" {
			funxyElem = "HostObject"
		}
		return GoTypeRef{
			Kind:      GoTypeArray,
			GoString:  fmt.Sprintf("[%d]%s", t.Len(), elem.GoString),
			ElemType:  &elem,
			FunxyType: "List<" + funxyElem + ">",
		}

	case *types.Map:
		key := goTypeToRef(t.Key())
		val := goTypeToRef(t.Elem())
		return GoTypeRef{
			Kind:      GoTypeMap,
			GoString:  fmt.Sprintf("map[%s]%s", key.GoString, val.GoString),
			KeyType:   &key,
			ElemType:  &val,
			FunxyType: "Map<" + key.FunxyType + ", " + val.FunxyType + ">",
		}

	case *types.Signature:
		sig := extractSignature(t)
		return GoTypeRef{
			Kind:      GoTypeFunc,
			GoString:  types.TypeString(t, nil),
			FunxyType: funxyFuncTypeString(sig),
			FuncSig:   sig,
		}

	case *types.Chan:
		elem := goTypeToRef(t.Elem())
		return GoTypeRef{
			Kind:      GoTypeChan,
			GoString:  types.TypeString(t, nil),
			ElemType:  &elem,
			FunxyType: "HostObject",
		}

	case *types.Interface:
		// Unnamed interface (e.g. interface{} / any)
		if t.NumMethods() == 0 {
			// Treat empty interface as a basic type that accepts any value (via toGoAny)
			return GoTypeRef{Kind: GoTypeBasic, GoString: "any", FunxyType: "HostObject"}
		}
		return GoTypeRef{Kind: GoTypeInterface, GoString: types.TypeString(t, nil), FunxyType: "HostObject"}

	case *types.Struct:
		// Anonymous struct — no direct Funxy equivalent, wrap as HostObject
		return GoTypeRef{Kind: GoTypeStruct, GoString: types.TypeString(t, nil), FunxyType: "HostObject"}

	case *types.TypeParam:
		name := t.Obj().Name()
		return GoTypeRef{
			Kind:           GoTypeTypeParam,
			GoString:       name,
			FunxyType:      strings.ToLower(name),
			TypeParamIndex: t.Index(),
		}

	default:
		return GoTypeRef{Kind: GoTypeNamed, GoString: types.TypeString(t, nil), FunxyType: "HostObject"}
	}
}

// basicTypeToRef maps Go basic types to GoTypeRef.
func basicTypeToRef(t *types.Basic) GoTypeRef {
	ref := GoTypeRef{
		Kind:     GoTypeBasic,
		GoString: t.Name(),
	}

	switch t.Kind() {
	case types.Bool:
		ref.FunxyType = "Bool"
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64:
		ref.FunxyType = "Int"
	case types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
		ref.FunxyType = "Int"
	case types.Float32, types.Float64:
		ref.FunxyType = "Float"
	case types.String:
		ref.FunxyType = "String"
	case types.UntypedBool:
		ref.FunxyType = "Bool"
	case types.UntypedInt:
		ref.FunxyType = "Int"
	case types.UntypedFloat:
		ref.FunxyType = "Float"
	case types.UntypedString:
		ref.FunxyType = "String"
	default:
		ref.FunxyType = "HostObject"
	}

	return ref
}

// isContextType checks if a type is context.Context.
func isContextType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "context" && obj.Name() == "Context"
}

// isErrorType checks if a type is the error interface.
func isErrorType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if ok {
		t = named.Underlying()
	}
	iface, ok := t.(*types.Interface)
	if !ok {
		return false
	}
	return iface.NumMethods() == 1 && iface.Method(0).Name() == "Error"
}

// buildMethodFilter returns a predicate that checks whether a method name
// should be included based on the bind spec's methods/exclude_methods lists.
func buildMethodFilter(bind BindSpec) func(string) bool {
	if len(bind.Methods) > 0 {
		// Whitelist mode
		allowed := make(map[string]bool, len(bind.Methods))
		for _, m := range bind.Methods {
			allowed[m] = true
		}
		return func(name string) bool { return allowed[name] }
	}

	if len(bind.ExcludeMethods) > 0 {
		// Blacklist mode
		excluded := make(map[string]bool, len(bind.ExcludeMethods))
		for _, m := range bind.ExcludeMethods {
			excluded[m] = true
		}
		return func(name string) bool { return !excluded[name] }
	}

	// No filter — include all exported methods
	return func(name string) bool { return true }
}

// makeFunxyMethodName creates a Funxy function name from alias + method name.
// e.g. ("redis", "Get") → "redisGet", ("s3", "PutObject") → "s3PutObject"
func makeFunxyMethodName(alias, method string) string {
	return alias + ucFirst(method)
}

// detectChainMethod checks if a return type has a .Result() method returning (T, error).
// Returns the method name (e.g. "Result") if found, empty string otherwise.
// This detects fluent API patterns like go-redis where methods return command
// objects (*StatusCmd, *StringCmd, etc.) with a .Result() → (T, error) method.
func detectChainMethod(retType types.Type) string {
	// Try pointer type first (most common: *StatusCmd), then the type itself
	for _, t := range []types.Type{retType, types.NewPointer(retType)} {
		mset := types.NewMethodSet(t)
		for _, name := range []string{"Result"} {
			for i := 0; i < mset.Len(); i++ {
				sel := mset.At(i)
				if sel.Obj().Name() != name || !sel.Obj().Exported() {
					continue
				}
				fn, ok := sel.Obj().(*types.Func)
				if !ok {
					continue
				}
				sig := fn.Type().(*types.Signature)
				results := sig.Results()
				// Must return (T, error) — exactly 2 results, last is error
				if results.Len() >= 2 && isErrorType(results.At(results.Len()-1).Type()) {
					return name
				}
			}
		}
	}
	return ""
}

// collectPackagePaths extracts unique Go package import paths from the config.
func collectPackagePaths(cfg *Config) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, dep := range cfg.Deps {
		if !seen[dep.Pkg] {
			paths = append(paths, dep.Pkg)
			seen[dep.Pkg] = true
		}
	}
	return paths
}

// bindTarget returns a human-readable name for a bind spec (for error messages).
func bindTarget(bind BindSpec) string {
	if bind.Type != "" {
		return bind.Type
	}
	return bind.Func
}

// funxyFuncTypeString builds a Funxy function type string from a FuncSignature.
// E.g. "(String, Int) -> Bool" or "() -> Nil".
func funxyFuncTypeString(sig *FuncSignature) string {
	var params []string
	for _, p := range sig.Params {
		ft := p.Type.FunxyType
		if ft == "" {
			ft = "HostObject"
		}
		params = append(params, ft)
	}

	retType := "Nil"
	if len(sig.Results) == 1 {
		retType = sig.Results[0].Type.FunxyType
		if retType == "" {
			retType = "HostObject"
		}
	} else if len(sig.Results) > 1 {
		retType = "HostObject"
	}

	return "(" + strings.Join(params, ", ") + ") -> " + retType
}

// isAnyConstraint checks if a type parameter's constraint is interface{}/any (unconstrained).
func isAnyConstraint(tp *types.TypeParam) bool {
	constraint := tp.Constraint()
	if constraint == nil {
		return true
	}
	iface, ok := constraint.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	return iface.NumMethods() == 0 && !iface.IsComparable() && iface.NumEmbeddeds() == 0
}

// resolveTypeArgs resolves the Go type argument strings for a generic binding.
// If explicit typeArgs are provided in the bind spec, they are used (with validation).
// Otherwise, unconstrained (any) params get "any" automatically; constrained params cause an error.
// Returns the type arg strings for the Go call, and the substitution GoTypeRefs.
func resolveTypeArgs(typeParams []TypeParamInfo, explicitArgs []string) ([]string, []GoTypeRef, error) {
	if len(explicitArgs) > 0 {
		if len(explicitArgs) != len(typeParams) {
			return nil, nil, fmt.Errorf(
				"type_args count mismatch: got %d, need %d", len(explicitArgs), len(typeParams))
		}
		refs := make([]GoTypeRef, len(explicitArgs))
		for i, arg := range explicitArgs {
			refs[i] = parseTypeArgRef(arg)
		}
		return explicitArgs, refs, nil
	}

	// Auto-infer: all must be unconstrained
	args := make([]string, len(typeParams))
	refs := make([]GoTypeRef, len(typeParams))
	for i, tp := range typeParams {
		if !tp.IsAny {
			return nil, nil, fmt.Errorf(
				"type parameter %s has constraint %q — add type_args to specify concrete types",
				tp.Name, tp.Constraint)
		}
		args[i] = "any"
		refs[i] = GoTypeRef{Kind: GoTypeBasic, GoString: "any", FunxyType: "HostObject"}
	}
	return args, refs, nil
}

// parseTypeArgRef converts a type arg string (e.g. "string", "int", "any") to a GoTypeRef.
func parseTypeArgRef(s string) GoTypeRef {
	switch s {
	case "string":
		return GoTypeRef{Kind: GoTypeBasic, GoString: "string", FunxyType: "String"}
	case "int", "int8", "int16", "int32", "int64":
		return GoTypeRef{Kind: GoTypeBasic, GoString: s, FunxyType: "Int"}
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return GoTypeRef{Kind: GoTypeBasic, GoString: s, FunxyType: "Int"}
	case "float32", "float64":
		return GoTypeRef{Kind: GoTypeBasic, GoString: s, FunxyType: "Float"}
	case "bool":
		return GoTypeRef{Kind: GoTypeBasic, GoString: "bool", FunxyType: "Bool"}
	case "any":
		return GoTypeRef{Kind: GoTypeBasic, GoString: "any", FunxyType: "HostObject"}
	case "[]byte":
		return GoTypeRef{Kind: GoTypeByteSlice, GoString: "[]byte", FunxyType: "Bytes"}
	default:
		return GoTypeRef{Kind: GoTypeNamed, GoString: s, FunxyType: "HostObject"}
	}
}

// parseBasicGoType converts a type arg string to a go/types.Type.
// Used for types.Instantiate when resolving generic types.
func parseBasicGoType(s string) types.Type {
	switch s {
	case "string":
		return types.Typ[types.String]
	case "int":
		return types.Typ[types.Int]
	case "int8":
		return types.Typ[types.Int8]
	case "int16":
		return types.Typ[types.Int16]
	case "int32":
		return types.Typ[types.Int32]
	case "int64":
		return types.Typ[types.Int64]
	case "uint":
		return types.Typ[types.Uint]
	case "uint8":
		return types.Typ[types.Uint8]
	case "uint16":
		return types.Typ[types.Uint16]
	case "uint32":
		return types.Typ[types.Uint32]
	case "uint64":
		return types.Typ[types.Uint64]
	case "float32":
		return types.Typ[types.Float32]
	case "float64":
		return types.Typ[types.Float64]
	case "bool":
		return types.Typ[types.Bool]
	case "any":
		return types.Universe.Lookup("any").Type()
	default:
		// Named types — fall back to any (the codegen will use the string as-is)
		return types.Universe.Lookup("any").Type()
	}
}

// substituteSignature replaces GoTypeTypeParam refs in a signature with concrete types.
func substituteSignature(sig *FuncSignature, subs []GoTypeRef) *FuncSignature {
	if len(subs) == 0 {
		return sig
	}
	result := &FuncSignature{
		IsVariadic:      sig.IsVariadic,
		HasContextParam: sig.HasContextParam,
		HasErrorReturn:  sig.HasErrorReturn,
	}
	for _, p := range sig.Params {
		np := &ParamInfo{
			Name:       p.Name,
			Type:       substituteTypeRef(p.Type, subs),
			IsVariadic: p.IsVariadic,
		}
		result.Params = append(result.Params, np)
	}
	for _, r := range sig.Results {
		nr := &ParamInfo{
			Name: r.Name,
			Type: substituteTypeRef(r.Type, subs),
		}
		result.Results = append(result.Results, nr)
	}
	return result
}

// substituteTypeRef recursively replaces GoTypeTypeParam refs in a type reference.
func substituteTypeRef(ref GoTypeRef, subs []GoTypeRef) GoTypeRef {
	if ref.Kind == GoTypeTypeParam {
		if ref.TypeParamIndex < len(subs) {
			return subs[ref.TypeParamIndex]
		}
		// Fallback: treat as any
		return GoTypeRef{Kind: GoTypeBasic, GoString: "any", FunxyType: "HostObject"}
	}

	// Recurse into composite types
	if ref.ElemType != nil {
		elem := substituteTypeRef(*ref.ElemType, subs)
		ref.ElemType = &elem
		// Update GoString and FunxyType for slices/arrays/pointers
		switch ref.Kind {
		case GoTypeSlice:
			ref.GoString = "[]" + elem.GoString
			ref.FunxyType = "List<" + elem.FunxyType + ">"
		case GoTypeArray:
			// Keep existing format
		case GoTypePtr:
			ref.GoString = "*" + elem.GoString
			ref.FunxyType = elem.FunxyType
		}
	}
	if ref.KeyType != nil {
		key := substituteTypeRef(*ref.KeyType, subs)
		ref.KeyType = &key
		if ref.Kind == GoTypeMap && ref.ElemType != nil {
			ref.GoString = fmt.Sprintf("map[%s]%s", key.GoString, ref.ElemType.GoString)
			ref.FunxyType = fmt.Sprintf("Map<%s, %s>", key.FunxyType, ref.ElemType.FunxyType)
		}
	}
	if ref.FuncSig != nil {
		subSig := substituteSignature(ref.FuncSig, subs)
		ref.FuncSig = subSig
	}
	return ref
}

// ucFirst uppercases the first rune of a string.
func ucFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	if runes[0] >= 'a' && runes[0] <= 'z' {
		runes[0] -= 32
	}
	return string(runes)
}

// lcFirst lowercases the first rune of a string.
func lcFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	if runes[0] >= 'A' && runes[0] <= 'Z' {
		runes[0] += 32
	}
	return string(runes)
}

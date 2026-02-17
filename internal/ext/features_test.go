package ext

import (
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/funvibe/funxy/internal/typesystem"
)

// =============================================================================
// Unit tests: goTypeToRef — map types
// =============================================================================

func TestGoTypeToRef_MapStringInt(t *testing.T) {
	// map[string]int → Map<String, Int>
	m := types.NewMap(types.Typ[types.String], types.Typ[types.Int])
	ref := goTypeToRef(m)

	if ref.Kind != GoTypeMap {
		t.Errorf("Kind = %v, want GoTypeMap", ref.Kind)
	}
	if ref.FunxyType != "Map<String, Int>" {
		t.Errorf("FunxyType = %q, want %q", ref.FunxyType, "Map<String, Int>")
	}
	if ref.KeyType == nil {
		t.Fatal("KeyType is nil")
	}
	if ref.KeyType.FunxyType != "String" {
		t.Errorf("KeyType.FunxyType = %q, want String", ref.KeyType.FunxyType)
	}
	if ref.ElemType == nil {
		t.Fatal("ElemType is nil")
	}
	if ref.ElemType.FunxyType != "Int" {
		t.Errorf("ElemType.FunxyType = %q, want Int", ref.ElemType.FunxyType)
	}
	if ref.GoString != "map[string]int" {
		t.Errorf("GoString = %q, want map[string]int", ref.GoString)
	}
}

func TestGoTypeToRef_MapIntString(t *testing.T) {
	// map[int]string → Map<Int, String>
	m := types.NewMap(types.Typ[types.Int], types.Typ[types.String])
	ref := goTypeToRef(m)

	if ref.Kind != GoTypeMap {
		t.Errorf("Kind = %v, want GoTypeMap", ref.Kind)
	}
	if ref.FunxyType != "Map<Int, String>" {
		t.Errorf("FunxyType = %q, want %q", ref.FunxyType, "Map<Int, String>")
	}
}

func TestGoTypeToRef_MapStringBool(t *testing.T) {
	// map[string]bool → Map<String, Bool>
	m := types.NewMap(types.Typ[types.String], types.Typ[types.Bool])
	ref := goTypeToRef(m)

	if ref.FunxyType != "Map<String, Bool>" {
		t.Errorf("FunxyType = %q, want %q", ref.FunxyType, "Map<String, Bool>")
	}
}

func TestGoTypeToRef_MapStringFloat64(t *testing.T) {
	// map[string]float64 → Map<String, Float>
	m := types.NewMap(types.Typ[types.String], types.Typ[types.Float64])
	ref := goTypeToRef(m)

	if ref.FunxyType != "Map<String, Float>" {
		t.Errorf("FunxyType = %q, want %q", ref.FunxyType, "Map<String, Float>")
	}
}

func TestGoTypeToRef_MapStringString(t *testing.T) {
	// map[string]string → Map<String, String> (NOT "Record")
	m := types.NewMap(types.Typ[types.String], types.Typ[types.String])
	ref := goTypeToRef(m)

	if ref.Kind != GoTypeMap {
		t.Errorf("Kind = %v, want GoTypeMap", ref.Kind)
	}
	if ref.FunxyType != "Map<String, String>" {
		t.Errorf("FunxyType = %q, want %q (map[string]string should be Map, not Record)", ref.FunxyType, "Map<String, String>")
	}
}

func TestGoTypeToRef_MapStringAny(t *testing.T) {
	// map[string]interface{} → Map<String, HostObject>
	// Empty interface maps to HostObject (uses toGoAny converter)
	any := types.NewInterfaceType(nil, nil)
	m := types.NewMap(types.Typ[types.String], any)
	ref := goTypeToRef(m)

	if ref.Kind != GoTypeMap {
		t.Errorf("Kind = %v, want GoTypeMap", ref.Kind)
	}
	if ref.FunxyType != "Map<String, HostObject>" {
		t.Errorf("FunxyType = %q, want %q", ref.FunxyType, "Map<String, HostObject>")
	}
}

// Regression: all map types must be consistent, no special "Record" for string keys.
func TestGoTypeToRef_AllMapsConsistent(t *testing.T) {
	keyTypes := []types.BasicKind{types.String, types.Int, types.Int64, types.Float64, types.Bool}
	for _, kk := range keyTypes {
		for _, vk := range keyTypes {
			m := types.NewMap(types.Typ[kk], types.Typ[vk])
			ref := goTypeToRef(m)
			if ref.Kind != GoTypeMap {
				t.Errorf("map[%s]%s: Kind = %v, want GoTypeMap", types.Typ[kk], types.Typ[vk], ref.Kind)
			}
			if !strings.HasPrefix(ref.FunxyType, "Map<") {
				t.Errorf("map[%s]%s: FunxyType = %q, want Map<...>", types.Typ[kk], types.Typ[vk], ref.FunxyType)
			}
		}
	}
}

// =============================================================================
// Unit tests: goTypeToRef — func types
// =============================================================================

func TestGoTypeToRef_FuncStringBool(t *testing.T) {
	// func(string) bool
	params := types.NewTuple(types.NewVar(0, nil, "s", types.Typ[types.String]))
	results := types.NewTuple(types.NewVar(0, nil, "", types.Typ[types.Bool]))
	sig := types.NewSignatureType(nil, nil, nil, params, results, false)

	ref := goTypeToRef(sig)

	if ref.Kind != GoTypeFunc {
		t.Errorf("Kind = %v, want GoTypeFunc", ref.Kind)
	}
	if ref.FunxyType != "(String) -> Bool" {
		t.Errorf("FunxyType = %q, want %q", ref.FunxyType, "(String) -> Bool")
	}
	if ref.FuncSig == nil {
		t.Fatal("FuncSig is nil")
	}
	if len(ref.FuncSig.Params) != 1 {
		t.Fatalf("FuncSig.Params = %d, want 1", len(ref.FuncSig.Params))
	}
	if ref.FuncSig.Params[0].Type.FunxyType != "String" {
		t.Errorf("param type = %q, want String", ref.FuncSig.Params[0].Type.FunxyType)
	}
	if len(ref.FuncSig.Results) != 1 {
		t.Fatalf("FuncSig.Results = %d, want 1", len(ref.FuncSig.Results))
	}
	if ref.FuncSig.Results[0].Type.FunxyType != "Bool" {
		t.Errorf("result type = %q, want Bool", ref.FuncSig.Results[0].Type.FunxyType)
	}
}

func TestGoTypeToRef_FuncNoReturn(t *testing.T) {
	// func(int, int)
	params := types.NewTuple(
		types.NewVar(0, nil, "a", types.Typ[types.Int]),
		types.NewVar(0, nil, "b", types.Typ[types.Int]),
	)
	results := types.NewTuple()
	sig := types.NewSignatureType(nil, nil, nil, params, results, false)

	ref := goTypeToRef(sig)

	if ref.FuncSig == nil {
		t.Fatal("FuncSig is nil")
	}
	if len(ref.FuncSig.Params) != 2 {
		t.Errorf("FuncSig.Params = %d, want 2", len(ref.FuncSig.Params))
	}
	if len(ref.FuncSig.Results) != 0 {
		t.Errorf("FuncSig.Results = %d, want 0", len(ref.FuncSig.Results))
	}
}

func TestGoTypeToRef_FuncMultiReturn(t *testing.T) {
	// func(string) (int, string)
	params := types.NewTuple(types.NewVar(0, nil, "s", types.Typ[types.String]))
	results := types.NewTuple(
		types.NewVar(0, nil, "", types.Typ[types.Int]),
		types.NewVar(0, nil, "", types.Typ[types.String]),
	)
	sig := types.NewSignatureType(nil, nil, nil, params, results, false)

	ref := goTypeToRef(sig)

	if ref.FuncSig == nil {
		t.Fatal("FuncSig is nil")
	}
	if len(ref.FuncSig.Results) != 2 {
		t.Errorf("FuncSig.Results = %d, want 2", len(ref.FuncSig.Results))
	}
}

// =============================================================================
// Unit tests: goTypeToRef — slices
// =============================================================================

func TestGoTypeToRef_SliceInt(t *testing.T) {
	// []int → List<Int>
	s := types.NewSlice(types.Typ[types.Int])
	ref := goTypeToRef(s)

	if ref.Kind != GoTypeSlice {
		t.Errorf("Kind = %v, want GoTypeSlice", ref.Kind)
	}
	if ref.FunxyType != "List<Int>" {
		t.Errorf("FunxyType = %q, want List<Int>", ref.FunxyType)
	}
}

func TestGoTypeToRef_ByteSlice(t *testing.T) {
	// []byte → Bytes (special case)
	s := types.NewSlice(types.Typ[types.Byte])
	ref := goTypeToRef(s)

	if ref.Kind != GoTypeByteSlice {
		t.Errorf("Kind = %v, want GoTypeByteSlice", ref.Kind)
	}
	if ref.FunxyType != "Bytes" {
		t.Errorf("FunxyType = %q, want Bytes", ref.FunxyType)
	}
}

// =============================================================================
// Validation: FunxyType must always be a valid Funxy type
// =============================================================================

// validFunxyBasicTypes are the only allowed non-parametric FunxyType values.
var validFunxyBasicTypes = map[string]bool{
	"Int":        true,
	"Float":      true,
	"Bool":       true,
	"String":     true,
	"Bytes":      true,
	"Nil":        true,
	"HostObject": true,
}

// isValidFunxyType checks that a FunxyType string is a legitimate Funxy type.
// Allowed forms:
//   - Basic: "Int", "Float", "Bool", "String", "Bytes", "Nil", "HostObject"
//   - Parametric: "List<...>", "Map<...>", "Result<...>"
//   - Function: "(...)  -> ..."
//   - Lowercase single-letter type variables from generics: "t", "u", etc.
func isValidFunxyType(ft string) bool {
	if ft == "" {
		return false
	}
	if validFunxyBasicTypes[ft] {
		return true
	}
	if strings.HasPrefix(ft, "List<") || strings.HasPrefix(ft, "Map<") || strings.HasPrefix(ft, "Result<") {
		return true
	}
	if strings.HasPrefix(ft, "(") && strings.Contains(ft, "->") {
		return true
	}
	// Lowercase type variable from generics (e.g. "t", "u")
	if len(ft) <= 2 && ft == strings.ToLower(ft) {
		return true
	}
	return false
}

// TestFunxyType_AllGoBasicTypes ensures goTypeToRef produces valid FunxyType
// for every Go basic type. This would have caught "Any", "Fn", "Record" bugs.
func TestFunxyType_AllGoBasicTypes(t *testing.T) {
	goTypes := []types.Type{
		types.Typ[types.Bool],
		types.Typ[types.Int],
		types.Typ[types.Int8],
		types.Typ[types.Int16],
		types.Typ[types.Int32],
		types.Typ[types.Int64],
		types.Typ[types.Uint],
		types.Typ[types.Uint8],
		types.Typ[types.Uint16],
		types.Typ[types.Uint32],
		types.Typ[types.Uint64],
		types.Typ[types.Float32],
		types.Typ[types.Float64],
		types.Typ[types.String],
		types.Typ[types.UntypedNil],
	}

	for _, gt := range goTypes {
		ref := goTypeToRef(gt)
		if !isValidFunxyType(ref.FunxyType) {
			t.Errorf("goTypeToRef(%v) produced invalid FunxyType %q", gt, ref.FunxyType)
		}
	}
}

// TestFunxyType_CompositeTypes ensures composed types produce valid FunxyType.
func TestFunxyType_CompositeTypes(t *testing.T) {
	composites := map[string]types.Type{
		"[]int":             types.NewSlice(types.Typ[types.Int]),
		"[]byte":            types.NewSlice(types.Typ[types.Byte]),
		"map[string]int":    types.NewMap(types.Typ[types.String], types.Typ[types.Int]),
		"map[string]any":    types.NewMap(types.Typ[types.String], types.Universe.Lookup("any").Type()),
		"*int":              types.NewPointer(types.Typ[types.Int]),
		"func(string) bool": types.NewSignatureType(nil, nil, nil, types.NewTuple(types.NewVar(0, nil, "s", types.Typ[types.String])), types.NewTuple(types.NewVar(0, nil, "", types.Typ[types.Bool])), false),
		"func()":            types.NewSignatureType(nil, nil, nil, types.NewTuple(), types.NewTuple(), false),
		"interface{}":       types.Universe.Lookup("any").Type(),
		"struct{}":          types.NewStruct(nil, nil),
	}

	for name, gt := range composites {
		ref := goTypeToRef(gt)
		if !isValidFunxyType(ref.FunxyType) {
			t.Errorf("goTypeToRef(%s) produced invalid FunxyType %q", name, ref.FunxyType)
		}
	}
}

// =============================================================================
// Unit tests: goTypeRefToFunxyType — virtual package type mapping
// =============================================================================

func TestGoTypeRefToFunxyType_MapStringInt(t *testing.T) {
	ref := GoTypeRef{
		Kind:      GoTypeMap,
		FunxyType: "Map<String, Int>",
		KeyType:   &GoTypeRef{Kind: GoTypeBasic, FunxyType: "String"},
		ElemType:  &GoTypeRef{Kind: GoTypeBasic, FunxyType: "Int"},
	}
	ft := goTypeRefToFunxyType(ref)

	tapp, ok := ft.(typesystem.TApp)
	if !ok {
		t.Fatalf("expected TApp, got %T: %v", ft, ft)
	}
	tcon, ok := tapp.Constructor.(typesystem.TCon)
	if !ok || tcon.Name != "Map" {
		t.Errorf("Constructor = %v, want TCon{Map}", tapp.Constructor)
	}
	if len(tapp.Args) != 2 {
		t.Fatalf("Args len = %d, want 2", len(tapp.Args))
	}
	if k, ok := tapp.Args[0].(typesystem.TCon); !ok || k.Name != "String" {
		t.Errorf("key type = %v, want TCon{String}", tapp.Args[0])
	}
	if v, ok := tapp.Args[1].(typesystem.TCon); !ok || v.Name != "Int" {
		t.Errorf("val type = %v, want TCon{Int}", tapp.Args[1])
	}
}

func TestGoTypeRefToFunxyType_MapIntFloat(t *testing.T) {
	ref := GoTypeRef{
		Kind:      GoTypeMap,
		FunxyType: "Map<Int, Float>",
		KeyType:   &GoTypeRef{Kind: GoTypeBasic, FunxyType: "Int"},
		ElemType:  &GoTypeRef{Kind: GoTypeBasic, FunxyType: "Float"},
	}
	ft := goTypeRefToFunxyType(ref)

	tapp, ok := ft.(typesystem.TApp)
	if !ok {
		t.Fatalf("expected TApp, got %T", ft)
	}
	if tcon, ok := tapp.Constructor.(typesystem.TCon); !ok || tcon.Name != "Map" {
		t.Errorf("Constructor = %v, want TCon{Map}", tapp.Constructor)
	}
}

func TestGoTypeRefToFunxyType_FnWithSig(t *testing.T) {
	ref := GoTypeRef{
		Kind:      GoTypeFunc,
		FunxyType: "(String, Int) -> Bool",
		FuncSig: &FuncSignature{
			Params: []*ParamInfo{
				{Name: "s", Type: GoTypeRef{Kind: GoTypeBasic, FunxyType: "String"}},
			},
			Results: []*ParamInfo{
				{Type: GoTypeRef{Kind: GoTypeBasic, FunxyType: "Bool"}},
			},
		},
	}
	ft := goTypeRefToFunxyType(ref)

	tfunc, ok := ft.(typesystem.TFunc)
	if !ok {
		t.Fatalf("expected TFunc, got %T: %v", ft, ft)
	}
	if len(tfunc.Params) != 1 {
		t.Fatalf("Params = %d, want 1", len(tfunc.Params))
	}
	if p, ok := tfunc.Params[0].(typesystem.TCon); !ok || p.Name != "String" {
		t.Errorf("param = %v, want TCon{String}", tfunc.Params[0])
	}
	if r, ok := tfunc.ReturnType.(typesystem.TCon); !ok || r.Name != "Bool" {
		t.Errorf("return = %v, want TCon{Bool}", tfunc.ReturnType)
	}
}

func TestGoTypeRefToFunxyType_FnNoSig(t *testing.T) {
	ref := GoTypeRef{
		Kind:      GoTypeFunc,
		FunxyType: "() -> Nil",
		FuncSig:   nil,
	}
	ft := goTypeRefToFunxyType(ref)

	tcon, ok := ft.(typesystem.TCon)
	if !ok {
		t.Fatalf("expected TCon, got %T", ft)
	}
	if tcon.Name != "HostObject" {
		t.Errorf("type = %q, want HostObject", tcon.Name)
	}
}

func TestGoTypeRefToFunxyType_ListInt(t *testing.T) {
	ref := GoTypeRef{
		Kind:      GoTypeSlice,
		FunxyType: "List<Int>",
		ElemType:  &GoTypeRef{Kind: GoTypeBasic, FunxyType: "Int"},
	}
	ft := goTypeRefToFunxyType(ref)

	tapp, ok := ft.(typesystem.TApp)
	if !ok {
		t.Fatalf("expected TApp, got %T", ft)
	}
	if tcon, ok := tapp.Constructor.(typesystem.TCon); !ok || tcon.Name != "List" {
		t.Errorf("Constructor = %v, want TCon{List}", tapp.Constructor)
	}
}

func TestGoTypeRefToFunxyType_Primitives(t *testing.T) {
	cases := []struct {
		ref  GoTypeRef
		want string
	}{
		{GoTypeRef{Kind: GoTypeBasic, FunxyType: "Int"}, "Int"},
		{GoTypeRef{Kind: GoTypeBasic, FunxyType: "Float"}, "Float"},
		{GoTypeRef{Kind: GoTypeBasic, FunxyType: "Bool"}, "Bool"},
		{GoTypeRef{Kind: GoTypeBasic, FunxyType: "String"}, "String"},
		{GoTypeRef{Kind: GoTypeByteSlice, FunxyType: "Bytes"}, "Bytes"},
		{GoTypeRef{Kind: GoTypeNamed, FunxyType: "HostObject"}, "HostObject"},
		{GoTypeRef{Kind: GoTypeError, FunxyType: "String"}, "String"},
	}
	for _, tc := range cases {
		ft := goTypeRefToFunxyType(tc.ref)
		tcon, ok := ft.(typesystem.TCon)
		if !ok {
			t.Errorf("%v: expected TCon, got %T", tc.ref.Kind, ft)
			continue
		}
		if tcon.Name != tc.want {
			t.Errorf("Kind=%v: Name = %q, want %q", tc.ref.Kind, tcon.Name, tc.want)
		}
	}
}

// =============================================================================
// Unit tests: config validation for const bindings
// =============================================================================

func TestParseConfig_ValidConstBinding(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind:
      - const: StatusOK
        as: httpStatusOK
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bind := cfg.Deps[0].Bind[0]
	if bind.Const != "StatusOK" {
		t.Errorf("const = %q, want StatusOK", bind.Const)
	}
	if bind.As != "httpStatusOK" {
		t.Errorf("as = %q, want httpStatusOK", bind.As)
	}
}

func TestParseConfig_ErrorConstAndType(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind:
      - const: StatusOK
        type: Client
        as: thing
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for const + type")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, expected to mention 'mutually exclusive'", err.Error())
	}
}

func TestParseConfig_ErrorConstAndFunc(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind:
      - const: StatusOK
        func: New
        as: thing
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for const + func")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, expected to mention 'mutually exclusive'", err.Error())
	}
}

func TestParseConfig_ErrorConstWithMethods(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind:
      - const: StatusOK
        as: httpStatusOK
        methods: [Foo]
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for const with methods")
	}
	if !strings.Contains(err.Error(), "const bindings only support") {
		t.Errorf("error = %q, expected to mention 'const bindings only support'", err.Error())
	}
}

func TestParseConfig_ErrorConstWithErrorToResult(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind:
      - const: StatusOK
        as: httpStatusOK
        error_to_result: true
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for const with error_to_result")
	}
}

func TestParseConfig_ErrorConstWithSkipContext(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind:
      - const: StatusOK
        as: httpStatusOK
        skip_context: true
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for const with skip_context")
	}
}

func TestParseConfig_ErrorConstWithChainResult(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind:
      - const: StatusOK
        as: httpStatusOK
        chain_result: Result
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for const with chain_result")
	}
}

func TestParseConfig_ErrorConstNoAs(t *testing.T) {
	yaml := `
deps:
  - pkg: github.com/example/pkg
    bind:
      - const: StatusOK
`
	_, err := ParseConfig([]byte(yaml), "test.yaml")
	if err == nil {
		t.Fatal("expected error for const without as")
	}
}

// =============================================================================
// Unit tests: codegen output patterns
// =============================================================================

func TestCodegen_MapParamGeneratesToGoMap(t *testing.T) {
	// Construct an InspectResult with a function that takes map[string]int.
	result := &InspectResult{
		Bindings: []*ResolvedBinding{
			{
				Spec:          BindSpec{Func: "DoSomething", As: "doSomething"},
				Dep:           Dep{Pkg: "github.com/example/pkg", Version: "v1.0.0", As: "pkg"},
				GoPackagePath: "github.com/example/pkg",
				FuncBinding: &FuncBinding{
					GoName: "DoSomething",
					Signature: &FuncSignature{
						Params: []*ParamInfo{
							{
								Name: "data",
								Type: GoTypeRef{
									Kind:      GoTypeMap,
									GoString:  "map[string]int",
									FunxyType: "Map<String, Int>",
									KeyType:   &GoTypeRef{Kind: GoTypeBasic, GoString: "string", FunxyType: "String"},
									ElemType:  &GoTypeRef{Kind: GoTypeBasic, GoString: "int", FunxyType: "Int"},
								},
							},
						},
						Results: []*ParamInfo{},
					},
				},
			},
		},
	}

	codegen := NewCodeGenerator("parser")
	files, err := codegen.Generate(result)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var bindingContent string
	for _, f := range files {
		if strings.HasPrefix(f.Filename, "ext_") {
			bindingContent = f.Content
			break
		}
	}
	if bindingContent == "" {
		t.Fatal("no binding file generated")
	}

	t.Logf("--- binding ---\n%s", bindingContent)

	// Must use toGoMap, not toGoHost or special Record handling
	if !strings.Contains(bindingContent, "toGoMap[string, int]") {
		t.Error("binding should use toGoMap[string, int] for map[string]int parameter")
	}
}

func TestCodegen_MapStringStringParam(t *testing.T) {
	// Regression: map[string]string must use toGoMap, not Record handling
	result := &InspectResult{
		Bindings: []*ResolvedBinding{
			{
				Spec:          BindSpec{Func: "Process", As: "process"},
				Dep:           Dep{Pkg: "github.com/example/pkg", Version: "v1.0.0", As: "pkg"},
				GoPackagePath: "github.com/example/pkg",
				FuncBinding: &FuncBinding{
					GoName: "Process",
					Signature: &FuncSignature{
						Params: []*ParamInfo{
							{
								Name: "headers",
								Type: GoTypeRef{
									Kind:      GoTypeMap,
									GoString:  "map[string]string",
									FunxyType: "Map<String, String>",
									KeyType:   &GoTypeRef{Kind: GoTypeBasic, GoString: "string", FunxyType: "String"},
									ElemType:  &GoTypeRef{Kind: GoTypeBasic, GoString: "string", FunxyType: "String"},
								},
							},
						},
						Results: []*ParamInfo{},
					},
				},
			},
		},
	}

	codegen := NewCodeGenerator("parser")
	files, err := codegen.Generate(result)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var bindingContent string
	for _, f := range files {
		if strings.HasPrefix(f.Filename, "ext_") {
			bindingContent = f.Content
			break
		}
	}

	if !strings.Contains(bindingContent, "toGoMap[string, string]") {
		t.Error("map[string]string should use toGoMap[string, string], not Record conversion")
	}
}

func TestCodegen_CallbackParam(t *testing.T) {
	// Function taking func(string) bool callback
	result := &InspectResult{
		Bindings: []*ResolvedBinding{
			{
				Spec:          BindSpec{Func: "Filter", As: "filter"},
				Dep:           Dep{Pkg: "github.com/example/pkg", Version: "v1.0.0", As: "pkg"},
				GoPackagePath: "github.com/example/pkg",
				FuncBinding: &FuncBinding{
					GoName: "Filter",
					Signature: &FuncSignature{
						Params: []*ParamInfo{
							{
								Name: "items",
								Type: GoTypeRef{
									Kind:      GoTypeSlice,
									GoString:  "[]string",
									FunxyType: "List<String>",
									ElemType:  &GoTypeRef{Kind: GoTypeBasic, GoString: "string", FunxyType: "String"},
								},
							},
							{
								Name: "predicate",
								Type: GoTypeRef{
									Kind:      GoTypeFunc,
									GoString:  "func(string) bool",
									FunxyType: "(String) -> Bool",
									FuncSig: &FuncSignature{
										Params: []*ParamInfo{
											{Name: "s", Type: GoTypeRef{Kind: GoTypeBasic, GoString: "string", FunxyType: "String"}},
										},
										Results: []*ParamInfo{
											{Type: GoTypeRef{Kind: GoTypeBasic, GoString: "bool", FunxyType: "Bool"}},
										},
									},
								},
							},
						},
						Results: []*ParamInfo{
							{Type: GoTypeRef{Kind: GoTypeSlice, GoString: "[]string", FunxyType: "List<String>",
								ElemType: &GoTypeRef{Kind: GoTypeBasic, GoString: "string", FunxyType: "String"}}},
						},
					},
				},
			},
		},
	}

	codegen := NewCodeGenerator("parser")
	files, err := codegen.Generate(result)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var bindingContent string
	for _, f := range files {
		if strings.HasPrefix(f.Filename, "ext_") {
			bindingContent = f.Content
			break
		}
	}
	if bindingContent == "" {
		t.Fatal("no binding file generated")
	}

	t.Logf("--- binding ---\n%s", bindingContent)

	// Must generate callback wrapper: a func literal that calls ev.ApplyFunction
	if !strings.Contains(bindingContent, "ApplyFunction") {
		t.Error("callback parameter should generate ev.ApplyFunction call")
	}
	if !strings.Contains(bindingContent, "toFunxy(p0)") {
		t.Error("callback wrapper should convert Go args to Funxy via toFunxy")
	}
	if !strings.Contains(bindingContent, "func(") {
		t.Error("callback should generate a func literal")
	}
}

func TestCodegen_ConstValue(t *testing.T) {
	result := &InspectResult{
		Bindings: []*ResolvedBinding{
			{
				Spec:          BindSpec{Const: "StatusOK", As: "httpStatusOK"},
				Dep:           Dep{Pkg: "github.com/example/http", Version: "v1.0.0", As: "http"},
				GoPackagePath: "github.com/example/http",
				ConstBinding: &ConstBinding{
					GoName: "StatusOK",
					Type:   GoTypeRef{Kind: GoTypeBasic, GoString: "int", FunxyType: "Int"},
				},
			},
		},
	}

	codegen := NewCodeGenerator("parser")
	files, err := codegen.Generate(result)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var bindingContent string
	for _, f := range files {
		if strings.HasPrefix(f.Filename, "ext_") {
			bindingContent = f.Content
			break
		}
	}
	if bindingContent == "" {
		t.Fatal("no binding file generated")
	}

	t.Logf("--- binding ---\n%s", bindingContent)

	// Constant should be registered as a direct value, not a function
	if !strings.Contains(bindingContent, "httpStatusOK") {
		t.Error("binding should contain httpStatusOK")
	}
	if !strings.Contains(bindingContent, "toFunxy(int(") {
		t.Error("binding should cast constant to int via toFunxy(int(...))")
	}
	if !strings.Contains(bindingContent, "StatusOK") {
		t.Error("binding should reference the Go constant StatusOK")
	}
}

func TestCodegen_FieldGetters(t *testing.T) {
	result := &InspectResult{
		Bindings: []*ResolvedBinding{
			{
				Spec:          BindSpec{Type: "Options", As: "opts"},
				Dep:           Dep{Pkg: "github.com/example/pkg", Version: "v1.0.0", As: "pkg"},
				GoPackagePath: "github.com/example/pkg",
				TypeBinding: &TypeBinding{
					GoName:   "Options",
					IsStruct: true,
					Methods:  []*MethodInfo{},
					Fields: []*FieldInfo{
						{GoName: "Host", FunxyName: "host", Type: GoTypeRef{Kind: GoTypeBasic, GoString: "string", FunxyType: "String"}},
						{GoName: "Port", FunxyName: "port", Type: GoTypeRef{Kind: GoTypeBasic, GoString: "int", FunxyType: "Int"}},
						{GoName: "Timeout", FunxyName: "timeout", Type: GoTypeRef{Kind: GoTypeBasic, GoString: "int64", FunxyType: "Int"}},
					},
				},
			},
		},
	}

	codegen := NewCodeGenerator("parser")
	files, err := codegen.Generate(result)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var bindingContent string
	for _, f := range files {
		if strings.HasPrefix(f.Filename, "ext_") {
			bindingContent = f.Content
			break
		}
	}
	if bindingContent == "" {
		t.Fatal("no binding file generated")
	}

	t.Logf("--- binding ---\n%s", bindingContent)

	// Field getters: optsHost, optsPort, optsTimeout
	for _, want := range []string{"optsHost", "optsPort", "optsTimeout"} {
		if !strings.Contains(bindingContent, `"`+want+`"`) {
			t.Errorf("binding should contain field getter %q", want)
		}
	}
	// Should access struct fields
	if !strings.Contains(bindingContent, ".Host") {
		t.Error("getter should access .Host field")
	}
	if !strings.Contains(bindingContent, ".Port") {
		t.Error("getter should access .Port field")
	}
}

func TestCodegen_Constructor(t *testing.T) {
	result := &InspectResult{
		Bindings: []*ResolvedBinding{
			{
				Spec:          BindSpec{Type: "Options", As: "opts", Constructor: true},
				Dep:           Dep{Pkg: "github.com/example/pkg", Version: "v1.0.0", As: "pkg"},
				GoPackagePath: "github.com/example/pkg",
				TypeBinding: &TypeBinding{
					GoName:   "Options",
					IsStruct: true,
					Methods:  []*MethodInfo{},
					Fields: []*FieldInfo{
						{GoName: "Addr", FunxyName: "addr", Type: GoTypeRef{Kind: GoTypeBasic, GoString: "string", FunxyType: "String"}},
						{GoName: "Password", FunxyName: "password", Type: GoTypeRef{Kind: GoTypeBasic, GoString: "string", FunxyType: "String"}},
						{GoName: "DB", FunxyName: "dB", Type: GoTypeRef{Kind: GoTypeBasic, GoString: "int", FunxyType: "Int"}},
					},
				},
			},
		},
	}

	codegen := NewCodeGenerator("parser")
	files, err := codegen.Generate(result)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var bindingContent string
	for _, f := range files {
		if strings.HasPrefix(f.Filename, "ext_") {
			bindingContent = f.Content
			break
		}
	}
	if bindingContent == "" {
		t.Fatal("no binding file generated")
	}

	t.Logf("--- binding ---\n%s", bindingContent)

	// Constructor function should be registered as "opts" (bare prefix)
	if !strings.Contains(bindingContent, `"opts"`) {
		t.Error("binding should contain constructor name \"opts\"")
	}

	// Should extract from RecordInstance
	if !strings.Contains(bindingContent, "RecordInstance") {
		t.Error("constructor should use RecordInstance")
	}

	// Should set struct fields
	if !strings.Contains(bindingContent, "obj.Addr") {
		t.Error("constructor should set obj.Addr")
	}
	if !strings.Contains(bindingContent, "obj.Password") {
		t.Error("constructor should set obj.Password")
	}
	if !strings.Contains(bindingContent, "obj.DB") {
		t.Error("constructor should set obj.DB")
	}

	// Should return HostObject with pointer
	if !strings.Contains(bindingContent, "HostObject{Value: &obj}") {
		t.Error("constructor should return &ext.HostObject{Value: &obj}")
	}

	// Constructor should NOT be generated for non-struct types
	resultNoStruct := &InspectResult{
		Bindings: []*ResolvedBinding{
			{
				Spec:          BindSpec{Type: "Client", As: "redis", Constructor: true},
				Dep:           Dep{Pkg: "github.com/example/pkg", Version: "v1.0.0", As: "pkg"},
				GoPackagePath: "github.com/example/pkg",
				TypeBinding: &TypeBinding{
					GoName:      "Client",
					IsStruct:    false,
					IsInterface: true,
					Methods:     []*MethodInfo{},
				},
			},
		},
	}
	files2, err := codegen.Generate(resultNoStruct)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, f := range files2 {
		if strings.HasPrefix(f.Filename, "ext_") {
			if strings.Contains(f.Content, "RecordInstance") {
				t.Error("constructor should NOT be generated for interface types")
			}
		}
	}
}

func TestCodegen_ConstructorPtrBasicFields(t *testing.T) {
	// Simulates AWS SDK-style structs with *string, *int32, *bool fields
	strRef := GoTypeRef{Kind: GoTypeBasic, GoString: "string", FunxyType: "String"}
	int32Ref := GoTypeRef{Kind: GoTypeBasic, GoString: "int32", FunxyType: "Int"}
	boolRef := GoTypeRef{Kind: GoTypeBasic, GoString: "bool", FunxyType: "Bool"}

	result := &InspectResult{
		Bindings: []*ResolvedBinding{
			{
				Spec:          BindSpec{Type: "PutInput", As: "s3Put", Constructor: true},
				Dep:           Dep{Pkg: "github.com/example/s3", Version: "v1.0.0", As: "s3"},
				GoPackagePath: "github.com/example/s3",
				TypeBinding: &TypeBinding{
					GoName:   "PutInput",
					IsStruct: true,
					Methods:  []*MethodInfo{},
					Fields: []*FieldInfo{
						{GoName: "Bucket", FunxyName: "bucket", Type: GoTypeRef{Kind: GoTypePtr, GoString: "*string", FunxyType: "String", ElemType: &strRef}},
						{GoName: "Key", FunxyName: "key", Type: GoTypeRef{Kind: GoTypePtr, GoString: "*string", FunxyType: "String", ElemType: &strRef}},
						{GoName: "MaxKeys", FunxyName: "maxKeys", Type: GoTypeRef{Kind: GoTypePtr, GoString: "*int32", FunxyType: "Int", ElemType: &int32Ref}},
						{GoName: "Recursive", FunxyName: "recursive", Type: GoTypeRef{Kind: GoTypePtr, GoString: "*bool", FunxyType: "Bool", ElemType: &boolRef}},
					},
				},
			},
		},
	}

	codegen := NewCodeGenerator("parser")
	files, err := codegen.Generate(result)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var bindingContent string
	for _, f := range files {
		if strings.HasPrefix(f.Filename, "ext_") {
			bindingContent = f.Content
			break
		}
	}
	if bindingContent == "" {
		t.Fatal("no binding file generated")
	}

	t.Logf("--- binding ---\n%s", bindingContent)

	// Should use toGoString (not toGoHost) for *string fields
	if !strings.Contains(bindingContent, "toGoString") {
		t.Error("constructor should use toGoString for *string fields")
	}
	// Should take address: &tmp
	if !strings.Contains(bindingContent, "&tmp") {
		t.Error("constructor should take address of converted value for pointer fields")
	}
	// Should set the struct fields
	if !strings.Contains(bindingContent, "obj.Bucket") {
		t.Error("constructor should set obj.Bucket")
	}
	if !strings.Contains(bindingContent, "obj.Key") {
		t.Error("constructor should set obj.Key")
	}
}

// =============================================================================
// Unit tests: extractSignature
// =============================================================================

func TestExtractSignature_WithMapParam(t *testing.T) {
	// func(map[string]int) string
	mapType := types.NewMap(types.Typ[types.String], types.Typ[types.Int])
	params := types.NewTuple(types.NewVar(0, nil, "data", mapType))
	results := types.NewTuple(types.NewVar(0, nil, "", types.Typ[types.String]))
	sig := types.NewSignatureType(nil, nil, nil, params, results, false)

	fs := extractSignature(sig)

	if len(fs.Params) != 1 {
		t.Fatalf("Params = %d, want 1", len(fs.Params))
	}
	if fs.Params[0].Type.Kind != GoTypeMap {
		t.Errorf("param Kind = %v, want GoTypeMap", fs.Params[0].Type.Kind)
	}
	if fs.Params[0].Type.FunxyType != "Map<String, Int>" {
		t.Errorf("param FunxyType = %q, want Map<String, Int>", fs.Params[0].Type.FunxyType)
	}
}

func TestExtractSignature_WithFuncParam(t *testing.T) {
	// func(func(int) bool) int
	cbParams := types.NewTuple(types.NewVar(0, nil, "n", types.Typ[types.Int]))
	cbResults := types.NewTuple(types.NewVar(0, nil, "", types.Typ[types.Bool]))
	cbSig := types.NewSignatureType(nil, nil, nil, cbParams, cbResults, false)

	params := types.NewTuple(types.NewVar(0, nil, "predicate", cbSig))
	results := types.NewTuple(types.NewVar(0, nil, "", types.Typ[types.Int]))
	sig := types.NewSignatureType(nil, nil, nil, params, results, false)

	fs := extractSignature(sig)

	if len(fs.Params) != 1 {
		t.Fatalf("Params = %d, want 1", len(fs.Params))
	}
	if fs.Params[0].Type.Kind != GoTypeFunc {
		t.Errorf("param Kind = %v, want GoTypeFunc", fs.Params[0].Type.Kind)
	}
	if fs.Params[0].Type.FuncSig == nil {
		t.Fatal("param FuncSig is nil")
	}
	if len(fs.Params[0].Type.FuncSig.Params) != 1 {
		t.Errorf("callback params = %d, want 1", len(fs.Params[0].Type.FuncSig.Params))
	}
	if fs.Params[0].Type.FuncSig.Params[0].Type.FunxyType != "Int" {
		t.Errorf("callback param type = %q, want Int", fs.Params[0].Type.FuncSig.Params[0].Type.FunxyType)
	}
}

// =============================================================================
// Unit tests: genFromFunxy dispatching
// =============================================================================

func TestGenFromFunxy_MapTypes(t *testing.T) {
	ctx := &bindingFileContext{
		FunxyModulePath: "parser",
		Imports:         make(map[string]string),
	}

	// map[string]int
	ref := GoTypeRef{
		Kind:      GoTypeMap,
		GoString:  "map[string]int",
		FunxyType: "Map<String, Int>",
		KeyType:   &GoTypeRef{Kind: GoTypeBasic, GoString: "string", FunxyType: "String"},
		ElemType:  &GoTypeRef{Kind: GoTypeBasic, GoString: "int", FunxyType: "Int"},
	}
	expr := ctx.genFromFunxy("arg", ref, "github.com/example/pkg")

	if expr != "toGoMap[string, int](arg)" {
		t.Errorf("genFromFunxy for map[string]int = %q, want toGoMap[string, int](arg)", expr)
	}
}

func TestGenFromFunxy_MapStringString(t *testing.T) {
	ctx := &bindingFileContext{
		FunxyModulePath: "parser",
		Imports:         make(map[string]string),
	}

	// map[string]string — must also use toGoMap, not any Record path
	ref := GoTypeRef{
		Kind:      GoTypeMap,
		GoString:  "map[string]string",
		FunxyType: "Map<String, String>",
		KeyType:   &GoTypeRef{Kind: GoTypeBasic, GoString: "string", FunxyType: "String"},
		ElemType:  &GoTypeRef{Kind: GoTypeBasic, GoString: "string", FunxyType: "String"},
	}
	expr := ctx.genFromFunxy("arg", ref, "github.com/example/pkg")

	if expr != "toGoMap[string, string](arg)" {
		t.Errorf("genFromFunxy for map[string]string = %q, want toGoMap[string, string](arg)", expr)
	}
}

func TestGenFromFunxy_FuncFallsThrough(t *testing.T) {
	ctx := &bindingFileContext{
		FunxyModulePath: "parser",
		Imports:         make(map[string]string),
	}

	// func types fall through to toGoAny (actual conversion is via genCallbackConversion)
	ref := GoTypeRef{
		Kind:      GoTypeFunc,
		GoString:  "func(string) bool",
		FunxyType: "(String) -> Bool",
		FuncSig:   &FuncSignature{},
	}
	expr := ctx.genFromFunxy("arg", ref, "github.com/example/pkg")

	if expr != "toGoAny(arg)" {
		t.Errorf("genFromFunxy for func = %q, want toGoAny(arg)", expr)
	}
}

// =============================================================================
// Integration tests: inspector with real packages
// =============================================================================

func TestInspector_ConstBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not found")
	}

	// uuid.Person is a typed constant: const Person = Domain(0), where Domain is byte.
	yaml := `
deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - const: Person
        as: uuidPerson
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	inspector := NewInspector("1.25.3")
	defer inspector.Cleanup()

	result, err := inspector.Inspect(cfg)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	cb := result.Bindings[0].ConstBinding
	if cb == nil {
		t.Fatal("expected ConstBinding")
	}
	if cb.GoName != "Person" {
		t.Errorf("GoName = %q, want Person", cb.GoName)
	}
	t.Logf("Const type: Kind=%v GoString=%q FunxyType=%q", cb.Type.Kind, cb.Type.GoString, cb.Type.FunxyType)
}

func TestInspector_ConstNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not found")
	}

	yaml := `
deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - const: NonExistentConst
        as: phantom
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	inspector := NewInspector("1.25.3")
	defer inspector.Cleanup()

	_, err = inspector.Inspect(cfg)
	if err == nil {
		t.Fatal("expected error for non-existent constant")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
	t.Logf("Expected error: %v", err)
}

func TestInspector_StructFields(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not found")
	}

	// uuid.UUID is a [16]byte, not a struct, so it won't have struct fields.
	// Let's use a package that definitely has structs with exported fields.
	// We use github.com/google/uuid and bind UUID as a type to verify
	// that non-struct types correctly have empty Fields.
	yaml := `
deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - type: UUID
        as: uuid
        methods: [String]
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	inspector := NewInspector("1.25.3")
	defer inspector.Cleanup()

	result, err := inspector.Inspect(cfg)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	tb := result.Bindings[0].TypeBinding
	if tb == nil {
		t.Fatal("expected TypeBinding")
	}

	// UUID is a [16]byte, not a struct — Fields should be empty
	if tb.IsStruct {
		t.Error("UUID should not be IsStruct (it's a named array type)")
	}
	if len(tb.Fields) != 0 {
		t.Errorf("UUID should have 0 fields, got %d", len(tb.Fields))
	}
}

// =============================================================================
// E2E test: build and run with maps
// =============================================================================

func TestE2E_BuildAndRunWithMaps(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	// Create a project that exercises map conversions.
	// We use uuid + a simple script that creates a Map and passes it around.
	yamlContent := `deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - func: New
        as: uuidNew
      - func: Parse
        as: uuidParse
        error_to_result: true
`
	projectDir := writeTestProject(t, yamlContent)

	// Script uses map builtins from lib/map alongside ext functions
	script := `import "ext/uuid" (uuidNew, uuidParse)
import "lib/map" (mapNew, mapPut, mapGet, mapSize)

myMap = mapNew()
myMap2 = mapPut(myMap, "key1", "value1")
myMap3 = mapPut(myMap2, "key2", "value2")

print("Map size: " ++ show(mapSize(myMap3)))
print("key1: " ++ match mapGet(myMap3, "key1") { Some(v) -> v, None -> "missing" })

myUuid = uuidNew()
print("UUID: " ++ show(myUuid))
`
	if err := os.WriteFile(filepath.Join(projectDir, "app.lang"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	appBinary := filepath.Join(projectDir, "app")
	output, err := runFunxy(t, binary, sourceDir, projectDir,
		"build", "app.lang",
		"--config", filepath.Join(projectDir, "funxy.yaml"),
		"-o", appBinary,
		"--ext-verbose",
	)
	t.Logf("Build output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy build failed: %v", err)
	}

	runCmd := exec.Command(appBinary)
	runOutput, err := runCmd.CombinedOutput()
	t.Logf("App output:\n%s", string(runOutput))

	outStr := string(runOutput)
	if !strings.Contains(outStr, "Map size: 2") {
		t.Error("expected 'Map size: 2' in output")
	}
	if !strings.Contains(outStr, "key1: value1") {
		t.Error("expected 'key1: value1' in output")
	}
	if !strings.Contains(outStr, "UUID:") {
		t.Error("expected 'UUID:' in output")
	}
}

// =============================================================================
// Stubs: verify field getter and const stubs
// =============================================================================

func TestStubs_FieldGettersAndConsts(t *testing.T) {
	// Test stub generation for field getters and constants
	result := &InspectResult{
		Bindings: []*ResolvedBinding{
			{
				Spec:          BindSpec{Type: "Options", As: "opts"},
				Dep:           Dep{Pkg: "github.com/example/pkg", Version: "v1.0.0", As: "pkg"},
				GoPackagePath: "github.com/example/pkg",
				TypeBinding: &TypeBinding{
					GoName:   "Options",
					IsStruct: true,
					Methods:  []*MethodInfo{},
					Fields: []*FieldInfo{
						{GoName: "Host", FunxyName: "host", Type: GoTypeRef{Kind: GoTypeBasic, GoString: "string", FunxyType: "String"}},
						{GoName: "Port", FunxyName: "port", Type: GoTypeRef{Kind: GoTypeBasic, GoString: "int", FunxyType: "Int"}},
					},
				},
			},
			{
				Spec:          BindSpec{Const: "MaxRetries", As: "maxRetries"},
				Dep:           Dep{Pkg: "github.com/example/pkg", Version: "v1.0.0", As: "pkg"},
				GoPackagePath: "github.com/example/pkg",
				ConstBinding: &ConstBinding{
					GoName: "MaxRetries",
					Type:   GoTypeRef{Kind: GoTypeBasic, GoString: "int", FunxyType: "Int"},
				},
			},
		},
	}

	cfg := &Config{
		Deps: []Dep{
			{Pkg: "github.com/example/pkg", Version: "v1.0.0", As: "pkg"},
		},
	}

	projectDir := t.TempDir()
	if err := GenerateStubs(cfg, result, projectDir); err != nil {
		t.Fatalf("GenerateStubs: %v", err)
	}

	stubPath := filepath.Join(projectDir, ".funxy", "ext", "pkg.d.lang")
	data, err := os.ReadFile(stubPath)
	if err != nil {
		t.Fatalf("reading stub: %v", err)
	}
	content := string(data)
	t.Logf("--- stub ---\n%s", content)

	// Verify field getter stubs
	if !strings.Contains(content, "optsHost") {
		t.Error("stub should contain optsHost field getter")
	}
	if !strings.Contains(content, "optsPort") {
		t.Error("stub should contain optsPort field getter")
	}

	// Verify constant stub
	if !strings.Contains(content, "maxRetries") {
		t.Error("stub should contain maxRetries constant")
	}
}

// =============================================================================
// Unit tests: Go generics support
// =============================================================================

func TestGoTypeToRef_TypeParam(t *testing.T) {
	// Create a type parameter T with constraint any
	tparam := types.NewTypeParam(types.NewTypeName(0, nil, "T", nil), types.Universe.Lookup("any").Type())
	tparam.SetConstraint(types.Universe.Lookup("any").Type())

	ref := goTypeToRef(tparam)

	if ref.Kind != GoTypeTypeParam {
		t.Errorf("Kind = %v, want GoTypeTypeParam", ref.Kind)
	}
	if ref.GoString != "T" {
		t.Errorf("GoString = %q, want %q", ref.GoString, "T")
	}
	if ref.FunxyType != "t" {
		t.Errorf("FunxyType = %q, want %q (lowercase)", ref.FunxyType, "t")
	}
}

func TestResolveTypeArgs_AllAny(t *testing.T) {
	params := []TypeParamInfo{
		{Name: "T", Constraint: "any", IsAny: true},
		{Name: "U", Constraint: "any", IsAny: true},
	}

	args, refs, err := resolveTypeArgs(params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 2 || args[0] != "any" || args[1] != "any" {
		t.Errorf("args = %v, want [any any]", args)
	}
	if len(refs) != 2 || refs[0].GoString != "any" || refs[1].GoString != "any" {
		t.Errorf("refs not properly set")
	}
}

func TestResolveTypeArgs_ConstrainedRequiresExplicit(t *testing.T) {
	params := []TypeParamInfo{
		{Name: "T", Constraint: "comparable", IsAny: false},
	}

	_, _, err := resolveTypeArgs(params, nil)
	if err == nil {
		t.Fatal("expected error for constrained type param without type_args")
	}
	if !strings.Contains(err.Error(), "type_args") {
		t.Errorf("error should mention type_args: %v", err)
	}
}

func TestResolveTypeArgs_Explicit(t *testing.T) {
	params := []TypeParamInfo{
		{Name: "T", Constraint: "comparable", IsAny: false},
	}

	args, refs, err := resolveTypeArgs(params, []string{"string"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 1 || args[0] != "string" {
		t.Errorf("args = %v, want [string]", args)
	}
	if refs[0].FunxyType != "String" {
		t.Errorf("ref FunxyType = %q, want String", refs[0].FunxyType)
	}
}

func TestResolveTypeArgs_CountMismatch(t *testing.T) {
	params := []TypeParamInfo{
		{Name: "T", Constraint: "any", IsAny: true},
		{Name: "U", Constraint: "any", IsAny: true},
	}

	_, _, err := resolveTypeArgs(params, []string{"string"})
	if err == nil {
		t.Fatal("expected error for type_args count mismatch")
	}
}

func TestSubstituteSignature_BasicTypes(t *testing.T) {
	// Simulate: func Foo[T any](x T) T
	tRef := GoTypeRef{Kind: GoTypeTypeParam, GoString: "T", FunxyType: "t", TypeParamIndex: 0}
	sig := &FuncSignature{
		Params:  []*ParamInfo{{Name: "x", Type: tRef}},
		Results: []*ParamInfo{{Type: tRef}},
	}
	subs := []GoTypeRef{{Kind: GoTypeBasic, GoString: "any", FunxyType: "HostObject"}}

	result := substituteSignature(sig, subs)

	if result.Params[0].Type.Kind != GoTypeBasic {
		t.Errorf("param kind = %v, want GoTypeBasic", result.Params[0].Type.Kind)
	}
	if result.Params[0].Type.GoString != "any" {
		t.Errorf("param GoString = %q, want %q", result.Params[0].Type.GoString, "any")
	}
	if result.Results[0].Type.FunxyType != "HostObject" {
		t.Errorf("result FunxyType = %q, want %q", result.Results[0].Type.FunxyType, "HostObject")
	}
}

func TestSubstituteSignature_SliceOfTypeParam(t *testing.T) {
	// Simulate: func Foo[T any](s []T) []T
	tRef := GoTypeRef{Kind: GoTypeTypeParam, GoString: "T", FunxyType: "t", TypeParamIndex: 0}
	sliceRef := GoTypeRef{Kind: GoTypeSlice, GoString: "[]T", ElemType: &tRef, FunxyType: "List<t>"}
	sig := &FuncSignature{
		Params:  []*ParamInfo{{Name: "s", Type: sliceRef}},
		Results: []*ParamInfo{{Type: sliceRef}},
	}
	subs := []GoTypeRef{{Kind: GoTypeBasic, GoString: "string", FunxyType: "String"}}

	result := substituteSignature(sig, subs)

	if result.Params[0].Type.GoString != "[]string" {
		t.Errorf("param GoString = %q, want %q", result.Params[0].Type.GoString, "[]string")
	}
	if result.Params[0].Type.FunxyType != "List<String>" {
		t.Errorf("param FunxyType = %q, want %q", result.Params[0].Type.FunxyType, "List<String>")
	}
}

func TestCodegen_GenericFuncAny(t *testing.T) {
	// Generic function: func Contains[T any](s []T, v T) bool — auto-instantiated with any
	tRef := GoTypeRef{Kind: GoTypeBasic, GoString: "any", FunxyType: "HostObject"}
	sliceRef := GoTypeRef{Kind: GoTypeSlice, GoString: "[]any", ElemType: &tRef, FunxyType: "List<HostObject>"}

	result := &InspectResult{
		Bindings: []*ResolvedBinding{
			{
				Spec:          BindSpec{Func: "Contains", As: "contains"},
				Dep:           Dep{Pkg: "github.com/example/generics", Version: "v1.0.0", As: "generics"},
				GoPackagePath: "github.com/example/generics",
				FuncBinding: &FuncBinding{
					GoName: "Contains",
					Signature: &FuncSignature{
						Params: []*ParamInfo{
							{Name: "s", Type: sliceRef},
							{Name: "v", Type: tRef},
						},
						Results: []*ParamInfo{
							{Type: GoTypeRef{Kind: GoTypeBasic, GoString: "bool", FunxyType: "Bool"}},
						},
					},
					TypeParams: []TypeParamInfo{{Name: "T", Constraint: "any", IsAny: true}},
					TypeArgs:   []string{"any"},
				},
			},
		},
	}

	cg := NewCodeGenerator("parser")
	files, err := cg.Generate(result)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var bindingFile string
	for _, f := range files {
		if strings.Contains(f.Filename, "ext_generics") {
			bindingFile = f.Content
			break
		}
	}

	if bindingFile == "" {
		t.Fatal("no ext_generics.go file generated")
	}
	t.Logf("--- generated ---\n%s", bindingFile)

	// The call expression should include type args
	if !strings.Contains(bindingFile, "Contains[any]") {
		t.Error("generated code should contain Contains[any](...)")
	}
}

func TestCodegen_GenericFuncExplicitTypeArgs(t *testing.T) {
	// Generic function with explicit type_args: func Contains[T comparable](s []T, v T) bool
	// type_args: [string] → Contains[string](...)
	strRef := GoTypeRef{Kind: GoTypeBasic, GoString: "string", FunxyType: "String"}
	sliceRef := GoTypeRef{Kind: GoTypeSlice, GoString: "[]string", ElemType: &strRef, FunxyType: "List<String>"}

	result := &InspectResult{
		Bindings: []*ResolvedBinding{
			{
				Spec:          BindSpec{Func: "Contains", As: "contains", TypeArgs: []string{"string"}},
				Dep:           Dep{Pkg: "github.com/example/generics", Version: "v1.0.0", As: "generics"},
				GoPackagePath: "github.com/example/generics",
				FuncBinding: &FuncBinding{
					GoName: "Contains",
					Signature: &FuncSignature{
						Params: []*ParamInfo{
							{Name: "s", Type: sliceRef},
							{Name: "v", Type: strRef},
						},
						Results: []*ParamInfo{
							{Type: GoTypeRef{Kind: GoTypeBasic, GoString: "bool", FunxyType: "Bool"}},
						},
					},
					TypeParams: []TypeParamInfo{{Name: "T", Constraint: "comparable", IsAny: false}},
					TypeArgs:   []string{"string"},
				},
			},
		},
	}

	cg := NewCodeGenerator("parser")
	files, err := cg.Generate(result)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var bindingFile string
	for _, f := range files {
		if strings.Contains(f.Filename, "ext_generics") {
			bindingFile = f.Content
			break
		}
	}

	if bindingFile == "" {
		t.Fatal("no ext_generics.go file generated")
	}
	t.Logf("--- generated ---\n%s", bindingFile)

	if !strings.Contains(bindingFile, "Contains[string]") {
		t.Error("generated code should contain Contains[string](...)")
	}
}

func TestIsAnyConstraint(t *testing.T) {
	// any constraint
	tp1 := types.NewTypeParam(types.NewTypeName(0, nil, "T", nil), types.Universe.Lookup("any").Type())
	tp1.SetConstraint(types.Universe.Lookup("any").Type())
	if !isAnyConstraint(tp1) {
		t.Error("expected isAnyConstraint=true for 'any' constraint")
	}

	// comparable constraint
	tp2 := types.NewTypeParam(types.NewTypeName(0, nil, "T", nil), types.Universe.Lookup("comparable").Type())
	tp2.SetConstraint(types.Universe.Lookup("comparable").Type())
	if isAnyConstraint(tp2) {
		t.Error("expected isAnyConstraint=false for 'comparable' constraint")
	}
}

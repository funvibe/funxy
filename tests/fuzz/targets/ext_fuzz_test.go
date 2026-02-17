package targets

import (
	"go/parser"
	"go/token"
	"github.com/funvibe/funxy/internal/ext"
	"strings"
	"testing"
)

// =============================================================================
// FuzzConfigParse — random bytes as YAML config
// =============================================================================

// FuzzConfigParse tests that ext.ParseConfig never panics on arbitrary input.
func FuzzConfigParse(f *testing.F) {
	capFuzzProcs()

	// Seed corpus: valid configs
	f.Add([]byte(`deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - func: New
        as: uuidNew
`))
	f.Add([]byte(`deps:
  - pkg: github.com/example/pkg
    bind:
      - type: Client
        as: cli
        methods: [Get, Set]
        error_to_result: true
        skip_context: true
`))
	f.Add([]byte(`deps:
  - pkg: github.com/example/pkg
    bind_all: true
    as: pkg
`))
	f.Add([]byte(`deps:
  - pkg: github.com/example/pkg
    bind:
      - const: StatusOK
        as: httpStatusOK
`))
	// Edge cases
	f.Add([]byte(""))
	f.Add([]byte("deps: []"))
	f.Add([]byte("{}"))
	f.Add([]byte("null"))
	f.Add([]byte("deps:\n  - pkg: x\n    bind:\n      - type: A\n        func: B\n        as: c"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must not panic — errors are expected and fine
		_, _ = ext.ParseConfig(data, "fuzz.yaml")
	})
}

// =============================================================================
// FuzzCodegen — constructed bindings → Generate → verify Go code is parseable
// =============================================================================

// FuzzCodegen tests that code generation produces syntactically valid Go
// for arbitrary combinations of binding types.
func FuzzCodegen(f *testing.F) {
	capFuzzProcs()

	// Seeds encode: funcName(byte), paramKind(byte), returnKind(byte), ...
	f.Add([]byte{0, 0, 0})
	f.Add([]byte{1, 1, 1})
	f.Add([]byte{2, 2, 2})
	f.Add([]byte{3, 3, 3, 4, 5, 6})
	f.Add([]byte{255, 128, 64, 32, 16, 8, 4, 2, 1, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 3 {
			return
		}

		// Build synthetic bindings from fuzz data
		result := buildFuzzInspectResult(data)
		if result == nil || len(result.Bindings) == 0 {
			return
		}

		codegen := ext.NewCodeGenerator("parser")
		files, err := codegen.Generate(result)
		if err != nil {
			return // Generation errors are acceptable
		}

		// Verify all generated .go files are syntactically valid Go
		fset := token.NewFileSet()
		for _, file := range files {
			if !strings.HasSuffix(file.Filename, ".go") {
				continue
			}
			_, err := parser.ParseFile(fset, file.Filename, file.Content, parser.AllErrors)
			if err != nil {
				t.Errorf("Generated file %s is not valid Go:\n%s\nError: %v",
					file.Filename, file.Content, err)
			}
		}
	})
}

// =============================================================================
// Helpers for FuzzCodegen
// =============================================================================

// buildFuzzInspectResult constructs an InspectResult from raw bytes.
func buildFuzzInspectResult(data []byte) *ext.InspectResult {
	if len(data) < 3 {
		return nil
	}

	var bindings []*ext.ResolvedBinding
	dep := ext.Dep{Pkg: "github.com/fuzz/pkg", Version: "v1.0.0", As: "fuzz"}

	i := 0
	bindingIdx := 0
	for i < len(data)-2 && bindingIdx < 5 { // Max 5 bindings
		kind := data[i] % 4 // 0=func, 1=type+methods, 2=const, 3=func with callback
		i++

		switch kind {
		case 0: // Function binding with various param/return types
			paramCount := int(data[i]%4) + 1
			i++
			params := make([]*ext.ParamInfo, 0, paramCount)
			for p := 0; p < paramCount && i < len(data); p++ {
				params = append(params, &ext.ParamInfo{
					Name: fuzzParamName(p),
					Type: fuzzGoTypeRef(data[i]),
				})
				i++
			}

			var results []*ext.ParamInfo
			hasError := false
			if i < len(data) {
				retRef := fuzzGoTypeRef(data[i])
				i++
				results = append(results, &ext.ParamInfo{Type: retRef})
				if i < len(data) && data[i]%3 == 0 {
					results = append(results, &ext.ParamInfo{Type: ext.GoTypeRef{Kind: ext.GoTypeError, GoString: "error", FunxyType: "String"}})
					hasError = true
				}
				i++
			}

			name := fuzzFuncName(bindingIdx)
			bindings = append(bindings, &ext.ResolvedBinding{
				Spec:          ext.BindSpec{Func: name, As: "fuzz" + name},
				Dep:           dep,
				GoPackagePath: dep.Pkg,
				FuncBinding: &ext.FuncBinding{
					GoName: name,
					Signature: &ext.FuncSignature{
						Params:         params,
						Results:        results,
						HasErrorReturn: hasError,
					},
				},
			})

		case 1: // Type binding with methods and fields
			methodCount := 1
			if i < len(data) {
				methodCount = int(data[i]%3) + 1
				i++
			}
			methods := make([]*ext.MethodInfo, 0, methodCount)
			for m := 0; m < methodCount && i < len(data); m++ {
				mName := fuzzMethodName(m)
				var mParams []*ext.ParamInfo
				var mResults []*ext.ParamInfo
				if i < len(data) {
					mResults = append(mResults, &ext.ParamInfo{Type: fuzzGoTypeRef(data[i])})
					i++
				}
				methods = append(methods, &ext.MethodInfo{
					GoName:    mName,
					FunxyName: "fuzz" + mName,
					Signature: &ext.FuncSignature{
						Params:  mParams,
						Results: mResults,
					},
				})
			}

			var fields []*ext.FieldInfo
			if i < len(data) && data[i]%2 == 0 {
				fields = append(fields, &ext.FieldInfo{
					GoName:    "Value",
					FunxyName: "value",
					Type:      ext.GoTypeRef{Kind: ext.GoTypeBasic, GoString: "string", FunxyType: "String"},
				})
			}
			i++

			bindings = append(bindings, &ext.ResolvedBinding{
				Spec:          ext.BindSpec{Type: "FuzzType", As: "fuzz"},
				Dep:           dep,
				GoPackagePath: dep.Pkg,
				TypeBinding: &ext.TypeBinding{
					GoName:   "FuzzType",
					IsStruct: len(fields) > 0,
					Methods:  methods,
					Fields:   fields,
				},
			})

		case 2: // Constant binding
			if i < len(data) {
				ref := fuzzGoTypeRef(data[i])
				i++
				name := fuzzConstName(bindingIdx)
				bindings = append(bindings, &ext.ResolvedBinding{
					Spec:          ext.BindSpec{Const: name, As: "fuzz" + name},
					Dep:           dep,
					GoPackagePath: dep.Pkg,
					ConstBinding: &ext.ConstBinding{
						GoName: name,
						Type:   ref,
					},
				})
			}

		case 3: // Function with callback parameter
			if i+1 < len(data) {
				cbRetType := fuzzGoTypeRef(data[i])
				i++
				cbParamType := fuzzGoTypeRef(data[i])
				i++

				name := fuzzFuncName(bindingIdx)
				bindings = append(bindings, &ext.ResolvedBinding{
					Spec:          ext.BindSpec{Func: name, As: "fuzz" + name},
					Dep:           dep,
					GoPackagePath: dep.Pkg,
					FuncBinding: &ext.FuncBinding{
						GoName: name,
						Signature: &ext.FuncSignature{
							Params: []*ext.ParamInfo{
								{
									Name: "cb",
									Type: ext.GoTypeRef{
										Kind:      ext.GoTypeFunc,
										GoString:  "func(string) bool",
										FunxyType: "Fn",
										FuncSig: &ext.FuncSignature{
											Params:  []*ext.ParamInfo{{Name: "x", Type: cbParamType}},
											Results: []*ext.ParamInfo{{Type: cbRetType}},
										},
									},
								},
							},
							Results: []*ext.ParamInfo{{Type: ext.GoTypeRef{Kind: ext.GoTypeBasic, GoString: "int", FunxyType: "Int"}}},
						},
					},
				})
			}
		}
		bindingIdx++
	}

	if len(bindings) == 0 {
		return nil
	}
	return &ext.InspectResult{Bindings: bindings}
}

func fuzzGoTypeRef(b byte) ext.GoTypeRef {
	switch b % 8 {
	case 0:
		return ext.GoTypeRef{Kind: ext.GoTypeBasic, GoString: "int", FunxyType: "Int"}
	case 1:
		return ext.GoTypeRef{Kind: ext.GoTypeBasic, GoString: "string", FunxyType: "String"}
	case 2:
		return ext.GoTypeRef{Kind: ext.GoTypeBasic, GoString: "bool", FunxyType: "Bool"}
	case 3:
		return ext.GoTypeRef{Kind: ext.GoTypeBasic, GoString: "float64", FunxyType: "Float"}
	case 4:
		return ext.GoTypeRef{Kind: ext.GoTypeByteSlice, GoString: "[]byte", FunxyType: "Bytes"}
	case 5:
		return ext.GoTypeRef{
			Kind: ext.GoTypeSlice, GoString: "[]string", FunxyType: "List<String>",
			ElemType: &ext.GoTypeRef{Kind: ext.GoTypeBasic, GoString: "string", FunxyType: "String"},
		}
	case 6:
		return ext.GoTypeRef{
			Kind: ext.GoTypeMap, GoString: "map[string]int", FunxyType: "Map<String, Int>",
			KeyType:  &ext.GoTypeRef{Kind: ext.GoTypeBasic, GoString: "string", FunxyType: "String"},
			ElemType: &ext.GoTypeRef{Kind: ext.GoTypeBasic, GoString: "int", FunxyType: "Int"},
		}
	default:
		return ext.GoTypeRef{Kind: ext.GoTypeNamed, GoString: "fuzz.FuzzType", PkgPath: "github.com/fuzz/pkg", TypeName: "FuzzType", FunxyType: "HostObject"}
	}
}

func fuzzParamName(i int) string {
	names := []string{"a", "b", "c", "d"}
	if i < len(names) {
		return names[i]
	}
	return "x"
}

func fuzzFuncName(i int) string {
	names := []string{"DoWork", "Process", "Transform", "Execute", "Handle"}
	if i < len(names) {
		return names[i]
	}
	return "Action"
}

func fuzzMethodName(i int) string {
	names := []string{"Get", "Set", "Run"}
	if i < len(names) {
		return names[i]
	}
	return "Do"
}

func fuzzConstName(i int) string {
	names := []string{"MaxSize", "DefaultPort", "Version", "Enabled", "Timeout"}
	if i < len(names) {
		return names[i]
	}
	return "Value"
}

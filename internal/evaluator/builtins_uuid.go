package evaluator

import (
	"hash/fnv"
	"github.com/funvibe/funxy/internal/typesystem"
	"strings"

	"github.com/google/uuid"
)

// Uuid object wraps google/uuid
type Uuid struct {
	Value uuid.UUID
}

func (u *Uuid) Type() ObjectType             { return "UUID" }
func (u *Uuid) TypeName() string             { return "Uuid" }
func (u *Uuid) Inspect() string              { return u.Value.String() }
func (u *Uuid) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Uuid"} }
func (u *Uuid) Hash() uint32 {
	h := fnv.New32a()
	h.Write(u.Value[:])
	return h.Sum32()
}

// Pre-defined namespace UUIDs for v5
var (
	NamespaceDNS  = &Uuid{Value: uuid.NameSpaceDNS}
	NamespaceURL  = &Uuid{Value: uuid.NameSpaceURL}
	NamespaceOID  = &Uuid{Value: uuid.NameSpaceOID}
	NamespaceX500 = &Uuid{Value: uuid.NameSpaceX500}
)

// UuidBuiltins returns built-in functions for lib/uuid virtual package
func UuidBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// Generation
		"uuidNew": {Fn: builtinUuidNew, Name: "uuidNew"},
		"uuidV4":  {Fn: builtinUuidNew, Name: "uuidV4"}, // alias
		"uuidV5":  {Fn: builtinUuidV5, Name: "uuidV5"},
		"uuidV7":  {Fn: builtinUuidV7, Name: "uuidV7"},
		"uuidNil": {Fn: builtinUuidNil, Name: "uuidNil"},
		"uuidMax": {Fn: builtinUuidMax, Name: "uuidMax"},

		// Namespaces for v5
		"uuidNamespaceDNS":  {Fn: builtinUuidNamespaceDNS, Name: "uuidNamespaceDNS"},
		"uuidNamespaceURL":  {Fn: builtinUuidNamespaceURL, Name: "uuidNamespaceURL"},
		"uuidNamespaceOID":  {Fn: builtinUuidNamespaceOID, Name: "uuidNamespaceOID"},
		"uuidNamespaceX500": {Fn: builtinUuidNamespaceX500, Name: "uuidNamespaceX500"},

		// Parsing
		"uuidParse":     {Fn: builtinUuidParse, Name: "uuidParse"},
		"uuidFromBytes": {Fn: builtinUuidFromBytes, Name: "uuidFromBytes"},

		// Conversion
		"uuidToString":        {Fn: builtinUuidToString, Name: "uuidToString"},
		"uuidToStringCompact": {Fn: builtinUuidToStringCompact, Name: "uuidToStringCompact"},
		"uuidToStringUrn":     {Fn: builtinUuidToStringUrn, Name: "uuidToStringUrn"},
		"uuidToStringBraces":  {Fn: builtinUuidToStringBraces, Name: "uuidToStringBraces"},
		"uuidToStringUpper":   {Fn: builtinUuidToStringUpper, Name: "uuidToStringUpper"},
		"uuidToBytes":         {Fn: builtinUuidToBytes, Name: "uuidToBytes"},

		// Info
		"uuidVersion": {Fn: builtinUuidVersion, Name: "uuidVersion"},
		"uuidIsNil":   {Fn: builtinUuidIsNil, Name: "uuidIsNil"},
	}
}

// SetUuidBuiltinTypes sets type information for uuid builtins
func SetUuidBuiltinTypes(builtins map[string]*Builtin) {
	uuidType := typesystem.TCon{Name: "Uuid"}
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	bytesType := typesystem.TCon{Name: "Bytes"}
	intType := typesystem.Int
	boolType := typesystem.Bool

	resultUuid := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, uuidType},
	}

	types := map[string]typesystem.Type{
		// Generation
		"uuidNew": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
		"uuidV4":  typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
		"uuidV5":  typesystem.TFunc{Params: []typesystem.Type{uuidType, stringType}, ReturnType: uuidType},
		"uuidV7":  typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
		"uuidNil": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
		"uuidMax": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},

		// Namespaces
		"uuidNamespaceDNS":  typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
		"uuidNamespaceURL":  typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
		"uuidNamespaceOID":  typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
		"uuidNamespaceX500": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},

		// Parsing
		"uuidParse":     typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultUuid},
		"uuidFromBytes": typesystem.TFunc{Params: []typesystem.Type{bytesType}, ReturnType: resultUuid},

		// Conversion
		"uuidToString":        typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: stringType},
		"uuidToStringCompact": typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: stringType},
		"uuidToStringUrn":     typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: stringType},
		"uuidToStringBraces":  typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: stringType},
		"uuidToStringUpper":   typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: stringType},
		"uuidToBytes":         typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: bytesType},

		// Info
		"uuidVersion": typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: intType},
		"uuidIsNil":   typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: boolType},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

// ===== Generation =====

// uuidNew: () -> Uuid (v4 random)
func builtinUuidNew(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("uuidNew expects 0 arguments, got %d", len(args))
	}
	return &Uuid{Value: uuid.New()}
}

// uuidV5: (Uuid, String) -> Uuid (deterministic from namespace + name)
func builtinUuidV5(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("uuidV5 expects 2 arguments, got %d", len(args))
	}

	ns, ok := args[0].(*Uuid)
	if !ok {
		return newError("uuidV5: first argument must be Uuid namespace, got %s", args[0].Type())
	}

	nameList, ok := args[1].(*List)
	if !ok {
		return newError("uuidV5: second argument must be String, got %s", args[1].Type())
	}
	name := listToString(nameList)

	return &Uuid{Value: uuid.NewSHA1(ns.Value, []byte(name))}
}

// uuidV7: () -> Uuid (time-ordered)
func builtinUuidV7(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("uuidV7 expects 0 arguments, got %d", len(args))
	}
	u, err := uuid.NewV7()
	if err != nil {
		return newError("uuidV7: failed to generate: %s", err.Error())
	}
	return &Uuid{Value: u}
}

// uuidNil: () -> Uuid (00000000-0000-0000-0000-000000000000)
func builtinUuidNil(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("uuidNil expects 0 arguments, got %d", len(args))
	}
	return &Uuid{Value: uuid.Nil}
}

// uuidMax: () -> Uuid (ffffffff-ffff-ffff-ffff-ffffffffffff)
func builtinUuidMax(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("uuidMax expects 0 arguments, got %d", len(args))
	}
	return &Uuid{Value: uuid.Max}
}

// ===== Namespaces =====

func builtinUuidNamespaceDNS(e *Evaluator, args ...Object) Object {
	return NamespaceDNS
}

func builtinUuidNamespaceURL(e *Evaluator, args ...Object) Object {
	return NamespaceURL
}

func builtinUuidNamespaceOID(e *Evaluator, args ...Object) Object {
	return NamespaceOID
}

func builtinUuidNamespaceX500(e *Evaluator, args ...Object) Object {
	return NamespaceX500
}

// ===== Parsing =====

// uuidParse: (String) -> Result<Uuid, String>
func builtinUuidParse(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("uuidParse expects 1 argument, got %d", len(args))
	}

	strList, ok := args[0].(*List)
	if !ok {
		return newError("uuidParse: argument must be String, got %s", args[0].Type())
	}
	str := listToString(strList)

	u, parseErr := uuid.Parse(str)
	if parseErr != nil {
		return makeFailStr(parseErr.Error())
	}

	return makeOk(&Uuid{Value: u})
}

// uuidFromBytes: (Bytes) -> Result<Uuid, String>
func builtinUuidFromBytes(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("uuidFromBytes expects 1 argument, got %d", len(args))
	}

	b, ok := args[0].(*Bytes)
	if !ok {
		return newError("uuidFromBytes: argument must be Bytes, got %s", args[0].Type())
	}

	if len(b.data) != 16 {
		return makeFailStr("uuidFromBytes: expected 16 bytes")
	}

	u, err := uuid.FromBytes(b.data)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Uuid{Value: u})
}

// ===== Conversion =====

// uuidToString: (Uuid) -> String (standard format: 8-4-4-4-12)
func builtinUuidToString(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("uuidToString expects 1 argument, got %d", len(args))
	}

	u, ok := args[0].(*Uuid)
	if !ok {
		return newError("uuidToString: argument must be Uuid, got %s", args[0].Type())
	}

	return stringToList(u.Value.String())
}

// uuidToStringCompact: (Uuid) -> String (no dashes)
func builtinUuidToStringCompact(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("uuidToStringCompact expects 1 argument, got %d", len(args))
	}

	u, ok := args[0].(*Uuid)
	if !ok {
		return newError("uuidToStringCompact: argument must be Uuid, got %s", args[0].Type())
	}

	str := strings.ReplaceAll(u.Value.String(), "-", "")
	return stringToList(str)
}

// uuidToStringUrn: (Uuid) -> String (urn:uuid:...)
func builtinUuidToStringUrn(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("uuidToStringUrn expects 1 argument, got %d", len(args))
	}

	u, ok := args[0].(*Uuid)
	if !ok {
		return newError("uuidToStringUrn: argument must be Uuid, got %s", args[0].Type())
	}

	return stringToList(u.Value.URN())
}

// uuidToStringBraces: (Uuid) -> String ({...})
func builtinUuidToStringBraces(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("uuidToStringBraces expects 1 argument, got %d", len(args))
	}

	u, ok := args[0].(*Uuid)
	if !ok {
		return newError("uuidToStringBraces: argument must be Uuid, got %s", args[0].Type())
	}

	return stringToList("{" + u.Value.String() + "}")
}

// uuidToStringUpper: (Uuid) -> String (uppercase)
func builtinUuidToStringUpper(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("uuidToStringUpper expects 1 argument, got %d", len(args))
	}

	u, ok := args[0].(*Uuid)
	if !ok {
		return newError("uuidToStringUpper: argument must be Uuid, got %s", args[0].Type())
	}

	return stringToList(strings.ToUpper(u.Value.String()))
}

// uuidToBytes: (Uuid) -> Bytes
func builtinUuidToBytes(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("uuidToBytes expects 1 argument, got %d", len(args))
	}

	u, ok := args[0].(*Uuid)
	if !ok {
		return newError("uuidToBytes: argument must be Uuid, got %s", args[0].Type())
	}

	bytes := u.Value[:]
	return &Bytes{data: bytes}
}

// ===== Info =====

// uuidVersion: (Uuid) -> Int
func builtinUuidVersion(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("uuidVersion expects 1 argument, got %d", len(args))
	}

	u, ok := args[0].(*Uuid)
	if !ok {
		return newError("uuidVersion: argument must be Uuid, got %s", args[0].Type())
	}

	return &Integer{Value: int64(u.Value.Version())}
}

// uuidIsNil: (Uuid) -> Bool
func builtinUuidIsNil(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("uuidIsNil expects 1 argument, got %d", len(args))
	}

	u, ok := args[0].(*Uuid)
	if !ok {
		return newError("uuidIsNil: argument must be Uuid, got %s", args[0].Type())
	}

	if u.Value == uuid.Nil {
		return TRUE
	}
	return FALSE
}

package evaluator

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"github.com/funvibe/funxy/internal/typesystem"
	"strconv"
	"strings"
	"unicode/utf8"
)

// BytesBuiltins returns built-in functions for lib/bytes virtual package
func BytesBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// Creation
		"bytesNew":        {Fn: builtinBytesNew, Name: "bytesNew"},
		"bytesFromString": {Fn: builtinBytesFromString, Name: "bytesFromString"},
		"bytesFromList":   {Fn: builtinBytesFromList, Name: "bytesFromList"},
		"bytesFromHex":    {Fn: builtinBytesFromHex, Name: "bytesFromHex"},
		"bytesFromBin":    {Fn: builtinBytesFromBin, Name: "bytesFromBin"},
		"bytesFromOct":    {Fn: builtinBytesFromOct, Name: "bytesFromOct"},

		// Access
		"bytesSlice": {Fn: builtinBytesSlice, Name: "bytesSlice"},

		// Conversion
		"bytesToString": {Fn: builtinBytesToString, Name: "bytesToString"},
		"bytesToList":   {Fn: builtinBytesToList, Name: "bytesToList"},
		"bytesToHex":    {Fn: builtinBytesToHex, Name: "bytesToHex"},
		"bytesToBin":    {Fn: builtinBytesToBin, Name: "bytesToBin"},
		"bytesToOct":    {Fn: builtinBytesToOct, Name: "bytesToOct"},

		// Modification
		"bytesConcat": {Fn: builtinBytesConcat, Name: "bytesConcat"},

		// Numeric encoding/decoding
		"bytesEncodeInt":   {Fn: builtinBytesEncodeInt, Name: "bytesEncodeInt"},
		"bytesDecodeInt":   {Fn: builtinBytesDecodeInt, Name: "bytesDecodeInt"},
		"bytesEncodeFloat": {Fn: builtinBytesEncodeFloat, Name: "bytesEncodeFloat"},
		"bytesDecodeFloat": {Fn: builtinBytesDecodeFloat, Name: "bytesDecodeFloat"},

		// Search
		"bytesContains":   {Fn: builtinBytesContains, Name: "bytesContains"},
		"bytesIndexOf":    {Fn: builtinBytesIndexOf, Name: "bytesIndexOf"},
		"bytesStartsWith": {Fn: builtinBytesStartsWith, Name: "bytesStartsWith"},
		"bytesEndsWith":   {Fn: builtinBytesEndsWith, Name: "bytesEndsWith"},

		// Split/Join
		"bytesSplit": {Fn: builtinBytesSplit, Name: "bytesSplit"},
		"bytesJoin":  {Fn: builtinBytesJoin, Name: "bytesJoin"},
	}
}

// === Creation ===

func builtinBytesNew(e *Evaluator, args ...Object) Object {
	return bytesNew()
}

func builtinBytesFromString(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bytesFromString expects 1 argument, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("bytesFromString expects String (List<Char>), got %s", args[0].Type())
	}
	// Convert List<Char> to string
	s := listToString(list)
	return bytesFromString(s)
}

func builtinBytesFromList(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bytesFromList expects 1 argument, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("bytesFromList expects List<Int>, got %s", args[0].Type())
	}
	data := make([]byte, list.len())
	for i := 0; i < list.len(); i++ {
		elem := list.get(i)
		intVal, ok := elem.(*Integer)
		if !ok {
			return newError("bytesFromList: element %d is not Int, got %s", i, elem.Type())
		}
		if intVal.Value < 0 || intVal.Value > 255 {
			return newError("bytesFromList: byte value out of range at index %d: %d", i, intVal.Value)
		}
		data[i] = byte(intVal.Value)
	}
	return bytesFromSlice(data)
}

func builtinBytesFromHex(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bytesFromHex expects 1 argument, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("bytesFromHex expects String, got %s", args[0].Type())
	}
	s := listToString(list)
	data, err := hex.DecodeString(s)
	if err != nil {
		return makeFail(stringToList(fmt.Sprintf("invalid hex string: %s", err.Error())))
	}
	return makeOk(bytesFromSlice(data))
}

func builtinBytesFromBin(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bytesFromBin expects 1 argument, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("bytesFromBin expects String, got %s", args[0].Type())
	}
	s := listToString(list)
	if len(s)%8 != 0 {
		return makeFail(stringToList(fmt.Sprintf("binary string must be multiple of 8 bits, got %d bits", len(s))))
	}
	data := make([]byte, len(s)/8)
	for i := 0; i < len(data); i++ {
		byteStr := s[i*8 : (i+1)*8]
		val, err := strconv.ParseUint(byteStr, 2, 8)
		if err != nil {
			return makeFail(stringToList(fmt.Sprintf("invalid binary string at position %d: %s", i*8, err.Error())))
		}
		data[i] = byte(val)
	}
	return makeOk(bytesFromSlice(data))
}

func builtinBytesFromOct(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bytesFromOct expects 1 argument, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("bytesFromOct expects String, got %s", args[0].Type())
	}
	s := listToString(list)
	// Octal: 3 chars = 1 byte (roughly, but actually it's more complex)
	// Let's interpret as: each group of 3 chars is one octal byte
	if len(s)%3 != 0 {
		return makeFail(stringToList(fmt.Sprintf("octal string length must be multiple of 3, got %d", len(s))))
	}
	data := make([]byte, len(s)/3)
	for i := 0; i < len(data); i++ {
		octStr := s[i*3 : (i+1)*3]
		val, err := strconv.ParseUint(octStr, 8, 8)
		if err != nil {
			return makeFail(stringToList(fmt.Sprintf("invalid octal string at position %d: %s", i*3, err.Error())))
		}
		data[i] = byte(val)
	}
	return makeOk(bytesFromSlice(data))
}

// === Access ===

func builtinBytesSlice(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("bytesSlice expects 3 arguments, got %d", len(args))
	}
	b, ok := args[0].(*Bytes)
	if !ok {
		return newError("bytesSlice expects Bytes as first argument, got %s", args[0].Type())
	}
	start, ok := args[1].(*Integer)
	if !ok {
		return newError("bytesSlice expects Int as second argument, got %s", args[1].Type())
	}
	end, ok := args[2].(*Integer)
	if !ok {
		return newError("bytesSlice expects Int as third argument, got %s", args[2].Type())
	}

	from := int(start.Value)
	to := int(end.Value)
	length := b.Len()

	if from < 0 || to > length || from > to {
		return makeFail(stringToList(fmt.Sprintf("bytesSlice: indices [%d, %d) out of bounds for bytes of length %d", from, to, length)))
	}

	return makeOk(b.slice(from, to))
}

// === Conversion ===

func builtinBytesToString(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bytesToString expects 1 argument, got %d", len(args))
	}
	b, ok := args[0].(*Bytes)
	if !ok {
		return newError("bytesToString expects Bytes, got %s", args[0].Type())
	}
	// Check if valid UTF-8
	if !utf8.Valid(b.ToSlice()) {
		return makeFail(stringToList("bytes are not valid UTF-8"))
	}
	return makeOk(stringToList(b.toString()))
}

func builtinBytesToList(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bytesToList expects 1 argument, got %d", len(args))
	}
	b, ok := args[0].(*Bytes)
	if !ok {
		return newError("bytesToList expects Bytes, got %s", args[0].Type())
	}
	elements := make([]Object, b.Len())
	for i, byteVal := range b.ToSlice() {
		elements[i] = &Integer{Value: int64(byteVal)}
	}
	return newList(elements)
}

func builtinBytesToHex(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bytesToHex expects 1 argument, got %d", len(args))
	}
	b, ok := args[0].(*Bytes)
	if !ok {
		return newError("bytesToHex expects Bytes, got %s", args[0].Type())
	}
	return stringToList(b.toHex())
}

func builtinBytesToBin(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bytesToBin expects 1 argument, got %d", len(args))
	}
	b, ok := args[0].(*Bytes)
	if !ok {
		return newError("bytesToBin expects Bytes, got %s", args[0].Type())
	}
	var sb strings.Builder
	for _, byteVal := range b.ToSlice() {
		sb.WriteString(fmt.Sprintf("%08b", byteVal))
	}
	return stringToList(sb.String())
}

func builtinBytesToOct(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bytesToOct expects 1 argument, got %d", len(args))
	}
	b, ok := args[0].(*Bytes)
	if !ok {
		return newError("bytesToOct expects Bytes, got %s", args[0].Type())
	}
	var sb strings.Builder
	for _, byteVal := range b.ToSlice() {
		sb.WriteString(fmt.Sprintf("%03o", byteVal))
	}
	return stringToList(sb.String())
}

// === Modification ===

func builtinBytesConcat(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("bytesConcat expects 2 arguments, got %d", len(args))
	}
	b1, ok := args[0].(*Bytes)
	if !ok {
		return newError("bytesConcat expects Bytes as first argument, got %s", args[0].Type())
	}
	b2, ok := args[1].(*Bytes)
	if !ok {
		return newError("bytesConcat expects Bytes as second argument, got %s", args[1].Type())
	}
	return b1.Concat(b2)
}

// === Numeric encoding/decoding ===

func builtinBytesEncodeInt(e *Evaluator, args ...Object) Object {
	if len(args) < 2 || len(args) > 3 {
		return newError("bytesEncodeInt expects 2-3 arguments, got %d", len(args))
	}
	val, ok := args[0].(*Integer)
	if !ok {
		return newError("bytesEncodeInt expects Int as first argument, got %s", args[0].Type())
	}
	size, ok := args[1].(*Integer)
	if !ok {
		return newError("bytesEncodeInt expects Int as second argument, got %s", args[1].Type())
	}

	// Default big-endian
	endianness := "big"
	if len(args) == 3 {
		endianList, ok := args[2].(*List)
		if !ok {
			return newError("bytesEncodeInt expects String as third argument, got %s", args[2].Type())
		}
		endianness = listToString(endianList)
	}

	isLittle := strings.HasPrefix(endianness, "little")
	if strings.HasPrefix(endianness, "native") {
		isLittle = isNativeLittleEndian()
	}

	data := make([]byte, size.Value)
	switch size.Value {
	case 1:
		data[0] = byte(val.Value)
	case 2:
		if isLittle {
			binary.LittleEndian.PutUint16(data, uint16(val.Value))
		} else {
			binary.BigEndian.PutUint16(data, uint16(val.Value))
		}
	case 4:
		if isLittle {
			binary.LittleEndian.PutUint32(data, uint32(val.Value))
		} else {
			binary.BigEndian.PutUint32(data, uint32(val.Value))
		}
	case 8:
		if isLittle {
			binary.LittleEndian.PutUint64(data, uint64(val.Value))
		} else {
			binary.BigEndian.PutUint64(data, uint64(val.Value))
		}
	default:
		return newError("bytesEncodeInt: size must be 1, 2, 4, or 8, got %d", size.Value)
	}
	return bytesFromSlice(data)
}

func builtinBytesDecodeInt(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("bytesDecodeInt expects 1-2 arguments, got %d", len(args))
	}
	b, ok := args[0].(*Bytes)
	if !ok {
		return newError("bytesDecodeInt expects Bytes as first argument, got %s", args[0].Type())
	}

	// Default big-endian
	endianness := "big"
	if len(args) == 2 {
		endianList, ok := args[1].(*List)
		if !ok {
			return newError("bytesDecodeInt expects String as second argument, got %s", args[1].Type())
		}
		endianness = listToString(endianList)
	}

	isLittle := strings.HasPrefix(endianness, "little")
	if strings.HasPrefix(endianness, "native") {
		isLittle = isNativeLittleEndian()
	}

	data := b.ToSlice()
	var result int64
	switch len(data) {
	case 1:
		result = int64(data[0])
	case 2:
		if isLittle {
			result = int64(binary.LittleEndian.Uint16(data))
		} else {
			result = int64(binary.BigEndian.Uint16(data))
		}
	case 4:
		if isLittle {
			result = int64(binary.LittleEndian.Uint32(data))
		} else {
			result = int64(binary.BigEndian.Uint32(data))
		}
	case 8:
		if isLittle {
			result = int64(binary.LittleEndian.Uint64(data))
		} else {
			result = int64(binary.BigEndian.Uint64(data))
		}
	default:
		return newError("bytesDecodeInt: bytes length must be 1, 2, 4, or 8, got %d", len(data))
	}

	// Handle signed if specified
	if strings.Contains(endianness, "signed") {
		switch len(data) {
		case 1:
			result = int64(int8(data[0]))
		case 2:
			if isLittle {
				result = int64(int16(binary.LittleEndian.Uint16(data)))
			} else {
				result = int64(int16(binary.BigEndian.Uint16(data)))
			}
		case 4:
			if isLittle {
				result = int64(int32(binary.LittleEndian.Uint32(data)))
			} else {
				result = int64(int32(binary.BigEndian.Uint32(data)))
			}
		}
	}

	return &Integer{Value: result}
}

func builtinBytesEncodeFloat(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("bytesEncodeFloat expects 2 arguments, got %d", len(args))
	}
	val, ok := args[0].(*Float)
	if !ok {
		return newError("bytesEncodeFloat expects Float as first argument, got %s", args[0].Type())
	}
	size, ok := args[1].(*Integer)
	if !ok {
		return newError("bytesEncodeFloat expects Int as second argument, got %s", args[1].Type())
	}

	data := make([]byte, size.Value)
	switch size.Value {
	case 4:
		binary.BigEndian.PutUint32(data, math.Float32bits(float32(val.Value)))
	case 8:
		binary.BigEndian.PutUint64(data, math.Float64bits(val.Value))
	default:
		return newError("bytesEncodeFloat: size must be 4 or 8, got %d", size.Value)
	}
	return bytesFromSlice(data)
}

func builtinBytesDecodeFloat(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("bytesDecodeFloat expects 2 arguments, got %d", len(args))
	}
	b, ok := args[0].(*Bytes)
	if !ok {
		return newError("bytesDecodeFloat expects Bytes as first argument, got %s", args[0].Type())
	}
	size, ok := args[1].(*Integer)
	if !ok {
		return newError("bytesDecodeFloat expects Int as second argument, got %s", args[1].Type())
	}

	data := b.ToSlice()
	if int64(len(data)) != size.Value {
		return newError("bytesDecodeFloat: bytes length %d doesn't match size %d", len(data), size.Value)
	}

	var result float64
	switch size.Value {
	case 4:
		result = float64(math.Float32frombits(binary.BigEndian.Uint32(data)))
	case 8:
		result = math.Float64frombits(binary.BigEndian.Uint64(data))
	default:
		return newError("bytesDecodeFloat: size must be 4 or 8, got %d", size.Value)
	}
	return &Float{Value: result}
}

// === Search ===

func builtinBytesContains(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("bytesContains expects 2 arguments, got %d", len(args))
	}
	haystack, ok := args[0].(*Bytes)
	if !ok {
		return newError("bytesContains expects Bytes as first argument, got %s", args[0].Type())
	}
	needle, ok := args[1].(*Bytes)
	if !ok {
		return newError("bytesContains expects Bytes as second argument, got %s", args[1].Type())
	}
	return &Boolean{Value: bytes.Contains(haystack.ToSlice(), needle.ToSlice())}
}

func builtinBytesIndexOf(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("bytesIndexOf expects 2 arguments, got %d", len(args))
	}
	haystack, ok := args[0].(*Bytes)
	if !ok {
		return newError("bytesIndexOf expects Bytes as first argument, got %s", args[0].Type())
	}
	needle, ok := args[1].(*Bytes)
	if !ok {
		return newError("bytesIndexOf expects Bytes as second argument, got %s", args[1].Type())
	}
	idx := bytes.Index(haystack.ToSlice(), needle.ToSlice())
	if idx < 0 {
		return makeNone()
	}
	return makeSome(&Integer{Value: int64(idx)})
}

func builtinBytesStartsWith(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("bytesStartsWith expects 2 arguments, got %d", len(args))
	}
	b, ok := args[0].(*Bytes)
	if !ok {
		return newError("bytesStartsWith expects Bytes as first argument, got %s", args[0].Type())
	}
	prefix, ok := args[1].(*Bytes)
	if !ok {
		return newError("bytesStartsWith expects Bytes as second argument, got %s", args[1].Type())
	}
	return &Boolean{Value: bytes.HasPrefix(b.ToSlice(), prefix.ToSlice())}
}

func builtinBytesEndsWith(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("bytesEndsWith expects 2 arguments, got %d", len(args))
	}
	b, ok := args[0].(*Bytes)
	if !ok {
		return newError("bytesEndsWith expects Bytes as first argument, got %s", args[0].Type())
	}
	suffix, ok := args[1].(*Bytes)
	if !ok {
		return newError("bytesEndsWith expects Bytes as second argument, got %s", args[1].Type())
	}
	return &Boolean{Value: bytes.HasSuffix(b.ToSlice(), suffix.ToSlice())}
}

// === Split/Join ===

func builtinBytesSplit(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("bytesSplit expects 2 arguments, got %d", len(args))
	}
	b, ok := args[0].(*Bytes)
	if !ok {
		return newError("bytesSplit expects Bytes as first argument, got %s", args[0].Type())
	}
	sep, ok := args[1].(*Bytes)
	if !ok {
		return newError("bytesSplit expects Bytes as second argument, got %s", args[1].Type())
	}
	parts := bytes.Split(b.ToSlice(), sep.ToSlice())
	elements := make([]Object, len(parts))
	for i, part := range parts {
		elements[i] = bytesFromSlice(part)
	}
	return newList(elements)
}

func builtinBytesJoin(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("bytesJoin expects 2 arguments, got %d", len(args))
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("bytesJoin expects List<Bytes> as first argument, got %s", args[0].Type())
	}
	sep, ok := args[1].(*Bytes)
	if !ok {
		return newError("bytesJoin expects Bytes as second argument, got %s", args[1].Type())
	}

	parts := make([][]byte, list.len())
	for i := 0; i < list.len(); i++ {
		elem := list.get(i)
		b, ok := elem.(*Bytes)
		if !ok {
			return newError("bytesJoin: element %d is not Bytes, got %s", i, elem.Type())
		}
		parts[i] = b.ToSlice()
	}
	return bytesFromSlice(bytes.Join(parts, sep.ToSlice()))
}

func SetBytesBuiltinTypes(builtins map[string]*Builtin) {
	bytesType := typesystem.TCon{Name: "Bytes"}
	stringType := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{typesystem.TCon{Name: "Char"}}}
	intType := typesystem.TCon{Name: "Int"}
	boolType := typesystem.TCon{Name: "Bool"}
	floatType := typesystem.TCon{Name: "Float"}
	listIntType := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{intType}}
	listBytesType := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{bytesType}}

	// Result types
	resultBytesType := typesystem.TApp{Constructor: typesystem.TCon{Name: "Result"}, Args: []typesystem.Type{stringType, bytesType}}
	resultStringType := typesystem.TApp{Constructor: typesystem.TCon{Name: "Result"}, Args: []typesystem.Type{stringType, stringType}}
	resultFloatType := typesystem.TApp{Constructor: typesystem.TCon{Name: "Result"}, Args: []typesystem.Type{stringType, floatType}}

	// Option types
	optionIntType := typesystem.TApp{Constructor: typesystem.TCon{Name: "Option"}, Args: []typesystem.Type{intType}}

	types := map[string]typesystem.Type{
		"bytesNew":        typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: bytesType},
		"bytesFromString": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: bytesType},
		"bytesFromList":   typesystem.TFunc{Params: []typesystem.Type{listIntType}, ReturnType: bytesType},
		"bytesFromHex":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultBytesType},
		"bytesFromBin":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultBytesType},
		"bytesFromOct":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultBytesType},

		"bytesSlice": typesystem.TFunc{Params: []typesystem.Type{bytesType, intType, intType}, ReturnType: bytesType},

		"bytesToString": typesystem.TFunc{Params: []typesystem.Type{bytesType}, ReturnType: resultStringType},
		"bytesToList":   typesystem.TFunc{Params: []typesystem.Type{bytesType}, ReturnType: listIntType},
		"bytesToHex":    typesystem.TFunc{Params: []typesystem.Type{bytesType}, ReturnType: stringType},
		"bytesToBin":    typesystem.TFunc{Params: []typesystem.Type{bytesType}, ReturnType: stringType},
		"bytesToOct":    typesystem.TFunc{Params: []typesystem.Type{bytesType}, ReturnType: stringType},

		"bytesConcat": typesystem.TFunc{Params: []typesystem.Type{bytesType, bytesType}, ReturnType: bytesType},

		"bytesEncodeInt":   typesystem.TFunc{Params: []typesystem.Type{intType, intType, stringType}, ReturnType: bytesType, DefaultCount: 1}, // endianness optional
		"bytesDecodeInt":   typesystem.TFunc{Params: []typesystem.Type{bytesType, stringType}, ReturnType: intType, DefaultCount: 1},          // endianness optional
		"bytesEncodeFloat": typesystem.TFunc{Params: []typesystem.Type{floatType, intType}, ReturnType: bytesType},
		"bytesDecodeFloat": typesystem.TFunc{Params: []typesystem.Type{bytesType, intType}, ReturnType: resultFloatType},

		"bytesContains":   typesystem.TFunc{Params: []typesystem.Type{bytesType, bytesType}, ReturnType: boolType},
		"bytesIndexOf":    typesystem.TFunc{Params: []typesystem.Type{bytesType, bytesType}, ReturnType: optionIntType},
		"bytesStartsWith": typesystem.TFunc{Params: []typesystem.Type{bytesType, bytesType}, ReturnType: boolType},
		"bytesEndsWith":   typesystem.TFunc{Params: []typesystem.Type{bytesType, bytesType}, ReturnType: boolType},

		"bytesSplit": typesystem.TFunc{Params: []typesystem.Type{bytesType, bytesType}, ReturnType: listBytesType},
		"bytesJoin":  typesystem.TFunc{Params: []typesystem.Type{listBytesType, bytesType}, ReturnType: bytesType},
	}

	// Override decodeFloat return type if needed based on my analysis
	types["bytesDecodeFloat"] = typesystem.TFunc{Params: []typesystem.Type{bytesType, intType}, ReturnType: floatType}
	// Same for decodeInt
	types["bytesDecodeInt"] = typesystem.TFunc{Params: []typesystem.Type{bytesType, stringType}, ReturnType: intType, DefaultCount: 1}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

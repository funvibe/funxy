package evaluator

import (
	"encoding/hex"
	"fmt"
	"math"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/typesystem"
	"unsafe"
)

// BitsBuiltins returns built-in functions for lib/bits virtual package
func BitsBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// Creation
		"bitsNew":        {Fn: builtinBitsNew, Name: "bitsNew"},
		"bitsFromBytes":  {Fn: builtinBitsFromBytes, Name: "bitsFromBytes"},
		"bitsFromBinary": {Fn: builtinBitsFromBinary, Name: "bitsFromBinary"},
		"bitsFromHex":    {Fn: builtinBitsFromHex, Name: "bitsFromHex"},
		"bitsFromOctal":  {Fn: builtinBitsFromOctal, Name: "bitsFromOctal"},

		// Conversion
		"bitsToBytes":  {Fn: builtinBitsToBytes, Name: "bitsToBytes"},
		"bitsToBinary": {Fn: builtinBitsToBinary, Name: "bitsToBinary"},
		"bitsToHex":    {Fn: builtinBitsToHex, Name: "bitsToHex"},

		// Access
		"bitsSlice": {Fn: builtinBitsSlice, Name: "bitsSlice"},
		"bitsGet":   {Fn: builtinBitsGet, Name: "bitsGet"},

		// Modification
		"bitsConcat":   {Fn: builtinBitsConcat, Name: "bitsConcat"},
		"bitsSet":      {Fn: builtinBitsSet, Name: "bitsSet"},
		"bitsPadLeft":  {Fn: builtinBitsPadLeft, Name: "bitsPadLeft"},
		"bitsPadRight": {Fn: builtinBitsPadRight, Name: "bitsPadRight"},

		// Numeric operations
		"bitsAddInt":   {Fn: builtinBitsAddInt, Name: "bitsAddInt"},
		"bitsAddFloat": {Fn: builtinBitsAddFloat, Name: "bitsAddFloat"},

		// Pattern matching API
		"bitsExtract": {Fn: builtinBitsExtract, Name: "bitsExtract"},
		"bitsInt":     {Fn: builtinBitsInt, Name: "bitsInt"},
		"bitsBytes":   {Fn: builtinBitsBytes, Name: "bitsBytes"},
		"bitsRest":    {Fn: builtinBitsRest, Name: "bitsRest"},
	}
}

// SetBitsBuiltinTypes sets type information for bits builtins
func SetBitsBuiltinTypes(builtins map[string]*Builtin) {
	bitsType := typesystem.TCon{Name: config.BitsTypeName}
	bytesType := typesystem.TCon{Name: config.BytesTypeName}
	intType := typesystem.TCon{Name: "Int"}
	charType := typesystem.TCon{Name: "Char"}
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{charType},
	}

	optionInt := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.OptionTypeName},
		Args:        []typesystem.Type{intType},
	}

	resultStringBits := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ResultTypeName},
		Args:        []typesystem.Type{stringType, bitsType},
	}

	recordType := typesystem.TRecord{IsOpen: true, Fields: map[string]typesystem.Type{}}
	specListType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{recordType},
	}
	resultMapType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ResultTypeName},
		Args:        []typesystem.Type{stringType, typesystem.TCon{Name: "Map"}},
	}

	sizeType := typesystem.TUnion{Types: []typesystem.Type{intType, stringType}}

	types := map[string]typesystem.Type{
		"bitsNew":        typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: bitsType},
		"bitsFromBytes":  typesystem.TFunc{Params: []typesystem.Type{bytesType}, ReturnType: bitsType},
		"bitsFromBinary": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringBits},
		"bitsFromHex":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringBits},
		"bitsFromOctal":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringBits},

		"bitsToBytes":  typesystem.TFunc{Params: []typesystem.Type{bitsType, stringType}, ReturnType: bytesType, DefaultCount: 1},
		"bitsToBinary": typesystem.TFunc{Params: []typesystem.Type{bitsType}, ReturnType: stringType},
		"bitsToHex":    typesystem.TFunc{Params: []typesystem.Type{bitsType}, ReturnType: stringType},

		"bitsSlice": typesystem.TFunc{Params: []typesystem.Type{bitsType, intType, intType}, ReturnType: bitsType},
		"bitsGet":   typesystem.TFunc{Params: []typesystem.Type{bitsType, intType}, ReturnType: optionInt},

		"bitsConcat":   typesystem.TFunc{Params: []typesystem.Type{bitsType, bitsType}, ReturnType: bitsType},
		"bitsSet":      typesystem.TFunc{Params: []typesystem.Type{bitsType, intType, intType}, ReturnType: bitsType},
		"bitsPadLeft":  typesystem.TFunc{Params: []typesystem.Type{bitsType, intType}, ReturnType: bitsType},
		"bitsPadRight": typesystem.TFunc{Params: []typesystem.Type{bitsType, intType}, ReturnType: bitsType},

		"bitsAddInt":   typesystem.TFunc{Params: []typesystem.Type{bitsType, intType, intType, stringType}, ReturnType: bitsType, DefaultCount: 1},
		"bitsAddFloat": typesystem.TFunc{Params: []typesystem.Type{bitsType, typesystem.Float, intType}, ReturnType: bitsType},

		"bitsExtract": typesystem.TFunc{Params: []typesystem.Type{bitsType, specListType}, ReturnType: resultMapType},
		"bitsInt":     typesystem.TFunc{Params: []typesystem.Type{stringType, intType, stringType}, ReturnType: recordType, DefaultCount: 1},
		"bitsBytes":   typesystem.TFunc{Params: []typesystem.Type{stringType, sizeType}, ReturnType: recordType},
		"bitsRest":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: recordType},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

// === Creation ===

func builtinBitsNew(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("bitsNew expects 0 arguments, got %d", len(args))
	}
	return bitsNew()
}

func builtinBitsFromBytes(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bitsFromBytes expects 1 argument, got %d", len(args))
	}
	b, ok := args[0].(*Bytes)
	if !ok {
		return newError("bitsFromBytes expects Bytes, got %s", args[0].Type())
	}
	return bitsFromBytes(b)
}

func builtinBitsFromBinary(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bitsFromBinary expects 1 argument, got %d", len(args))
	}
	s := objectToString(args[0])
	bits, err := bitsFromBinary(s)
	if err != nil {
		return makeFailStr(err.Error())
	}
	return makeOk(bits)
}

func builtinBitsFromHex(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bitsFromHex expects 1 argument, got %d", len(args))
	}
	s := objectToString(args[0])
	bits, err := bitsFromHex(s)
	if err != nil {
		return makeFailStr(err.Error())
	}
	return makeOk(bits)
}

func builtinBitsFromOctal(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bitsFromOctal expects 1 argument, got %d", len(args))
	}
	s := objectToString(args[0])
	bits, err := bitsFromOctal(s)
	if err != nil {
		return makeFailStr(err.Error())
	}
	return makeOk(bits)
}

// === Conversion ===

func builtinBitsToBytes(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("bitsToBytes expects 1-2 arguments, got %d", len(args))
	}
	bits, ok := args[0].(*Bits)
	if !ok {
		return newError("bitsToBytes expects Bits as first argument, got %s", args[0].Type())
	}
	// Default padding is "low" (per Erlang spec: big endian = MSB first)
	padding := "low"
	if len(args) == 2 {
		padding = objectToString(args[1])
	}
	return bits.toBytes(padding)
}

func builtinBitsToBinary(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bitsToBinary expects 1 argument, got %d", len(args))
	}
	bits, ok := args[0].(*Bits)
	if !ok {
		return newError("bitsToBinary expects Bits, got %s", args[0].Type())
	}
	return goStringToList(bits.toBinary())
}

func builtinBitsToHex(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bitsToHex expects 1 argument, got %d", len(args))
	}
	bits, ok := args[0].(*Bits)
	if !ok {
		return newError("bitsToHex expects Bits, got %s", args[0].Type())
	}
	// Convert to bytes first (with low padding), then to hex
	bytes := bits.toBytes("low")
	return goStringToList(hex.EncodeToString(bytes.data))
}

// === Access ===

func builtinBitsSlice(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("bitsSlice expects 3 arguments, got %d", len(args))
	}
	bits, ok := args[0].(*Bits)
	if !ok {
		return newError("bitsSlice expects Bits as first argument, got %s", args[0].Type())
	}
	start, ok := args[1].(*Integer)
	if !ok {
		return newError("bitsSlice expects Int as second argument, got %s", args[1].Type())
	}
	end, ok := args[2].(*Integer)
	if !ok {
		return newError("bitsSlice expects Int as third argument, got %s", args[2].Type())
	}
	return bits.slice(int(start.Value), int(end.Value))
}

func builtinBitsGet(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("bitsGet expects 2 arguments, got %d", len(args))
	}
	bits, ok := args[0].(*Bits)
	if !ok {
		return newError("bitsGet expects Bits as first argument, got %s", args[0].Type())
	}
	idx, ok := args[1].(*Integer)
	if !ok {
		return newError("bitsGet expects Int as second argument, got %s", args[1].Type())
	}
	bit := bits.Get(int(idx.Value))
	if bit < 0 {
		return makeZero()
	}
	return makeSome(&Integer{Value: int64(bit)})
}

// === Modification ===

func builtinBitsConcat(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("bitsConcat expects 2 arguments, got %d", len(args))
	}
	b1, ok := args[0].(*Bits)
	if !ok {
		return newError("bitsConcat expects Bits as first argument, got %s", args[0].Type())
	}
	b2, ok := args[1].(*Bits)
	if !ok {
		return newError("bitsConcat expects Bits as second argument, got %s", args[1].Type())
	}
	return b1.Concat(b2)
}

func builtinBitsSet(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("bitsSet expects 3 arguments, got %d", len(args))
	}
	bits, ok := args[0].(*Bits)
	if !ok {
		return newError("bitsSet expects Bits as first argument, got %s", args[0].Type())
	}
	idx, ok := args[1].(*Integer)
	if !ok {
		return newError("bitsSet expects Int as second argument, got %s", args[1].Type())
	}
	val, ok := args[2].(*Integer)
	if !ok {
		return newError("bitsSet expects Int as third argument (0 or 1), got %s", args[2].Type())
	}

	i := int(idx.Value)
	if i < 0 || i >= bits.length {
		return newError("bit index out of bounds: %d (length: %d)", i, bits.length)
	}

	// Create new Bits with the bit set
	newData := make([]byte, len(bits.data))
	copy(newData, bits.data)

	byteIdx := i / 8
	bitIdx := 7 - (i % 8)
	if val.Value != 0 {
		newData[byteIdx] |= 1 << bitIdx
	} else {
		newData[byteIdx] &^= 1 << bitIdx
	}

	return &Bits{data: newData, length: bits.length}
}

func builtinBitsPadLeft(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("bitsPadLeft expects 2 arguments, got %d", len(args))
	}
	bits, ok := args[0].(*Bits)
	if !ok {
		return newError("bitsPadLeft expects Bits as first argument, got %s", args[0].Type())
	}
	targetLen, ok := args[1].(*Integer)
	if !ok {
		return newError("bitsPadLeft expects Int as second argument, got %s", args[1].Type())
	}

	target := int(targetLen.Value)
	if target <= bits.length {
		return bits
	}

	padBits := target - bits.length
	// Create zero bits to prepend
	zeroBits := &Bits{data: make([]byte, (padBits+7)/8), length: padBits}
	return zeroBits.Concat(bits)
}

func builtinBitsPadRight(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("bitsPadRight expects 2 arguments, got %d", len(args))
	}
	bits, ok := args[0].(*Bits)
	if !ok {
		return newError("bitsPadRight expects Bits as first argument, got %s", args[0].Type())
	}
	targetLen, ok := args[1].(*Integer)
	if !ok {
		return newError("bitsPadRight expects Int as second argument, got %s", args[1].Type())
	}

	target := int(targetLen.Value)
	if target <= bits.length {
		return bits
	}

	padBits := target - bits.length
	// Create zero bits to append
	zeroBits := &Bits{data: make([]byte, (padBits+7)/8), length: padBits}
	return bits.Concat(zeroBits)
}

// === Numeric operations ===

func builtinBitsAddInt(e *Evaluator, args ...Object) Object {
	if len(args) < 3 || len(args) > 4 {
		return newError("bitsAddInt expects 3-4 arguments, got %d", len(args))
	}
	bits, ok := args[0].(*Bits)
	if !ok {
		return newError("bitsAddInt expects Bits as first argument, got %s", args[0].Type())
	}
	n, ok := args[1].(*Integer)
	if !ok {
		return newError("bitsAddInt expects Int as second argument, got %s", args[1].Type())
	}
	size, ok := args[2].(*Integer)
	if !ok {
		return newError("bitsAddInt expects Int as third argument (size), got %s", args[2].Type())
	}

	spec := "big"
	if len(args) == 4 {
		spec = objectToString(args[3])
	}

	// Encode integer to bits
	intBits := encodeIntToBits(n.Value, int(size.Value), spec)
	return bits.Concat(intBits)
}

func builtinBitsAddFloat(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("bitsAddFloat expects 3 arguments, got %d", len(args))
	}
	bits, ok := args[0].(*Bits)
	if !ok {
		return newError("bitsAddFloat expects Bits as first argument, got %s", args[0].Type())
	}
	f, ok := args[1].(*Float)
	if !ok {
		return newError("bitsAddFloat expects Float as second argument, got %s", args[1].Type())
	}
	size, ok := args[2].(*Integer)
	if !ok {
		return newError("bitsAddFloat expects Int as third argument (size), got %s", args[2].Type())
	}

	// Encode float to bits
	floatBits := encodeFloatToBits(f.Value, int(size.Value))
	return bits.Concat(floatBits)
}

// isNativeLittleEndian returns true if the system is little-endian
func isNativeLittleEndian() bool {
	var x uint16 = 0x0001
	return *(*byte)(unsafe.Pointer(&x)) == 0x01
}

// encodeIntToBits encodes an integer to Bits
func encodeIntToBits(val int64, sizeBits int, spec string) *Bits {
	numBytes := (sizeBits + 7) / 8
	data := make([]byte, numBytes)

	// Determine endianness from spec
	littleEndian := false
	if len(spec) >= 6 && spec[:6] == "little" {
		littleEndian = true
	} else if len(spec) >= 6 && spec[:6] == "native" {
		littleEndian = isNativeLittleEndian()
	}

	// Encode value
	for i := 0; i < numBytes && i < 8; i++ {
		byteVal := byte((val >> (8 * i)) & 0xFF)
		if littleEndian {
			data[i] = byteVal
		} else {
			data[numBytes-1-i] = byteVal
		}
	}

	return &Bits{data: data, length: sizeBits}
}

// encodeFloatToBits encodes a float to Bits (IEEE 754)
func encodeFloatToBits(val float64, sizeBits int) *Bits {
	var data []byte
	if sizeBits == 32 {
		bits := math.Float32bits(float32(val))
		data = []byte{
			byte(bits >> 24),
			byte(bits >> 16),
			byte(bits >> 8),
			byte(bits),
		}
	} else {
		bits := math.Float64bits(val)
		data = []byte{
			byte(bits >> 56),
			byte(bits >> 48),
			byte(bits >> 40),
			byte(bits >> 32),
			byte(bits >> 24),
			byte(bits >> 16),
			byte(bits >> 8),
			byte(bits),
		}
	}
	return &Bits{data: data, length: sizeBits}
}

// === Pattern matching API ===

// SpecEntry represents a field specification for pattern matching
type SpecEntry struct {
	Name string
	Kind string // "int", "bytes", "rest"
	Size int    // Size in bits (for int) or bytes (for bytes), 0 for rest
	Spec string // Additional spec like "big-signed"
}

func builtinBitsInt(e *Evaluator, args ...Object) Object {
	if len(args) < 2 || len(args) > 3 {
		return newError("bitsInt expects 2-3 arguments, got %d", len(args))
	}
	name := objectToString(args[0])
	size, ok := args[1].(*Integer)
	if !ok {
		return newError("bitsInt expects Int as second argument, got %s", args[1].Type())
	}

	spec := "big"
	if len(args) == 3 {
		spec = objectToString(args[2])
	}

	return NewRecord(map[string]Object{
		"_specKind": goStringToList("int"),
		"name":      goStringToList(name),
		"size":      &Integer{Value: size.Value},
		"spec":      goStringToList(spec),
	})
}

func builtinBitsBytes(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("bitsBytes expects 2 arguments, got %d", len(args))
	}
	name := objectToString(args[0])

	var sizeVal int64
	switch s := args[1].(type) {
	case *Integer:
		sizeVal = s.Value
	case *List:
		// Dynamic size from field name - store as string reference
		sizeRefStr := objectToString(s)
		return NewRecord(map[string]Object{
			"_specKind": goStringToList("bytes"),
			"name":      goStringToList(name),
			"sizeRef":   goStringToList(sizeRefStr),
		})
	default:
		// Try as List<Char> (string)
		sizeStr := objectToString(args[1])
		return NewRecord(map[string]Object{
			"_specKind": goStringToList("bytes"),
			"name":      goStringToList(name),
			"sizeRef":   goStringToList(sizeStr),
		})
	}

	return NewRecord(map[string]Object{
		"_specKind": goStringToList("bytes"),
		"name":      goStringToList(name),
		"size":      &Integer{Value: sizeVal},
	})
}

func builtinBitsRest(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bitsRest expects 1 argument, got %d", len(args))
	}
	name := objectToString(args[0])

	return NewRecord(map[string]Object{
		"_specKind": goStringToList("rest"),
		"name":      goStringToList(name),
	})
}

func builtinBitsExtract(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("bitsExtract expects 2 arguments, got %d", len(args))
	}
	bits, ok := args[0].(*Bits)
	if !ok {
		return newError("bitsExtract expects Bits as first argument, got %s", args[0].Type())
	}
	specs, ok := args[1].(*List)
	if !ok {
		return newError("bitsExtract expects List as second argument, got %s", args[1].Type())
	}

	result := newMap()
	offset := 0

	for i := 0; i < specs.len(); i++ {
		spec := specs.get(i)
		rec, ok := spec.(*RecordInstance)
		if !ok {
			return makeFailStr(fmt.Sprintf("spec %d is not a valid spec record", i))
		}

		kindObj := rec.Get("_specKind")
		if kindObj == nil {
			return makeFailStr(fmt.Sprintf("spec %d missing _specKind", i))
		}
		kind := objectToString(kindObj)

		nameObj := rec.Get("name")
		if nameObj == nil {
			return makeFailStr(fmt.Sprintf("spec %d missing name", i))
		}
		name := objectToString(nameObj)

		switch kind {
		case "int":
			sizeObj := rec.Get("size")
			if sizeObj == nil {
				return makeFailStr(fmt.Sprintf("spec %s missing size", name))
			}
			size := int(sizeObj.(*Integer).Value)

			if offset+size > bits.length {
				return makeFailStr(fmt.Sprintf("not enough bits for field %s: need %d, have %d", name, size, bits.length-offset))
			}

			specStr := "big"
			if specObj := rec.Get("spec"); specObj != nil {
				specStr = objectToString(specObj)
			}

			// Extract integer value
			fieldBits := bits.slice(offset, offset+size)
			val := decodeBitsToInt(fieldBits, specStr)
			result = result.put(goStringToList(name), &Integer{Value: val})
			offset += size

		case "bytes":
			var numBytes int
			if sizeRef := rec.Get("sizeRef"); sizeRef != nil {
				// Dynamic size from another field
				refName := objectToString(sizeRef)
				refVal := result.get(goStringToList(refName))
				if refVal == nil {
					return makeFailStr(fmt.Sprintf("referenced field %s not found for %s", refName, name))
				}
				numBytes = int(refVal.(*Integer).Value)
			} else if sizeObj := rec.Get("size"); sizeObj != nil {
				numBytes = int(sizeObj.(*Integer).Value)
			} else {
				return makeFailStr(fmt.Sprintf("spec %s missing size or sizeRef", name))
			}

			numBits := numBytes * 8
			if offset+numBits > bits.length {
				return makeFailStr(fmt.Sprintf("not enough bits for field %s: need %d, have %d", name, numBits, bits.length-offset))
			}

			fieldBits := bits.slice(offset, offset+numBits)
			fieldBytes := fieldBits.toBytes("low")
			result = result.put(goStringToList(name), fieldBytes)
			offset += numBits

		case "rest":
			restBits := bits.slice(offset, bits.length)
			result = result.put(goStringToList(name), restBits)
			offset = bits.length
		}
	}

	return makeOk(result)
}

// decodeBitsToInt decodes bits to an integer
func decodeBitsToInt(bits *Bits, spec string) int64 {
	if bits.length == 0 {
		return 0
	}

	littleEndian := false
	signed := false

	// Parse spec
	if len(spec) >= 6 && spec[:6] == "little" {
		littleEndian = true
	} else if len(spec) >= 6 && spec[:6] == "native" {
		littleEndian = isNativeLittleEndian()
	}
	if len(spec) >= 6 && spec[len(spec)-6:] == "signed" {
		signed = true
	}

	// Read bits directly, bit by bit
	var val uint64
	if littleEndian {
		// Read LSB first
		for i := bits.length - 1; i >= 0; i-- {
			val = (val << 1) | uint64(bits.Get(i))
		}
	} else {
		// Read MSB first (big endian)
		for i := 0; i < bits.length; i++ {
			val = (val << 1) | uint64(bits.Get(i))
		}
	}

	if signed && bits.length > 0 {
		// Check sign bit
		signBit := uint64(1) << (bits.length - 1)
		if val&signBit != 0 {
			// Negative number - sign extend
			val |= ^((1 << bits.length) - 1)
		}
	}

	return int64(val)
}

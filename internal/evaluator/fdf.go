package evaluator

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"github.com/funvibe/funxy/internal/typesystem"
	"sort"
)

const FDF_MAGIC = "FDF1"

const (
	maxFDFStringBytes    uint32 = 8 << 20  // 8 MiB
	maxFDFPayloadBytes   uint32 = 64 << 20 // 64 MiB for bytes/bits payloads
	maxFDFCollectionSize uint32 = 1 << 20  // 1,048,576 elements
)

const (
	fdfNil byte = iota
	fdfInteger
	fdfFloat
	fdfBoolean
	fdfChar
	fdfBigInt
	fdfRational
	fdfBytes
	fdfBits
	fdfList
	fdfTuple
	fdfMap
	fdfRecordInstance
	fdfDataInstance
	fdfConstructor
	fdfTypeObject
	fdfRange
)

func writeString(buf *bytes.Buffer, s string) error {
	if err := binary.Write(buf, binary.BigEndian, uint32(len(s))); err != nil {
		return err
	}
	buf.WriteString(s)
	return nil
}

func readBoundedLen32(buf *bytes.Reader, max uint32, what string) (uint32, error) {
	var l uint32
	if err := binary.Read(buf, binary.BigEndian, &l); err != nil {
		return 0, err
	}
	if l > max {
		return 0, fmt.Errorf("FDF %s length %d exceeds limit %d", what, l, max)
	}
	return l, nil
}

func readString(buf *bytes.Reader) (string, error) {
	l, err := readBoundedLen32(buf, maxFDFStringBytes, "string")
	if err != nil {
		return "", err
	}
	b := make([]byte, l)
	if _, err := io.ReadFull(buf, b); err != nil {
		return "", err
	}
	return string(b), nil
}

// SerializeFDF serializes a Funxy Object using the Funxy Data Format (FDF).
func SerializeFDF(val Object) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(FDF_MAGIC)
	if err := encodeFDF(&buf, val); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeFDF(buf *bytes.Buffer, val Object) error {
	if val == nil {
		buf.WriteByte(fdfNil)
		return nil
	}

	switch v := val.(type) {
	case *Nil:
		buf.WriteByte(fdfNil)
	case *Integer:
		buf.WriteByte(fdfInteger)
		binary.Write(buf, binary.BigEndian, v.Value)
	case *Float:
		buf.WriteByte(fdfFloat)
		binary.Write(buf, binary.BigEndian, v.Value)
	case *Boolean:
		buf.WriteByte(fdfBoolean)
		if v.Value {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}
	case *Char:
		buf.WriteByte(fdfChar)
		binary.Write(buf, binary.BigEndian, v.Value)
	case *BigInt:
		buf.WriteByte(fdfBigInt)
		writeString(buf, v.Value.String())
	case *Rational:
		buf.WriteByte(fdfRational)
		writeString(buf, v.Value.String())
	case *Bytes:
		buf.WriteByte(fdfBytes)
		b := v.ToSlice()
		binary.Write(buf, binary.BigEndian, uint32(len(b)))
		buf.Write(b)
	case *Bits:
		buf.WriteByte(fdfBits)
		if v.length < 0 || v.length > len(v.data)*8 {
			return fmt.Errorf("invalid Bits length %d for payload bytes %d", v.length, len(v.data))
		}
		binary.Write(buf, binary.BigEndian, uint32(len(v.data)))
		buf.Write(v.data)
		binary.Write(buf, binary.BigEndian, uint32(v.length))
	case *List:
		buf.WriteByte(fdfList)
		writeString(buf, v.ElementType)
		elements := v.ToSlice()
		binary.Write(buf, binary.BigEndian, uint32(len(elements)))
		for _, el := range elements {
			if err := encodeFDF(buf, el); err != nil {
				return err
			}
		}
	case *Tuple:
		buf.WriteByte(fdfTuple)
		binary.Write(buf, binary.BigEndian, uint32(len(v.Elements)))
		for _, el := range v.Elements {
			if err := encodeFDF(buf, el); err != nil {
				return err
			}
		}
	case *Map:
		buf.WriteByte(fdfMap)
		writeString(buf, v.KeyType)
		writeString(buf, v.ValType)
		items := v.Items()
		binary.Write(buf, binary.BigEndian, uint32(len(items)))
		for _, item := range items {
			if err := encodeFDF(buf, item.Key); err != nil {
				return err
			}
			if err := encodeFDF(buf, item.Value); err != nil {
				return err
			}
		}
	case *RecordInstance:
		buf.WriteByte(fdfRecordInstance)
		writeString(buf, v.ModuleName)
		writeString(buf, v.TypeName)
		isOpen := byte(0)
		if v.RowPolyExtended {
			isOpen = 1
		}
		buf.WriteByte(isOpen)
		binary.Write(buf, binary.BigEndian, uint32(len(v.Fields)))
		for _, f := range v.Fields {
			writeString(buf, f.Key)
			if err := encodeFDF(buf, f.Value); err != nil {
				return err
			}
		}
	case *DataInstance:
		buf.WriteByte(fdfDataInstance)
		writeString(buf, v.Name)
		binary.Write(buf, binary.BigEndian, uint32(len(v.Fields)))
		for _, el := range v.Fields {
			if err := encodeFDF(buf, el); err != nil {
				return err
			}
		}
		writeString(buf, v.TypeName)
		binary.Write(buf, binary.BigEndian, uint32(len(v.TypeArgs)))
		for _, arg := range v.TypeArgs {
			if err := encodeFDFType(buf, arg); err != nil {
				return err
			}
		}
	case *Constructor:
		buf.WriteByte(fdfConstructor)
		writeString(buf, v.Name)
	case *Range:
		buf.WriteByte(fdfRange)
		if err := encodeFDF(buf, v.Start); err != nil {
			return err
		}
		if err := encodeFDF(buf, v.Next); err != nil {
			return err
		}
		if err := encodeFDF(buf, v.End); err != nil {
			return err
		}
	case *TypeObject:
		buf.WriteByte(fdfTypeObject)
		writeString(buf, v.TypeVal.String())
	default:
		return fmt.Errorf("FDF unsupported type: %s", v.Type())
	}
	return nil
}

// DeserializeFDF deserializes a Funxy Object from a byte array in FDF format.
func DeserializeFDF(data []byte) (Object, error) {
	if len(data) < 4 || string(data[:4]) != FDF_MAGIC {
		return nil, fmt.Errorf("invalid FDF magic bytes")
	}
	buf := bytes.NewReader(data[4:])
	return decodeFDF(buf)
}

func decodeFDF(buf *bytes.Reader) (Object, error) {
	tag, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}

	switch tag {
	case fdfNil:
		return &Nil{}, nil
	case fdfInteger:
		var v int64
		if err := binary.Read(buf, binary.BigEndian, &v); err != nil {
			return nil, err
		}
		return &Integer{Value: v}, nil
	case fdfFloat:
		var v float64
		if err := binary.Read(buf, binary.BigEndian, &v); err != nil {
			return nil, err
		}
		return &Float{Value: v}, nil
	case fdfBoolean:
		b, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}
		return &Boolean{Value: b != 0}, nil
	case fdfChar:
		var v int64
		if err := binary.Read(buf, binary.BigEndian, &v); err != nil {
			return nil, err
		}
		return &Char{Value: v}, nil
	case fdfBigInt:
		s, err := readString(buf)
		if err != nil {
			return nil, err
		}
		v := new(big.Int)
		if _, ok := v.SetString(s, 10); !ok {
			return nil, fmt.Errorf("invalid FDF BigInt literal")
		}
		return &BigInt{Value: v}, nil
	case fdfRational:
		s, err := readString(buf)
		if err != nil {
			return nil, err
		}
		v := new(big.Rat)
		if _, ok := v.SetString(s); !ok {
			return nil, fmt.Errorf("invalid FDF Rational literal")
		}
		return &Rational{Value: v}, nil
	case fdfBytes:
		l, err := readBoundedLen32(buf, maxFDFPayloadBytes, "bytes")
		if err != nil {
			return nil, err
		}
		b := make([]byte, l)
		if _, err := io.ReadFull(buf, b); err != nil {
			return nil, err
		}
		return BytesFromSlice(b), nil
	case fdfBits:
		l, err := readBoundedLen32(buf, maxFDFPayloadBytes, "bits bytes")
		if err != nil {
			return nil, err
		}
		b := make([]byte, l)
		if _, err := io.ReadFull(buf, b); err != nil {
			return nil, err
		}
		bl, err := readBoundedLen32(buf, maxFDFPayloadBytes*8, "bits length")
		if err != nil {
			return nil, err
		}
		if bl > l*8 {
			return nil, fmt.Errorf("invalid FDF bits length %d for %d payload bytes", bl, l)
		}
		return &Bits{data: b, length: int(bl)}, nil
	case fdfList:
		elementType, err := readString(buf)
		if err != nil {
			return nil, err
		}
		l, err := readBoundedLen32(buf, maxFDFCollectionSize, "list size")
		if err != nil {
			return nil, err
		}
		elements := make([]Object, 0, l)
		for i := 0; i < int(l); i++ {
			el, err := decodeFDF(buf)
			if err != nil {
				return nil, err
			}
			elements = append(elements, el)
		}
		list := newList(elements)
		list.ElementType = elementType
		return list, nil
	case fdfTuple:
		l, err := readBoundedLen32(buf, maxFDFCollectionSize, "tuple size")
		if err != nil {
			return nil, err
		}
		elements := make([]Object, 0, l)
		for i := 0; i < int(l); i++ {
			el, err := decodeFDF(buf)
			if err != nil {
				return nil, err
			}
			elements = append(elements, el)
		}
		return &Tuple{Elements: elements}, nil
	case fdfMap:
		keyType, err := readString(buf)
		if err != nil {
			return nil, err
		}
		valType, err := readString(buf)
		if err != nil {
			return nil, err
		}
		l, err := readBoundedLen32(buf, maxFDFCollectionSize, "map size")
		if err != nil {
			return nil, err
		}
		m := NewMap()
		for i := 0; i < int(l); i++ {
			k, err := decodeFDF(buf)
			if err != nil {
				return nil, err
			}
			v, err := decodeFDF(buf)
			if err != nil {
				return nil, err
			}
			m = m.Put(k, v)
		}
		m.KeyType = keyType
		m.ValType = valType
		return m, nil
	case fdfRecordInstance:
		modName, err := readString(buf)
		if err != nil {
			return nil, err
		}
		typeName, err := readString(buf)
		if err != nil {
			return nil, err
		}
		isOpen, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}
		l, err := readBoundedLen32(buf, maxFDFCollectionSize, "record field count")
		if err != nil {
			return nil, err
		}
		fields := make([]RecordField, 0, l)
		for i := 0; i < int(l); i++ {
			key, err := readString(buf)
			if err != nil {
				return nil, err
			}
			val, err := decodeFDF(buf)
			if err != nil {
				return nil, err
			}
			fields = append(fields, RecordField{Key: key, Value: val})
		}
		return &RecordInstance{ModuleName: modName, TypeName: typeName, RowPolyExtended: isOpen == 1, Fields: fields}, nil
	case fdfDataInstance:
		name, err := readString(buf)
		if err != nil {
			return nil, err
		}
		l, err := readBoundedLen32(buf, maxFDFCollectionSize, "data field count")
		if err != nil {
			return nil, err
		}
		elements := make([]Object, 0, l)
		for i := 0; i < int(l); i++ {
			el, err := decodeFDF(buf)
			if err != nil {
				return nil, err
			}
			elements = append(elements, el)
		}
		typeName, err := readString(buf)
		if err != nil {
			return nil, err
		}
		typeArgsCount, err := readBoundedLen32(buf, maxFDFCollectionSize, "type args count")
		if err != nil {
			return nil, err
		}
		typeArgs := make([]typesystem.Type, 0, typeArgsCount)
		for i := 0; i < int(typeArgsCount); i++ {
			arg, err := decodeFDFType(buf)
			if err != nil {
				return nil, err
			}
			typeArgs = append(typeArgs, arg)
		}
		return &DataInstance{Name: name, Fields: elements, TypeName: typeName, TypeArgs: typeArgs}, nil
	case fdfConstructor:
		name, err := readString(buf)
		if err != nil {
			return nil, err
		}
		return &Constructor{Name: name}, nil
	case fdfRange:
		start, err := decodeFDF(buf)
		if err != nil {
			return nil, err
		}
		next, err := decodeFDF(buf)
		if err != nil {
			return nil, err
		}
		end, err := decodeFDF(buf)
		if err != nil {
			return nil, err
		}
		return &Range{Start: start, Next: next, End: end}, nil
	case fdfTypeObject:
		name, err := readString(buf)
		if err != nil {
			return nil, err
		}
		return &TypeObject{TypeVal: typesystem.TCon{Name: name}}, nil
	default:
		return nil, fmt.Errorf("FDF decoding error: unknown tag %d", tag)
	}
}

const (
	fdfTypeTVar byte = iota
	fdfTypeTCon
	fdfTypeTApp
	fdfTypeTFunc
	fdfTypeTRecord
	fdfTypeTTuple
	fdfTypeTUnion
	fdfTypeTType
	fdfTypeTForall
)

func encodeFDFKind(buf *bytes.Buffer, k typesystem.Kind) error {
	if k == nil {
		buf.WriteByte(0)
		return nil
	}
	switch v := k.(type) {
	case typesystem.KStar:
		buf.WriteByte(1)
	case typesystem.KWildcard:
		buf.WriteByte(2)
	case typesystem.KVar:
		buf.WriteByte(3)
		if err := writeString(buf, v.Name); err != nil {
			return err
		}
	case typesystem.KArrow:
		buf.WriteByte(4)
		if err := encodeFDFKind(buf, v.Left); err != nil {
			return err
		}
		if err := encodeFDFKind(buf, v.Right); err != nil {
			return err
		}
	default:
		return fmt.Errorf("FDF unsupported typesystem.Kind: %T", k)
	}
	return nil
}

func decodeFDFKind(buf *bytes.Reader) (typesystem.Kind, error) {
	tag, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	switch tag {
	case 0:
		return nil, nil
	case 1:
		return typesystem.KStar{}, nil
	case 2:
		return typesystem.KWildcard{}, nil
	case 3:
		name, err := readString(buf)
		if err != nil {
			return nil, err
		}
		return typesystem.KVar{Name: name}, nil
	case 4:
		left, err := decodeFDFKind(buf)
		if err != nil {
			return nil, err
		}
		right, err := decodeFDFKind(buf)
		if err != nil {
			return nil, err
		}
		return typesystem.KArrow{Left: left, Right: right}, nil
	default:
		return nil, fmt.Errorf("FDF decoding error: unknown typesystem.Kind tag %d", tag)
	}
}

func encodeFDFConstraint(buf *bytes.Buffer, c typesystem.Constraint) error {
	if err := writeString(buf, c.TypeVar); err != nil {
		return err
	}
	if err := writeString(buf, c.Trait); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, uint32(len(c.Args))); err != nil {
		return err
	}
	for _, arg := range c.Args {
		if err := encodeFDFType(buf, arg); err != nil {
			return err
		}
	}
	return nil
}

func decodeFDFConstraint(buf *bytes.Reader) (typesystem.Constraint, error) {
	typeVar, err := readString(buf)
	if err != nil {
		return typesystem.Constraint{}, err
	}
	trait, err := readString(buf)
	if err != nil {
		return typesystem.Constraint{}, err
	}
	l, err := readBoundedLen32(buf, maxFDFCollectionSize, "constraint args count")
	if err != nil {
		return typesystem.Constraint{}, err
	}
	args := make([]typesystem.Type, l)
	for i := 0; i < int(l); i++ {
		arg, err := decodeFDFType(buf)
		if err != nil {
			return typesystem.Constraint{}, err
		}
		args[i] = arg
	}
	return typesystem.Constraint{TypeVar: typeVar, Trait: trait, Args: args}, nil
}

func encodeFDFType(buf *bytes.Buffer, t typesystem.Type) error {
	if t == nil {
		buf.WriteByte(255) // nil type
		return nil
	}

	switch v := t.(type) {
	case typesystem.TVar:
		buf.WriteByte(fdfTypeTVar)
		if err := writeString(buf, v.Name); err != nil {
			return err
		}
		if err := encodeFDFKind(buf, v.KindVal); err != nil {
			return err
		}
	case typesystem.TCon:
		buf.WriteByte(fdfTypeTCon)
		if err := writeString(buf, v.Name); err != nil {
			return err
		}
		if err := encodeFDFKind(buf, v.KindVal); err != nil {
			return err
		}
	case typesystem.TApp:
		buf.WriteByte(fdfTypeTApp)
		if err := encodeFDFType(buf, v.Constructor); err != nil {
			return err
		}
		if err := binary.Write(buf, binary.BigEndian, uint32(len(v.Args))); err != nil {
			return err
		}
		for _, arg := range v.Args {
			if err := encodeFDFType(buf, arg); err != nil {
				return err
			}
		}
		if err := encodeFDFKind(buf, v.KindVal); err != nil {
			return err
		}
	case typesystem.TFunc:
		buf.WriteByte(fdfTypeTFunc)
		if err := binary.Write(buf, binary.BigEndian, uint32(len(v.Params))); err != nil {
			return err
		}
		for _, arg := range v.Params {
			if err := encodeFDFType(buf, arg); err != nil {
				return err
			}
		}
		if err := encodeFDFType(buf, v.ReturnType); err != nil {
			return err
		}
		isVar := byte(0)
		if v.IsVariadic {
			isVar = 1
		}
		buf.WriteByte(isVar)
		if err := binary.Write(buf, binary.BigEndian, uint32(v.DefaultCount)); err != nil {
			return err
		}
		if err := binary.Write(buf, binary.BigEndian, uint32(len(v.Constraints))); err != nil {
			return err
		}
		for _, c := range v.Constraints {
			if err := encodeFDFConstraint(buf, c); err != nil {
				return err
			}
		}
	case typesystem.TRecord:
		buf.WriteByte(fdfTypeTRecord)
		if err := binary.Write(buf, binary.BigEndian, uint32(len(v.Fields))); err != nil {
			return err
		}
		// Write fields in stable order
		var keys []string
		for name := range v.Fields {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		for _, name := range keys {
			if err := writeString(buf, name); err != nil {
				return err
			}
			if err := encodeFDFType(buf, v.Fields[name]); err != nil {
				return err
			}
		}
		isOpen := byte(0)
		if v.IsOpen {
			isOpen = 1
		}
		buf.WriteByte(isOpen)
		if err := encodeFDFType(buf, v.Row); err != nil {
			return err
		}
	case typesystem.TTuple:
		buf.WriteByte(fdfTypeTTuple)
		if err := binary.Write(buf, binary.BigEndian, uint32(len(v.Elements))); err != nil {
			return err
		}
		for _, el := range v.Elements {
			if err := encodeFDFType(buf, el); err != nil {
				return err
			}
		}
	case typesystem.TUnion:
		buf.WriteByte(fdfTypeTUnion)
		if err := binary.Write(buf, binary.BigEndian, uint32(len(v.Types))); err != nil {
			return err
		}
		for _, el := range v.Types {
			if err := encodeFDFType(buf, el); err != nil {
				return err
			}
		}
	case typesystem.TType:
		buf.WriteByte(fdfTypeTType)
		if err := encodeFDFType(buf, v.Type); err != nil {
			return err
		}
	case typesystem.TForall:
		buf.WriteByte(fdfTypeTForall)
		if err := binary.Write(buf, binary.BigEndian, uint32(len(v.Vars))); err != nil {
			return err
		}
		for _, vName := range v.Vars {
			if err := writeString(buf, vName.Name); err != nil {
				return err
			}
		}
		if err := binary.Write(buf, binary.BigEndian, uint32(len(v.Constraints))); err != nil {
			return err
		}
		for _, c := range v.Constraints {
			if err := encodeFDFConstraint(buf, c); err != nil {
				return err
			}
		}
		if err := encodeFDFType(buf, v.Type); err != nil {
			return err
		}
	default:
		return fmt.Errorf("FDF unsupported typesystem.Type: %T", v)
	}
	return nil
}

func decodeFDFType(buf *bytes.Reader) (typesystem.Type, error) {
	tag, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	if tag == 255 {
		return nil, nil // nil type
	}

	switch tag {
	case fdfTypeTVar:
		name, err := readString(buf)
		if err != nil {
			return nil, err
		}
		kind, err := decodeFDFKind(buf)
		if err != nil {
			return nil, err
		}
		return typesystem.TVar{Name: name, KindVal: kind}, nil
	case fdfTypeTCon:
		name, err := readString(buf)
		if err != nil {
			return nil, err
		}
		kind, err := decodeFDFKind(buf)
		if err != nil {
			return nil, err
		}
		return typesystem.TCon{Name: name, KindVal: kind}, nil
	case fdfTypeTApp:
		constructor, err := decodeFDFType(buf)
		if err != nil {
			return nil, err
		}
		l, err := readBoundedLen32(buf, maxFDFCollectionSize, "type app args count")
		if err != nil {
			return nil, err
		}
		args := make([]typesystem.Type, l)
		for i := 0; i < int(l); i++ {
			arg, err := decodeFDFType(buf)
			if err != nil {
				return nil, err
			}
			args[i] = arg
		}
		kind, err := decodeFDFKind(buf)
		if err != nil {
			return nil, err
		}
		return typesystem.TApp{Constructor: constructor, Args: args, KindVal: kind}, nil
	case fdfTypeTFunc:
		l, err := readBoundedLen32(buf, maxFDFCollectionSize, "func params count")
		if err != nil {
			return nil, err
		}
		params := make([]typesystem.Type, l)
		for i := 0; i < int(l); i++ {
			arg, err := decodeFDFType(buf)
			if err != nil {
				return nil, err
			}
			params[i] = arg
		}
		retType, err := decodeFDFType(buf)
		if err != nil {
			return nil, err
		}
		isVar, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}
		var defaultCount uint32
		if err := binary.Read(buf, binary.BigEndian, &defaultCount); err != nil {
			return nil, err
		}
		constraintCount, err := readBoundedLen32(buf, maxFDFCollectionSize, "func constraint count")
		if err != nil {
			return nil, err
		}
		constraints := make([]typesystem.Constraint, constraintCount)
		for i := 0; i < int(constraintCount); i++ {
			c, err := decodeFDFConstraint(buf)
			if err != nil {
				return nil, err
			}
			constraints[i] = c
		}
		return typesystem.TFunc{
			Params:       params,
			ReturnType:   retType,
			IsVariadic:   isVar == 1,
			DefaultCount: int(defaultCount),
			Constraints:  constraints,
		}, nil
	case fdfTypeTRecord:
		l, err := readBoundedLen32(buf, maxFDFCollectionSize, "record type field count")
		if err != nil {
			return nil, err
		}
		fields := make(map[string]typesystem.Type)
		for i := 0; i < int(l); i++ {
			name, err := readString(buf)
			if err != nil {
				return nil, err
			}
			t, err := decodeFDFType(buf)
			if err != nil {
				return nil, err
			}
			fields[name] = t
		}
		isOpen, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}
		row, err := decodeFDFType(buf)
		if err != nil {
			return nil, err
		}
		return typesystem.TRecord{
			Fields: fields,
			IsOpen: isOpen == 1,
			Row:    row,
		}, nil
	case fdfTypeTTuple:
		l, err := readBoundedLen32(buf, maxFDFCollectionSize, "tuple type size")
		if err != nil {
			return nil, err
		}
		elements := make([]typesystem.Type, l)
		for i := 0; i < int(l); i++ {
			arg, err := decodeFDFType(buf)
			if err != nil {
				return nil, err
			}
			elements[i] = arg
		}
		return typesystem.TTuple{Elements: elements}, nil
	case fdfTypeTUnion:
		l, err := readBoundedLen32(buf, maxFDFCollectionSize, "union type size")
		if err != nil {
			return nil, err
		}
		cases := make([]typesystem.Type, l)
		for i := 0; i < int(l); i++ {
			arg, err := decodeFDFType(buf)
			if err != nil {
				return nil, err
			}
			cases[i] = arg
		}
		return typesystem.TUnion{Types: cases}, nil
	case fdfTypeTType:
		inner, err := decodeFDFType(buf)
		if err != nil {
			return nil, err
		}
		return typesystem.TType{Type: inner}, nil
	case fdfTypeTForall:
		l, err := readBoundedLen32(buf, maxFDFCollectionSize, "forall vars count")
		if err != nil {
			return nil, err
		}
		vars := make([]typesystem.TVar, l)
		for i := 0; i < int(l); i++ {
			v, err := readString(buf)
			if err != nil {
				return nil, err
			}
			vars[i] = typesystem.TVar{Name: v}
		}
		constraintCount, err := readBoundedLen32(buf, maxFDFCollectionSize, "forall constraint count")
		if err != nil {
			return nil, err
		}
		constraints := make([]typesystem.Constraint, constraintCount)
		for i := 0; i < int(constraintCount); i++ {
			c, err := decodeFDFConstraint(buf)
			if err != nil {
				return nil, err
			}
			constraints[i] = c
		}
		inner, err := decodeFDFType(buf)
		if err != nil {
			return nil, err
		}
		return typesystem.TForall{
			Vars:        vars,
			Constraints: constraints,
			Type:        inner,
		}, nil
	default:
		return nil, fmt.Errorf("FDF decoding error: unknown typesystem tag %d", tag)
	}
}

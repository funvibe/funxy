package evaluator

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"math/big"
	"github.com/funvibe/funxy/internal/typesystem"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestFDF_KindsSerialization(t *testing.T) {
	kinds := []typesystem.Kind{
		typesystem.KStar{},
		typesystem.KWildcard{},
		typesystem.KVar{Name: "k1"},
		typesystem.KArrow{Left: typesystem.KStar{}, Right: typesystem.KStar{}},
		typesystem.KArrow{
			Left:  typesystem.KVar{Name: "a"},
			Right: typesystem.KArrow{Left: typesystem.KStar{}, Right: typesystem.KWildcard{}},
		},
		nil, // Ensure nil kind is handled gracefully
	}

	for i, k := range kinds {
		var buf bytes.Buffer
		err := encodeFDFKind(&buf, k)
		if err != nil {
			t.Fatalf("encodeFDFKind failed on case %d (%T): %v", i, k, err)
		}

		reader := bytes.NewReader(buf.Bytes())
		decoded, err := decodeFDFKind(reader)
		if err != nil {
			t.Fatalf("decodeFDFKind failed on case %d (%T): %v", i, k, err)
		}

		if !reflect.DeepEqual(k, decoded) {
			t.Errorf("Mismatch on case %d.\nGot:  %#v\nWant: %#v", i, decoded, k)
		}
	}
}

func TestFDF_ConstraintsSerialization(t *testing.T) {
	constraints := []typesystem.Constraint{
		{
			TypeVar: "a",
			Trait:   "Eq",
			Args:    nil,
		},
		{
			TypeVar: "b",
			Trait:   "Show",
			Args: []typesystem.Type{
				typesystem.TVar{Name: "a", KindVal: typesystem.KStar{}},
			},
		},
	}

	for i, c := range constraints {
		var buf bytes.Buffer
		err := encodeFDFConstraint(&buf, c)
		if err != nil {
			t.Fatalf("encodeFDFConstraint failed on case %d: %v", i, err)
		}

		reader := bytes.NewReader(buf.Bytes())
		decoded, err := decodeFDFConstraint(reader)
		if err != nil {
			t.Fatalf("decodeFDFConstraint failed on case %d: %v", i, err)
		}

		// Types inside constraint might have empty slice instead of nil slice from decode
		if len(c.Args) == 0 && len(decoded.Args) == 0 {
			c.Args = []typesystem.Type{}
			decoded.Args = []typesystem.Type{}
		}

		if !reflect.DeepEqual(c, decoded) {
			t.Errorf("Mismatch on case %d.\nGot:  %#v\nWant: %#v", i, decoded, c)
		}
	}
}

func TestFDF_TypesSerialization(t *testing.T) {
	types := []typesystem.Type{
		nil,
		typesystem.TVar{Name: "a", KindVal: typesystem.KStar{}},
		typesystem.TCon{Name: "Int", KindVal: typesystem.KStar{}},
		typesystem.TApp{
			Constructor: typesystem.TCon{Name: "List", KindVal: typesystem.KArrow{Left: typesystem.KStar{}, Right: typesystem.KStar{}}},
			Args: []typesystem.Type{
				typesystem.TCon{Name: "String", KindVal: typesystem.KStar{}},
			},
			KindVal: typesystem.KStar{},
		},
		typesystem.TFunc{
			Params: []typesystem.Type{
				typesystem.TCon{Name: "Int", KindVal: typesystem.KStar{}},
				typesystem.TCon{Name: "String", KindVal: typesystem.KStar{}},
			},
			ReturnType:   typesystem.TCon{Name: "Bool", KindVal: typesystem.KStar{}},
			IsVariadic:   true,
			DefaultCount: 1,
			Constraints: []typesystem.Constraint{
				{TypeVar: "a", Trait: "Eq", Args: []typesystem.Type{}},
			},
		},
		typesystem.TRecord{
			Fields: map[string]typesystem.Type{
				"name": typesystem.TCon{Name: "String", KindVal: typesystem.KStar{}},
				"age":  typesystem.TCon{Name: "Int", KindVal: typesystem.KStar{}},
			},
			IsOpen: true,
			Row:    typesystem.TVar{Name: "r", KindVal: typesystem.KStar{}},
		},
		typesystem.TTuple{
			Elements: []typesystem.Type{
				typesystem.TCon{Name: "Int", KindVal: typesystem.KStar{}},
				typesystem.TCon{Name: "Float", KindVal: typesystem.KStar{}},
			},
		},
		typesystem.TUnion{
			Types: []typesystem.Type{
				typesystem.TCon{Name: "String", KindVal: typesystem.KStar{}},
				typesystem.TCon{Name: "Int", KindVal: typesystem.KStar{}},
			},
		},
		typesystem.TType{
			Type: typesystem.TCon{Name: "Module", KindVal: typesystem.KStar{}},
		},
		typesystem.TForall{
			Vars: []typesystem.TVar{
				{Name: "a", KindVal: nil}, // FDF decode might yield nil kind for default
				{Name: "b", KindVal: nil},
			},
			Constraints: []typesystem.Constraint{
				{TypeVar: "a", Trait: "Show", Args: []typesystem.Type{}},
			},
			Type: typesystem.TVar{Name: "a", KindVal: typesystem.KStar{}},
		},
	}

	for i, typ := range types {
		var buf bytes.Buffer
		err := encodeFDFType(&buf, typ)
		if err != nil {
			t.Fatalf("encodeFDFType failed on case %d (%T): %v", i, typ, err)
		}

		reader := bytes.NewReader(buf.Bytes())
		decoded, err := decodeFDFType(reader)
		if err != nil {
			t.Fatalf("decodeFDFType failed on case %d (%T): %v", i, typ, err)
		}

		// Normalizing nil vs empty slice for reflect.DeepEqual
		normalizeType(&typ)
		normalizeType(&decoded)

		if !reflect.DeepEqual(typ, decoded) {
			t.Errorf("Mismatch on case %d (%T).\nGot:  %#v\nWant: %#v", i, typ, decoded, typ)
		}
	}
}

// normalizeType is a helper to turn nil slices into empty slices to pass reflect.DeepEqual,
// since serialization/deserialization might replace a nil slice with an empty slice.
func normalizeType(t *typesystem.Type) {
	if t == nil || *t == nil {
		return
	}
	switch v := (*t).(type) {
	case typesystem.TApp:
		if v.Args == nil {
			v.Args = []typesystem.Type{}
		}
		*t = v
	case typesystem.TFunc:
		if v.Params == nil {
			v.Params = []typesystem.Type{}
		}
		if v.Constraints == nil {
			v.Constraints = []typesystem.Constraint{}
		}
		for i := range v.Constraints {
			if v.Constraints[i].Args == nil {
				v.Constraints[i].Args = []typesystem.Type{}
			}
		}
		*t = v
	case typesystem.TRecord:
		if v.Fields == nil {
			v.Fields = map[string]typesystem.Type{}
		}
		*t = v
	case typesystem.TTuple:
		if v.Elements == nil {
			v.Elements = []typesystem.Type{}
		}
		*t = v
	case typesystem.TUnion:
		if v.Types == nil {
			v.Types = []typesystem.Type{}
		}
		*t = v
	case typesystem.TForall:
		if v.Vars == nil {
			v.Vars = []typesystem.TVar{}
		}
		if v.Constraints == nil {
			v.Constraints = []typesystem.Constraint{}
		}
		for i := range v.Constraints {
			if v.Constraints[i].Args == nil {
				v.Constraints[i].Args = []typesystem.Type{}
			}
		}
		*t = v
	}
}

func TestFDF_AllValueTypes(t *testing.T) {
	// Comprehensive FDF roundtrip tests for all supported value types
	tests := []struct {
		name string
		obj  Object
	}{
		{"Int", &Integer{Value: 42}},
		{"Int_negative", &Integer{Value: -999}},
		{"Float", &Float{Value: 3.14}},
		{"Bool_true", &Boolean{Value: true}},
		{"Bool_false", &Boolean{Value: false}},
		{"Char", &Char{Value: 'A'}},
		{"Nil", &Nil{}},
		{"BigInt", &BigInt{Value: big.NewInt(1234567890123456789)}},
		{"Rational", &Rational{Value: big.NewRat(22, 7)}},
		{"Bytes", BytesFromSlice([]byte{1, 2, 3, 0xff})},
		{"Bits", bitsFromBytes(BytesFromSlice([]byte{0xab}))},
		{"String", func() *List {
			l := goStringToList("hello")
			l.ElementType = "Char"
			return l
		}()},
		{"List_Int", func() *List {
			l := newList([]Object{&Integer{Value: 1}, &Integer{Value: 2}})
			l.ElementType = "Int"
			return l
		}()},
		{"Tuple", &Tuple{Elements: []Object{&Integer{Value: 1}, &Boolean{Value: true}}}},
		{"Map", func() *Map {
			m := NewMap()
			m = m.Put(goStringToList("a"), &Integer{Value: 1})
			m = m.Put(goStringToList("b"), &Integer{Value: 2})
			m.KeyType = "String"
			m.ValType = "Int"
			return m
		}()},
		{"Option_None", MakeNone()},
		{"Option_Some", MakeSome(&Integer{Value: 42})},
		{"Result_Ok", makeOk(&Integer{Value: 100})},
		{"Result_Fail", makeFailStr("error")},
		{"Range_simple", &Range{Start: &Integer{Value: 1}, Next: &Nil{}, End: &Integer{Value: 10}}},
		{"Range_step", &Range{Start: &Integer{Value: 0}, Next: &Integer{Value: 2}, End: &Integer{Value: 10}}},
		{"Range_char", &Range{Start: &Char{Value: 'a'}, Next: &Nil{}, End: &Char{Value: 'z'}}},
		{"Identity", &DataInstance{Name: "identity", TypeName: "Identity", Fields: []Object{&Integer{Value: 42}}}},
		{"Reader", &DataInstance{Name: "reader", TypeName: "Reader", Fields: []Object{&Integer{Value: 1}}}},
		{"Json_JNull", &DataInstance{Name: "JNull", TypeName: "Json", Fields: []Object{}}},
		{"Json_JBool", &DataInstance{Name: "JBool", TypeName: "Json", Fields: []Object{&Boolean{Value: true}}}},
		{"Json_JNum", &DataInstance{Name: "JNum", TypeName: "Json", Fields: []Object{&Float{Value: 3.14}}}},
		{"Json_JStr", &DataInstance{Name: "JStr", TypeName: "Json", Fields: []Object{goStringToList("hi")}}},
		{"RecordInstance", &RecordInstance{
			ModuleName: "lib/date",
			TypeName:   "Date",
			Fields: []RecordField{
				{Key: "year", Value: &Integer{Value: 2025}},
				{Key: "month", Value: &Integer{Value: 2}},
				{Key: "day", Value: &Integer{Value: 23}},
			},
		}},
		{"OptionT", &DataInstance{Name: "OptionT", TypeName: "OptionT", Fields: []Object{MakeSome(&Integer{Value: 1})}}},
		{"ResultT", &DataInstance{Name: "ResultT", TypeName: "ResultT", Fields: []Object{makeOk(&Integer{Value: 2})}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := SerializeValue(tt.obj, "fdf")
			if err != nil {
				t.Fatalf("serialize failed: %v", err)
			}
			decoded, err := DeserializeValue(data)
			if err != nil {
				t.Fatalf("deserialize failed: %v", err)
			}
			// Compare Inspect() for value equality (handles structural types)
			if decoded.Inspect() != tt.obj.Inspect() {
				t.Errorf("roundtrip mismatch:\n  orig: %s\n  got:  %s", tt.obj.Inspect(), decoded.Inspect())
			}
		})
	}
}

func TestFDF_ComplexObjectsSerialization(t *testing.T) {
	// Test full object serialization involving FDF type details that we added recently
	listObj := newList([]Object{&Integer{Value: 1}})
	listObj.ElementType = "Int"

	mapObj := NewMap()
	mapObj = mapObj.Put(goStringToList("key"), &Integer{Value: 10})
	mapObj.KeyType = "String"
	mapObj.ValType = "Int"

	recordObj := &RecordInstance{
		ModuleName:      "TestMod",
		TypeName:        "TestType",
		RowPolyExtended: true,
		Fields: []RecordField{
			{Key: "foo", Value: &Integer{Value: 1}},
		},
	}

	dataObj := &DataInstance{
		Name:     "SomeData",
		TypeName: "SomeType",
		Fields:   []Object{&Integer{Value: 42}},
		TypeArgs: []typesystem.Type{
			typesystem.TCon{Name: "Int", KindVal: typesystem.KStar{}},
		},
	}

	objects := []Object{
		listObj,
		mapObj,
		recordObj,
		dataObj,
	}

	for i, obj := range objects {
		data, err := SerializeValue(obj, "fdf")
		if err != nil {
			t.Fatalf("serialize fdf failed on case %d (%T): %v", i, obj, err)
		}

		decoded, err := DeserializeValue(data)
		if err != nil {
			t.Fatalf("DeserializeValue failed on case %d (%T): %v", i, obj, err)
		}

		// Perform specific checks based on the object
		switch v := decoded.(type) {
		case *List:
			orig := obj.(*List)
			if v.ElementType != orig.ElementType {
				t.Errorf("List ElementType mismatch: got %v, want %v", v.ElementType, orig.ElementType)
			}
		case *Map:
			orig := obj.(*Map)
			if v.KeyType != orig.KeyType {
				t.Errorf("Map KeyType mismatch: got %v, want %v", v.KeyType, orig.KeyType)
			}
			if v.ValType != orig.ValType {
				t.Errorf("Map ValType mismatch: got %v, want %v", v.ValType, orig.ValType)
			}
		case *RecordInstance:
			orig := obj.(*RecordInstance)
			if v.TypeName != orig.TypeName {
				t.Errorf("RecordInstance TypeName mismatch: got %v, want %v", v.TypeName, orig.TypeName)
			}
			if v.RowPolyExtended != orig.RowPolyExtended {
				t.Errorf("RecordInstance RowPolyExtended mismatch: got %v, want %v", v.RowPolyExtended, orig.RowPolyExtended)
			}
		case *DataInstance:
			orig := obj.(*DataInstance)
			if len(v.TypeArgs) != len(orig.TypeArgs) {
				t.Fatalf("DataInstance TypeArgs length mismatch: got %v, want %v", len(v.TypeArgs), len(orig.TypeArgs))
			}
			for idx, arg := range v.TypeArgs {
				if !reflect.DeepEqual(arg, orig.TypeArgs[idx]) {
					t.Errorf("DataInstance TypeArgs[%d] mismatch: got %#v, want %#v", idx, arg, orig.TypeArgs[idx])
				}
			}
		}
	}
}

func TestFDF_BitsLengthRoundtripOver255(t *testing.T) {
	bits, err := bitsFromBinary(strings.Repeat("1", 300))
	if err != nil {
		t.Fatalf("bitsFromBinary failed: %v", err)
	}
	data, err := SerializeValue(bits, "fdf")
	if err != nil {
		t.Fatalf("SerializeValue failed: %v", err)
	}
	decoded, err := DeserializeValue(data)
	if err != nil {
		t.Fatalf("DeserializeValue failed: %v", err)
	}
	out, ok := decoded.(*Bits)
	if !ok {
		t.Fatalf("expected *Bits, got %T", decoded)
	}
	if out.Len() != 300 {
		t.Fatalf("bits length mismatch: got %d, want 300", out.Len())
	}
}

func TestFDF_InvalidNumericLiteralsRejected(t *testing.T) {
	makePayload := func(tag byte, s string) []byte {
		var buf bytes.Buffer
		buf.WriteString(FDF_MAGIC)
		buf.WriteByte(tag)
		_ = binary.Write(&buf, binary.BigEndian, uint32(len(s)))
		buf.WriteString(s)
		return buf.Bytes()
	}

	if _, err := DeserializeValue(makePayload(fdfBigInt, "not-a-bigint")); err == nil {
		t.Fatalf("expected invalid big int error, got nil")
	}
	if _, err := DeserializeValue(makePayload(fdfRational, "not-a-rational")); err == nil {
		t.Fatalf("expected invalid rational error, got nil")
	}
}

func TestFDF_InvalidBitsLengthRejected(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString(FDF_MAGIC)
	buf.WriteByte(fdfBits)
	_ = binary.Write(&buf, binary.BigEndian, uint32(1)) // one byte payload
	buf.WriteByte(0xff)
	_ = binary.Write(&buf, binary.BigEndian, uint32(9)) // invalid: > 1*8

	if _, err := DeserializeValue(buf.Bytes()); err == nil {
		t.Fatalf("expected invalid bits length error, got nil")
	}
}

func benchmarkPayload() Object {
	rec := NewRecord(map[string]Object{
		"service": goStringToList("billing"),
		"ok":      &Boolean{Value: true},
		"count":   &Integer{Value: 12345},
		"ratio":   &Rational{Value: big.NewRat(22, 7)},
	})
	list := newList([]Object{
		rec,
		goStringToList("hello"),
		&Integer{Value: 42},
		bitsFromBytes(BytesFromSlice([]byte{0xde, 0xad, 0xbe, 0xef})),
	})
	list.ElementType = "Any"
	return list
}

var gobBenchRegisterOnce sync.Once

func ensureGobBenchTypesRegistered() {
	gobBenchRegisterOnce.Do(func() {
		gob.Register(&Nil{})
		gob.Register(&Integer{})
		gob.Register(&Float{})
		gob.Register(&Boolean{})
		gob.Register(&Char{})
		gob.Register(&BigInt{})
		gob.Register(&Rational{})
		gob.Register(&Bytes{})
		gob.Register(&Bits{})
		gob.Register(&List{})
		gob.Register(&Tuple{})
		gob.Register(&Map{})
		gob.Register(&RecordInstance{})
		gob.Register(&DataInstance{})
		gob.Register(&TypeObject{})
		gob.Register(&Constructor{})
		gob.Register(&Range{})
	})
}

func BenchmarkSerializeValue_FDF(b *testing.B) {
	obj := benchmarkPayload()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := SerializeValue(obj, "fdf"); err != nil {
			b.Fatalf("SerializeValue(fdf) failed: %v", err)
		}
	}
}

func BenchmarkSerializeValue_Ephemeral(b *testing.B) {
	obj := benchmarkPayload()
	ensureGobBenchTypesRegistered()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := SerializeValue(obj, SerializeModeEphemeral); err != nil {
			b.Fatalf("SerializeValue(ephemeral) failed: %v", err)
		}
	}
}

func BenchmarkDeserializeValue_FDF(b *testing.B) {
	obj := benchmarkPayload()
	data, err := SerializeValue(obj, "fdf")
	if err != nil {
		b.Fatalf("seed SerializeValue(fdf) failed: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := DeserializeValue(data); err != nil {
			b.Fatalf("DeserializeValue(fdf) failed: %v", err)
		}
	}
}

func BenchmarkDeserializeValue_Ephemeral(b *testing.B) {
	obj := benchmarkPayload()
	ensureGobBenchTypesRegistered()
	data, err := SerializeValue(obj, "ephemeral")
	if err != nil {
		b.Fatalf("seed SerializeValue(ephemeral) failed: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := DeserializeValue(data); err != nil {
			b.Fatalf("DeserializeValue(ephemeral) failed: %v", err)
		}
	}
}

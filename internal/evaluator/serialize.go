package evaluator

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

// CheckSerializable recursively checks if a Funxy object can be safely serialized.
// It rejects HostObjects, closures, and other non-data types.
func CheckSerializable(obj Object) error {
	if obj == nil {
		return nil
	}
	switch v := obj.(type) {
	case *Integer, *Float, *Boolean, *Char, *Nil, *BigInt, *Rational, *Bytes, *Bits:
		return nil
	case *List:
		if v == nil {
			return nil
		}
		for _, el := range v.ToSlice() {
			if err := CheckSerializable(el); err != nil {
				return err
			}
		}
		return nil
	case *Tuple:
		if v == nil {
			return nil
		}
		for _, el := range v.Elements {
			if err := CheckSerializable(el); err != nil {
				return err
			}
		}
		return nil
	case *Map:
		if v == nil {
			return nil
		}
		var iterErr error
		for _, item := range v.Items() {
			if e := CheckSerializable(item.Key); e != nil {
				iterErr = e
				break
			}
			if e := CheckSerializable(item.Value); e != nil {
				iterErr = e
				break
			}
		}
		return iterErr
	case *RecordInstance:
		if v == nil {
			return nil
		}
		for _, f := range v.Fields {
			if err := CheckSerializable(f.Value); err != nil {
				return err
			}
		}
		return nil
	case *DataInstance:
		if v == nil {
			return nil
		}
		for _, f := range v.Fields {
			if err := CheckSerializable(f); err != nil {
				return err
			}
		}
		return nil
	case *Constructor, *TypeObject:
		return nil // Types are safe to serialize (e.g. if they are part of a generic ADT)
	case *HostObject:
		return fmt.Errorf("cannot serialize HostObject")
	case *Function, *Builtin, *ClassMethod, *OperatorFunction, *ComposedFunction, *PartialApplication:
		return fmt.Errorf("cannot serialize function or closure")
	default:
		// Catch any other non-serializable objects from the vm package (e.g., ObjClosure)
		if v.Type() == FUNCTION_OBJ || v.Type() == BUILTIN_OBJ || v.Type() == HOST_OBJ {
			return fmt.Errorf("cannot serialize function, closure or host object (type: %s)", v.Type())
		}
		return nil
	}
}

// SerializeValue serializes a Funxy Object to a byte array.
func SerializeValue(val Object, mode string) ([]byte, error) {
	if err := CheckSerializable(val); err != nil {
		return nil, err
	}

	if mode == "fdf" {
		return SerializeFDF(val)
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	// We encode a pointer to the interface so gob knows it's an interface value
	// IMPORTANT: We must encode &val (pointer to interface) so gob preserves the concrete type info.
	// If we encode val directly, gob only sees the concrete value and might lose type info if it's nil.
	if err := enc.Encode(&val); err != nil {
		return nil, fmt.Errorf("failed to serialize value: %w", err)
	}
	return buf.Bytes(), nil
}

// DeserializeValue deserializes a Funxy Object from a byte array.
func DeserializeValue(data []byte) (Object, error) {
	if len(data) >= 4 && string(data[:4]) == "FDF1" {
		return DeserializeFDF(data)
	}

	// We need to decode into a pointer to an interface to preserve type information
	// properly when using gob with interfaces.
	var val Object
	buf := bytes.NewReader(data)
	dec := gob.NewDecoder(buf)

	// Decode into the address of the interface variable
	if err := dec.Decode(&val); err != nil {
		return nil, fmt.Errorf("failed to deserialize value: %w", err)
	}

	// If val is nil after decode (e.g. decoding nil interface), return Nil object
	if val == nil {
		return &Nil{}, nil
	}
	return val, nil
}

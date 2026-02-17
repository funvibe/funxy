package funxy

import (
	"fmt"
	"github.com/funvibe/funxy/internal/evaluator"
	"reflect"
)

// Marshaller handles conversion between Go and Funxy values.
type Marshaller struct{}

func NewMarshaller() *Marshaller {
	return &Marshaller{}
}

// ToValue converts a Go value to a Funxy Object.
func (m *Marshaller) ToValue(val interface{}) (evaluator.Object, error) {
	if val == nil {
		return &evaluator.Nil{}, nil
	}

	// Unpack interface if it's contained in one
	v := reflect.ValueOf(val)
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	if !v.IsValid() {
		return &evaluator.Nil{}, nil
	}

	// Check if already an Object
	if obj, ok := val.(evaluator.Object); ok {
		return obj, nil
	}

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &evaluator.Integer{Value: v.Int()}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &evaluator.Integer{Value: int64(v.Uint())}, nil
	case reflect.Float32, reflect.Float64:
		return &evaluator.Float{Value: v.Float()}, nil
	case reflect.Bool:
		return &evaluator.Boolean{Value: v.Bool()}, nil
	case reflect.String:
		return stringToList(v.String()), nil
	case reflect.Slice:
		return m.sliceToList(v)
	case reflect.Map:
		return m.mapToFunxyMap(v)
	case reflect.Struct:
		// Struct by value -> Record (copy)
		return m.structToRecord(v)
	case reflect.Ptr:
		// Pointer -> HostObject (reference)
		return &evaluator.HostObject{Value: val}, nil
	case reflect.Func:
		// Go function -> Builtin?
		// We can't easily convert arbitrary Go func to evaluator.Builtin because Builtin expects (env, args).
		// We will wrap it in a closure in VM.Bind, but here treating as HostObject might be safer unless we explicitly handle it.
		// For now, HostObject.
		return &evaluator.HostObject{Value: val}, nil
	default:
		return &evaluator.HostObject{Value: val}, nil
	}
}

// FromValue converts a Funxy Object to a Go value.
// targetType is optional; if provided, tries to convert to that type.
func (m *Marshaller) FromValue(obj evaluator.Object, targetType reflect.Type) (interface{}, error) {
	if obj == nil {
		return nil, nil
	}

	// If target type is evaluator.Object, return as is
	if targetType != nil && targetType == reflect.TypeOf((*evaluator.Object)(nil)).Elem() {
		return obj, nil
	}

	switch o := obj.(type) {
	case *evaluator.Integer:
		if targetType != nil {
			switch targetType.Kind() {
			case reflect.Int:
				return int(o.Value), nil
			case reflect.Int64:
				return o.Value, nil
			case reflect.Float64:
				return float64(o.Value), nil
			}
		}
		return int(o.Value), nil // Default to int
	case *evaluator.Float:
		return o.Value, nil
	case *evaluator.Boolean:
		return o.Value, nil
	case *evaluator.List:
		// Check if string
		if evaluator.IsStringList(o) {
			return evaluator.ListToString(o), nil
		}
		// Convert to slice
		return m.listToSlice(o, targetType)
	case *evaluator.RecordInstance:
		// Convert to map or struct
		if targetType != nil && targetType.Kind() == reflect.Struct {
			// TODO: Populate struct
			return nil, fmt.Errorf("conversion to struct not implemented yet")
		}
		// Default to map[string]interface{}
		return m.recordToMap(o)
	case *evaluator.Map:
		return m.funxyMapToGoMap(o, targetType)
	case *evaluator.HostObject:
		return o.Value, nil
	case *evaluator.Nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported type for conversion: %s", o.Type())
	}
}

func stringToList(s string) *evaluator.List {
	runes := []rune(s)
	chars := make([]evaluator.Object, len(runes))
	for i, r := range runes {
		chars[i] = &evaluator.Char{Value: int64(r)}
	}
	return evaluator.NewList(chars)
}

func (m *Marshaller) sliceToList(v reflect.Value) (*evaluator.List, error) {
	elements := make([]evaluator.Object, v.Len())
	for i := 0; i < v.Len(); i++ {
		val, err := m.ToValue(v.Index(i).Interface())
		if err != nil {
			return nil, err
		}
		elements[i] = val
	}
	return evaluator.NewList(elements), nil
}

func (m *Marshaller) mapToFunxyMap(v reflect.Value) (*evaluator.Map, error) {
	result := evaluator.NewMap()
	iter := v.MapRange()
	for iter.Next() {
		key, err := m.ToValue(iter.Key().Interface())
		if err != nil {
			return nil, fmt.Errorf("map key: %w", err)
		}
		val, err := m.ToValue(iter.Value().Interface())
		if err != nil {
			return nil, fmt.Errorf("map value: %w", err)
		}
		result = result.Put(key, val)
	}
	return result, nil
}

func (m *Marshaller) structToRecord(v reflect.Value) (*evaluator.RecordInstance, error) {
	fields := make(map[string]evaluator.Object)
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" { // Skip unexported fields
			continue
		}
		val, err := m.ToValue(v.Field(i).Interface())
		if err != nil {
			return nil, err
		}
		fields[field.Name] = val
	}
	return evaluator.NewRecord(fields), nil
}

func (m *Marshaller) listToSlice(l *evaluator.List, targetType reflect.Type) (interface{}, error) {
	// If targetType is nil, default to []interface{}
	elemType := reflect.TypeOf((*interface{})(nil)).Elem()
	if targetType != nil && targetType.Kind() == reflect.Slice {
		elemType = targetType.Elem()
	}

	slice := reflect.MakeSlice(reflect.SliceOf(elemType), 0, l.Len())

	els := l.ToSlice()
	for _, el := range els {
		val, err := m.FromValue(el, elemType)
		if err != nil {
			return nil, err
		}

		rv := reflect.ValueOf(val)
		if val == nil {
			// Handle nil for pointers/interfaces
			slice = reflect.Append(slice, reflect.Zero(elemType))
		} else {
			if rv.Type().AssignableTo(elemType) {
				slice = reflect.Append(slice, rv)
			} else if rv.Type().ConvertibleTo(elemType) {
				slice = reflect.Append(slice, rv.Convert(elemType))
			} else {
				return nil, fmt.Errorf("cannot convert %s to %s", rv.Type(), elemType)
			}
		}
	}
	return slice.Interface(), nil
}

func (m *Marshaller) funxyMapToGoMap(fm *evaluator.Map, targetType reflect.Type) (interface{}, error) {
	items := fm.Items()

	// If target type is a concrete map type, convert to that
	if targetType != nil && targetType.Kind() == reflect.Map {
		result := reflect.MakeMapWithSize(targetType, len(items))
		keyType := targetType.Key()
		valType := targetType.Elem()
		for _, item := range items {
			key, err := m.FromValue(item.Key, keyType)
			if err != nil {
				return nil, fmt.Errorf("map key: %w", err)
			}
			val, err := m.FromValue(item.Value, valType)
			if err != nil {
				return nil, fmt.Errorf("map value: %w", err)
			}
			kv := reflect.ValueOf(key)
			vv := reflect.ValueOf(val)
			if key == nil {
				kv = reflect.Zero(keyType)
			}
			if val == nil {
				vv = reflect.Zero(valType)
			}
			if kv.Type().ConvertibleTo(keyType) {
				kv = kv.Convert(keyType)
			}
			if vv.Type().ConvertibleTo(valType) {
				vv = vv.Convert(valType)
			}
			result.SetMapIndex(kv, vv)
		}
		return result.Interface(), nil
	}

	// Default: map[interface{}]interface{}
	result := make(map[interface{}]interface{}, len(items))
	for _, item := range items {
		key, err := m.FromValue(item.Key, nil)
		if err != nil {
			return nil, fmt.Errorf("map key: %w", err)
		}
		val, err := m.FromValue(item.Value, nil)
		if err != nil {
			return nil, fmt.Errorf("map value: %w", err)
		}
		result[key] = val
	}
	return result, nil
}

func (m *Marshaller) recordToMap(r *evaluator.RecordInstance) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, f := range r.Fields {
		val, err := m.FromValue(f.Value, nil)
		if err != nil {
			return nil, err
		}
		result[f.Key] = val
	}
	return result, nil
}

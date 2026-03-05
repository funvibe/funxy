package funxy

import (
	"fmt"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/vm"
	"reflect"
	"strings"
	"sync"
)

// Marshaller handles conversion between Go and Funxy values.
type Marshaller struct {
	machine *vm.VM
}

const maxMarshallingDepth = 128

type fieldInfo struct {
	Index int
	Name  string
	Omit  bool
}

// Global cache for reflection info of struct fields.
var typeCache sync.Map // map[reflect.Type][]fieldInfo

func getStructFields(t reflect.Type) []fieldInfo {
	if cached, ok := typeCache.Load(t); ok {
		return cached.([]fieldInfo)
	}

	var fields []fieldInfo
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" { // Skip unexported fields
			continue
		}

		fi := fieldInfo{
			Index: i,
			Name:  sf.Name,
		}

		// Parse tags: funxy first, fallback to json
		tag := sf.Tag.Get("funxy")
		if tag == "" {
			tag = sf.Tag.Get("json")
		}

		if tag == "-" {
			fi.Omit = true
		} else if tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] != "" {
				fi.Name = parts[0]
			}
		}

		fields = append(fields, fi)
	}

	typeCache.Store(t, fields)
	return fields
}

func NewMarshaller(machine *vm.VM) *Marshaller {
	return &Marshaller{machine: machine}
}

// ToValue converts a Go value to a Funxy Object.
func (m *Marshaller) ToValue(val interface{}) (evaluator.Object, error) {
	return m.toValue(val, 0)
}

func (m *Marshaller) toValue(val interface{}, depth int) (evaluator.Object, error) {
	if depth > maxMarshallingDepth {
		return nil, fmt.Errorf("marshaller max depth exceeded (cyclic reference?)")
	}

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
		return m.sliceToList(v, depth+1)
	case reflect.Map:
		return m.mapToFunxyMap(v, depth+1)
	case reflect.Struct:
		// Struct by value -> Record (copy)
		return m.structToRecord(v, depth+1)
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
	return m.fromValue(obj, targetType, 0)
}

func (m *Marshaller) fromValue(obj evaluator.Object, targetType reflect.Type, depth int) (interface{}, error) {
	if depth > maxMarshallingDepth {
		return nil, fmt.Errorf("marshaller max depth exceeded (cyclic reference?)")
	}

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
			case reflect.Ptr:
				elemType := targetType.Elem()
				switch elemType.Kind() {
				case reflect.Int:
					v := int(o.Value)
					return &v, nil
				case reflect.Int64:
					v := o.Value
					return &v, nil
				case reflect.Float64:
					v := float64(o.Value)
					return &v, nil
				}
			}
		}
		return int(o.Value), nil // Default to int
	case *evaluator.Float:
		if targetType != nil && targetType.Kind() == reflect.Ptr {
			elemType := targetType.Elem()
			switch elemType.Kind() {
			case reflect.Float32:
				v := float32(o.Value)
				return &v, nil
			case reflect.Float64:
				v := o.Value
				return &v, nil
			}
		}
		return o.Value, nil
	case *evaluator.Boolean:
		if targetType != nil && targetType.Kind() == reflect.Ptr {
			elemType := targetType.Elem()
			if elemType.Kind() == reflect.Bool {
				v := o.Value
				return &v, nil
			}
		}
		return o.Value, nil
	case *evaluator.List:
		// Check if string
		if evaluator.IsStringList(o) {
			return evaluator.ListToString(o), nil
		}
		// Convert to slice
		return m.listToSlice(o, targetType, depth+1)
	case *evaluator.RecordInstance:
		// Convert to map or struct
		if targetType != nil && (targetType.Kind() == reflect.Struct || (targetType.Kind() == reflect.Ptr && targetType.Elem().Kind() == reflect.Struct)) {
			return m.recordToStruct(o, targetType, depth+1)
		}
		// Default to map[string]interface{}
		return m.recordToMap(o, depth+1)
	case *evaluator.Map:
		return m.funxyMapToGoMap(o, targetType, depth+1)
	case *evaluator.HostObject:
		return o.Value, nil
	case *evaluator.Function, *vm.ObjClosure, *evaluator.Builtin, *evaluator.PartialApplication, *vm.VMComposedFunction, *evaluator.ComposedFunction:
		// Convert Funxy function back to Go function
		if targetType != nil && targetType.Kind() == reflect.Func {
			return m.createGoFunc(o, targetType)
		}
		return nil, fmt.Errorf("cannot convert function %s without reflect.Func targetType", o.Type())
	case *evaluator.Nil:
		if targetType != nil {
			switch targetType.Kind() {
			case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Func:
				// Return a nil value of the target type instead of untyped nil
				return reflect.Zero(targetType).Interface(), nil
			}
		}
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

func (m *Marshaller) sliceToList(v reflect.Value, depth int) (*evaluator.List, error) {
	elements := make([]evaluator.Object, v.Len())
	for i := 0; i < v.Len(); i++ {
		val, err := m.toValue(v.Index(i).Interface(), depth)
		if err != nil {
			return nil, err
		}
		elements[i] = val
	}
	return evaluator.NewList(elements), nil
}

func (m *Marshaller) mapToFunxyMap(v reflect.Value, depth int) (*evaluator.Map, error) {
	result := evaluator.NewMap()
	iter := v.MapRange()
	for iter.Next() {
		key, err := m.toValue(iter.Key().Interface(), depth)
		if err != nil {
			return nil, fmt.Errorf("map key: %w", err)
		}
		val, err := m.toValue(iter.Value().Interface(), depth)
		if err != nil {
			return nil, fmt.Errorf("map value: %w", err)
		}
		result = result.Put(key, val)
	}
	return result, nil
}

func (m *Marshaller) structToRecord(v reflect.Value, depth int) (*evaluator.RecordInstance, error) {
	fields := make(map[string]evaluator.Object)
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" { // Skip unexported fields
			continue
		}

		name := field.Name
		// Parse tags: funxy first, fallback to json
		tag := field.Tag.Get("funxy")
		if tag == "" {
			tag = field.Tag.Get("json")
		}

		if tag == "-" {
			continue // omit
		} else if tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] != "" {
				name = parts[0]
			}
		}

		val, err := m.toValue(v.Field(i).Interface(), depth)
		if err != nil {
			return nil, err
		}
		fields[name] = val
	}

	return evaluator.NewRecord(fields), nil
}

func (m *Marshaller) recordToStruct(r *evaluator.RecordInstance, targetType reflect.Type, depth int) (interface{}, error) {
	makePtr := false
	structType := targetType
	if targetType.Kind() == reflect.Ptr {
		makePtr = true
		structType = targetType.Elem()
	}

	recordByKey := make(map[string]evaluator.Object, len(r.Fields))
	for _, f := range r.Fields {
		recordByKey[f.Key] = f.Value
	}

	out := reflect.New(structType).Elem()
	for _, fi := range getStructFields(structType) {
		if fi.Omit {
			continue
		}

		targetField := out.Field(fi.Index)
		if !targetField.CanSet() {
			continue
		}

		val, ok := recordByKey[fi.Name]
		if !ok {
			continue // omitted fields keep Go zero value
		}

		goVal, err := m.fromValue(val, targetField.Type(), depth)
		if err != nil {
			return nil, fmt.Errorf("struct field %s: %w", fi.Name, err)
		}
		if goVal == nil {
			switch targetField.Kind() {
			case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Func:
				targetField.Set(reflect.Zero(targetField.Type()))
				continue
			default:
				return nil, fmt.Errorf("struct field %s: cannot assign nil to non-nullable type %s", fi.Name, targetField.Type())
			}
		}

		rv := reflect.ValueOf(goVal)
		if rv.Type().AssignableTo(targetField.Type()) {
			targetField.Set(rv)
		} else if rv.Type().ConvertibleTo(targetField.Type()) {
			targetField.Set(rv.Convert(targetField.Type()))
		} else {
			return nil, fmt.Errorf("struct field %s: cannot convert %s to %s", fi.Name, rv.Type(), targetField.Type())
		}
	}

	if makePtr {
		ptr := reflect.New(structType)
		ptr.Elem().Set(out)
		return ptr.Interface(), nil
	}
	return out.Interface(), nil
}

func (m *Marshaller) listToSlice(l *evaluator.List, targetType reflect.Type, depth int) (interface{}, error) {
	// If targetType is nil, default to []interface{}
	elemType := reflect.TypeOf((*interface{})(nil)).Elem()
	if targetType != nil && targetType.Kind() == reflect.Slice {
		elemType = targetType.Elem()
	}

	slice := reflect.MakeSlice(reflect.SliceOf(elemType), 0, l.Len())

	els := l.ToSlice()
	for _, el := range els {
		val, err := m.fromValue(el, elemType, depth)
		if err != nil {
			return nil, err
		}

		if val == nil {
			// Handle nil for pointers/interfaces
			slice = reflect.Append(slice, reflect.Zero(elemType))
			continue
		}

		rv := reflect.ValueOf(val)
		if rv.Type().AssignableTo(elemType) {
			slice = reflect.Append(slice, rv)
		} else if rv.Type().ConvertibleTo(elemType) {
			slice = reflect.Append(slice, rv.Convert(elemType))
		} else {
			return nil, fmt.Errorf("cannot convert %s to %s", rv.Type(), elemType)
		}
	}
	return slice.Interface(), nil
}

func (m *Marshaller) funxyMapToGoMap(fm *evaluator.Map, targetType reflect.Type, depth int) (interface{}, error) {
	items := fm.Items()

	// If target type is a concrete map type, convert to that
	if targetType != nil && targetType.Kind() == reflect.Map {
		result := reflect.MakeMapWithSize(targetType, len(items))
		keyType := targetType.Key()
		valType := targetType.Elem()
		for _, item := range items {
			key, err := m.fromValue(item.Key, keyType, depth)
			if err != nil {
				return nil, fmt.Errorf("map key: %w", err)
			}
			val, err := m.fromValue(item.Value, valType, depth)
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
		key, err := m.fromValue(item.Key, nil, depth)
		if err != nil {
			return nil, fmt.Errorf("map key: %w", err)
		}
		val, err := m.fromValue(item.Value, nil, depth)
		if err != nil {
			return nil, fmt.Errorf("map value: %w", err)
		}
		result[key] = val
	}
	return result, nil
}

func (m *Marshaller) recordToMap(r *evaluator.RecordInstance, depth int) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, f := range r.Fields {
		val, err := m.fromValue(f.Value, nil, depth)
		if err != nil {
			return nil, err
		}
		result[f.Key] = val
	}
	return result, nil
}

func (m *Marshaller) createGoFunc(obj evaluator.Object, targetType reflect.Type) (interface{}, error) {
	if m.machine == nil {
		return nil, fmt.Errorf("marshaller has no VM reference to create Go func")
	}

	wrapper := reflect.MakeFunc(targetType, func(args []reflect.Value) []reflect.Value {
		funxyArgs := make([]evaluator.Object, len(args))
		for i, arg := range args {
			fArg, err := m.toValue(arg.Interface(), 0)
			if err != nil {
				panic(fmt.Sprintf("callback arg conversion failed: %v", err))
			}
			funxyArgs[i] = fArg
		}

		res, err := m.machine.CallFunction(obj, funxyArgs)

		if err != nil {
			numOut := targetType.NumOut()
			if numOut > 0 {
				lastOutType := targetType.Out(numOut - 1)
				if lastOutType.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
					results := make([]reflect.Value, numOut)
					for i := 0; i < numOut-1; i++ {
						results[i] = reflect.Zero(targetType.Out(i))
					}
					results[numOut-1] = reflect.ValueOf(err)
					return results
				}
			}
			panic(fmt.Sprintf("Funxy callback error: %v", err))
		}

		numOut := targetType.NumOut()
		if numOut == 0 {
			return []reflect.Value{}
		}

		results := make([]reflect.Value, numOut)
		if numOut == 1 {
			goRes, err := m.FromValue(res, targetType.Out(0))
			if err != nil {
				panic(fmt.Sprintf("callback result conversion failed: %v", err))
			}
			if goRes == nil {
				results[0] = reflect.Zero(targetType.Out(0))
			} else {
				results[0] = reflect.ValueOf(goRes)
			}
			return results
		}

		tuple, ok := res.(*evaluator.Tuple)
		if !ok || len(tuple.Elements) != numOut {
			panic(fmt.Sprintf("callback expected %d return values, but Funxy returned %s", numOut, res.Type()))
		}

		for i := 0; i < numOut; i++ {
			goRes, err := m.FromValue(tuple.Elements[i], targetType.Out(i))
			if err != nil {
				panic(fmt.Sprintf("callback result [%d] conversion failed: %v", i, err))
			}
			if goRes == nil {
				results[i] = reflect.Zero(targetType.Out(i))
			} else {
				results[i] = reflect.ValueOf(goRes)
			}
		}

		return results
	})

	return wrapper.Interface(), nil
}

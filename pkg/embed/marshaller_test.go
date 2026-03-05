package funxy

import (
	"github.com/funvibe/funxy/internal/evaluator"
	"reflect"
	"strings"
	"testing"
)

type marshallerOpts struct {
	Addr string
	DB   int
	TLS  bool
}

func TestMarshaller_RecordToStruct(t *testing.T) {
	m := NewMarshaller(nil)

	addrObj, err := m.ToValue("127.0.0.1:6379")
	if err != nil {
		t.Fatalf("ToValue(addr) failed: %v", err)
	}
	dbObj, err := m.ToValue(3)
	if err != nil {
		t.Fatalf("ToValue(db) failed: %v", err)
	}
	tlsObj, err := m.ToValue(true)
	if err != nil {
		t.Fatalf("ToValue(tls) failed: %v", err)
	}
	ignoredObj, err := m.ToValue("ignored")
	if err != nil {
		t.Fatalf("ToValue(ignored) failed: %v", err)
	}

	record := evaluator.NewRecord(map[string]evaluator.Object{
		"Addr":    addrObj,
		"DB":      dbObj,
		"TLS":     tlsObj,
		"ignored": ignoredObj, // Unknown keys must be ignored.
	})

	out, err := m.FromValue(record, reflect.TypeOf(marshallerOpts{}))
	if err != nil {
		t.Fatalf("FromValue(record, struct) failed: %v", err)
	}
	opts, ok := out.(marshallerOpts)
	if !ok {
		t.Fatalf("Expected marshallerOpts, got %T", out)
	}
	if opts.Addr != "127.0.0.1:6379" || opts.DB != 3 || !opts.TLS {
		t.Fatalf("Unexpected struct output: %+v", opts)
	}
}

func TestMarshaller_RecordToStructPointer(t *testing.T) {
	m := NewMarshaller(nil)

	addrObj, err := m.ToValue("localhost:6380")
	if err != nil {
		t.Fatalf("ToValue(addr) failed: %v", err)
	}
	dbObj, err := m.ToValue(9)
	if err != nil {
		t.Fatalf("ToValue(db) failed: %v", err)
	}
	record := evaluator.NewRecord(map[string]evaluator.Object{
		"Addr": addrObj,
		"DB":   dbObj,
	})

	out, err := m.FromValue(record, reflect.TypeOf(&marshallerOpts{}))
	if err != nil {
		t.Fatalf("FromValue(record, *struct) failed: %v", err)
	}
	opts, ok := out.(*marshallerOpts)
	if !ok {
		t.Fatalf("Expected *marshallerOpts, got %T", out)
	}
	if opts.Addr != "localhost:6380" || opts.DB != 9 {
		t.Fatalf("Unexpected pointer struct output: %+v", *opts)
	}
}

func TestMarshaller_RecordToStructRejectsNilForNonNullableField(t *testing.T) {
	m := NewMarshaller(nil)

	addrObj, err := m.ToValue("localhost:6381")
	if err != nil {
		t.Fatalf("ToValue(addr) failed: %v", err)
	}
	record := evaluator.NewRecord(map[string]evaluator.Object{
		"Addr": addrObj,
		"DB":   &evaluator.Nil{},
	})

	_, err = m.FromValue(record, reflect.TypeOf(marshallerOpts{}))
	if err == nil {
		t.Fatal("Expected error for nil -> non-nullable field")
	}
	if !strings.Contains(err.Error(), "cannot assign nil to non-nullable type") {
		t.Fatalf("Unexpected error: %v", err)
	}
}

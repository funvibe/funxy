package funxy

import (
	"github.com/funvibe/funxy/internal/evaluator"
	"reflect"
	"testing"
)

func TestMarshaller_NilToSliceElement(t *testing.T) {
	m := NewMarshaller(nil)

	// A Funxy list: [1, nil, 3]
	list := evaluator.NewList([]evaluator.Object{
		&evaluator.Integer{Value: 1},
		&evaluator.Nil{},
		&evaluator.Integer{Value: 3},
	})

	// Target type: []*int
	intPtrType := reflect.TypeOf((**int)(nil)).Elem()
	sliceType := reflect.SliceOf(intPtrType)

	out, err := m.FromValue(list, sliceType)
	if err != nil {
		t.Fatalf("Failed to convert: %v", err)
	}

	ptrSlice, ok := out.([]*int)
	if !ok {
		t.Fatalf("Expected []*int, got %T", out)
	}

	if len(ptrSlice) != 3 {
		t.Fatalf("Expected len 3, got %d", len(ptrSlice))
	}

	if ptrSlice[0] == nil || *ptrSlice[0] != 1 {
		t.Errorf("Expected ptr to 1, got %v", ptrSlice[0])
	}
	if ptrSlice[1] != nil {
		t.Errorf("Expected nil, got %v", ptrSlice[1])
	}
	if ptrSlice[2] == nil || *ptrSlice[2] != 3 {
		t.Errorf("Expected ptr to 3, got %v", ptrSlice[2])
	}
}

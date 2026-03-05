package funxy

import (
	"fmt"
	"testing"
)

type TestStruct struct {
	FirstName string
	URLValue  string
}

func TestMarshaller_StructToRecord(t *testing.T) {
	m := NewMarshaller(nil)
	v := TestStruct{FirstName: "John", URLValue: "http://example.com"}
	obj, err := m.ToValue(v)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%#v\n", obj.Inspect())
}

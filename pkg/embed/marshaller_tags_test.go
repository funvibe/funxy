package funxy

import (
	"github.com/funvibe/funxy/internal/evaluator"
	"reflect"
	"testing"
)

type TaggedStruct struct {
	VisibleField string `funxy:"visible"`
	HiddenField  string `funxy:"-"`
	JSONField    string `json:"json_field"`
	NoTagField   string
}

func TestMarshaller_Tags(t *testing.T) {
	m := NewMarshaller(nil)
	v := TaggedStruct{
		VisibleField: "visible",
		HiddenField:  "hidden",
		JSONField:    "json",
		NoTagField:   "no_tag",
	}

	// Test ToValue (Go -> Funxy)
	obj, err := m.ToValue(v)
	if err != nil {
		t.Fatalf("ToValue failed: %v", err)
	}

	rec, ok := obj.(*evaluator.RecordInstance)
	if !ok {
		t.Fatalf("Expected RecordInstance, got %T", obj)
	}

	foundVisible := false
	foundHidden := false
	foundJSON := false
	foundNoTag := false

	for _, f := range rec.Fields {
		switch f.Key {
		case "visible":
			foundVisible = true
		case "HiddenField":
			foundHidden = true
		case "json_field":
			foundJSON = true
		case "NoTagField":
			foundNoTag = true
		}
	}

	if !foundVisible {
		t.Errorf("Expected 'visible' field (via funxy tag)")
	}
	if foundHidden {
		t.Errorf("Expected 'HiddenField' to be omitted (via funxy tag '-')")
	}
	if !foundJSON {
		t.Errorf("Expected 'json_field' field (via json tag fallback)")
	}
	if !foundNoTag {
		t.Errorf("Expected 'NoTagField' to be present with default name")
	}

	// Test FromValue (Funxy -> Go)
	// Create mock string values as List of Chars
	visibleVal := stringToList("v2")
	jsonVal := stringToList("j2")
	noTagVal := stringToList("n2")

	rec2 := evaluator.NewRecord(map[string]evaluator.Object{
		"visible":    visibleVal,
		"json_field": jsonVal,
		"NoTagField": noTagVal,
	})

	out, err := m.FromValue(rec2, reflect.TypeOf(TaggedStruct{}))
	if err != nil {
		t.Fatalf("FromValue failed: %v", err)
	}

	outStruct, ok := out.(TaggedStruct)
	if !ok {
		t.Fatalf("Expected TaggedStruct, got %T", out)
	}

	if outStruct.VisibleField != "v2" {
		t.Errorf("Expected VisibleField='v2', got %q", outStruct.VisibleField)
	}
	if outStruct.JSONField != "j2" {
		t.Errorf("Expected JSONField='j2', got %q", outStruct.JSONField)
	}
	if outStruct.NoTagField != "n2" {
		t.Errorf("Expected NoTagField='n2', got %q", outStruct.NoTagField)
	}
}

type CycleStruct struct {
	Name  string
	Child *CycleStruct
}

func TestMarshaller_CyclicReference(t *testing.T) {
	m := NewMarshaller(nil)

	// Create cyclic struct directly since we reverted Pointer handling
	// to make embed_test.go pass (HostObject wrapping)
	// Pointers no longer get dereferenced, so they don't cause cycles in ToValue.
	// But structs with self-referential slice/maps can cause cycles.

	type CycleNode struct {
		Children []interface{}
	}

	root := CycleNode{Children: make([]interface{}, 1)}
	root.Children[0] = root // Value copy of itself

	// Since Go doesn't allow cyclic value types easily,
	// we use interface{} slice pointing to itself.
	cycleSlice := make([]interface{}, 1)
	cycleSlice[0] = cycleSlice

	_, err := m.ToValue(cycleSlice)
	if err == nil {
		t.Errorf("Expected error for cyclic reference")
	} else if err.Error() != "marshaller max depth exceeded (cyclic reference?)" {
		t.Errorf("Expected max depth error, got: %v", err)
	}
}

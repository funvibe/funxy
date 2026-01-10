package typesystem

import (
	"reflect"
	"testing"
)

func TestUnify(t *testing.T) {
	// Helpers
	intType := TCon{Name: "Int"}
	boolType := TCon{Name: "Bool"}
	listCon := TCon{Name: "List"}

	varType := func(name string) TVar { return TVar{Name: name} }
	listType := func(arg Type) TApp { return TApp{Constructor: listCon, Args: []Type{arg}} }

	tests := []struct {
		name    string
		t1      Type
		t2      Type
		wantErr bool
		wantSub Subst
	}{
		{
			name:    "Identity Int",
			t1:      intType,
			t2:      intType,
			wantErr: false,
			wantSub: Subst{},
		},
		{
			name:    "Identity Var",
			t1:      varType("a"),
			t2:      varType("a"),
			wantErr: false,
			wantSub: Subst{},
		},
		{
			name:    "Var to Const",
			t1:      varType("a"),
			t2:      intType,
			wantErr: false,
			wantSub: Subst{"a": intType},
		},
		{
			name:    "Const to Var",
			t1:      intType,
			t2:      varType("a"),
			wantErr: false,
			wantSub: Subst{"a": intType},
		},
		{
			name:    "List a ~ List Int",
			t1:      listType(varType("a")),
			t2:      listType(intType),
			wantErr: false,
			wantSub: Subst{"a": intType},
		},
		{
			name:    "Mismatch Const",
			t1:      intType,
			t2:      boolType,
			wantErr: true,
		},
		{
			name:    "Occurs Check",
			t1:      varType("a"),
			t2:      listType(varType("a")),
			wantErr: true,
		},
		{
			name: "Complex Unification: Map k v ~ Map Int Bool",
			t1: TApp{
				Constructor: TCon{Name: "Map"},
				Args:        []Type{varType("k"), varType("v")},
			},
			t2: TApp{
				Constructor: TCon{Name: "Map"},
				Args:        []Type{intType, boolType},
			},
			wantErr: false,
			wantSub: Subst{"k": intType, "v": boolType},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Unify(tt.t1, tt.t2)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unify() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.wantSub) {
				t.Errorf("Unify() = %v, want %v", got, tt.wantSub)
			}
		})
	}
}

func TestUnifyAllowExtra(t *testing.T) {
	intType := TCon{Name: "Int"}

	// Small: { x: Int }
	smallRec := TRecord{Fields: map[string]Type{"x": intType}}

	// Large: { x: Int, y: Int }
	largeRec := TRecord{Fields: map[string]Type{"x": intType, "y": intType}}

	// Nested Small: { inner: Small }
	nestedSmall := TRecord{Fields: map[string]Type{"inner": smallRec}}

	// Nested Large: { inner: Large }
	nestedLarge := TRecord{Fields: map[string]Type{"inner": largeRec}}

	tests := []struct {
		name     string
		expected Type // t1 (Supertype)
		actual   Type // t2 (Subtype)
		wantErr  bool
	}{
		{
			name:     "Width Subtyping: Large <: Small",
			expected: smallRec,
			actual:   largeRec,
			wantErr:  false,
		},
		{
			name:     "Strictness: Small <: Large (Fail)",
			expected: largeRec,
			actual:   smallRec,
			wantErr:  true,
		},
		{
			name:     "Depth Subtyping (Safe): Nested Large <: Nested Small (FAIL if strict)",
			expected: nestedSmall,
			actual:   nestedLarge,
			wantErr:  true, // We decided to forbid this for safety!
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := UnifyAllowExtra(tt.expected, tt.actual)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnifyAllowExtra() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

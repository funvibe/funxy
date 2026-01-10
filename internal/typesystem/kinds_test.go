package typesystem

import (
	"testing"
)

func TestKinds(t *testing.T) {
	// 1. Check KStar
	if Star.String() != "*" {
		t.Errorf("KStar.String() = %s, want *", Star.String())
	}

	// 2. Check Arrow
	arrow := MakeArrow(Star, Star) // * -> *
	if arrow.String() != "(* -> *)" {
		t.Errorf("Arrow string = %s, want (* -> *)", arrow.String())
	}

	// 3. Check Arrow Equality
	arrow2 := KArrow{Left: Star, Right: Star}
	if !arrow.Equal(arrow2) {
		t.Errorf("Arrows should be equal")
	}

	if arrow.Equal(Star) {
		t.Errorf("Arrow should not equal Star")
	}
}

func TestTypeKinds(t *testing.T) {
	intType := TCon{Name: "Int", KindVal: Star}
	listCon := TCon{Name: "List", KindVal: MakeArrow(Star, Star)}     // * -> *
	mapCon := TCon{Name: "Map", KindVal: MakeArrow(Star, Star, Star)} // * -> * -> *

	tVar := TVar{Name: "a", KindVal: Star}
	tVarM := TVar{Name: "m", KindVal: MakeArrow(Star, Star)}

	tests := []struct {
		name     string
		typ      Type
		wantKind Kind
	}{
		{
			name:     "Int Kind",
			typ:      intType,
			wantKind: Star,
		},
		{
			name:     "List Constructor Kind",
			typ:      listCon,
			wantKind: MakeArrow(Star, Star),
		},
		{
			name:     "TVar Kind",
			typ:      tVar,
			wantKind: Star,
		},
		{
			name:     "TVarM Kind",
			typ:      tVarM,
			wantKind: MakeArrow(Star, Star),
		},
		{
			name:     "List Int Kind",
			typ:      TApp{Constructor: listCon, Args: []Type{intType}},
			wantKind: Star, // (* -> *) applied to * -> *
		},
		{
			name:     "Map Int Kind (Partial)",
			typ:      TApp{Constructor: mapCon, Args: []Type{intType}},
			wantKind: MakeArrow(Star, Star), // (* -> * -> *) applied to * -> (* -> *)
		},
		{
			name:     "Map Int String Kind",
			typ:      TApp{Constructor: mapCon, Args: []Type{intType, TCon{Name: "String"}}},
			wantKind: Star,
		},
		{
			name:     "Tuple Kind",
			typ:      TTuple{Elements: []Type{intType, intType}},
			wantKind: Star,
		},
		{
			name:     "Func Kind",
			typ:      TFunc{Params: []Type{intType}, ReturnType: intType},
			wantKind: Star,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.typ.Kind()
			if !got.Equal(tt.wantKind) {
				t.Errorf("%s Kind() = %s, want %s", tt.name, got, tt.wantKind)
			}
		})
	}
}

func TestKindUnification(t *testing.T) {
	// Setup types
	intType := TCon{Name: "Int", KindVal: Star}
	listCon := TCon{Name: "List", KindVal: MakeArrow(Star, Star)}

	// M :: * -> *
	mVar := TVar{Name: "m", KindVal: MakeArrow(Star, Star)}

	// A :: *
	aVar := TVar{Name: "a", KindVal: Star}

	tests := []struct {
		name    string
		t1      Type
		t2      Type
		wantErr bool
	}{
		{
			name:    "Kind Mismatch: M (*->*) ~ Int (*)",
			t1:      mVar,
			t2:      intType,
			wantErr: true,
		},
		{
			name:    "Kind Match: M (*->*) ~ List (*->*)",
			t1:      mVar,
			t2:      listCon,
			wantErr: false,
		},
		{
			name: "HOU: M<A> ~ List<Int>",
			// M a ~ List Int
			t1:      TApp{Constructor: mVar, Args: []Type{aVar}},
			t2:      TApp{Constructor: listCon, Args: []Type{intType}},
			wantErr: false,
		},
		{
			name: "HOU Partial: M<A> ~ Map<Int, Bool>",
			// M a ~ Map Int Bool
			// Should infer M = Map Int, A = Bool
			t1: TApp{Constructor: mVar, Args: []Type{aVar}},
			t2: TApp{
				Constructor: TCon{Name: "Map", KindVal: MakeArrow(Star, Star, Star)},
				Args:        []Type{intType, TCon{Name: "Bool"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Unify(tt.t1, tt.t2)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unify() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

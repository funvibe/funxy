package funxy

import (
	"reflect"
	"testing"
)

type BenchmarkStruct struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	IsActive bool    `json:"is_active"`
	Score    float64 `funxy:"score"`
	Hidden   string  `funxy:"-"`
	NoTag    string
}

func BenchmarkMarshaller_StructFieldsCache(b *testing.B) {
	typ := reflect.TypeOf(BenchmarkStruct{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getStructFields(typ)
	}
}

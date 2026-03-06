package evaluator

import "github.com/funvibe/funxy/internal/typesystem"

// StringKey is an internal fast string wrapper for PersistentMap keys
type StringKey struct {
	Value string
}

func (s *StringKey) Type() ObjectType             { return "STRING_KEY" }
func (s *StringKey) Inspect() string              { return s.Value }
func (s *StringKey) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "String"} }
func (s *StringKey) Hash() uint32                 { return hashString(s.Value) }

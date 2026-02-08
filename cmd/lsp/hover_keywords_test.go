package main

import (
	"testing"
)

func TestKeywordHover(t *testing.T) {
	tests := []struct {
		name     string
		code     string // Use '$' to mark cursor position
		expected string // Expected substring in hover
	}{
		{
			name:     "Package Keyword",
			code:     "pack$age main",
			expected: "Keyword: package",
		},
		{
			name:     "Import Keyword",
			code:     "imp$ort \"lib/std\"",
			expected: "Keyword: import",
		},
		{
			name:     "As Keyword",
			code:     "import \"lib/std\" a$s std",
			expected: "Keyword: as",
		},
		{
			name:     "Match Keyword",
			code:     "fun test() { mat$ch 1 { _ -> 0 } }",
			expected: "Keyword: match",
		},
		{
			name:     "If Keyword",
			code:     "fun test() { i$f true { } }",
			expected: "Keyword: if",
		},
		{
			name:     "Else Keyword",
			code:     "fun test() { if true { } el$se { } }",
			expected: "Keyword: else",
		},
		{
			name:     "Fun Keyword",
			code:     "f$un main() {}",
			expected: "Keyword: fun",
		},
		{
			name:     "Type Keyword",
			code:     "ty$pe MyInt = Int",
			expected: "Keyword: type",
		},
		{
			name:     "Trait Keyword",
			code:     "tra$it Show { }",
			expected: "Keyword: trait",
		},
		{
			name:     "Instance Keyword",
			code:     "inst$ance Show Int { }",
			expected: "Keyword: instance",
		},
		{
			name:     "Return Keyword",
			code:     "fun test() { ret$urn 1 }",
			expected: "Keyword: return",
		},
		{
			name:     "Break Keyword",
			code:     "fun test() { for x in [] { bre$ak } }",
			expected: "Keyword: break",
		},
		{
			name:     "Continue Keyword",
			code:     "fun test() { for x in [] { cont$inue } }",
			expected: "Keyword: continue",
		},
		{
			name:     "For Keyword",
			code:     "fun test() { f$or x in [] { } }",
			expected: "Keyword: for",
		},
		{
			name:     "While Keyword",
			code:     "fun test() { wh$ile true { } }",
			expected: "Keyword: while",
		},
		{
			name:     "Directive Keyword",
			code:     "direc$tive \"strict\"",
			expected: "Keyword: directive",
		},
		{
			name:     "Alias Keyword",
			code:     "ali$as MyInt = Int",
			expected: "Keyword: alias",
		},
		{
			name:     "True Keyword",
			code:     "tr$ue",
			expected: "Bool",
		},
		{
			name:     "False Keyword",
			code:     "fal$se",
			expected: "Bool",
		},
		{
			name:     "Nil Keyword",
			code:     "n$il",
			expected: "Nil",
		},
		{
			name:     "Operator Keyword",
			code:     "trait Semigroup { oper$ator (+) (a: Self, b: Self) -> Self }",
			expected: "Keyword: operator",
		},
		{
			name:     "In Keyword",
			code:     "fun test() { for x i$n [] {} }",
			expected: "Keyword: in",
		},
		{
			name:     "Underscore Keyword",
			code:     "fun test() { match 1 { _$ -> 0 } }",
			expected: "Keyword: _",
		},
		{
			name:     "Do Keyword",
			code:     "fun test() { d$o { x <- [1]; return x } }",
			expected: "Keyword: do",
		},
		{
			name:     "Const Keyword",
			code:     "con$st x = 1",
			expected: "Keyword: const",
		},
		{
			name:     "Forall Keyword",
			code:     "fun run(f: (for$all a. a -> a)) {}",
			expected: "Keyword: forall",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkHover(t, tt.code, tt.expected)
		})
	}
}

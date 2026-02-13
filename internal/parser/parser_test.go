package parser_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/prettyprinter"
)

var update = flag.Bool("update", false, "update snapshot files")

func TestParser(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"simple_assignment", "a = 5"},
		{"infix_expression", "a = 5 + 2 * 10"},
		{"prefix_expression", "a = -5"},
		{"complex_expression", "a = (b + c) * -d"},
		{"tuple_literal", "x = (1, true, a)"},
		{"empty_tuple", "x = ()"},
		{"nested_tuple", "x = ((1, 2), 3)"},
		{"tuple_type", "type alias Pair = (Int, Bool)"},
		{"pattern_matching_tuple", "match x { (1, a) -> true }"},
		{"function_basic", "fun add(x: Int, y: Int) Int { x + y }"},
		{"function_variadic", "fun sum(...nums) Int { 0 }"},
		{"function_mixed_variadic", "fun process(id: Int, ...args) { 0 }"},
		{"import_simple", `import "lib/json"`},
		{"import_selective", `import "lib/json" (jsonEncode, jsonDecode)`},
		{"import_multiline", "import \"lib/term\" (red, green,\n    bold, table,\n    spinnerStart)"},
		{"import_multiline_nl_after_paren", "import \"lib/term\" (\n    red, green,\n    bold\n)"},
		{"import_alias", `import "lib/json" as json`},
		{"import_all", `import "lib/json" (*)`},
		{"import_exclude", `import "lib/json" !(jsonParse)`},
		{"match_newline_after_arrow", "match x {\n    Ok(v) ->\n        v + 1\n    Fail(e) ->\n        0\n}"},
		{"match_guard_newline_after_arrow", "match x {\n    Ok(v) if v > 0 ->\n        v\n    _ ->\n        0\n}"},
		{"assign_newline_after_eq", "x =\n    5 + 3"},
		{"compound_assign_newline", "x +=\n    10"},
		{"if_else_newline", "if true { 1 }\nelse\n{ 2 }"},
		{"if_else_if_newline", "if true { 1 }\nelse\nif false { 2 } else { 3 }"},
		{"for_in_newline", "for x in\n    [1, 2, 3] {\n    print(x)\n}"},
		{"fun_arrow_newline", "f = fun(x: Int) ->\n    x + 1"},
		{"index_newline", "x[\n    0\n]"},
		{"tuple_pattern_newline", "match x {\n    (a,\n     b) -> a\n}"},
		{"list_pattern_newline", "match x {\n    [\n     a, b\n    ] -> a\n}"},
		{"constructor_pattern_newline", "match x {\n    Ok(a,\n       b) -> a\n    _ -> 0\n}"},
		{"lambda_newline_after_arrow", "(\\x ->\n    x + 1)"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup pipeline context
			ctx := &pipeline.PipelineContext{SourceCode: tc.input}

			// Run lexer
			lexerProcessor := &lexer.LexerProcessor{}
			ctx = lexerProcessor.Process(ctx)

			// Run parser
			parserProcessor := &parser.ParserProcessor{}
			ctx = parserProcessor.Process(ctx)

			if len(ctx.Errors) > 0 {
				var errorMessages []string
				for _, err := range ctx.Errors {
					errorMessages = append(errorMessages, err.Error())
				}
				t.Fatalf("parsing failed with errors:\n%s", strings.Join(errorMessages, "\n"))
			}

			// 1. Tree Printer (AST Structure)
			treePrinter := prettyprinter.NewTreePrinter()
			ctx.AstRoot.Accept(treePrinter)
			treeOutput := treePrinter.String()

			// 2. Code Printer (Source Code Reconstruction)
			codePrinter := prettyprinter.NewCodePrinter()
			ctx.AstRoot.Accept(codePrinter)
			codeOutput := codePrinter.String()

			// Combine outputs — include original input so snapshots show what was parsed
			actual := "--- Input ---\n" + tc.input + "\n\n--- AST Tree ---\n" + treeOutput + "\n--- Source Code ---\n" + codeOutput

			// Snapshot testing
			snapshotFile := filepath.Join("testdata", tc.name+".snap")

			if *update {
				err := os.WriteFile(snapshotFile, []byte(actual), 0644)
				if err != nil {
					t.Fatalf("failed to update snapshot: %v", err)
				}
				return
			}

			expected, err := os.ReadFile(snapshotFile)
			if err != nil {
				t.Fatalf("failed to read snapshot file: %v. Run with -update flag to create it.", err)
			}

			if string(expected) != actual {
				t.Errorf("snapshot mismatch:\n--- expected\n%s\n--- actual\n%s", string(expected), actual)
			}
		})
	}
}

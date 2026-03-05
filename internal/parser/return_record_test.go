package parser_test

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"strings"
	"testing"
)

func TestReturnDisambiguation3(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantType string
	}{
		{"record with assignment in value with parens", "fun test() { return {\n a: (b = 1)\n } }", "*ast.RecordLiteral"},
		{"record with multiline value", "fun test() { return {\n a: b(\n c\n )\n } }", "*ast.RecordLiteral"},
		{"record with nested record", "fun test() { return {\n a: {\n b: 1\n }\n } }", "*ast.RecordLiteral"},
		{"block with assignment (negative for record)", "fun test() { return {\n a: b = 1\n } }", "*ast.BlockStatement"},
		{"block with compound assignment (negative)", "fun test() { return {\n a: b += 1\n } }", "*ast.BlockStatement"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := pipeline.NewPipelineContext(tc.input)
			lexerProcessor := &lexer.LexerProcessor{}
			ctx = lexerProcessor.Process(ctx)
			parserProcessor := &parser.ParserProcessor{}
			ctx = parserProcessor.Process(ctx)

			prog := ctx.AstRoot.(*ast.Program)
			funDecl := prog.Statements[0].(*ast.FunctionStatement)
			block := funDecl.Body
			retStmt := block.Statements[0].(*ast.ReturnStatement)

			gotType := ""
			switch retStmt.Value.(type) {
			case *ast.RecordLiteral:
				gotType = "*ast.RecordLiteral"
			case *ast.BlockStatement:
				gotType = "*ast.BlockStatement"
			default:
				gotType = "other"
			}

			if gotType != tc.wantType {
				t.Errorf("got %s, want %s (errors: %v)", gotType, tc.wantType, ctx.Errors)
			}
		})
	}
}

func TestNegativeRecordAssignments(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		errSubstr string
	}{
		{"assignment in unambiguous record spread", "fun test() { return {\n ...base,\n a: b = 1\n } }", "Assignment is forbidden"},
		{"compound assignment in unambiguous record", "fun test() { return {\n ...base,\n a: b += 1\n } }", "Assignment is forbidden"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := pipeline.NewPipelineContext(tc.input)
			lexerProcessor := &lexer.LexerProcessor{}
			ctx = lexerProcessor.Process(ctx)
			parserProcessor := &parser.ParserProcessor{}
			ctx = parserProcessor.Process(ctx)

			if len(ctx.Errors) == 0 {
				t.Errorf("expected error containing %q, got none", tc.errSubstr)
				return
			}

			found := false
			for _, err := range ctx.Errors {
				if strings.Contains(err.Error(), tc.errSubstr) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error containing %q, got errors: %v", tc.errSubstr, ctx.Errors)
			}
		})
	}
}

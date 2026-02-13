package parser_test

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"testing"
)

// parse is a test helper: lexes+parses input and fails on errors.
func parse(t *testing.T, input string) *ast.Program {
	t.Helper()
	ctx := &pipeline.PipelineContext{SourceCode: input}
	lp := &lexer.LexerProcessor{}
	ctx = lp.Process(ctx)
	pp := &parser.ParserProcessor{}
	ctx = pp.Process(ctx)
	if len(ctx.Errors) > 0 {
		for _, e := range ctx.Errors {
			t.Errorf("parse error: %s", e)
		}
		t.FailNow()
	}
	return ctx.AstRoot.(*ast.Program)
}

// stmtExpr extracts the expression from the nth ExpressionStatement.
func stmtExpr(t *testing.T, prog *ast.Program, idx int) ast.Expression {
	t.Helper()
	if idx >= len(prog.Statements) {
		t.Fatalf("expected at least %d statements, got %d", idx+1, len(prog.Statements))
	}
	es, ok := prog.Statements[idx].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("statement %d: expected ExpressionStatement, got %T", idx, prog.Statements[idx])
	}
	return es.Expression
}

// ---------- import ----------

func TestNewline_ImportMultiline(t *testing.T) {
	prog := parse(t, `import "lib/json" (jsonEncode, jsonDecode)`)
	imp := prog.Statements[0].(*ast.ImportStatement)
	if len(imp.Symbols) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(imp.Symbols))
	}

	// Same result with newlines
	prog2 := parse(t, "import \"lib/json\" (jsonEncode,\n    jsonDecode)")
	imp2 := prog2.Statements[0].(*ast.ImportStatement)
	if len(imp2.Symbols) != 2 {
		t.Fatalf("multiline: expected 2 symbols, got %d", len(imp2.Symbols))
	}
	if imp2.Symbols[0].Value != "jsonEncode" || imp2.Symbols[1].Value != "jsonDecode" {
		t.Fatalf("multiline: wrong symbols: %s, %s", imp2.Symbols[0].Value, imp2.Symbols[1].Value)
	}
}

func TestNewline_ImportNLAfterParen(t *testing.T) {
	prog := parse(t, "import \"lib/json\" (\n    jsonEncode,\n    jsonDecode\n)")
	imp := prog.Statements[0].(*ast.ImportStatement)
	if len(imp.Symbols) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(imp.Symbols))
	}
}

func TestNewline_ImportExcludeMultiline(t *testing.T) {
	prog := parse(t, "import \"lib/json\" !(\n    jsonParse,\n    jsonFormat\n)")
	imp := prog.Statements[0].(*ast.ImportStatement)
	if len(imp.Exclude) != 2 {
		t.Fatalf("expected 2 exclude symbols, got %d", len(imp.Exclude))
	}
}

// ---------- assignment ----------

func TestNewline_AssignAfterEq(t *testing.T) {
	prog := parse(t, "x =\n    5 + 3")
	expr := stmtExpr(t, prog, 0)
	assign, ok := expr.(*ast.AssignExpression)
	if !ok {
		t.Fatalf("expected AssignExpression, got %T", expr)
	}
	infix, ok := assign.Value.(*ast.InfixExpression)
	if !ok {
		t.Fatalf("expected InfixExpression as value, got %T", assign.Value)
	}
	if infix.Operator != "+" {
		t.Fatalf("expected +, got %s", infix.Operator)
	}
}

func TestNewline_CompoundAssign(t *testing.T) {
	// x = 1 first, then x +=\n 10
	prog := parse(t, "x = 1\nx +=\n    10")
	if len(prog.Statements) < 2 {
		t.Fatalf("expected 2 statements, got %d", len(prog.Statements))
	}
	expr := stmtExpr(t, prog, 1)
	assign, ok := expr.(*ast.AssignExpression)
	if !ok {
		t.Fatalf("expected AssignExpression, got %T", expr)
	}
	// compound assign desugars to x = x + 10
	infix, ok := assign.Value.(*ast.InfixExpression)
	if !ok {
		t.Fatalf("expected InfixExpression as value, got %T", assign.Value)
	}
	if infix.Operator != "+" {
		t.Fatalf("expected +, got %s", infix.Operator)
	}
}

// ---------- if/else ----------

func TestNewline_ElseNewline(t *testing.T) {
	prog := parse(t, "if true { 1 }\nelse\n{ 2 }")
	expr := stmtExpr(t, prog, 0)
	ifExpr, ok := expr.(*ast.IfExpression)
	if !ok {
		t.Fatalf("expected IfExpression, got %T", expr)
	}
	if ifExpr.Alternative == nil {
		t.Fatal("expected else branch, got nil")
	}
}

func TestNewline_ElseIfNewline(t *testing.T) {
	prog := parse(t, "if true { 1 }\nelse\nif false { 2 } else { 3 }")
	expr := stmtExpr(t, prog, 0)
	ifExpr, ok := expr.(*ast.IfExpression)
	if !ok {
		t.Fatalf("expected IfExpression, got %T", expr)
	}
	if ifExpr.Alternative == nil {
		t.Fatal("expected else-if branch, got nil")
	}
}

// ---------- for-in ----------

func TestNewline_ForInNewline(t *testing.T) {
	prog := parse(t, "for x in\n    [1, 2, 3] {\n    print(x)\n}")
	expr := stmtExpr(t, prog, 0)
	forExpr, ok := expr.(*ast.ForExpression)
	if !ok {
		t.Fatalf("expected ForExpression, got %T", expr)
	}
	if forExpr.ItemName == nil {
		t.Fatal("expected ItemName, got nil")
	}
	if forExpr.ItemName.Value != "x" {
		t.Fatalf("expected item name 'x', got '%s'", forExpr.ItemName.Value)
	}
	if forExpr.Iterable == nil {
		t.Fatal("expected Iterable, got nil")
	}
}

// ---------- match ----------

func TestNewline_MatchArrowNewline(t *testing.T) {
	prog := parse(t, "match x {\n    Ok(v) ->\n        v + 1\n    Fail(e) ->\n        0\n}")
	expr := stmtExpr(t, prog, 0)
	matchExpr, ok := expr.(*ast.MatchExpression)
	if !ok {
		t.Fatalf("expected MatchExpression, got %T", expr)
	}
	if len(matchExpr.Arms) != 2 {
		t.Fatalf("expected 2 arms, got %d", len(matchExpr.Arms))
	}
	// First arm body should be infix +
	infix, ok := matchExpr.Arms[0].Expression.(*ast.InfixExpression)
	if !ok {
		t.Fatalf("arm 0: expected InfixExpression, got %T", matchExpr.Arms[0].Expression)
	}
	if infix.Operator != "+" {
		t.Fatalf("arm 0: expected +, got %s", infix.Operator)
	}
}

func TestNewline_MatchGuardNewline(t *testing.T) {
	prog := parse(t, "match x {\n    Ok(v) if v > 0 ->\n        v\n    _ ->\n        0\n}")
	expr := stmtExpr(t, prog, 0)
	matchExpr, ok := expr.(*ast.MatchExpression)
	if !ok {
		t.Fatalf("expected MatchExpression, got %T", expr)
	}
	if matchExpr.Arms[0].Guard == nil {
		t.Fatal("arm 0: expected guard, got nil")
	}
}

// ---------- lambda ----------

func TestNewline_LambdaArrowNewline(t *testing.T) {
	prog := parse(t, "f = \\x ->\n    x + 1")
	expr := stmtExpr(t, prog, 0)
	assign, ok := expr.(*ast.AssignExpression)
	if !ok {
		t.Fatalf("expected AssignExpression, got %T", expr)
	}
	lambda, ok := assign.Value.(*ast.FunctionLiteral)
	if !ok {
		t.Fatalf("expected FunctionLiteral, got %T", assign.Value)
	}
	if len(lambda.Parameters) != 1 || lambda.Parameters[0].Name.Value != "x" {
		t.Fatalf("expected 1 param 'x', got %d params", len(lambda.Parameters))
	}
}

// ---------- fun literal ----------

func TestNewline_FunLiteralArrowNewline(t *testing.T) {
	prog := parse(t, "f = fun(x: Int) ->\n    x + 1")
	expr := stmtExpr(t, prog, 0)
	assign, ok := expr.(*ast.AssignExpression)
	if !ok {
		t.Fatalf("expected AssignExpression, got %T", expr)
	}
	_, ok = assign.Value.(*ast.FunctionLiteral)
	if !ok {
		t.Fatalf("expected FunctionLiteral, got %T", assign.Value)
	}
}

// ---------- index ----------

func TestNewline_IndexNewline(t *testing.T) {
	prog := parse(t, "x = [1, 2, 3]\ny = x[\n    0\n]")
	expr := stmtExpr(t, prog, 1)
	assign, ok := expr.(*ast.AssignExpression)
	if !ok {
		t.Fatalf("expected AssignExpression, got %T", expr)
	}
	idx, ok := assign.Value.(*ast.IndexExpression)
	if !ok {
		t.Fatalf("expected IndexExpression, got %T", assign.Value)
	}
	intLit, ok := idx.Index.(*ast.IntegerLiteral)
	if !ok {
		t.Fatalf("expected IntegerLiteral index, got %T", idx.Index)
	}
	if intLit.Value != 0 {
		t.Fatalf("expected index 0, got %d", intLit.Value)
	}
}

// ---------- patterns ----------

func TestNewline_TuplePatternNewline(t *testing.T) {
	prog := parse(t, "match x {\n    (a,\n     b) -> a\n}")
	expr := stmtExpr(t, prog, 0)
	matchExpr := expr.(*ast.MatchExpression)
	arm := matchExpr.Arms[0]
	tp, ok := arm.Pattern.(*ast.TuplePattern)
	if !ok {
		t.Fatalf("expected TuplePattern, got %T", arm.Pattern)
	}
	if len(tp.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(tp.Elements))
	}
}

func TestNewline_ListPatternNewline(t *testing.T) {
	prog := parse(t, "match x {\n    [\n     a, b\n    ] -> a\n}")
	expr := stmtExpr(t, prog, 0)
	matchExpr := expr.(*ast.MatchExpression)
	arm := matchExpr.Arms[0]
	lp, ok := arm.Pattern.(*ast.ListPattern)
	if !ok {
		t.Fatalf("expected ListPattern, got %T", arm.Pattern)
	}
	if len(lp.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(lp.Elements))
	}
}

func TestNewline_ConstructorPatternNewline(t *testing.T) {
	prog := parse(t, "match x {\n    Ok(a,\n       b) -> a\n    _ -> 0\n}")
	expr := stmtExpr(t, prog, 0)
	matchExpr := expr.(*ast.MatchExpression)
	arm := matchExpr.Arms[0]
	cp, ok := arm.Pattern.(*ast.ConstructorPattern)
	if !ok {
		t.Fatalf("expected ConstructorPattern, got %T", arm.Pattern)
	}
	if len(cp.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(cp.Elements))
	}
}

// ---------- const decl ----------

func TestNewline_ConstDeclNewline(t *testing.T) {
	prog := parse(t, "pi :-\n    3")
	if len(prog.Statements) == 0 {
		t.Fatal("expected at least 1 statement")
	}
	cd, ok := prog.Statements[0].(*ast.ConstantDeclaration)
	if !ok {
		t.Fatalf("expected ConstantDeclaration, got %T", prog.Statements[0])
	}
	if cd.Name.Value != "pi" {
		t.Fatalf("expected name 'pi', got '%s'", cd.Name.Value)
	}
	intLit, ok := cd.Value.(*ast.IntegerLiteral)
	if !ok {
		t.Fatalf("expected IntegerLiteral, got %T", cd.Value)
	}
	if intLit.Value != 3 {
		t.Fatalf("expected 3, got %d", intLit.Value)
	}
}

// ---------- call args ----------

func TestNewline_CallArgsNewline(t *testing.T) {
	prog := parse(t, "print(1,\n    2,\n    3)")
	expr := stmtExpr(t, prog, 0)
	call, ok := expr.(*ast.CallExpression)
	if !ok {
		t.Fatalf("expected CallExpression, got %T", expr)
	}
	if len(call.Arguments) != 3 {
		t.Fatalf("expected 3 arguments, got %d", len(call.Arguments))
	}
}

// ---------- list literal ----------

func TestNewline_ListLiteralNewline(t *testing.T) {
	prog := parse(t, "x = [\n    1,\n    2,\n    3\n]")
	expr := stmtExpr(t, prog, 0)
	assign := expr.(*ast.AssignExpression)
	list, ok := assign.Value.(*ast.ListLiteral)
	if !ok {
		t.Fatalf("expected ListLiteral, got %T", assign.Value)
	}
	if len(list.Elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(list.Elements))
	}
}

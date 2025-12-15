package prettyprinter

import (
	"bytes"
	"github.com/funvibe/funxy/internal/ast"
	"strings"
)

// --- Code Printer (Output looks like source code) ---

// Operator precedence (higher = binds tighter)
var operatorPrecedence = map[string]int{
	"||":  1,
	"&&":  2,
	"==":  3,
	"!=":  3,
	"<":   4,
	">":   4,
	"<=":  4,
	">=":  4,
	"<>":  5, // Semigroup
	"++":  5, // Concatenation
	"|>":  6, // Pipe
	"<|":  6,
	"+":   7,
	"-":   7,
	"*":   8,
	"/":   8,
	"%":   8,
	"**":  9, // Power (right-assoc)
	",,":  9, // Composition
	">>=": 2, // Monad bind
	"<*>": 5, // Applicative
	"::":  6, // Cons
	"$":   0, // Application (lowest)
}

func getPrecedence(op string) int {
	if p, ok := operatorPrecedence[op]; ok {
		return p
	}
	return 10 // Default high precedence for unknown ops
}

// Right-associative operators
var rightAssoc = map[string]bool{
	"**": true,
	"$":  true,
	"::": true,
	",,": true,
}

type CodePrinter struct {
	buf       bytes.Buffer
	indent    int
	lineWidth int // max line width (0 = unlimited)
	column    int // current column position
}

func NewCodePrinter() *CodePrinter {
	return &CodePrinter{indent: 0, lineWidth: 100, column: 0}
}

func NewCodePrinterWithWidth(width int) *CodePrinter {
	return &CodePrinter{indent: 0, lineWidth: width, column: 0}
}

func (p *CodePrinter) SetLineWidth(width int) {
	p.lineWidth = width
}

func (p *CodePrinter) writeIndent() {
	for i := 0; i < p.indent; i++ {
		p.buf.WriteString("    ")
	}
	p.column = p.indent * 4
}

// countPipeSteps counts the number of |> operators in a chain (left-associative)
func countPipeSteps(expr ast.Expression) int {
	infix, ok := expr.(*ast.InfixExpression)
	if !ok || infix.Operator != "|>" {
		return 0
	}
	return 1 + countPipeSteps(infix.Left)
}

// printExpr prints an expression, adding parentheses only if needed
func (p *CodePrinter) printExpr(expr ast.Expression, parentPrec int, isRight bool) {
	switch e := expr.(type) {
	case *ast.InfixExpression:
		prec := getPrecedence(e.Operator)
		needParens := prec < parentPrec
		// For same precedence, check associativity
		if prec == parentPrec {
			if isRight && !rightAssoc[e.Operator] {
				needParens = true
			} else if !isRight && rightAssoc[e.Operator] {
				needParens = true
			}
		}
		if needParens {
			p.write("(")
		}

		// Special handling for pipe chains
		if e.Operator == "|>" && countPipeSteps(e) >= 2 && parentPrec == 0 {
			p.printPipeChain(e)
		} else {
			p.printExpr(e.Left, prec, false)
			p.write(" " + e.Operator + " ")
			p.printExpr(e.Right, prec, true)
		}

		if needParens {
			p.write(")")
		}
	case *ast.PrefixExpression:
		p.write(e.Operator)
		// Prefix has high precedence
		p.printExpr(e.Right, 100, false)
	default:
		// For non-infix expressions, just use visitor
		expr.Accept(p)
	}
}

// printPipeChain prints a |> chain with each step on a new line
// Pipe is left-associative: a |> b |> c parses as ((a |> b) |> c)
func (p *CodePrinter) printPipeChain(expr *ast.InfixExpression) {
	// Collect all steps by traversing left
	var steps []ast.Expression
	current := ast.Expression(expr)
	for {
		infix, ok := current.(*ast.InfixExpression)
		if !ok || infix.Operator != "|>" {
			// This is the leftmost (source) expression
			steps = append(steps, current)
			break
		}
		// Prepend the right side (the function being piped to)
		steps = append(steps, infix.Right)
		current = infix.Left
	}

	// Reverse to get [source, step1, step2, ...]
	for i, j := 0, len(steps)-1; i < j; i, j = i+1, j-1 {
		steps[i], steps[j] = steps[j], steps[i]
	}

	// Print first step (source)
	steps[0].Accept(p)

	// Print remaining steps on new lines
	p.indent++
	for i := 1; i < len(steps); i++ {
		p.writeln()
		p.writeIndent()
		p.write("|> ")
		steps[i].Accept(p)
	}
	p.indent--
}

func (p *CodePrinter) String() string {
	return p.buf.String()
}

func (p *CodePrinter) write(s string) {
	p.buf.WriteString(s)
	// Track column position
	if idx := strings.LastIndex(s, "\n"); idx != -1 {
		p.column = len(s) - idx - 1
	} else {
		p.column += len(s)
	}
}

func (p *CodePrinter) writeln() {
	p.buf.WriteString("\n")
	p.column = 0
}

func (p *CodePrinter) VisitPackageDeclaration(n *ast.PackageDeclaration) {
	p.write("package ")
	p.write(n.Name.Value)
	if n.ExportAll || len(n.Exports) > 0 {
		p.write(" (")
		first := true
		if n.ExportAll {
			p.write("*")
			first = false
		}
		for _, ex := range n.Exports {
			if !first {
				p.write(", ")
			}
			first = false
			if ex.IsReexport() {
				p.write(ex.ModuleName.Value)
				p.write("(")
				if ex.ReexportAll {
					p.write("*")
				} else {
					for j, sym := range ex.Symbols {
						if j > 0 {
							p.write(", ")
						}
						p.write(sym.Value)
					}
				}
				p.write(")")
			} else {
				p.write(ex.Symbol.Value)
			}
		}
		p.write(")")
	}
	p.write("\n")
}

func (p *CodePrinter) VisitImportStatement(n *ast.ImportStatement) {
	p.write("import ")
	p.write("\"" + n.Path.Value + "\"")
	if n.Alias != nil {
		p.write(" as ")
		p.write(n.Alias.Value)
	}
	p.write("\n")
}

func (p *CodePrinter) VisitProgram(n *ast.Program) {
	for _, stmt := range n.Statements {
		stmt.Accept(p)
		p.write("\n")
	}
}

func (p *CodePrinter) VisitExpressionStatement(n *ast.ExpressionStatement) {
	n.Expression.Accept(p)
}

func (p *CodePrinter) VisitFunctionStatement(n *ast.FunctionStatement) {
	p.write("fun ")
	p.write(n.Name.Value)

	// Generics <T: Show>
	if len(n.TypeParams) > 0 {
		p.write("<")
		for i, tp := range n.TypeParams {
			if i > 0 {
				p.write(", ")
			}
			p.write(tp.Value)

			// Find constraints for this param
			var constraints []string
			for _, c := range n.Constraints {
				if c.TypeVar == tp.Value {
					constraints = append(constraints, c.Trait)
				}
			}
			if len(constraints) > 0 {
				p.write(": ")
				p.write(strings.Join(constraints, " + "))
			}
		}
		p.write(">")
	}

	if len(n.Parameters) > 3 {
		// Multiline parameters with alignment
		maxNameLen := 0
		for _, param := range n.Parameters {
			if len(param.Name.Value) > maxNameLen {
				maxNameLen = len(param.Name.Value)
			}
		}

		p.write("(\n")
		p.indent++
		for i, param := range n.Parameters {
			p.writeIndent()
			p.write(param.Name.Value)
			// Align colons
			for j := len(param.Name.Value); j < maxNameLen; j++ {
				p.write(" ")
			}
			p.write(": ")
			if param.IsVariadic {
				p.write("...")
			}
			if param.Type != nil {
				param.Type.Accept(p)
			}
			if i < len(n.Parameters)-1 {
				p.write(",")
			}
			p.writeln()
		}
		p.indent--
		p.writeIndent()
		p.write(")")
	} else {
		p.write("(")
		for i, param := range n.Parameters {
			if i > 0 {
				p.write(", ")
			}
			p.write(param.Name.Value)
			p.write(": ")
			if param.IsVariadic {
				p.write("...")
			}
			if param.Type != nil {
				param.Type.Accept(p)
			}
		}
		p.write(")")
	}

	if n.ReturnType != nil {
		p.write(" -> ")
		n.ReturnType.Accept(p)
	}

	p.write(" ")
	if n.Body != nil {
		n.Body.Accept(p)
	}
}

func (p *CodePrinter) VisitTraitDeclaration(n *ast.TraitDeclaration) {
	p.write("trait ")
	p.write(n.Name.Value)
	if len(n.TypeParams) > 0 {
		p.write("<")
		for i, tp := range n.TypeParams {
			if i > 0 {
				p.write(", ")
			}
			p.write(tp.Value)
		}
		p.write(">")
	}
	p.write(" {\n")
	for _, method := range n.Signatures {
		method.Accept(p) // Prints the function signature
		p.write("\n")
	}
	p.write("}")
}

func (p *CodePrinter) VisitInstanceDeclaration(n *ast.InstanceDeclaration) {
	p.write("instance ")
	p.write(n.TraitName.Value)
	p.write(" ")
	n.Target.Accept(p)
	p.write(" {\n")
	for _, method := range n.Methods {
		method.Accept(p)
		p.write("\n")
	}
	p.write("}")
}

func (p *CodePrinter) VisitConstantDeclaration(n *ast.ConstantDeclaration) {
	n.Name.Accept(p)
	if n.TypeAnnotation != nil {
		p.write(": ")
		n.TypeAnnotation.Accept(p)
	}
	p.write(" :- ")
	n.Value.Accept(p)
}

func (p *CodePrinter) VisitFunctionLiteral(n *ast.FunctionLiteral) {
	p.write("fun(")
	for i, param := range n.Parameters {
		if i > 0 {
			p.write(", ")
		}
		p.write(param.Name.Value)
		if param.Type != nil {
			p.write(": ")
			param.Type.Accept(p)
		}
	}
	p.write(")")
	if n.ReturnType != nil {
		p.write(" -> ")
		n.ReturnType.Accept(p)
	}
	p.write(" ")
	n.Body.Accept(p)
}

func (p *CodePrinter) VisitIdentifier(n *ast.Identifier) {
	p.write(n.Value)
}

func (p *CodePrinter) VisitIntegerLiteral(n *ast.IntegerLiteral) {
	p.write(n.Token.Lexeme)
}

func (p *CodePrinter) VisitFloatLiteral(n *ast.FloatLiteral) {
	p.write(n.Token.Lexeme)
}

func (p *CodePrinter) VisitBigIntLiteral(n *ast.BigIntLiteral) {
	p.write(n.Token.Lexeme)
}

func (p *CodePrinter) VisitRationalLiteral(n *ast.RationalLiteral) {
	p.write(n.Token.Lexeme)
}

func (p *CodePrinter) VisitBooleanLiteral(n *ast.BooleanLiteral) {
	p.write(n.Token.Lexeme)
}

func (p *CodePrinter) VisitNilLiteral(n *ast.NilLiteral) {
	p.write("nil")
}

func (p *CodePrinter) VisitTupleLiteral(n *ast.TupleLiteral) {
	if len(n.Elements) > 4 {
		// Multiline for large tuples
		p.write("(\n")
		p.indent++
		for i, el := range n.Elements {
			p.writeIndent()
			el.Accept(p)
			if i < len(n.Elements)-1 {
				p.write(",")
			}
			p.writeln()
		}
		p.indent--
		p.writeIndent()
		p.write(")")
	} else {
		p.write("(")
		for i, el := range n.Elements {
			if i > 0 {
				p.write(", ")
			}
			el.Accept(p)
		}
		p.write(")")
	}
}

func (p *CodePrinter) VisitListLiteral(n *ast.ListLiteral) {
	if len(n.Elements) > 5 {
		// Multiline for large lists
		p.write("[\n")
		p.indent++
		for i, el := range n.Elements {
			p.writeIndent()
			el.Accept(p)
			if i < len(n.Elements)-1 {
				p.write(",")
			}
			p.writeln()
		}
		p.indent--
		p.writeIndent()
		p.write("]")
	} else {
		p.write("[")
		for i, el := range n.Elements {
			if i > 0 {
				p.write(", ")
			}
			el.Accept(p)
		}
		p.write("]")
	}
}

func (p *CodePrinter) VisitIndexExpression(n *ast.IndexExpression) {
	n.Left.Accept(p)
	p.write("[")
	n.Index.Accept(p)
	p.write("]")
}

func (p *CodePrinter) VisitStringLiteral(n *ast.StringLiteral) {
	p.write("\"" + n.Value + "\"")
}

func (p *CodePrinter) VisitFormatStringLiteral(n *ast.FormatStringLiteral) {
	p.write("%\"" + n.Value + "\"")
}

func (p *CodePrinter) VisitInterpolatedString(n *ast.InterpolatedString) {
	p.write("\"")
	for _, part := range n.Parts {
		if sl, ok := part.(*ast.StringLiteral); ok {
			p.write(sl.Value)
		} else {
			p.write("${")
			part.Accept(p)
			p.write("}")
		}
	}
	p.write("\"")
}

func (p *CodePrinter) VisitCharLiteral(n *ast.CharLiteral) {
	p.write("'" + string(rune(n.Value)) + "'")
}

func (p *CodePrinter) VisitBytesLiteral(n *ast.BytesLiteral) {
	switch n.Kind {
	case "string":
		p.write("@\"" + n.Content + "\"")
	case "hex":
		p.write("@x\"" + n.Content + "\"")
	case "bin":
		p.write("@b\"" + n.Content + "\"")
	}
}

func (p *CodePrinter) VisitBitsLiteral(n *ast.BitsLiteral) {
	switch n.Kind {
	case "bin":
		p.write("#b\"" + n.Content + "\"")
	case "hex":
		p.write("#x\"" + n.Content + "\"")
	case "oct":
		p.write("#o\"" + n.Content + "\"")
	}
}

func (p *CodePrinter) VisitTupleType(n *ast.TupleType) {
	p.write("(")
	for i, t := range n.Types {
		if i > 0 {
			p.write(", ")
		}
		t.Accept(p)
	}
	p.write(")")
}

func (p *CodePrinter) VisitFunctionType(n *ast.FunctionType) {
	p.write("(")
	for i, t := range n.Parameters {
		if i > 0 {
			p.write(", ")
		}
		t.Accept(p)
	}
	p.write(") -> ")
	n.ReturnType.Accept(p)
}

func (p *CodePrinter) VisitPrefixExpression(n *ast.PrefixExpression) {
	p.write(n.Operator)
	p.printExpr(n.Right, 100, false)
}

func (p *CodePrinter) VisitInfixExpression(n *ast.InfixExpression) {
	// When called directly (not via printExpr), use lowest precedence context
	p.printExpr(n, 0, false)
}

func (p *CodePrinter) VisitOperatorAsFunction(n *ast.OperatorAsFunction) {
	p.write("(" + n.Operator + ")")
}

func (p *CodePrinter) VisitPostfixExpression(n *ast.PostfixExpression) {
	n.Left.Accept(p)
	p.write(n.Operator)
}

func (p *CodePrinter) VisitAssignExpression(n *ast.AssignExpression) {
	n.Left.Accept(p)
	p.write(" = ")
	n.Value.Accept(p)
}

func (p *CodePrinter) VisitPatternAssignExpression(n *ast.PatternAssignExpression) {
	n.Pattern.Accept(p)
	p.write(" = ")
	n.Value.Accept(p)
}

func (p *CodePrinter) VisitAnnotatedExpression(n *ast.AnnotatedExpression) {
	n.Expression.Accept(p)
	p.write(": ")
	n.TypeAnnotation.Accept(p)
}

func (p *CodePrinter) VisitCallExpression(n *ast.CallExpression) {
	n.Function.Accept(p)
	p.write("(")

	// If many args or long, format multiline
	multiline := len(n.Arguments) > 4

	for i, arg := range n.Arguments {
		if i > 0 {
			p.write(", ")
			if multiline {
				p.writeln()
				p.writeIndent()
				p.write("    ") // extra indent for args
			}
		}
		arg.Accept(p)
	}
	p.write(")")
}

func (p *CodePrinter) VisitTypeApplicationExpression(n *ast.TypeApplicationExpression) {
	n.Expression.Accept(p)
	p.write("<")
	for i, t := range n.TypeArguments {
		if i > 0 {
			p.write(", ")
		}
		t.Accept(p)
	}
	p.write(">")
}

func (p *CodePrinter) VisitBlockStatement(n *ast.BlockStatement) {
	p.write("{\n")
	p.indent++
	for _, stmt := range n.Statements {
		p.writeIndent()
		stmt.Accept(p)
		p.write("\n")
	}
	p.indent--
	p.writeIndent()
	p.write("}")
}

func (p *CodePrinter) VisitIfExpression(n *ast.IfExpression) {
	p.write("if ")
	n.Condition.Accept(p)
	p.write(" ")
	n.Consequence.Accept(p)
	if n.Alternative != nil {
		p.write(" else ")
		n.Alternative.Accept(p)
	}
}

func (p *CodePrinter) VisitForExpression(n *ast.ForExpression) {
	p.write("for ")
	if n.Iterable != nil {
		// for item in iterable
		p.write(n.ItemName.Value)
		p.write(" in ")
		n.Iterable.Accept(p)
	} else {
		// for condition
		n.Condition.Accept(p)
	}
	p.write(" ")
	n.Body.Accept(p)
}

func (p *CodePrinter) VisitBreakStatement(n *ast.BreakStatement) {
	p.write("break")
	if n.Value != nil {
		p.write(" ")
		n.Value.Accept(p)
	}
}

func (p *CodePrinter) VisitContinueStatement(n *ast.ContinueStatement) {
	p.write("continue")
}

func (p *CodePrinter) VisitTypeDeclarationStatement(n *ast.TypeDeclarationStatement) {
	p.write("type ")
	if n.IsAlias {
		p.write("alias ")
	}
	n.Name.Accept(p)

	if len(n.TypeParameters) > 0 {
		p.write("<")
		for i, param := range n.TypeParameters {
			if i > 0 {
				p.write(", ")
			}
			param.Accept(p)
		}
		p.write(">")
	}

	p.write(" = ")
	if n.TargetType != nil {
		n.TargetType.Accept(p)
	}

	for i, c := range n.Constructors {
		if i > 0 {
			p.write(" | ")
		}
		c.Accept(p)
	}
}

func (p *CodePrinter) VisitNamedType(n *ast.NamedType) {
	n.Name.Accept(p)
	for _, arg := range n.Args {
		p.write(" ")
		arg.Accept(p)
	}
}

func (p *CodePrinter) VisitDataConstructor(n *ast.DataConstructor) {
	n.Name.Accept(p)
	if len(n.Parameters) > 0 {
		p.write("(")
		for i, param := range n.Parameters {
			if i > 0 {
				p.write(", ")
			}
			param.Accept(p)
		}
		p.write(")")
	}
}

func (p *CodePrinter) VisitMatchExpression(n *ast.MatchExpression) {
	p.write("match ")
	n.Expression.Accept(p)
	p.write(" {\n")
	p.indent++

	// Calculate max pattern width for alignment
	maxPatLen := 0
	patStrings := make([]string, len(n.Arms))
	for i, arm := range n.Arms {
		// Print pattern to temp buffer to get length
		temp := &CodePrinter{indent: 0, lineWidth: 0}
		arm.Pattern.Accept(temp)
		patStrings[i] = temp.String()
		if len(patStrings[i]) > maxPatLen {
			maxPatLen = len(patStrings[i])
		}
	}

	for i, arm := range n.Arms {
		p.writeIndent()
		p.write(patStrings[i])
		// Align arrows
		for j := len(patStrings[i]); j < maxPatLen; j++ {
			p.write(" ")
		}
		p.write(" -> ")
		arm.Expression.Accept(p)
		p.write("\n")
	}
	p.indent--
	p.writeIndent()
	p.write("}")
}

func (p *CodePrinter) VisitWildcardPattern(n *ast.WildcardPattern) { p.write("_") }
func (p *CodePrinter) VisitLiteralPattern(n *ast.LiteralPattern)   { p.write(n.Token.Lexeme) }
func (p *CodePrinter) VisitIdentifierPattern(n *ast.IdentifierPattern) {
	p.write(n.Value)
}
func (p *CodePrinter) VisitConstructorPattern(n *ast.ConstructorPattern) {
	n.Name.Accept(p)
	if len(n.Elements) > 0 {
		p.write("(")
		for i, el := range n.Elements {
			if i > 0 {
				p.write(", ")
			}
			el.Accept(p)
		}
		p.write(")")
	}
}

func (p *CodePrinter) VisitListPattern(n *ast.ListPattern) {
	p.write("[")
	for i, el := range n.Elements {
		if i > 0 {
			p.write(", ")
		}
		el.Accept(p)
	}
	p.write("]")
}

func (p *CodePrinter) VisitTuplePattern(n *ast.TuplePattern) {
	p.write("(")
	for i, el := range n.Elements {
		if i > 0 {
			p.write(", ")
		}
		el.Accept(p)
	}
	p.write(")")
}

func (p *CodePrinter) VisitRecordPattern(n *ast.RecordPattern) {
	p.write("{")
	i := 0
	for k, v := range n.Fields {
		if i > 0 {
			p.write(", ")
		}
		p.write(k)
		p.write(": ")
		v.Accept(p)
		i++
	}
	p.write("}")
}

func (p *CodePrinter) VisitTypePattern(n *ast.TypePattern) {
	p.write(n.Name)
	p.write(": ")
	n.Type.Accept(p)
}

func (p *CodePrinter) VisitStringPattern(n *ast.StringPattern) {
	p.write("\"")
	for _, part := range n.Parts {
		if part.IsCapture {
			p.write("{")
			p.write(part.Value)
			if part.Greedy {
				p.write("...")
			}
			p.write("}")
		} else {
			p.write(part.Value)
		}
	}
	p.write("\"")
}

func (p *CodePrinter) VisitPinPattern(n *ast.PinPattern) {
	p.write("^")
	p.write(n.Name)
}

func (p *CodePrinter) VisitSpreadExpression(n *ast.SpreadExpression) {
	n.Expression.Accept(p)
	p.write("...")
}

func (p *CodePrinter) VisitSpreadPattern(n *ast.SpreadPattern) {
	n.Pattern.Accept(p)
	p.write("...")
}

func (p *CodePrinter) VisitRecordLiteral(n *ast.RecordLiteral) {
	// Check if it's shorthand { name } where value is same as key
	isShorthand := func(k string, v ast.Expression) bool {
		if v == nil {
			return true
		}
		if ident, ok := v.(*ast.Identifier); ok {
			return ident.Value == k
		}
		return false
	}

	// Check if all fields are shorthand
	allShorthand := true
	for k, v := range n.Fields {
		if !isShorthand(k, v) {
			allShorthand = false
			break
		}
	}

	// Multi-field records with alignment
	if len(n.Fields) > 3 && !allShorthand {
		// Find max key length for alignment
		maxKeyLen := 0
		keys := make([]string, 0, len(n.Fields))
		for k := range n.Fields {
			keys = append(keys, k)
			if len(k) > maxKeyLen {
				maxKeyLen = len(k)
			}
		}

		p.write("{\n")
		p.indent++
		for i, k := range keys {
			p.writeIndent()
			p.write(k)
			// Align colons
			for j := len(k); j < maxKeyLen; j++ {
				p.write(" ")
			}
			p.write(": ")
			n.Fields[k].Accept(p)
			if i < len(keys)-1 {
				p.write(",")
			}
			p.writeln()
		}
		p.indent--
		p.writeIndent()
		p.write("}")
	} else if allShorthand && len(n.Fields) > 3 {
		// Multiline shorthand with commas
		p.write("{ ")
		keys := make([]string, 0, len(n.Fields))
		for k := range n.Fields {
			keys = append(keys, k)
		}
		for i, k := range keys {
			if i > 0 {
				p.write(", ")
			}
			p.write(k)
		}
		p.write(" }")
	} else {
		// Inline for small records
		p.write("{ ")
		i := 0
		for k, v := range n.Fields {
			if i > 0 {
				p.write(", ")
			}
			if isShorthand(k, v) {
				p.write(k)
			} else {
				p.write(k)
				p.write(": ")
				v.Accept(p)
			}
			i++
		}
		p.write(" }")
	}
}

func (p *CodePrinter) VisitMapLiteral(n *ast.MapLiteral) {
	if len(n.Pairs) > 3 {
		// Multiline with key alignment
		maxKeyLen := 0
		keyStrings := make([]string, len(n.Pairs))
		for i, pair := range n.Pairs {
			temp := &CodePrinter{indent: 0, lineWidth: 0}
			pair.Key.Accept(temp)
			keyStrings[i] = temp.String()
			if len(keyStrings[i]) > maxKeyLen {
				maxKeyLen = len(keyStrings[i])
			}
		}

		p.write("%{\n")
		p.indent++
		for i, pair := range n.Pairs {
			p.writeIndent()
			p.write(keyStrings[i])
			// Align arrows
			for j := len(keyStrings[i]); j < maxKeyLen; j++ {
				p.write(" ")
			}
			p.write(" => ")
			pair.Value.Accept(p)
			if i < len(n.Pairs)-1 {
				p.write(",")
			}
			p.writeln()
		}
		p.indent--
		p.writeIndent()
		p.write("}")
	} else {
		p.write("%{ ")
		for i, pair := range n.Pairs {
			if i > 0 {
				p.write(", ")
			}
			pair.Key.Accept(p)
			p.write(" => ")
			pair.Value.Accept(p)
		}
		p.write(" }")
	}
}

func (p *CodePrinter) VisitRecordType(n *ast.RecordType) {
	p.write("{")
	i := 0
	for k, v := range n.Fields {
		if i > 0 {
			p.write(", ")
		}
		p.write(k)
		p.write(": ")
		v.Accept(p)
		i++
	}
	p.write("}")
}

func (p *CodePrinter) VisitUnionType(n *ast.UnionType) {
	for i, t := range n.Types {
		if i > 0 {
			p.write(" | ")
		}
		t.Accept(p)
	}
}

func (p *CodePrinter) VisitMemberExpression(n *ast.MemberExpression) {
	n.Left.Accept(p)
	p.write(".")
	p.write(n.Member.Value)
}

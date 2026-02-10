package prettyprinter

import (
	"bytes"
	"github.com/funvibe/funxy/internal/ast"
	"sort"
	"strconv"
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
	"|":   5, // Bitwise OR (used for list comp separator precedence)
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
	if !ok || infix == nil || infix.Operator != "|>" {
		return 0
	}
	return 1 + countPipeSteps(infix.Left)
}

// printExpr prints an expression, adding parentheses only if needed
func (p *CodePrinter) printExpr(expr ast.Expression, parentPrec int, isRight bool) {
	if expr == nil {
		p.write("<???>")
		return
	}
	switch e := expr.(type) {
	case *ast.InfixExpression:
		if e == nil {
			p.write("<???>")
			return
		}
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
		if e == nil {
			p.write("<???>")
			return
		}
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
		if current == nil {
			break
		}
		infix, ok := current.(*ast.InfixExpression)
		if !ok || infix == nil || infix.Operator != "|>" {
			// This is the leftmost (source) expression
			steps = append(steps, current)
			break
		}
		// Prepend the right side (the function being piped to)
		if infix.Right != nil {
			steps = append(steps, infix.Right)
		} else {
			steps = append(steps, nil)
		}
		current = infix.Left
	}

	if len(steps) == 0 {
		p.write("<?>")
		return
	}

	// Reverse to get [source, step1, step2, ...]
	for i, j := 0, len(steps)-1; i < j; i, j = i+1, j-1 {
		steps[i], steps[j] = steps[j], steps[i]
	}

	// Print first step (source)
	if steps[0] != nil {
		steps[0].Accept(p)
	} else {
		p.write("<?>")
	}

	// Print remaining steps on new lines
	p.indent++
	for i := 1; i < len(steps); i++ {
		p.writeln()
		p.writeIndent()
		p.write("|> ")
		if steps[i] != nil {
			steps[i].Accept(p)
		} else {
			p.write("<?>")
		}
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
	if n.Name != nil {
		p.write(n.Name.Value)
	} else {
		p.write("<???>")
	}
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
		if stmt != nil {
			stmt.Accept(p)
		} else {
			p.write("<???>")
		}
		p.write("\n")
	}
}

func (p *CodePrinter) VisitExpressionStatement(n *ast.ExpressionStatement) {
	if n.Expression != nil {
		n.Expression.Accept(p)
	} else {
		p.write("<???>")
	}
}

func (p *CodePrinter) VisitFunctionStatement(n *ast.FunctionStatement) {
	p.write("fun ")
	if n.Name != nil {
		p.write(n.Name.Value)
	} else {
		p.write("<???>")
	}

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
			if param != nil && param.Name != nil {
				if len(param.Name.Value) > maxNameLen {
					maxNameLen = len(param.Name.Value)
				}
			}
		}

		p.write("(\n")
		p.indent++
		for i, param := range n.Parameters {
			p.writeIndent()
			if param != nil {
				if param.Name != nil {
					p.write(param.Name.Value)
					// Align colons
					for j := len(param.Name.Value); j < maxNameLen; j++ {
						p.write(" ")
					}
				} else {
					p.write("<???>")
				}
				p.write(": ")
				if param.IsVariadic {
					p.write("...")
				}
				if param.Type != nil {
					param.Type.Accept(p)
				}
			} else {
				p.write("<???>")
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
			if param != nil {
				if param.Name != nil {
					p.write(param.Name.Value)
				} else {
					p.write("<???>")
				}
				p.write(": ")
				if param.IsVariadic {
					p.write("...")
				}
				if param.Type != nil {
					param.Type.Accept(p)
				}
			} else {
				p.write("<???>")
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
	if n == nil {
		p.write("nil")
		return
	}
	p.write("trait ")
	if n.Name != nil {
		p.write(n.Name.Value)
	} else {
		p.write("<???>")
	}
	if len(n.TypeParams) > 0 {
		p.write("<")
		for i, tp := range n.TypeParams {
			if i > 0 {
				p.write(", ")
			}
			if tp != nil {
				p.write(tp.Value)
			} else {
				p.write("<???>")
			}
		}
		p.write(">")
	}
	p.write(" {\n")
	for _, method := range n.Signatures {
		if method != nil {
			method.Accept(p) // Prints the function signature
		} else {
			p.write("<???>")
		}
		p.write("\n")
	}
	p.write("}")
}

func (p *CodePrinter) VisitInstanceDeclaration(n *ast.InstanceDeclaration) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("instance ")
	if n.TraitName != nil {
		p.write(n.TraitName.Value)
	} else {
		p.write("<???>")
	}
	if len(n.Args) > 1 {
		p.write("<")
		for i, arg := range n.Args {
			if i > 0 {
				p.write(", ")
			}
			if arg != nil {
				arg.Accept(p)
			} else {
				p.write("<???>")
			}
		}
		p.write(">")
	} else if len(n.Args) == 1 {
		p.write(" ")
		if n.Args[0] != nil {
			n.Args[0].Accept(p)
		} else {
			p.write("<???>")
		}
	}
	p.write(" {\n")
	for _, method := range n.Methods {
		if method != nil {
			method.Accept(p)
		} else {
			p.write("<???>")
		}
		p.write("\n")
	}
	p.write("}")
}

func (p *CodePrinter) VisitConstantDeclaration(n *ast.ConstantDeclaration) {
	if n == nil {
		p.write("nil")
		return
	}
	if n.Name != nil {
		n.Name.Accept(p)
	} else if n.Pattern != nil {
		n.Pattern.Accept(p)
	} else {
		p.write("<???>")
	}
	if n.TypeAnnotation != nil {
		p.write(": ")
		n.TypeAnnotation.Accept(p)
	}
	p.write(" :- ")
	if n.Value != nil {
		n.Value.Accept(p)
	} else {
		p.write("<???>")
	}
}

func (p *CodePrinter) VisitFunctionLiteral(n *ast.FunctionLiteral) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("fun(")
	for i, param := range n.Parameters {
		if i > 0 {
			p.write(", ")
		}
		if param != nil {
			if param.Name != nil {
				p.write(param.Name.Value)
			} else {
				p.write("<???>")
			}
			if param.Type != nil {
				p.write(": ")
				param.Type.Accept(p)
			}
		} else {
			p.write("<???>")
		}
	}
	p.write(")")
	if n.ReturnType != nil {
		p.write(" -> ")
		n.ReturnType.Accept(p)
	}
	p.write(" ")
	if n.Body != nil {
		n.Body.Accept(p)
	} else {
		p.write("<???>")
	}
}

func (p *CodePrinter) VisitIdentifier(n *ast.Identifier) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write(n.Value)
}

func (p *CodePrinter) VisitIntegerLiteral(n *ast.IntegerLiteral) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write(n.Token.Lexeme)
}

func (p *CodePrinter) VisitFloatLiteral(n *ast.FloatLiteral) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write(n.Token.Lexeme)
}

func (p *CodePrinter) VisitBigIntLiteral(n *ast.BigIntLiteral) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write(n.Token.Lexeme)
}

func (p *CodePrinter) VisitRationalLiteral(n *ast.RationalLiteral) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write(n.Token.Lexeme)
}

func (p *CodePrinter) VisitBooleanLiteral(n *ast.BooleanLiteral) {
	if n == nil {
		p.write("nil")
		return
	}
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
			if el != nil {
				el.Accept(p)
			} else {
				p.write("<???>")
			}
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
			if el != nil {
				el.Accept(p)
			} else {
				p.write("<???>")
			}
		}
		p.write(")")
	}
}

func (p *CodePrinter) VisitListLiteral(n *ast.ListLiteral) {
	if n == nil {
		p.write("nil")
		return
	}
	if len(n.Elements) > 5 {
		// Multiline for large lists
		p.write("[\n")
		p.indent++
		for i, el := range n.Elements {
			p.writeIndent()
			if i == 0 {
				// First element needs to avoid ambiguity with list comprehension |
				p.printExpr(el, getPrecedence("|"), false)
			} else {
				if el != nil {
					el.Accept(p)
				} else {
					p.write("<???>")
				}
			}
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
			if i == 0 {
				// First element needs to avoid ambiguity with list comprehension |
				p.printExpr(el, getPrecedence("|"), false)
			} else {
				if el != nil {
					el.Accept(p)
				} else {
					p.write("<???>")
				}
			}
		}
		p.write("]")
	}
}

func (p *CodePrinter) VisitIndexExpression(n *ast.IndexExpression) {
	if n.Left != nil {
		n.Left.Accept(p)
	} else {
		p.write("<???>")
	}
	p.write("[")
	if n.Index != nil {
		n.Index.Accept(p)
	} else {
		p.write("<???>")
	}
	p.write("]")
}

func (p *CodePrinter) VisitStringLiteral(n *ast.StringLiteral) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write(strconv.Quote(n.Value))
}

func (p *CodePrinter) VisitFormatStringLiteral(n *ast.FormatStringLiteral) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("%\"" + n.Value + "\"")
}

func (p *CodePrinter) VisitInterpolatedString(n *ast.InterpolatedString) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("\"")
	for _, part := range n.Parts {
		if sl, ok := part.(*ast.StringLiteral); ok {
			// Escape the string content but remove the surrounding quotes added by Quote
			quoted := strconv.Quote(sl.Value)
			p.write(quoted[1 : len(quoted)-1])
		} else {
			p.write("${")
			part.Accept(p)
			p.write("}")
		}
	}
	p.write("\"")
}

func (p *CodePrinter) VisitCharLiteral(n *ast.CharLiteral) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write(strconv.QuoteRune(rune(n.Value)))
}

func (p *CodePrinter) VisitBytesLiteral(n *ast.BytesLiteral) {
	if n == nil {
		p.write("nil")
		return
	}
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
	if n == nil {
		p.write("nil")
		return
	}
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
		if t != nil {
			t.Accept(p)
		} else {
			p.write("<???>")
		}
	}
	p.write(")")
}

func (p *CodePrinter) VisitFunctionType(n *ast.FunctionType) {
	p.write("(")
	for i, t := range n.Parameters {
		if i > 0 {
			p.write(", ")
		}
		if t != nil {
			t.Accept(p)
		} else {
			p.write("<???>")
		}
	}
	p.write(") -> ")
	if n.ReturnType != nil {
		n.ReturnType.Accept(p)
	} else {
		p.write("<???>")
	}
}

func (p *CodePrinter) VisitPrefixExpression(n *ast.PrefixExpression) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write(n.Operator)
	p.printExpr(n.Right, 100, false)
}

func (p *CodePrinter) VisitInfixExpression(n *ast.InfixExpression) {
	if n == nil {
		p.write("nil")
		return
	}
	// When called directly (not via printExpr), use lowest precedence context
	p.printExpr(n, 0, false)
}

func (p *CodePrinter) VisitOperatorAsFunction(n *ast.OperatorAsFunction) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("(" + n.Operator + ")")
}

func (p *CodePrinter) VisitPostfixExpression(n *ast.PostfixExpression) {
	if n == nil {
		p.write("nil")
		return
	}
	if n.Left != nil {
		n.Left.Accept(p)
	} else {
		p.write("<???>")
	}
	p.write(n.Operator)
}

func (p *CodePrinter) VisitAssignExpression(n *ast.AssignExpression) {
	if n == nil {
		p.write("nil")
		return
	}
	if n.Left != nil {
		n.Left.Accept(p)
	} else {
		p.write("<???>")
	}
	if n.AnnotatedType != nil {
		p.write(" : ")
		n.AnnotatedType.Accept(p)
	}
	p.write(" = ")
	if n.Value != nil {
		n.Value.Accept(p)
	} else {
		p.write("<???>")
	}
}

func (p *CodePrinter) VisitPatternAssignExpression(n *ast.PatternAssignExpression) {
	if n == nil {
		p.write("nil")
		return
	}
	if n.Pattern != nil {
		n.Pattern.Accept(p)
	} else {
		p.write("<???>")
	}
	p.write(" = ")
	if n.Value != nil {
		n.Value.Accept(p)
	} else {
		p.write("<???>")
	}
}

func (p *CodePrinter) VisitAnnotatedExpression(n *ast.AnnotatedExpression) {
	if n == nil {
		p.write("nil")
		return
	}
	if n.Expression != nil {
		n.Expression.Accept(p)
	} else {
		p.write("<???>")
	}
	p.write(": ")
	if n.TypeAnnotation != nil {
		n.TypeAnnotation.Accept(p)
	} else {
		p.write("<???>")
	}
}

func (p *CodePrinter) VisitCallExpression(n *ast.CallExpression) {
	if n == nil {
		p.write("nil")
		return
	}
	if n.Function != nil {
		n.Function.Accept(p)
	} else {
		p.write("<???>")
	}
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
		if arg != nil {
			arg.Accept(p)
		} else {
			p.write("<???>")
		}
	}
	p.write(")")
}

func (p *CodePrinter) VisitTypeApplicationExpression(n *ast.TypeApplicationExpression) {
	if n == nil {
		p.write("nil")
		return
	}
	if n.Expression != nil {
		n.Expression.Accept(p)
	} else {
		p.write("<???>")
	}
	p.write("<")
	for i, t := range n.TypeArguments {
		if i > 0 {
			p.write(", ")
		}
		if t != nil {
			t.Accept(p)
		} else {
			p.write("<???>")
		}
	}
	p.write(">")
}

func (p *CodePrinter) VisitBlockStatement(n *ast.BlockStatement) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("{\n")
	p.indent++
	for _, stmt := range n.Statements {
		p.writeIndent()
		if stmt != nil {
			stmt.Accept(p)
		} else {
			p.write("<???>")
		}
		p.write("\n")
	}
	p.indent--
	p.writeIndent()
	p.write("}")
}

func (p *CodePrinter) VisitIfExpression(n *ast.IfExpression) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("if ")
	if n.Condition != nil {
		n.Condition.Accept(p)
	} else {
		p.write("<???>")
	}
	p.write(" ")
	if n.Consequence != nil {
		n.Consequence.Accept(p)
	} else {
		p.write("<???>")
	}
	if n.Alternative != nil {
		p.write(" else ")
		n.Alternative.Accept(p)
	}
}

func (p *CodePrinter) VisitForExpression(n *ast.ForExpression) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("for ")
	if n.Iterable != nil {
		// for item in iterable
		if n.ItemName != nil {
			p.write(n.ItemName.Value)
		} else {
			p.write("<???>")
		}
		p.write(" in ")
		n.Iterable.Accept(p)
	} else {
		// for condition
		if n.Condition != nil {
			n.Condition.Accept(p)
		} else {
			p.write("<???>")
		}
	}
	p.write(" ")
	if n.Body != nil {
		n.Body.Accept(p)
	} else {
		p.write("<???>")
	}
}

func (p *CodePrinter) VisitBreakStatement(n *ast.BreakStatement) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("break")
	if n.Value != nil {
		p.write(" ")
		n.Value.Accept(p)
	}
}

func (p *CodePrinter) VisitContinueStatement(n *ast.ContinueStatement) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("continue")
}

func (p *CodePrinter) VisitReturnStatement(n *ast.ReturnStatement) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("return")
	if n.Value != nil {
		p.write(" ")
		n.Value.Accept(p)
	}
}

func (p *CodePrinter) VisitTypeDeclarationStatement(n *ast.TypeDeclarationStatement) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("type ")
	if n.IsAlias {
		p.write("alias ")
	}
	if n.Name != nil {
		n.Name.Accept(p)
	} else {
		p.write("<???>")
	}

	if len(n.TypeParameters) > 0 {
		p.write("<")
		for i, param := range n.TypeParameters {
			if i > 0 {
				p.write(", ")
			}
			if param != nil {
				param.Accept(p)
			} else {
				p.write("<???>")
			}
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
		if c != nil {
			c.Accept(p)
		} else {
			p.write("<???>")
		}
	}
}

func (p *CodePrinter) VisitNamedType(n *ast.NamedType) {
	n.Name.Accept(p)
	if len(n.Args) > 0 {
		p.write("<")
		for i, arg := range n.Args {
			if i > 0 {
				p.write(", ")
			}
			arg.Accept(p)
		}
		// Avoid >> token ambiguity
		if strings.HasSuffix(p.buf.String(), ">") {
			p.write(" ")
		}
		p.write(">")
	}
}

func (p *CodePrinter) VisitDataConstructor(n *ast.DataConstructor) {
	if n == nil {
		p.write("nil")
		return
	}
	if n.Name != nil {
		n.Name.Accept(p)
	} else {
		p.write("<???>")
	}
	if len(n.Parameters) > 0 {
		p.write("(")
		for i, param := range n.Parameters {
			if i > 0 {
				p.write(", ")
			}
			if param != nil {
				param.Accept(p)
			} else {
				p.write("<???>")
			}
		}
		p.write(")")
	}
}

func (p *CodePrinter) VisitMatchExpression(n *ast.MatchExpression) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("match ")
	if n.Expression != nil {
		n.Expression.Accept(p)
	} else {
		p.write("<???>")
	}
	p.write(" {\n")
	p.indent++

	// Calculate max pattern width for alignment
	maxPatLen := 0
	patStrings := make([]string, len(n.Arms))
	for i, arm := range n.Arms {
		// Print pattern to temp buffer to get length
		temp := &CodePrinter{indent: 0, lineWidth: 0}
		if arm.Pattern != nil {
			arm.Pattern.Accept(temp)
		} else {
			temp.write("<???>")
		}
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
		if arm.Expression != nil {
			arm.Expression.Accept(p)
		} else {
			p.write("<???>")
		}
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
	if n == nil {
		p.write("nil")
		return
	}
	if n.Name != nil {
		n.Name.Accept(p)
	} else {
		p.write("<???>")
	}
	if len(n.Elements) > 0 {
		p.write("(")
		for i, el := range n.Elements {
			if i > 0 {
				p.write(", ")
			}
			if el != nil {
				el.Accept(p)
			} else {
				p.write("<???>")
			}
		}
		p.write(")")
	}
}

func (p *CodePrinter) VisitListPattern(n *ast.ListPattern) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("[")
	for i, el := range n.Elements {
		if i > 0 {
			p.write(", ")
		}
		if el != nil {
			el.Accept(p)
		} else {
			p.write("<???>")
		}
	}
	p.write("]")
}

func (p *CodePrinter) VisitTuplePattern(n *ast.TuplePattern) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("(")
	for i, el := range n.Elements {
		if i > 0 {
			p.write(", ")
		}
		if el != nil {
			el.Accept(p)
		} else {
			p.write("<???>")
		}
	}
	p.write(")")
}

func (p *CodePrinter) VisitRecordPattern(n *ast.RecordPattern) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("{")
	keys := make([]string, 0, len(n.Fields))
	for k := range n.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, k := range keys {
		if i > 0 {
			p.write(", ")
		}
		p.write(k)
		p.write(": ")
		if n.Fields[k] != nil {
			n.Fields[k].Accept(p)
		} else {
			p.write("<???>")
		}
	}
	p.write("}")
}

func (p *CodePrinter) VisitTypePattern(n *ast.TypePattern) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write(n.Name)
	p.write(": ")
	if n.Type != nil {
		n.Type.Accept(p)
	} else {
		p.write("<???>")
	}
}

func (p *CodePrinter) VisitStringPattern(n *ast.StringPattern) {
	if n == nil {
		p.write("nil")
		return
	}
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
	if n == nil {
		p.write("nil")
		return
	}
	p.write("^")
	p.write(n.Name)
}

func (p *CodePrinter) VisitSpreadExpression(n *ast.SpreadExpression) {
	if n == nil {
		p.write("nil")
		return
	}
	if n.Expression != nil {
		n.Expression.Accept(p)
	} else {
		p.write("<???>")
	}
	p.write("...")
}

func (p *CodePrinter) VisitSpreadPattern(n *ast.SpreadPattern) {
	if n == nil {
		p.write("nil")
		return
	}
	if n.Pattern != nil {
		n.Pattern.Accept(p)
	} else {
		p.write("<???>")
	}
	p.write("...")
}

func (p *CodePrinter) VisitRecordLiteral(n *ast.RecordLiteral) {
	if n == nil {
		p.write("nil")
		return
	}
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
		sort.Strings(keys)

		p.write("{\n")
		p.indent++

		// Handle spread expression first if present
		if n.Spread != nil {
			p.writeIndent()
			p.write("...")
			n.Spread.Accept(p)
			p.write(",\n")
		}

		for i, k := range keys {
			p.writeIndent()
			p.write(k)
			// Align colons
			for j := len(k); j < maxKeyLen; j++ {
				p.write(" ")
			}
			p.write(": ")
			if n.Fields[k] != nil {
				n.Fields[k].Accept(p)
			} else {
				p.write("<???>")
			}
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

		// Handle spread expression first if present
		if n.Spread != nil {
			p.write("...")
			n.Spread.Accept(p)
			p.write(", ")
		}

		keys := make([]string, 0, len(n.Fields))
		for k := range n.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)

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

		// Handle spread expression first if present
		if n.Spread != nil {
			p.write("...")
			n.Spread.Accept(p)
			if len(n.Fields) > 0 {
				p.write(", ")
			}
		}

		keys := make([]string, 0, len(n.Fields))
		for k := range n.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for i, k := range keys {
			v := n.Fields[k]
			if i > 0 {
				p.write(", ")
			}
			if isShorthand(k, v) {
				p.write(k)
			} else {
				p.write(k)
				p.write(": ")
				if v != nil {
					v.Accept(p)
				} else {
					p.write("<???>")
				}
			}
		}
		p.write(" }")
	}
}

func (p *CodePrinter) VisitMapLiteral(n *ast.MapLiteral) {
	if n == nil {
		p.write("nil")
		return
	}
	if len(n.Pairs) > 3 {
		// Multiline with key alignment
		maxKeyLen := 0
		keyStrings := make([]string, len(n.Pairs))
		for i, pair := range n.Pairs {
			temp := &CodePrinter{indent: 0, lineWidth: 0}
			if pair.Key != nil {
				pair.Key.Accept(temp)
			}
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
			if pair.Value != nil {
				pair.Value.Accept(p)
			} else {
				p.write("<???>")
			}
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
			if pair.Key != nil {
				pair.Key.Accept(p)
			} else {
				p.write("<???>")
			}
			p.write(" => ")
			if pair.Value != nil {
				pair.Value.Accept(p)
			} else {
				p.write("<???>")
			}
		}
		p.write(" }")
	}
}

func (p *CodePrinter) VisitRecordType(n *ast.RecordType) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("{")
	keys := make([]string, 0, len(n.Fields))
	for k := range n.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, k := range keys {
		if i > 0 {
			p.write(", ")
		}
		p.write(k)
		p.write(": ")
		if n.Fields[k] != nil {
			n.Fields[k].Accept(p)
		} else {
			p.write("<???>")
		}
	}
	p.write("}")
}

func (p *CodePrinter) VisitUnionType(n *ast.UnionType) {
	if n == nil {
		p.write("nil")
		return
	}
	for i, t := range n.Types {
		if i > 0 {
			p.write(" | ")
		}
		if t != nil {
			t.Accept(p)
		} else {
			p.write("<???>")
		}
	}
}

func (p *CodePrinter) VisitForallType(n *ast.ForallType) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("forall")
	for _, param := range n.Vars {
		p.write(" ")
		if param != nil {
			param.Accept(p)
		} else {
			p.write("<???>")
		}
	}
	p.write(". ")
	if n.Type != nil {
		n.Type.Accept(p)
	} else {
		p.write("<???>")
	}
}

func (p *CodePrinter) VisitMemberExpression(n *ast.MemberExpression) {
	if n == nil {
		p.write("nil")
		return
	}
	if n.Left != nil {
		n.Left.Accept(p)
	} else {
		p.write("<???>")
	}
	p.write(".")
	if n.Member != nil {
		p.write(n.Member.Value)
	} else {
		p.write("<???>")
	}
}

func (p *CodePrinter) VisitListComprehension(n *ast.ListComprehension) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("[")
	// Use precedence of | to ensure output expression is parenthesized if needed
	// e.g. [(a && b) | ...] because && has lower precedence than |
	if n.Output != nil {
		p.printExpr(n.Output, getPrecedence("|"), false)
	} else {
		p.write("<???>")
	}
	p.write(" | ")
	for i, clause := range n.Clauses {
		if i > 0 {
			p.write(", ")
		}
		switch c := clause.(type) {
		case *ast.CompGenerator:
			if c == nil {
				p.write("<???>")
				continue
			}
			if c.Pattern != nil {
				c.Pattern.Accept(p)
			} else {
				p.write("<???>")
			}
			p.write(" <- ")
			if c.Iterable != nil {
				c.Iterable.Accept(p)
			} else {
				p.write("<???>")
			}
		case *ast.CompFilter:
			if c == nil {
				p.write("<???>")
				continue
			}
			if c.Condition != nil {
				c.Condition.Accept(p)
			} else {
				p.write("<???>")
			}
		}
	}
	p.write("]")
}

func (p *CodePrinter) VisitRangeExpression(n *ast.RangeExpression) {
	if n == nil {
		p.write("nil")
		return
	}
	if n.Next != nil {
		p.write("(")
		if n.Start != nil {
			n.Start.Accept(p)
		} else {
			p.write("<???>")
		}
		p.write(", ")
		n.Next.Accept(p)
		p.write(")")
	} else {
		if n.Start != nil {
			n.Start.Accept(p)
		} else {
			p.write("<???>")
		}
	}
	p.write("..")
	if n.End != nil {
		n.End.Accept(p)
	} else {
		p.write("<???>")
	}
}

func (p *CodePrinter) VisitDirectiveStatement(n *ast.DirectiveStatement) {
	if n == nil {
		p.write("nil")
		return
	}
	p.write("directive \"")
	p.write(n.Name)
	p.write("\"\n")
}

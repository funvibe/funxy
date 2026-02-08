package prettyprinter

import (
	"bytes"
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"strings"
)

// --- Tree Printer (Output looks like a tree structure) ---

type TreePrinter struct {
	buf    bytes.Buffer
	indent int
}

func NewTreePrinter() *TreePrinter {
	return &TreePrinter{}
}

func (p *TreePrinter) String() string {
	return p.buf.String()
}

func (p *TreePrinter) write(s string) {
	p.buf.WriteString(s)
}

func (p *TreePrinter) writeIndent() {
	p.write(strings.Repeat("  ", p.indent))
}

func (p *TreePrinter) VisitPackageDeclaration(n *ast.PackageDeclaration) {
	p.writeIndent()
	p.write("Package: " + n.Name.Value + "\n")
	if n.ExportAll {
		p.writeIndent()
		p.write("  Exports: *\n")
	} else if len(n.Exports) > 0 {
		p.writeIndent()
		p.write("  Exports: ")
		if n.ExportAll {
			p.write("*")
			if len(n.Exports) > 0 {
				p.write(", ")
			}
		}
		for i, ex := range n.Exports {
			if i > 0 {
				p.write(", ")
			}
			if ex.IsReexport() {
				p.write(ex.ModuleName.Value + "(")
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
		p.write("\n")
	}
}

func (p *TreePrinter) VisitImportStatement(n *ast.ImportStatement) {
	p.writeIndent()
	p.write("Import: " + n.Path.Value)
	if n.Alias != nil {
		p.write(" as " + n.Alias.Value)
	}
	p.write("\n")
}

func (p *TreePrinter) VisitProgram(n *ast.Program) {
	p.write("Program\n")
	p.indent++
	for _, stmt := range n.Statements {
		stmt.Accept(p)
	}
	p.indent--
}

func (p *TreePrinter) VisitExpressionStatement(n *ast.ExpressionStatement) {
	// We don't print "ExpressionStatement" wrapper to keep it cleaner, or we can.
	// Let's print the expression directly but maybe indent?
	n.Expression.Accept(p)
	p.write("\n")
}

func (p *TreePrinter) VisitFunctionStatement(n *ast.FunctionStatement) {
	p.writeIndent()
	p.write("FunctionStatement\n")
	p.indent++
	p.writeIndent()
	p.write("Name: " + n.Name.Value + "\n")

	if len(n.TypeParams) > 0 {
		p.writeIndent()
		p.write("Generics: ")
		for i, tp := range n.TypeParams {
			if i > 0 {
				p.write(", ")
			}
			p.write(tp.Value)
			for _, c := range n.Constraints {
				if c.TypeVar == tp.Value {
					p.write(":" + c.Trait)
				}
			}
		}
		p.write("\n")
	}

	p.writeIndent()
	p.write("Params:\n")
	p.indent++
	for _, param := range n.Parameters {
		p.writeIndent()
		p.write(param.Name.Value + ": ")
		if param.IsVariadic {
			p.write("...")
		}
		if param.Type != nil {
			param.Type.Accept(p)
		}
		p.write("\n")
	}
	p.indent--
	if n.ReturnType != nil {
		p.writeIndent()
		p.write("Return: ")
		n.ReturnType.Accept(p)
		p.write("\n")
	}
	p.writeIndent()
	p.write("Body:\n")
	p.indent++
	if n.Body != nil {
		n.Body.Accept(p)
	}
	p.indent--
	p.indent--
	p.write("\n")
}

func (p *TreePrinter) VisitTraitDeclaration(n *ast.TraitDeclaration) {
	p.writeIndent()
	p.write("Trait: ")
	p.write(n.Name.Value)
	for _, tp := range n.TypeParams {
		p.write(" " + tp.Value)
	}
	p.write("\n")
	p.indent++
	for _, m := range n.Signatures {
		m.Accept(p)
	}
	p.indent--
}

func (p *TreePrinter) VisitInstanceDeclaration(n *ast.InstanceDeclaration) {
	p.writeIndent()
	p.write("Instance: ")
	p.write(n.TraitName.Value)
	if len(n.Args) > 0 {
		for _, arg := range n.Args {
			p.write(" ")
			arg.Accept(p)
		}
	}
	p.write("\n")
	p.indent++
	for _, m := range n.Methods {
		m.Accept(p)
	}
	p.indent--
}

func (p *TreePrinter) VisitConstantDeclaration(n *ast.ConstantDeclaration) {
	p.writeIndent()
	p.write("ConstantDeclaration\n")
	p.indent++
	p.writeIndent()
	p.write("Name: ")
	n.Name.Accept(p)
	p.write("\n")
	if n.TypeAnnotation != nil {
		p.writeIndent()
		p.write("Type: ")
		n.TypeAnnotation.Accept(p)
		p.write("\n")
	}
	p.writeIndent()
	p.write("Value: ")
	n.Value.Accept(p)
	p.write("\n")
	p.indent--
}

func (p *TreePrinter) VisitFunctionLiteral(n *ast.FunctionLiteral) {
	p.write("FunctionLiteral\n")
	p.indent++
	p.writeIndent()
	p.write("Params:\n")
	p.indent++
	for _, param := range n.Parameters {
		p.writeIndent()
		p.write(param.Name.Value + ": ")
		if param.Type != nil {
			param.Type.Accept(p)
		}
		p.write("\n")
	}
	p.indent--
	if n.ReturnType != nil {
		p.writeIndent()
		p.write("Return: ")
		n.ReturnType.Accept(p)
		p.write("\n")
	}
	p.writeIndent()
	p.write("Body:\n")
	p.indent++
	n.Body.Accept(p)
	p.indent--
	p.indent--
}

func (p *TreePrinter) VisitIdentifier(n *ast.Identifier) {
	p.write("Identifier(")
	p.write(n.Value)
	p.write(")")
}

func (p *TreePrinter) VisitIntegerLiteral(n *ast.IntegerLiteral) {
	p.write("IntegerLiteral(")
	p.write(n.Token.Lexeme)
	p.write(")")
}

func (p *TreePrinter) VisitFloatLiteral(n *ast.FloatLiteral) {
	p.write("FloatLiteral(")
	p.write(n.Token.Lexeme)
	p.write(")")
}

func (p *TreePrinter) VisitBigIntLiteral(n *ast.BigIntLiteral) {
	p.write("BigIntLiteral(")
	p.write(n.Token.Lexeme)
	p.write(")")
}

func (p *TreePrinter) VisitRationalLiteral(n *ast.RationalLiteral) {
	p.write("RationalLiteral(")
	p.write(n.Token.Lexeme)
	p.write(")")
}

func (p *TreePrinter) VisitTupleLiteral(n *ast.TupleLiteral) {
	p.write("Tuple\n")
	p.indent++
	for _, el := range n.Elements {
		p.writeIndent()
		el.Accept(p)
		p.write("\n")
	}
	p.indent--
}

func (p *TreePrinter) VisitListLiteral(n *ast.ListLiteral) {
	p.write("List\n")
	p.indent++
	for _, el := range n.Elements {
		p.writeIndent()
		el.Accept(p)
		p.write("\n")
	}
	p.indent--
}

func (p *TreePrinter) VisitIndexExpression(n *ast.IndexExpression) {
	p.write("Index\n")
	p.indent++
	p.writeIndent()
	p.write("Target: ")
	n.Left.Accept(p)
	p.write("\n")
	p.writeIndent()
	p.write("Idx: ")
	n.Index.Accept(p)
	p.indent--
}

func (p *TreePrinter) VisitStringLiteral(n *ast.StringLiteral) {
	p.write("StringLiteral(\"" + n.Value + "\")")
}

func (p *TreePrinter) VisitFormatStringLiteral(n *ast.FormatStringLiteral) {
	p.write("FormatStringLiteral(%\"" + n.Value + "\")")
}

func (p *TreePrinter) VisitInterpolatedString(n *ast.InterpolatedString) {
	p.write("InterpolatedString(")
	for i, part := range n.Parts {
		if i > 0 {
			p.write(", ")
		}
		part.Accept(p)
	}
	p.write(")")
}

func (p *TreePrinter) VisitCharLiteral(n *ast.CharLiteral) {
	p.write("CharLiteral('" + string(rune(n.Value)) + "')")
}

func (p *TreePrinter) VisitBytesLiteral(n *ast.BytesLiteral) {
	p.write("BytesLiteral(" + n.Kind + ": \"" + n.Content + "\")")
}

func (p *TreePrinter) VisitBitsLiteral(n *ast.BitsLiteral) {
	p.write("BitsLiteral(" + n.Kind + ": \"" + n.Content + "\")")
}

func (p *TreePrinter) VisitTupleType(n *ast.TupleType) {
	p.write("TupleType(")
	for i, t := range n.Types {
		if i > 0 {
			p.write(", ")
		}
		t.Accept(p)
	}
	p.write(")")
}

func (p *TreePrinter) VisitFunctionType(n *ast.FunctionType) {
	p.write("FunctionType\n")
	p.indent++
	p.writeIndent()
	p.write("Params:\n")
	p.indent++
	for _, t := range n.Parameters {
		p.writeIndent()
		t.Accept(p)
		p.write("\n")
	}
	p.indent--
	p.writeIndent()
	p.write("Return: ")
	n.ReturnType.Accept(p)
	p.indent--
}

func (p *TreePrinter) VisitBooleanLiteral(n *ast.BooleanLiteral) {
	p.write("BooleanLiteral(")
	p.write(n.Token.Lexeme)
	p.write(")")
}

func (p *TreePrinter) VisitNilLiteral(n *ast.NilLiteral) {
	p.write("NilLiteral")
}

func (p *TreePrinter) VisitPrefixExpression(n *ast.PrefixExpression) {
	p.write("Prefix(")
	p.write(n.Operator)
	p.write(")\n")
	p.indent++
	p.writeIndent()
	n.Right.Accept(p)
	p.indent--
}

func (p *TreePrinter) VisitInfixExpression(n *ast.InfixExpression) {
	p.write("Infix(")
	p.write(n.Operator)
	p.write(")\n")
	p.indent++
	p.writeIndent()
	p.write("Left: ")
	n.Left.Accept(p)
	p.write("\n")
	p.writeIndent()
	p.write("Right: ")
	n.Right.Accept(p)
	p.indent--
}

func (p *TreePrinter) VisitOperatorAsFunction(n *ast.OperatorAsFunction) {
	p.write("OperatorAsFunction(")
	p.write(n.Operator)
	p.write(")")
}

func (p *TreePrinter) VisitPostfixExpression(n *ast.PostfixExpression) {
	p.write("Postfix(")
	p.write(n.Operator)
	p.write(")\n")
	p.indent++
	p.writeIndent()
	p.write("Left: ")
	n.Left.Accept(p)
	p.indent--
}

func (p *TreePrinter) VisitAssignExpression(n *ast.AssignExpression) {
	p.writeIndent()
	p.write("Assign\n")
	p.indent++
	p.writeIndent()
	p.write("Left: ")
	n.Left.Accept(p)
	p.write("\n")
	p.writeIndent()
	p.write("Value: ")
	n.Value.Accept(p)
	p.indent--
	p.write("\n")
}

func (p *TreePrinter) VisitPatternAssignExpression(n *ast.PatternAssignExpression) {
	p.writeIndent()
	p.write("PatternAssign\n")
	p.indent++
	p.writeIndent()
	p.write("Pattern: ")
	n.Pattern.Accept(p)
	p.write("\n")
	p.writeIndent()
	p.write("Value: ")
	n.Value.Accept(p)
	p.indent--
	p.write("\n")
}

func (p *TreePrinter) VisitAnnotatedExpression(n *ast.AnnotatedExpression) {
	p.write("Annotated(")
	n.Expression.Accept(p)
	p.write(": ")
	n.TypeAnnotation.Accept(p)
	p.write(")")
}

func (p *TreePrinter) VisitCallExpression(n *ast.CallExpression) {
	p.write("Call\n")
	p.indent++
	p.writeIndent()
	p.write("Function: ")
	n.Function.Accept(p)
	p.write("\n")
	if len(n.Arguments) > 0 {
		p.writeIndent()
		p.write("Arguments:\n")
		p.indent++
		for _, arg := range n.Arguments {
			p.writeIndent()
			arg.Accept(p)
			p.write("\n")
		}
		p.indent--
	}
	p.indent--
}

func (p *TreePrinter) VisitTypeApplicationExpression(n *ast.TypeApplicationExpression) {
	p.write("TypeApp\n")
	p.indent++
	p.writeIndent()
	p.write("Expr: ")
	n.Expression.Accept(p)
	p.write("\n")
	if len(n.TypeArguments) > 0 {
		p.writeIndent()
		p.write("TypeArgs:\n")
		p.indent++
		for _, t := range n.TypeArguments {
			p.writeIndent()
			t.Accept(p)
			p.write("\n")
		}
		p.indent--
	}
	p.indent--
}

func (p *TreePrinter) VisitBlockStatement(n *ast.BlockStatement) {
	p.write("Block\n")
	p.indent++
	for _, stmt := range n.Statements {
		p.writeIndent()
		stmt.Accept(p)
		p.write("\n")
	}
	p.indent--
}

func (p *TreePrinter) VisitIfExpression(n *ast.IfExpression) {
	p.write("If\n")
	p.indent++
	p.writeIndent()
	p.write("Cond: ")
	n.Condition.Accept(p)
	p.write("\n")
	p.writeIndent()
	p.write("Then: ")
	n.Consequence.Accept(p)
	if n.Alternative != nil {
		p.write("\n")
		p.writeIndent()
		p.write("Else: ")
		n.Alternative.Accept(p)
	}
	p.indent--
}

func (p *TreePrinter) VisitForExpression(n *ast.ForExpression) {
	p.write("For\n")
	p.indent++
	if n.Iterable != nil {
		p.writeIndent()
		p.write("IterItem: " + n.ItemName.Value + "\n")
		p.writeIndent()
		p.write("Iterable: ")
		n.Iterable.Accept(p)
		p.write("\n")
	} else {
		p.writeIndent()
		p.write("Cond: ")
		n.Condition.Accept(p)
		p.write("\n")
	}
	p.writeIndent()
	p.write("Body: ")
	n.Body.Accept(p)
	p.indent--
}

func (p *TreePrinter) VisitBreakStatement(n *ast.BreakStatement) {
	p.writeIndent()
	p.write("Break")
	if n.Value != nil {
		p.write(" ")
		n.Value.Accept(p)
	}
	p.write("\n")
}

func (p *TreePrinter) VisitContinueStatement(n *ast.ContinueStatement) {
	p.writeIndent()
	p.write("Continue\n")
}

func (p *TreePrinter) VisitReturnStatement(n *ast.ReturnStatement) {
	p.writeIndent()
	p.write("Return")
	if n.Value != nil {
		p.write(" ")
		n.Value.Accept(p)
	}
	p.write("\n")
}

func (p *TreePrinter) VisitTypeDeclarationStatement(n *ast.TypeDeclarationStatement) {
	p.writeIndent()
	p.write("TypeDeclaration")
	if n.IsAlias {
		p.write(" (Alias)")
	}
	p.write("\n")
	p.indent++
	p.writeIndent()
	p.write("Name: ")
	n.Name.Accept(p)
	p.write("\n")

	if len(n.TypeParameters) > 0 {
		p.writeIndent()
		p.write("Params: ")
		for i, param := range n.TypeParameters {
			if i > 0 {
				p.write(", ")
			}
			param.Accept(p)
		}
		p.write("\n")
	}

	if n.TargetType != nil {
		p.writeIndent()
		p.write("Target: ")
		n.TargetType.Accept(p)
		p.write("\n")
	}

	if len(n.Constructors) > 0 {
		p.writeIndent()
		p.write("Constructors:\n")
		p.indent++
		for _, c := range n.Constructors {
			c.Accept(p)
			p.write("\n")
		}
		p.indent--
	}

	p.indent--
	p.write("\n")
}

func (p *TreePrinter) VisitNamedType(n *ast.NamedType) {
	p.write("NamedType(")
	n.Name.Accept(p)
	for _, arg := range n.Args {
		p.write(" ")
		arg.Accept(p)
	}
	p.write(")")
}

func (p *TreePrinter) VisitDataConstructor(n *ast.DataConstructor) {
	p.writeIndent()
	p.write("Constructor: ")
	n.Name.Accept(p)
	if len(n.Parameters) > 0 {
		p.write(" (")
		for i, param := range n.Parameters {
			if i > 0 {
				p.write(", ")
			}
			param.Accept(p)
		}
		p.write(")")
	}
}

func (p *TreePrinter) VisitMatchExpression(n *ast.MatchExpression) {
	p.write("Match\n")
	p.indent++
	p.writeIndent()
	n.Expression.Accept(p)
	p.write("\n")
	for _, arm := range n.Arms {
		p.writeIndent()
		p.write("Arm: ")
		arm.Pattern.Accept(p)
		p.write(" -> ")
		arm.Expression.Accept(p)
		p.write("\n")
	}
	p.indent--
}

func (p *TreePrinter) VisitWildcardPattern(n *ast.WildcardPattern) {
	p.write("_")
}

func (p *TreePrinter) VisitLiteralPattern(n *ast.LiteralPattern) {
	p.write("Lit(")
	p.write(n.Token.Lexeme)
	p.write(")")
}

func (p *TreePrinter) VisitIdentifierPattern(n *ast.IdentifierPattern) {
	p.write("Var(")
	p.write(n.Value)
	p.write(")")
}

func (p *TreePrinter) VisitConstructorPattern(n *ast.ConstructorPattern) {
	p.write("Con(")
	n.Name.Accept(p)
	for _, el := range n.Elements {
		p.write(" ")
		el.Accept(p)
	}
	p.write(")")
}

func (p *TreePrinter) VisitListPattern(n *ast.ListPattern) {
	p.write("ListPattern\n")
	p.indent++
	for _, el := range n.Elements {
		p.writeIndent()
		el.Accept(p)
		p.write("\n")
	}
	p.indent--
}

func (p *TreePrinter) VisitTuplePattern(n *ast.TuplePattern) {
	p.write("TuplePattern\n")
	p.indent++
	for _, el := range n.Elements {
		p.writeIndent()
		el.Accept(p)
		p.write("\n")
	}
	p.indent--
}

func (p *TreePrinter) VisitRecordPattern(n *ast.RecordPattern) {
	p.write("RecordPattern\n")
	p.indent++
	for k, v := range n.Fields {
		p.writeIndent()
		p.write(k + ": ")
		v.Accept(p)
		p.write("\n")
	}
	p.indent--
}

func (p *TreePrinter) VisitTypePattern(n *ast.TypePattern) {
	p.write("TypePattern(" + n.Name + ": ")
	n.Type.Accept(p)
	p.write(")")
}

func (p *TreePrinter) VisitStringPattern(n *ast.StringPattern) {
	p.write("StringPattern(")
	for i, part := range n.Parts {
		if i > 0 {
			p.write(", ")
		}
		if part.IsCapture {
			p.write("{" + part.Value)
			if part.Greedy {
				p.write("...")
			}
			p.write("}")
		} else {
			p.write("\"" + part.Value + "\"")
		}
	}
	p.write(")")
}

func (p *TreePrinter) VisitPinPattern(n *ast.PinPattern) {
	p.write("PinPattern(^" + n.Name + ")")
}

func (p *TreePrinter) VisitSpreadExpression(n *ast.SpreadExpression) {
	p.write("Spread(")
	n.Expression.Accept(p)
	p.write("...)")
}

func (p *TreePrinter) VisitSpreadPattern(n *ast.SpreadPattern) {
	p.write("SpreadPattern(")
	n.Pattern.Accept(p)
	p.write("...)")
}

func (p *TreePrinter) VisitRecordLiteral(n *ast.RecordLiteral) {
	p.write("RecordLiteral\n")
	p.indent++
	for k, v := range n.Fields {
		p.writeIndent()
		p.write(k + ": ")
		v.Accept(p)
		p.write("\n")
	}
	p.indent--
}

func (p *TreePrinter) VisitMapLiteral(n *ast.MapLiteral) {
	p.write("MapLiteral\n")
	p.indent++
	for i, pair := range n.Pairs {
		p.writeIndent()
		p.write("pair ")
		p.write(fmt.Sprintf("%d", i))
		p.write(":\n")
		p.indent++
		p.writeIndent()
		p.write("key: ")
		pair.Key.Accept(p)
		p.write("\n")
		p.writeIndent()
		p.write("value: ")
		pair.Value.Accept(p)
		p.write("\n")
		p.indent--
	}
	p.indent--
}

func (p *TreePrinter) VisitRecordType(n *ast.RecordType) {
	p.write("RecordType\n")
	p.indent++
	for k, v := range n.Fields {
		p.writeIndent()
		p.write(k + ": ")
		v.Accept(p)
		p.write("\n")
	}
	p.indent--
}

func (p *TreePrinter) VisitUnionType(n *ast.UnionType) {
	p.write("UnionType\n")
	p.indent++
	for i, t := range n.Types {
		p.writeIndent()
		p.write(fmt.Sprintf("[%d]: ", i))
		t.Accept(p)
		p.write("\n")
	}
	p.indent--
}

func (p *TreePrinter) VisitForallType(n *ast.ForallType) {
	p.write("ForallType\n")
	p.indent++
	p.writeIndent()
	p.write("Vars: ")
	for i, v := range n.Vars {
		if i > 0 {
			p.write(", ")
		}
		p.write(v.Value)
	}
	p.write("\n")
	p.writeIndent()
	p.write("Body: ")
	n.Type.Accept(p)
	p.write("\n")
	p.indent--
}

func (p *TreePrinter) VisitMemberExpression(n *ast.MemberExpression) {
	p.write("MemberAccess\n")
	p.indent++
	p.writeIndent()
	p.write("Left: ")
	n.Left.Accept(p)
	p.write("\n")
	p.writeIndent()
	p.write("Field: " + n.Member.Value + "\n")
	p.indent--
}

func (p *TreePrinter) VisitListComprehension(n *ast.ListComprehension) {
	p.write("ListComprehension\n")
	p.indent++
	p.writeIndent()
	p.write("Output: ")
	n.Output.Accept(p)
	p.write("\n")
	p.writeIndent()
	p.write("Clauses:\n")
	p.indent++
	for _, clause := range n.Clauses {
		p.writeIndent()
		switch c := clause.(type) {
		case *ast.CompGenerator:
			p.write("Generator: ")
			c.Pattern.Accept(p)
			p.write(" <- ")
			c.Iterable.Accept(p)
		case *ast.CompFilter:
			p.write("Filter: ")
			c.Condition.Accept(p)
		}
		p.write("\n")
	}
	p.indent--
	p.indent--
}

func (p *TreePrinter) VisitRangeExpression(n *ast.RangeExpression) {
	p.write("Range\n")
	p.indent++
	p.writeIndent()
	p.write("Start: ")
	n.Start.Accept(p)
	p.write("\n")
	if n.Next != nil {
		p.writeIndent()
		p.write("Next: ")
		n.Next.Accept(p)
		p.write("\n")
	}
	p.writeIndent()
	p.write("End: ")
	n.End.Accept(p)
	p.write("\n")
	p.indent--
}

func (p *TreePrinter) VisitDirectiveStatement(n *ast.DirectiveStatement) {
	p.writeIndent()
	p.write("Directive: " + n.Name + "\n")
}

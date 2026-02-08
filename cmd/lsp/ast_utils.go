package main

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/token"
	"reflect"
)

// isInsideComment checks if the given 0-based position is inside a comment.
// It implements a simple state machine to parse string literals and comments.
func isInsideComment(content string, targetLine, targetChar int) bool {
	// State constants
	const (
		NORMAL            = iota
		STRING            // "..."
		RAW_STRING        // `...`
		TRIPLE_RAW_STRING // ```...```
		CHAR              // '...'
		LINE_COMMENT      // // ...
		BLOCK_COMMENT     // /* ... */
	)

	state := NORMAL
	line := 0
	col := 0

	i := 0
	runes := []rune(content)
	n := len(runes)

	for i < n {
		// Check if we reached the target position
		if line == targetLine && col == targetChar {
			return state == LINE_COMMENT || state == BLOCK_COMMENT
		}

		if line > targetLine {
			return false
		}

		r := runes[i]
		next := rune(0)
		if i+1 < n {
			next = runes[i+1]
		}

		switch state {
		case NORMAL:
			switch r {
			case '/':
				if next == '/' {
					state = LINE_COMMENT
					if line == targetLine && col == targetChar {
						return true
					}
				} else if next == '*' {
					state = BLOCK_COMMENT
					if line == targetLine && col == targetChar {
						return true
					}
				}
			case '"':
				state = STRING
			case '`':
				if next == '`' && i+2 < n && runes[i+2] == '`' {
					state = TRIPLE_RAW_STRING
					i += 2
					col += 2
				} else {
					state = RAW_STRING
				}
			case '\'':
				state = CHAR
			}
		case LINE_COMMENT:
			if r == '\n' {
				state = NORMAL
			}
		case BLOCK_COMMENT:
			if r == '*' && next == '/' {
				state = NORMAL
				i++
				col++
			}
		case STRING:
			if r == '\\' {
				// Skip escaped char
				i++
				col++
			} else if r == '"' {
				state = NORMAL
			}
		case RAW_STRING:
			if r == '`' {
				state = NORMAL
			}
		case TRIPLE_RAW_STRING:
			if r == '`' && next == '`' && i+2 < n && runes[i+2] == '`' {
				state = NORMAL
				i += 2
			}
		case CHAR:
			if r == '\\' {
				i++
				col++
			} else if r == '\'' {
				state = NORMAL
			}
		}

		// Update position
		if r == '\n' {
			line++
			col = 0
		} else {
			col++
		}
		i++
	}

	return false
}

// FindNodeAt finds the most nested AST node that contains the given position.
// line and char are 1-based (from Token.Line/Column or main.go's adjusted LSP coords).
func FindNodeAt(node ast.Node, line, char int) ast.Node {
	return findNodeAtWithDepth(node, line, char, 0)
}

const MaxASTTraversalDepth = 1000

func findNodeAtWithDepth(node ast.Node, line, char int, depth int) ast.Node {
	if node == nil || depth > MaxASTTraversalDepth {
		return node
	}

	// Check if this node contains the position
	if !containsPosition(node, line, char) {
		return nil
	}

	// Try to find a child that contains the position
	child := GetChildAt(node, line, char)
	if child != nil {
		// Optimization: if child is the same as node (shouldn't happen), avoid infinite recursion
		if child == node {
			return node
		}
		res := findNodeAtWithDepth(child, line, char, depth+1)
		if res != nil {
			return res
		}
	}

	// No child contains the position, return this node
	return node
}

// FindNodePath returns the path of nodes from root to the most specific node containing the position.
func FindNodePath(node ast.Node, line, char int) []ast.Node {
	return findNodePathWithDepth(node, line, char, 0)
}

func findNodePathWithDepth(node ast.Node, line, char int, depth int) []ast.Node {
	if node == nil || depth > MaxASTTraversalDepth {
		return nil
	}

	// For completion, we want to find the node even if cursor is at the end (inclusive)
	if !containsPositionInclusive(node, line, char) {
		return nil
	}

	path := []ast.Node{node}
	// Use inclusive check for children too
	child := GetChildAt(node, line, char)
	if child != nil && child != node {
		tail := findNodePathWithDepth(child, line, char, depth+1)
		if tail != nil {
			path = append(path, tail...)
		}
	}
	return path
}

// GetChildAt finds the immediate child node that contains the position
func GetChildAt(node ast.Node, line, char int) ast.Node {
	// Use inclusive check for completion support.
	// For Hover, FindNodeAt checks strict containsPosition on the returned child,
	// so returning a boundary node here is safe.
	return getChildAtInclusive(node, line, char)
}

func getChildAtInclusive(node ast.Node, line, char int) ast.Node {
	switch n := node.(type) {
	case *ast.Program:
		if n.Package != nil && containsPositionInclusive(n.Package, line, char) {
			return n.Package
		}
		for _, imp := range n.Imports {
			if containsPositionInclusive(imp, line, char) {
				return imp
			}
		}
		for _, stmt := range n.Statements {
			if containsPositionInclusive(stmt, line, char) {
				return stmt
			}
		}
	case *ast.PackageDeclaration:
		if containsPositionInclusive(n.Name, line, char) {
			return n.Name
		}
		for _, exp := range n.Exports {
			if exp.Symbol != nil && containsPositionInclusive(exp.Symbol, line, char) {
				return exp.Symbol
			}
			if exp.ModuleName != nil && containsPositionInclusive(exp.ModuleName, line, char) {
				return exp.ModuleName
			}
			for _, sym := range exp.Symbols {
				if containsPositionInclusive(sym, line, char) {
					return sym
				}
			}
		}
	case *ast.ImportStatement:
		if containsPositionInclusive(n.Path, line, char) {
			return n.Path
		}
		if n.Alias != nil && containsPositionInclusive(n.Alias, line, char) {
			return n.Alias
		}
		for _, sym := range n.Symbols {
			if containsPositionInclusive(sym, line, char) {
				return sym
			}
		}
		for _, sym := range n.Exclude {
			if containsPositionInclusive(sym, line, char) {
				return sym
			}
		}
	case *ast.ExpressionStatement:
		if containsPositionInclusive(n.Expression, line, char) {
			return n.Expression
		}
	case *ast.FunctionStatement:
		if containsPositionInclusive(n.Name, line, char) {
			return n.Name
		}
		if n.Receiver != nil {
			if n.Receiver.Name != nil && containsPositionInclusive(n.Receiver.Name, line, char) {
				return n.Receiver.Name
			}
			if n.Receiver.Type != nil && containsPositionInclusive(n.Receiver.Type, line, char) {
				return n.Receiver.Type
			}
		}
		for _, param := range n.Parameters {
			if param.Name != nil && containsPositionInclusive(param.Name, line, char) {
				return param.Name
			}
			if param.Type != nil && containsPositionInclusive(param.Type, line, char) {
				return param.Type
			}
			if param.Default != nil && containsPositionInclusive(param.Default, line, char) {
				return param.Default
			}
		}
		if n.ReturnType != nil && containsPositionInclusive(n.ReturnType, line, char) {
			return n.ReturnType
		}
		if n.Body != nil && containsPositionInclusive(n.Body, line, char) {
			return n.Body
		}
	case *ast.BlockStatement:
		for _, stmt := range n.Statements {
			if containsPositionInclusive(stmt, line, char) {
				return stmt
			}
		}
	case *ast.IfExpression:
		if containsPositionInclusive(n.Condition, line, char) {
			return n.Condition
		}
		if containsPositionInclusive(n.Consequence, line, char) {
			return n.Consequence
		}
		if n.Alternative != nil && containsPositionInclusive(n.Alternative, line, char) {
			return n.Alternative
		}
	case *ast.ForExpression:
		if containsPositionInclusive(n.Initializer, line, char) {
			return n.Initializer
		}
		if containsPositionInclusive(n.Condition, line, char) {
			return n.Condition
		}
		if containsPositionInclusive(n.ItemName, line, char) {
			return n.ItemName
		}
		if containsPositionInclusive(n.Iterable, line, char) {
			return n.Iterable
		}
		if containsPositionInclusive(n.Body, line, char) {
			return n.Body
		}
	case *ast.ReturnStatement:
		if containsPositionInclusive(n.Value, line, char) {
			return n.Value
		}
	case *ast.BreakStatement:
		if containsPositionInclusive(n.Value, line, char) {
			return n.Value
		}
	case *ast.ContinueStatement:
		// No children
	case *ast.CallExpression:
		if containsPositionInclusive(n.Function, line, char) {
			return n.Function
		}
		for _, arg := range n.Arguments {
			if containsPositionInclusive(arg, line, char) {
				return arg
			}
		}
	case *ast.InfixExpression:
		if containsPositionInclusive(n.Left, line, char) {
			return n.Left
		}
		if containsPositionInclusive(n.Right, line, char) {
			return n.Right
		}
	case *ast.PrefixExpression:
		if containsPositionInclusive(n.Right, line, char) {
			return n.Right
		}
	case *ast.IndexExpression:
		if containsPositionInclusive(n.Left, line, char) {
			return n.Left
		}
		if containsPositionInclusive(n.Index, line, char) {
			return n.Index
		}
	case *ast.MemberExpression:
		if containsPositionInclusive(n.Member, line, char) {
			return n.Member
		}
		if containsPositionInclusive(n.Left, line, char) {
			return n.Left
		}
	case *ast.AssignExpression:
		if containsPositionInclusive(n.Left, line, char) {
			return n.Left
		}
		if containsPositionInclusive(n.Value, line, char) {
			return n.Value
		}
	case *ast.AnnotatedExpression:
		if containsPositionInclusive(n.Expression, line, char) {
			return n.Expression
		}
		if containsPositionInclusive(n.TypeAnnotation, line, char) {
			return n.TypeAnnotation
		}
	case *ast.ConstantDeclaration:
		if containsPositionInclusive(n.Name, line, char) {
			return n.Name
		}
		if n.TypeAnnotation != nil && containsPositionInclusive(n.TypeAnnotation, line, char) {
			return n.TypeAnnotation
		}
		if containsPositionInclusive(n.Value, line, char) {
			return n.Value
		}
	case *ast.MatchExpression:
		if containsPositionInclusive(n.Expression, line, char) {
			return n.Expression
		}
		for _, arm := range n.Arms {
			if containsPositionInclusive(arm.Pattern, line, char) {
				return arm.Pattern
			}
			if arm.Guard != nil && containsPositionInclusive(arm.Guard, line, char) {
				return arm.Guard
			}
			if containsPositionInclusive(arm.Expression, line, char) {
				return arm.Expression
			}
		}
	case *ast.IdentifierPattern:
		// Leaf node
	case *ast.WildcardPattern:
		// Leaf node
	case *ast.LiteralPattern:
		// Leaf node
	case *ast.TypePattern:
		if containsPositionInclusive(n.Type, line, char) {
			return n.Type
		}
	case *ast.ConstructorPattern:
		if n.Name != nil && containsPositionInclusive(n.Name, line, char) {
			return n.Name
		}
		for _, el := range n.Elements {
			if containsPositionInclusive(el, line, char) {
				return el
			}
		}
	case *ast.ListPattern:
		for _, el := range n.Elements {
			if containsPositionInclusive(el, line, char) {
				return el
			}
		}
	case *ast.TuplePattern:
		for _, el := range n.Elements {
			if containsPositionInclusive(el, line, char) {
				return el
			}
		}
	case *ast.RecordPattern:
		for _, el := range n.Fields {
			if containsPositionInclusive(el, line, char) {
				return el
			}
		}
	case *ast.SpreadPattern:
		if n.Pattern != nil && containsPositionInclusive(n.Pattern, line, char) {
			return n.Pattern
		}
	case *ast.PinPattern:
		// Leaf node (Name is string)
	case *ast.ListLiteral:
		for _, elem := range n.Elements {
			if containsPositionInclusive(elem, line, char) {
				return elem
			}
		}
	case *ast.TupleLiteral:
		for _, elem := range n.Elements {
			if containsPositionInclusive(elem, line, char) {
				return elem
			}
		}
	case *ast.MapLiteral:
		for _, pair := range n.Pairs {
			if containsPositionInclusive(pair.Key, line, char) {
				return pair.Key
			}
			if containsPositionInclusive(pair.Value, line, char) {
				return pair.Value
			}
		}
	case *ast.RecordLiteral:
		if n.Spread != nil && containsPositionInclusive(n.Spread, line, char) {
			return n.Spread
		}
		for _, val := range n.Fields {
			if containsPositionInclusive(val, line, char) {
				return val
			}
		}
	case *ast.FunctionLiteral:
		for _, param := range n.Parameters {
			if param.Name != nil && containsPositionInclusive(param.Name, line, char) {
				return param.Name
			}
			if param.Type != nil && containsPositionInclusive(param.Type, line, char) {
				return param.Type
			}
		}
		if n.ReturnType != nil && containsPositionInclusive(n.ReturnType, line, char) {
			return n.ReturnType
		}
		if containsPositionInclusive(n.Body, line, char) {
			return n.Body
		}
	case *ast.InstanceDeclaration:
		if containsPositionInclusive(n.TraitName, line, char) {
			return n.TraitName
		}
		if n.ModuleName != nil && containsPositionInclusive(n.ModuleName, line, char) {
			return n.ModuleName
		}
		for _, arg := range n.Args {
			if containsPositionInclusive(arg, line, char) {
				return arg
			}
		}
		for _, method := range n.Methods {
			if containsPositionInclusive(method, line, char) {
				return method
			}
		}
	case *ast.TraitDeclaration:
		if containsPositionInclusive(n.Name, line, char) {
			return n.Name
		}
		for _, method := range n.Signatures {
			if containsPositionInclusive(method, line, char) {
				return method
			}
		}
	case *ast.TypeDeclarationStatement:
		if containsPositionInclusive(n.Name, line, char) {
			return n.Name
		}
		if n.TargetType != nil && containsPositionInclusive(n.TargetType, line, char) {
			return n.TargetType
		}
		for _, c := range n.Constructors {
			if containsPositionInclusive(c, line, char) {
				return c
			}
		}
	case *ast.DataConstructor:
		if containsPositionInclusive(n.Name, line, char) {
			return n.Name
		}
		for _, p := range n.Parameters {
			if containsPositionInclusive(p, line, char) {
				return p
			}
		}
	case *ast.NamedType:
		if containsPositionInclusive(n.Name, line, char) {
			return n.Name
		}
		for _, arg := range n.Args {
			if containsPositionInclusive(arg, line, char) {
				return arg
			}
		}
	case *ast.TupleType:
		for _, t := range n.Types {
			if containsPositionInclusive(t, line, char) {
				return t
			}
		}
	case *ast.RecordType:
		for _, t := range n.Fields {
			if containsPositionInclusive(t, line, char) {
				return t
			}
		}
	case *ast.FunctionType:
		for _, p := range n.Parameters {
			if containsPositionInclusive(p, line, char) {
				return p
			}
		}
		if containsPositionInclusive(n.ReturnType, line, char) {
			return n.ReturnType
		}
	case *ast.UnionType:
		for _, t := range n.Types {
			if containsPositionInclusive(t, line, char) {
				return t
			}
		}
	}
	return nil
}

// containsPosition checks if a node spans the given position
// Arguments line and char are 1-based.
func containsPosition(node ast.Node, line, char int) bool {
	if node == nil {
		return false
	}

	startTok, endTok, ok := getNodePosition(node)
	if !ok {
		return false
	}

	// All coordinates are 1-based (Token.Line is 1-based, input line is 1-based)
	startLine := startTok.Line
	startChar := startTok.Column
	endLine := endTok.Line
	// End column is start + len. Token columns are 1-based.
	endChar := endTok.Column + len(endTok.Lexeme)

	// Check if position is within the node's range
	if line < startLine || line > endLine {
		return false
	}
	if line == startLine && char < startChar {
		return false
	}
	// Inclusive end for multi-line?
	// If line == endLine, char must be < endChar.
	if line == endLine && char >= endChar {
		return false
	}
	return true
}

// containsPositionInclusive checks if a node spans the given position,
// allowing the cursor to be exactly at the end of the node.
func containsPositionInclusive(node ast.Node, line, char int) bool {
	if node == nil {
		return false
	}

	startTok, endTok, ok := getNodePosition(node)
	if !ok {
		return false
	}

	startLine := startTok.Line
	startChar := startTok.Column
	endLine := endTok.Line
	endChar := endTok.Column + len(endTok.Lexeme)

	if line < startLine || line > endLine {
		return false
	}
	if line == startLine && char < startChar {
		return false
	}
	// Allow char == endChar (touching the end)
	// But strictly char > endChar is outside
	if line == endLine && char > endChar {
		return false
	}
	return true
}

// getNodePosition returns the start and end tokens of a node
func getNodePosition(node ast.Node) (token.Token, token.Token, bool) {
	if node == nil {
		return token.Token{}, token.Token{}, false
	}
	// Check for typed nil
	if val := reflect.ValueOf(node); val.Kind() == reflect.Ptr && val.IsNil() {
		return token.Token{}, token.Token{}, false
	}

	// Logic to find start and end tokens
	var startTok, endTok token.Token
	var startFound, endFound bool

	switch n := node.(type) {
	case *ast.Program:
		if len(n.Statements) > 0 {
			startTok, _, startFound = getNodePosition(n.Statements[0])
			_, endTok, endFound = getNodePosition(n.Statements[len(n.Statements)-1])
		}
	case *ast.PackageDeclaration:
		startTok = n.Token
		startFound = true
		// End at closing paren if exports exist, or Name if not
		if len(n.Exports) > 0 {
			lastExp := n.Exports[len(n.Exports)-1]
			_, endTok, endFound = getNodePosition(lastExp.Symbol) // might be nil if ModuleName
			if !endFound {
				endTok = lastExp.Token
				endFound = true
			}
		} else {
			_, endTok, endFound = getNodePosition(n.Name)
		}
	case *ast.ImportStatement:
		startTok = n.Token
		startFound = true
		if len(n.Symbols) > 0 {
			_, endTok, endFound = getNodePosition(n.Symbols[len(n.Symbols)-1])
		} else if len(n.Exclude) > 0 {
			_, endTok, endFound = getNodePosition(n.Exclude[len(n.Exclude)-1])
		} else if n.Alias != nil {
			_, endTok, endFound = getNodePosition(n.Alias)
		} else {
			_, endTok, endFound = getNodePosition(n.Path)
		}
	case *ast.ExpressionStatement:
		return getNodePosition(n.Expression)
	case *ast.FunctionStatement:
		startTok = n.Token
		startFound = true
		if n.Body != nil {
			_, endTok, endFound = getNodePosition(n.Body)
		} else if n.ReturnType != nil {
			_, endTok, endFound = getNodePosition(n.ReturnType)
		} else {
			// Signature only
			if len(n.Parameters) > 0 {
				p := n.Parameters[len(n.Parameters)-1]
				if p.Type != nil {
					_, endTok, endFound = getNodePosition(p.Type)
				} else if p.Name != nil {
					_, endTok, endFound = getNodePosition(p.Name)
				}
			} else {
				_, endTok, endFound = getNodePosition(n.Name)
			}
		}
	case *ast.BlockStatement:
		if n.Token.Type != "" {
			startTok = n.Token
			startFound = true
		}

		// Use explicit closing brace if available
		if n.RBraceToken.Type != "" {
			endTok = n.RBraceToken
			endFound = true
		} else if len(n.Statements) > 0 {
			if !startFound {
				startTok, _, startFound = getNodePosition(n.Statements[0])
			}
			_, endTok, endFound = getNodePosition(n.Statements[len(n.Statements)-1])
		} else {
			endTok = startTok
			endFound = true
		}
	case *ast.IfExpression:
		startTok = n.Token
		startFound = true
		if n.Alternative != nil {
			_, endTok, endFound = getNodePosition(n.Alternative)
		} else {
			_, endTok, endFound = getNodePosition(n.Consequence)
		}
	case *ast.CallExpression:
		startTok, _, startFound = getNodePosition(n.Function)
		if len(n.Arguments) > 0 {
			_, endTok, endFound = getNodePosition(n.Arguments[len(n.Arguments)-1])
		} else {
			_, endTok, endFound = getNodePosition(n.Function)
		}
	case *ast.InfixExpression:
		startTok, _, startFound = getNodePosition(n.Left)
		if n.Right != nil {
			_, endTok, endFound = getNodePosition(n.Right)
		} else {
			// Partial expression: "a +"
			endTok = n.Token // The operator
			endFound = true
		}
	case *ast.PrefixExpression:
		startTok = n.Token
		startFound = true
		_, endTok, endFound = getNodePosition(n.Right)
	case *ast.IndexExpression:
		startTok, _, startFound = getNodePosition(n.Left)
		_, endTok, endFound = getNodePosition(n.Index)
	case *ast.MemberExpression:
		startTok, _, startFound = getNodePosition(n.Left)
		_, endTok, endFound = getNodePosition(n.Member)
	case *ast.AnnotatedExpression:
		startTok, _, startFound = getNodePosition(n.Expression)
		_, endTok, endFound = getNodePosition(n.TypeAnnotation)
	case *ast.AssignExpression:
		startTok, _, startFound = getNodePosition(n.Left)
		_, endTok, endFound = getNodePosition(n.Value)
	case *ast.ConstantDeclaration:
		startTok, _, startFound = getNodePosition(n.Name)
		_, endTok, endFound = getNodePosition(n.Value)
	case *ast.MatchExpression:
		startTok = n.Token
		startFound = true
		if len(n.Arms) > 0 {
			lastArm := n.Arms[len(n.Arms)-1]
			_, endTok, endFound = getNodePosition(lastArm.Expression)
		} else {
			_, endTok, endFound = getNodePosition(n.Expression)
		}
	case *ast.ListLiteral:
		startTok = n.Token
		startFound = true
		if len(n.Elements) > 0 {
			_, endTok, endFound = getNodePosition(n.Elements[len(n.Elements)-1])
		} else {
			endTok = startTok
			endFound = true
		}
	case *ast.TupleLiteral:
		startTok = n.Token
		startFound = true
		if len(n.Elements) > 0 {
			_, endTok, endFound = getNodePosition(n.Elements[len(n.Elements)-1])
		} else {
			endTok = startTok
			endFound = true
		}
	case *ast.MapLiteral:
		startTok = n.Token
		startFound = true
		if len(n.Pairs) > 0 {
			_, endTok, endFound = getNodePosition(n.Pairs[len(n.Pairs)-1].Value)
		} else {
			endTok = startTok
			endFound = true
		}
	case *ast.RecordLiteral:
		startTok = n.Token
		startFound = true
		var maxEndTok token.Token
		var foundMax bool

		if n.Spread != nil {
			_, end, found := getNodePosition(n.Spread)
			if found {
				maxEndTok = end
				foundMax = true
			}
		}

		for _, val := range n.Fields {
			_, end, found := getNodePosition(val)
			if found {
				if !foundMax {
					maxEndTok = end
					foundMax = true
				} else {
					if end.Line > maxEndTok.Line || (end.Line == maxEndTok.Line && end.Column > maxEndTok.Column) {
						maxEndTok = end
					}
				}
			}
		}

		if foundMax {
			endTok = maxEndTok
			endFound = true
		} else {
			endTok = startTok
			endFound = true
		}
	case *ast.FunctionLiteral:
		startTok = n.Token
		startFound = true
		if n.Body != nil {
			_, endTok, endFound = getNodePosition(n.Body)
		} else {
			endTok = startTok
			endFound = true
		}
	case *ast.TypeDeclarationStatement:
		startTok = n.Token
		startFound = true
		if len(n.Constructors) > 0 {
			_, endTok, endFound = getNodePosition(n.Constructors[len(n.Constructors)-1])
		} else if n.TargetType != nil {
			_, endTok, endFound = getNodePosition(n.TargetType)
		} else {
			_, endTok, endFound = getNodePosition(n.Name)
		}
	case *ast.DataConstructor:
		if n.Name != nil {
			startTok, _, startFound = getNodePosition(n.Name)
		} else {
			startTok = n.Token
			startFound = true
		}
		if len(n.Parameters) > 0 {
			_, endTok, endFound = getNodePosition(n.Parameters[len(n.Parameters)-1])
		} else if n.Name != nil {
			_, endTok, endFound = getNodePosition(n.Name)
		} else {
			endTok = startTok
			endFound = true
		}
	case *ast.NamedType:
		if n.Name != nil {
			startTok, _, startFound = getNodePosition(n.Name)
		} else {
			startTok = n.Token
			startFound = true
		}
		if len(n.Args) > 0 {
			_, endTok, endFound = getNodePosition(n.Args[len(n.Args)-1])
		} else if n.Name != nil {
			_, endTok, endFound = getNodePosition(n.Name)
		} else {
			endTok = startTok
			endFound = true
		}
	case *ast.TupleType:
		startTok = n.Token
		startFound = true
		if len(n.Types) > 0 {
			_, endTok, endFound = getNodePosition(n.Types[len(n.Types)-1])
		} else {
			endTok = startTok
			endFound = true
		}
	case *ast.RecordType:
		startTok = n.Token
		startFound = true
		var maxEndTok token.Token
		var foundMax bool
		for _, typ := range n.Fields {
			_, end, found := getNodePosition(typ)
			if found {
				if !foundMax {
					maxEndTok = end
					foundMax = true
				} else {
					if end.Line > maxEndTok.Line || (end.Line == maxEndTok.Line && end.Column > maxEndTok.Column) {
						maxEndTok = end
					}
				}
			}
		}
		if foundMax {
			endTok = maxEndTok
			endFound = true
		} else {
			endTok = startTok
			endFound = true
		}
	case *ast.FunctionType:
		if len(n.Parameters) > 0 {
			startTok, _, startFound = getNodePosition(n.Parameters[0])
		} else {
			startTok = n.Token
			startFound = true
		}
		if n.ReturnType != nil {
			_, endTok, endFound = getNodePosition(n.ReturnType)
		} else {
			endTok = startTok
			endFound = true
		}
	case *ast.UnionType:
		if len(n.Types) > 0 {
			startTok, _, startFound = getNodePosition(n.Types[0])
			_, endTok, endFound = getNodePosition(n.Types[len(n.Types)-1])
		} else {
			startTok = n.Token
			startFound = true
			endTok = startTok
			endFound = true
		}
	case *ast.InstanceDeclaration:
		startTok = n.Token
		startFound = true
		if len(n.Methods) > 0 {
			_, endTok, endFound = getNodePosition(n.Methods[len(n.Methods)-1])
		} else if len(n.Args) > 0 {
			_, endTok, endFound = getNodePosition(n.Args[len(n.Args)-1])
		} else if n.TraitName != nil {
			_, endTok, endFound = getNodePosition(n.TraitName)
		} else {
			endTok = startTok
			endFound = true
		}
	case *ast.TraitDeclaration:
		startTok = n.Token
		startFound = true
		if len(n.Signatures) > 0 {
			_, endTok, endFound = getNodePosition(n.Signatures[len(n.Signatures)-1])
		} else if len(n.Constraints) > 0 {
			_, endTok, endFound = getNodePosition(n.Name)
		} else {
			_, endTok, endFound = getNodePosition(n.Name)
		}
	default:
		// For leaf nodes (literals, identifiers), use GetToken
		if tp, ok := node.(ast.TokenProvider); ok {
			tok := tp.GetToken()
			return tok, tok, true
		}
	}

	// Fallback if we couldn't determine start/end fully
	if !startFound && !endFound {
		if tp, ok := node.(ast.TokenProvider); ok {
			tok := tp.GetToken()
			return tok, tok, true
		}
		return token.Token{}, token.Token{}, false
	}

	if startFound && !endFound {
		endTok = startTok
		endFound = true
	}
	if !startFound && endFound {
		startTok = endTok
		startFound = true
	}

	return startTok, endTok, true
}

// hasPackageDeclaration checks if the source content starts with a package declaration.
// It uses the lexer to scan tokens, skipping comments and whitespace.
func hasPackageDeclaration(content string) bool {
	l := lexer.New(content)

	// Scan until we find a non-comment token or EOF
	for {
		tok := l.NextToken()

		// If we hit EOF, no package declaration found
		if tok.Type == token.EOF {
			return false
		}

		// Lexer automatically skips whitespace, but not newlines (which are tokens in Funxy)
		// We should skip NEWLINE tokens as they are whitespace-equivalent here
		if tok.Type == token.NEWLINE {
			continue
		}

		if tok.Type == token.PACKAGE {
			return true
		}

		// If we found any other token first (e.g. import, fun, etc.), then no package declaration
		return false
	}
}

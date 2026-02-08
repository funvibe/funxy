package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

func (s *LanguageServer) handleHover(id interface{}, params HoverParams) error {
	log.Printf("Handling hover request for %s at line %d, char %d", params.TextDocument.URI, params.Position.Line, params.Position.Character)

	// Get document state from cache
	s.mu.RLock()
	docState, exists := s.documents[params.TextDocument.URI]
	s.mu.RUnlock()

	if !exists {
		return s.sendResponse(ResponseMessage{
			Jsonrpc: "2.0",
			ID:      id,
			Result:  nil,
		})
	}

	// Read from cache
	docState.Mu.RLock()
	content := docState.Content
	finalCtx := docState.Context
	docState.Mu.RUnlock()

	if finalCtx == nil {
		return s.sendResponse(ResponseMessage{
			Jsonrpc: "2.0",
			ID:      id,
			Result:  nil,
		})
	}

	// Check if the position is inside a comment
	if isInsideComment(content, params.Position.Line, params.Position.Character) {
		return s.sendResponse(ResponseMessage{
			Jsonrpc: "2.0",
			ID:      id,
			Result:  nil,
		})
	}

	// Use cached analysis result
	if len(finalCtx.Errors) > 0 {
		// Proceed even if there are errors, as we might still have useful AST/Type info
	}

	// Find the node at the given position
	// Use FindNodePath to get context
	path := FindNodePath(finalCtx.AstRoot, params.Position.Line+1, params.Position.Character+1)
	var node ast.Node
	if len(path) > 0 {
		node = path[len(path)-1]
	}

	if node == nil {
		// Heuristic: Check if cursor is on/after a closing bracket.
		// Map (line, char) to byte offset.
		byteOffset := 0
		lines := strings.Split(content, "\n")
		for i := 0; i < params.Position.Line; i++ {
			if i < len(lines) {
				byteOffset += len(lines[i]) + 1 // +1 for newline (assuming \n)
			}
		}
		byteOffset += params.Position.Character

		targetPos := -1

		if byteOffset < len(content) {
			b := content[byteOffset]
			if b == ')' || b == ']' || b == '}' {
				targetPos = byteOffset
			}
		}

		// Also check cursor-1 if cursor is not on bracket
		if targetPos == -1 && byteOffset > 0 && byteOffset-1 < len(content) {
			b := content[byteOffset-1]
			if b == ')' || b == ']' || b == '}' {
				targetPos = byteOffset - 1
			}
		}

		if targetPos != -1 {
			openerPos := findMatchingOpener(content, targetPos)
			if openerPos != -1 {
				// Convert openerPos back to Line/Char
				openerLine := 0
				openerChar := 0
				currentOffset := 0
				for i, lineStr := range lines {
					lineLen := len(lineStr) + 1 // +1 for \n
					if currentOffset+lineLen > openerPos {
						openerLine = i
						openerChar = openerPos - currentOffset
						break
					}
					currentOffset += lineLen
				}

				// Try FindNodeAt with opener position
				node = FindNodeAt(finalCtx.AstRoot, openerLine+1, openerChar+1)
			}
		}
	}

	if node == nil {
		// Check for keywords even if no AST node found (e.g. syntax error or top-level)
		word := getWordAtPosition(content, params.Position.Line, params.Position.Character)
		if word != "" {
			keywordHover := getKeywordHoverText(word, nil)
			if keywordHover != "" {
				return s.sendResponse(ResponseMessage{
					Jsonrpc: "2.0",
					ID:      id,
					Result: &Hover{
						Contents: MarkupContent{
							Kind:  "markdown",
							Value: keywordHover,
						},
					},
				})
			}
		}

		return s.sendResponse(ResponseMessage{
			Jsonrpc: "2.0",
			ID:      id,
			Result:  nil,
		})
	}

	// Handle parent CallExpression for closing parenthesis hover
	// If we are at ')' of "func(arg)", FindNodePath returns [CallExpr, arg].
	// We want to check if the cursor is actually on the closing paren of the CallExpr.
	if len(path) >= 2 {
		if callExpr, ok := path[len(path)-2].(*ast.CallExpression); ok {
			// Check if we are hovering on ')'
			isClosingParen := false
			lineContent := getLine(content, params.Position.Line)

			// Case 1: Cursor is ON ')' (character matches)
			if params.Position.Character < len(lineContent) && lineContent[params.Position.Character] == ')' {
				isClosingParen = true
			} else if params.Position.Character > 0 && params.Position.Character-1 < len(lineContent) && lineContent[params.Position.Character-1] == ')' {
				// Case 2: Cursor is JUST AFTER ')' (previous character matches)
				isClosingParen = true
			}

			if isClosingParen {
				// We are likely on the closing paren of this call.
				// Redirect node to the CallExpression (to show return type)
				node = callExpr
			}
		}
	}

	// Ignore root nodes that don't provide useful hover info
	switch node.(type) {
	case *ast.Program, *ast.BlockStatement:
		// Check for keywords before ignoring
		word := getWordAtPosition(content, params.Position.Line, params.Position.Character)
		if word != "" {
			keywordHover := getKeywordHoverText(word, node)
			if keywordHover != "" {
				return s.sendResponse(ResponseMessage{
					Jsonrpc: "2.0",
					ID:      id,
					Result: &Hover{
						Contents: MarkupContent{
							Kind:  "markdown",
							Value: keywordHover,
						},
					},
				})
			}
		}

		return s.sendResponse(ResponseMessage{
			Jsonrpc: "2.0",
			ID:      id,
			Result:  nil,
		})
	}

	// Special handling for CallExpression parentheses:
	// If hovering over the '(' token of a call, show the function signature instead of the return type.
	if callExpr, ok := node.(*ast.CallExpression); ok {
		// Calculate position of '(' token
		parenLine := callExpr.Token.Line - 1
		parenCol := callExpr.Token.Column - 1

		if params.Position.Line == parenLine && params.Position.Character == parenCol {
			// Redirect node to the function being called
			node = callExpr.Function
		}
	}

	// Heuristic: Closing parenthesis of a nested call
	if callExpr, ok := node.(*ast.CallExpression); ok {
		// Check if we are hovering on ')'
		isClosingParen := false
		lineContent := getLine(content, params.Position.Line)

		// Case 1: Cursor is ON ')' (character matches)
		if params.Position.Character < len(lineContent) && lineContent[params.Position.Character] == ')' {
			isClosingParen = true
		} else if params.Position.Character > 0 && params.Position.Character-1 < len(lineContent) && lineContent[params.Position.Character-1] == ')' {
			// Case 2: Cursor is JUST AFTER ')' (previous character matches)
			isClosingParen = true
		}

		if isClosingParen {
			// Check arguments
			for _, arg := range callExpr.Arguments {
				// Only care if arg is also a CallExpression (or maybe Block?)
				if nestedCall, isNested := arg.(*ast.CallExpression); isNested {
					_, endTok, ok := getNodePosition(nestedCall)
					if ok {
						// Check if ')' is "near" the end of this argument
						argEndLine := endTok.Line - 1
						argEndCol := endTok.Column + len(endTok.Lexeme) - 1 // 0-based end index (exclusive)

						// Simple heuristic: if on same line and close enough
						if argEndLine == params.Position.Line && params.Position.Character >= argEndCol {
							// Check text between argEnd and cursor
							// Should match \s*)\s*
							startIdx := argEndCol
							endIdx := params.Position.Character
							if endIdx >= startIdx && endIdx < len(lineContent) {
								between := lineContent[startIdx : endIdx+1] // +1 to include current char ')'
								if strings.Contains(between, ")") {
									// Found it! This ')' likely closes nestedCall.
									// Redirect node to nestedCall's return type
									node = nestedCall
									break
								}
							}
						}
					}
				}
			}
		}
	}

	// Get type information for the node
	var hoverText string

	// Helper to resolve symbol info
	resolveSymbol := func(name string, node ast.Node) {
		var symbol symbols.Symbol
		var found bool

		// Try ResolutionMap first
		if sym, ok := finalCtx.ResolutionMap[node]; ok {
			symbol = sym
			found = true
		} else if sym, ok := finalCtx.SymbolTable.Find(name); ok {
			// Fallback to name lookup
			symbol = sym
			found = true
		}

		if found {
			if symbol.Kind == symbols.TraitSymbol {
				// Prevent incorrectly resolving a type Identifier to a Trait Symbol
				// unless the identifier matches the trait name
				if name != symbol.Name {
					if sym, ok := finalCtx.SymbolTable.Find(name); ok {
						symbol = sym
					} else {
						found = false
					}
				}
			}
		}

		if found {
			if symbol.Kind == symbols.TraitSymbol {
				traitName := symbol.Name
				typeParams, _ := finalCtx.SymbolTable.GetTraitTypeParams(traitName)
				deps, _ := finalCtx.SymbolTable.GetTraitFunctionalDependencies(traitName)

				text := fmt.Sprintf("trait %s", traitName)
				if len(typeParams) > 0 {
					text += "<" + strings.Join(typeParams, ", ") + ">"
				}
				for _, dep := range deps {
					text += fmt.Sprintf("\n| %s -> %s", strings.Join(dep.From, ", "), strings.Join(dep.To, ", "))
				}
				hoverText = fmt.Sprintf("```funxy\n%s\n```", text)
			} else if symbol.Kind == symbols.TypeSymbol {
				hoverText = fmt.Sprintf("```funxy\ntype %s\n```", symbol.Name)
				if symbol.Type != nil {
					hoverText = fmt.Sprintf("```funxy\ntype %s = %s\n```", symbol.Name, PrettifyType(symbol.Type))
				}
			} else if symbol.Type != nil {
				hoverText = fmt.Sprintf("```funxy\n%s: %s\n```", name, PrettifyType(symbol.Type))
			}
		}
	}

	// 1. Check ResolutionMap/SymbolTable for canonical type information
	if ident, ok := node.(*ast.Identifier); ok {
		resolveSymbol(ident.Value, ident)
		if hoverText == "" {
			// If not found in symbol table (e.g. local variable or parameter), allow TypeMap to handle it below
			// Special check for function parameters which might not be in ResolutionMap/SymbolTable directly
			// but are part of a function signature that is typed.
			if len(path) >= 2 {
				parent := path[len(path)-2]
				if fn, ok := parent.(*ast.FunctionStatement); ok {
					// Check if ident is one of the parameters
					for i, param := range fn.Parameters {
						if param.Name == ident {
							// Found it! Try to get function type from TypeMap
							if fnType, ok := finalCtx.TypeMap[fn.Name]; ok {
								// Handle TFunc
								if tFunc, ok := fnType.(typesystem.TFunc); ok {
									if i < len(tFunc.Params) {
										hoverText = fmt.Sprintf("```funxy\n%s: %s\n```", ident.Value, PrettifyType(tFunc.Params[i]))
									}
								} else if tForall, ok := fnType.(typesystem.TForall); ok {
									// Handle Generic Function (TForall -> TFunc)
									if tFunc, ok := tForall.Type.(typesystem.TFunc); ok {
										if i < len(tFunc.Params) {
											hoverText = fmt.Sprintf("```funxy\n%s: %s\n```", ident.Value, PrettifyType(tFunc.Params[i]))
										}
									}
								}
							}
							break
						}
					}
				} else if fn, ok := parent.(*ast.FunctionLiteral); ok {
					// Check if ident is one of the parameters
					for i, param := range fn.Parameters {
						if param.Name == ident {
							// Found it! Try to get function type from TypeMap
							// FunctionLiteral node itself should be the key in TypeMap
							if fnType, ok := finalCtx.TypeMap[fn]; ok {
								if tFunc, ok := fnType.(typesystem.TFunc); ok {
									if i < len(tFunc.Params) {
										hoverText = fmt.Sprintf("```funxy\n%s: %s\n```", ident.Value, PrettifyType(tFunc.Params[i]))
									}
								}
							}
							break
						}
					}
				}
			}
		}
	} else if pat, ok := node.(*ast.IdentifierPattern); ok {
		resolveSymbol(pat.Value, pat)
	} else if pat, ok := node.(*ast.TypePattern); ok {
		resolveSymbol(pat.Name, pat)
	}

	// Check for keywords
	if hoverText == "" {
		word := getWordAtPosition(content, params.Position.Line, params.Position.Character)
		if word != "" {
			// Don't show "Keyword: match" if we are inside a pattern binding that happens to be named like a keyword (unlikely for match but possible for others)
			// But primarily, if we found a node but failed to resolve its type, we shouldn't fallback to keyword unless it really IS a keyword usage.
			// Node type check:
			switch node.(type) {
			case *ast.MatchExpression, *ast.IfExpression, *ast.ForExpression, *ast.ReturnStatement, *ast.BreakStatement, *ast.ContinueStatement:
				// isKeywordUsage = true
			}

			keywordHover := getKeywordHoverText(word, node)
			if keywordHover != "" {
				hoverText = keywordHover
			}
		}
	}

	if hoverText == "" {
		if typ, ok := finalCtx.TypeMap[node]; ok {
			switch n := node.(type) {
			case *ast.Identifier:
				hoverText = fmt.Sprintf("```funxy\n%s: %s\n```", n.Value, PrettifyType(typ))
			default:
				// For other nodes (literals, expressions), just show the type
				hoverText = fmt.Sprintf("```funxy\n%s\n```", PrettifyType(typ))
			}
		} else {
			// Fallback: just show the node type
			switch node.(type) {
			case *ast.Identifier:
				hoverText = "```funxy\nidentifier\n```"
			case *ast.StringLiteral:
				hoverText = "```funxy\nString\n```"
			case *ast.IntegerLiteral:
				hoverText = "```funxy\nInt\n```"
			case *ast.FloatLiteral:
				hoverText = "```funxy\nFloat\n```"
			case *ast.BooleanLiteral:
				hoverText = "```funxy\nBool\n```"
			case *ast.TupleLiteral:
				hoverText = "```funxy\nTuple\n```"
			case *ast.ListLiteral:
				hoverText = "```funxy\nList\n```"
			case *ast.MapLiteral:
				hoverText = "```funxy\nMap\n```"
			case *ast.RecordLiteral:
				hoverText = "```funxy\nRecord\n```"
			case *ast.FunctionLiteral:
				hoverText = "```funxy\nFunction\n```"
			case *ast.CallExpression:
				hoverText = "```funxy\nCall\n```"
			case *ast.MemberExpression:
				hoverText = "```funxy\nField Access\n```"
			case *ast.IndexExpression:
				hoverText = "```funxy\nIndex\n```"
			default:
				hoverText = fmt.Sprintf("```funxy\n%T\n```", node)
			}
		}
	}

	hover := Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: hoverText,
		},
	}

	return s.sendResponse(ResponseMessage{
		Jsonrpc: "2.0",
		ID:      id,
		Result:  hover,
	})
}

func getKeywordHoverText(word string, node ast.Node) string {
	// Fallback to allow keywords if we are in a container node (parsing might have stopped at container)
	isContainer := false
	if node == nil {
		isContainer = true
	} else {
		switch node.(type) {
		case *ast.BlockStatement, *ast.Program:
			isContainer = true
		}
	}

	switch word {
	case "package":
		if _, ok := node.(*ast.PackageDeclaration); ok || isContainer {
			return "Keyword: package"
		}
	case "import":
		if _, ok := node.(*ast.ImportStatement); ok || isContainer {
			return "Keyword: import"
		}
	case "as":
		if _, ok := node.(*ast.ImportStatement); ok || isContainer {
			return "Keyword: as"
		}
	case "match":
		if _, ok := node.(*ast.MatchExpression); ok || isContainer {
			return "Keyword: match"
		}
	case "if":
		if _, ok := node.(*ast.IfExpression); ok || isContainer {
			return "Keyword: if"
		}
	case "else":
		if _, ok := node.(*ast.IfExpression); ok || isContainer {
			return "Keyword: else"
		}
	case "fun":
		if _, ok := node.(*ast.FunctionStatement); ok {
			return "Keyword: fun"
		}
		if _, ok := node.(*ast.FunctionLiteral); ok {
			return "Keyword: fun"
		}
		if isContainer {
			return "Keyword: fun"
		}
	case "type":
		if _, ok := node.(*ast.TypeDeclarationStatement); ok || isContainer {
			return "Keyword: type"
		}
	case "trait":
		if _, ok := node.(*ast.TraitDeclaration); ok || isContainer {
			return "Keyword: trait"
		}
	case "instance":
		if _, ok := node.(*ast.InstanceDeclaration); ok || isContainer {
			return "Keyword: instance"
		}
	case "return":
		if _, ok := node.(*ast.ReturnStatement); ok || isContainer {
			return "Keyword: return"
		}
	case "break":
		if _, ok := node.(*ast.BreakStatement); ok || isContainer {
			return "Keyword: break"
		}
	case "continue":
		if _, ok := node.(*ast.ContinueStatement); ok || isContainer {
			return "Keyword: continue"
		}
	case "for":
		if _, ok := node.(*ast.ForExpression); ok || isContainer {
			return "Keyword: for"
		}
	case "while":
		if _, ok := node.(*ast.ForExpression); ok || isContainer {
			return "Keyword: while"
		}
	case "directive":
		if _, ok := node.(*ast.DirectiveStatement); ok || isContainer {
			return "Keyword: directive"
		}
	case "alias":
		// Alias is usually TypeDeclarationStatement with IsAlias=true or similar,
		// but checking AST type might be specific.
		if _, ok := node.(*ast.TypeDeclarationStatement); ok || isContainer {
			return "Keyword: alias"
		}
		// Also allow alias to be mapped to identifier in some parse errors
		if _, ok := node.(*ast.Identifier); ok {
			return "Keyword: alias"
		}
		return "Keyword: alias"
	case "operator":
		if _, ok := node.(*ast.FunctionStatement); ok || isContainer {
			return "Keyword: operator"
		}
		// If parsed as function statement but without operator keyword (or operator keyword is the name?)
		// Actually for operator keyword, parsing usually creates FunctionStatement with Operator field set.
		if fn, ok := node.(*ast.FunctionStatement); ok && fn.Operator != "" {
			return "Keyword: operator"
		}
	case "in":
		if _, ok := node.(*ast.ForExpression); ok || isContainer {
			return "Keyword: in"
		}
		// If cursor is on 'in' but node is Identifier or something else due to partial parse
		if _, ok := node.(*ast.Identifier); ok {
			return "Keyword: in"
		}
	case "_":
		// Underscore can be Identifier (in pattern) or just token
		if _, ok := node.(*ast.Identifier); ok || isContainer {
			return "Keyword: _"
		}
		// Also match pattern if it's not an identifier
		if _, ok := node.(ast.Pattern); ok {
			return "Keyword: _"
		}
		// Sometimes _ is just an identifier literal in other contexts
		return "Keyword: _"
	case "do":
		// Do notation usually desugars to something, or is a specific expression
		// If AST doesn't have DoExpression, it might be BlockStatement or similar.
		// Assuming BlockStatement for now or just container.
		if isContainer {
			return "Keyword: do"
		}
		// Allow any node if matching word "do", assuming token is correct
		return "Keyword: do"
	case "const":
		if _, ok := node.(*ast.ConstantDeclaration); ok || isContainer {
			return "Keyword: const"
		}
		// Allow fallback
		return "Keyword: const"
	case "forall":
		// Forall usually in types
		if isContainer {
			return "Keyword: forall"
		}
		return "Keyword: forall"
	}
	return ""
}

package main

import (
	"log"
	"strings"

	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/token"
)

func (s *LanguageServer) handleCompletion(id interface{}, params CompletionParams) error {
	log.Printf("Handling completion request for %s at line %d, char %d", params.TextDocument.URI, params.Position.Line, params.Position.Character)

	// Get document state from cache
	s.mu.RLock()
	docState, exists := s.documents[params.TextDocument.URI]
	s.mu.RUnlock()

	if !exists {
		return s.sendResponse(ResponseMessage{
			Jsonrpc: "2.0",
			ID:      id,
			Result:  CompletionList{IsIncomplete: false, Items: []CompletionItem{}},
		})
	}

	// Read from cache
	docState.Mu.RLock()
	finalCtx := docState.Context
	docState.Mu.RUnlock()

	if finalCtx == nil {
		return s.sendResponse(ResponseMessage{
			Jsonrpc: "2.0",
			ID:      id,
			Result:  CompletionList{IsIncomplete: false, Items: []CompletionItem{}},
		})
	}

	// Get completion items
	items := s.getCompletionItems(finalCtx, params.Position)

	return s.sendResponse(ResponseMessage{
		Jsonrpc: "2.0",
		ID:      id,
		Result: CompletionList{
			IsIncomplete: false,
			Items:        items,
		},
	})
}

func (s *LanguageServer) getCompletionItems(ctx *pipeline.PipelineContext, position Position) []CompletionItem {
	var items []CompletionItem
	seen := make(map[string]bool)

	// Add keywords from token package
	for keyword := range token.Keywords {
		items = append(items, CompletionItem{
			Label: keyword,
			Kind:  CompletionItemKeyword,
		})
		seen[keyword] = true
	}

	// 1. Collect local symbols from the AST path
	// Convert LSP position (0-based) to AST position (1-based)
	line := position.Line + 1
	char := position.Character + 1
	path := FindNodePath(ctx.AstRoot, line, char)

	// Helper to add symbol
	addSymbol := func(name string, kind CompletionItemKind, detail string) {
		if !seen[name] {
			items = append(items, CompletionItem{
				Label:  name,
				Kind:   kind,
				Detail: detail,
			})
			seen[name] = true
		}
	}

	for _, node := range path {
		switch n := node.(type) {
		case *ast.FunctionStatement:
			for _, param := range n.Parameters {
				if param.Name != nil {
					addSymbol(param.Name.Value, CompletionItemVariable, "parameter")
				}
			}
		case *ast.BlockStatement:
			for _, stmt := range n.Statements {
				// Check if statement is defined BEFORE the cursor
				stmtToken := stmt.GetToken()
				if stmtToken.Line < line || (stmtToken.Line == line && stmtToken.Column < char) {
					if decl, ok := stmt.(*ast.ConstantDeclaration); ok {
						if decl.Name != nil {
							addSymbol(decl.Name.Value, CompletionItemVariable, "local constant")
						}
					}
					// Handle Assignment (x = 1)
					if exprStmt, ok := stmt.(*ast.ExpressionStatement); ok {
						if assign, ok := exprStmt.Expression.(*ast.AssignExpression); ok {
							if ident, ok := assign.Left.(*ast.Identifier); ok {
								addSymbol(ident.Value, CompletionItemVariable, "local variable")
							}
						}
					}
				}
			}
		}
	}

	// 2. Add global symbols from symbol table
	if ctx.SymbolTable != nil {
		allSymbols := ctx.SymbolTable.All()
		for name, symbol := range allSymbols {
			if seen[name] {
				continue
			}

			var kind CompletionItemKind
			switch symbol.Kind {
			case symbols.VariableSymbol:
				if symbol.Type != nil && strings.Contains(symbol.Type.String(), "->") {
					kind = CompletionItemFunction
				} else {
					kind = CompletionItemVariable
				}
			case symbols.TypeSymbol:
				kind = CompletionItemClass
			case symbols.TraitSymbol:
				kind = CompletionItemInterface
			case symbols.ConstructorSymbol:
				kind = CompletionItemConstructor
			default:
				kind = CompletionItemVariable
			}

			detail := ""
			if symbol.Type != nil {
				detail = symbol.Type.String()
			}

			addSymbol(name, kind, detail)
		}
	}

	// 3. Add builtins from the prelude
	if prelude := modules.GetDocPackage("prelude"); prelude != nil {
		// Functions
		for _, fn := range prelude.Functions {
			if seen[fn.Name] {
				continue
			}
			items = append(items, CompletionItem{
				Label:  fn.Name,
				Kind:   CompletionItemFunction,
				Detail: fn.Signature,
				Documentation: &MarkupContent{
					Kind:  "markdown",
					Value: fn.Description,
				},
			})
			seen[fn.Name] = true
		}

		// Types
		for _, t := range prelude.Types {
			if seen[t.Name] {
				continue
			}
			items = append(items, CompletionItem{
				Label:  t.Name,
				Kind:   CompletionItemClass,
				Detail: t.Signature,
				Documentation: &MarkupContent{
					Kind:  "markdown",
					Value: t.Description,
				},
			})
			seen[t.Name] = true
		}

		// Traits
		for _, t := range prelude.Traits {
			// Extract name from "Trait<T>"
			name := t.Name
			if idx := strings.Index(name, "<"); idx != -1 {
				name = name[:idx]
			}
			if seen[name] {
				continue
			}
			items = append(items, CompletionItem{
				Label:  name,
				Kind:   CompletionItemInterface,
				Detail: t.Signature,
				Documentation: &MarkupContent{
					Kind:  "markdown",
					Value: t.Description,
				},
			})
			seen[name] = true
		}
	}

	return items
}

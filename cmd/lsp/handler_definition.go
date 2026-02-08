package main

import (
	"log"
	"strings"

	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

func (s *LanguageServer) handleDefinition(id interface{}, params DefinitionParams) error {
	log.Printf("Handling definition request for %s at line %d, char %d", params.TextDocument.URI, params.Position.Line, params.Position.Character)

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
	finalCtx := docState.Context
	docState.Mu.RUnlock()

	if finalCtx == nil {
		return s.sendResponse(ResponseMessage{
			Jsonrpc: "2.0",
			ID:      id,
			Result:  nil,
		})
	}

	// Even if errors exist, try to resolve

	// Find the node path at the given position
	path := FindNodePath(finalCtx.AstRoot, params.Position.Line+1, params.Position.Character+1)
	if len(path) == 0 {
		return s.sendResponse(ResponseMessage{
			Jsonrpc: "2.0",
			ID:      id,
			Result:  nil,
		})
	}
	node := path[len(path)-1]

	// Check if it's an identifier
	ident, ok := node.(*ast.Identifier)
	if !ok {
		return s.sendResponse(ResponseMessage{
			Jsonrpc: "2.0",
			ID:      id,
			Result:  nil,
		})
	}

	var symbol symbols.Symbol
	var found bool

	// 1. Try ResolutionMap (exact match for locals, args, etc.)
	if finalCtx.ResolutionMap != nil {
		symbol, found = finalCtx.ResolutionMap[ident]
		if !found {
			// Try map iteration if pointer equality failed but content matches
			symbol, found = finalCtx.ResolutionMap[node]
		}
	}

	// 2. Fallback: Look up in global symbol table
	if !found {
		symbol, found = finalCtx.SymbolTable.Find(ident.Value)
	}

	// 3. Fallback: Member Resolution (Type-based)
	if !found && len(path) >= 2 {
		if memExpr, ok := path[len(path)-2].(*ast.MemberExpression); ok && memExpr.Member == ident {
			if leftType, hasType := finalCtx.TypeMap[memExpr.Left]; hasType {
				// Unwrap type to find the nominal type (TCon)
				var typeName string
				switch t := leftType.(type) {
				case typesystem.TCon:
					typeName = t.Name
				case typesystem.TApp:
					if tCon, ok := t.Constructor.(typesystem.TCon); ok {
						typeName = tCon.Name
					}
				}

				if typeName != "" {
					typeSym, typeFound := finalCtx.SymbolTable.Find(typeName)
					if typeFound {
						// Jump to the Type definition (fallback for field access)
						symbol = typeSym
						found = true
					}
				}
			}
		}
	}

	if !found {
		return s.sendResponse(ResponseMessage{
			Jsonrpc: "2.0",
			ID:      id,
			Result:  nil,
		})
	}

	// Check if the symbol is defined in the current package
	currentPackage := ""
	if prog, ok := finalCtx.AstRoot.(*ast.Program); ok {
		if prog.Package != nil {
			currentPackage = prog.Package.Name.Value
		}
	}

	// If OriginModule is set and differs from current package, it's external.
	if symbol.OriginModule != "" && symbol.OriginModule != currentPackage {
		return s.sendResponse(ResponseMessage{
			Jsonrpc: "2.0",
			ID:      id,
			Result:  nil,
		})
	}

	// Get the location from the symbol's definition node
	if symbol.DefinitionNode == nil {
		return s.sendResponse(ResponseMessage{
			Jsonrpc: "2.0",
			ID:      id,
			Result:  nil,
		})
	}

	startPos, endPos, ok := getNodePosition(symbol.DefinitionNode)
	if !ok {
		return s.sendResponse(ResponseMessage{
			Jsonrpc: "2.0",
			ID:      id,
			Result:  nil,
		})
	}

	// Use DefinitionFile if available, otherwise fallback to current URI
	defURI := params.TextDocument.URI
	if symbol.DefinitionFile != "" {
		if !strings.HasPrefix(symbol.DefinitionFile, "file://") {
			defURI = "file://" + symbol.DefinitionFile
		} else {
			defURI = symbol.DefinitionFile
		}
	}

	location := Location{
		URI: defURI,
		Range: Range{
			Start: Position{
				Line:      startPos.Line - 1,
				Character: startPos.Column - 1,
			},
			End: Position{
				Line:      endPos.Line - 1,
				Character: endPos.Column - 1,
			},
		},
	}

	return s.sendResponse(ResponseMessage{
		Jsonrpc: "2.0",
		ID:      id,
		Result:  location,
	})
}

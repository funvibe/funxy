package modules

import (
	"path/filepath"
	"sort"

	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

// Module represents a loaded package consisting of multiple source files.
type Module struct {
	Name        string
	Dir         string
	Files       []*ast.Program
	SymbolTable *symbols.SymbolTable
	Exports     map[string]bool              // Set of exported symbol names (local + resolved re-exports)
	Imports     map[string]*Module           // Map alias/name -> Module
	TypeMap     map[ast.Node]typesystem.Type // Type inference results
	IsVirtual   bool                         // True if this is a virtual (built-in) package

	// Re-export specifications from package declaration
	// Stored during loading, resolved during analysis
	ReexportSpecs []*ast.ExportSpec

	// Trait default implementations found during analysis
	TraitDefaults map[string]*ast.FunctionStatement

	// Evaluator-specific data
	ClassImplementations map[string]map[string]interface{} // Runtime trait implementations

	// Package group support (import "dir" imports all dir/*)
	IsPackageGroup bool     // True if this is a combined package from subdirectories
	SubPackages    []string // Names of sub-packages (e.g., ["utils", "helpers"])

	HeadersAnalyzed  bool
	HeadersAnalyzing bool
	BodiesAnalyzed   bool
	BodiesAnalyzing  bool
}

// GetExports returns a map of exported symbol names to their runtime values.
func (m *Module) GetExports() map[string]symbols.Symbol {
	exportedSymbols := make(map[string]symbols.Symbol)

	if m.IsPackageGroup {
		// For package groups, look up symbols in sub-modules
		for name := range m.Exports {
			// Find which sub-module exports this symbol
			for _, subMod := range m.Imports {
				if subMod.Exports[name] {
					if sym, ok := subMod.SymbolTable.Find(name); ok {
						exportedSymbols[name] = sym
						break // Found it
					}
				}
			}
		}
	} else {
		// Regular module
		for name := range m.Exports {
			if sym, ok := m.SymbolTable.Find(name); ok {
				exportedSymbols[name] = sym
			}
		}
	}
	return exportedSymbols
}

func (m *Module) GetName() string {
	return m.Name
}

func (m *Module) GetExportedSymbols() map[string]typesystem.Type {
	return nil // Deprecated
}

func (m *Module) IsHeadersAnalyzed() bool {
	return m.HeadersAnalyzed
}

func (m *Module) SetHeadersAnalyzed(v bool) {
	m.HeadersAnalyzed = v
}

func (m *Module) IsHeadersAnalyzing() bool {
	return m.HeadersAnalyzing
}

func (m *Module) SetHeadersAnalyzing(v bool) {
	m.HeadersAnalyzing = v
}

func (m *Module) IsBodiesAnalyzed() bool {
	return m.BodiesAnalyzed
}

func (m *Module) SetBodiesAnalyzed(v bool) {
	m.BodiesAnalyzed = v
}

func (m *Module) IsBodiesAnalyzing() bool {
	return m.BodiesAnalyzing
}

func (m *Module) SetBodiesAnalyzing(v bool) {
	m.BodiesAnalyzing = v
}

func (m *Module) GetFiles() []*ast.Program {
	return m.Files
}

func (m *Module) GetSymbolTable() *symbols.SymbolTable {
	return m.SymbolTable
}

func (m *Module) SetTypeMap(tm map[ast.Node]typesystem.Type) {
	m.TypeMap = tm
}

func (m *Module) IsPackageGroupModule() bool {
	return m.IsPackageGroup
}

func (m *Module) GetSubModulesRaw() map[string]interface{} {
	subs := make(map[string]interface{})
	for k, v := range m.Imports {
		subs[k] = v
	}
	return subs
}

func (m *Module) GetReexportSpecs() []*ast.ExportSpec {
	return m.ReexportSpecs
}

func (m *Module) AddExport(name string) {
	m.Exports[name] = true
}

func (m *Module) SetTraitDefaults(defaults map[string]*ast.FunctionStatement) {
	m.TraitDefaults = defaults
}

func (m *Module) GetTraitDefaults() map[string]*ast.FunctionStatement {
	return m.TraitDefaults
}

// OrderedFiles returns module files in a dependency-aware order with the entry
// file (pkgname.ext) last. This avoids runtime order issues for cross-file
// constants and top-level expressions.
func (m *Module) OrderedFiles() []*ast.Program {
	if m == nil || len(m.Files) == 0 || m.Name == "" {
		return m.Files
	}

	var entry *ast.Program
	nonEntry := make([]*ast.Program, 0, len(m.Files))
	for _, file := range m.Files {
		if file == nil || file.File == "" {
			nonEntry = append(nonEntry, file)
			continue
		}
		base := filepath.Base(file.File)
		if filepath.Ext(base) != "" && base == m.Name+filepath.Ext(base) {
			entry = file
			continue
		}
		nonEntry = append(nonEntry, file)
	}

	ordered := orderByTopLevelDeps(nonEntry)
	if entry != nil {
		ordered = append(ordered, entry)
	}
	return ordered
}

func orderByTopLevelDeps(files []*ast.Program) []*ast.Program {
	if len(files) <= 1 {
		return files
	}

	type fileInfo struct {
		file     *ast.Program
		provides map[string]bool
		deps     map[string]bool
		index    int
	}

	infos := make([]*fileInfo, 0, len(files))
	providers := make(map[string]int)
	for i, file := range files {
		info := &fileInfo{
			file:     file,
			provides: collectProvides(file),
			deps:     collectTopLevelDeps(file),
			index:    i,
		}
		infos = append(infos, info)
		for name := range info.provides {
			if _, ok := providers[name]; !ok {
				providers[name] = i
			}
		}
	}

	edges := make([][]int, len(infos))
	inDegree := make([]int, len(infos))
	for i, info := range infos {
		for dep := range info.deps {
			if providerIdx, ok := providers[dep]; ok && providerIdx != i {
				edges[providerIdx] = append(edges[providerIdx], i)
				inDegree[i]++
			}
		}
	}

	queue := make([]int, 0, len(infos))
	for i := range infos {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}
	sort.Slice(queue, func(i, j int) bool {
		return infos[queue[i]].index < infos[queue[j]].index
	})

	ordered := make([]*ast.Program, 0, len(infos))
	for len(queue) > 0 {
		idx := queue[0]
		queue = queue[1:]
		ordered = append(ordered, infos[idx].file)
		for _, next := range edges[idx] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
				sort.Slice(queue, func(i, j int) bool {
					return infos[queue[i]].index < infos[queue[j]].index
				})
			}
		}
	}

	if len(ordered) != len(infos) {
		// Cycle or unresolved deps: fall back to original order.
		ordered = ordered[:0]
		for _, info := range infos {
			ordered = append(ordered, info.file)
		}
	}
	return ordered
}

func collectProvides(file *ast.Program) map[string]bool {
	provides := make(map[string]bool)
	if file == nil {
		return provides
	}
	for _, stmt := range file.Statements {
		switch s := stmt.(type) {
		case *ast.ConstantDeclaration:
			if s.Name != nil {
				provides[s.Name.Value] = true
			}
		case *ast.FunctionStatement:
			if s.Name != nil {
				provides[s.Name.Value] = true
			}
		case *ast.TypeDeclarationStatement:
			if s.Name != nil {
				provides[s.Name.Value] = true
			}
		case *ast.ExpressionStatement:
			if assign, ok := s.Expression.(*ast.AssignExpression); ok {
				if ident, ok := assign.Left.(*ast.Identifier); ok {
					provides[ident.Value] = true
				}
			}
		}
	}
	return provides
}

func collectTopLevelDeps(file *ast.Program) map[string]bool {
	deps := make(map[string]bool)
	if file == nil {
		return deps
	}
	for _, stmt := range file.Statements {
		switch s := stmt.(type) {
		case *ast.ConstantDeclaration:
			collectExprDeps(s.Value, deps)
		case *ast.ExpressionStatement:
			collectExprDeps(s.Expression, deps)
		}
	}
	return deps
}

func collectExprDeps(expr ast.Expression, deps map[string]bool) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.Identifier:
		deps[e.Value] = true
	case *ast.AssignExpression:
		collectExprDeps(e.Value, deps)
	case *ast.PatternAssignExpression:
		collectExprDeps(e.Value, deps)
	case *ast.InfixExpression:
		collectExprDeps(e.Left, deps)
		collectExprDeps(e.Right, deps)
	case *ast.PrefixExpression:
		collectExprDeps(e.Right, deps)
	case *ast.PostfixExpression:
		collectExprDeps(e.Left, deps)
	case *ast.CallExpression:
		collectExprDeps(e.Function, deps)
		for _, arg := range e.Arguments {
			collectExprDeps(arg, deps)
		}
	case *ast.TypeApplicationExpression:
		collectExprDeps(e.Expression, deps)
	case *ast.IndexExpression:
		collectExprDeps(e.Left, deps)
		collectExprDeps(e.Index, deps)
	case *ast.MemberExpression:
		collectExprDeps(e.Left, deps)
	case *ast.TupleLiteral:
		for _, el := range e.Elements {
			collectExprDeps(el, deps)
		}
	case *ast.ListLiteral:
		for _, el := range e.Elements {
			collectExprDeps(el, deps)
		}
	case *ast.RecordLiteral:
		for _, val := range e.Fields {
			collectExprDeps(val, deps)
		}
	case *ast.MapLiteral:
		for _, pair := range e.Pairs {
			collectExprDeps(pair.Key, deps)
			collectExprDeps(pair.Value, deps)
		}
	case *ast.RangeExpression:
		collectExprDeps(e.Start, deps)
		collectExprDeps(e.End, deps)
	case *ast.AnnotatedExpression:
		collectExprDeps(e.Expression, deps)
	case *ast.BlockStatement:
		for _, stmt := range e.Statements {
			if es, ok := stmt.(*ast.ExpressionStatement); ok {
				collectExprDeps(es.Expression, deps)
			} else if cd, ok := stmt.(*ast.ConstantDeclaration); ok {
				collectExprDeps(cd.Value, deps)
			}
		}
	case *ast.IfExpression:
		collectExprDeps(e.Condition, deps)
		collectExprDeps(e.Consequence, deps)
		if e.Alternative != nil {
			collectExprDeps(e.Alternative, deps)
		}
	case *ast.MatchExpression:
		collectExprDeps(e.Expression, deps)
		for _, arm := range e.Arms {
			if arm.Guard != nil {
				collectExprDeps(arm.Guard, deps)
			}
			collectExprDeps(arm.Expression, deps)
		}
	case *ast.ListComprehension:
		collectExprDeps(e.Output, deps)
		for _, clause := range e.Clauses {
			switch c := clause.(type) {
			case *ast.CompFilter:
				collectExprDeps(c.Condition, deps)
			case *ast.CompGenerator:
				collectExprDeps(c.Iterable, deps)
			}
		}
	case *ast.FunctionLiteral:
		// Skip function body to avoid ordering dependencies.
		return
	case *ast.InterpolatedString:
		for _, part := range e.Parts {
			collectExprDeps(part, deps)
		}
	}
}

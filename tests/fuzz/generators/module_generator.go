package generators

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

// ModuleGenerator generates a random module structure on disk.
type ModuleGenerator struct {
	src     RandomSource
	rootDir string
	modules []string // List of generated module names (paths relative to root)
}

func NewModuleGenerator(seed int64, rootDir string) *ModuleGenerator {
	return &ModuleGenerator{
		src:     &RandSource{rand.New(rand.NewSource(seed))},
		rootDir: rootDir,
		modules: []string{},
	}
}

func (g *ModuleGenerator) GenerateModules(count int) error {
	// Create root module (main)
	if err := g.createModule("main", true); err != nil {
		return err
	}
	g.modules = append(g.modules, "main")

	// Create other modules
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("mod_%d", i)
		// 20% chance to be a sub-package
		if len(g.modules) > 1 && g.src.Intn(5) == 0 {
			parent := g.modules[g.src.Intn(len(g.modules))]
			name = filepath.Join(parent, fmt.Sprintf("sub_%d", i))
		}

		if err := g.createModule(name, false); err != nil {
			return err
		}
		g.modules = append(g.modules, name)
	}
	return nil
}

func (g *ModuleGenerator) createModule(name string, isMain bool) error {
	dirPath := filepath.Join(g.rootDir, name)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return err
	}

	// Generate 1-3 files per module
	fileCount := g.src.Intn(3) + 1
	for i := 0; i < fileCount; i++ {
		fileName := fmt.Sprintf("file_%d.lang", i)
		if i == 0 {
			// Main file matches directory name usually, but let's stick to simple names
			fileName = filepath.Base(name) + ".lang"
		}

		content := g.generateModuleContent(name, isMain)
		if err := os.WriteFile(filepath.Join(dirPath, fileName), []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}

func (g *ModuleGenerator) generateModuleContent(pkgName string, isMain bool) string {
	var sb strings.Builder

	// Package declaration
	baseName := filepath.Base(pkgName)
	if isMain {
		sb.WriteString("package main\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("package %s\n\n", baseName))
	}

	// Imports
	// Pick random existing modules to import
	if len(g.modules) > 0 {
		importCount := g.src.Intn(3)
		for i := 0; i < importCount; i++ {
			target := g.modules[g.src.Intn(len(g.modules))]
			if target != pkgName {
				// Calculate relative path or use absolute?
				// Loader supports relative paths.
				// Let's use relative path from root for simplicity, assuming root is in include path?
				// Or relative to current file.

				// For fuzzing, let's try to generate valid relative imports
				rel, err := filepath.Rel(pkgName, target)
				if err == nil {
					// Ensure it starts with ./ or ../ if it's relative
					if !strings.HasPrefix(rel, ".") {
						rel = "./" + rel
					}
					sb.WriteString(g.generateImport(rel))
				}
			}
		}
	}
	sb.WriteString("\n")

	// Generate some code
	gen := New(int64(g.src.Intn(1000)))
	sb.WriteString(gen.GenerateProgram())

	return sb.String()
}

// generateImport generates an import statement with random formatting variants:
//   - plain:      import "path"
//   - selective:  import "path" (a, b, c)
//   - multiline:  import "path" (a,\n  b,\n  c)
//   - nl-paren:   import "path" (\n  a, b\n)
//   - wildcard:   import "path" (*)
//   - alias:      import "path" as foo
func (g *ModuleGenerator) generateImport(path string) string {
	symbols := []string{"foo", "bar", "baz", "qux", "quux", "corge", "grault"}

	variant := g.src.Intn(6)
	switch variant {
	case 0: // plain
		return fmt.Sprintf("import \"%s\"\n", path)
	case 1: // selective, single line
		n := g.src.Intn(4) + 1
		syms := symbols[:n]
		return fmt.Sprintf("import \"%s\" (%s)\n", path, strings.Join(syms, ", "))
	case 2: // multiline: each symbol on its own line after comma
		n := g.src.Intn(4) + 2
		syms := symbols[:n]
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("import \"%s\" (%s,\n", path, syms[0]))
		for i := 1; i < len(syms)-1; i++ {
			sb.WriteString(fmt.Sprintf("    %s,\n", syms[i]))
		}
		sb.WriteString(fmt.Sprintf("    %s)\n", syms[len(syms)-1]))
		return sb.String()
	case 3: // newline after opening paren
		n := g.src.Intn(3) + 2
		syms := symbols[:n]
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("import \"%s\" (\n", path))
		sb.WriteString(fmt.Sprintf("    %s\n)\n", strings.Join(syms, ", ")))
		return sb.String()
	case 4: // wildcard
		return fmt.Sprintf("import \"%s\" (*)\n", path)
	case 5: // alias
		alias := fmt.Sprintf("m%d", g.src.Intn(100))
		return fmt.Sprintf("import \"%s\" as %s\n", path, alias)
	default:
		return fmt.Sprintf("import \"%s\"\n", path)
	}
}

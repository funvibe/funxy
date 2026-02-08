package generators

import (
	"flag"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var _ = flag.Bool("tree", false, "ignored")

func TestGenerator_GenerateProgram(t *testing.T) {
	// Test with a fixed seed for reproducibility
	gen := New(12345)
	code := gen.GenerateProgram()

	if len(code) == 0 {
		t.Error("Generated code is empty")
	}

	// Verify that the generated code is syntactically valid
	ctx := pipeline.NewPipelineContext(code)
	l := lexer.New(code)
	stream := lexer.NewTokenStream(l)
	p := parser.New(stream, ctx)
	program := p.ParseProgram()

	if len(ctx.Errors) > 0 {
		t.Errorf("Generated code has syntax errors:\n%s\nErrors:\n%v", code, ctx.Errors)
	}

	if program == nil {
		t.Error("Parsed program is nil")
	}
}

func TestGenerator_Determinism(t *testing.T) {
	// Same seed should produce same code
	gen1 := New(12345)
	code1 := gen1.GenerateProgram()

	gen2 := New(12345)
	code2 := gen2.GenerateProgram()

	if code1 != code2 {
		t.Error("Generator is not deterministic with same seed")
	}
}

func TestGenerator_FromData(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	gen1 := NewFromData(data)
	code1 := gen1.GenerateProgram()

	gen2 := NewFromData(data)
	code2 := gen2.GenerateProgram()

	if code1 != code2 {
		t.Error("Generator is not deterministic with same data")
	}

	if len(code1) == 0 {
		t.Error("Generated code from data is empty")
	}
}

func TestGenerator_Features(t *testing.T) {
	// Generate enough code to likely cover most features
	gen := New(999)
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString(gen.GenerateProgram())
		sb.WriteString("\n")
	}
	code := sb.String()

	features := []string{
		"fun ",
		"if ",
		"match ",
		"type ",
		"trait ",
		"instance ",
		"{",
		"}",
		"const ",
	}

	for _, feature := range features {
		if !strings.Contains(code, feature) {
			t.Logf("Warning: Generated code might not contain feature '%s' (could be random chance)", feature)
		}
	}
}

func TestBytecodeGenerator_GenerateChunk(t *testing.T) {
	gen := NewBytecodeGenerator([]byte{1, 2, 3, 4, 5})
	chunk := gen.GenerateChunk()

	if chunk == nil {
		t.Error("Generated chunk is nil")
	}

	if len(chunk.Code) == 0 {
		t.Error("Generated chunk has no code")
	}
}

func TestStdLibGenerator_GenerateStdLibProgram(t *testing.T) {
	gen := NewStdLibGenerator(12345)
	code := gen.GenerateStdLibProgram()

	if len(code) == 0 {
		t.Error("Generated stdlib code is empty")
	}

	// Should contain some stdlib calls
	keywords := []string{"List.", "Map.", "String."}
	found := false
	for _, kw := range keywords {
		if strings.Contains(code, kw) {
			found = true
			break
		}
	}
	if !found {
		t.Log("Warning: Generated stdlib code might not contain expected keywords (random chance)")
	}
}

func TestModuleGenerator_GenerateModules(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewModuleGenerator(12345, tmpDir)

	err := gen.GenerateModules(3)
	if err != nil {
		t.Fatalf("Failed to generate modules: %v", err)
	}

	// Check if files were created
	// We expect main module + 3 generated modules
	// But structure is random, so let's just check if tmpDir is not empty
	// and contains some .lang files
	foundLang := false
	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".lang") {
			foundLang = true
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk tmp dir: %v", err)
	}

	if !foundLang {
		t.Error("No .lang files generated")
	}
}

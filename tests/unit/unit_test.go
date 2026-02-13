package unit

import (
	"flag"
	"fmt"
	"os"
	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/backend"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"path/filepath"
	"strings"
	"testing"
)

var useTreeWalk = flag.Bool("tree", false, "run tests with tree-walk backend")

// isTestFile checks if filename matches *_test.{lang,funxy,fx}
func isTestFile(name string) bool {
	for _, ext := range config.SourceFileExtensions {
		if strings.HasSuffix(name, "_test"+ext) {
			return true
		}
	}
	return false
}

// trimTestExt removes _test.{lang,funxy,fx} extension from filename
func trimTestExt(name string) string {
	for _, ext := range config.SourceFileExtensions {
		suffix := "_test" + ext
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSuffix(name, suffix)
		}
	}
	return name
}

// TestUnitTests recursively finds and runs all test files in tests/unit
func TestUnitTests(t *testing.T) {
	// Change working directory to project root so imports like "kit/..." work
	// tests/unit -> ../.. -> project root
	wd, _ := os.Getwd()
	if !strings.HasSuffix(wd, "parser") { // Avoid double chdir if running from root
		if err := os.Chdir("../.."); err != nil {
			t.Fatalf("Failed to change working directory to project root: %v", err)
		}
	}

	// Initialize virtual packages
	modules.InitVirtualPackages()

	// Find all *_test.{lang,funxy,fx} files recursively in tests/unit
	// Since we are now in root, we walk "tests/unit"
	var testFiles []string
	err := filepath.Walk("tests/unit", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && isTestFile(info.Name()) {
			testFiles = append(testFiles, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk directory: %v", err)
	}

	if len(testFiles) == 0 {
		t.Skip("No test files found in tests/unit")
	}

	for _, file := range testFiles {
		file := file // capture for closure
		testName := trimTestExt(filepath.Base(file))

		t.Run(testName, func(t *testing.T) {
			runLangTest(t, file)
		})
	}
}

// runLangTest runs a single .lang test file using the unified pipeline
func runLangTest(t *testing.T, filePath string) {
	// Set test mode flag for type normalization
	config.IsTestMode = true

	// Get absolute path for proper module resolution
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		t.Fatalf("Failed to get absolute path for %s: %v", filePath, err)
	}

	// Read source file
	sourceCode, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", filePath, err)
	}

	// Initialize test runner
	// Note: We use nil evaluator here because VM handles execution
	evaluator.InitTestRunner(nil)

	// Create pipeline context
	ctx := pipeline.NewPipelineContext(string(sourceCode))
	ctx.FilePath = absPath
	ctx.IsTestMode = true // Enable test mode

	// Select backend (default to VM, as unit tests should test the main runtime)
	var execBackend backend.Backend
	if *useTreeWalk {
		execBackend = backend.NewTreeWalk()
	} else {
		execBackend = backend.NewVM()
	}

	// Create pipeline
	processingPipeline := pipeline.New(
		&lexer.LexerProcessor{},
		&parser.ParserProcessor{},
		&analyzer.SemanticAnalyzerProcessor{},
		backend.NewExecutionProcessor(execBackend),
	)

	// Run pipeline
	finalCtx := processingPipeline.Run(ctx)

	// Check for errors (lexer, parser, analyzer, runtime)
	if len(finalCtx.Errors) > 0 {
		var errMsg strings.Builder
		errMsg.WriteString("Processing errors:\n")
		for _, e := range finalCtx.Errors {
			errMsg.WriteString(fmt.Sprintf("- %s\n", e.Error()))
		}
		t.Fatalf("%s", errMsg.String())
	}

	// Check test results from the test runner
	results := evaluator.GetTestResults()
	failed := false
	var failureMsg strings.Builder

	for _, r := range results {
		if !r.Passed {
			failed = true
			failureMsg.WriteString(fmt.Sprintf("✗ %s: %s\n", r.Name, r.Error))
		}
	}

	if failed {
		t.Errorf("Tests failed:\n%s", failureMsg.String())
	}
}

package vm_test

import (
	"io/ioutil"
	"os"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/pkg/embed"
	"github.com/funvibe/funxy/internal/vm"
	"path/filepath"
	"testing"
)

func TestRunBytecode(t *testing.T) {
	// Setup temp dir
	tmpDir, err := ioutil.TempDir("", "funxy_run_bytecode_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	bundle := &vm.Bundle{
		MainChunk:     vm.NewChunk(),
		Modules:       make(map[string]*vm.BundledModule),
		TraitDefaults: make(map[string]*vm.CompiledFunction),
	}
	// Add a simple return string instruction
	// Return "Bytecode result"

	// WriteConstant writes OP_CONST and the 2-byte index.
	bundle.MainChunk.WriteConstant(&evaluator.Integer{Value: 42}, 1)
	bundle.MainChunk.Write(byte(vm.OP_RETURN), 1)

	fbcPath := filepath.Join(tmpDir, "script.fbc")
	bundleData, err := bundle.Serialize()
	if err != nil {
		t.Fatalf("serialize error: %v", err)
	}
	if err := ioutil.WriteFile(fbcPath, bundleData, 0644); err != nil {
		t.Fatal(err)
	}

	callerCode2 := `
import "lib/io" (runBytecode)
	res = runBytecode("` + filepath.ToSlash(fbcPath) + `")
	match res {
		Ok(val) -> val
		Fail(err) -> err
	}
	`

	v := funxy.New()
	v.AllowModule("lib/io") // runBytecode is in lib/io (requires capability in sandbox)
	res2, err := v.Eval(callerCode2)
	if err != nil {
		t.Fatalf("runBytecode match failed: %v", err)
	}

	// The result is the string representation of the bytecode evaluation.
	// Since we returned 42, it will be "42".
	resStr, ok := res2.(string)
	if !ok {
		t.Fatalf("expected string result, got %T: %v", res2, res2)
	}

	if resStr != "42" {
		t.Errorf("Expected \"42\", got %s", resStr)
	}
}

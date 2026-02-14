package evaluator

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestBuild tests the `funxy build` command that creates self-contained binaries.
// This covers: compilation to bundle, self-contained binary creation, and execution.
func TestBuild(t *testing.T) {
	// Get project root (parent of evaluator/)
	projectRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	binaryPath := filepath.Join(projectRoot, "funxy-build-test-binary")
	defer os.Remove(binaryPath)

	// Build fresh funxy binary
	t.Log("Building fresh funxy binary for build tests...")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/funxy")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\n%s", err, output)
	}

	tmpDir := t.TempDir()

	// ==========================================
	// Test: funxy build <script> — basic
	// ==========================================
	t.Run("build simple script", func(t *testing.T) {
		script := filepath.Join(tmpDir, "hello.lang")
		writeFile(t, script, `print("Hello from build!")`)

		outputBin := filepath.Join(tmpDir, "hello")

		// Build
		runCmd(t, binaryPath, projectRoot, nil, "build", script, "-o", outputBin)

		// Verify binary exists
		info, err := os.Stat(outputBin)
		if err != nil {
			t.Fatalf("Binary not created: %v", err)
		}
		if info.Size() < 1024 {
			t.Errorf("Binary too small: %d bytes", info.Size())
		}

		// Run and check output
		got := runCmd(t, outputBin, tmpDir, nil)
		if got != "Hello from build!" {
			t.Errorf("Output mismatch: want %q, got %q", "Hello from build!", got)
		}
	})

	// ==========================================
	// Test: build with virtual imports
	// ==========================================
	t.Run("build with virtual imports", func(t *testing.T) {
		script := filepath.Join(tmpDir, "imports.lang")
		writeFile(t, script, `
import "lib/json" (jsonEncode)
import "lib/string" (stringToUpper)

print(stringToUpper("hello"))
print(jsonEncode(42))
`)
		outputBin := filepath.Join(tmpDir, "imports")

		runCmd(t, binaryPath, projectRoot, nil, "build", script, "-o", outputBin)

		got := runCmd(t, outputBin, tmpDir, nil)
		if got != "HELLO\n42" {
			t.Errorf("Output mismatch: want %q, got %q", "HELLO\n42", got)
		}
	})

	// ==========================================
	// Test: build with pipes and lambdas
	// ==========================================
	t.Run("build with pipes and lambdas", func(t *testing.T) {
		script := filepath.Join(tmpDir, "pipes.lang")
		writeFile(t, script, `
import "lib/list" (map, filter)

result = [1, 2, 3, 4, 5] |> map(\x -> x * x) |> filter(\x -> x > 5)
print(result)
`)
		outputBin := filepath.Join(tmpDir, "pipes")

		runCmd(t, binaryPath, projectRoot, nil, "build", script, "-o", outputBin)

		got := runCmd(t, outputBin, tmpDir, nil)
		if got != "[9, 16, 25]" {
			t.Errorf("Output mismatch: want %q, got %q", "[9, 16, 25]", got)
		}
	})

	// ==========================================
	// Test: build with functions and recursion
	// ==========================================
	t.Run("build with functions", func(t *testing.T) {
		script := filepath.Join(tmpDir, "functions.lang")
		writeFile(t, script, `
factorial = \n -> match n {
    0 -> 1
    _ -> n * factorial(n - 1)
}
print(factorial(10))
`)
		outputBin := filepath.Join(tmpDir, "functions")

		runCmd(t, binaryPath, projectRoot, nil, "build", script, "-o", outputBin)

		got := runCmd(t, outputBin, tmpDir, nil)
		if got != "3628800" {
			t.Errorf("Output mismatch: want %q, got %q", "3628800", got)
		}
	})

	// ==========================================
	// Test: build with records
	// ==========================================
	t.Run("build with records", func(t *testing.T) {
		script := filepath.Join(tmpDir, "records.lang")
		writeFile(t, script, `
import "lib/json" (jsonEncode)

person = { name: "Alice", age: 30 }
print(jsonEncode(person))
`)
		outputBin := filepath.Join(tmpDir, "records")

		runCmd(t, binaryPath, projectRoot, nil, "build", script, "-o", outputBin)

		got := runCmd(t, outputBin, tmpDir, nil)
		// JSON output may have different key ordering
		if !strings.Contains(got, `"name":"Alice"`) || !strings.Contains(got, `"age":30`) {
			t.Errorf("Output mismatch, got: %q", got)
		}
	})

	// ==========================================
	// Test: compile to .fbc (v2 bundle) and run
	// ==========================================
	t.Run("compile to fbc and run", func(t *testing.T) {
		script := filepath.Join(tmpDir, "compile_test.lang")
		writeFile(t, script, `print("compiled OK")`)

		// Compile
		runCmd(t, binaryPath, projectRoot, nil, "-c", script)

		fbcPath := filepath.Join(tmpDir, "compile_test.fbc")
		if _, err := os.Stat(fbcPath); err != nil {
			t.Fatalf(".fbc file not created: %v", err)
		}

		// Run compiled
		got := runCmd(t, binaryPath, projectRoot, nil, "-r", fbcPath)
		if got != "compiled OK" {
			t.Errorf("Output mismatch: want %q, got %q", "compiled OK", got)
		}
	})

	// ==========================================
	// Test: compile with imports to .fbc and run (v2 bundle format)
	// ==========================================
	t.Run("compile with imports to fbc and run", func(t *testing.T) {
		script := filepath.Join(tmpDir, "compile_imports.lang")
		writeFile(t, script, `
import "lib/string" (stringToUpper, stringToLower)

print(stringToUpper("hello"))
print(stringToLower("WORLD"))
`)
		// Compile
		runCmd(t, binaryPath, projectRoot, nil, "-c", script)

		fbcPath := filepath.Join(tmpDir, "compile_imports.fbc")
		got := runCmd(t, binaryPath, projectRoot, nil, "-r", fbcPath)
		if got != "HELLO\nworld" {
			t.Errorf("Output mismatch: want %q, got %q", "HELLO\nworld", got)
		}
	})

	// ==========================================
	// Test: default output path (no -o flag)
	// ==========================================
	t.Run("build default output path", func(t *testing.T) {
		script := filepath.Join(tmpDir, "default_out.lang")
		writeFile(t, script, `print("default path")`)

		// Build without -o
		runCmd(t, binaryPath, projectRoot, nil, "build", script)

		defaultOutput := filepath.Join(tmpDir, "default_out")
		defer os.Remove(defaultOutput)

		if _, err := os.Stat(defaultOutput); err != nil {
			t.Fatalf("Default output binary not created: %v", err)
		}

		got := runCmd(t, defaultOutput, tmpDir, nil)
		if got != "default path" {
			t.Errorf("Output mismatch: want %q, got %q", "default path", got)
		}
	})

	// ==========================================
	// Test: self-contained binary ignores CLI args meant for funxy
	// ==========================================
	t.Run("built binary ignores funxy flags", func(t *testing.T) {
		script := filepath.Join(tmpDir, "args_test.lang")
		writeFile(t, script, `print("no flags")`)

		outputBin := filepath.Join(tmpDir, "args_test")
		runCmd(t, binaryPath, projectRoot, nil, "build", script, "-o", outputBin)

		// Running with extra args should still work
		got := runCmd(t, outputBin, tmpDir, nil, "some", "args")
		if got != "no flags" {
			t.Errorf("Output mismatch: want %q, got %q", "no flags", got)
		}
	})

	// ==========================================
	// Test: build binary size is reasonable
	// ==========================================
	t.Run("build binary size reasonable", func(t *testing.T) {
		script := filepath.Join(tmpDir, "size_test.lang")
		writeFile(t, script, `print(1)`)

		outputBin := filepath.Join(tmpDir, "size_test")
		runCmd(t, binaryPath, projectRoot, nil, "build", script, "-o", outputBin)

		info, err := os.Stat(outputBin)
		if err != nil {
			t.Fatalf("Binary not created: %v", err)
		}

		hostInfo, err := os.Stat(binaryPath)
		if err != nil {
			t.Fatalf("Host binary not found: %v", err)
		}

		// Self-contained binary should be slightly larger than host binary
		// (host + bundle + footer overhead)
		overhead := info.Size() - hostInfo.Size()
		if overhead < 0 {
			t.Errorf("Self-contained binary (%d bytes) smaller than host (%d bytes)", info.Size(), hostInfo.Size())
		}
		// Bundle overhead should be minimal for simple scripts (< 100KB)
		if overhead > 100*1024 {
			t.Errorf("Bundle overhead too large: %d bytes (expected < 100KB)", overhead)
		}
	})

	// ==========================================
	// Test: build fails gracefully with invalid source
	// ==========================================
	t.Run("build with invalid source", func(t *testing.T) {
		script := filepath.Join(tmpDir, "invalid.lang")
		writeFile(t, script, `this is not valid code @#$%`)

		outputBin := filepath.Join(tmpDir, "invalid")

		cmd := exec.Command(binaryPath, "build", script, "-o", outputBin)
		cmd.Dir = projectRoot
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err == nil {
			t.Error("Expected build to fail with invalid source")
			os.Remove(outputBin)
			return
		}

		if !strings.Contains(stderr.String(), "error") && !strings.Contains(stderr.String(), "Error") {
			t.Errorf("Expected error message, got: %s", stderr.String())
		}
	})

	// ==========================================
	// Test: build fails gracefully with missing file
	// ==========================================
	t.Run("build with missing file", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "build", "/nonexistent/file.lang")
		cmd.Dir = projectRoot
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err == nil {
			t.Error("Expected build to fail with missing file")
			return
		}

		if !strings.Contains(stderr.String(), "error") && !strings.Contains(stderr.String(), "Error") {
			t.Errorf("Expected error message about missing file, got: %s", stderr.String())
		}
	})

	// #36: --embed nonexistent path → exit 1, stderr contains "Error"
	t.Run("embed nonexistent path", func(t *testing.T) {
		script := filepath.Join(tmpDir, "embed_nonexist.lang")
		writeFile(t, script, `print("ok")`)

		cmd := exec.Command(binaryPath, "build", script, "--embed", "/no/such/path", "-o", filepath.Join(tmpDir, "out"))
		cmd.Dir = projectRoot
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			t.Error("Expected build to fail with nonexistent embed path")
		}
		if !strings.Contains(stderr.String(), "Error") && !strings.Contains(stderr.String(), "error") {
			t.Errorf("Expected Error in stderr, got: %s", stderr.String())
		}
	})

	// #37: --embed glob without matches → binary builds, stderr contains "Warning"
	t.Run("embed glob no matches", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "embed_nomatch")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "app.lang"), `print("ok")`)

		cmd := exec.Command(binaryPath, "build", filepath.Join(dir, "app.lang"), "--embed", filepath.Join(dir, "*.xyz"), "-o", filepath.Join(tmpDir, "nomatch_bin"))
		cmd.Dir = projectRoot
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			t.Fatalf("Build should succeed (glob no matches): %v\n%s", err, stderr.String())
		}
		if !strings.Contains(stderr.String(), "Warning") && !strings.Contains(stderr.String(), "matched no files") {
			t.Errorf("Expected Warning in stderr for glob no matches, got: %s", stderr.String())
		}
		got := runCmd(t, filepath.Join(tmpDir, "nomatch_bin"), tmpDir, nil)
		if got != "ok" {
			t.Errorf("Binary should run: got %q", got)
		}
	})

	// ==========================================
	// Test: build preserves data types
	// ==========================================
	t.Run("build preserves data types", func(t *testing.T) {
		script := filepath.Join(tmpDir, "types.lang")
		writeFile(t, script, `
import "lib/json" (jsonEncode)

data = {
    int_val: 42,
    float_val: 3.14,
    bool_val: true,
    list_val: [1, 2, 3],
    str_val: "hello"
}
print(jsonEncode(data))
`)
		outputBin := filepath.Join(tmpDir, "types")
		runCmd(t, binaryPath, projectRoot, nil, "build", script, "-o", outputBin)

		got := runCmd(t, outputBin, tmpDir, nil)
		for _, expected := range []string{`"int_val":42`, `"bool_val":true`, `"str_val":"hello"`} {
			if !strings.Contains(got, expected) {
				t.Errorf("Output missing %s, got: %s", expected, got)
			}
		}
	})

	// ==========================================
	// Test: build multi-file package
	// ==========================================
	t.Run("build multi-file package", func(t *testing.T) {
		// Create a package directory with multiple files
		pkgDir := filepath.Join(tmpDir, "mypkg")
		os.MkdirAll(pkgDir, 0755)

		// common.lang defines shared data and helpers
		writeFile(t, filepath.Join(pkgDir, "common.lang"), `package mypkg

greeting = "Hello"

fun formatGreeting(name) {
    "${greeting}, ${name}!"
}

fun double(x) { x * 2 }
`)
		// mypkg.lang is the entry file that uses cross-file symbols
		writeFile(t, filepath.Join(pkgDir, "mypkg.lang"), `package mypkg

print(formatGreeting("World"))
print(double(21))
`)

		outputBin := filepath.Join(tmpDir, "mypkg_bin")

		// Get interpreted output
		interpreted := runCmd(t, binaryPath, projectRoot, nil, filepath.Join(pkgDir, "mypkg.lang"))

		// Build and get compiled output
		runCmd(t, binaryPath, projectRoot, nil, "build", filepath.Join(pkgDir, "mypkg.lang"), "-o", outputBin)
		compiled := runCmd(t, outputBin, tmpDir, nil)

		if interpreted != compiled {
			t.Errorf("Output mismatch:\nInterpreted: %s\nCompiled:    %s", interpreted, compiled)
		}

		if compiled != "Hello, World!\n42" {
			t.Errorf("Unexpected output: want %q, got %q", "Hello, World!\n42", compiled)
		}
	})

	// ==========================================
	// Test: build with --embed single file
	// ==========================================
	t.Run("build with embed single file", func(t *testing.T) {
		// Create a data file to embed
		dataDir := filepath.Join(tmpDir, "embed_single")
		os.MkdirAll(dataDir, 0755)

		writeFile(t, filepath.Join(dataDir, "greeting.txt"), "Hello from embedded file!")
		writeFile(t, filepath.Join(dataDir, "app.lang"), `
import "lib/io" (fileRead)
import "lib/sys" (sysScriptDir)
import "lib/path" (pathJoin)

scriptDir = sysScriptDir()
content = pathJoin([scriptDir, "greeting.txt"]) |>> fileRead
print(content)
`)

		outputBin := filepath.Join(tmpDir, "embed_single_bin")
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dataDir, "app.lang"),
			"--embed", dataDir+"/@.@greeting.txt",
			"-o", outputBin)

		// Run from a DIFFERENT directory — the embedded file should still be found
		otherDir := filepath.Join(tmpDir, "other_dir")
		os.MkdirAll(otherDir, 0755)

		got := runCmd(t, outputBin, otherDir, nil)
		if got != "Hello from embedded file!" {
			t.Errorf("Output mismatch: want %q, got %q", "Hello from embedded file!", got)
		}
	})

	// ==========================================
	// Test: build with --embed directory
	// ==========================================
	t.Run("build with embed directory", func(t *testing.T) {
		embedRoot := filepath.Join(tmpDir, "embed_dir_test")
		os.MkdirAll(filepath.Join(embedRoot, "templates"), 0755)

		writeFile(t, filepath.Join(embedRoot, "templates", "header.html"), "<h1>Welcome</h1>")
		writeFile(t, filepath.Join(embedRoot, "templates", "footer.html"), "<footer>Bye</footer>")

		writeFile(t, filepath.Join(embedRoot, "app.lang"), `
import "lib/io" (fileRead, fileExists)
import "lib/sys" (sysScriptDir)
import "lib/path" (pathJoin)

scriptDir = sysScriptDir()
headerExists = pathJoin([scriptDir, "templates/header.html"]) |>> fileExists
print(headerExists)

header = pathJoin([scriptDir, "templates/header.html"]) |>> fileRead
footer = pathJoin([scriptDir, "templates/footer.html"]) |>> fileRead
print(header)
print(footer)
`)

		outputBin := filepath.Join(tmpDir, "embed_dir_bin")
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(embedRoot, "app.lang"),
			"--embed", filepath.Join(embedRoot, "templates")+"@templates@",
			"-o", outputBin)

		otherDir := filepath.Join(tmpDir, "other_dir2")
		os.MkdirAll(otherDir, 0755)

		got := runCmd(t, outputBin, otherDir, nil)
		if got != "true\n<h1>Welcome</h1>\n<footer>Bye</footer>" {
			t.Errorf("Output mismatch: want %q, got %q", "true\n<h1>Welcome</h1>\n<footer>Bye</footer>", got)
		}
	})

	// ==========================================
	// Test: build with --embed and fileSize
	// ==========================================
	t.Run("build with embed fileSize", func(t *testing.T) {
		embedRoot := filepath.Join(tmpDir, "embed_size_test")
		os.MkdirAll(embedRoot, 0755)

		writeFile(t, filepath.Join(embedRoot, "data.bin"), "12345678") // 8 bytes

		writeFile(t, filepath.Join(embedRoot, "app.lang"), `
import "lib/io" (fileSize)

size = fileSize("data.bin") |>> \x -> x
print(size)
`)

		outputBin := filepath.Join(tmpDir, "embed_size_bin")
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(embedRoot, "app.lang"),
			"--embed", embedRoot+"/@.@data.bin",
			"-o", outputBin)

		otherDir := filepath.Join(tmpDir, "other_dir3")
		os.MkdirAll(otherDir, 0755)

		got := runCmd(t, outputBin, otherDir, nil)
		if got != "8" {
			t.Errorf("Output mismatch: want %q, got %q", "8", got)
		}
	})

	// ==========================================
	// Test: build with multiple --embed flags
	// ==========================================
	t.Run("build with multiple embed flags", func(t *testing.T) {
		embedRoot := filepath.Join(tmpDir, "embed_multi_test")
		os.MkdirAll(filepath.Join(embedRoot, "static"), 0755)
		os.MkdirAll(filepath.Join(embedRoot, "config"), 0755)

		writeFile(t, filepath.Join(embedRoot, "static", "index.html"), "<html>OK</html>")
		writeFile(t, filepath.Join(embedRoot, "config", "app.toml"), `name = "test"`)

		writeFile(t, filepath.Join(embedRoot, "app.lang"), `
import "lib/io" (fileRead)
import "lib/sys" (sysScriptDir)
import "lib/path" (pathJoin)

scriptDir = sysScriptDir()
html = pathJoin([scriptDir, "static/index.html"]) |>> fileRead
conf = pathJoin([scriptDir, "config/app.toml"]) |>> fileRead
print(html)
print(conf)
`)

		outputBin := filepath.Join(tmpDir, "embed_multi_bin")
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(embedRoot, "app.lang"),
			"--embed", filepath.Join(embedRoot, "static")+"@static@",
			"--embed", filepath.Join(embedRoot, "config")+"@config@",
			"-o", outputBin)

		otherDir := filepath.Join(tmpDir, "other_dir4")
		os.MkdirAll(otherDir, 0755)

		got := runCmd(t, outputBin, otherDir, nil)
		if got != "<html>OK</html>\nname = \"test\"" {
			t.Errorf("Output mismatch: want %q, got %q", "<html>OK</html>\nname = \"test\"", got)
		}
	})

	// ==========================================
	// Test: build with --embed comma-separated
	// ==========================================
	t.Run("build with embed comma-separated", func(t *testing.T) {
		embedRoot := filepath.Join(tmpDir, "embed_comma_test")
		os.MkdirAll(filepath.Join(embedRoot, "css"), 0755)
		os.MkdirAll(filepath.Join(embedRoot, "js"), 0755)

		writeFile(t, filepath.Join(embedRoot, "css", "style.css"), "body { color: red; }")
		writeFile(t, filepath.Join(embedRoot, "js", "app.js"), "console.log('hi')")

		writeFile(t, filepath.Join(embedRoot, "app.lang"), `
import "lib/io" (fileRead)

css = fileRead("css/style.css") |>> \x -> x
js = fileRead("js/app.js") |>> \x -> x
print(css)
print(js)
`)

		outputBin := filepath.Join(tmpDir, "embed_comma_bin")
		// Use comma-separated paths in a single --embed
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(embedRoot, "app.lang"),
			"--embed", filepath.Join(embedRoot, "css")+"@css@,"+filepath.Join(embedRoot, "js")+"@js@",
			"-o", outputBin)

		otherDir := filepath.Join(tmpDir, "other_dir_comma")
		os.MkdirAll(otherDir, 0755)

		got := runCmd(t, outputBin, otherDir, nil)
		if got != "body { color: red; }\nconsole.log('hi')" {
			t.Errorf("Output mismatch: want %q, got %q", "body { color: red; }\nconsole.log('hi')", got)
		}
	})

	// ==========================================
	// Test: build with --embed glob pattern
	// ==========================================
	t.Run("build with embed glob", func(t *testing.T) {
		embedRoot := filepath.Join(tmpDir, "embed_glob_test")
		os.MkdirAll(embedRoot, 0755)

		writeFile(t, filepath.Join(embedRoot, "a.txt"), "file-a")
		writeFile(t, filepath.Join(embedRoot, "b.txt"), "file-b")
		writeFile(t, filepath.Join(embedRoot, "c.dat"), "file-c") // NOT matched by *.txt

		writeFile(t, filepath.Join(embedRoot, "app.lang"), `
import "lib/io" (fileRead, fileExists)

aExists = fileExists("a.txt")
bExists = fileExists("b.txt")
cExists = fileExists("c.dat")
print(aExists)
print(bExists)
print(cExists)

a = fileRead("a.txt") |>> \x -> x
b = fileRead("b.txt") |>> \x -> x
print(a)
print(b)
`)

		outputBin := filepath.Join(tmpDir, "embed_glob_bin")
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(embedRoot, "app.lang"),
			"--embed", embedRoot+"/@.@*.txt",
			"-o", outputBin)

		otherDir := filepath.Join(tmpDir, "other_dir_glob")
		os.MkdirAll(otherDir, 0755)

		got := runCmd(t, outputBin, otherDir, nil)
		// *.txt matches a.txt and b.txt, but NOT c.dat
		if got != "true\ntrue\nfalse\nfile-a\nfile-b" {
			t.Errorf("Output mismatch: want %q, got %q", "true\ntrue\nfalse\nfile-a\nfile-b", got)
		}
	})

	// ==========================================
	// Test: build with --embed brace expansion
	// ==========================================
	t.Run("build with embed brace expansion", func(t *testing.T) {
		embedRoot := filepath.Join(tmpDir, "embed_brace_test")
		os.MkdirAll(embedRoot, 0755)

		writeFile(t, filepath.Join(embedRoot, "style.css"), "body{}")
		writeFile(t, filepath.Join(embedRoot, "app.js"), "alert(1)")
		writeFile(t, filepath.Join(embedRoot, "data.txt"), "should-not-match")

		writeFile(t, filepath.Join(embedRoot, "app.lang"), `
import "lib/io" (fileRead, fileExists)

cssOk = fileExists("style.css")
jsOk = fileExists("app.js")
txtOk = fileExists("data.txt")
print(cssOk)
print(jsOk)
print(txtOk)

css = fileRead("style.css") |>> \x -> x
js = fileRead("app.js") |>> \x -> x
print(css)
print(js)
`)

		outputBin := filepath.Join(tmpDir, "embed_brace_bin")
		// Use brace expansion: *.{css,js} should match style.css and app.js but NOT data.txt
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(embedRoot, "app.lang"),
			"--embed", embedRoot+"/@.@*.{css,js}",
			"-o", outputBin)

		otherDir := filepath.Join(tmpDir, "other_dir_brace")
		os.MkdirAll(otherDir, 0755)

		got := runCmd(t, outputBin, otherDir, nil)
		if got != "true\ntrue\nfalse\nbody{}\nalert(1)" {
			t.Errorf("Output mismatch: want %q, got %q", "true\ntrue\nfalse\nbody{}\nalert(1)", got)
		}
	})

	// ==========================================
	// Test: build with --embed comma + brace mix
	// ==========================================
	t.Run("build with embed comma brace mix", func(t *testing.T) {
		embedRoot := filepath.Join(tmpDir, "embed_mix_test")
		os.MkdirAll(filepath.Join(embedRoot, "data"), 0755)

		writeFile(t, filepath.Join(embedRoot, "index.html"), "<h1>hi</h1>")
		writeFile(t, filepath.Join(embedRoot, "app.js"), "run()")
		writeFile(t, filepath.Join(embedRoot, "data", "config.toml"), "key=1")

		writeFile(t, filepath.Join(embedRoot, "app.lang"), `
import "lib/io" (fileRead, fileExists)

htmlOk = fileExists("index.html")
jsOk = fileExists("app.js")
cfgOk = fileExists("data/config.toml")
print(htmlOk)
print(jsOk)
print(cfgOk)
`)

		outputBin := filepath.Join(tmpDir, "embed_mix_bin")
		// Mix: brace expansion + comma-separated directory
		// "*.{html,js}" matches index.html, app.js; "data" matches data/config.toml
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(embedRoot, "app.lang"),
			"--embed", embedRoot+"/@.@*.{html,js},"+filepath.Join(embedRoot, "data")+"@data@",
			"-o", outputBin)

		otherDir := filepath.Join(tmpDir, "other_dir_mix")
		os.MkdirAll(otherDir, 0755)

		got := runCmd(t, outputBin, otherDir, nil)
		if got != "true\ntrue\ntrue" {
			t.Errorf("Output mismatch: want %q, got %q", "true\ntrue\ntrue", got)
		}
	})

	// ==========================================
	// Section 1: User module imports in bundles
	// ==========================================

	// 1.1 Simple user import: app.lang imports ./utils
	t.Run("user import simple", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "user_import_simple")
		os.MkdirAll(dir, 0755)

		utilsDir := filepath.Join(dir, "utils")
		os.MkdirAll(utilsDir, 0755)
		writeFile(t, filepath.Join(utilsDir, "utils.lang"), `package utils (*)
fun greet(name) { "Hello, " ++ name }`)
		writeFile(t, filepath.Join(dir, "app.lang"), `
import "./utils" (greet)
print(greet("World"))
`)

		outputBin := filepath.Join(tmpDir, "user_import_simple_bin")
		interpreted := runCmd(t, binaryPath, projectRoot, nil, filepath.Join(dir, "app.lang"))
		runCmd(t, binaryPath, projectRoot, nil, "build", filepath.Join(dir, "app.lang"), "-o", outputBin)
		compiled := runCmd(t, outputBin, tmpDir, nil)

		if interpreted != compiled {
			t.Errorf("Output mismatch:\nInterpreted: %s\nCompiled:    %s", interpreted, compiled)
		}
		if compiled != "Hello, World" {
			t.Errorf("Unexpected output: want %q, got %q", "Hello, World", compiled)
		}
	})

	// 1.2 Import chain: app.lang → ./math → ./constants
	t.Run("user import chain", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "user_import_chain")
		os.MkdirAll(dir, 0755)

		constDir := filepath.Join(dir, "constants")
		os.MkdirAll(constDir, 0755)
		writeFile(t, filepath.Join(constDir, "constants.lang"), `package constants (*)
pi = 3.14`)

		mathDir := filepath.Join(dir, "math")
		os.MkdirAll(mathDir, 0755)
		writeFile(t, filepath.Join(mathDir, "math.lang"), `package math (*)
import "../constants" (pi)
fun circleArea(r) { pi * r * r }`)

		writeFile(t, filepath.Join(dir, "app.lang"), `
import "./math" (circleArea)
print(circleArea(10))
`)

		outputBin := filepath.Join(tmpDir, "user_import_chain_bin")
		interpreted := runCmd(t, binaryPath, projectRoot, nil, filepath.Join(dir, "app.lang"))
		runCmd(t, binaryPath, projectRoot, nil, "build", filepath.Join(dir, "app.lang"), "-o", outputBin)
		compiled := runCmd(t, outputBin, tmpDir, nil)

		if interpreted != compiled {
			t.Errorf("Output mismatch:\nInterpreted: %s\nCompiled:    %s", interpreted, compiled)
		}
		// 3.14 * 10 * 10 = 314.0 or 314
		if compiled != "314" && compiled != "314.0" {
			t.Errorf("Unexpected output: want 314 or 314.0, got %q", compiled)
		}
	})

	// 1.3 Diamond import: app → a and b, both import ./shared
	t.Run("user import diamond", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "user_import_diamond")
		os.MkdirAll(dir, 0755)

		sharedDir := filepath.Join(dir, "shared")
		os.MkdirAll(sharedDir, 0755)
		writeFile(t, filepath.Join(sharedDir, "shared.lang"), `package shared (*)
fun double(x) { x * 2 }`)

		aDir := filepath.Join(dir, "a")
		os.MkdirAll(aDir, 0755)
		writeFile(t, filepath.Join(aDir, "a.lang"), `package a (*)
import "../shared" (double)
fun processA(x) { double(x) + 1 }`)

		bDir := filepath.Join(dir, "b")
		os.MkdirAll(bDir, 0755)
		writeFile(t, filepath.Join(bDir, "b.lang"), `package b (*)
import "../shared" (double)
fun processB(x) { double(x) + 10 }`)

		writeFile(t, filepath.Join(dir, "app.lang"), `
import "./a" (processA)
import "./b" (processB)
print(processA(5))
print(processB(5))
`)

		outputBin := filepath.Join(tmpDir, "user_import_diamond_bin")
		interpreted := runCmd(t, binaryPath, projectRoot, nil, filepath.Join(dir, "app.lang"))
		runCmd(t, binaryPath, projectRoot, nil, "build", filepath.Join(dir, "app.lang"), "-o", outputBin)
		compiled := runCmd(t, outputBin, tmpDir, nil)

		if interpreted != compiled {
			t.Errorf("Output mismatch:\nInterpreted: %s\nCompiled:    %s", interpreted, compiled)
		}
		if compiled != "11\n20" {
			t.Errorf("Unexpected output: want %q, got %q", "11\n20", compiled)
		}
	})

	// 1.4 Import subdirectory (package group)
	t.Run("user import package group", func(t *testing.T) {
		mylibDir := filepath.Join(tmpDir, "user_pkg_group", "mylib")
		os.MkdirAll(mylibDir, 0755)

		writeFile(t, filepath.Join(mylibDir, "mylib.lang"), `package mylib (add, mul)
fun add(a: Int, b: Int) -> Int { a + b }
`)
		writeFile(t, filepath.Join(mylibDir, "extras.lang"), `package mylib
fun mul(a: Int, b: Int) -> Int { a * b }
`)

		appDir := filepath.Join(tmpDir, "user_pkg_group")
		writeFile(t, filepath.Join(appDir, "app.lang"), `
import "./mylib" (add, mul)
print(add(2, 3))
print(mul(4, 5))
`)

		outputBin := filepath.Join(tmpDir, "user_pkg_group_bin")
		interpreted := runCmd(t, binaryPath, projectRoot, nil, filepath.Join(appDir, "app.lang"))
		runCmd(t, binaryPath, projectRoot, nil, "build", filepath.Join(appDir, "app.lang"), "-o", outputBin)
		compiled := runCmd(t, outputBin, tmpDir, nil)

		if interpreted != compiled {
			t.Errorf("Output mismatch:\nInterpreted: %s\nCompiled:    %s", interpreted, compiled)
		}
		if compiled != "5\n20" {
			t.Errorf("Unexpected output: want %q, got %q", "5\n20", compiled)
		}
	})

	// ==========================================
	// Section 2: Traits in bundles
	// ==========================================

	// 2.1 Simple trait with default method
	t.Run("trait with default method", func(t *testing.T) {
		script := filepath.Join(tmpDir, "trait_default.lang")
		writeFile(t, script, `
trait Printable<t> {
    fun display(val: t) -> String
    fun debugPrint(val: t) {
        print("[DEBUG] " ++ display(val))
    }
}

type alias Point = { x: Int, y: Int }

instance Printable Point {
    fun display(p: Point) -> String {
        "(" ++ show(p.x) ++ ", " ++ show(p.y) ++ ")"
    }
}

p: Point = { x: 3, y: 4 }
debugPrint(p)
`)

		outputBin := filepath.Join(tmpDir, "trait_default_bin")
		interpreted := runCmd(t, binaryPath, projectRoot, nil, script)
		runCmd(t, binaryPath, projectRoot, nil, "build", script, "-o", outputBin)
		compiled := runCmd(t, outputBin, tmpDir, nil)

		if interpreted != compiled {
			t.Errorf("Output mismatch:\nInterpreted: %s\nCompiled:    %s", interpreted, compiled)
		}
		if compiled != "[DEBUG] (3, 4)" {
			t.Errorf("Unexpected output: want %q, got %q", "[DEBUG] (3, 4)", compiled)
		}
	})

	// 2.2 Trait with override default method
	t.Run("trait with override default", func(t *testing.T) {
		script := filepath.Join(tmpDir, "trait_override.lang")
		writeFile(t, script, `
trait Printable<t> {
    fun display(val: t) -> String
    fun debugPrint(val: t) {
        print("[DEBUG] " ++ display(val))
    }
}

type alias Point = { x: Int, y: Int }

instance Printable Point {
    fun display(p: Point) -> String {
        "(" ++ show(p.x) ++ ", " ++ show(p.y) ++ ")"
    }
    fun debugPrint(p: Point) {
        print("OVERRIDE: " ++ display(p))
    }
}

p: Point = { x: 3, y: 4 }
debugPrint(p)
`)

		outputBin := filepath.Join(tmpDir, "trait_override_bin")
		interpreted := runCmd(t, binaryPath, projectRoot, nil, script)
		runCmd(t, binaryPath, projectRoot, nil, "build", script, "-o", outputBin)
		compiled := runCmd(t, outputBin, tmpDir, nil)

		if interpreted != compiled {
			t.Errorf("Output mismatch:\nInterpreted: %s\nCompiled:    %s", interpreted, compiled)
		}
		if compiled != "OVERRIDE: (3, 4)" {
			t.Errorf("Unexpected output: want %q, got %q", "OVERRIDE: (3, 4)", compiled)
		}
	})

	// 2.3 Trait from user module (trait defined in ./traits, impl in app)
	t.Run("trait from user module", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "trait_user_mod")
		os.MkdirAll(dir, 0755)

		traitsDir := filepath.Join(dir, "traits")
		os.MkdirAll(traitsDir, 0755)
		writeFile(t, filepath.Join(traitsDir, "traits.lang"), `package traits (*)

trait Greetable<t> {
    fun greet(val: t) -> String
}`)

		writeFile(t, filepath.Join(dir, "app.lang"), `
import "./traits" (*)

type alias Person = { name: String }

instance Greetable Person {
    fun greet(p: Person) -> String { "Hello, " ++ p.name }
}

p: Person = { name: "World" }
print(greet(p))
`)

		outputBin := filepath.Join(tmpDir, "trait_user_mod_bin")
		interpreted := runCmd(t, binaryPath, projectRoot, nil, filepath.Join(dir, "app.lang"))
		runCmd(t, binaryPath, projectRoot, nil, "build", filepath.Join(dir, "app.lang"), "-o", outputBin)
		compiled := runCmd(t, outputBin, tmpDir, nil)

		if interpreted != compiled {
			t.Errorf("Output mismatch:\nInterpreted: %s\nCompiled:    %s", interpreted, compiled)
		}
		if compiled != "Hello, World" {
			t.Errorf("Unexpected output: want %q, got %q", "Hello, World", compiled)
		}
	})

	// ==========================================
	// Section 3: Serialization edge cases
	// ==========================================

	// 3.1 Complex default parameter expressions
	t.Run("complex default parameters", func(t *testing.T) {
		script := filepath.Join(tmpDir, "complex_defaults.lang")
		writeFile(t, script, `
fun greet(name: String, prefix: String = "Hello, " ++ "dear ") -> String {
    prefix ++ name
}
print(greet("Alice"))
print(greet("Bob", "Hi, "))
`)
		outputBin := filepath.Join(tmpDir, "complex_defaults_bin")
		interpreted := runCmd(t, binaryPath, projectRoot, nil, script)
		runCmd(t, binaryPath, projectRoot, nil, "build", script, "-o", outputBin)
		compiled := runCmd(t, outputBin, tmpDir, nil)

		if interpreted != compiled {
			t.Errorf("Output mismatch:\nInterpreted: %s\nCompiled:    %s", interpreted, compiled)
		}
		if compiled != "Hello, dear Alice\nHi, Bob" {
			t.Errorf("Unexpected output: want %q, got %q", "Hello, dear Alice\nHi, Bob", compiled)
		}
	})

	// 3.2 String pattern matching in bundle
	t.Run("string pattern matching", func(t *testing.T) {
		script := filepath.Join(tmpDir, "string_pattern.lang")
		writeFile(t, script, `
fun parseGreeting(s: String) -> String {
    match s {
        "hello {name}" -> "Greeting for: " ++ name
        "bye {name}"   -> "Farewell to: " ++ name
        _              -> "Unknown: " ++ s
    }
}
print(parseGreeting("hello Alice"))
print(parseGreeting("bye Bob"))
print(parseGreeting("other"))
`)
		outputBin := filepath.Join(tmpDir, "string_pattern_bin")
		interpreted := runCmd(t, binaryPath, projectRoot, nil, script)
		runCmd(t, binaryPath, projectRoot, nil, "build", script, "-o", outputBin)
		compiled := runCmd(t, outputBin, tmpDir, nil)

		if interpreted != compiled {
			t.Errorf("Output mismatch:\nInterpreted: %s\nCompiled:    %s", interpreted, compiled)
		}
		if compiled != "Greeting for: Alice\nFarewell to: Bob\nUnknown: other" {
			t.Errorf("Unexpected output: want %q, got %q",
				"Greeting for: Alice\nFarewell to: Bob\nUnknown: other", compiled)
		}
	})

	// 3.3 Rank-N types (forall) in bundle
	t.Run("rank-n types forall", func(t *testing.T) {
		script := filepath.Join(tmpDir, "rankn.lang")
		writeFile(t, script, `
fun applyToInts(f: forall a. a -> a, x: Int, y: Int) -> (Int, Int) {
    (f(x), f(y))
}

fun myId(x) { x }

result = applyToInts(myId, 10, 20)
print(result)
`)
		outputBin := filepath.Join(tmpDir, "rankn_bin")
		interpreted := runCmd(t, binaryPath, projectRoot, nil, script)
		runCmd(t, binaryPath, projectRoot, nil, "build", script, "-o", outputBin)
		compiled := runCmd(t, outputBin, tmpDir, nil)

		if interpreted != compiled {
			t.Errorf("Output mismatch:\nInterpreted: %s\nCompiled:    %s", interpreted, compiled)
		}
	})

	// ==========================================
	// Section 4: Embedded resources — I/O functions
	// ==========================================

	// 4.1 fileReadAt with embedded resource
	t.Run("embed fileReadAt", func(t *testing.T) {
		embedRoot := filepath.Join(tmpDir, "embed_readat")
		os.MkdirAll(embedRoot, 0755)
		writeFile(t, filepath.Join(embedRoot, "data.txt"), "Hello, World!")

		writeFile(t, filepath.Join(embedRoot, "app.lang"), `
import "lib/io" (fileReadAt)
content = fileReadAt("data.txt", 7, 5) |>> \x -> x
print(content)
`)

		outputBin := filepath.Join(tmpDir, "embed_readat_bin")
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(embedRoot, "app.lang"),
			"--embed", embedRoot+"/@.@data.txt",
			"-o", outputBin)

		otherDir := filepath.Join(tmpDir, "embed_readat_other")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, outputBin, otherDir, nil)
		if got != "World" {
			t.Errorf("Output mismatch: want %q, got %q", "World", got)
		}
	})

	// 4.2 fileReadBytes with embedded resource
	t.Run("embed fileReadBytes", func(t *testing.T) {
		embedRoot := filepath.Join(tmpDir, "embed_bytes")
		os.MkdirAll(embedRoot, 0755)
		writeFile(t, filepath.Join(embedRoot, "data.bin"), "1234567890") // 10 bytes

		writeFile(t, filepath.Join(embedRoot, "app.lang"), `
import "lib/io" (fileReadBytes)
bytes = fileReadBytes("data.bin") |>> \x -> x
print(len(bytes))
`)

		outputBin := filepath.Join(tmpDir, "embed_bytes_bin")
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(embedRoot, "app.lang"),
			"--embed", embedRoot+"/@.@data.bin",
			"-o", outputBin)

		otherDir := filepath.Join(tmpDir, "embed_bytes_other")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, outputBin, otherDir, nil)
		if got != "10" {
			t.Errorf("Output mismatch: want %q, got %q", "10", got)
		}
	})

	// 4.3 fileReadBytesAt with embedded resource
	t.Run("embed fileReadBytesAt", func(t *testing.T) {
		embedRoot := filepath.Join(tmpDir, "embed_bytesat")
		os.MkdirAll(embedRoot, 0755)
		writeFile(t, filepath.Join(embedRoot, "data.bin"), "abcdefgh")

		writeFile(t, filepath.Join(embedRoot, "app.lang"), `
import "lib/io" (fileReadBytesAt)
bytes = fileReadBytesAt("data.bin", 2, 3) |>> \x -> x
print(len(bytes))
`)

		outputBin := filepath.Join(tmpDir, "embed_bytesat_bin")
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(embedRoot, "app.lang"),
			"--embed", embedRoot+"/@.@data.bin",
			"-o", outputBin)

		otherDir := filepath.Join(tmpDir, "embed_bytesat_other")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, outputBin, otherDir, nil)
		if got != "3" {
			t.Errorf("Output mismatch: want %q, got %q", "3", got)
		}
	})

	// 4.4 isFile with embedded resource
	t.Run("embed isFile", func(t *testing.T) {
		embedRoot := filepath.Join(tmpDir, "embed_isfile")
		os.MkdirAll(embedRoot, 0755)
		writeFile(t, filepath.Join(embedRoot, "data.txt"), "hello")

		writeFile(t, filepath.Join(embedRoot, "app.lang"), `
import "lib/io" (isFile)
print(isFile("data.txt"))
print(isFile("nonexistent.txt"))
`)

		outputBin := filepath.Join(tmpDir, "embed_isfile_bin")
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(embedRoot, "app.lang"),
			"--embed", embedRoot+"/@.@data.txt",
			"-o", outputBin)

		otherDir := filepath.Join(tmpDir, "embed_isfile_other")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, outputBin, otherDir, nil)
		if got != "true\nfalse" {
			t.Errorf("Output mismatch: want %q, got %q", "true\nfalse", got)
		}
	})

	// 4.5 Embedded takes priority over disk
	t.Run("embed priority over disk", func(t *testing.T) {
		embedRoot := filepath.Join(tmpDir, "embed_priority")
		os.MkdirAll(embedRoot, 0755)
		dataPath := filepath.Join(embedRoot, "data.txt")

		writeFile(t, filepath.Join(embedRoot, "app.lang"), `
import "lib/io" (fileRead)
content = fileRead("data.txt") |>> \x -> x
print(content)
`)

		// Build with "from bundle" embedded (temporarily overwrite data.txt)
		writeFile(t, dataPath, "from bundle")
		outputBin := filepath.Join(tmpDir, "embed_priority_bin")
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(embedRoot, "app.lang"),
			"--embed", embedRoot+"/@.@data.txt",
			"-o", outputBin)

		// Restore disk file to "from disk", run from embedRoot
		writeFile(t, dataPath, "from disk")
		got := runCmd(t, outputBin, embedRoot, nil)
		if got != "from bundle" {
			t.Errorf("Output mismatch: want %q (embedded should win), got %q", "from bundle", got)
		}
	})

	// 4.6 Binary data (non-text)
	t.Run("embed binary data", func(t *testing.T) {
		embedRoot := filepath.Join(tmpDir, "embed_binary")
		os.MkdirAll(embedRoot, 0755)
		// File with null bytes and non-printable chars
		binContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0x00}
		os.WriteFile(filepath.Join(embedRoot, "data.bin"), binContent, 0644)

		writeFile(t, filepath.Join(embedRoot, "app.lang"), `
import "lib/io" (fileReadBytes)
import "lib/bytes" (bytesToList)
bytes = fileReadBytes("data.bin") |>> \x -> x
print(len(bytes))
lst = bytesToList(bytes)
print(len(lst))
`)
		outputBin := filepath.Join(tmpDir, "embed_binary_bin")
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(embedRoot, "app.lang"),
			"--embed", embedRoot+"/@.@data.bin",
			"-o", outputBin)

		otherDir := filepath.Join(tmpDir, "embed_binary_other")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, outputBin, otherDir, nil)
		if got != "6\n6" {
			t.Errorf("Output mismatch: want %q, got %q", "6\n6", got)
		}
	})

	// ==========================================
	// Level 4: Embed edge cases (spec 75-89)
	// ==========================================

	// 76: Empty file
	t.Run("embed empty file", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "embed_empty")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "empty.txt"), "")
		writeFile(t, filepath.Join(dir, "app.lang"), `
import "lib/io" (fileRead)
content = fileRead("empty.txt") |>> \x -> x
print(len(content))
print(content == "")
`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "app.lang"),
			"--embed", dir+"/@.@empty.txt",
			"-o", filepath.Join(tmpDir, "embed_empty_bin"))

		otherDir := filepath.Join(tmpDir, "embed_empty_run")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, filepath.Join(tmpDir, "embed_empty_bin"), otherDir, nil)
		if got != "0\ntrue" {
			t.Errorf("empty file: got %q", got)
		}
	})

	// 78: Nested directories
	t.Run("embed nested directories", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "embed_nested")
		os.MkdirAll(filepath.Join(dir, "assets", "css"), 0755)
		os.MkdirAll(filepath.Join(dir, "assets", "js"), 0755)
		writeFile(t, filepath.Join(dir, "assets", "css", "style.css"), "body{}")
		writeFile(t, filepath.Join(dir, "assets", "js", "app.js"), "run()")
		writeFile(t, filepath.Join(dir, "app.lang"), `
import "lib/io" (fileRead)
css = fileRead("assets/css/style.css") |>> \x -> x
js = fileRead("assets/js/app.js") |>> \x -> x
print(css)
print(js)
`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "app.lang"),
			"--embed", filepath.Join(dir, "assets")+"@assets@",
			"-o", filepath.Join(tmpDir, "embed_nested_bin"))

		otherDir := filepath.Join(tmpDir, "embed_nested_run")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, filepath.Join(tmpDir, "embed_nested_bin"), otherDir, nil)
		if got != "body{}\nrun()" {
			t.Errorf("nested dirs: got %q", got)
		}
	})

	// 79: Resource with spaces in name
	t.Run("embed filename with spaces", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "embed_spaces")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "my file.txt"), "content with spaces")
		writeFile(t, filepath.Join(dir, "app.lang"), `
import "lib/io" (fileRead)
content = fileRead("my file.txt") |>> \x -> x
print(content)
`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "app.lang"),
			"--embed", dir+"/@.@my file.txt",
			"-o", filepath.Join(tmpDir, "embed_spaces_bin"))

		otherDir := filepath.Join(tmpDir, "embed_spaces_run")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, filepath.Join(tmpDir, "embed_spaces_bin"), otherDir, nil)
		if got != "content with spaces" {
			t.Errorf("spaces in filename: got %q", got)
		}
	})

	// 81: fileRead fallback — a.txt from embed, b.txt from disk
	t.Run("embed fallback to disk", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "embed_fallback")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "a.txt"), "from embed")
		writeFile(t, filepath.Join(dir, "app.lang"), `
import "lib/io" (fileRead)
a = fileRead("a.txt") |>> \x -> x
b = fileRead("b.txt") |>> \x -> x
print(a)
print(b)
`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "app.lang"),
			"--embed", dir+"/@.@a.txt",
			"-o", filepath.Join(tmpDir, "embed_fallback_bin"))

		// b.txt only on disk (in run dir)
		runDir := filepath.Join(tmpDir, "embed_fallback_run")
		os.MkdirAll(runDir, 0755)
		writeFile(t, filepath.Join(runDir, "b.txt"), "from disk")

		got := runCmd(t, filepath.Join(tmpDir, "embed_fallback_bin"), runDir, nil)
		if got != "from embed\nfrom disk" {
			t.Errorf("fallback: got %q", got)
		}
	})

	// 85: Multi-command — one script uses resource, other doesn't
	t.Run("multi-command one uses embed", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "multi_one_embed")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "config.json"), `{"x":1}`)
		writeFile(t, filepath.Join(dir, "api.lang"), `
import "lib/io" (fileRead)
import "lib/sys" (sysScriptDir)
import "lib/path" (pathJoin)
scriptDir = sysScriptDir()
print(pathJoin([scriptDir, "config.json"]) |>> fileRead)
`)
		writeFile(t, filepath.Join(dir, "worker.lang"), `print("worker")`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "api.lang"), filepath.Join(dir, "worker.lang"),
			"--embed", dir+"/@.@config.json",
			"-o", filepath.Join(tmpDir, "multi_one_bin"))

		otherDir := filepath.Join(tmpDir, "multi_one_run")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, filepath.Join(tmpDir, "multi_one_bin"), otherDir, nil, "api")
		if !strings.Contains(got, "x") {
			t.Errorf("api should read config: %q", got)
		}
		got = runCmd(t, filepath.Join(tmpDir, "multi_one_bin"), otherDir, nil, "worker")
		if got != "worker" {
			t.Errorf("worker: got %q", got)
		}
	})

	// 86: Multi-command fileExists on embed
	t.Run("multi-command fileExists embed", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "multi_fileexists")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "shared.txt"), "data")
		writeFile(t, filepath.Join(dir, "api.lang"), `
import "lib/io" (fileExists)
import "lib/sys" (sysScriptDir)
import "lib/path" (pathJoin)
scriptDir = sysScriptDir()
print(pathJoin([scriptDir, "shared.txt"]) |>> fileExists)
`)
		writeFile(t, filepath.Join(dir, "worker.lang"), `print("worker")`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "api.lang"), filepath.Join(dir, "worker.lang"),
			"--embed", dir+"/@.@shared.txt",
			"-o", filepath.Join(tmpDir, "multi_fileexists_bin"))

		got := runCmd(t, filepath.Join(tmpDir, "multi_fileexists_bin"), tmpDir, nil, "api")
		if got != "true" {
			t.Errorf("fileExists(shared.txt): got %q", got)
		}
	})

	// 87: Embed + run from different CWD
	t.Run("embed from different CWD", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "embed_cwd")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "config.txt"), "ok")
		writeFile(t, filepath.Join(dir, "app.lang"), `
import "lib/io" (fileRead)
import "lib/sys" (sysScriptDir)
import "lib/path" (pathJoin)
scriptDir = sysScriptDir()
print(pathJoin([scriptDir, "config.txt"]) |>> fileRead)
`)

		binPath := filepath.Join(tmpDir, "embed_cwd_bin")
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "app.lang"),
			"--embed", dir+"/@.@config.txt",
			"-o", binPath)

		// Run from /tmp or other dir — resource should still be found
		otherDir := filepath.Join(tmpDir, "other_cwd")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, binPath, otherDir, nil)
		if got != "ok" {
			t.Errorf("embed from different CWD: got %q", got)
		}
	})

	// #77: Large file (1+ MB)
	t.Run("embed large file", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "embed_large")
		os.MkdirAll(dir, 0755)
		// ~1.5 MB
		largeContent := make([]byte, 1536000)
		for i := range largeContent {
			largeContent[i] = byte(i % 256)
		}
		os.WriteFile(filepath.Join(dir, "big.bin"), largeContent, 0644)
		writeFile(t, filepath.Join(dir, "app.lang"), `
import "lib/io" (fileReadBytes)
bytes = fileReadBytes("big.bin") |>> \x -> x
print(len(bytes))
`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "app.lang"),
			"--embed", dir+"/@.@big.bin",
			"-o", filepath.Join(tmpDir, "embed_large_bin"))

		otherDir := filepath.Join(tmpDir, "embed_large_run")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, filepath.Join(tmpDir, "embed_large_bin"), otherDir, nil)
		if got != "1536000" {
			t.Errorf("large file len: got %q, want 1536000", got)
		}
	})

	// #80: fileWrite to embed path → writes to disk, embed unchanged
	t.Run("fileWrite to embed writes disk", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "embed_write")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "data.txt"), "from-embed")
		writeFile(t, filepath.Join(dir, "app.lang"), `
import "lib/io" (fileRead, fileWrite)
content1 = fileRead("data.txt") |>> \x -> x
fileWrite("data.txt", "from-disk")
content2 = fileRead("data.txt") |>> \x -> x
print(content1)
print(content2)
`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "app.lang"),
			"--embed", dir+"/@.@data.txt",
			"-o", filepath.Join(tmpDir, "embed_write_bin"))

		runDir := filepath.Join(tmpDir, "embed_write_run")
		os.MkdirAll(runDir, 0755)
		got := runCmd(t, filepath.Join(tmpDir, "embed_write_bin"), runDir, nil)
		// content1 = from embed. content2 = could be from-disk (if write succeeded) or from-embed (if embed wins on read)
		// Spec: fileWrite writes to disk. Next fileRead: embed takes priority, so content2 might still be from-embed.
		// Or: fileWrite creates data.txt on disk, fileRead checks embed first - data.txt in embed, returns from-embed.
		// So content2 = from-embed (embed priority). content1 = from-embed. Both from-embed.
		// The spec says "embed-версия при следующем fileRead всё ещё возвращает старое содержимое" - so embed is unchanged.
		// So content2 should still be from-embed (old content). content1 = from-embed.
		if got != "from-embed\nfrom-embed" {
			t.Errorf("embed unchanged after fileWrite: got %q", got)
		}
	})

	// #83: Path with ../
	t.Run("embed path with parent ref", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "embed_parent")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "data.txt"), "in-embed")
		writeFile(t, filepath.Join(dir, "app.lang"), `
import "lib/io" (fileRead)
r = fileRead("../secret.txt")
match r {
    Ok(x) -> print("got: " ++ x)
    Fail(e) -> print("fail: " ++ e)
}
`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "app.lang"),
			"--embed", dir+"/@.@data.txt",
			"-o", filepath.Join(tmpDir, "embed_parent_bin"))

		// No secret.txt on disk - should Fail (or fallback to disk which doesn't have it)
		runDir := filepath.Join(tmpDir, "embed_parent_run")
		os.MkdirAll(runDir, 0755)
		got := runCmd(t, filepath.Join(tmpDir, "embed_parent_bin"), runDir, nil)
		if !strings.Contains(got, "fail:") {
			t.Errorf("../secret.txt should not leak from embed, got: %q", got)
		}
	})

	// #89: Overlapping embed paths
	t.Run("embed overlapping paths", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "embed_overlap")
		os.MkdirAll(filepath.Join(dir, "static", "sub"), 0755)
		writeFile(t, filepath.Join(dir, "static", "root.txt"), "root")
		writeFile(t, filepath.Join(dir, "static", "sub", "nested.txt"), "nested")
		writeFile(t, filepath.Join(dir, "app.lang"), `
import "lib/io" (fileRead, fileExists)
r = fileExists("static/root.txt")
s = fileExists("static/sub/nested.txt")
print(r)
print(s)
a = fileRead("static/root.txt") |>> \x -> x
b = fileRead("static/sub/nested.txt") |>> \x -> x
print(a)
print(b)
`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "app.lang"),
			"--embed", filepath.Join(dir, "static")+"@static@",
			"--embed", filepath.Join(dir, "static", "sub")+"@static/sub@",
			"-o", filepath.Join(tmpDir, "embed_overlap_bin"))

		otherDir := filepath.Join(tmpDir, "embed_overlap_run")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, filepath.Join(tmpDir, "embed_overlap_bin"), otherDir, nil)
		// Document behavior: both paths work (may duplicate or not)
		if got != "true\ntrue\nroot\nnested" {
			t.Errorf("overlapping embed: got %q", got)
		}
	})

	// ==========================================
	// Portability: binary runs from any directory
	// These tests build from a "project root" and run from a different dir.
	// This catches bugs where bundle keys depend on CWD.
	// ==========================================

	// Non-dot import (e.g. "mylib") — the import that broke kit/web
	t.Run("portable: non-dot import from different dir", func(t *testing.T) {
		projDir := filepath.Join(tmpDir, "portable_nondot")
		os.MkdirAll(filepath.Join(projDir, "mylib"), 0755)

		writeFile(t, filepath.Join(projDir, "mylib", "mylib.lang"),
			`package mylib (*)
fun greet(name) { "Hello, " ++ name }`)

		writeFile(t, filepath.Join(projDir, "app.lang"),
			`import "mylib" (greet)
print(greet("World"))`)

		outputBin := filepath.Join(tmpDir, "portable_nondot_bin")

		// Build from project root
		runCmd(t, binaryPath, projDir, nil, "build", "app.lang", "-o", outputBin)

		// Run from a completely different directory
		otherDir := filepath.Join(tmpDir, "somewhere_else")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, outputBin, otherDir, nil)
		if got != "Hello, World" {
			t.Errorf("want %q, got %q", "Hello, World", got)
		}
	})

	// Nested non-dot imports: app → "pkg/core" → "pkg/util"
	t.Run("portable: nested non-dot imports", func(t *testing.T) {
		projDir := filepath.Join(tmpDir, "portable_nested")
		os.MkdirAll(filepath.Join(projDir, "pkg", "util"), 0755)
		os.MkdirAll(filepath.Join(projDir, "pkg", "core"), 0755)

		writeFile(t, filepath.Join(projDir, "pkg", "util", "util.lang"),
			`package util (*)
fun double(x) { x * 2 }`)

		writeFile(t, filepath.Join(projDir, "pkg", "core", "core.lang"),
			`package core (*)
import "pkg/util" (double)
fun process(x) { double(x) + 1 }`)

		writeFile(t, filepath.Join(projDir, "app.lang"),
			`import "pkg/core" (process)
print(process(5))`)

		outputBin := filepath.Join(tmpDir, "portable_nested_bin")
		runCmd(t, binaryPath, projDir, nil, "build", "app.lang", "-o", outputBin)

		otherDir := filepath.Join(tmpDir, "other_nested")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, outputBin, otherDir, nil)
		if got != "11" {
			t.Errorf("want %q, got %q", "11", got)
		}
	})

	// Mixed: app uses non-dot "mylib", mylib uses "../shared"
	t.Run("portable: mixed dot and non-dot imports", func(t *testing.T) {
		projDir := filepath.Join(tmpDir, "portable_mixed")
		os.MkdirAll(filepath.Join(projDir, "shared"), 0755)
		os.MkdirAll(filepath.Join(projDir, "mylib"), 0755)

		writeFile(t, filepath.Join(projDir, "shared", "shared.lang"),
			`package shared (*)
fun base() { 100 }`)

		writeFile(t, filepath.Join(projDir, "mylib", "mylib.lang"),
			`package mylib (*)
import "../shared" (base)
fun compute() { base() + 42 }`)

		writeFile(t, filepath.Join(projDir, "app.lang"),
			`import "mylib" (compute)
print(compute())`)

		outputBin := filepath.Join(tmpDir, "portable_mixed_bin")
		runCmd(t, binaryPath, projDir, nil, "build", "app.lang", "-o", outputBin)

		otherDir := filepath.Join(tmpDir, "other_mixed")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, outputBin, otherDir, nil)
		if got != "142" {
			t.Errorf("want %q, got %q", "142", got)
		}
	})

	// |>> in module function — must unwrap Ok/Some
	t.Run("portable: pipe unwrap in module", func(t *testing.T) {
		projDir := filepath.Join(tmpDir, "portable_unwrap")
		os.MkdirAll(filepath.Join(projDir, "mymod"), 0755)

		writeFile(t, filepath.Join(projDir, "mymod", "mymod.lang"),
			`package mymod (*)
fun unwrapValue() { Ok(42) |>> \x -> x }`)

		writeFile(t, filepath.Join(projDir, "app.lang"),
			`import "mymod" (unwrapValue)
print(unwrapValue())`)

		outputBin := filepath.Join(tmpDir, "portable_unwrap_bin")
		runCmd(t, binaryPath, projDir, nil, "build", "app.lang", "-o", outputBin)

		otherDir := filepath.Join(tmpDir, "other_unwrap")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, outputBin, otherDir, nil)
		if got != "42" {
			t.Errorf("want %q, got %q", "42", got)
		}
	})

	// Non-dot import + embedded resources + |>> in module
	t.Run("portable: non-dot import with embed", func(t *testing.T) {
		projDir := filepath.Join(tmpDir, "portable_embed")
		os.MkdirAll(filepath.Join(projDir, "helpers"), 0755)

		writeFile(t, filepath.Join(projDir, "data.txt"), "embedded content")
		writeFile(t, filepath.Join(projDir, "helpers", "helpers.lang"),
			`package helpers (*)
import "lib/io" (fileRead)
fun loadData() { fileRead("data.txt") |>> \x -> x }`)

		writeFile(t, filepath.Join(projDir, "app.lang"),
			`import "helpers" (loadData)
print(loadData())`)

		outputBin := filepath.Join(tmpDir, "portable_embed_bin")
		runCmd(t, binaryPath, projDir, nil, "build", "app.lang",
			"--embed", projDir+"/@.@data.txt",
			"-o", outputBin)

		otherDir := filepath.Join(tmpDir, "other_embed")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, outputBin, otherDir, nil)
		if got != "embedded content" {
			t.Errorf("want %q, got %q", "embedded content", got)
		}
	})

	// ==========================================
	// Section 7: --host cross-compilation
	// ==========================================

	// 7.1 --host with self (use own binary as host)
	t.Run("build with host self", func(t *testing.T) {
		script := filepath.Join(tmpDir, "host_self.lang")
		writeFile(t, script, `print("host-self")`)

		outputBin := filepath.Join(tmpDir, "host_self_bin")
		runCmd(t, binaryPath, projectRoot, nil, "build", script, "--host", binaryPath, "-o", outputBin)

		got := runCmd(t, outputBin, tmpDir, nil)
		if got != "host-self" {
			t.Errorf("Output mismatch: want %q, got %q", "host-self", got)
		}
	})

	// 7.2 --host with self-contained binary
	t.Run("build with host self-contained", func(t *testing.T) {
		aScript := filepath.Join(tmpDir, "host_a.lang")
		writeFile(t, aScript, `print("script A")`)
		aBin := filepath.Join(tmpDir, "host_a_bin")
		runCmd(t, binaryPath, projectRoot, nil, "build", aScript, "-o", aBin)

		bScript := filepath.Join(tmpDir, "host_b.lang")
		writeFile(t, bScript, `print("script B")`)
		bBin := filepath.Join(tmpDir, "host_b_bin")
		runCmd(t, binaryPath, projectRoot, nil, "build", bScript, "--host", aBin, "-o", bBin)

		got := runCmd(t, bBin, tmpDir, nil)
		if got != "script B" {
			t.Errorf("Output mismatch: want %q, got %q", "script B", got)
		}
	})

	// 7.3 --host with nonexistent file
	t.Run("build with host nonexistent", func(t *testing.T) {
		script := filepath.Join(tmpDir, "host_fail.lang")
		writeFile(t, script, `print("x")`)

		cmd := exec.Command(binaryPath, "build", script, "--host", "/nonexistent/path", "-o", filepath.Join(tmpDir, "out"))
		cmd.Dir = projectRoot
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err == nil {
			t.Error("Expected build to fail with nonexistent host")
		}
		if !strings.Contains(stderr.String(), "error") && !strings.Contains(stderr.String(), "Error") && !strings.Contains(stderr.String(), "Cannot") {
			t.Errorf("Expected error message, got: %s", stderr.String())
		}
	})

	// ==========================================
	// Test: build output matches interpreted output
	// ==========================================
	t.Run("build output matches interpreted", func(t *testing.T) {
		script := filepath.Join(tmpDir, "match_test.lang")
		writeFile(t, script, `
import "lib/string" (stringToUpper)
import "lib/list" (range, map)

fib = \n -> match n {
    0 -> 0
    1 -> 1
    _ -> fib(n - 1) + fib(n - 2)
}

results = range(0, 10) |> map(fib)
print(results)
print(stringToUpper("fibonacci"))
`)
		outputBin := filepath.Join(tmpDir, "match_test")

		// Get interpreted output
		interpreted := runCmd(t, binaryPath, projectRoot, nil, script)

		// Build and get compiled output
		runCmd(t, binaryPath, projectRoot, nil, "build", script, "-o", outputBin)
		compiled := runCmd(t, outputBin, tmpDir, nil)

		if interpreted != compiled {
			t.Errorf("Output mismatch:\nInterpreted: %s\nCompiled:    %s", interpreted, compiled)
		}
	})
}

// TestDualMode tests that self-contained binaries work in dual mode:
// - No args / any flags → runs embedded bundle (user app receives all args)
// - First arg "$" → switches to interpreter mode ("$" is stripped from args)
func TestDualMode(t *testing.T) {
	projectRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	binaryPath := filepath.Join(projectRoot, "funxy-dual-test-binary")
	defer os.Remove(binaryPath)

	t.Log("Building fresh binary for dual-mode tests...")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/funxy")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\n%s", err, output)
	}

	tmpDir := t.TempDir()

	// Build a simple embedded script
	embeddedScript := filepath.Join(tmpDir, "embedded.lang")
	writeFile(t, embeddedScript, `print("embedded mode")`)

	dualBin := filepath.Join(tmpDir, "dual_bin")
	runCmd(t, binaryPath, projectRoot, nil, "build", embeddedScript, "-o", dualBin)

	// Build a second binary that prints its sysArgs — tests that flags reach the app
	argsScript := filepath.Join(tmpDir, "args_app.lang")
	writeFile(t, argsScript, `import "lib/sys" (sysArgs)
args = sysArgs()
i = 0
while i < len(args) {
    print(args[i])
    i = i + 1
}`)
	argsBin := filepath.Join(tmpDir, "args_bin")
	runCmd(t, binaryPath, projectRoot, nil, "build", argsScript, "-o", argsBin)

	// Test 1: No args → runs embedded bundle
	t.Run("no args runs embedded bundle", func(t *testing.T) {
		got := runCmd(t, dualBin, tmpDir, nil)
		if got != "embedded mode" {
			t.Errorf("Expected %q, got %q", "embedded mode", got)
		}
	})

	// Test 2: sysArgs()[0] is the binary path (consistent with interpreter where sysArgs()[0] is script path)
	t.Run("sysArgs[0] is binary path", func(t *testing.T) {
		got := runCmd(t, argsBin, tmpDir, nil)
		// sysArgs()[0] should be the absolute path to the binary (might have /private prefix on macOS)
		if !strings.Contains(got, "args_bin") {
			t.Errorf("sysArgs()[0] should be the binary path containing 'args_bin', got %q", got)
		}
	})

	// Test 2b: User flags → runs embedded bundle AND app receives them after sysArgs()[0]
	t.Run("user flags reach app via sysArgs", func(t *testing.T) {
		got := runCmd(t, argsBin, tmpDir, nil, "--port", "8080", "--verbose")
		lines := strings.Split(got, "\n")
		// First line = binary path, rest = user args
		if len(lines) < 4 {
			t.Fatalf("Expected 4 lines (binary path + 3 args), got %d: %q", len(lines), got)
		}
		if !strings.Contains(lines[0], "args_bin") {
			t.Errorf("sysArgs()[0] should be binary path, got %q", lines[0])
		}
		userArgs := strings.Join(lines[1:], "\n")
		expected := "--port\n8080\n--verbose"
		if userArgs != expected {
			t.Errorf("Expected user args %q, got %q", expected, userArgs)
		}
	})

	// Test 2c: Flags starting with -e, -p, etc. must NOT switch to interpreter
	t.Run("dash-e flag reaches app not interpreter", func(t *testing.T) {
		got := runCmd(t, argsBin, tmpDir, nil, "-e", "some_value")
		lines := strings.Split(got, "\n")
		if len(lines) < 3 {
			t.Fatalf("Expected 3 lines (binary path + 2 args), got %d: %q", len(lines), got)
		}
		userArgs := strings.Join(lines[1:], "\n")
		expected := "-e\nsome_value"
		if userArgs != expected {
			t.Errorf("Expected app to receive -e flag, not switch to interpreter:\n  want: %q\n  got:  %q", expected, userArgs)
		}
	})

	// Test 2d: Source file name as arg must NOT switch to interpreter
	t.Run("lang filename as arg reaches app", func(t *testing.T) {
		got := runCmd(t, argsBin, tmpDir, nil, "input.lang", "--output", "result.txt")
		lines := strings.Split(got, "\n")
		if len(lines) < 4 {
			t.Fatalf("Expected 4 lines (binary path + 3 args), got %d: %q", len(lines), got)
		}
		userArgs := strings.Join(lines[1:], "\n")
		expected := "input.lang\n--output\nresult.txt"
		if userArgs != expected {
			t.Errorf("Expected app to receive .lang filename as arg:\n  want: %q\n  got:  %q", expected, userArgs)
		}
	})

	// Test 3: "$" as first arg → interpreter mode with source file
	t.Run("dollar source file acts as interpreter", func(t *testing.T) {
		script := filepath.Join(tmpDir, "hello.lang")
		writeFile(t, script, `print("interpreter mode")`)

		got := runCmd(t, dualBin, tmpDir, nil, "$", script)
		if got != "interpreter mode" {
			t.Errorf("Expected %q, got %q", "interpreter mode", got)
		}
	})

	// Test 4: "$" + sysExePath() returns the binary path
	t.Run("dollar sysExePath returns binary path", func(t *testing.T) {
		script := filepath.Join(tmpDir, "exepath.lang")
		writeFile(t, script, `import "lib/sys" (sysExePath)
print(sysExePath())`)

		got := runCmd(t, dualBin, tmpDir, nil, "$", script)
		if !strings.Contains(got, "dual_bin") {
			t.Errorf("sysExePath should contain 'dual_bin', got %q", got)
		}
	})

	// Test 5: "$" + interpreter mode with imports
	t.Run("dollar interpreter mode with imports", func(t *testing.T) {
		script := filepath.Join(tmpDir, "imports.lang")
		writeFile(t, script, `import "lib/string" (stringToUpper)
print(stringToUpper("hello"))`)

		got := runCmd(t, dualBin, tmpDir, nil, "$", script)
		if got != "HELLO" {
			t.Errorf("Expected %q, got %q", "HELLO", got)
		}
	})

	// Test 6: "$" + -e flag → eval mode
	t.Run("dollar eval mode with -e flag", func(t *testing.T) {
		got := runCmd(t, dualBin, tmpDir, nil, "$", "-e", "print(1 + 2)")
		if got != "3" {
			t.Errorf("Expected %q, got %q", "3", got)
		}
	})

	// Test 7: "$" + -pe flag → eval mode with auto-print
	t.Run("dollar eval mode with -pe flag", func(t *testing.T) {
		input := "hello"
		got := runCmd(t, dualBin, tmpDir, &input, "$", "-pe", "stringToUpper(stdin)")
		if got != "HELLO" {
			t.Errorf("Expected %q, got %q", "HELLO", got)
		}
	})

	// Test 8: "$" + --help flag → help
	t.Run("dollar help flag shows help", func(t *testing.T) {
		got := runCmd(t, dualBin, tmpDir, nil, "$", "--help")
		if !strings.Contains(got, "Usage:") {
			t.Errorf("Expected help output with 'Usage:', got %q", got)
		}
	})

	// Test 9: "$HOME" (not bare "$") → runs embedded bundle
	t.Run("dollar-prefixed string runs embedded bundle", func(t *testing.T) {
		got := runCmd(t, dualBin, tmpDir, nil, "$HOME")
		if got != "embedded mode" {
			t.Errorf("Expected %q, got %q", "embedded mode", got)
		}
	})
}

// TestBuildMultiCommand tests multi-command binaries (Level 2.2, 3.2, 3.3, 3.4).
func TestBuildMultiCommand(t *testing.T) {
	projectRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	binaryPath := filepath.Join(projectRoot, "funxy-multicmd-test-binary")
	defer os.Remove(binaryPath)

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/funxy")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\n%s", err, output)
	}

	tmpDir := t.TempDir()

	// 2.2.43 Two scripts → binary created, stdout contains "2 commands"
	t.Run("two scripts", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "two_scripts")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `print("api")`)
		writeFile(t, filepath.Join(dir, "worker.lang"), `print("worker")`)

		var stdout, stderr bytes.Buffer
		buildCmd := exec.Command(binaryPath, "build", filepath.Join(dir, "api.lang"), filepath.Join(dir, "worker.lang"), "-o", filepath.Join(tmpDir, "server"))
		buildCmd.Dir = projectRoot
		buildCmd.Stdout = &stdout
		buildCmd.Stderr = &stderr
		if err := buildCmd.Run(); err != nil {
			t.Fatalf("Build failed: %v\n%s%s", err, stderr.String(), stdout.String())
		}
		if !strings.Contains(stdout.String(), "2 commands") {
			t.Errorf("Expected '2 commands' in output, got: %s", stdout.String())
		}

		got := runCmd(t, filepath.Join(tmpDir, "server"), tmpDir, nil, "api")
		if got != "api" {
			t.Errorf("./server api: got %q, want api", got)
		}
		got = runCmd(t, filepath.Join(tmpDir, "server"), tmpDir, nil, "worker")
		if got != "worker" {
			t.Errorf("./server worker: got %q, want worker", got)
		}
	})

	// 2.2.44 Three+ scripts
	t.Run("three scripts", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "three_scripts")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `print("api")`)
		writeFile(t, filepath.Join(dir, "worker.lang"), `print("worker")`)
		writeFile(t, filepath.Join(dir, "cron.lang"), `print("cron")`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "api.lang"), filepath.Join(dir, "worker.lang"), filepath.Join(dir, "cron.lang"),
			"-o", filepath.Join(tmpDir, "server3"))

		got := runCmd(t, filepath.Join(tmpDir, "server3"), tmpDir, nil, "cron")
		if got != "cron" {
			t.Errorf("./server3 cron: got %q", got)
		}
	})

	// #45: Multi-command without -o → creates file named after first script (api)
	t.Run("multi-command without -o", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "multi_no_o")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `print("api")`)
		writeFile(t, filepath.Join(dir, "worker.lang"), `print("worker")`)

		runCmd(t, binaryPath, projectRoot, nil, "build", filepath.Join(dir, "api.lang"), filepath.Join(dir, "worker.lang"))

		apiBin := filepath.Join(dir, "api")
		if runtime.GOOS == "windows" {
			apiBin = filepath.Join(dir, "api.exe")
		}
		if _, err := os.Stat(apiBin); err != nil {
			t.Fatalf("Expected binary api (or api.exe) to be created: %v", err)
		}
		got := runCmd(t, apiBin, dir, nil, "api")
		if got != "api" {
			t.Errorf("./api api: got %q", got)
		}
	})

	// #48: One script with compilation error in multi-build → exit 1, stderr names the file
	t.Run("multi-build one script broken", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "multi_broken")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `print("api")`)
		writeFile(t, filepath.Join(dir, "broken.lang"), `syntax error @#$`)

		cmd := exec.Command(binaryPath, "build", filepath.Join(dir, "api.lang"), filepath.Join(dir, "broken.lang"), "-o", filepath.Join(tmpDir, "broken_bin"))
		cmd.Dir = projectRoot
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			t.Error("Expected build to fail")
		}
		if !strings.Contains(stderr.String(), "broken.lang") {
			t.Errorf("Expected stderr to mention broken.lang, got: %s", stderr.String())
		}
	})

	// #49: Scripts from different directories
	t.Run("scripts from different dirs", func(t *testing.T) {
		root := filepath.Join(tmpDir, "diff_dirs")
		os.MkdirAll(filepath.Join(root, "scripts"), 0755)
		os.MkdirAll(filepath.Join(root, "tools"), 0755)
		writeFile(t, filepath.Join(root, "scripts", "api.lang"), `print("api")`)
		writeFile(t, filepath.Join(root, "tools", "worker.lang"), `print("worker")`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(root, "scripts", "api.lang"), filepath.Join(root, "tools", "worker.lang"),
			"-o", filepath.Join(tmpDir, "diff_bin"))

		got := runCmd(t, filepath.Join(tmpDir, "diff_bin"), tmpDir, nil, "api")
		if got != "api" {
			t.Errorf("./diff_bin api: got %q", got)
		}
		got = runCmd(t, filepath.Join(tmpDir, "diff_bin"), tmpDir, nil, "worker")
		if got != "worker" {
			t.Errorf("./diff_bin worker: got %q", got)
		}
	})

	// #51: Embed + scripts from different dirs → resource key correct (relative from common parent)
	t.Run("embed with scripts from different dirs", func(t *testing.T) {
		root := filepath.Join(tmpDir, "embed_diff_dirs")
		os.MkdirAll(filepath.Join(root, "dir1"), 0755)
		os.MkdirAll(filepath.Join(root, "dir2"), 0755)
		os.MkdirAll(filepath.Join(root, "dir1", "static"), 0755)
		writeFile(t, filepath.Join(root, "dir1", "static", "config.txt"), "shared-config")
		// Resource key is relative to common parent (root): dir1/static/config.txt
		writeFile(t, filepath.Join(root, "dir1", "api.lang"), `
import "lib/io" (fileRead)
print(fileRead("dir1/static/config.txt") |>> \x -> x)
`)
		writeFile(t, filepath.Join(root, "dir2", "worker.lang"), `
import "lib/io" (fileRead)
print(fileRead("dir1/static/config.txt") |>> \x -> x)
`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(root, "dir1", "api.lang"), filepath.Join(root, "dir2", "worker.lang"),
			"--embed", filepath.Join(root, "dir1", "static")+"@dir1/static@",
			"-o", filepath.Join(tmpDir, "embed_diff_bin"))

		otherDir := filepath.Join(tmpDir, "embed_diff_run")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, filepath.Join(tmpDir, "embed_diff_bin"), otherDir, nil, "api")
		if got != "shared-config" {
			t.Errorf("api should see config: got %q", got)
		}
		got = runCmd(t, filepath.Join(tmpDir, "embed_diff_bin"), otherDir, nil, "worker")
		if got != "shared-config" {
			t.Errorf("worker should see config: got %q", got)
		}
	})

	// 2.2.47 Duplicate names → exit 1, "duplicate command name"
	t.Run("duplicate command names", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "dup_names")
		os.MkdirAll(dir, 0755)
		os.MkdirAll(filepath.Join(dir, "dir1"), 0755)
		os.MkdirAll(filepath.Join(dir, "dir2"), 0755)
		writeFile(t, filepath.Join(dir, "dir1", "api.lang"), `print("api1")`)
		writeFile(t, filepath.Join(dir, "dir2", "api.lang"), `print("api2")`)

		buildCmd := exec.Command(binaryPath, "build",
			filepath.Join(dir, "dir1", "api.lang"), filepath.Join(dir, "dir2", "api.lang"),
			"-o", filepath.Join(tmpDir, "dup_bin"))
		buildCmd.Dir = projectRoot
		var stderr bytes.Buffer
		buildCmd.Stderr = &stderr
		err := buildCmd.Run()
		if err == nil {
			t.Error("Expected build to fail with duplicate command name")
		}
		if !strings.Contains(stderr.String(), "duplicate") {
			t.Errorf("Expected 'duplicate' in stderr, got: %s", stderr.String())
		}
	})

	// 2.2.50 --embed shared across commands
	t.Run("embed shared across commands", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "embed_shared")
		os.MkdirAll(filepath.Join(dir, "static"), 0755)
		writeFile(t, filepath.Join(dir, "static", "config.json"), `{"port":8080}`)
		writeFile(t, filepath.Join(dir, "api.lang"), `
import "lib/io" (fileRead)
import "lib/sys" (sysScriptDir)
import "lib/path" (pathJoin)
scriptDir = sysScriptDir()
print(pathJoin([scriptDir, "static/config.json"]) |>> fileRead)
`)
		writeFile(t, filepath.Join(dir, "worker.lang"), `
import "lib/io" (fileRead)
import "lib/sys" (sysScriptDir)
import "lib/path" (pathJoin)
scriptDir = sysScriptDir()
print(pathJoin([scriptDir, "static/config.json"]) |>> fileRead)
`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "api.lang"), filepath.Join(dir, "worker.lang"),
			"--embed", filepath.Join(dir, "static")+"@static@",
			"-o", filepath.Join(tmpDir, "embed_server"))

		got := runCmd(t, filepath.Join(tmpDir, "embed_server"), tmpDir, nil, "api")
		if !strings.Contains(got, "8080") {
			t.Errorf("api should see config: %q", got)
		}
		got = runCmd(t, filepath.Join(tmpDir, "embed_server"), tmpDir, nil, "worker")
		if !strings.Contains(got, "8080") {
			t.Errorf("worker should see config: %q", got)
		}
	})

	// Regression: --embed with multiple files (shell glob expansion) must not treat
	// non-.lang files as source scripts. Simulates: --embed a.html b.js c.css
	t.Run("embed shell glob expansion", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "embed_glob_expand")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `
import "lib/io" (fileRead)
print(fileRead("index.html") |>> \x -> x)
`)
		writeFile(t, filepath.Join(dir, "index.html"), "<h1>hello</h1>")
		writeFile(t, filepath.Join(dir, "app.js"), "console.log(1)")
		writeFile(t, filepath.Join(dir, "style.css"), "body{}")

		// Simulate shell expansion: --embed index.html app.js style.css (3 separate args)
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "api.lang"),
			"--embed", dir+"/@.@index.html", dir+"/@.@app.js", dir+"/@.@style.css",
			"-o", filepath.Join(tmpDir, "embed_glob_bin"))

		otherDir := filepath.Join(tmpDir, "embed_glob_run")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, filepath.Join(tmpDir, "embed_glob_bin"), otherDir, nil)
		if got != "<h1>hello</h1>" {
			t.Errorf("embed glob expansion: got %q, want <h1>hello</h1>", got)
		}
	})

	// Multiple --embed flags: --embed static --embed templates
	t.Run("multiple embed flags", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "multi_embed_flags")
		os.MkdirAll(filepath.Join(dir, "static"), 0755)
		os.MkdirAll(filepath.Join(dir, "templates"), 0755)
		writeFile(t, filepath.Join(dir, "static", "app.js"), "js-content")
		writeFile(t, filepath.Join(dir, "templates", "index.html"), "html-content")
		writeFile(t, filepath.Join(dir, "api.lang"), `
import "lib/io" (fileRead)
import "lib/sys" (sysScriptDir)
import "lib/path" (pathJoin)
scriptDir = sysScriptDir()
a = pathJoin([scriptDir, "static/app.js"]) |>> fileRead
b = pathJoin([scriptDir, "templates/index.html"]) |>> fileRead
print(a)
print(b)
`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "api.lang"),
			"--embed", filepath.Join(dir, "static")+"@static@",
			"--embed", filepath.Join(dir, "templates")+"@templates@",
			"-o", filepath.Join(tmpDir, "multi_embed_bin"))

		otherDir := filepath.Join(tmpDir, "multi_embed_run")
		os.MkdirAll(otherDir, 0755)
		got := runCmd(t, filepath.Join(tmpDir, "multi_embed_bin"), otherDir, nil)
		if got != "js-content\nhtml-content" {
			t.Errorf("multiple --embed flags: got %q", got)
		}
	})

	// Regression: non-.lang files without --embed must not become source scripts
	t.Run("non-lang file rejected as source", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "non_lang_source")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `print("api")`)
		writeFile(t, filepath.Join(dir, "page.html"), "<html></html>")

		cmd := exec.Command(binaryPath, "build",
			filepath.Join(dir, "api.lang"), filepath.Join(dir, "page.html"),
			"-o", filepath.Join(tmpDir, "non_lang_bin"))
		cmd.Dir = projectRoot
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		// Should succeed (page.html is skipped with warning, not compiled)
		if err != nil {
			t.Fatalf("Build should succeed (skip non-.lang): %v\n%s", err, stderr.String())
		}
		if !strings.Contains(stderr.String(), "Warning") && !strings.Contains(stderr.String(), "recognized source") {
			t.Errorf("Expected warning about non-source file, got stderr: %s", stderr.String())
		}
		// Binary should work — only api.lang compiled
		got := runCmd(t, filepath.Join(tmpDir, "non_lang_bin"), tmpDir, nil)
		if got != "api" {
			t.Errorf("non-lang file rejected: got %q, want api", got)
		}
	})

	// 3.2.58 ./server api, 3.2.59 ./server worker
	t.Run("subcommand dispatch", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "dispatch")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `print("api")`)
		writeFile(t, filepath.Join(dir, "worker.lang"), `print("worker")`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "api.lang"), filepath.Join(dir, "worker.lang"),
			"-o", filepath.Join(tmpDir, "dispatch_server"))

		got := runCmd(t, filepath.Join(tmpDir, "dispatch_server"), tmpDir, nil, "api")
		if got != "api" {
			t.Errorf("./server api: got %q", got)
		}
		got = runCmd(t, filepath.Join(tmpDir, "dispatch_server"), tmpDir, nil, "worker")
		if got != "worker" {
			t.Errorf("./server worker: got %q", got)
		}
	})

	// 3.2.60 sysArgs does NOT contain command name (multi-command: ./server api --port 8080)
	t.Run("sysArgs without command name", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "sysargs_cmd")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `import "lib/sys" (sysArgs)
args = sysArgs()
for a in args { print(a) }
`)
		writeFile(t, filepath.Join(dir, "worker.lang"), `print("worker")`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "api.lang"), filepath.Join(dir, "worker.lang"),
			"-o", filepath.Join(tmpDir, "sysargs_server"))

		// ./sysargs_server api --port 8080 → api script runs, sysArgs = [binaryPath, "--port", "8080"], NOT "api"
		got := runCmd(t, filepath.Join(tmpDir, "sysargs_server"), tmpDir, nil, "api", "--port", "8080")
		lines := strings.Split(got, "\n")
		hasPort := false
		hasApi := false
		for _, l := range lines {
			if l == "--port" || l == "8080" {
				hasPort = true
			}
			if l == "api" {
				hasApi = true
			}
		}
		if !hasPort {
			t.Errorf("sysArgs should contain --port 8080, got: %s", got)
		}
		if hasApi {
			t.Errorf("sysArgs should NOT contain 'api' (command name), got: %s", got)
		}
	})

	// #64: ./server --help → exit 1, usage, NOT "Unknown command"
	t.Run("help flag shows usage not unknown command", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "help_flag")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `print("api")`)
		writeFile(t, filepath.Join(dir, "worker.lang"), `print("worker")`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "api.lang"), filepath.Join(dir, "worker.lang"),
			"-o", filepath.Join(tmpDir, "help_server"))

		cmd := exec.Command(filepath.Join(tmpDir, "help_server"), "--help")
		cmd.Dir = tmpDir
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			t.Error("Expected exit 1 for --help (no command)")
		}
		if !strings.Contains(stderr.String(), "Usage:") {
			t.Errorf("Expected Usage in stderr, got: %s", stderr.String())
		}
		if strings.Contains(stderr.String(), "Unknown command") {
			t.Errorf("Should NOT contain 'Unknown command' for --help flag, got: %s", stderr.String())
		}
	})

	// 3.2.62 ./server (no args) → exit 1, usage
	t.Run("no args shows usage", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "usage")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `print("api")`)
		writeFile(t, filepath.Join(dir, "worker.lang"), `print("worker")`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "api.lang"), filepath.Join(dir, "worker.lang"),
			"-o", filepath.Join(tmpDir, "usage_server"))

		usageCmd := exec.Command(filepath.Join(tmpDir, "usage_server"))
		usageCmd.Dir = tmpDir
		var stderr bytes.Buffer
		usageCmd.Stderr = &stderr
		err := usageCmd.Run()
		if err == nil {
			t.Error("Expected exit 1 when no args")
		}
		if !strings.Contains(stderr.String(), "Usage:") && !strings.Contains(stderr.String(), "command") {
			t.Errorf("Expected usage message, got: %s", stderr.String())
		}
	})

	// 3.2.63 ./server blah → "Unknown command: blah"
	t.Run("unknown command", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "unknown")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `print("api")`)
		writeFile(t, filepath.Join(dir, "worker.lang"), `print("worker")`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "api.lang"), filepath.Join(dir, "worker.lang"),
			"-o", filepath.Join(tmpDir, "multi_cmd"))

		unknownCmd := exec.Command(filepath.Join(tmpDir, "multi_cmd"), "blah")
		unknownCmd.Dir = tmpDir
		var stderr bytes.Buffer
		unknownCmd.Stderr = &stderr
		err := unknownCmd.Run()
		if err == nil {
			t.Error("Expected exit 1 for unknown command")
		}
		if !strings.Contains(stderr.String(), "Unknown") && !strings.Contains(stderr.String(), "blah") {
			t.Errorf("Expected 'Unknown command' or 'blah', got: %s", stderr.String())
		}
	})

	// 3.2.65 $ escape-hatch, 3.2.66 $ -e
	t.Run("dollar escape hatch", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "dollar_multi")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `print("api")`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "api.lang"),
			"-o", filepath.Join(tmpDir, "dollar_server"))

		got := runCmd(t, filepath.Join(tmpDir, "dollar_server"), tmpDir, nil, "$", "-e", "print(42)")
		if got != "42" {
			t.Errorf("$ -e 'print(42)': got %q", got)
		}
	})

	// 3.3.67-68 Symlink dispatch: ln -s server api; ./api runs api command (argv[0] basename = "api")
	t.Run("symlink dispatch", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "symlink")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `print("api")`)
		writeFile(t, filepath.Join(dir, "worker.lang"), `print("worker")`)

		serverPath := filepath.Join(dir, "server")
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "api.lang"), filepath.Join(dir, "worker.lang"),
			"-o", serverPath)

		// Symlinks must be named "api" and "worker" to match command names (argv[0] basename)
		apiLink := filepath.Join(dir, "api")
		workerLink := filepath.Join(dir, "worker")
		if err := os.Symlink(serverPath, apiLink); err != nil {
			t.Skipf("Symlink not supported: %v", err)
		}
		if err := os.Symlink(serverPath, workerLink); err != nil {
			t.Skipf("Symlink not supported: %v", err)
		}

		got := runCmd(t, apiLink, dir, nil)
		if got != "api" {
			t.Errorf("./api (symlink): got %q, want api", got)
		}
		got = runCmd(t, workerLink, dir, nil)
		if got != "worker" {
			t.Errorf("./worker (symlink): got %q, want worker", got)
		}
	})

	// #69: Symlink + args → sysArgs = [apiPath, "--port", "8080"] (multi-command)
	t.Run("symlink sysArgs", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "symlink_args")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `import "lib/sys" (sysArgs)
args = sysArgs()
for a in args { print(a) }
`)
		writeFile(t, filepath.Join(dir, "worker.lang"), `print("worker")`)

		runCmd(t, binaryPath, projectRoot, nil, "build", filepath.Join(dir, "api.lang"), filepath.Join(dir, "worker.lang"), "-o", filepath.Join(dir, "server"))

		apiLink := filepath.Join(dir, "api")
		if err := os.Symlink(filepath.Join(dir, "server"), apiLink); err != nil {
			t.Skipf("Symlink not supported: %v", err)
		}

		got := runCmd(t, apiLink, dir, nil, "--port", "8080")
		lines := strings.Split(got, "\n")
		if len(lines) < 3 {
			t.Fatalf("Expected 3+ lines (path + --port + 8080), got: %s", got)
		}
		if !strings.Contains(lines[0], "api") {
			t.Errorf("sysArgs[0] should contain api path, got: %s", lines[0])
		}
		hasPort := false
		for _, l := range lines {
			if l == "--port" || l == "8080" {
				hasPort = true
				break
			}
		}
		if !hasPort {
			t.Errorf("sysArgs should contain --port 8080, got: %s", got)
		}
	})

	// #70: Symlink with unknown name (foo) → usage
	t.Run("symlink unknown name", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "symlink_unknown")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `print("api")`)
		writeFile(t, filepath.Join(dir, "worker.lang"), `print("worker")`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "api.lang"), filepath.Join(dir, "worker.lang"),
			"-o", filepath.Join(dir, "server"))

		fooLink := filepath.Join(dir, "foo")
		if err := os.Symlink(filepath.Join(dir, "server"), fooLink); err != nil {
			t.Skipf("Symlink not supported: %v", err)
		}

		cmd := exec.Command(fooLink)
		cmd.Dir = dir
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			t.Error("Expected exit 1 for ./foo (unknown argv[0])")
		}
		if !strings.Contains(stderr.String(), "Usage:") && !strings.Contains(stderr.String(), "command") {
			t.Errorf("Expected usage, got: %s", stderr.String())
		}
	})

	// #71: Symlink api + arg worker → runs api, worker in sysArgs (NOT worker command)
	t.Run("symlink api with worker arg", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "symlink_api_worker")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `import "lib/sys" (sysArgs)
args = sysArgs()
for a in args { print(a) }
`)
		writeFile(t, filepath.Join(dir, "worker.lang"), `print("worker-cmd")`)

		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "api.lang"), filepath.Join(dir, "worker.lang"),
			"-o", filepath.Join(dir, "server"))

		apiLink := filepath.Join(dir, "api")
		if err := os.Symlink(filepath.Join(dir, "server"), apiLink); err != nil {
			t.Skipf("Symlink not supported: %v", err)
		}

		// ./api worker → argv[0]=api, runs api; "worker" is arg (NOT worker command)
		got := runCmd(t, apiLink, dir, nil, "worker")
		if !strings.Contains(got, "worker") {
			t.Errorf("api should receive 'worker' in sysArgs, got: %s", got)
		}
		if strings.Contains(got, "worker-cmd") {
			t.Error("Should NOT run worker command, should run api with 'worker' as arg")
		}
	})

	// #72: Binary named api (-o api) → ./api runs api via symlink dispatch
	t.Run("binary name equals command", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "bin_name_cmd")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `print("api")`)
		writeFile(t, filepath.Join(dir, "worker.lang"), `print("worker")`)

		apiBin := filepath.Join(dir, "api")
		if runtime.GOOS == "windows" {
			apiBin = filepath.Join(dir, "api.exe")
		}
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "api.lang"), filepath.Join(dir, "worker.lang"),
			"-o", apiBin)

		got := runCmd(t, apiBin, dir, nil)
		if got != "api" {
			t.Errorf("./api (binary named api): got %q, want api", got)
		}
	})

	// #73: ./api worker → runs api, "worker" is arg (NOT worker command)
	t.Run("binary name api with worker arg", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "bin_api_worker_arg")
		os.MkdirAll(dir, 0755)
		writeFile(t, filepath.Join(dir, "api.lang"), `import "lib/sys" (sysArgs)
args = sysArgs()
for a in args { print(a) }
`)
		writeFile(t, filepath.Join(dir, "worker.lang"), `print("worker-cmd")`)

		apiBin := filepath.Join(dir, "api")
		if runtime.GOOS == "windows" {
			apiBin = filepath.Join(dir, "api.exe")
		}
		runCmd(t, binaryPath, projectRoot, nil, "build",
			filepath.Join(dir, "api.lang"), filepath.Join(dir, "worker.lang"),
			"-o", apiBin)

		got := runCmd(t, apiBin, dir, nil, "worker")
		if !strings.Contains(got, "worker") {
			t.Errorf("api should receive 'worker' as arg, got: %s", got)
		}
		if strings.Contains(got, "worker-cmd") {
			t.Error("Should NOT run worker command, should run api with 'worker' as arg")
		}
	})
}

// TestBundleSerialization tests the Bundle serialize/deserialize roundtrip.
func TestBundleSerialization(t *testing.T) {
	projectRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	binaryPath := filepath.Join(projectRoot, "funxy-bundle-test-binary")
	defer os.Remove(binaryPath)

	t.Log("Building fresh binary for bundle serialization tests...")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/funxy")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\n%s", err, output)
	}

	tmpDir := t.TempDir()

	// Test: compile and run produces identical output to direct execution
	scripts := []struct {
		name string
		code string
	}{
		{
			name: "arithmetic",
			code: `print(2 + 3 * 4)`,
		},
		{
			name: "string_ops",
			code: `import "lib/string" (stringToUpper, stringToLower)
print(stringToUpper("hello"))
print(stringToLower("WORLD"))`,
		},
		{
			name: "match_expression",
			code: `
classify = \x -> match x {
    0 -> "zero"
    1 -> "one"
    _ -> "many"
}
print(classify(0))
print(classify(1))
print(classify(42))`,
		},
		{
			name: "closures",
			code: `
makeCounter = fun() {
    count = 0
    inc = fun() {
        count = count + 1
        count
    }
    inc
}
c = makeCounter()
print(c())
print(c())
print(c())`,
		},
	}

	for _, script := range scripts {
		t.Run(fmt.Sprintf("roundtrip_%s", script.name), func(t *testing.T) {
			scriptPath := filepath.Join(tmpDir, script.name+".lang")
			writeFile(t, scriptPath, script.code)

			// Direct execution
			directOutput := runCmd(t, binaryPath, projectRoot, nil, scriptPath)

			// Compile to .fbc
			runCmd(t, binaryPath, projectRoot, nil, "-c", scriptPath)

			// Run from .fbc
			fbcPath := filepath.Join(tmpDir, script.name+".fbc")
			fbcOutput := runCmd(t, binaryPath, projectRoot, nil, "-r", fbcPath)

			if directOutput != fbcOutput {
				t.Errorf("Roundtrip mismatch for %s:\nDirect: %s\nFBC:    %s", script.name, directOutput, fbcOutput)
			}
		})
	}
}

// --- Helpers ---

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file %s: %v", path, err)
	}
}

func runCmd(t *testing.T, binary, dir string, stdin *string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir

	if stdin != nil {
		cmd.Stdin = bytes.NewBufferString(*stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Command %q %v failed: %v\nStderr: %s\nStdout: %s",
			binary, args, err, stderr.String(), stdout.String())
	}

	return strings.TrimSpace(stdout.String())
}

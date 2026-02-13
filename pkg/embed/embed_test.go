package funxy_test

import (
	"fmt"
	funxy "github.com/funvibe/funxy/pkg/embed"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// User represents a Go struct to be used as a Host Object
type User struct {
	Name  string
	Score int
}

func (u *User) AddScore(points int) {
	u.Score += points
}

func (u *User) GetStatus() string {
	return fmt.Sprintf("User %s has %d points", u.Name, u.Score)
}

func TestEmbedAPI(t *testing.T) {
	vm := funxy.New()

	// 1. Bind a simple function
	vm.Bind("double", func(x int) int {
		return x * 2
	})

	// 2. Bind a Host Object
	user := &User{Name: "Alice", Score: 10}
	vm.Bind("player", user)

	// 3. Eval script using bound values
	code := `
	doubled = double(21)

	// Access field
	name = player.Name

	// Call method
	player.AddScore(5)
	status = player.GetStatus()

	[doubled, name, status]
	`

	res, err := vm.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	// 4. Verify results
	list, ok := res.([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{} result, got %T", res)
	}

	if len(list) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(list))
	}

	// Check doubled
	// Note: Funxy Integer is int64, but Marshaller defaults to int for generic conversion
	val1, ok := list[0].(int)
	if !ok {
		// Try int64
		if val164, ok64 := list[0].(int64); ok64 {
			val1 = int(val164)
		} else {
			t.Errorf("Expected int for doubled, got %T", list[0])
		}
	}
	if val1 != 42 {
		t.Errorf("Expected 42, got %d", val1)
	}

	// Check name
	val2, ok := list[1].(string)
	if !ok {
		t.Errorf("Expected string for name, got %T", list[1])
	}
	if val2 != "Alice" {
		t.Errorf("Expected Alice, got %s", val2)
	}

	// Check status
	val3, ok := list[2].(string)
	if !ok {
		t.Errorf("Expected string for status, got %T", list[2])
	}
	expectedStatus := "User Alice has 15 points"
	if val3 != expectedStatus {
		t.Errorf("Expected '%s', got '%s'", expectedStatus, val3)
	}

	// 5. Verify side effect on Go struct
	if user.Score != 15 {
		t.Errorf("Go struct not updated! Score is %d, expected 15", user.Score)
	}
}

func TestLoadFile(t *testing.T) {
	// Setup temp dir
	tmpDir, err := ioutil.TempDir("", "funxy_embed_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create package dir "mylib"
	pkgDir := filepath.Join(tmpDir, "mylib")
	if err := os.Mkdir(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create file in package
	libCode := `
	package mylib(*)

	fun get_greeting() { "Hello from Import" }
	`
	libPath := filepath.Join(pkgDir, "mylib.lang")
	if err := ioutil.WriteFile(libPath, []byte(libCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Create main file
	// Note: We use the absolute path to import the library package
	mainCode := fmt.Sprintf(`
	import "%s" as lib

	greeting = lib.get_greeting()
	`, pkgDir)

	mainPath := filepath.Join(tmpDir, "main.lang")
	if err := ioutil.WriteFile(mainPath, []byte(mainCode), 0644); err != nil {
		t.Fatal(err)
	}

	vm := funxy.New()
	err = vm.LoadFile(mainPath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	// Check result via side effect (global variable 'greeting')
	res, err := vm.Get("greeting")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Convert result to string
	// Get returns interface{} via Marshaller.FromValue
	str, ok := res.(string)
	if !ok {
		t.Fatalf("Expected string, got %T", res)
	}
	if str != "Hello from Import" {
		t.Errorf("Expected 'Hello from Import', got '%s'", str)
	}
}

// --- Tests based on examples/embed_demo ---

// Calculator mirrors the struct from examples/embed_demo
type Calculator struct {
	BaseValue int
}

func (c *Calculator) Add(a, b int) int {
	return c.BaseValue + a + b
}

func (c *Calculator) Multiply(a, b int) int {
	return a * b
}

// AppConfig mirrors the struct from examples/embed_demo
type AppConfig struct {
	Version  string
	LastUser string
}

func (c *AppConfig) UpdateLastUser(user string) {
	c.LastUser = user
}

// TestCall verifies calling a Funxy-defined function from Go via vm.Call().
// Functions are defined via LoadFile (as in embed_demo), then called via Call.
func TestCall(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "funxy_call_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Define a function in a file
	script := filepath.Join(tmpDir, "funcs.lang")
	if err := ioutil.WriteFile(script, []byte(`
fun greet(name) { "Hello, ${name}!" }
`), 0644); err != nil {
		t.Fatal(err)
	}

	vm := funxy.New()
	if err := vm.LoadFile(script); err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	// Call it from Go
	result, err := vm.Call("greet", "World")
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	str, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string, got %T: %v", result, result)
	}
	if str != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got '%s'", str)
	}
}

// TestCallWithBoundObjects verifies that a Funxy function can use bound Go objects
// and the result is accessible from Go. This mirrors the process_user pattern from embed_demo.
func TestCallWithBoundObjects(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "funxy_call_bound_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	script := filepath.Join(tmpDir, "processor.lang")
	if err := ioutil.WriteFile(script, []byte(`
fun process_user(name, score) {
	appConfig.UpdateLastUser(name)
	bonus = calculator.Multiply(score, 2)
	"User ${name} processed. Bonus: ${bonus}"
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	vm := funxy.New()

	calc := &Calculator{BaseValue: 0}
	vm.Bind("calculator", calc)

	config := &AppConfig{Version: "1.0.0", LastUser: "None"}
	vm.Bind("appConfig", config)

	if err := vm.LoadFile(script); err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	// Call it from Go
	result, err := vm.Call("process_user", "Alice", 50)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	str, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string result, got %T: %v", result, result)
	}
	if str != "User Alice processed. Bonus: 100" {
		t.Errorf("Expected 'User Alice processed. Bonus: 100', got '%s'", str)
	}

	// Verify Go struct was mutated
	if config.LastUser != "Alice" {
		t.Errorf("Expected LastUser='Alice', got '%s'", config.LastUser)
	}
}

// TestCallNonExistentFunction verifies error handling for calling an undefined function.
func TestCallNonExistentFunction(t *testing.T) {
	vm := funxy.New()

	_, err := vm.Call("nonexistent", 1, 2)
	if err == nil {
		t.Fatal("Expected error when calling non-existent function")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' in error, got: %v", err)
	}
}

// TestMultipleBindings verifies binding multiple functions and structs
// and using them together in a single Eval, mirroring the embed_demo pattern.
func TestMultipleBindings(t *testing.T) {
	vm := funxy.New()

	// Bind a logger that collects messages
	var logs []string
	vm.Bind("logger", func(msg string) {
		logs = append(logs, msg)
	})

	// Bind a calculator
	calc := &Calculator{BaseValue: 10}
	vm.Bind("calculator", calc)

	// Bind an app config
	config := &AppConfig{Version: "2.0.0", LastUser: "None"}
	vm.Bind("appConfig", config)

	// Eval a script that uses all bindings
	_, err := vm.Eval(`
		logger("Starting")
		sum = calculator.Add(5, 3)
		logger("Sum: ${sum}")
		ver = appConfig.Version
		logger("Version: ${ver}")
		appConfig.UpdateLastUser("Bob")
		logger("Done")
	`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	// Verify logger calls
	expectedLogs := []string{
		"Starting",
		"Sum: 18", // BaseValue(10) + 5 + 3
		"Version: 2.0.0",
		"Done",
	}
	if len(logs) != len(expectedLogs) {
		t.Fatalf("Expected %d log entries, got %d: %v", len(expectedLogs), len(logs), logs)
	}
	for i, expected := range expectedLogs {
		if logs[i] != expected {
			t.Errorf("Log[%d]: expected %q, got %q", i, expected, logs[i])
		}
	}

	// Verify side effect
	if config.LastUser != "Bob" {
		t.Errorf("Expected LastUser='Bob', got '%s'", config.LastUser)
	}
}

// TestEvalMethodCall verifies calling a method on a bound struct via Eval,
// mirroring the `calculator.Add(5, 5)` example from embed_demo.
func TestEvalMethodCall(t *testing.T) {
	vm := funxy.New()

	calc := &Calculator{BaseValue: 0}
	vm.Bind("calculator", calc)

	res, err := vm.Eval("calculator.Add(5, 5)")
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	val, ok := res.(int)
	if !ok {
		if v64, ok64 := res.(int64); ok64 {
			val = int(v64)
		} else {
			t.Fatalf("Expected int, got %T: %v", res, res)
		}
	}
	if val != 10 {
		t.Errorf("Expected 10, got %d", val)
	}
}

// TestStructFieldAccess verifies reading fields of a bound Go struct.
func TestStructFieldAccess(t *testing.T) {
	vm := funxy.New()

	config := &AppConfig{Version: "3.5.1", LastUser: "Charlie"}
	vm.Bind("appConfig", config)

	// Access Version field
	res, err := vm.Eval("appConfig.Version")
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	str, ok := res.(string)
	if !ok {
		t.Fatalf("Expected string, got %T: %v", res, res)
	}
	if str != "3.5.1" {
		t.Errorf("Expected '3.5.1', got '%s'", str)
	}

	// Access LastUser field
	res, err = vm.Eval("appConfig.LastUser")
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	str, ok = res.(string)
	if !ok {
		t.Fatalf("Expected string, got %T: %v", res, res)
	}
	if str != "Charlie" {
		t.Errorf("Expected 'Charlie', got '%s'", str)
	}
}

// TestStructMethodMutatesState verifies that calling a method on a bound struct
// actually mutates the Go-side state, verifiable from Go after Eval.
func TestStructMethodMutatesState(t *testing.T) {
	vm := funxy.New()

	config := &AppConfig{Version: "1.0.0", LastUser: "None"}
	vm.Bind("appConfig", config)

	_, err := vm.Eval(`appConfig.UpdateLastUser("Dana")`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	if config.LastUser != "Dana" {
		t.Errorf("Expected LastUser='Dana', got '%s'", config.LastUser)
	}

	// Mutate again
	_, err = vm.Eval(`appConfig.UpdateLastUser("Eve")`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	if config.LastUser != "Eve" {
		t.Errorf("Expected LastUser='Eve', got '%s'", config.LastUser)
	}
}

// TestStringInterpolationWithBindings verifies string interpolation
// using bound objects, mirroring patterns from embed_demo.
func TestStringInterpolationWithBindings(t *testing.T) {
	vm := funxy.New()

	config := &AppConfig{Version: "4.2.0", LastUser: "Frank"}
	vm.Bind("appConfig", config)

	res, err := vm.Eval(`
		ver = appConfig.Version
		user = appConfig.LastUser
		"App v${ver}, last user: ${user}"
	`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	str, ok := res.(string)
	if !ok {
		t.Fatalf("Expected string, got %T: %v", res, res)
	}
	expected := "App v4.2.0, last user: Frank"
	if str != expected {
		t.Errorf("Expected %q, got %q", expected, str)
	}
}

// TestBoundFunctionVoid verifies that binding a void Go function works correctly.
func TestBoundFunctionVoid(t *testing.T) {
	vm := funxy.New()

	called := false
	vm.Bind("sideEffect", func() {
		called = true
	})

	_, err := vm.Eval("sideEffect()")
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	if !called {
		t.Error("Expected sideEffect() to be called")
	}
}

// TestBoundFunctionMultipleArgs verifies bound functions with multiple arguments.
func TestBoundFunctionMultipleArgs(t *testing.T) {
	vm := funxy.New()

	vm.Bind("add3", func(a, b, c int) int {
		return a + b + c
	})

	res, err := vm.Eval("add3(10, 20, 30)")
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	val, ok := res.(int)
	if !ok {
		if v64, ok64 := res.(int64); ok64 {
			val = int(v64)
		} else {
			t.Fatalf("Expected int, got %T", res)
		}
	}
	if val != 60 {
		t.Errorf("Expected 60, got %d", val)
	}
}

// TestSetAndGet verifies the Set/Get API for global variables.
func TestSetAndGet(t *testing.T) {
	vm := funxy.New()

	vm.Set("myValue", 42)

	res, err := vm.Get("myValue")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	val, ok := res.(int)
	if !ok {
		if v64, ok64 := res.(int64); ok64 {
			val = int(v64)
		} else {
			t.Fatalf("Expected int, got %T", res)
		}
	}
	if val != 42 {
		t.Errorf("Expected 42, got %d", val)
	}
}

// TestGetNonExistent verifies error when getting an undefined variable.
func TestGetNonExistent(t *testing.T) {
	vm := funxy.New()

	_, err := vm.Get("undefinedVar")
	if err == nil {
		t.Fatal("Expected error when getting undefined variable")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' in error, got: %v", err)
	}
}

// TestLoadFileWithBindingsAndCall mirrors the full embed_demo scenario:
// Bind objects → LoadFile → Call Funxy function → verify side effects.
func TestLoadFileWithBindingsAndCall(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "funxy_embed_demo_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mylib package
	pkgDir := filepath.Join(tmpDir, "mylib")
	if err := os.Mkdir(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	libCode := `package mylib(*)

fun format_bonus(bonus) {
	"*** BONUS: ${bonus} ***"
}

fun get_welcome_message() {
	"Welcome to the Funxy Embedding Demo!"
}
`
	if err := ioutil.WriteFile(filepath.Join(pkgDir, "mylib.lang"), []byte(libCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Create main script (use absolute path for import)
	mainCode := fmt.Sprintf(`import "%s"

logger(mylib.get_welcome_message())

current_version = appConfig.Version
logger("App Version: ${current_version}")

sum = calculator.Add(10, 20)
logger("10 + 20 = ${sum}")

fun process_user(name, score) {
	logger("Processing user: ${name}")
	appConfig.UpdateLastUser(name)
	bonus = calculator.Multiply(score, 2)
	formatted_bonus = mylib.format_bonus(bonus)
	"User ${name} processed. ${formatted_bonus}"
}
`, pkgDir)

	mainPath := filepath.Join(tmpDir, "script.lang")
	if err := ioutil.WriteFile(mainPath, []byte(mainCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Setup VM with bindings (mirroring embed_demo/main.go)
	vm := funxy.New()

	var logs []string
	vm.Bind("logger", func(msg string) {
		logs = append(logs, msg)
	})

	calc := &Calculator{BaseValue: 0}
	vm.Bind("calculator", calc)

	config := &AppConfig{Version: "1.0.0", LastUser: "None"}
	vm.Bind("appConfig", config)

	// Load and execute file
	err = vm.LoadFile(mainPath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	// Verify logger was called during LoadFile
	expectedLogs := []string{
		"Welcome to the Funxy Embedding Demo!",
		"App Version: 1.0.0",
		"10 + 20 = 30",
	}
	if len(logs) != len(expectedLogs) {
		t.Fatalf("After LoadFile: expected %d log entries, got %d: %v", len(expectedLogs), len(logs), logs)
	}
	for i, expected := range expectedLogs {
		if logs[i] != expected {
			t.Errorf("Log[%d]: expected %q, got %q", i, expected, logs[i])
		}
	}

	// Call Funxy function from Go
	result, err := vm.Call("process_user", "Alice", 50)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	str, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string, got %T: %v", result, result)
	}
	expected := "User Alice processed. *** BONUS: 100 ***"
	if str != expected {
		t.Errorf("Expected %q, got %q", expected, str)
	}

	// Verify Go struct was mutated by the Funxy function
	if config.LastUser != "Alice" {
		t.Errorf("Expected LastUser='Alice', got '%s'", config.LastUser)
	}

	// Verify logger was also called during Call
	if len(logs) != 4 {
		t.Errorf("Expected 4 total logs, got %d: %v", len(logs), logs)
	}
	if len(logs) >= 4 && logs[3] != "Processing user: Alice" {
		t.Errorf("Expected 'Processing user: Alice', got %q", logs[3])
	}
}

// TestEvalError verifies that syntax errors in Eval are properly reported.
func TestEvalError(t *testing.T) {
	vm := funxy.New()

	_, err := vm.Eval("1 + + 2")
	if err == nil {
		t.Fatal("Expected error for invalid syntax")
	}
}

// TestCalculatorBaseValue verifies that Calculator.Add respects BaseValue state.
func TestCalculatorBaseValue(t *testing.T) {
	vm := funxy.New()

	calc := &Calculator{BaseValue: 100}
	vm.Bind("calculator", calc)

	res, err := vm.Eval("calculator.Add(1, 2)")
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	val, ok := res.(int)
	if !ok {
		if v64, ok64 := res.(int64); ok64 {
			val = int(v64)
		} else {
			t.Fatalf("Expected int, got %T", res)
		}
	}
	// BaseValue(100) + 1 + 2 = 103
	if val != 103 {
		t.Errorf("Expected 103, got %d", val)
	}
}

// TestMultipleEvalsArithmetic verifies that Eval works for standalone expressions.
func TestMultipleEvalsArithmetic(t *testing.T) {
	vm := funxy.New()

	tests := []struct {
		expr     string
		expected int
	}{
		{"1 + 2", 3},
		{"10 * 5", 50},
		{"100 - 37", 63},
		{"42", 42},
	}

	for _, tc := range tests {
		res, err := vm.Eval(tc.expr)
		if err != nil {
			t.Fatalf("Eval(%q) failed: %v", tc.expr, err)
		}
		val, ok := res.(int)
		if !ok {
			if v64, ok64 := res.(int64); ok64 {
				val = int(v64)
			} else {
				t.Fatalf("Eval(%q): expected int, got %T", tc.expr, res)
			}
		}
		if val != tc.expected {
			t.Errorf("Eval(%q) = %d, want %d", tc.expr, val, tc.expected)
		}
	}
}

// TestCallWithReturnTypes verifies Call with different return types.
func TestCallWithReturnTypes(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "funxy_return_types_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Define functions in a file (Eval can't marshal closures)
	script := filepath.Join(tmpDir, "funcs.lang")
	if err := ioutil.WriteFile(script, []byte(`
fun getInt() { 42 }
fun getString() { "hello" }
fun getBool() { true }
fun getList() { [1, 2, 3] }
fun getRecord() { { name: "Alice", age: 30 } }
`), 0644); err != nil {
		t.Fatal(err)
	}

	vm := funxy.New()
	if err := vm.LoadFile(script); err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	// Test int return
	t.Run("int return", func(t *testing.T) {
		res, err := vm.Call("getInt")
		if err != nil {
			t.Fatalf("Call failed: %v", err)
		}
		val, ok := res.(int)
		if !ok {
			if v64, ok64 := res.(int64); ok64 {
				val = int(v64)
			} else {
				t.Fatalf("Expected int, got %T", res)
			}
		}
		if val != 42 {
			t.Errorf("Expected 42, got %d", val)
		}
	})

	// Test string return
	t.Run("string return", func(t *testing.T) {
		res, err := vm.Call("getString")
		if err != nil {
			t.Fatalf("Call failed: %v", err)
		}
		str, ok := res.(string)
		if !ok {
			t.Fatalf("Expected string, got %T", res)
		}
		if str != "hello" {
			t.Errorf("Expected 'hello', got '%s'", str)
		}
	})

	// Test bool return
	t.Run("bool return", func(t *testing.T) {
		res, err := vm.Call("getBool")
		if err != nil {
			t.Fatalf("Call failed: %v", err)
		}
		b, ok := res.(bool)
		if !ok {
			t.Fatalf("Expected bool, got %T", res)
		}
		if !b {
			t.Error("Expected true, got false")
		}
	})

	// Test list return
	t.Run("list return", func(t *testing.T) {
		res, err := vm.Call("getList")
		if err != nil {
			t.Fatalf("Call failed: %v", err)
		}
		list, ok := res.([]interface{})
		if !ok {
			t.Fatalf("Expected []interface{}, got %T", res)
		}
		if len(list) != 3 {
			t.Errorf("Expected 3 elements, got %d", len(list))
		}
	})

	// Test record return
	t.Run("record return", func(t *testing.T) {
		res, err := vm.Call("getRecord")
		if err != nil {
			t.Fatalf("Call failed: %v", err)
		}
		m, ok := res.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected map[string]interface{}, got %T", res)
		}
		if m["name"] != "Alice" {
			t.Errorf("Expected name='Alice', got %v", m["name"])
		}
	})
}

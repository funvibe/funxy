package funxy_test

import (
	"fmt"
	funxy "github.com/funvibe/funxy/pkg/embed"
	"io/ioutil"
	"os"
	"path/filepath"
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

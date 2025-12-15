package vm

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"testing"
)

func parse(t *testing.T, input string) *ast.Program {
	ctx := pipeline.NewPipelineContext(input)
	
	// Lexer
	l := lexer.LexerProcessor{}
	ctx = l.Process(ctx)
	if len(ctx.Errors) > 0 {
		t.Fatalf("lexer error: %s", ctx.Errors[0].Error())
	}
	
	// Parser
	p := parser.ParserProcessor{}
	ctx = p.Process(ctx)
	if len(ctx.Errors) > 0 {
		t.Fatalf("parser error: %s", ctx.Errors[0].Error())
	}
	
	return ctx.AstRoot.(*ast.Program)
}

func runVM(t *testing.T, input string) evaluator.Object {
	program := parse(t, input)

	compiler := NewCompiler()
	chunk, err := compiler.Compile(program)
	if err != nil {
		t.Fatalf("compilation error: %s", err)
	}

	vm := New()
	result, err := vm.Run(chunk)
	if err != nil {
		t.Fatalf("runtime error: %s", err)
	}

	return result
}

func runVMWithBuiltins(t *testing.T, input string) evaluator.Object {
	program := parse(t, input)

	compiler := NewCompiler()
	chunk, err := compiler.Compile(program)
	if err != nil {
		t.Fatalf("compilation error: %s", err)
	}

	vm := New()
	vm.RegisterBuiltins()
	result, err := vm.Run(chunk)
	if err != nil {
		t.Fatalf("runtime error: %s", err)
	}

	return result
}

func testIntegerObject(t *testing.T, obj evaluator.Object, expected int64) {
	result, ok := obj.(*evaluator.Integer)
	if !ok {
		t.Fatalf("object is not Integer. got=%T (%+v)", obj, obj)
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%d, want=%d", result.Value, expected)
	}
}

func testFloatObject(t *testing.T, obj evaluator.Object, expected float64) {
	result, ok := obj.(*evaluator.Float)
	if !ok {
		t.Fatalf("object is not Float. got=%T (%+v)", obj, obj)
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%f, want=%f", result.Value, expected)
	}
}

func testBooleanObject(t *testing.T, obj evaluator.Object, expected bool) {
	result, ok := obj.(*evaluator.Boolean)
	if !ok {
		t.Fatalf("object is not Boolean. got=%T (%+v)", obj, obj)
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%t, want=%t", result.Value, expected)
	}
}

// Phase 1 Tests: Calculator

func TestIntegerArithmetic(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"1", 1},
		{"2", 2},
		{"1 + 2", 3},
		{"1 - 2", -1},
		{"1 * 2", 2},
		{"4 / 2", 2},
		{"50 / 2 * 2 + 10 - 5", 55},
		{"5 + 5 + 5 + 5 - 10", 10},
		{"2 * 2 * 2 * 2 * 2", 32},
		{"5 * 2 + 10", 20},
		{"5 + 2 * 10", 25},
		{"5 * (2 + 10)", 60},
		{"-5", -5},
		{"-10", -10},
		{"-50 + 100 + -50", 0},
		{"(5 + 10 * 2 + 15 / 3) * 2 + -10", 50},
		{"10 % 3", 1},
		{"10 % 5", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := runVM(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}

func TestFloatArithmetic(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"1.5", 1.5},
		{"1.5 + 2.5", 4.0},
		{"3.0 - 1.5", 1.5},
		{"2.0 * 3.0", 6.0},
		{"6.0 / 2.0", 3.0},
		{"-1.5", -1.5},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := runVM(t, tt.input)
			testFloatObject(t, result, tt.expected)
		})
	}
}

func TestBooleanExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"1 < 2", true},
		{"1 > 2", false},
		{"1 < 1", false},
		{"1 > 1", false},
		{"1 == 1", true},
		{"1 != 1", false},
		{"1 == 2", false},
		{"1 != 2", true},
		{"true == true", true},
		{"false == false", true},
		{"true == false", false},
		{"true != false", true},
		{"false != true", true},
		{"(1 < 2) == true", true},
		{"(1 < 2) == false", false},
		{"(1 > 2) == true", false},
		{"(1 > 2) == false", true},
		{"!true", false},
		{"!false", true},
		{"!!true", true},
		{"!!false", false},
		{"1 <= 2", true},
		{"2 <= 2", true},
		{"3 <= 2", false},
		{"1 >= 2", false},
		{"2 >= 2", true},
		{"3 >= 2", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := runVM(t, tt.input)
			testBooleanObject(t, result, tt.expected)
		})
	}
}

func TestIfExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"if true { 10 }", int64(10)},
		{"if false { 10 }", nil},
		{"if 1 { 10 }", int64(10)}, // truthy
		{"if 1 < 2 { 10 }", int64(10)},
		{"if 1 > 2 { 10 }", nil},
		{"if 1 > 2 { 10 } else { 20 }", int64(20)},
		{"if 1 < 2 { 10 } else { 20 }", int64(10)},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := runVM(t, tt.input)
			if tt.expected == nil {
				if _, ok := result.(*evaluator.Nil); !ok {
					t.Fatalf("expected Nil, got=%T", result)
				}
			} else {
				testIntegerObject(t, result, tt.expected.(int64))
			}
		})
	}
}

// Phase 2 Tests: Variables

func TestGlobalVariables(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"x = 5\nx", 5},
		{"x = 5\ny = 10\nx + y", 15},
		{"x = 5\ny = x\ny", 5},
		{"x = 5\ny = x\nz = x + y\nz", 10},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := runVM(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}

func TestLocalVariables(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"{\nx = 5\nx\n}", 5},
		{"{\nx = 5\ny = 10\nx + y\n}", 15},
		{"{\na = 1\n{\nb = 2\na + b\n}\n}", 3},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := runVM(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}

func TestShadowing(t *testing.T) {
	input := `
	x = 10
	{
		x = 20
		x
	}
	`
	result := runVM(t, input)
	testIntegerObject(t, result, 20)
}

// Test disassembler
func TestDisassembler(t *testing.T) {
	input := "1 + 2 * 3"
	program := parse(t, input)
	
	compiler := NewCompiler()
	chunk, err := compiler.Compile(program)
	if err != nil {
		t.Fatalf("compilation error: %s", err)
	}
	
	output := Disassemble(chunk, "test")
	if output == "" {
		t.Fatal("disassembler produced empty output")
	}
	
	// Should contain these opcodes
	expectedParts := []string{"CONST", "MUL", "ADD", "HALT"}
	for _, part := range expectedParts {
		if !containsString(output, part) {
			t.Errorf("disassembly missing %s:\n%s", part, output)
		}
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || containsString(s[1:], substr)))
}

// Phase 3 Tests: Functions

func TestSimpleFunction(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			"no args",
			`fun five() { 5 }
			five()`,
			5,
		},
		{
			"one arg",
			`fun double(x) { x * 2 }
			double(5)`,
			10,
		},
		{
			"two args",
			`fun add(a, b) { a + b }
			add(3, 4)`,
			7,
		},
		{
			"three args",
			`fun sum3(a, b, c) { a + b + c }
			sum3(1, 2, 3)`,
			6,
		},
		{
			"nested calls",
			`fun double(x) { x * 2 }
			double(double(5))`,
			20,
		},
		{
			"multiple functions",
			`fun add(a, b) { a + b }
			fun mul(a, b) { a * b }
			add(mul(2, 3), mul(4, 5))`,
			26,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVM(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}

func TestFunctionWithLocals(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			"local in function",
			`fun calc(x) {
				y = x * 2
				y + 1
			}
			calc(5)`,
			11,
		},
		{
			"multiple locals",
			`fun calc(x) {
				a = x + 1
				b = a * 2
				c = b - 3
				c
			}
			calc(5)`,
			9,
		},
		{
			"shadowing param",
			`fun test(x) {
				x = x + 10
				x
			}
			test(5)`,
			15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVM(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}

func TestRecursiveFunction(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			"factorial",
			`fun factorial(n) {
				if n <= 1 { 1 }
				else { n * factorial(n - 1) }
			}
			factorial(5)`,
			120,
		},
		{
			"fibonacci",
			`fun fib(n) {
				if n < 2 { n }
				else { fib(n - 1) + fib(n - 2) }
			}
			fib(10)`,
			55,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVM(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}

// Logical operators
func TestLogicalOperators(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true && true", true},
		{"true && false", false},
		{"false && true", false},
		{"false && false", false},
		{"true || true", true},
		{"true || false", true},
		{"false || true", true},
		{"false || false", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := runVM(t, tt.input)
			testBooleanObject(t, result, tt.expected)
		})
	}
}

// Nested scopes
func TestNestedScopes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			"simple nested",
			`{
				a = 1
				{
					b = 2
					a + b
				}
			}`,
			3,
		},
		{
			"triple nested",
			`{
				a = 1
				{
					b = 2
					{
						c = 3
						a + b + c
					}
				}
			}`,
			6,
		},
		{
			"shadow in nested",
			`{
				x = 10
				{
					x = 20
					{
						x = 30
						x
					}
				}
			}`,
			30,
		},
		{
			"access outer after inner",
			`{
				a = 100
				{
					b = 50
					b
				}
				a
			}`,
			100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVM(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}

// Complex expressions
func TestComplexExpressions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			"fibonacci manual",
			`{
				a = 0
				b = 1
				c = a + b
				d = b + c
				e = c + d
				f = d + e
				f
			}`,
			5,
		},
		{
			"conditional in block",
			`{
				x = 10
				y = if x > 5 { x * 2 } else { x }
				y
			}`,
			20,
		},
		{
			"multiple conditions",
			`{
				a = 5
				b = 10
				c = if a < b { 1 } else { 0 }
				d = if b < a { 1 } else { 0 }
				c + d
			}`,
			1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVM(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}

// Global and local interaction
func TestGlobalLocalInteraction(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			"global then local",
			`
			g = 100
			{
				l = 50
				g + l
			}
			`,
			150,
		},
		{
			"shadow global in local",
			`
			x = 1
			{
				x = 2
				x
			}
			`,
			2,
		},
		{
			"use global after local scope",
			`
			g = 42
			{
				l = 10
				l
			}
			g
			`,
			42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVM(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}

// Test reassignment
func TestReassignment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			"global reassignment",
			`
			x = 10
			x = 20
			x
			`,
			20,
		},
		{
			"local reassignment",
			`{
				x = 10
				x = 20
				x
			}`,
			20,
		},
		{
			"reassign with expression",
			`{
				x = 5
				x = x + 10
				x
			}`,
			15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVM(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}

// Test closures - the most important phase 4 tests
func TestClosures(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			"simple closure",
			`
			fun outer() {
				x = 10
				fun inner() { x }
				inner()
			}
			outer()
			`,
			10,
		},
		{
			"closure captures parameter",
			`
			fun makeAdder(x) {
				fun add(y) { x + y }
				add
			}
			adder = makeAdder(5)
			adder(3)
			`,
			8,
		},
		{
			"closure captures multiple values",
			`
			fun outer() {
				a = 1
				b = 2
				c = 3
				fun inner() { a + b + c }
				inner()
			}
			outer()
			`,
			6,
		},
		{
			"nested closures",
			`
			fun outer() {
				x = 10
				fun middle() {
					y = 20
					fun inner() { x + y }
					inner()
				}
				middle()
			}
			outer()
			`,
			30,
		},
		{
			"closure survives outer function return",
			`
			fun makeCounter() {
				count = 0
				fun increment() {
					count = count + 1
					count
				}
				increment
			}
			counter = makeCounter()
			counter()
			counter()
			counter()
			`,
			3,
		},
		{
			"multiple closures share variable",
			`
			fun makeCounters() {
				count = 0
				fun inc() {
					count = count + 1
					count
				}
				fun get() { count }
				inc()
				inc()
				get()
			}
			makeCounters()
			`,
			2,
		},
		{
			"closure with shadowing",
			`
			fun outer() {
				x = 10
				fun inner() {
					x = 20
					x
				}
				inner()
			}
			outer()
			`,
			20,
		},
		{
			"closure with block scope",
			`
			fun outer() {
				result = 0
				{
					x = 5
					fun inner() { x }
					result = inner()
				}
				result
			}
			outer()
			`,
			5,
		},
		{
			"closure captures after modification",
			`
			fun outer() {
				x = 1
				fun get() { x }
				x = 2
				get()
			}
			outer()
			`,
			2,
		},
		{
			"deeply nested closure",
			`
			fun level1() {
				a = 1
				fun level2() {
					b = 2
					fun level3() {
						c = 3
						fun level4() { a + b + c }
						level4()
					}
					level3()
				}
				level2()
			}
			level1()
			`,
			6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVM(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}

// Test counter pattern (classic closure test)
func TestCounterPattern(t *testing.T) {
	input := `
	fun makeCounter() {
		count = 0
		fun inc() {
			count = count + 1
			count
		}
		inc
	}
	
	c1 = makeCounter()
	c2 = makeCounter()
	
	c1() // 1
	c1() // 2
	c2() // 1 (independent counter)
	c1() // 3
	`
	result := runVM(t, input)
	testIntegerObject(t, result, 3)
}

// Test fibonacci with closure (for benchmarking)
func TestFibonacciClosure(t *testing.T) {
	input := `
	fun fib(n) {
		if n < 2 { n }
		else { fib(n - 1) + fib(n - 2) }
	}
	fib(20)
	`
	result := runVM(t, input)
	testIntegerObject(t, result, 6765)
}

// Test while loops
func TestWhileLoops(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			"simple while loop",
			`
			i = 0
			for i < 5 {
				i = i + 1
			}
			`,
			5,
		},
		{
			"while with accumulator",
			`
			sum = 0
			i = 1
			for i <= 10 {
				sum = sum + i
				i = i + 1
			}
			sum
			`,
			55,
		},
		{
			"while returns last body value",
			`
			i = 0
			for i < 3 {
				i = i + 1
				i * 10
			}
			`,
			30,
		},
		{
			"while with zero iterations",
			`
			x = 100
			for false {
				x = 0
			}
			x
			`,
			100,
		},
		{
			"nested while loops",
			`
			result = 0
			i = 0
			for i < 3 {
				j = 0
				for j < 3 {
					result = result + 1
					j = j + 1
				}
				i = i + 1
			}
			result
			`,
			9,
		},
		{
			"while in function",
			`
			fun sumTo(n) {
				sum = 0
				i = 1
				for i <= n {
					sum = sum + i
					i = i + 1
				}
				sum
			}
			sumTo(10)
			`,
			55,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVM(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}

// Test break statement
func TestBreakStatement(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			"simple break",
			`
			i = 0
			for true {
				i = i + 1
				if i == 5 { break }
			}
			i
			`,
			5,
		},
		{
			"break with value",
			`
			i = 0
			for true {
				i = i + 1
				if i == 5 { break i * 10 }
			}
			`,
			50,
		},
		{
			"break in nested loop",
			`
			result = 0
			i = 0
			for i < 10 {
				j = 0
				for true {
					j = j + 1
					if j == 3 { break }
				}
				result = result + j
				i = i + 1
			}
			result
			`,
			30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVM(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}

// Test continue statement
func TestContinueStatement(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			"simple continue",
			`
			sum = 0
			i = 0
			for i < 10 {
				i = i + 1
				if i % 2 == 0 { continue }
				sum = sum + i
			}
			sum
			`,
			25, // 1 + 3 + 5 + 7 + 9 = 25
		},
		{
			"continue skips rest of body",
			`
			count = 0
			i = 0
			for i < 5 {
				i = i + 1
				if i == 3 { continue }
				count = count + 1
			}
			count
			`,
			4, // skips when i == 3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVM(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}

// Test loop performance (iterative fibonacci)
func TestIterativeFibonacci(t *testing.T) {
	input := `
	fun fib(n) {
		if n < 2 { n }
		else {
			a = 0
			b = 1
			i = 2
			for i <= n {
				temp = a + b
				a = b
				b = temp
				i = i + 1
			}
			b
		}
	}
	fib(35)
	`
	result := runVM(t, input)
	testIntegerObject(t, result, 9227465)
}

// Test builtin functions
func TestBuiltins(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			"length of list",
			`length([1, 2, 3, 4, 5])`,
			5,
		},
		{
			"head of list",
			`head([1, 2, 3])`,
			1,
		},
		{
			"range function",
			`length(range(0, 10))`,
			10,
		},
		{
			"absInt function",
			`absInt(-42)`,
			42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVMWithBuiltins(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}

// Test Option type constructors
func TestOptionConstructors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"Some constructor",
			`Some(42)`,
			"Some(42)",
		},
		{
			"Zero constructor",
			`Zero`,
			"Zero",
		},
		{
			"isSome with Some",
			`isSome(Some(10))`,
			"true",
		},
		{
			"isSome with Zero",
			`isSome(Zero)`,
			"false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVMWithBuiltins(t, tt.input)
			if result.Inspect() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result.Inspect())
			}
		})
	}
}

// Test higher-order builtins
func TestHigherOrderBuiltins(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			"map with function",
			`
			fun double(x) { x * 2 }
			fun add(a, b) { a + b }
			foldl(add, 0, map(double, [1, 2, 3]))
			`,
			12, // 2 + 4 + 6
		},
		{
			"filter with function",
			`
			fun isEven(x) { x % 2 == 0 }
			length(filter(isEven, [1, 2, 3, 4, 5, 6]))
			`,
			3,
		},
		{
			"foldl with function",
			`
			fun add(a, b) { a + b }
			foldl(add, 0, [1, 2, 3, 4, 5])
			`,
			15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVMWithBuiltins(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}

// Test pattern matching with literals and wildcards
func TestPatternMatchingBasic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			"match with wildcard",
			`
			match 42 {
				_ -> 1
			}
			`,
			1,
		},
		{
			"match with literal",
			`
			match 10 {
				10 -> 100
				_ -> 0
			}
			`,
			100,
		},
		{
			"match with variable binding",
			`
			match 5 {
				x -> x * 2
			}
			`,
			10,
		},
		{
			"match multiple arms",
			`
			x = 2
			match x {
				1 -> 10
				2 -> 20
				3 -> 30
				_ -> 0
			}
			`,
			20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVM(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}

// Test pattern matching with Option type (Some/Zero)
func TestPatternMatchingOption(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			"match Some and extract value",
			`
			opt = Some(42)
			match opt {
				Some(x) -> x
				Zero -> 0
			}
			`,
			42,
		},
		{
			"match Zero",
			`
			opt = Zero
			match opt {
				Some(x) -> x
				Zero -> -1
			}
			`,
			-1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVMWithBuiltins(t, tt.input)
			testIntegerObject(t, result, tt.expected)
		})
	}
}


package evaluator

import (
	"fmt"
	"github.com/funvibe/funxy/internal/typesystem"
	"regexp"
	"strings"
	"sync"
)

// ============================================================================
// Test Runner State (global for the test session)
// ============================================================================

// TestResult represents the outcome of a single test
type TestResult struct {
	Name       string
	Passed     bool
	Skipped    bool
	ExpectFail bool // True if test was marked as expected to fail
	Error      string
}

// TestRunner manages test execution state
type TestRunner struct {
	mu          sync.Mutex
	Results     []TestResult
	CurrentTest string
	Evaluator   *Evaluator

	// HTTP mocks: pattern -> response or error
	HttpMocks       map[string]Object // pattern -> HttpResponse record
	HttpMockErrors  map[string]string // pattern -> error message
	HttpMocksActive bool
	HttpBypass      bool // temporary bypass flag

	// File mocks: path -> Result<String, String>
	FileMocks       map[string]Object
	FileMocksActive bool
	FileBypass      bool

	// Env mocks: key -> value
	EnvMocks       map[string]string
	EnvMocksActive bool
	EnvBypass      bool
}

// Global test runner instance
var (
	testRunner     *TestRunner
	testRunnerOnce sync.Once
)

// InitTestRunner creates or resets the test runner
func InitTestRunner(e *Evaluator) {
	testRunnerOnce.Do(func() {
		testRunner = &TestRunner{
			Evaluator:       e,
			HttpMocks:       make(map[string]Object),
			HttpMockErrors:  make(map[string]string),
			HttpMocksActive: false,
			FileMocks:       make(map[string]Object),
			FileMocksActive: false,
			EnvMocks:        make(map[string]string),
			EnvMocksActive:  false,
			Results:         make([]TestResult, 0),
		}
	})
	// If already initialized, update evaluator and reset state completely
	if testRunner != nil {
		testRunner.Evaluator = e
		testRunner.ResetMocks()
		testRunner.ClearResults() // Fix: Clear previous results to avoid pollution
	}
}

// GetTestRunner returns the global test runner (creates if needed)
func GetTestRunner() *TestRunner {
	testRunnerOnce.Do(func() {
		testRunner = &TestRunner{
			HttpMocks:      make(map[string]Object),
			HttpMockErrors: make(map[string]string),
			FileMocks:      make(map[string]Object),
			EnvMocks:       make(map[string]string),
			Results:        make([]TestResult, 0),
		}
	})
	return testRunner
}

// ClearResults clears the accumulated test results
func (tr *TestRunner) ClearResults() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.Results = make([]TestResult, 0)
}

// ResetMocks clears all mocks (called after each test)
func (tr *TestRunner) ResetMocks() {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.HttpMocks = make(map[string]Object)
	tr.HttpMockErrors = make(map[string]string)
	tr.HttpMocksActive = false
	tr.HttpBypass = false

	tr.FileMocks = make(map[string]Object)
	tr.FileMocksActive = false
	tr.FileBypass = false

	tr.EnvMocks = make(map[string]string)
	tr.EnvMocksActive = false
	tr.EnvBypass = false
}

// ============================================================================
// Mock Pattern Matching
// ============================================================================

// matchPattern checks if a URL/path matches a glob pattern
// Supports * for single path segment and ** for multiple segments
func matchPattern(pattern, value string) bool {
	// Convert glob pattern to regex
	// * matches anything except /
	// ** matches anything including /
	regexPattern := regexp.QuoteMeta(pattern)
	regexPattern = strings.ReplaceAll(regexPattern, `\*\*`, `.*`)
	regexPattern = strings.ReplaceAll(regexPattern, `\*`, `[^/]*`)
	regexPattern = "^" + regexPattern + "$"

	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return false
	}
	return re.MatchString(value)
}

// FindHttpMock looks for a matching HTTP mock for the given URL
func (tr *TestRunner) FindHttpMock(url string) (Object, bool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if !tr.HttpMocksActive || tr.HttpBypass {
		return nil, false
	}

	for pattern, response := range tr.HttpMocks {
		if matchPattern(pattern, url) {
			return response, true
		}
	}
	return nil, false
}

// FindHttpMockError looks for a matching HTTP error mock
func (tr *TestRunner) FindHttpMockError(url string) (string, bool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if !tr.HttpMocksActive || tr.HttpBypass {
		return "", false
	}

	for pattern, errMsg := range tr.HttpMockErrors {
		if matchPattern(pattern, url) {
			return errMsg, true
		}
	}
	return "", false
}

// ShouldBlockHttp returns true if HTTP should be blocked (mocks active, no match)
func (tr *TestRunner) ShouldBlockHttp(url string) bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if !tr.HttpMocksActive || tr.HttpBypass {
		return false
	}

	// If mocks are active but no match found, block
	for pattern := range tr.HttpMocks {
		if matchPattern(pattern, url) {
			return false
		}
	}
	for pattern := range tr.HttpMockErrors {
		if matchPattern(pattern, url) {
			return false
		}
	}
	return true
}

// FindFileMock looks for a matching file mock
func (tr *TestRunner) FindFileMock(path string) (Object, bool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if !tr.FileMocksActive || tr.FileBypass {
		return nil, false
	}

	for pattern, result := range tr.FileMocks {
		if matchPattern(pattern, path) {
			return result, true
		}
	}
	return nil, false
}

// ShouldBlockFile returns true if file ops should be blocked
func (tr *TestRunner) ShouldBlockFile(path string) bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if !tr.FileMocksActive || tr.FileBypass {
		return false
	}

	for pattern := range tr.FileMocks {
		if matchPattern(pattern, path) {
			return false
		}
	}
	return true
}

// FindEnvMock looks for a matching env mock
func (tr *TestRunner) FindEnvMock(key string) (string, bool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if !tr.EnvMocksActive || tr.EnvBypass {
		return "", false
	}

	if val, ok := tr.EnvMocks[key]; ok {
		return val, true
	}
	return "", false
}

// ShouldBlockEnv returns true if env should be blocked
func (tr *TestRunner) ShouldBlockEnv(key string) bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if !tr.EnvMocksActive || tr.EnvBypass {
		return false
	}

	if _, ok := tr.EnvMocks[key]; ok {
		return false
	}
	return true
}

// ============================================================================
// Test Builtins
// ============================================================================

// TestBuiltins returns built-in functions for lib/test virtual package
func TestBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// Test definition
		"testRun":        {Fn: builtinTestRun, Name: "testRun"},
		"testSkip":       {Fn: builtinTestSkip, Name: "testSkip"},
		"testExpectFail": {Fn: builtinTestExpectFail, Name: "testExpectFail"},

		// Assertions
		"assert":       {Fn: builtinAssert, Name: "assert"},
		"assertTrue":   {Fn: builtinAssertTrue, Name: "assertTrue"},
		"assertFalse":  {Fn: builtinAssertFalse, Name: "assertFalse"},
		"assertEquals": {Fn: builtinAssertEquals, Name: "assertEquals"},
		"assertOk":     {Fn: builtinAssertOk, Name: "assertOk"},
		"assertFail":   {Fn: builtinAssertFail, Name: "assertFail"},
		"assertSome":   {Fn: builtinAssertSome, Name: "assertSome"},
		"assertNone":   {Fn: builtinAssertNone, Name: "assertNone"},

		// HTTP mocks
		"mockHttp":       {Fn: builtinMockHttp, Name: "mockHttp"},
		"mockHttpError":  {Fn: builtinMockHttpError, Name: "mockHttpError"},
		"mockHttpOff":    {Fn: builtinMockHttpOff, Name: "mockHttpOff"},
		"mockHttpBypass": {Fn: builtinMockHttpBypass, Name: "mockHttpBypass"},

		// File mocks
		"mockFile":       {Fn: builtinMockFile, Name: "mockFile"},
		"mockFileOff":    {Fn: builtinMockFileOff, Name: "mockFileOff"},
		"mockFileBypass": {Fn: builtinMockFileBypass, Name: "mockFileBypass"},

		// Env mocks
		"mockEnv":       {Fn: builtinMockEnv, Name: "mockEnv"},
		"mockEnvOff":    {Fn: builtinMockEnvOff, Name: "mockEnvOff"},
		"mockEnvBypass": {Fn: builtinMockEnvBypass, Name: "mockEnvBypass"},
	}
}

// testRun(name: String, body: () -> Nil) -> Nil
func builtinTestRun(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("testRun expects 2 arguments, got %d", len(args))
	}

	nameList, ok := args[0].(*List)
	if !ok {
		return newError("testRun expects a string name, got %s", args[0].Type())
	}
	testName := listToString(nameList)

	// Body can be a Function or something callable
	body := args[1]

	tr := GetTestRunner()
	tr.CurrentTest = testName

	// Run the test body
	result := e.ApplyFunction(body, []Object{})

	// Record result
	testResult := TestResult{Name: testName, Passed: true}

	if result != nil {
		if errObj, ok := result.(*Error); ok {
			testResult.Passed = false
			testResult.Error = errObj.Message
		}
	}

	tr.Results = append(tr.Results, testResult)

	// Reset mocks after each test
	tr.ResetMocks()

	// Print result using evaluator's output writer
	if testResult.Passed {
		_, _ = fmt.Fprintf(e.Out, "✓ %s\n", testName)
	} else {
		_, _ = fmt.Fprintf(e.Out, "✗ %s: %s\n", testName, testResult.Error)
	}

	return &Nil{}
}

// testSkip(reason: String) -> Nil
func builtinTestSkip(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("testSkip expects 1 argument, got %d", len(args))
	}

	reasonList, ok := args[0].(*List)
	if !ok {
		return newError("testSkip expects a string reason, got %s", args[0].Type())
	}
	reason := listToString(reasonList)

	tr := GetTestRunner()
	testResult := TestResult{
		Name:    tr.CurrentTest,
		Passed:  true,
		Skipped: true,
		Error:   reason,
	}
	tr.Results = append(tr.Results, testResult)

	_, _ = fmt.Fprintf(e.Out, "⊘ %s (skipped: %s)\n", tr.CurrentTest, reason)

	return &Nil{}
}

// testExpectFail(name: String, body: () -> Nil) -> Nil
// Test passes if body throws an error, fails if body succeeds
func builtinTestExpectFail(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("testExpectFail expects 2 arguments, got %d", len(args))
	}

	nameList, ok := args[0].(*List)
	if !ok {
		return newError("testExpectFail expects a string name, got %s", args[0].Type())
	}
	testName := listToString(nameList)

	body := args[1]

	tr := GetTestRunner()
	tr.CurrentTest = testName

	// Run the test body
	result := e.ApplyFunction(body, []Object{})

	// Record result - opposite logic: pass if error, fail if success
	testResult := TestResult{Name: testName, ExpectFail: true}

	if result != nil {
		if errObj, ok := result.(*Error); ok {
			// Body threw an error - this is expected, test passes
			testResult.Passed = true
			testResult.Error = errObj.Message
		} else {
			// Body returned normally - unexpected, test fails
			testResult.Passed = false
			testResult.Error = "expected test to fail, but it passed"
		}
	} else {
		// Body returned nil (success) - unexpected, test fails
		testResult.Passed = false
		testResult.Error = "expected test to fail, but it passed"
	}

	tr.Results = append(tr.Results, testResult)
	tr.ResetMocks()

	// Print result
	if testResult.Passed {
		_, _ = fmt.Fprintf(e.Out, "⚠ %s (expected fail: %s)\n", testName, testResult.Error)
	} else {
		_, _ = fmt.Fprintf(e.Out, "✗ %s: %s\n", testName, testResult.Error)
	}

	return &Nil{}
}

// extractMessage extracts optional message from args
func extractMessage(args []Object, startIdx int) string {
	if len(args) > startIdx {
		if list, ok := args[startIdx].(*List); ok {
			return listToString(list)
		}
	}
	return ""
}

// assert(condition: Bool, msg?: String) -> Nil
func builtinAssert(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("assert expects 1-2 arguments, got %d", len(args))
	}

	boolVal, ok := args[0].(*Boolean)
	if !ok {
		return newError("assert expects a boolean, got %s", args[0].Type())
	}

	if !boolVal.Value {
		msg := extractMessage(args, 1)
		if msg != "" {
			return newError("assertion failed: %s", msg)
		}
		return newError("assertion failed")
	}

	return &Nil{}
}

func builtinAssertTrue(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("assertTrue expects 1-2 arguments, got %d", len(args))
	}

	boolVal, ok := args[0].(*Boolean)
	if !ok {
		return newError("assertTrue expects a boolean, got %s", args[0].Type())
	}

	if !boolVal.Value {
		msg := extractMessage(args, 1)
		if msg != "" {
			return newError("assertTrue failed: expected true, got false - %s", msg)
		}
		return newError("assertTrue failed: expected true, got false")
	}

	return &Nil{}
}

func builtinAssertFalse(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("assertFalse expects 1-2 arguments, got %d", len(args))
	}

	boolVal, ok := args[0].(*Boolean)
	if !ok {
		return newError("assertFalse expects a boolean, got %s", args[0].Type())
	}

	if boolVal.Value {
		msg := extractMessage(args, 1)
		if msg != "" {
			return newError("assertFalse failed: expected false, got true - %s", msg)
		}
		return newError("assertFalse failed: expected false, got true")
	}

	return &Nil{}
}

// assertEquals(expected: T, actual: T, msg?: String) -> Nil
func builtinAssertEquals(e *Evaluator, args ...Object) Object {
	if len(args) < 2 || len(args) > 3 {
		return newError("assertEquals expects 2-3 arguments, got %d", len(args))
	}

	expected := args[0]
	actual := args[1]

	if !objectsEqual(expected, actual) {
		msg := extractMessage(args, 2)
		if msg != "" {
			return newError("assertion failed: %s (expected %s, got %s)", msg, expected.Inspect(), actual.Inspect())
		}
		return newError("assertion failed: expected %s, got %s", expected.Inspect(), actual.Inspect())
	}

	return &Nil{}
}

// assertOk(result: Result<E, T>, msg?: String) -> Nil
func builtinAssertOk(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("assertOk expects 1-2 arguments, got %d", len(args))
	}

	di, ok := args[0].(*DataInstance)
	if !ok {
		return newError("assertOk expects a Result, got %s", args[0].Type())
	}

	if di.Name != "Ok" {
		errVal := ""
		if len(di.Fields) > 0 {
			errVal = di.Fields[0].Inspect()
		}
		msg := extractMessage(args, 1)
		if msg != "" {
			return newError("assertion failed: %s (expected Ok, got Fail(%s))", msg, errVal)
		}
		return newError("assertion failed: expected Ok, got Fail(%s)", errVal)
	}

	return &Nil{}
}

// assertFail(result: Result<E, T>, msg?: String) -> Nil
func builtinAssertFail(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("assertFail expects 1-2 arguments, got %d", len(args))
	}

	di, ok := args[0].(*DataInstance)
	if !ok {
		return newError("assertFail expects a Result, got %s", args[0].Type())
	}

	if di.Name != "Fail" {
		okVal := ""
		if len(di.Fields) > 0 {
			okVal = di.Fields[0].Inspect()
		}
		msg := extractMessage(args, 1)
		if msg != "" {
			return newError("assertion failed: %s (expected Fail, got Ok(%s))", msg, okVal)
		}
		return newError("assertion failed: expected Fail, got Ok(%s)", okVal)
	}

	return &Nil{}
}

// assertSome(option: Option<T>, msg?: String) -> Nil
func builtinAssertSome(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("assertSome expects 1-2 arguments, got %d", len(args))
	}

	di, ok := args[0].(*DataInstance)
	if !ok {
		return newError("assertSome expects an Option, got %s", args[0].Type())
	}

	if di.Name != "Some" {
		msg := extractMessage(args, 1)
		if msg != "" {
			return newError("assertion failed: %s (expected Some, got None)", msg)
		}
		return newError("assertion failed: expected Some, got None")
	}

	return &Nil{}
}

// assertNone(option: Option<T>, msg?: String) -> Nil
func builtinAssertNone(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("assertNone expects 1-2 arguments, got %d", len(args))
	}

	di, ok := args[0].(*DataInstance)
	if !ok {
		return newError("assertNone expects an Option, got %s", args[0].Type())
	}

	if di.Name != "None" {
		val := ""
		if len(di.Fields) > 0 {
			val = di.Fields[0].Inspect()
		}
		msg := extractMessage(args, 1)
		if msg != "" {
			return newError("assertion failed: %s (expected None, got Some(%s))", msg, val)
		}
		return newError("assertion failed: expected None, got Some(%s)", val)
	}

	return &Nil{}
}

// mockHttp(pattern: String, response: HttpResponse) -> Nil
func builtinMockHttp(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("mockHttp expects 2 arguments, got %d", len(args))
	}

	patternList, ok := args[0].(*List)
	if !ok {
		return newError("mockHttp expects a string pattern, got %s", args[0].Type())
	}
	pattern := listToString(patternList)

	response := args[1]

	tr := GetTestRunner()
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.HttpMocks[pattern] = response
	tr.HttpMocksActive = true

	return &Nil{}
}

// mockHttpError(pattern: String, error: String) -> Nil
func builtinMockHttpError(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("mockHttpError expects 2 arguments, got %d", len(args))
	}

	patternList, ok := args[0].(*List)
	if !ok {
		return newError("mockHttpError expects a string pattern, got %s", args[0].Type())
	}
	pattern := listToString(patternList)

	errList, ok := args[1].(*List)
	if !ok {
		return newError("mockHttpError expects a string error, got %s", args[1].Type())
	}
	errMsg := listToString(errList)

	tr := GetTestRunner()
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.HttpMockErrors[pattern] = errMsg
	tr.HttpMocksActive = true

	return &Nil{}
}

// mockHttpOff() -> Nil
func builtinMockHttpOff(e *Evaluator, args ...Object) Object {
	tr := GetTestRunner()
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.HttpMocks = make(map[string]Object)
	tr.HttpMockErrors = make(map[string]string)
	tr.HttpMocksActive = false

	return &Nil{}
}

// mockHttpBypass(call: A) -> A
func builtinMockHttpBypass(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("mockHttpBypass expects 1 argument, got %d", len(args))
	}

	tr := GetTestRunner()

	// Set bypass flag temporarily
	tr.mu.Lock()
	tr.HttpBypass = true
	tr.mu.Unlock()

	// The argument is already evaluated (the HTTP call result)
	result := args[0]

	// Reset bypass flag
	tr.mu.Lock()
	tr.HttpBypass = false
	tr.mu.Unlock()

	return result
}

// mockFile(pattern: String, result: Result<String, String>) -> Nil
func builtinMockFile(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("mockFile expects 2 arguments, got %d", len(args))
	}

	patternList, ok := args[0].(*List)
	if !ok {
		return newError("mockFile expects a string pattern, got %s", args[0].Type())
	}
	pattern := listToString(patternList)

	result := args[1]

	tr := GetTestRunner()
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.FileMocks[pattern] = result
	tr.FileMocksActive = true

	return &Nil{}
}

// mockFileOff() -> Nil
func builtinMockFileOff(e *Evaluator, args ...Object) Object {
	tr := GetTestRunner()
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.FileMocks = make(map[string]Object)
	tr.FileMocksActive = false

	return &Nil{}
}

// mockFileBypass(call: A) -> A
func builtinMockFileBypass(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("mockFileBypass expects 1 argument, got %d", len(args))
	}

	tr := GetTestRunner()

	tr.mu.Lock()
	tr.FileBypass = true
	tr.mu.Unlock()

	result := args[0]

	tr.mu.Lock()
	tr.FileBypass = false
	tr.mu.Unlock()

	return result
}

// mockEnv(key: String, value: String) -> Nil
func builtinMockEnv(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("mockEnv expects 2 arguments, got %d", len(args))
	}

	keyList, ok := args[0].(*List)
	if !ok {
		return newError("mockEnv expects a string key, got %s", args[0].Type())
	}
	key := listToString(keyList)

	valList, ok := args[1].(*List)
	if !ok {
		return newError("mockEnv expects a string value, got %s", args[1].Type())
	}
	val := listToString(valList)

	tr := GetTestRunner()
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.EnvMocks[key] = val
	tr.EnvMocksActive = true

	return &Nil{}
}

// mockEnvOff() -> Nil
func builtinMockEnvOff(e *Evaluator, args ...Object) Object {
	tr := GetTestRunner()
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.EnvMocks = make(map[string]string)
	tr.EnvMocksActive = false

	return &Nil{}
}

// mockEnvBypass(call: A) -> A
func builtinMockEnvBypass(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("mockEnvBypass expects 1 argument, got %d", len(args))
	}

	tr := GetTestRunner()

	tr.mu.Lock()
	tr.EnvBypass = true
	tr.mu.Unlock()

	result := args[0]

	tr.mu.Lock()
	tr.EnvBypass = false
	tr.mu.Unlock()

	return result
}

// SetTestBuiltinTypes sets type info for test builtins
func SetTestBuiltinTypes(builtins map[string]*Builtin) {
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}

	T := typesystem.TVar{Name: "T"}
	A := typesystem.TVar{Name: "A"}
	E := typesystem.TVar{Name: "E"}

	optionT := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{T},
	}

	resultTE := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{T, E},
	}

	resultAString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{A, stringType},
	}

	headerTuple := typesystem.TTuple{
		Elements: []typesystem.Type{stringType, stringType},
	}
	headersType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{headerTuple},
	}
	responseType := typesystem.TRecord{
		Fields: map[string]typesystem.Type{
			"status":  typesystem.Int,
			"body":    stringType,
			"headers": headersType,
		},
	}

	testBodyType := typesystem.TFunc{
		Params:     []typesystem.Type{},
		ReturnType: typesystem.Nil,
	}

	types := map[string]typesystem.Type{
		"testRun":        typesystem.TFunc{Params: []typesystem.Type{stringType, testBodyType}, ReturnType: typesystem.Nil},
		"testSkip":       typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Nil},
		"testExpectFail": typesystem.TFunc{Params: []typesystem.Type{stringType, testBodyType}, ReturnType: typesystem.Nil},
		"assert":         typesystem.TFunc{Params: []typesystem.Type{typesystem.Bool, stringType}, ReturnType: typesystem.Nil, IsVariadic: true},
		"assertTrue":     typesystem.TFunc{Params: []typesystem.Type{typesystem.Bool, stringType}, ReturnType: typesystem.Nil, IsVariadic: true},
		"assertFalse":    typesystem.TFunc{Params: []typesystem.Type{typesystem.Bool, stringType}, ReturnType: typesystem.Nil, IsVariadic: true},
		"assertEquals":   typesystem.TFunc{Params: []typesystem.Type{T, T, stringType}, ReturnType: typesystem.Nil, IsVariadic: true},
		"assertOk":       typesystem.TFunc{Params: []typesystem.Type{resultTE, stringType}, ReturnType: typesystem.Nil, IsVariadic: true},
		"assertFail":     typesystem.TFunc{Params: []typesystem.Type{resultTE, stringType}, ReturnType: typesystem.Nil, IsVariadic: true},
		"assertSome":     typesystem.TFunc{Params: []typesystem.Type{optionT, stringType}, ReturnType: typesystem.Nil, IsVariadic: true},
		"assertNone":     typesystem.TFunc{Params: []typesystem.Type{optionT, stringType}, ReturnType: typesystem.Nil, IsVariadic: true},
		"mockHttp":       typesystem.TFunc{Params: []typesystem.Type{stringType, responseType}, ReturnType: typesystem.Nil},
		"mockHttpError":  typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: typesystem.Nil},
		"mockHttpOff":    typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Nil},
		"mockHttpBypass": typesystem.TFunc{Params: []typesystem.Type{A}, ReturnType: A},
		"mockFile":       typesystem.TFunc{Params: []typesystem.Type{stringType, resultAString}, ReturnType: typesystem.Nil},
		"mockFileOff":    typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Nil},
		"mockFileBypass": typesystem.TFunc{Params: []typesystem.Type{A}, ReturnType: A},
		"mockEnv":        typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: typesystem.Nil},
		"mockEnvOff":     typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Nil},
		"mockEnvBypass":  typesystem.TFunc{Params: []typesystem.Type{A}, ReturnType: A},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

// GetTestResults returns all test results
func GetTestResults() []TestResult {
	tr := GetTestRunner()
	return tr.Results
}

// PrintTestSummary prints a summary of test results
func PrintTestSummary() {
	tr := GetTestRunner()
	passed := 0
	failed := 0
	skipped := 0
	expectFail := 0

	var skippedTests []TestResult
	var expectFailTests []TestResult

	for _, r := range tr.Results {
		if r.Skipped {
			skipped++
			skippedTests = append(skippedTests, r)
		} else if r.ExpectFail {
			expectFail++
			expectFailTests = append(expectFailTests, r)
			if !r.Passed {
				failed++ // ExpectFail test that didn't fail is a failure
			}
		} else if r.Passed {
			passed++
		} else {
			failed++
		}
	}

	total := len(tr.Results)
	fmt.Printf("\n%d tests, %d passed, %d failed, %d skipped, %d expect-fail\n", total, passed, failed, skipped, expectFail)

	// Print lists if any
	if len(skippedTests) > 0 {
		fmt.Printf("\nSkipped tests:\n")
		for _, t := range skippedTests {
			fmt.Printf("  ⊘ %s: %s\n", t.Name, t.Error)
		}
	}

	if len(expectFailTests) > 0 {
		fmt.Printf("\nExpect-fail tests (known bugs):\n")
		for _, t := range expectFailTests {
			if t.Passed {
				fmt.Printf("  ⚠ %s: %s\n", t.Name, t.Error)
			} else {
				fmt.Printf("  ✗ %s: %s (BUG FIXED! Remove testExpectFail)\n", t.Name, t.Error)
			}
		}
	}
}

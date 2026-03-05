package evaluator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"
)

// HypervisorInterface allows the test runner to interact with the real hypervisor without cyclical imports
type HypervisorInterface interface {
	SpawnVM(path string, config map[string]interface{}) (string, error)
	KillVM(id string, saveState bool, timeoutMs int) ([]byte, error)
	GetStats(id string) (map[string]uint64, error)
	InjectHandlers(e *Evaluator)
}

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
	Hypervisor  interface{} // Holds the Hypervisor instance

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

	// RPC mocks: "targetVm:method" -> callback function
	RpcMocks       map[string]Object
	RpcMocksActive bool

	// Mailbox mocks: "targetVm" -> callback function
	MailboxMocks       map[string]Object
	MailboxMocksActive bool

	// Supervisor event mock: callback(timeoutMs: Int) -> Record
	SupervisorEventMock        Object
	SupervisorEventMocksActive bool

	// Cluster VMs spawned during the current test
	SpawnedVMs []string

	// Active load generators to clean up
	ActiveLoads []context.CancelFunc
}

// Global test runner instance
var (
	testRunner     *TestRunner
	testRunnerOnce sync.Once
)

// InitTestRunner creates or resets the test runner
func InitTestRunner(e *Evaluator, h interface{}) {
	testRunnerOnce.Do(func() {
		testRunner = &TestRunner{
			Evaluator:                  e,
			Hypervisor:                 h,
			HttpMocks:                  make(map[string]Object),
			HttpMockErrors:             make(map[string]string),
			HttpMocksActive:            false,
			FileMocks:                  make(map[string]Object),
			FileMocksActive:            false,
			EnvMocks:                   make(map[string]string),
			EnvMocksActive:             false,
			RpcMocks:                   make(map[string]Object),
			RpcMocksActive:             false,
			MailboxMocks:               make(map[string]Object),
			MailboxMocksActive:         false,
			SupervisorEventMocksActive: false,
			SpawnedVMs:                 make([]string, 0),
			ActiveLoads:                make([]context.CancelFunc, 0),
			Results:                    make([]TestResult, 0),
		}
	})
	// If already initialized, update evaluator and reset state completely
	if testRunner != nil {
		testRunner.Evaluator = e
		testRunner.Hypervisor = h
		testRunner.ResetMocks()
		testRunner.ClearResults() // Fix: Clear previous results to avoid pollution
	}
}

// GetTestRunner returns the global test runner (creates if needed)
func GetTestRunner() *TestRunner {
	testRunnerOnce.Do(func() {
		testRunner = &TestRunner{
			HttpMocks:                  make(map[string]Object),
			HttpMockErrors:             make(map[string]string),
			FileMocks:                  make(map[string]Object),
			EnvMocks:                   make(map[string]string),
			RpcMocks:                   make(map[string]Object),
			MailboxMocks:               make(map[string]Object),
			Results:                    make([]TestResult, 0),
			SupervisorEventMocksActive: false,
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

	tr.RpcMocks = make(map[string]Object)
	tr.RpcMocksActive = false

	tr.MailboxMocks = make(map[string]Object)
	tr.MailboxMocksActive = false
	tr.SupervisorEventMock = nil
	tr.SupervisorEventMocksActive = false

	// Clean up all VMs spawned during this test
	if tr.Hypervisor != nil {
		if hyp, ok := tr.Hypervisor.(HypervisorInterface); ok {
			for _, id := range tr.SpawnedVMs {
				_, _ = hyp.KillVM(id, false, 5) // Extremely short timeout, don't block tests
			}
		}
	}
	tr.SpawnedVMs = make([]string, 0)

	// Stop any active load generators
	for _, cancel := range tr.ActiveLoads {
		cancel()
	}
	tr.ActiveLoads = make([]context.CancelFunc, 0)
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

// FindRpcMock looks for a matching RPC mock for the given target and method
func (tr *TestRunner) FindRpcMock(targetVm, method string) (Object, bool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if !tr.RpcMocksActive {
		return nil, false
	}

	key := fmt.Sprintf("%s:%s", targetVm, method)
	if callback, ok := tr.RpcMocks[key]; ok {
		return callback, true
	}

	// Check wildcard method
	wildcardKey := fmt.Sprintf("%s:*", targetVm)
	if callback, ok := tr.RpcMocks[wildcardKey]; ok {
		return callback, true
	}

	// Check wildcard target
	wildcardTargetKey := fmt.Sprintf("*:%s", method)
	if callback, ok := tr.RpcMocks[wildcardTargetKey]; ok {
		return callback, true
	}

	// Check full wildcard
	if callback, ok := tr.RpcMocks["*:*"]; ok {
		return callback, true
	}

	return nil, false
}

// FindMailboxMock looks for a matching Mailbox mock for the given target
func (tr *TestRunner) FindMailboxMock(targetVm string) (Object, bool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if !tr.MailboxMocksActive {
		return nil, false
	}

	if callback, ok := tr.MailboxMocks[targetVm]; ok {
		return callback, true
	}

	// Check wildcard target
	if callback, ok := tr.MailboxMocks["*"]; ok {
		return callback, true
	}

	return nil, false
}

// FindSupervisorEventMock returns callback for mocked supervisor receiveEventWait.
func (tr *TestRunner) FindSupervisorEventMock() (Object, bool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if !tr.SupervisorEventMocksActive || tr.SupervisorEventMock == nil {
		return nil, false
	}
	return tr.SupervisorEventMock, true
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
		"assert":          {Fn: builtinAssert, Name: "assert"},
		"assertTrue":      {Fn: builtinAssertTrue, Name: "assertTrue"},
		"assertFalse":     {Fn: builtinAssertFalse, Name: "assertFalse"},
		"assertEquals":    {Fn: builtinAssertEquals, Name: "assertEquals"},
		"assertNotEquals": {Fn: builtinAssertNotEquals, Name: "assertNotEquals"},
		"assertOk":        {Fn: builtinAssertOk, Name: "assertOk"},
		"assertFail":      {Fn: builtinAssertFail, Name: "assertFail"},
		"assertSome":      {Fn: builtinAssertSome, Name: "assertSome"},
		"assertNone":      {Fn: builtinAssertNone, Name: "assertNone"},

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

		// RPC mocks
		"mockRpc":    {Fn: builtinMockRpc, Name: "mockRpc"},
		"mockRpcOff": {Fn: builtinMockRpcOff, Name: "mockRpcOff"},

		// Mailbox mocks
		"mockMailboxSend":        {Fn: builtinMockMailboxSend, Name: "mockMailboxSend"},
		"mockMailboxOff":         {Fn: builtinMockMailboxOff, Name: "mockMailboxOff"},
		"mockSupervisorEvent":    {Fn: builtinMockSupervisorEvent, Name: "mockSupervisorEvent"},
		"mockSupervisorEventOff": {Fn: builtinMockSupervisorEventOff, Name: "mockSupervisorEventOff"},

		// Cluster testing
		"testSpawnVM":      {Fn: builtinTestSpawnVM, Name: "testSpawnVM"},
		"testSpawnVMGroup": {Fn: builtinTestSpawnVMGroup, Name: "testSpawnVMGroup"},
		"testSpawnLoad":    {Fn: builtinTestSpawnLoad, Name: "testSpawnLoad"},
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

// assertNotEquals(expected: T, actual: T, msg?: String) -> Nil
func builtinAssertNotEquals(e *Evaluator, args ...Object) Object {
	if len(args) < 2 || len(args) > 3 {
		return newError("assertNotEquals expects 2-3 arguments, got %d", len(args))
	}

	expected := args[0]
	actual := args[1]

	if objectsEqual(expected, actual) {
		msg := extractMessage(args, 2)
		if msg != "" {
			return newError("assertion failed: %s (expected not %s, got %s)", msg, expected.Inspect(), actual.Inspect())
		}
		return newError("assertion failed: expected not %s, got %s", expected.Inspect(), actual.Inspect())
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

// mockRpc(targetVm: String, method: String, callback: (Any) -> Any) -> Nil
func builtinMockRpc(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("mockRpc expects 3 arguments, got %d", len(args))
	}

	targetVmList, ok := args[0].(*List)
	if !ok {
		return newError("mockRpc expects a string targetVm, got %s", args[0].Type())
	}
	targetVm := listToString(targetVmList)

	methodList, ok := args[1].(*List)
	if !ok {
		return newError("mockRpc expects a string method, got %s", args[1].Type())
	}
	method := listToString(methodList)

	callback := args[2]

	tr := GetTestRunner()
	tr.mu.Lock()
	defer tr.mu.Unlock()

	key := fmt.Sprintf("%s:%s", targetVm, method)
	tr.RpcMocks[key] = callback
	tr.RpcMocksActive = true

	return &Nil{}
}

// mockRpcOff() -> Nil
func builtinMockRpcOff(e *Evaluator, args ...Object) Object {
	tr := GetTestRunner()
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.RpcMocks = make(map[string]Object)
	tr.RpcMocksActive = false

	return &Nil{}
}

// mockMailboxSend(targetVm: String, callback: (Any) -> Any) -> Nil
func builtinMockMailboxSend(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("mockMailboxSend expects 2 arguments, got %d", len(args))
	}

	targetVmList, ok := args[0].(*List)
	if !ok {
		return newError("mockMailboxSend expects a string targetVm, got %s", args[0].Type())
	}
	targetVm := listToString(targetVmList)

	callback := args[1]

	tr := GetTestRunner()
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.MailboxMocks[targetVm] = callback
	tr.MailboxMocksActive = true

	return &Nil{}
}

// mockMailboxOff() -> Nil
func builtinMockMailboxOff(e *Evaluator, args ...Object) Object {
	tr := GetTestRunner()
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.MailboxMocks = make(map[string]Object)
	tr.MailboxMocksActive = false

	return &Nil{}
}

// mockSupervisorEvent(callback: (Int) -> Record) -> Nil
func builtinMockSupervisorEvent(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("mockSupervisorEvent expects 1 argument, got %d", len(args))
	}

	tr := GetTestRunner()
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.SupervisorEventMock = args[0]
	tr.SupervisorEventMocksActive = true
	return &Nil{}
}

// mockSupervisorEventOff() -> Nil
func builtinMockSupervisorEventOff(e *Evaluator, args ...Object) Object {
	tr := GetTestRunner()
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.SupervisorEventMock = nil
	tr.SupervisorEventMocksActive = false
	return &Nil{}
}

// builtinTestSpawnVM(path: String, config: Record) -> Result<String, String>
func builtinTestSpawnVM(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("testSpawnVM expects 2 arguments: path and config")
	}

	pathObj, ok := args[0].(*List)
	if !ok || !IsStringList(pathObj) {
		return newError("testSpawnVM first argument must be a String")
	}
	path := ListToString(pathObj)

	configObj, ok := args[1].(*RecordInstance)
	if !ok {
		return newError("testSpawnVM second argument must be a Record")
	}

	tr := GetTestRunner()
	if tr == nil || tr.Hypervisor == nil {
		return makeFailStr("Hypervisor is not available in this test environment")
	}

	// We use reflection/type assertion trick to avoid circular imports.
	// We know Hypervisor is a *funxy.Hypervisor which has SpawnVM
	var id string
	var err error

	// Convert RecordInstance to map[string]interface{} for Hypervisor
	configMap := make(map[string]interface{})
	for _, field := range configObj.Fields {
		// Just basic mapping for now, assuming config has strings/ints
		if strList, isStr := field.Value.(*List); isStr && IsStringList(strList) {
			configMap[field.Key] = ListToString(strList)
		} else if intObj, isInt := field.Value.(*Integer); isInt {
			configMap[field.Key] = int(intObj.Value)
		} else if boolObj, isBool := field.Value.(*Boolean); isBool {
			configMap[field.Key] = boolObj.Value
		} else {
			// For complex types (capabilities array, mailbox map), we'd need a deeper conversion.
			// Let's rely on standard Go map conversion logic if needed, or pass it raw for now.
			configMap[field.Key] = field.Value
		}
	}

	// Handle 'capabilities' specifically since Hypervisor expects []interface{} of strings
	if capsObj, found := getRecordField(configObj, "capabilities"); found {
		if capsList, ok := capsObj.(*List); ok {
			var caps []interface{}
			elements := capsList.ToSlice()
			for _, item := range elements {
				if str, ok := item.(*List); ok && IsStringList(str) {
					caps = append(caps, ListToString(str))
				}
			}
			configMap["capabilities"] = caps
		}
	}

	// Handle 'mailbox' specifically since Hypervisor expects map[string]interface{}
	if mbObj, found := getRecordField(configObj, "mailbox"); found {
		if mbRec, ok := mbObj.(*RecordInstance); ok {
			mbMap := make(map[string]interface{})
			for _, f := range mbRec.Fields {
				if intObj, isInt := f.Value.(*Integer); isInt {
					mbMap[f.Key] = int(intObj.Value)
				} else if strObj, isStr := f.Value.(*List); isStr && IsStringList(strObj) {
					mbMap[f.Key] = ListToString(strObj)
				} else if dataObj, isData := f.Value.(*DataInstance); isData {
					// importance is a DataInstance
					mbMap[f.Key] = dataObj.Name
				}
			}
			configMap["mailbox"] = mbMap
		}
	}

	// Helper function to resolve paths
	resolvePath := func(p string) string {
		if filepath.IsAbs(p) {
			return p
		}
		// In tests, resolve relative to the current working directory
		cwd, err := os.Getwd()
		if err == nil {
			return filepath.Join(cwd, p)
		}
		return p
	}

	if hyp, ok := tr.Hypervisor.(HypervisorInterface); ok {
		id, err = hyp.SpawnVM(resolvePath(path), configMap)
		// Inject handlers so test scripts can interact with spawned VMs
		hyp.InjectHandlers(e)
	} else {
		// Try using reflection as a fallback if the interface doesn't match perfectly
		val := reflect.ValueOf(tr.Hypervisor)
		method := val.MethodByName("SpawnVM")
		if method.IsValid() {
			args := []reflect.Value{
				reflect.ValueOf(path),
				reflect.ValueOf(configMap),
			}
			results := method.Call(args)
			id = results[0].String()
			if !results[1].IsNil() {
				err = results[1].Interface().(error)
			}
		} else {
			return makeFailStr("Hypervisor object does not implement required interface")
		}
	}
	if err != nil {
		return makeFailStr(err.Error())
	}

	// Track the spawned VM
	tr.mu.Lock()
	tr.SpawnedVMs = append(tr.SpawnedVMs, id)
	tr.mu.Unlock()

	return makeOk(stringToList(id))
}

// builtinTestSpawnVMGroup(path: String, config: Record, size: Int) -> Result<String, List<String>>
func builtinTestSpawnVMGroup(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("testSpawnVMGroup expects 3 arguments: path, config, and size")
	}

	sizeArg, ok := args[2].(*Integer)
	if !ok {
		return newError("testSpawnVMGroup third argument must be an Integer")
	}
	size := int(sizeArg.Value)
	if size <= 0 {
		return makeFailStr("testSpawnVMGroup size must be > 0")
	}

	var ids []Object
	for i := 0; i < size; i++ {
		res := builtinTestSpawnVM(e, args[0], args[1])

		if dataInst, ok := res.(*DataInstance); ok {
			if dataInst.Name == "Fail" {
				return res
			} else if dataInst.Name == "Ok" {
				ids = append(ids, dataInst.Fields[0])
			}
		} else {
			return makeFailStr("testSpawnVM returned invalid result type")
		}
	}

	return makeOk(newList(ids))
}

const (
	defaultLoadDurationMs = 1000
	defaultLoadThreads    = 1
	defaultLoadType       = "cpu"
	defaultMailboxRps     = 1000

	// CPU load batch size (iterations per context check)
	// We use a batch loop to avoid the overhead of select/ctx.Done() on every single operation,
	// which would dominate the CPU profile instead of the actual math load.
	cpuBatchSize = 1000

	// Memory load defaults
	memoryChunkSizeBytes = 1024 * 1024 // 1MB
	memoryAllocDelayMs   = 10
)

// builtinTestSpawnLoad(config: Record) -> Result<String, String>
func builtinTestSpawnLoad(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return makeFailStr(fmt.Sprintf("testSpawnLoad expects 1 argument (config), got %d", len(args)))
	}

	configRecord, ok := args[0].(*RecordInstance)
	if !ok {
		return makeFailStr("config must be a record")
	}

	loadTypeStr := defaultLoadType
	if tObj, ok := getRecordField(configRecord, "type"); ok {
		if tList, isList := tObj.(*List); isList {
			loadTypeStr = listToString(tList)
		}
	}

	durationMs := defaultLoadDurationMs
	if dObj, ok := getRecordField(configRecord, "duration"); ok {
		if dInt, isInt := dObj.(*Integer); isInt {
			durationMs = int(dInt.Value)
		}
	}

	threads := defaultLoadThreads
	if tObj, ok := getRecordField(configRecord, "threads"); ok {
		if tInt, isInt := tObj.(*Integer); isInt {
			threads = int(tInt.Value)
		}
	}

	jobId := fmt.Sprintf("loadjob_%d", time.Now().UnixNano())

	// Grab a reference to MailboxHandler in case we need it
	var mHandler *MailboxHandler
	if e.MailboxHandler != nil {
		mHandler = e.MailboxHandler
	}

	tr := GetTestRunner()
	tr.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(durationMs)*time.Millisecond)
	tr.ActiveLoads = append(tr.ActiveLoads, cancel)
	tr.mu.Unlock()

	// We can spin up goroutines based on loadTypeStr
	go func() {
		defer cancel()

		var wg sync.WaitGroup
		for i := 0; i < threads; i++ {
			wg.Add(1)
			go func(threadId int) {
				defer wg.Done()
				switch loadTypeStr {
				case "cpu":
					// tight loop doing some math
					dummy := 0.0
					for {
						select {
						case <-ctx.Done():
							return
						default:
							for j := 0; j < cpuBatchSize; j++ {
								dummy += 1.0
							}
						}
					}
				case "memory":
					// allocate arrays
					var allocations [][]byte
					for {
						select {
						case <-ctx.Done():
							return
						default:
							allocations = append(allocations, make([]byte, memoryChunkSizeBytes))
							time.Sleep(memoryAllocDelayMs * time.Millisecond)
						}
					}
				case "mailbox":
					targetVm := ""
					if tObj, ok := getRecordField(configRecord, "target_vm"); ok {
						if tList, isList := tObj.(*List); isList {
							targetVm = listToString(tList)
						}
					}
					if targetVm == "" || mHandler == nil {
						return
					}

					rps := defaultMailboxRps
					if rObj, ok := getRecordField(configRecord, "rps"); ok {
						if rInt, isInt := rObj.(*Integer); isInt {
							rps = int(rInt.Value)
						}
					}

					// Sleep interval between messages
					interval := time.Second / time.Duration(rps)
					if interval < time.Microsecond {
						interval = time.Microsecond
					}

					ticker := time.NewTicker(interval)
					defer ticker.Stop()

					msgObj := stringToList("load_ping")

					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							// Send a fire-and-forget message
							_ = mHandler.Send(targetVm, msgObj)
						}
					}
				default:
					// unknown load type, just wait it out
					<-ctx.Done()
				}
			}(i)
		}
		wg.Wait()
	}()

	return makeOk(stringToList(jobId))
}

// SetTestBuiltinTypes sets type info for test builtins

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

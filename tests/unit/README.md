# Unit Tests

This directory contains unit tests for the Funxy language. Unlike functional tests, these tests are written in Funxy itself (using the `lib/test` module) and are executed by the `unit_test.go` runner. They are designed to test specific features, edge cases, and internal behaviors of the language implementation.

## Structure

*   `*_test.lang`: Funxy source files containing unit tests.
*   `unit_test.go`: The Go test runner that executes these tests.

## Running Tests

To run all unit tests:

```bash
go test ./tests/unit/unit_test.go
```

Or from the project root:

```bash
go test ./tests/unit/...
```

### Running Specific Tests

To run a specific test file (e.g., `complex/arithmetic_test.lang`):

```bash
go test -v ./tests/unit/unit_test.go -run TestUnitTests/arithmetic
```

Note: The test name corresponds to the filename without the `_test.lang` suffix.

## Writing Tests

Unit tests use the `lib/test` module (which is built-in for the test runner).

Example `my_feature_test.lang`:

```rust
import "lib/test" (*)

testRun("My Feature Test", fun() -> {
    result = 1 + 2
    assertEquals(3, result, "Addition should work")
})

testRun("Another Test Case", fun() -> {
    // ...
})
```

## How it Works

The `TestUnitTests` function in `unit_test.go`:
1.  Recursively finds all `*_test.lang` files in the `tests/unit` directory.
2.  For each file:
    *   Initializes the Funxy runtime (VM or Tree-Walk).
    *   Parses and compiles the test file.
    *   Executes the code.
    *   Checks the results collected by the `lib/test` module (which registers pass/fail status internally).
    *   Reports any failures to the Go testing framework.

## Tree-Walk Interpreter

By default, tests run using the VM backend. To run tests using the Tree-Walk interpreter:

```bash
go test -v ./tests/unit/unit_test.go -args -tree
```

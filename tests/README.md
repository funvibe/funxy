# Functional Tests

This directory contains functional tests for the Funxy language. These tests verify the behavior of the language from the user's perspective by compiling and running `.lang` files and comparing their output against expected `.want` files.

## Structure

*   `*.lang`: Source code files containing the test cases.
*   `*.want`: Expected output files corresponding to the `.lang` files.
*   `functional_test.go`: The Go test runner that executes the functional tests.

## Running Tests

To run all functional tests:

```bash
go test ./tests/functional_test.go
```

Or from the project root:

```bash
go test ./tests/...
```

### Running Specific Tests

To run a specific test case (e.g., `arithmetic.lang`):

```bash
go test -v ./tests/functional_test.go -run TestFunctional/arithmetic
```

## Adding New Tests

1.  Create a new `.lang` file in the `tests/` directory (e.g., `my_feature.lang`).
2.  Write your Funxy code in the file. Use `print()` to output results you want to verify.
3.  Create a corresponding `.want` file (e.g., `my_feature.want`) containing the expected output of your program.
4.  Run the tests to verify your new test case passes.

## How it Works

The `TestFunctional` function in `functional_test.go`:
1.  Builds a fresh `funxy-test-binary` from the project root.
2.  Finds all `*.lang` files in the `tests/` directory that have a matching `*.want` file.
3.  For each pair:
    *   Runs the compiled binary with the `.lang` file as input.
    *   Captures the standard output and standard error.
    *   Normalizes the output (e.g., removing absolute paths).
    *   Compares the actual output with the content of the `.want` file.
    *   Reports a failure if they do not match.

## Tree-Walk Interpreter

By default, tests run using the VM backend. To run tests using the Tree-Walk interpreter:

```bash
go test -v ./tests/functional_test.go -args -tree
```

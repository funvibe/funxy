# Hello World

The first program in Funxy.

## Screen Output

The `print` function outputs values to stdout:

```rust
print("Hello, World!")
```

## String Literals

**Regular strings** - in double quotes:

```rust
message = "Hello, World!"
print(message)
```

**Multi-line strings** - in backticks:

```rust
text = `This is a
multi-line
string`

json = `{"name": "test", "value": 42}`
print(json)
```

Features of raw strings:
- Can contain line breaks
- Don't process escape sequences
- Convenient for JSON, SQL, templates

## Running

```bash
./funxy hello.lang
```

## Tests

See `tests/hello_world.lang`

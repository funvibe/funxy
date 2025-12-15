# CSV

The `lib/csv` module provides functions for working with CSV files.

## Import

```rust
import "lib/csv" (*)
```

## CSV Parsing

### With Headers

`csvParse` parses a CSV string, using the first row as headers. Returns `List<Record>`:

```rust
import "lib/csv" (csvParse)

csv = "name,age,city
Alice,30,NYC
Bob,25,LA"

match csvParse(csv) {
    Ok(rows) -> {
        for row in rows {
            print(row.name ++ " is " ++ row.age ++ " from " ++ row.city)
        }
    }
    Fail(e) -> print("Error: " ++ e)
}
```

### Without Headers (Raw)

`csvParseRaw` returns `List<List<String>>`:

```rust
import "lib/csv" (csvParseRaw)

csv = "1,2,3
4,5,6"

match csvParseRaw(csv) {
    Ok(rows) -> {
        for row in rows {
            print(show(row))
        }
    }
    Fail(e) -> print("Error: " ++ e)
}
```

## Custom Delimiter

Default delimiter is comma. You can specify another:

```rust
import "lib/csv" (csvParse)

// Semicolon (European format)
csv1 = "name;age
Alice;30"

match csvParse(csv1, ';') {
    Ok(rows) -> print(rows[0].name)
    _ -> ()
}

// Tab (TSV)
csv2 = "name\tage
Bob\t25"

match csvParse(csv2, '\t') {
    Ok(rows) -> print(rows[0].name)
    _ -> ()
}
```

## Reading from File

```rust
import "lib/csv" (csvRead)

match csvRead("data.csv") {
    Ok(rows) -> {
        for row in rows {
            print(row.name)
        }
    }
    Fail(e) -> print("Cannot read file: " ++ e)
}
```

With delimiter:

```rust
import "lib/csv" (csvRead)

match csvRead("data.tsv", '\t') {
    Ok(rows) -> print("Loaded " ++ show(len(rows)) ++ " rows")
    Fail(e) -> print("Error: " ++ e)
}
```

## Encoding to CSV

### From Records

```rust
import "lib/csv" (csvEncode)

data = [
    { name: "Alice", age: "30" },
    { name: "Bob", age: "25" }
]

csv = csvEncode(data)
print(csv)
// name,age
// Alice,30
// Bob,25
```

### From Lists (Raw)

```rust
import "lib/csv" (csvEncodeRaw)

data = [
    ["Alice", "30"],
    ["Bob", "25"]
]

csv = csvEncodeRaw(data)
print(csv)
// Alice,30
// Bob,25
```

### With Delimiter

```rust
import "lib/csv" (csvEncode)

data = [{ a: "1", b: "2" }]

tsv = csvEncode(data, '\t')
print(tsv)
// a	b
// 1	2
```

## Writing to File

```rust
import "lib/csv" (csvWrite)

data = [
    { name: "Alice", score: "100" },
    { name: "Bob", score: "95" }
]

match csvWrite("output.csv", data) {
    Ok(_) -> print("Saved!")
    Fail(e) -> print("Error: " ++ e)
}
```

With delimiter:

```rust
import "lib/csv" (csvWrite)

data = [{ x: "1", y: "2" }]

match csvWrite("output.tsv", data, '\t') {
    Ok(_) -> print("Saved as TSV!")
    Fail(e) -> print("Error: " ++ e)
}
```

## Function Reference

| Function | Description |
|---------|----------|
| `csvParse(content, delimiter?)` | Parses CSV with headers → `Result<String, List<Record>>` |
| `csvParseRaw(content, delimiter?)` | Parses CSV without headers → `Result<String, List<List<String>>>` |
| `csvRead(path, delimiter?)` | Reads CSV file with headers |
| `csvReadRaw(path, delimiter?)` | Reads CSV file without headers |
| `csvEncode(records, delimiter?)` | Encodes records to CSV string |
| `csvEncodeRaw(rows, delimiter?)` | Encodes lists to CSV string |
| `csvWrite(path, records, delimiter?)` | Writes records to file |
| `csvWriteRaw(path, rows, delimiter?)` | Writes lists to file |

**Delimiter** default: `,` (comma). Can specify `';'`, `'\t'` or any other `Char`.

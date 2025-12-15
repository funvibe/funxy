# 01. Reading and Writing Files

## Task
Read, write and process files.

---

## Reading entire file

```rust
import "lib/io" (fileRead)

match fileRead("data.txt") {
    Ok(content) -> print(content)
    Fail(err) -> print("Error: " ++ err)
}

```

---

## Writing to file

```rust
import "lib/io" (fileWrite)

match fileWrite("output.txt", "Hello, World!") {
    Ok(_) -> print("Written!")
    Fail(err) -> print("Error: " ++ err)
}

```

---

## Appending to file

```rust
import "lib/io" (fileAppend)

match fileAppend("log.txt", "New log entry\n") {
    Ok(_) -> print("Appended!")
    Fail(err) -> print("Error: " ++ err)
}

```

---

## Checking existence

```rust
import "lib/io" (fileExists)

if fileExists("config.json") {
    print("Config found")
} else {
    print("Using defaults")
}
```

---

## Reading line by line

```rust
import "lib/io" (fileRead)
import "lib/string" (stringSplit)

match fileRead("data.txt") {
    Ok(content) -> {
        lines = stringSplit(content, "\n")
        for line in lines {
            print("> " ++ line)
        }
    }
    Fail(err) -> print("Error: " ++ err)
}

```

---

## Counting words in file

```rust
import "lib/io" (fileRead)
import "lib/string" (stringSplit)
import "lib/list" (filter)

match fileRead("book.txt") {
    Ok(content) -> {
        words = stringSplit(content, " ")
        nonEmpty = filter(fun(w) -> len(w) > 0, words)
        print("Words: " ++ show(len(nonEmpty)))
    }
    Fail(err) -> print("Error: " ++ err)
}

```

---

## Counting lines

```rust
import "lib/io" (fileRead)
import "lib/string" (stringSplit)

match fileRead("code.lang") {
    Ok(content) -> {
        lines = stringSplit(content, "\n")
        print("Lines: " ++ show(len(lines)))
    }
    Fail(err) -> print("Error: " ++ err)
}

```

---

## Copying file

```rust
import "lib/io" (fileRead, fileWrite)

fun copyFile(src: String, dst: String) {
    match fileRead(src) {
        Ok(content) -> {
            match fileWrite(dst, content) {
                Ok(_) -> print("Copied!")
                Fail(err) -> print("Write error: " ++ err)
            }
        }
        Fail(err) -> print("Read error: " ++ err)
    }
}

copyFile("original.txt", "backup.txt")

```

---

## Processing CSV

```rust
import "lib/io" (fileRead, fileWrite)
import "lib/string" (stringSplit, stringJoin)
import "lib/list" (filter, map, head, tail)

match fileRead("data.csv") {
    Ok(content) -> {
        allLines = stringSplit(content, "\n")
        rows = filter(fun(line) -> len(line) > 0, allLines)
        parsed = map(fun(line) -> stringSplit(line, ","), rows)

        // First line - headers
        headers = head(parsed)
        data = tail(parsed)

        // Processing
        for row in data {
            print("Name: " ++ row[0] ++ ", Age: " ++ row[1])
        }
    }
    Fail(err) -> print("Error: " ++ err)
}

```

---

## Writing CSV

```rust
import "lib/io" (fileWrite)
import "lib/string" (stringJoin)
import "lib/list" (map)

newData = [
    ["Alice", "30", "Engineer"],
    ["Bob", "25", "Designer"]
]

csvContent = stringJoin(map(fun(row) -> stringJoin(row, ","), newData), "\n")

match fileWrite("output.csv", csvContent) {
    Ok(_) -> print("CSV saved!")
    Fail(err) -> print("Error: " ++ err)
}

```

---

## Working with configuration

```rust
import "lib/io" (fileRead, fileWrite, fileExists)
import "lib/json" (jsonDecode, jsonEncode)

type Config = { port: Int, debug: Bool }

defaultConfig: Config = { port: 8080, debug: false }

fun loadConfig(path: String) -> Config {
    if fileExists(path) {
        match fileRead(path) {
            Ok(content) -> match jsonDecode(content) {
                Ok(cfg) -> { port: cfg.port, debug: cfg.debug }
                Fail(_) -> defaultConfig
            }
            Fail(_) -> defaultConfig
        }
    } else {
        defaultConfig
    }
}

fun saveConfig(path: String, config: Config) {
    match fileWrite(path, jsonEncode(config)) {
        Ok(_) -> print("Config saved!")
        Fail(err) -> print("Error: " ++ err)
    }
}

config = loadConfig("config.json")
print("Server port: " ++ show(config.port))
```

---

## Logging

```rust
import "lib/io" (fileAppend)
import "lib/date" (dateNow, dateFormat)

fun log(level: String, message: String) {
    timestamp = dateFormat(dateNow(), "YYYY-MM-DD HH:mm:ss")
    entry = "[" ++ timestamp ++ "] [" ++ level ++ "] " ++ message ++ "\n"
    fileAppend("app.log", entry)
}

log("INFO", "Application started")
log("DEBUG", "Processing request")
log("ERROR", "Something went wrong")
```

---

## Directory traversal

```rust
import "lib/io" (dirList, isDir)
import "lib/path" (pathJoin)

fun processDir(dir: String) {
    match dirList(dir) {
        Ok(entries) -> {
            for entry in entries {
                fullPath = pathJoin([dir, entry])
                if isDir(fullPath) {
                    print("[DIR] " ++ entry)
                    processDir(fullPath)
                } else {
                    print("[FILE] " ++ entry)
                }
            }
        }
        Fail(err) -> print("Error: " ++ err)
    }
}

processDir("./src")

```

---

## Searching in files

```rust
import "lib/io" (dirList, fileRead, isDir)
import "lib/path" (pathJoin)
import "lib/string" (stringEndsWith)
import "lib/regex" (regexMatch)
import "lib/list" (forEach)

fun grepInDir(dir: String, pattern: String) -> List<String> {
    results = []
    
    match dirList(dir) {
        Ok(entries) -> {
            forEach(fun(entry) -> {
                path = pathJoin([dir, entry])
                if isDir(path) {
                    results = results ++ grepInDir(path, pattern)
                } else {
                    if stringEndsWith(entry, ".lang") {
                        match fileRead(path) {
                            Ok(content) -> {
                                if regexMatch(pattern, content) {
                                    results = results ++ [path]
                                }
                            }
                            Fail(_) -> Nil
                        }
                    }
                }
            }, entries)
        }
        Fail(_) -> Nil
    }
    
    results
}

matches = grepInDir("./src", "TODO")
for m in matches {
    print("Found in: " ++ m)
}

```

---

## Safe reading

```rust
import "lib/io" (fileRead, fileExists)

fun safeRead(path: String) -> Option<String> {
    if fileExists(path) {
        match fileRead(path) {
            Ok(content) -> Some(content)
            Fail(_) -> Zero
        }
    } else {
        Zero
    }
}

match safeRead("maybe.txt") {
    Some(content) -> print("Got: " ++ content)
    Zero -> print("File not found")
}

```

---

## Temporary files

```rust
import "lib/io" (fileWrite, fileRead, fileDelete)
import "lib/path" (pathJoin, pathTemp)
import "lib/uuid" (uuidNew, uuidToString)

fun withTempFile(content: String, f) {
    tmpPath = pathJoin([pathTemp(), "tmp_" ++ uuidToString(uuidNew()) ++ ".txt"])
    
    match fileWrite(tmpPath, content) {
        Ok(_) -> {
            result = f(tmpPath)
            fileDelete(tmpPath)
            result
        }
        Fail(err) -> {
            print("Error creating temp file: " ++ err)
            Nil
        }
    }
}

result = withTempFile("test data", fun(path) -> {
    match fileRead(path) {
        Ok(data) -> len(data)
        Fail(_) -> 0
    }
})
print(show(result))  // 9

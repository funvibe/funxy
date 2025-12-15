# 23. CLI Utility

## Task
Create a full-featured command-line utility with arguments, options, and beautiful output.

---

## Complete example: Todo CLI

Creating a todo manager with commands `add`, `list`, `done`, `remove`.

### todo.lang

```rust
import "lib/sys" (sysArgs, sysExit)
import "lib/io" (fileRead, fileWrite, fileExists)
import "lib/json" (jsonEncode, jsonDecode)
import "lib/string" (stringPadRight, stringTrim)
import "lib/list" (filter, map, foldl)

// --- Types ---

type Todo = { id: Int, text: String, done: Bool }
type TodoList = List<Todo>

// --- Storage ---

dataFile = "todos.json"

fun loadTodos() -> TodoList {
    if fileExists(dataFile) {
        match fileRead(dataFile) {
            Ok(content) -> match jsonDecode(content) {
                Ok(todos) -> todos
                Fail(_) -> []
            }
            Fail(_) -> []
        }
    } else { [] }
}

fun saveTodos(todos: TodoList) {
    match fileWrite(dataFile, jsonEncode(todos)) {
        Ok(_) -> Nil
        Fail(e) -> {
            print("Error saving: " ++ e)
            Nil
        }
    }
}

fun nextId(todos: TodoList) -> Int {
    foldl(fun(acc, t) -> if t.id > acc { t.id } else { acc }, 0, todos) + 1
}

// --- Commands ---

fun cmdAdd(text: String) {
    todos = loadTodos()
    newTodo = { id: nextId(todos), text: text, done: false }
    saveTodos(todos ++ [newTodo])
    print("Added: " ++ text)
}

fun cmdList() {
    todos = loadTodos()
    if len(todos) == 0 {
        print("No todos yet. Add one with: todo add \"Your task\"")
    } else {
        print("\nYour Todos:\n")
        for t in todos {
            status = if t.done { "[x]" } else { "[ ]" }
            idStr = stringPadRight(show(t.id), 4, ' ')
            print("  " ++ status ++ " " ++ idStr ++ t.text)
        }
        print("")
        
        done = len(filter(fun(t) -> t.done, todos))
        total = len(todos)
        print("  " ++ show(done) ++ "/" ++ show(total) ++ " completed\n")
    }
}

fun cmdDone(idStr: String) {
    idOpt = read(idStr, Int)
    match idOpt {
        Some(id) -> {
            todos = loadTodos()
            updated = map(fun(t: Todo) -> if t.id == id { { id: t.id, text: t.text, done: true } } else { t }, todos)
            saveTodos(updated)
            print("Marked #" ++ idStr ++ " as done")
        }
        Zero -> print("Invalid id: " ++ idStr)
    }
}

fun cmdRemove(idStr: String) {
    idOpt = read(idStr, Int)
    match idOpt {
        Some(id) -> {
            todos = loadTodos()
            filtered = filter(fun(t) -> t.id != id, todos)
            saveTodos(filtered)
            print("Removed #" ++ idStr)
        }
        Zero -> print("Invalid id: " ++ idStr)
    }
}

fun cmdClear() {
    saveTodos([])
    print("All todos cleared")
}

fun showHelp() {
    print("
Todo CLI - Manage your tasks

Usage: lang todo.lang <command> [args]

Commands:
  add <text>    Add a new todo
  list          Show all todos
  done <id>     Mark todo as done
  remove <id>   Remove a todo
  clear         Remove all todos
  help          Show this help

Examples:
  lang todo.lang add \"Buy groceries\"
  lang todo.lang list
  lang todo.lang done 1
  lang todo.lang remove 1
")
}

// --- Main ---

fun main() {
    args = sysArgs()
    
    match args {
        [] -> showHelp()
        ["add", rest...] -> {
            text = foldl(fun(acc, s) -> acc ++ " " ++ s, "", rest) |> stringTrim
            if len(text) == 0 {
                print("Please provide todo text")
            } else {
                cmdAdd(text)
            }
        }
        ["list"] -> cmdList()
        ["done", id] -> cmdDone(id)
        ["remove", id] -> cmdRemove(id)
        ["clear"] -> cmdClear()
        ["help"] -> showHelp()
        _ -> showHelp()
    }
}

main()
```

### Usage

```bash
# Add tasks
lang todo.lang add "Buy milk"
lang todo.lang add "Write tests"
lang todo.lang add "Deploy to prod"

# View list
lang todo.lang list
#  Your Todos:
#    1   Buy milk
#    2   Write tests
#    3   Deploy to prod
#   0/3 completed

# Mark as done
lang todo.lang done 1
lang todo.lang list
#    1   Buy milk
#    2   Write tests
#    3   Deploy to prod
#   1/3 completed

# Remove
lang todo.lang remove 2

# Clear all
lang todo.lang clear
```

---

## Argument parsing

### Simple way

```rust
import "lib/sys" (sysArgs)

args = sysArgs()

match args {
    [] -> print("No arguments")
    [cmd] -> print("Command: " ++ cmd)
    [cmd, arg] -> print("Command: " ++ cmd ++ ", Arg: " ++ arg)
    [cmd, rest...] -> print("Command: " ++ cmd ++ ", Args: " ++ show(rest))
}
```

### Flag parsing

```rust
import "lib/sys" (sysArgs)
import "lib/string" (stringStartsWith)

type Options = {
    verbose: Bool,
    output: String,
    files: List<String>
}

fun parseArgs(args: List<String>) -> Options {
    fun go(args: List<String>, opts: Options) -> Options {
        match args {
            [] -> opts
            ["-v", rest...] -> go(rest, { ...opts, verbose: true })
            ["--verbose", rest...] -> go(rest, { ...opts, verbose: true })
            ["-o", out, rest...] -> go(rest, { ...opts, output: out })
            ["--output", out, rest...] -> go(rest, { ...opts, output: out })
            [arg, rest...] -> if stringStartsWith(arg, "-") { print("Unknown option: " ++ arg) go(rest, opts) } else { go(rest, { ...opts, files: opts.files ++ [arg] }) }
        }
    }
    
    go(args, { verbose: false, output: "", files: [] })
}

opts = parseArgs(sysArgs())
print("Verbose: " ++ show(opts.verbose))
print("Output: " ++ opts.output)
print("Files: " ++ show(opts.files))
```

---

## Colored output (ANSI)

```rust
// ANSI escape codes (in ANSI-supporting terminal)
red :- "\x1b[31m"
green :- "\x1b[32m"
yellow :- "\x1b[33m"
blue :- "\x1b[34m"
bold :- "\x1b[1m"
reset :- "\x1b[0m"

fun colorRed(s: String) -> String { red ++ s ++ reset }
fun colorGreen(s: String) -> String { green ++ s ++ reset }
fun colorYellow(s: String) -> String { yellow ++ s ++ reset }
fun colorBlue(s: String) -> String { blue ++ s ++ reset }
fun colorBold(s: String) -> String { bold ++ s ++ reset }

print(colorGreen("Success!"))
print(colorRed("Error!"))
print(colorYellow("Warning"))
print(colorBold(colorBlue("Info")))
```

---

## Progress indicator

```rust
import "lib/time" (sleepMs)
import "lib/string" (stringRepeat)
import "lib/list" (range)

fun progressBar(current: Int, total: Int, width: Int) -> String {
    percent = current * 100 / total
    filled = current * width / total
    bar = stringRepeat("#", filled) ++ stringRepeat("-", width - filled)
    "[" ++ bar ++ "] " ++ show(percent) ++ "%"
}

// Progress simulation
for i in range(1, 101) {
    write("\r" ++ progressBar(i, 100, 30))
    sleepMs(50)
}
print("\nDone!")
// ...
```

---

## Reading user input

```rust
import "lib/io" (readLine)

fun prompt(message: String) -> String {
    print(message)
    match readLine() {
        Some(line) -> line
        Zero -> ""
    }
}

fun confirm(message: String) -> Bool {
    answer = prompt(message ++ " (y/n): ")
    answer == "y" || answer == "Y" || answer == "yes"
}

// Usage
name = prompt("Enter your name: ")
print("Hello, " ++ name ++ "!")

if confirm("Do you want to continue?") {
    print("Continuing...")
} else {
    print("Aborted.")
}
// ...
```

---

## Environment variables

```rust
import "lib/sys" (sysEnv, sysExit)

// Read environment variable
match sysEnv("HOME") {
    Some(home) -> print("Home directory: " ++ home)
    Zero -> print("HOME not set")
}

// API key from environment
fun getApiKey() -> String {
    match sysEnv("API_KEY") {
        Some(key) -> key
        Zero -> {
            print("Error: API_KEY environment variable not set")
            sysExit(1)
            ""
        }
    }
}
// ...
```

---

## Exit codes

```rust
import "lib/sys" (sysExit)

fun main() -> Nil = {
    // ... do work ...
    
    if hasErrors then {
        print("Failed with errors")
        sysExit(1)  // failure
    } else {
        print("Success!")
        sysExit(0)  // success (optional, default)
    }
}
```

---

## Complete example: file search (grep)

```rust
import "lib/sys" (sysArgs, sysExit)
import "lib/io" (fileRead, dirList, isFile, isDir)
import "lib/path" (pathJoin)
import "lib/string" (stringLines, stringContains)

fun searchFile(pattern: String, path: String) -> List<(String, Int, String)> {
    match fileRead(path) {
        Ok(content) -> {
            lines = stringLines(content)
            results = []
            lineNum = 1
            for line in lines {
                if stringContains(line, pattern) {
                    results = results ++ [(path, lineNum, line)]
                }
                lineNum += 1
            }
            results
        }
        Fail(_) -> []
    }
}

fun searchDir(pattern: String, dir: String) -> List<(String, Int, String)> {
    match dirList(dir) {
        Ok(entries) -> {
            results = []
            for entry in entries {
                p = pathJoin([dir, entry])
                if isFile(p) {
                    results = results ++ searchFile(pattern, p)
                } else if isDir(p) {
                    results = results ++ searchDir(pattern, p)
                }
            }
            results
        }
        Fail(_) -> []
    }
}

fun main() {
    match sysArgs() {
        [pattern, path] -> {
            results = if isDir(path) { searchDir(pattern, path) } else { searchFile(pattern, path) }
            
            for (file, line, content) in results {
                print(file ++ ":" ++ show(line) ++ ": " ++ content)
            }
            
            print("\n" ++ show(len(results)) ++ " matches found")
        }
        _ -> {
            print("Usage: lang grep.lang <pattern> <path>")
            sysExit(1)
        }
    }
}

main()
// ...
```

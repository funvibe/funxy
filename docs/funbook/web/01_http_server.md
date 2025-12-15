# 01. HTTP Server

## Task
Create a web server to handle HTTP requests.

## Minimal server

```rust
import "lib/http" (httpServe)

fun handler(req) {
    { status: 200, body: "Hello, World!" }
}

// httpServe(8080, handler)
// Run and open: http://localhost:8080

```

## Routing

```rust
import "lib/http" (httpServe)

fun handler(req) {
    match req.path {
        "/" -> { status: 200, body: "Home page" }
        "/about" -> { status: 200, body: "About us" }
        "/health" -> { status: 200, body: "OK" }
        _ -> { status: 404, body: "Not found" }
    }
}

// httpServe(8080, handler)

```

## Method handling

```rust
import "lib/http" (httpServe)

fun handler(req) {
    match (req.method, req.path) {
        ("GET", "/") -> { status: 200, body: "Welcome!" }
        ("GET", "/users") -> { status: 200, body: "List users" }
        ("POST", "/users") -> { status: 201, body: "User created" }
        ("DELETE", "/users") -> { status: 200, body: "User deleted" }
        _ -> { status: 404, body: "Not found" }
    }
}

// httpServe(8080, handler)

```

## JSON API

```rust
import "lib/http" (httpServe)
import "lib/json" (jsonEncode, jsonDecode)

users = [
    { id: 1, name: "Alice" },
    { id: 2, name: "Bob" }
]

fun handler(req) {
    match (req.method, req.path) {
        ("GET", "/api/users") -> {
            status: 200,
            headers: { contentType: "application/json" },
            body: jsonEncode(users)
        }
        ("POST", "/api/users") -> {
            newUser = jsonDecode(req.body)
            {
                status: 201,
                headers: { contentType: "application/json" },
                body: jsonEncode({ success: true, user: newUser })
            }
        }
        _ -> { status: 404, body: "Not found" }
    }
}

// httpServe(8080, handler)

```

## HTML pages

```rust
import "lib/http" (httpServe)

fun htmlPage(title: String, content: String) -> String {
    "<!DOCTYPE html><html><head><title>" ++ title ++ "</title></head><body>" ++ content ++ "</body></html>"
}

fun handler(req) {
    match req.path {
        "/" -> {
            status: 200,
            headers: { contentType: "text/html" },
            body: htmlPage("Home", "<h1>Welcome to Funxy!</h1>")
        }
        _ -> { status: 404, body: "Not found" }
    }
}

// httpServe(8080, handler)

```

## Static files

```rust
import "lib/http" (httpServe)
import "lib/io" (fileRead, fileExists)
import "lib/string" (stringEndsWith)

fun contentType(path: String) -> String {
    match true {
        _ if stringEndsWith(path, ".html") -> "text/html"
        _ if stringEndsWith(path, ".css") -> "text/css"
        _ if stringEndsWith(path, ".js") -> "application/javascript"
        _ -> "text/plain"
    }
}

fun handler(req) {
    filePath = "public" ++ req.path
    if fileExists(filePath) {
        match fileRead(filePath) {
            Ok(content) -> {
                status: 200,
                headers: { contentType: contentType(filePath) },
                body: content
            }
            Fail(_) -> { status: 500, body: "Read error" }
        }
    } else {
        { status: 404, body: "File not found" }
    }
}

// httpServe(8080, handler)

```

## Query parameters

```rust
import "lib/http" (httpServe)
import "lib/url" (urlParse, urlQueryParam)

fun handler(req) {
    name = match urlParse("http://localhost" ++ req.path) {
        Ok(url) -> match urlQueryParam(url, "name") {
            Some(n) -> n
            Zero -> "Guest"
        }
        Fail(_) -> "Guest"
    }
    { status: 200, body: "Hello, " ++ name ++ "!" }
}

// httpServe(8080, handler)
// http://localhost:8080?name=Alice -> "Hello, Alice!"
```

## Middleware pattern

```rust
import "lib/http" (httpServe)

fun withLogging(handler) {
    fun(req) -> {
        response = handler(req)
        print("[" ++ req.method ++ "] " ++ req.path ++ " - " ++ show(response.status))
        response
    }
}

fun app(req) {
    match req.path {
        "/" -> { status: 200, body: "Home" }
        _ -> { status: 404, body: "Not found" }
    }
}

// httpServe(8080, withLogging(app))

```

## Complete example: REST API

```rust
import "lib/http" (httpServe)
import "lib/json" (jsonEncode, jsonDecode)
import "lib/uuid" (uuidNew, uuidToString)
import "lib/list" (find)

// In-memory "database"
todos = []

fun findTodo(id: String) {
    find(fun(t) -> t.id == id, todos)
}

fun handler(req) {
    match (req.method, req.path) {
        // List all todos
        ("GET", "/todos") -> {
            status: 200,
            headers: { contentType: "application/json" },
            body: jsonEncode(todos)
        }
        
        // Create todo
        ("POST", "/todos") -> match jsonDecode(req.body) {
            Ok(data) -> {
                todo = {
                    id: uuidToString(uuidNew()),
                    title: data.title,
                    done: false
                }
                todos = todos ++ [todo]
                {
                    status: 201,
                    headers: { contentType: "application/json" },
                    body: jsonEncode(todo)
                }
            }
            Fail(_) -> { status: 400, body: "Invalid JSON" }
        }
        
        _ -> { status: 404, body: "Not found" }
    }
}

// httpServe(8080, handler)

# HTTP Client (lib/http)

The `lib/http` module provides functions for making HTTP requests.

```rust
import "lib/http" (*)
```

## Response Type

All functions return `Result<HttpResponse, String>`, where:

```rust
type HttpResponse = {
    status: Int,              // HTTP status code (200, 404, 500, ...)
    body: String,             // Response body
    headers: List<(String, String)>  // Response headers
}
```

## Functions

### httpGet

```rust
httpGet(url: String) -> Result<HttpResponse, String>
```

Performs a GET request.

```rust
import "lib/http" (httpGet)

result = httpGet("https://api.example.com/users")

match result {
    Ok(resp) -> {
        print("Status: ${resp.status}")
        print("Body: ${resp.body}")
    }
    Fail(err) -> print("Error: ${err}")
}
```

### httpPost

```rust
httpPost(url: String, body: String) -> Result<HttpResponse, String>
```

Performs a POST request with string body.

```rust
import "lib/http" (httpPost)

result = httpPost("https://api.example.com/users", "name=John&age=30")

match result {
    Ok(resp) -> print("Created: ${resp.status}")
    Fail(err) -> print("Error: ${err}")
}
```

### httpPostJson

```rust
httpPostJson(url: String, data: A) -> Result<HttpResponse, String>
```

Performs a POST request with automatic JSON serialization of data. Adds `Content-Type: application/json` header.

```rust
import "lib/http" (httpPostJson)

user = { name: "John", age: 30, active: true }
result = httpPostJson("https://api.example.com/users", user)

match result {
    Ok(resp) -> print("Created: ${resp.status}")
    Fail(err) -> print("Error: ${err}")
}

// Lists also work
items = [1, 2, 3]
httpPostJson("https://api.example.com/items", items)
```

### httpPut

```rust
httpPut(url: String, body: String) -> Result<HttpResponse, String>
```

Performs a PUT request to update a resource.

```rust
import "lib/http" (httpPut)

result = httpPut("https://api.example.com/users/1", "{\"name\": \"Jane\"}")

match result {
    Ok(resp) -> print("Updated: ${resp.status}")
    Fail(err) -> print("Error: ${err}")
}
```

### httpDelete

```rust
httpDelete(url: String) -> Result<HttpResponse, String>
```

Performs a DELETE request.

```rust
import "lib/http" (httpDelete)

result = httpDelete("https://api.example.com/users/1")

match result {
    Ok(resp) -> print("Deleted: ${resp.status}")
    Fail(err) -> print("Error: ${err}")
}
```

### httpRequest

```
httpRequest(method: String, url: String, headers: List<(String, String)>, body: String = "", timeout: Int = 0) -> Result<HttpResponse, String>
```

Full control over HTTP request: method, headers, body, timeout.

**Default parameters:**
- **body** — request body (default `""`)
- **timeout** — timeout in milliseconds. If `0` — uses global timeout (default 30000 ms)

```rust
import "lib/http" (httpRequest)

// Minimal call - only method, url, headers
// body="" and timeout=0 (global) will be added automatically
result = httpRequest("GET", "https://api.example.com/users", [])

// With body, but global timeout
headers = [("Content-Type", "application/json")]
result = httpRequest("POST", "https://api.example.com/users", headers, "{\"name\":\"John\"}")

// Full control: all parameters explicit
headers = [
    ("Authorization", "Bearer token123"),
    ("Content-Type", "application/json"),
    ("Accept", "application/json")
]
body = "{\"query\": \"search term\"}"
result = httpRequest("POST", "https://api.example.com/search", headers, body, 5000)

match result {
    Ok(resp) -> {
        print("Status: ${resp.status}")
        // Find header in response
        for header in resp.headers {
            (key, value) = header
            if key == "Content-Type" {
                print("Content-Type: ${value}")
            }
        }
    }
    Fail(err) -> print("Error: ${err}")
}
```

### httpSetTimeout

```rust
httpSetTimeout(milliseconds: Int) -> Nil
```

Sets timeout for all subsequent requests (default 30000 ms = 30 seconds).

```rust
import "lib/http" (httpSetTimeout, httpGet)

// Set timeout to 5 seconds
httpSetTimeout(5000)

// Now requests will be interrupted after 5 seconds
result = httpGet("https://slow-api.example.com/data")
```

## Practical Examples

### Getting and Parsing JSON

```rust
import "lib/http" (httpGet)
import "lib/json" (jsonDecode)

type User = { name: String, email: String }

fun fetchUser(id: Int) -> Result<String, User> {
    match httpGet("https://api.example.com/users/${id}") {
        Ok(resp) -> {
            if resp.status == 200 {
                jsonDecode(resp.body)
            } else {
                Fail("HTTP ${resp.status}")
            }
        }
        Fail(err) -> Fail(err)
    }
}

match fetchUser(1) {
    Ok(user) -> print("User: ${user.name}")
    Fail(err) -> print("Error: ${err}")
}
```

### REST API Client

```rust
import "lib/http" (httpGet, httpPostJson, httpDelete)
import "lib/json" (jsonDecode)

type User = { id: Int, name: String, email: String }

baseUrl = "https://api.example.com"

fun getUsers() -> Result<String, List<User>> {
    match httpGet("${baseUrl}/users") {
        Ok(resp) -> {
            if resp.status == 200 { jsonDecode(resp.body) }
            else { Fail("HTTP ${resp.status}") }
        }
        Fail(err) -> Fail(err)
    }
}

fun createUser(name: String, email: String) -> Result<String, User> {
    data = { name: name, email: email }
    match httpPostJson("${baseUrl}/users", data) {
        Ok(resp) -> {
            if resp.status == 201 { jsonDecode(resp.body) }
            else { Fail("HTTP ${resp.status}") }
        }
        Fail(err) -> Fail(err)
    }
}

fun deleteUser(userId: Int) -> Result<String, Nil> {
    match httpDelete("${baseUrl}/users/${userId}") {
        Ok(resp) -> {
            if resp.status == 204 { Ok(Nil) }
            else { Fail("HTTP ${resp.status}") }
        }
        Fail(err) -> Fail(err)
    }
}
```

### Working with Headers

```rust
import "lib/http" (httpRequest)

fun authenticatedGet(url: String, token: String) -> Result<String, String> {
    headers = [
        ("Authorization", "Bearer ${token}"),
        ("Accept", "application/json")
    ]
    
    match httpRequest("GET", url, headers, "") {
        Ok(resp) -> {
            if resp.status == 200 { Ok(resp.body) }
            else if resp.status == 401 { Fail("Unauthorized") }
            else { Fail("HTTP ${resp.status}") }
        }
        Fail(err) -> Fail(err)
    }
}
```

### Retry Logic

```rust
import "lib/http" (httpGet)
import "lib/time" (sleepMs)

fun fetchWithRetry(url: String, maxRetries: Int) -> Result<String, String> {
    fun attempt(n: Int) -> Result<String, String> {
        if n > maxRetries {
            Fail("Max retries exceeded")
        } else {
            match httpGet(url) {
                Ok(resp) -> {
                    if resp.status == 200 { Ok(resp.body) }
                    else if resp.status >= 500 {
                        // Server error - retry
                        sleepMs(1000 * n)  // Exponential backoff
                        attempt(n + 1)
                    }
                    else { Fail("HTTP ${resp.status}") }
                }
                Fail(_) -> {
                    sleepMs(1000 * n)
                    attempt(n + 1)
                }
            }
        }
    }
    attempt(1)
}
```

## Summary

| Function | Type | Description |
|---------|-----|----------|
| `httpGet` | `String -> Result<HttpResponse, String>` | GET request |
| `httpPost` | `(String, String) -> Result<HttpResponse, String>` | POST with string |
| `httpPostJson` | `(String, A) -> Result<HttpResponse, String>` | POST with JSON |
| `httpPut` | `(String, String) -> Result<HttpResponse, String>` | PUT request |
| `httpDelete` | `String -> Result<HttpResponse, String>` | DELETE request |
| `httpRequest` | `(String, String, List<(String,String)>, String, Int) -> Result<HttpResponse, String>` | Full control (with timeout) |
| `httpSetTimeout` | `Int -> Nil` | Set timeout |

## HTTP Server

### httpServe

```rust
httpServe(port: Int, handler: (HttpRequest) -> HttpResponse) -> Result<Nil, String>
```

Starts an HTTP server on the specified port. Blocks program execution.

```rust
type HttpRequest = {
    method: String,              // "GET", "POST", etc.
    path: String,                // "/api/users"
    query: String,               // "id=1&name=test"
    headers: List<(String, String)>,
    body: String
}
```

```
import "lib/http" (httpServe)

fun handler(req: HttpRequest) -> HttpResponse {
    if req.path == "/" {
        { status: 200, body: "Hello, World!", headers: [] }
    } else if req.path == "/api/data" {
        { status: 200, body: "{\"value\": 42}", headers: [("Content-Type", "application/json")] }
    } else {
        { status: 404, body: "Not Found", headers: [] }
    }
}

// Start server on port 8080
print("Starting server on http://localhost:8080")
httpServe(8080, handler)
```

### Example: JSON API Server

```
import "lib/http" (httpServe)
import "lib/json" (jsonEncode, jsonDecode)
import "lib/list" (length)

type User = { id: Int, name: String }
type UserInput = { name: String }

users = [
    { id: 1, name: "Alice" },
    { id: 2, name: "Bob" }
]

fun handler(req: HttpRequest) -> HttpResponse {
    match (req.method, req.path) {
        ("GET", "/users") -> {
            { status: 200, body: jsonEncode(users), headers: [("Content-Type", "application/json")] }
        }
        ("POST", "/users") -> {
            match jsonDecode(req.body) : Result<String, UserInput> {
                Ok(data) -> {
                    newUser: User = { id: length(users) + 1, name: data.name }
                    { status: 201, body: jsonEncode(newUser), headers: [("Content-Type", "application/json")] }
                }
                Fail(err) -> {
                    { status: 400, body: "{\"error\": \"Invalid JSON\"}", headers: [("Content-Type", "application/json")] }
                }
            }
        }
        _ -> { status: 404, body: "Not Found", headers: [] }
    }
}

httpServe(8080, handler)
```

### httpServeAsync (non-blocking server)

```
httpServeAsync(port: Int, handler: (HttpRequest) -> HttpResponse) -> Int
```

Starts an HTTP server in background mode and returns server ID. Doesn't block program execution.

```
import "lib/http" (httpServeAsync, httpServerStop, httpGet)

fun handler(req: HttpRequest) -> HttpResponse {
    { status: 200, body: "Hello!", headers: [] }
}

// Start server in background
serverId = httpServeAsync(8080, handler)
print("Server started with ID: ${serverId}")

// Now you can make requests to the server
response = httpGet("http://localhost:8080/")
print(response)

// Stop server
httpServerStop(serverId)
print("Server stopped")
```

### httpServerStop

```rust
httpServerStop(serverId: Int) -> Nil
```

Stops a running server by its ID.

```rust
serverId = httpServeAsync(8080, handler)

// ... work with server ...

httpServerStop(serverId)  // Graceful shutdown
```

## Limitations

### Client
- Only synchronous requests
- No built-in cookie support (can be passed in headers)
- No automatic redirects (can be handled manually)
- Global timeout (applies to all requests)

### Server
- No HTTPS support (use reverse proxy)
- No routing (implement in handler)

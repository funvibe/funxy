# Web UI Demo

Example web application demonstrating `kit/web` router with `kit/ui` HTML rendering.

## Features

- **Clean block syntax**: Uses trailing block syntax for DSL-style component composition
- **Nested components**: Demonstrates nested UI components with `div`, `span`, `h1`, etc.
- **HTTP routing**: Multi-route setup with `kit/web` router
- **HTML rendering**: Server-side HTML generation with `kit/ui`
- **JSON API**: Example JSON endpoint

## Running

```bash
./funxy examples/web_ui_demo.lang
```

Then visit:
- http://localhost:8080/ - Home page
- http://localhost:8080/about - About page
- http://localhost:8080/users - User list
- http://localhost:8080/api/data - JSON API endpoint

## Code Structure

### Components

The example uses clean block syntax for UI composition:

```rust
fun layout(pageTitle, content) {
    html {
        head {
            title(pageTitle)
        }
        body {
            div {
                h1 { text("My Web App") }
                content
            }
        }
    }
}
```

### Handlers

Each route handler returns `Result<String, Response>`:

```rust
fun homeHandler(ctx) {
    Ok(resHtml(render(homePage())))
}
```

### Router Setup

Routes are configured with pipe syntax:

```rust
router = setNotFoundHandler(
    newRouter()
        |> get("/", homeHandler)
        |> get("/about", aboutHandler)
        |> get("/users", usersHandler)
        |> get("/api/data", apiHandler),
    notFoundHandler
)
```


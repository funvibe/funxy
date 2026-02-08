# 11. Recipes

[← Back to Index](./00_index.md)

## REST API with SQLite

```text
import "lib/http" (*)
import "lib/json" (*)
import "lib/sql" (*)

// Database Setup
db = match sqlOpen("sqlite", "app.db") {
    Ok(conn) -> conn
    Fail(e) -> panic("DB error: " ++ e)
}

sqlExec(db, `CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT)`, [])

// Types
type alias HttpRequest = { method: String, path: String, query: String, headers: List<(String, String)>, body: String }
type alias HttpResponse = { status: Int, body: String, headers: List<(String, String)> }

// Handler
fun handler(req: HttpRequest) -> HttpResponse {
    match (req.method, req.path) {
        ("GET", "/users") -> {
            rows = sqlQuery(db, "SELECT * FROM users", [])?
            users = [ { id: row["id"], name: row["name"] } | row <- rows ]
            { status: 200, body: jsonEncode(users), headers: [] }
        }
        ("POST", "/users") -> {
            match jsonDecode(req.body, { name: String }) {
                Ok(data) -> {
                    sqlExec(db, "INSERT INTO users (name) VALUES ($1)", [SqlString(data.name)])
                    { status: 201, body: "Created", headers: [] }
                }
                Fail(_) -> { status: 400, body: "Bad JSON", headers: [] }
            }
        }
        _ -> { status: 404, body: "Not Found", headers: [] }
    }
}

httpServe(8080, handler)
```

## CLI Tool

```text
import "lib/flag" (*)
import "lib/io" (*)
import "lib/string" (*)

flagSet("input", "", "Input file")
flagSet("upper", false, "Convert to uppercase")

fun main() {
    flagParse()

    input = flagGet("input")
    if input == "" {
        print("Usage: tool --input <file>")
        return ()
    }

    match fileRead(input) {
        Ok(content) -> {
            result = if flagGet("upper") {
                stringToUpper(content)
            } else {
                content
            }
            print(result)
        }
        Fail(e) -> print("Error: " ++ e)
    }
}

main()
```

## Data Processing

```text
import "lib/csv" (*)
import "lib/list" (*)

// Calculate total sales by region
fun main() {
    rows = csvRead("sales.csv")?

    // Group by region and sum amounts
    salesByRegion = foldl(
        fun(acc, row) -> {
            region = row.region
            amount = read(row.amount, Float) ?? 0.0
            current = mapGetOr(acc, region, 0.0)
            mapPut(acc, region, current + amount)
        },
        %{},
        rows
    )

    for (region, total) in mapItems(salesByRegion) {
        print(region ++ ": $" ++ show(total))
    }
}
```

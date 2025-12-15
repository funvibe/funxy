# SQL Database

Funxy provides SQLite database access through `lib/sql`.

## Getting Started

```rust
import "lib/sql" (*)

// Open in-memory database
match sqlOpen("sqlite", ":memory:") {
    Ok(db) -> {
        // Use database
        sqlClose(db)
    }
    Fail(e) -> print("Error: " ++ e)
}

// Open file-based database
match sqlOpen("sqlite", "myapp.db") {
    Ok(db) -> {
        // Database file created if doesn't exist
        sqlClose(db)
    }
    Fail(e) -> print("Error: " ++ e)
}
```

## SqlValue Type

SQL values are represented by the `SqlValue` ADT:

```rust
type SqlValue = SqlNull                  // NULL
              | SqlInt(Int)              // INTEGER
              | SqlFloat(Float)          // REAL
              | SqlString(String)        // TEXT
              | SqlBool(Bool)            // INTEGER (0/1)
              | SqlBytes(Bytes)          // BLOB
              | SqlTime(Date)            // TEXT (ISO 8601)
              | SqlBigInt(BigInt)        // TEXT (arbitrary precision)
// ...
```

## Creating Tables

```rust
import "lib/sql" (*)

match sqlOpen("sqlite", ":memory:") {
    Ok(db) -> {
        // Create table
        sql = "CREATE TABLE users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            email TEXT UNIQUE,
            age INTEGER,
            balance REAL,
            is_active INTEGER DEFAULT 1,
            created_at TEXT,
            avatar BLOB
        )"
        
        match sqlExec(db, sql, []) {
            Ok(_) -> print("Table created")
            Fail(e) -> print("Error: " ++ e)
        }
        
        sqlClose(db)
    }
    Fail(e) -> print("Connection failed: " ++ e)
}
```

## Inserting Data

Use `$1`, `$2`, ... placeholders for parameters:

```rust
import "lib/sql" (*)
import "lib/date" (dateNow)

match sqlOpen("sqlite", ":memory:") {
    Ok(db) -> {
        // Create table first
        sqlExec(db, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, age INTEGER, created_at TEXT)", [])
        
        // Insert with parameters
        match sqlExec(db, "INSERT INTO users (name, age) VALUES ($1, $2)", [SqlString("Alice"), SqlInt(30)]) {
            Ok(n) -> print("Inserted " ++ show(n) ++ " row(s)")
            Fail(e) -> print("Insert error: " ++ e)
        }
        
        // Insert with date
        now = dateNow()
        sqlExec(db, "INSERT INTO users (name, age, created_at) VALUES ($1, $2, $3)", 
            [SqlString("Bob"), SqlInt(25), SqlTime(now)])
        
        // Get last insert ID
        match sqlLastInsertId(db, "INSERT INTO users (name, age) VALUES ($1, $2)", [SqlString("Charlie"), SqlInt(35)]) {
            Ok(id) -> print("Inserted with ID: " ++ show(id))
            Fail(e) -> print("Insert error: " ++ e)
        }
        
        sqlClose(db)
    }
    Fail(e) -> print(e)
}
```

## Querying Data

### Multiple Rows

```rust
import "lib/sql" (*)
import "lib/map" (mapGet)
import "lib/list" (forEach)

match sqlOpen("sqlite", ":memory:") {
    Ok(db) -> {
        sqlExec(db, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, age INTEGER)", [])
        sqlExec(db, "INSERT INTO users (name, age) VALUES ('Alice', 30), ('Bob', 25), ('Charlie', 35)", [])
        
        // Query all rows
        match sqlQuery(db, "SELECT * FROM users ORDER BY age", []) {
            Ok(rows) -> {
                print("Found " ++ show(len(rows)) ++ " users")
                
                // Process each row (Row = Map<String, SqlValue>)
                forEach(fun(row) -> {
                    match mapGet(row, "name") {
                        Some(SqlString(name)) -> print("Name: " ++ name)
                        _ -> Nil
                    }
                }, rows)
            }
            Fail(e) -> print("Query error: " ++ e)
        }
        
        // Query with filter
        match sqlQuery(db, "SELECT name, age FROM users WHERE age > $1", [SqlInt(26)]) {
            Ok(rows) -> print("Users over 26: " ++ show(len(rows)))
            Fail(e) -> print(e)
        }
        
        sqlClose(db)
    }
    Fail(e) -> print(e)
}
```

### Single Row

```rust
import "lib/sql" (*)
import "lib/map" (mapGet)

match sqlOpen("sqlite", ":memory:") {
    Ok(db) -> {
        sqlExec(db, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, age INTEGER)", [])
        sqlExec(db, "INSERT INTO users (name, age) VALUES ('Alice', 30)", [])
        
        // Query single row
        match sqlQueryRow(db, "SELECT * FROM users WHERE name = $1", [SqlString("Alice")]) {
            Ok(Some(row)) -> {
                match mapGet(row, "age") {
                    Some(sqlVal) -> match sqlUnwrap(sqlVal) {
                        Some(age) -> print("Alice is " ++ show(age) ++ " years old")
                        Zero -> print("Age is NULL")
                    }
                    Zero -> print("No age column")
                }
            }
            Ok(Zero) -> print("User not found")
            Fail(e) -> print("Query error: " ++ e)
        }
        
        sqlClose(db)
    }
    Fail(e) -> print(e)
}
```

## Updating and Deleting

```rust
import "lib/sql" (*)

match sqlOpen("sqlite", ":memory:") {
    Ok(db) -> {
        sqlExec(db, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, age INTEGER)", [])
        sqlExec(db, "INSERT INTO users (name, age) VALUES ('Alice', 30)", [])
        
        // Update
        match sqlExec(db, "UPDATE users SET age = $1 WHERE name = $2", [SqlInt(31), SqlString("Alice")]) {
            Ok(n) -> print("Updated " ++ show(n) ++ " row(s)")
            Fail(e) -> print(e)
        }
        
        // Delete
        match sqlExec(db, "DELETE FROM users WHERE age < $1", [SqlInt(25)]) {
            Ok(n) -> print("Deleted " ++ show(n) ++ " row(s)")
            Fail(e) -> print(e)
        }
        
        sqlClose(db)
    }
    Fail(e) -> print(e)
}
```

## Transactions

```rust
import "lib/sql" (*)

match sqlOpen("sqlite", ":memory:") {
    Ok(db) -> {
        sqlExec(db, "CREATE TABLE accounts (id INTEGER PRIMARY KEY, name TEXT, balance REAL)", [])
        sqlExec(db, "INSERT INTO accounts (name, balance) VALUES ('Alice', 1000), ('Bob', 500)", [])
        
        // Transfer money atomically
        transfer = fun(fromName, toName, amount) -> {
            match sqlBegin(db) {
                Ok(tx) -> {
                    // Deduct from sender
                    result1 = sqlTxExec(tx, "UPDATE accounts SET balance = balance - $1 WHERE name = $2", 
                        [SqlFloat(amount), SqlString(fromName)])
                    
                    match result1 {
                        Ok(_) -> {
                            // Add to receiver
                            result2 = sqlTxExec(tx, "UPDATE accounts SET balance = balance + $1 WHERE name = $2",
                                [SqlFloat(amount), SqlString(toName)])
                            
                            match result2 {
                                Ok(_) -> {
                                    match sqlCommit(tx) {
                                        Ok(_) -> print("Transfer successful")
                                        Fail(e) -> print("Commit failed: " ++ e)
                                    }
                                }
                                Fail(e) -> {
                                    sqlRollback(tx)
                                    print("Transfer failed, rolled back: " ++ e)
                                }
                            }
                        }
                        Fail(e) -> {
                            sqlRollback(tx)
                            print("Transfer failed, rolled back: " ++ e)
                        }
                    }
                }
                Fail(e) -> print("Could not start transaction: " ++ e)
            }
        }
        
        transfer("Alice", "Bob", 100.0)
        
        sqlClose(db)
    }
    Fail(e) -> print(e)
}
```

## Working with NULL

```rust
import "lib/sql" (*)
import "lib/map" (mapGet)

match sqlOpen("sqlite", ":memory:") {
    Ok(db) -> {
        sqlExec(db, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, nickname TEXT)", [])
        sqlExec(db, "INSERT INTO users (name, nickname) VALUES ('Alice', NULL)", [])
        
        // Insert NULL explicitly
        sqlExec(db, "INSERT INTO users (name, nickname) VALUES ($1, $2)", 
            [SqlString("Bob"), SqlNull])
        
        match sqlQueryRow(db, "SELECT * FROM users WHERE name = 'Alice'", []) {
            Ok(Some(row)) -> {
                match mapGet(row, "nickname") {
                    Some(sqlVal) -> {
                        if sqlIsNull(sqlVal) {
                            print("Nickname is NULL")
                        } else {
                            match sqlUnwrap(sqlVal) {
                                Some(nick) -> print("Nickname: " ++ show(nick))
                                Zero -> print("Could not unwrap")
                            }
                        }
                    }
                    Zero -> print("No nickname column")
                }
            }
            Ok(Zero) -> print("Not found")
            Fail(e) -> print(e)
        }
        
        sqlClose(db)
    }
    Fail(e) -> print(e)
}
```

## All Data Types

```rust
import "lib/sql" (*)
import "lib/date" (dateNow)
import "lib/bignum" (bigIntNew)

match sqlOpen("sqlite", ":memory:") {
    Ok(db) -> {
        sqlExec(db, "CREATE TABLE data (
            int_val INTEGER,
            float_val REAL,
            str_val TEXT,
            bool_val INTEGER,
            bytes_val BLOB,
            time_val TEXT,
            bigint_val TEXT
        )", [])
        
        // Insert all types
        sqlExec(db, "INSERT INTO data VALUES ($1, $2, $3, $4, $5, $6, $7)", [
            SqlInt(42),
            SqlFloat(3.14159),
            SqlString("Hello, World!"),
            SqlBool(true),
            SqlBytes(@"binary data"),
            SqlTime(dateNow()),
            SqlBigInt(bigIntNew("12345678901234567890"))
        ])
        
        print("All types inserted successfully")
        
        sqlClose(db)
    }
    Fail(e) -> print(e)
}
```

## Error Handling

All SQL functions return `Result<String, T>`:

```rust
import "lib/sql" (*)

// Pattern: handle both success and failure
handleResult = fun(result, onSuccess, onError) -> match result {
    Ok(val) -> onSuccess(val)
    Fail(err) -> onError(err)
}

match sqlOpen("sqlite", ":memory:") {
    Ok(db) -> {
        // Intentional SQL error
        match sqlExec(db, "INVALID SQL SYNTAX", []) {
            Ok(_) -> print("Unexpected success")
            Fail(e) -> print("Expected error: " ++ e)
        }
        
        // Table doesn't exist
        match sqlQuery(db, "SELECT * FROM nonexistent", []) {
            Ok(_) -> print("Unexpected success")
            Fail(e) -> print("Expected error: " ++ e)
        }
        
        sqlClose(db)
    }
    Fail(e) -> print("Connection error: " ++ e)
}
```

## Summary

| Function | Description |
|----------|-------------|
| `sqlOpen(driver, dsn)` | Open connection |
| `sqlClose(db)` | Close connection |
| `sqlPing(db)` | Test connection |
| `sqlQuery(db, sql, params)` | SELECT → List<Row> |
| `sqlQueryRow(db, sql, params)` | SELECT → Option<Row> |
| `sqlExec(db, sql, params)` | INSERT/UPDATE/DELETE → affected rows |
| `sqlLastInsertId(db, sql, params)` | INSERT → last ID |
| `sqlBegin(db)` | Start transaction |
| `sqlCommit(tx)` | Commit |
| `sqlRollback(tx)` | Rollback |
| `sqlTxQuery(tx, sql, params)` | Query in tx |
| `sqlTxExec(tx, sql, params)` | Execute in tx |
| `sqlUnwrap(sqlVal)` | Extract value → Option |
| `sqlIsNull(sqlVal)` | Check NULL → Bool |


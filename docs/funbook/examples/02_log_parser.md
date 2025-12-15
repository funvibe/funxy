# 25. Log Parser

## Task
Parse log files, extract structured data, analyze and aggregate.

---

## Example: parsing Apache/Nginx access log

### Log format

```
192.168.1.1 - - [10/Dec/2024:10:15:30 +0000] "GET /api/users HTTP/1.1" 200 1234 "-" "Mozilla/5.0"
```

### log_parser.lang

```rust
import "lib/io" (fileRead)
import "lib/sys" (sysArgs)
import "lib/string" (stringSplit, stringTrim, stringLines, stringStartsWith, stringRepeat)
import "lib/regex" (regexCapture)
import "lib/list" (filter, map, foldl, sortBy, take, forEach)
import "lib/map" (mapGet, mapItems, mapSize, mapPut)
import "lib/tuple" (fst, snd)
import "lib/math" (round)

// --- Types ---

type LogEntry = {
    ip: String,
    timestamp: String,
    method: String,
    path: String,
    status: Int,
    size: Int,
    userAgent: String
}

type Stats = {
    totalRequests: Int,
    uniqueIps: Int,
    statusCounts: Map<Int, Int>,
    topPaths: List<(String, Int)>,
    topIps: List<(String, Int)>,
    avgResponseSize: Float
}

// --- Parsing ---

fun parseLine(line: String) -> Option<LogEntry> {
    // Regex for Apache Combined Log Format
    pattern = "^([\\d.]+) - - \\[([^\\]]+)\\] \"(\\w+) ([^\"]+) HTTP/[^\"]+\" (\\d+) (\\d+) \"[^\"]*\" \"([^\"]*)\""
    
    match regexCapture(pattern, line) {
        Some(groups) -> {
            statusOpt = read(groups[5], Int)
            sizeOpt = read(groups[6], Int)
            match (statusOpt, sizeOpt) {
                (Some(st), Some(sz)) -> Some({
                    ip: groups[1],
                    timestamp: groups[2],
                    method: groups[3],
                    path: groups[4],
                    status: st,
                    size: sz,
                    userAgent: groups[7]
                })
                _ -> Zero
            }
        }
        Zero -> Zero
    }
}

fun parseLog(content: String) -> List<LogEntry> {
    content
    |> stringLines
    |> filter(fun(line) -> len(line) > 0)
    |> map(fun(line) -> parseLine(line))
    |> filter(fun(opt) -> match opt { Some(_) -> true, Zero -> false })
    |> map(fun(opt) -> match opt { Some(e) -> e, Zero -> { ip: "", timestamp: "", method: "", path: "", status: 0, size: 0, userAgent: "" } })
}

// --- Analysis ---

fun countBy(items, keyFn) {
    foldl(fun(acc, item) -> {
        key = keyFn(item)
        count = match mapGet(acc, key) { Some(n) -> n, Zero -> 0 }
        mapPut(acc, key, count + 1)
    }, %{}, items)
}

fun topN(counts, n: Int) {
    items = mapItems(counts)
    sorted = sortBy(items, fun(a, b) -> snd(b) - snd(a))
    take(sorted, n)
}

fun analyze(entries: List<LogEntry>) -> Stats {
    ipCounts = countBy(entries, fun(e) -> e.ip)
    pathCounts = countBy(entries, fun(e) -> e.path)
    statusCounts = countBy(entries, fun(e) -> e.status)
    
    totalSize = foldl(fun(acc, e) -> acc + e.size, 0, entries)
    avgSize = if len(entries) > 0 { intToFloat(totalSize) / intToFloat(len(entries)) } else { 0.0 }
    
    {
        totalRequests: len(entries),
        uniqueIps: mapSize(ipCounts),
        statusCounts: statusCounts,
        topPaths: topN(pathCounts, 10),
        topIps: topN(ipCounts, 10),
        avgResponseSize: avgSize
    }
}

// --- Filters ---

fun filterByStatus(entries: List<LogEntry>, status: Int) -> List<LogEntry> {
    filter(fun(e) -> e.status == status, entries)
}

fun filterByMethod(entries: List<LogEntry>, method: String) -> List<LogEntry> {
    filter(fun(e) -> e.method == method, entries)
}

fun filterByPath(entries: List<LogEntry>, pathPrefix: String) -> List<LogEntry> {
    filter(fun(e) -> stringStartsWith(e.path, pathPrefix), entries)
}

fun filterErrors(entries: List<LogEntry>) -> List<LogEntry> {
    filter(fun(e) -> e.status >= 400, entries)
}

// --- Report ---

fun printStats(stats: Stats) {
    print("\nLog Analysis Report\n")
    print(stringRepeat("=", 50))
    
    print("\nOverview:")
    print("  Total Requests: " ++ show(stats.totalRequests))
    print("  Unique IPs: " ++ show(stats.uniqueIps))
    print("  Avg Response Size: " ++ show(round(stats.avgResponseSize)) ++ " bytes")
    
    print("\nStatus Codes:")
    sorted = sortBy(mapItems(stats.statusCounts), fun(a, b) -> fst(a) - fst(b))
    forEach(fun(item) -> {
        status = fst(item)
        count = snd(item)
        percent = count * 100 / stats.totalRequests
        print("  " ++ show(status) ++ ": " ++ show(count) ++ " (" ++ show(percent) ++ "%)")
    }, sorted)
    
    print("\nTop 10 Paths:")
    forEach(fun(item) -> print("  " ++ show(snd(item)) ++ " - " ++ fst(item)), stats.topPaths)
    
    print("\nTop 10 IPs:")
    forEach(fun(item) -> print("  " ++ show(snd(item)) ++ " - " ++ fst(item)), stats.topIps)
    
    print("")
}

// --- Main ---

fun main() {
    match sysArgs() {
        [logFile] -> {
            match fileRead(logFile) {
                Ok(content) -> {
                    print("Parsing " ++ logFile ++ "...")
                    entries = parseLog(content)
                    print("Parsed " ++ show(len(entries)) ++ " entries")
                    
                    stats = analyze(entries)
                    printStats(stats)
                    
                    // Show errors
                    errors = filterErrors(entries)
                    if len(errors) > 0 {
                        print("\nRecent Errors:")
                        forEach(fun(e) -> print("  " ++ show(e.status) ++ " " ++ e.method ++ " " ++ e.path), take(errors, 5))
                    }
                }
                Fail(e) -> print("Error reading file: " ++ e)
            }
        }
        _ -> print("Usage: lang log_parser.lang <access.log>")
    }
}

main()
```

---

## Example: JSON logs (structured logging)

### Format

```json
{"timestamp":"2024-12-10T10:15:30Z","level":"info","msg":"User logged in","user_id":123}
{"timestamp":"2024-12-10T10:15:31Z","level":"error","msg":"Database connection failed","error":"timeout"}
```

### json_log_parser.lang

```rust
import "lib/io" (fileRead)
import "lib/json" (jsonDecode)
import "lib/string" (stringLines)
import "lib/list" (filter, map)

type LogEntry = {
    timestamp: String,
    level: String,
    msg: String,
    extra: Map<String, Any>
}

fun parseLine(line: String) -> Option<LogEntry> {
    match jsonDecode(line) {
        Ok(data) -> Some({
            timestamp: data.timestamp,
            level: data.level,
            msg: data.msg,
            extra: data
        })
        Fail(_) -> Zero
    }
}

fun parseLog(content: String) -> List<LogEntry> {
    content
    |> stringLines
    |> filter(fun(line) -> len(line) > 0)
    |> map(fun(line) -> parseLine(line))
    |> filter(fun(opt) -> match opt { Some(_) -> true, _ -> false })
    |> map(fun(opt) -> match opt { Some(e) -> e, _ -> { timestamp: "", level: "", msg: "", extra: %{} } })
}

fun filterByLevel(entries: List<LogEntry>, level: String) -> List<LogEntry> {
    filter(fun(e) -> e.level == level, entries)
}

fun main() {
    match fileRead("app.log") {
        Ok(content) -> {
            entries = parseLog(content)
            
            print("Log Summary:")
            print("  Total entries: " ++ show(len(entries)))
            
            errors = filterByLevel(entries, "error")
            print("  Errors: " ++ show(len(errors)))
            
            if len(errors) > 0 {
                print("\n Error messages:")
                for e in errors {
                    print("  [" ++ e.timestamp ++ "] " ++ e.msg)
                }
            }
        }
        Fail e -> print("Error: " ++ e)
    }
}

main()
```

---

## Time aggregation

```rust
import "lib/regex" (regexCapture)
import "lib/string" (stringPadLeft, stringRepeat)
import "lib/list" (foldl, range)
import "lib/map" (mapGet, mapItems)
import "lib/math" (round)

// Group by hour
fun extractHour(timestamp: String) -> String {
    match regexCapture(":([0-9]{2}):", timestamp) {
        Some(groups) -> groups[1]
        Zero -> "unknown"
    }
}

// Print histogram
fun printHistogram(counts, maxCount: Int) {
    scale = 50.0 / intToFloat(maxCount)
    
    print("\nRequests by Hour:\n")
    for hour in range(0, 24) {
        h = stringPadLeft(show(hour), 2, '0')
        count = match mapGet(counts, h) { Some(n) -> n, Zero -> 0 }
        bar = stringRepeat("#", round(intToFloat(count) * scale))
        print(h ++ ":00 | " ++ bar ++ " " ++ show(count))
    }
}
// ...
```

---

## Anomaly detection

```rust
fun detectSpikes(entries, windowMinutes: Int, threshold: Int) -> List<String> {
    // Simplified version - count by minutes
    // In real code, need sliding window
    
    print("Analyzing " ++ show(len(entries)) ++ " entries...")
    print("Window: " ++ show(windowMinutes) ++ " minutes, threshold: " ++ show(threshold))
    []  // return empty list for example
}
// ...
    |> filter(fun(minute, count) -> count > threshold)
    |> map(fun(minute, count) -> 
        "Spike at " ++ minute ++ ": " ++ toString(count) ++ " requests"
    )
}

fun detect404Paths(entries, minCount: Int) {
    entries
    |> filter(fun(e) -> e.status == 404)
    |> map(fun(e) -> e.path)
}
// ...
```

---

## Export to CSV

```rust
import "lib/io" (fileWrite)
import "lib/list" (map)
import "lib/string" (stringJoin)

type LogEntry = { ip: String, timestamp: String, method: String, path: String, status: Int, size: Int }

fun toCSV(entries: List<LogEntry>) -> String {
    header = "ip,timestamp,method,path,status,size"
    rows = map(fun(e) -> e.ip ++ "," ++ e.timestamp ++ "," ++ e.method ++ "," ++ e.path ++ "," ++ show(e.status) ++ "," ++ show(e.size), entries)
    header ++ "\n" ++ stringJoin(rows, "\n")
}

// Usage example
entries = [{ ip: "127.0.0.1", timestamp: "2024-01-01", method: "GET", path: "/", status: 200, size: 1024 }]
match fileWrite("report.csv", toCSV(entries)) {
    Ok(_) -> print("Saved to report.csv")
    Fail e -> print("Error: " ++ e)
}
```

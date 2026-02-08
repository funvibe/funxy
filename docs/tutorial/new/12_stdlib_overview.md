# 12. Standard Library Overview

[← Back to Index](./00_index.md)

## Prelude (Always Available)

**Functions:**
*   `print`, `write`: Output to stdout.
*   `len`, `lenBytes`: Length of collections.
*   `show`: Convert to string.
*   `read`: Parse string to value.
*   `typeOf`, `getType`: Runtime type introspection.
*   `id`, `constant`: Functional helpers.
*   `default`: Default value for a type.
*   `panic`: Abort execution.
*   `debug`, `trace`: Debugging helpers.

**Types:**
*   `Int`, `Float`, `Bool`, `Char`, `String`, `Nil`
*   `BigInt`, `Rational`
*   `List<t>`, `Map<k, v>`, `Range<t>`
*   `Option<t>`, `Result<e, t>`
*   `Bytes`, `Bits`

## Modules

| Module | Description | Key Functions |
|--------|-------------|---------------|
| `lib/bignum` | BigInt/Rational helpers | `bigIntNew`, `ratNew`, `ratToFloat` |
| `lib/bits` | Bit manipulation | `bitsGet`, `bitsConcat` |
| `lib/bytes` | Byte manipulation | `bytesToHex`, `bytesSlice` |
| `lib/char` | Character utilities | `charIsUpper`, `charToUpper`, `charToLower` |
| `lib/crypto` | Cryptography | `sha256`, `base64` |
| `lib/csv` | CSV parsing | `csvRead`, `csvWrite` |
| `lib/date` | Date/time with timezone | `dateNow`, `dateFormat` |
| `lib/flag` | CLI flags | `flagSet`, `flagParse` |
| `lib/grpc` | gRPC client/server | `grpcConnect`, `grpcInvoke`, `grpcServe` |
| `lib/http` | HTTP Client/Server | `httpGet`, `httpPost`, `httpServe` |
| `lib/io` | File I/O | `fileRead`, `fileWrite`, `readLine` |
| `lib/json` | JSON handling | `jsonEncode`, `jsonDecode` |
| `lib/list` | List operations | `map`, `filter`, `foldl`, `sort`, `head`, `tail`, `insert`, `update` |
| `lib/log` | Logging | `logInfo`, `logError` |
| `lib/map` | Map operations | `mapGet`, `mapPut`, `mapKeys` |
| `lib/math` | Math functions | `sqrt`, `sin`, `cos`, `abs` |
| `lib/path` | File paths | `pathJoin`, `pathBase` |
| `lib/proto` | Protobuf encoding | `protoEncode`, `protoDecode` |
| `lib/rand` | Random numbers | `randomInt`, `randomFloat` |
| `lib/regex` | Regular expressions | `regexMatch`, `regexFind` |
| `lib/sql` | SQLite interface | `sqlOpen`, `sqlQuery`, `sqlExec` |
| `lib/string` | String manipulation | `stringSplit`, `stringJoin`, `stringToUpper` |
| `lib/sys` | System interaction | `sysArgs`, `sysEnv`, `sysExec` |
| `lib/task` | Async tasks | `async`, `await` |
| `lib/test` | Testing framework | `assert`, `testRun` |
| `lib/time` | Time and duration | `timeNow`, `sleep` |
| `lib/tuple` | Tuple helpers | `fst`, `snd`, `tupleSwap` |
| `lib/url` | URL parsing | `urlParse`, `urlJoin` |
| `lib/uuid` | UUIDs | `uuidNew`, `uuidParse` |
| `lib/ws` | WebSockets | `wsConnect`, `wsServe` |

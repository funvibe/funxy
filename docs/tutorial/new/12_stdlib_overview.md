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
*   `runBytecode`: Load and execute bytecode from file.

**Types:**
*   `Int`, `Float`, `Bool`, `Char`, `String`, `Nil`
*   `BigInt`, `Rational`
*   `List<t>`, `Map<k, v>`, `Range<t>`
*   `Option<t>`, `Result<e, t>`
*   `Bytes`, `Bits`

**Traits:**
*   `Equal`, `Order`: Comparison
*   `Numeric`: Arithmetic operations
*   `Show`: String representation
*   `Semigroup`, `Monoid`: Combining values
*   `Functor`, `Applicative`, `Monad`: Functional programming
*   `Iter`: Iteration over collections

## Modules

| Module | Description | Key Functions |
|--------|-------------|---------------|
| `lib/bignum` | Arbitrary precision numbers (BigInt, Rational) | `bigIntNew`, `ratNew`, `ratToFloat` |
| `lib/bits` | Bit sequence manipulation (non-byte-aligned) | `bitsGet`, `bitsConcat`, `bitsExtract` |
| `lib/bytes` | Byte sequence manipulation | `bytesToHex`, `bytesSlice`, `bytesEncodeInt` |
| `lib/char` | Character functions | `charIsUpper`, `charToUpper`, `charToLower` |
| `lib/crypto` | Cryptographic hashing, encoding, and secure random functions | `sha256`, `base64Encode`, `hmacSha256` |
| `lib/csv` | CSV parsing, encoding, and file I/O (optional delimiter, default ',') | `csvRead`, `csvWrite`, `csvParse` |
| `lib/date` | Date and time manipulation with timezone offset | `dateNow`, `dateFormat`, `dateAddDays` |
| `lib/flag` | Command line flag parsing. Supports both -flag=value and -flag value formats. | `flagSet`, `flagParse`, `flagGet` |
| `lib/grpc` | gRPC client and server support | `grpcConnect`, `grpcInvoke`, `grpcServe` |
| `lib/http` | HTTP client and server | `httpGet`, `httpPost`, `httpServe` |
| `lib/io` | File and stream I/O | `fileRead`, `fileWrite`, `readLine` |
| `lib/json` | JSON encoding, decoding, and manipulation | `jsonEncode`, `jsonDecode`, `jsonParse` |
| `lib/list` | List manipulation functions | `map`, `filter`, `foldl`, `sort`, `head`, `tail`, `insert`, `update` |
| `lib/log` | Structured logging with levels, formats, and prefixed loggers | `logInfo`, `logError`, `logWithFields` |
| `lib/mailbox` | Asynchronous actor messaging and queuing | `send`, `receive`, `sendWait`, `receiveWait` |
| `lib/map` | Immutable hash map (HAMT-based) | `mapGet`, `mapPut`, `mapKeys` |
| `lib/math` | Mathematical functions | `sqrt`, `sin`, `cos`, `abs` |
| `lib/path` | File path manipulation (OS-specific) | `pathJoin`, `pathBase`, `pathExt` |
| `lib/proto` | Protocol Buffers serialization | `protoEncode`, `protoDecode` |
| `lib/rand` | Random number generation | `randomInt`, `randomFloat`, `randomChoice` |
| `lib/regex` | Regular expression matching and manipulation | `regexMatch`, `regexFind`, `regexReplace` |
| `lib/rpc` | RPC cross-VM communication | `callWait`, `callWaitGroup` |
| `lib/sql` | SQLite database operations | `sqlOpen`, `sqlQuery`, `sqlExec` |
| `lib/string` | String manipulation functions (String = List<Char>) | `stringSplit`, `stringJoin`, `stringToUpper` |
| `lib/sys` | System interaction (sysArgs, sysEnv, sysExit, sysExec, sysExePath, sysScriptDir) | `sysArgs`, `sysEnv`, `sysExec`, `sysExePath` |
| `lib/task` | Asynchronous computations with Tasks (Futures/Promises) | `async`, `await`, `awaitTimeout` |
| `lib/term` | Terminal UI: colors, styles, prompts, spinners, progress bars, tables. Auto-detects color support, respects $NO_COLOR. | `red`, `bold`, `confirm`, `select`, `table`, `spinnerStart` |
| `lib/test` | Testing framework with assertions and mocking | `assert`, `testRun`, `assertEquals` |
| `lib/time` | Time and timing functions | `timeNow`, `sleep`, `sleepMs` |
| `lib/tuple` | Tuple manipulation functions | `fst`, `snd`, `tupleSwap` |
| `lib/url` | URL parsing, manipulation, and encoding | `urlParse`, `urlJoin`, `urlEncode` |
| `lib/uuid` | UUID generation and manipulation | `uuidNew`, `uuidParse`, `uuidV4` |
| `lib/vmm` | Virtual Machine Manager and state orchestration | `spawnVM`, `stopVM`, `listVMs` |
| `lib/ws` | WebSocket client and server (RFC 6455) | `wsConnect`, `wsSend`, `wsServe` |
| `lib/yaml` | YAML encoding, decoding, and file I/O | `yamlRead`, `yamlWrite`, `yamlDecode` |

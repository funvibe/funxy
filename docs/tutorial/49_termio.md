# Lossless terminal input (`lib/termio`)

`lib/termio` provides managed raw terminal input for Unix TUI applications.
Unlike `lib/term.readKey`, it parses a continuous byte stream and distinguishes
ordinary key presses from pasted text.

## Event model

```rust
type InputEvent = KeyEvent String | TextEvent String
```

- `KeyEvent(name)` represents one keyboard event.
- `TextEvent(text)` represents one bracketed paste. CR, LF, CRLF, tab,
  multiline text, and a trailing newline remain part of `text` and are never
  normalized or interpreted as commands.

Because the variants are tagged, `KeyEvent("space")` cannot be confused with
`TextEvent("space")`.

The key/text distinction depends on the terminal emulator implementing the
bracketed-paste protocol. On terminals without that protocol, pasted bytes are
still parsed losslessly but arrive as ordinary `KeyEvent` values.

### Stable key names

The `KeyEvent` vocabulary is part of the API:

| Input | Value |
|---|---|
| Printable Unicode character | The one-character string; ASCII space is `space` |
| CR or LF | `enter` |
| Tab | `tab` |
| BS or DEL | `backspace` |
| Standalone Escape | `escape` after a 100 ms ambiguity timeout |
| NUL | `ctrl+space` |
| Control bytes 1–26 | `ctrl+a`…`ctrl+z`, except Tab/LF/BS/CR above |
| FS, GS, RS, US | `ctrl+\\`, `ctrl+]`, `ctrl+^`, `ctrl+_` |
| Cursor keys | `up`, `down`, `left`, `right` |
| Navigation keys | `home`, `end`, `insert`, `delete`, `pageup`, `pagedown` |
| Shift+Tab | `shift+tab` |
| Function keys | `f1`…`f12` |
| Escape plus printable ASCII | `alt+` plus the literal character; Space is `alt+space` |

A complete unknown CSI or SS3 sequence is returned verbatim, including its
leading U+001B Escape character. For example, unknown `ESC [ 1 ; 5 A` becomes
U+001B followed by `[1;5A`, not a guessed name such as `ctrl+up`.

## Managed lifecycle

Terminal input is available only inside `withTerminalInput`:

```rust
import "lib/termio" (withTerminalInput, readInputEvent, InputEvent)

fun loop() {
    match readInputEvent(250) {
        Some(KeyEvent("escape")) -> Nil
        Some(KeyEvent(key)) -> {
            print("key: " ++ key)
            loop()
        }
        Some(TextEvent(text)) -> {
            print("paste: " ++ text)
            loop()
        }
        None -> loop()
        _ -> loop()
    }
}

withTerminalInput(loop)
```

The wrapper opens `/dev/tty`, saves its mode, enables raw mode and bracketed
paste, and then invokes the callback. It disables bracketed paste and restores
the saved terminal mode after a normal return, evaluator error, `sysExit`, or a
handled Unix termination signal. A completed session leaves no reader behind,
so a program may leave for another terminal owner and later call
`withTerminalInput` again. Nested or overlapping sessions are rejected.

`readInputEvent()` is non-blocking by default. A positive timeout waits for that
many milliseconds and returns `None` if no event arrives. `None` means only
timeout/no queued event. `/dev/tty` open failures, terminal setup/cleanup
failures, readiness-wait/`read` failures, EOF, and a closed stream are runtime errors
with operation-specific messages.

Correct UTF-8 remains intact even when a character is divided across system
reads. Funxy `String` cannot represent invalid UTF-8 losslessly, so invalid or
truncated input is reported as a runtime error; it is never silently converted
to U+FFFD. While waiting for a missing bracketed-paste end marker, termio keeps
at most 16 MiB. Crossing that limit is a runtime error, not a partial
`TextEvent`. An unterminated escape sequence is limited to 4 KiB.

Raw mode disables terminal-generated signals, so physical Ctrl+C is returned as
`KeyEvent("ctrl+c")`. Externally delivered SIGHUP, SIGINT, SIGQUIT, and SIGTERM
are different: termio restores the terminal, removes its signal handlers, and
re-delivers the same signal with the default disposition. Supervisors therefore
observe the original signal. Signal subscriptions installed before an ordinary
session exit remain installed afterward. `SIGKILL` cannot be intercepted by any
process.

## Combining with `lib/term`

The rendering part of `lib/term` can be used normally:

```rust
import "lib/term" (termClear, cursorTo)
import "lib/termio" (withTerminalInput, readInputEvent, InputEvent)
```

Do not call `termRaw` or `readKey` while `withTerminalInput` is active. Also do
not run `readLine`, `prompt`, `select`, `password`, or another `/dev/tty` reader
at the same time: terminal input must have exactly one owner.

## Platform scope

Functional support is limited to Linux, macOS, FreeBSD, and OpenBSD. Other
platforms return a clear unsupported-platform error when the session starts.

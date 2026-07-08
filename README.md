# xlog

[![Test, Lint](https://github.com/digineo/xlog/actions/workflows/test.yml/badge.svg)](https://github.com/digineo/xlog/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/digineo/xlog.svg)](https://pkg.go.dev/github.com/digineo/xlog)

`xlog` is a thin wrapper around the standard library's [`log/slog`][slog]
package. It trades a little of slog's flexibility for a small, opinionated
API that is quick to wire up in applications and easy to stub out in tests.

It was originally developed as part of [digineo/texd][texd] and extracted
into its own module for reuse.

[slog]: https://pkg.go.dev/log/slog
[texd]: https://github.com/digineo/texd

## Features

- A minimal `Logger` interface with `Debug`/`Info`/`Warn`/`Error`/`Fatal`
  and `With` methods.
- Functional options for configuration (`New(...Option)`).
- Text output (colorized via [slogor][slogor], TTY-aware) or JSON output.
- Convenience attribute constructors (`String`, `Int`, `Error`, …) so you
  don't need to import `log/slog` alongside `xlog`.
- Test helpers: a no-op `Discard` logger and a `MockClock` option for
  deterministic timestamps.

[slogor]: https://gitlab.com/greyxor/slogor

## Installation

```sh
go get github.com/digineo/xlog
```

Requires Go 1.26 or later.

## Usage

```go
package main

import (
	"log/slog"

	"github.com/digineo/xlog"
)

func main() {
	log, err := xlog.New(
		xlog.AsText(),
		xlog.Color(),
		xlog.Leveled(slog.LevelDebug),
	)
	if err != nil {
		panic(err)
	}

	log.Info("service started", xlog.String("addr", ":8080"))

	if err := run(); err != nil {
		log.Error("service failed", xlog.Error(err))
	}
}
```

### Options

| Option                | Effect                                                       |
| --------------------- | ------------------------------------------------------------ |
| `WriteTo(w)`          | Write log output to `w` (default: `os.Stdout`).              |
| `Leveled(l)`          | Set the minimum log level from a `slog.Level`.               |
| `LeveledString(s)`    | Set the level from a string (`"debug"`, `"info"`, …).        |
| `AsText()`            | Emit human-readable `key=value` lines (via slogor).          |
| `AsJSON()`            | Emit one JSON object per log record.                         |
| `Color()`             | Colorize text output when writing to a TTY.                  |
| `WithSource()`        | Include source file and line in each record.                 |
| `Discard()`           | Mute the logger entirely.                                    |
| `MockClock(t)`        | Use a fixed timestamp — handy in tests.                      |

Exactly one of `AsText()`, `AsJSON()` or `Discard()` selects the handler;
`New` returns an error if none is given.

### Testing

Use `NewDiscard()` to obtain a logger that swallows everything, or combine
`MockClock` with `WriteTo` to assert on deterministic output:

```go
var buf bytes.Buffer
log, _ := xlog.New(
	xlog.AsText(),
	xlog.WriteTo(&buf),
	xlog.MockClock(time.Unix(1650000000, 0).UTC()),
)
log.Info("hello", xlog.String("who", "world"))
// buf now holds a line with a fixed timestamp
```

## License

MIT — see [LICENSE](LICENSE).

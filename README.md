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
- Text or JSON output using the standard library's handlers — the core
  module has **no third-party dependencies**.
- A pluggable handler backend (`UseHandler`): opt into a richer handler
  without the core module depending on it. A colorized text backend is
  shipped as the optional [`xlog/slogor`](./slogor) sub-module.
- Convenience attribute constructors (`String`, `Int`, `Error`, …) so you
  don't need to import `log/slog` alongside `xlog`.
- Test helpers: a no-op `Discard` logger and a `MockClock` option for
  deterministic timestamps.

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
| `AsText()`            | Emit human-readable `key=value` lines (stdlib text handler). |
| `AsJSON()`            | Emit one JSON object per log record.                         |
| `WithSource()`        | Include source file and line in each record.                 |
| `Discard()`           | Mute the logger entirely.                                    |
| `MockClock(t)`        | Use a fixed timestamp — handy in tests.                      |
| `UseHandler(kind, f)` | Install a custom `slog.Handler` backend (see below).         |

Exactly one handler option (`AsText()`, `AsJSON()`, `Discard()`, or a
custom `UseHandler`) selects the output format; `New` returns an error if
none is given, or if two options request different kinds — e.g. passing
both `AsText()` and `AsJSON()` is a conflict.

### Colorized output (`xlog/slogor`)

The core module intentionally ships without a colorized handler so it stays
dependency-free. Colorized, human-friendly text output lives in the
optional `xlog/slogor` sub-module, backed by [slogor][slogor]:

```go
import (
	"log/slog"

	"github.com/digineo/xlog"
	"github.com/digineo/xlog/slogor"
)

log, err := xlog.New(
	slogor.Colorized(),
	xlog.Leveled(slog.LevelInfo),
)
```

`slogor.Colorized()` produces text output, so it implies `xlog.AsText()`
and conflicts with `xlog.AsJSON()`.

Colors are emitted only when the output is a terminal. Pass
`slogor.ForceColor()` to colorize regardless:

```go
log, err := xlog.New(slogor.Colorized(slogor.ForceColor()))
```

Install it with:

```sh
go get github.com/digineo/xlog/slogor
```

Because it is a separate module, projects that don't use it never pull in
the slogor or go-isatty dependencies. The same `UseHandler` extension point
lets you wire up any other `slog.Handler` backend.

[slogor]: https://gitlab.com/greyxor/slogor

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

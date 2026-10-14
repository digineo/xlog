# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

All modules share version numbers. Releases of the sub-modules are
tagged with their directory as prefix, e.g. `slogor/vX.Y.Z`.

## [Unreleased]

### Added

- `Tee` option to send log records to additional handlers, `OnExit`
  option to run functions before `Fatal` exits the program, and
  `Options` to combine several options into one.
- graylog: new sub-module to forward log records to a Graylog GELF input
  via UDP, TCP or TLS ([#10](https://github.com/digineo/xlog/issues/10)).

### Changed

- slogor: require `golang.org/x/sys` v0.49.0.

### Fixed

- `MockClock` is no longer lost on loggers derived via `Logger.With()`
  ([#9](https://github.com/digineo/xlog/issues/9)).

## [1.1.0] - 2026-08-11

### Added

- `LevelEnabler` interface and `IsEnabled(Logger, slog.Level)` helper to
  guard expensive computations. Loggers which cannot answer are assumed
  to be enabled.
- slogor: `ForceColor()` option to emit colors even when the output is
  not a terminal.

### Changed

- slogor: require `gitlab.com/greyxor/slogor` v1.7.0 and
  `github.com/mattn/go-isatty` v0.0.24.

### Fixed

- slogor: the module can be installed. v1.0.0 required the unresolvable
  `github.com/digineo/xlog` v0.0.0.

## [1.0.0] - 2026-07-08

Initial release, extracted from [digineo/texd](https://github.com/digineo/texd).

### Added

- `Logger` interface with `Debug`, `Info`, `Warn`, `Error`, `Fatal` and
  `With` methods.
- `New(...Option)` with the options `WriteTo`, `Leveled`, `LeveledString`,
  `AsText`, `AsJSON`, `WithSource`, `Discard` and `MockClock`.
- `UseHandler` extension point for custom `slog.Handler` backends.
- Attribute constructors such as `String`, `Int` and `Error`, so callers
  need not import `log/slog`.
- `NewDiscard()` no-op logger and `ParseLevel` helper.
- slogor: optional sub-module with a colorized text backend,
  `slogor.Colorized()`.

[Unreleased]: https://github.com/digineo/xlog/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/digineo/xlog/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/digineo/xlog/releases/tag/v1.0.0

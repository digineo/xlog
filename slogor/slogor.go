// Package slogor provides a colorized, human-friendly text handler for
// [github.com/digineo/xlog], backed by [gitlab.com/greyxor/slogor].
//
// It lives in its own module so that the slogor (and go-isatty)
// dependencies are only pulled in by consumers who actually want this
// handler; the core xlog module stays dependency-free.
//
// Usage:
//
//	log, err := xlog.New(slogor.Colorized(), xlog.Leveled(slog.LevelInfo))
package slogor

import (
	"log/slog"
	"os"

	"github.com/digineo/xlog"
	"github.com/mattn/go-isatty"
	"gitlab.com/greyxor/slogor"
)

// An Option configures the slogor-backed handler. Options are passed to
// [Colorized]; they are distinct from [xlog.Option].
type Option func(*settings)

type settings struct {
	forceColor bool
}

// ForceColor makes the handler emit colors even when the output is not a
// terminal, e.g. when logs are piped into a tool which understands ANSI
// escape sequences.
func ForceColor() Option {
	return func(s *settings) { s.forceColor = true }
}

// Colorized returns an [xlog.Option] that installs a slogor-backed text
// handler. Colors are emitted only when the output (see [xlog.WriteTo])
// is a terminal, unless [ForceColor] is given.
//
// Because it produces text output, Colorized implies [xlog.AsText] and
// therefore conflicts with [xlog.AsJSON]: passing both to [xlog.New]
// makes New return an error.
func Colorized(opt ...Option) xlog.Option {
	var s settings
	for _, o := range opt {
		o(&s)
	}

	return xlog.UseHandler(xlog.HandlerText, func(cfg *xlog.HandlerConfig) slog.Handler {
		level := slog.LevelInfo
		if cfg.Level != nil {
			level = cfg.Level.Level()
		}

		opts := []slogor.OptionFn{
			slogor.SetLevel(level),
			slogor.SetTimeFormat("[15:04:05.000]"),
		}
		if cfg.Source {
			opts = append(opts, slogor.ShowSource())
		}
		f, isFile := cfg.Output.(*os.File)
		if !s.forceColor && (!isFile || !isatty.IsTerminal(f.Fd())) {
			opts = append(opts, slogor.DisableColor())
		}
		return slogor.NewHandler(cfg.Output, opts...)
	})
}

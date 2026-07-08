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

// Colorized returns an [xlog.Option] that installs a slogor-backed text
// handler. Colors are emitted only when the output (see [xlog.WriteTo])
// is a terminal.
//
// Because it produces text output, Colorized implies [xlog.AsText] and
// therefore conflicts with [xlog.AsJSON]: passing both to [xlog.New]
// makes New return an error.
func Colorized() xlog.Option {
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
		if f, isFile := cfg.Output.(*os.File); !isFile || !isatty.IsTerminal(f.Fd()) {
			opts = append(opts, slogor.DisableColor())
		}
		return slogor.NewHandler(cfg.Output, opts...)
	})
}

package xlog

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/digineo/xlog/internal"
)

// An Option represents a functional configuration option. These are
// used to configure new logger instances.
type Option func(*options) error

type options struct {
	output io.Writer
	clock  internal.Clock
	level  slog.Leveler
	source bool

	handlerKind  HandlerKind
	buildHandler func(o *options) slog.Handler
}

// HandlerKind classifies the output format a handler produces. It is
// used to detect conflicting handler options: requesting two different
// kinds (e.g. both text and JSON output) makes New return an error,
// while options of the same kind override each other (last one wins).
type HandlerKind uint8

const (
	// HandlerText denotes a human-readable text handler.
	HandlerText HandlerKind = iota + 1
	// HandlerJSON denotes a machine-readable JSON handler.
	HandlerJSON
	// HandlerDiscard denotes a handler that mutes all output.
	HandlerDiscard
)

// String implements [fmt.Stringer].
func (k HandlerKind) String() string {
	switch k {
	case HandlerText:
		return "text"
	case HandlerJSON:
		return "json"
	case HandlerDiscard:
		return "discard"
	default:
		return fmt.Sprintf("HandlerKind(%d)", uint8(k))
	}
}

// HandlerConfig carries the resolved base configuration to a
// [HandlerFactory]. It gives handler provider packages (such as
// xlog/slogor) access to the settings shared by all handlers, without
// exposing xlog's internal option state.
type HandlerConfig struct {
	// Output is the writer log records should be written to.
	Output io.Writer
	// Level is the minimum level to log. It is never nil.
	Level slog.Leveler
	// Source reports whether source positions should be included.
	Source bool
}

// A HandlerFactory constructs a [slog.Handler] from the resolved
// configuration.
type HandlerFactory func(*HandlerConfig) slog.Handler

func (o *options) setHandler(kind HandlerKind, fn func(*options) slog.Handler) error {
	if o.handlerKind != 0 && o.handlerKind != kind {
		return fmt.Errorf("xlog: conflicting handler options: %s and %s", o.handlerKind, kind)
	}
	o.handlerKind = kind
	o.buildHandler = fn
	return nil
}

// UseHandler installs a handler produced by factory. The kind classifies
// the handler for conflict detection (see [HandlerKind]).
//
// It is the extension point used by handler provider packages such as
// xlog/slogor, so that alternative backends can be plugged in without
// the core module depending on them.
func UseHandler(kind HandlerKind, factory HandlerFactory) Option {
	return func(o *options) error {
		if factory == nil {
			return errors.New("xlog: nil handler factory")
		}
		return o.setHandler(kind, func(o *options) slog.Handler {
			return factory(&HandlerConfig{
				Output: o.output,
				Level:  o.level,
				Source: o.source,
			})
		})
	}
}

// Leveled sets the log level.
func Leveled(l slog.Level) Option {
	return func(o *options) error {
		o.level = l
		return nil
	}
}

// LeveledString interprets s (see ParseLevel) and sets the log level.
// If s is unknown, the error will be revealed with New().
func LeveledString(s string) Option {
	return func(o *options) error {
		l, err := ParseLevel(s)
		if err != nil {
			return err
		}
		o.level = l
		return nil
	}
}

var ErrNilWriter = errors.New("invalid writer: nil")

// WriteTo sets the output.
func WriteTo(w io.Writer) Option {
	return func(o *options) error {
		if w == nil {
			return ErrNilWriter
		}

		o.output = w
		return nil
	}
}

// MockClock sets up a canned timestamp.
func MockClock(t time.Time) Option {
	return func(o *options) error {
		o.clock = internal.MockClock(t)
		return nil
	}
}

// WithSource enables source code positions in log messages.
func WithSource() Option {
	return func(o *options) error {
		o.source = true
		return nil
	}
}

// AsJSON configures a JSONHandler, i.e. log messages will be printed
// as JSON string.
func AsJSON() Option {
	return func(o *options) error {
		return o.setHandler(HandlerJSON, func(o *options) slog.Handler {
			return slog.NewJSONHandler(o.output, &slog.HandlerOptions{
				AddSource: o.source,
				Level:     o.level,
			})
		})
	}
}

// AsText configures a TextHandler, i.e. the message output is a simple
// list of key=value pairs with minimal quoting. It uses the handler
// from the standard library and therefore has no external dependencies.
//
// For colorized, human-friendly text output, use the xlog/slogor
// sub-module, whose Colorized option is a drop-in replacement.
func AsText() Option {
	return func(o *options) error {
		return o.setHandler(HandlerText, func(o *options) slog.Handler {
			return slog.NewTextHandler(o.output, &slog.HandlerOptions{
				AddSource: o.source,
				Level:     o.level,
			})
		})
	}
}

// Discard mutes the logger. See also NewDiscard() for a simpler
// constructor.
func Discard() Option {
	return func(o *options) error {
		return o.setHandler(HandlerDiscard, func(o *options) slog.Handler {
			// the discard logger doesn't have any options
			return &discard{}
		})
	}
}

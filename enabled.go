package xlog

import (
	"context"
	"log/slog"
)

// A LevelEnabler reports whether it emits records at a given level.
// Loggers may implement this in addition to [Logger]; use [IsEnabled]
// instead of type-asserting yourself.
type LevelEnabler interface {
	Enabled(level slog.Level) bool
}

var _ LevelEnabler = (*logger)(nil)

// Enabled reports whether the logger emits records at the given level.
func (log *logger) Enabled(level slog.Level) bool {
	return log.l.Enabled(context.Background(), level)
}

// IsEnabled reports whether log emits records at the given level. It
// allows guarding expensive computations:
//
//	if xlog.IsEnabled(log, slog.LevelDebug) {
//		log.Debug("expensive", expensiveAttrs()...)
//	}
//
// Loggers which cannot answer the question are assumed to be enabled.
func IsEnabled(log Logger, level slog.Level) bool {
	switch l := log.(type) {
	case LevelEnabler:
		return l.Enabled(level)
	case slog.Handler: // the discard logger is its own handler
		return l.Enabled(context.Background(), level)
	}
	return true
}

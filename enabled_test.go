package xlog

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type plainLogger struct{ Logger }

func TestIsEnabled(t *testing.T) {
	log, err := New(AsText(), WriteTo(io.Discard), Leveled(slog.LevelWarn))
	require.NoError(t, err)

	assert.False(t, IsEnabled(log, slog.LevelInfo))
	assert.True(t, IsEnabled(log, slog.LevelError))

	// discard answers via its slog.Handler
	assert.False(t, IsEnabled(NewDiscard(), slog.LevelError))

	// unknown implementations are assumed to be enabled
	assert.True(t, IsEnabled(plainLogger{}, slog.LevelDebug))
}

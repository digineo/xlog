package slogor_test

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/digineo/xlog"
	"github.com/digineo/xlog/slogor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestColorized(t *testing.T) {
	var buf bytes.Buffer
	log, err := xlog.New(
		slogor.Colorized(),
		xlog.WriteTo(&buf),
		xlog.Leveled(slog.LevelInfo),
	)
	require.NoError(t, err)
	require.NotNil(t, log)

	log.Info("hello", xlog.String("who", "world"))

	out := buf.String()
	assert.Contains(t, out, "INFO")
	assert.Contains(t, out, "hello")
	assert.Contains(t, out, "who=world")
}

func TestColorized_impliesText(t *testing.T) {
	// Colorized is a text handler, so combining it with AsText is
	// redundant rather than a conflict.
	_, err := xlog.New(
		xlog.AsText(),
		slogor.Colorized(),
		xlog.WriteTo(&bytes.Buffer{}),
	)
	require.NoError(t, err)
}

func TestColorized_conflictsWithJSON(t *testing.T) {
	_, err := xlog.New(
		slogor.Colorized(),
		xlog.AsJSON(),
		xlog.WriteTo(&bytes.Buffer{}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicting handler options")
}

func TestColorized_levelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log, err := xlog.New(
		slogor.Colorized(),
		xlog.WriteTo(&buf),
		xlog.Leveled(slog.LevelWarn),
	)
	require.NoError(t, err)

	log.Info("should not appear")
	log.Warn("should appear")

	out := buf.String()
	assert.NotContains(t, out, "should not appear")
	assert.Contains(t, out, "should appear")
}

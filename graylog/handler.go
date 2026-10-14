package graylog

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"math"
	"runtime"
	"strings"
	"time"
)

// handler converts log records into GELF 1.1 messages. GELF only allows
// flat string or number fields, so attrs in groups are flattened into
// "_group_key" fields.
type handler struct {
	fwd    *Forwarder
	level  slog.Leveler
	source bool
	prefix string         // current group prefix
	fields map[string]any // fields from WithAttrs
}

var _ slog.Handler = (*handler)(nil)

func (h *handler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level.Level()
}

func (h *handler) Handle(_ context.Context, r slog.Record) error {
	msg := make(map[string]any, len(h.fields)+r.NumAttrs()+7) // 7 standard fields
	maps.Copy(msg, h.fields)
	r.Attrs(func(a slog.Attr) bool {
		addField(msg, h.prefix, a)
		return true
	})

	msg["version"] = "1.1"
	msg["host"] = h.fwd.host
	msg["short_message"] = r.Message
	msg["level"] = syslogLevel(r.Level)
	if !r.Time.IsZero() {
		msg["timestamp"] = float64(r.Time.UnixMicro()) / 1e6
	}
	if h.source && r.PC != 0 {
		f, _ := runtime.CallersFrames([]uintptr{r.PC}).Next()
		msg["_file"] = f.File
		msg["_line"] = f.Line
	}

	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	h.fwd.enqueue(b)
	return nil
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := *h
	h2.fields = make(map[string]any, len(h.fields)+len(attrs))
	maps.Copy(h2.fields, h.fields)
	for _, a := range attrs {
		addField(h2.fields, h.prefix, a)
	}
	return &h2
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := *h
	h2.prefix += name + "_"
	return &h2
}

func addField(m map[string]any, prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			prefix += a.Key + "_"
		}
		for _, ga := range a.Value.Group() {
			addField(m, prefix, ga)
		}
		return
	}
	m[fieldName(prefix+a.Key)] = fieldValue(a.Value)
}

// fieldName turns key into a valid GELF additional field name.
func fieldName(key string) string {
	name := "_" + strings.Map(func(r rune) rune {
		if r == '_' || r == '.' || r == '-' ||
			'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9' {
			return r
		}
		return '_'
	}, key)
	if name == "_id" { // reserved by Graylog
		return "__id"
	}
	return name
}

func fieldValue(v slog.Value) any {
	switch v.Kind() {
	case slog.KindInt64:
		return v.Int64()
	case slog.KindUint64:
		return v.Uint64()
	case slog.KindFloat64:
		if f := v.Float64(); !math.IsNaN(f) && !math.IsInf(f, 0) {
			return f
		}
	case slog.KindDuration:
		return v.Duration().Nanoseconds() // like slog.JSONHandler
	case slog.KindTime:
		return v.Time().Format(time.RFC3339Nano)
	}
	return v.String()
}

// syslogLevel maps l to the syslog severity Graylog expects.
func syslogLevel(l slog.Level) int {
	switch {
	case l >= slog.LevelError:
		return 3 // syslog "error"
	case l >= slog.LevelWarn:
		return 4 // syslog "warning"
	case l >= slog.LevelInfo:
		return 6 // syslog "informational"
	default:
		return 7 // syslog "debug"
	}
}

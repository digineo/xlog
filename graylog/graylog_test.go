package graylog

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/digineo/xlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_invalidAddress(t *testing.T) {
	for _, addr := range []string{
		"%",
		"graylog.example.com:12201",
		"http://graylog.example.com:12201",
		"udp://graylog.example.com",
		"udp://:12201",
	} {
		_, err := New(addr)
		assert.Error(t, err, addr)
	}
}

func TestForward_udp(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = pc.Close() })

	gl, err := New("udp://" + pc.LocalAddr().String())
	require.NoError(t, err)

	var local bytes.Buffer
	log, err := xlog.New(
		xlog.AsText(),
		xlog.WriteTo(&local),
		xlog.MockClock(time.Unix(1650000000, 0).UTC()),
		gl.Forward(slog.String("component", "test")),
	)
	require.NoError(t, err)

	log.With(xlog.String("k", "v")).Warn("hello", xlog.Int("n", 1), xlog.Group("g", xlog.String("a", "b")))
	require.NoError(t, gl.Close())

	buf := make([]byte, chunkSize)
	require.NoError(t, pc.SetReadDeadline(time.Now().Add(time.Second)))
	n, _, err := pc.ReadFrom(buf)
	require.NoError(t, err)
	zr, err := gzip.NewReader(bytes.NewReader(buf[:n]))
	require.NoError(t, err)
	var msg map[string]any
	require.NoError(t, json.NewDecoder(zr).Decode(&msg))

	host, err := os.Hostname()
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"version":       "1.1",
		"host":          host,
		"short_message": "hello",
		"level":         4.0,
		"timestamp":     1650000000.0,
		"_component":    "test",
		"_k":            "v",
		"_n":            1.0,
		"_g_a":          "b",
	}, msg)

	assert.Contains(t, local.String(), "msg=hello")
	assert.NotContains(t, local.String(), "component", "static attrs are sent to Graylog only")
	assert.Zero(t, gl.Dropped())
}

func TestForward_stream(t *testing.T) {
	// borrow a certificate valid for 127.0.0.1
	srv := httptest.NewTLSServer(nil)
	cert := srv.TLS.Certificates[0]
	roots := x509.NewCertPool()
	roots.AddCert(srv.Certificate())
	srv.Close()

	for _, tt := range []struct {
		scheme string
		listen func() (net.Listener, error)
		opt    []Option
	}{
		{
			scheme: "tcp",
			listen: func() (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") },
		}, {
			scheme: "tls",
			listen: func() (net.Listener, error) {
				return tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
			},
			opt: []Option{TLSConfig(&tls.Config{RootCAs: roots})},
		},
	} {
		t.Run(tt.scheme, func(t *testing.T) {
			ln, err := tt.listen()
			require.NoError(t, err)
			t.Cleanup(func() { _ = ln.Close() })

			received := make(chan []byte, 1)
			go func() {
				conn, err := ln.Accept()
				if !assert.NoError(t, err) {
					received <- nil
					return
				}
				b, err := io.ReadAll(conn)
				assert.NoError(t, err)
				received <- b
			}()

			gl, err := New(tt.scheme+"://"+ln.Addr().String(), tt.opt...)
			require.NoError(t, err)
			log, err := xlog.New(xlog.AsText(), xlog.WriteTo(io.Discard), gl.Forward())
			require.NoError(t, err)

			for i := range 100 {
				log.Info("msg", xlog.Int("i", i))
			}
			require.NoError(t, gl.Close()) // flushes and closes the connection

			frames := bytes.Split(<-received, []byte{0})
			require.Len(t, frames, 101, "100 null-terminated messages")
			for i, b := range frames[:100] {
				var msg map[string]any
				require.NoError(t, json.Unmarshal(b, &msg))
				assert.EqualValues(t, i, msg["_i"])
			}
			assert.Zero(t, gl.Dropped())
		})
	}
}

func TestForwarder_dropped(t *testing.T) {
	started := make(chan struct{}, 1)
	block := make(chan struct{})
	f := newForwarder("h", time.Second, func() (net.Conn, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-block
		return nil, errors.New("unreachable")
	}, tcpFrames)

	// the first message blocks the sender, the next ones fill the queue
	f.enqueue([]byte("{}"))
	<-started
	for range queueSize + 1 {
		f.enqueue([]byte("{}"))
	}
	assert.EqualValues(t, 1, f.Dropped(), "queue full")

	close(block)
	require.NoError(t, f.Close())
	assert.EqualValues(t, queueSize+2, f.Dropped(), "send failed")

	f.enqueue([]byte("{}"))
	assert.EqualValues(t, queueSize+3, f.Dropped(), "closed")
}

func TestForwarder_closeTimeout(t *testing.T) {
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	f := newForwarder("h", 10*time.Millisecond, func() (net.Conn, error) {
		<-block
		return nil, errors.New("unreachable")
	}, tcpFrames)

	f.enqueue([]byte("{}"))
	assert.EqualError(t, f.Close(), "graylog: flush timed out")
}

func TestForwarder_reconnect(t *testing.T) {
	broken, peer := net.Pipe()
	require.NoError(t, peer.Close())
	conn, srv := net.Pipe()
	conns := []net.Conn{broken, conn}

	f := newForwarder("h", time.Second, func() (net.Conn, error) {
		c := conns[0]
		conns = conns[1:]
		return c, nil
	}, tcpFrames)

	received := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(srv)
		received <- b
	}()

	f.enqueue([]byte("lost"))
	f.enqueue([]byte("sent"))
	require.NoError(t, f.Close())
	assert.Equal(t, "sent\x00", string(<-received))
	assert.EqualValues(t, 1, f.Dropped())
}

func TestUDPFrames(t *testing.T) {
	small, err := udpFrames([]byte("{}"))
	require.NoError(t, err)
	assert.Len(t, small, 1, "no chunking needed")

	msg := make([]byte, 3*chunkSize)
	_, _ = rand.Read(msg) // incompressible
	frames, err := udpFrames(msg)
	require.NoError(t, err)
	require.Len(t, frames, 4)

	var data []byte
	for i, b := range frames {
		require.LessOrEqual(t, len(b), chunkSize)
		assert.Equal(t, []byte{0x1e, 0x0f}, b[:2], "magic bytes")
		assert.Equal(t, frames[0][2:10], b[2:10], "message ID")
		assert.Equal(t, []byte{byte(i), byte(len(frames))}, b[10:12], "sequence")
		data = append(data, b[chunkHeader:]...)
	}
	zr, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	got, err := io.ReadAll(zr)
	require.NoError(t, err)
	assert.Equal(t, msg, got)

	huge := make([]byte, maxChunks*chunkSize)
	_, _ = rand.Read(huge)
	_, err = udpFrames(huge)
	assert.ErrorIs(t, err, errTooLarge)
}

func TestHandler(t *testing.T) {
	f := &Forwarder{host: "h", queue: make(chan []byte, 1)}
	var h slog.Handler = &handler{fwd: f, level: slog.LevelDebug}
	h = h.WithAttrs([]slog.Attr{slog.String("id", "x")}).
		WithGroup("req").
		WithAttrs([]slog.Attr{slog.String("päth", "/")})

	r := slog.NewRecord(time.Time{}, slog.LevelDebug, "msg", 0)
	r.AddAttrs(
		slog.Bool("ok", true),
		slog.Float64("nan", math.NaN()),
		slog.Duration("d", time.Second),
		slog.Time("t", time.Unix(0, 0).UTC()),
		slog.Group("", slog.String("inline", "x")),
		slog.Group("empty"),
		slog.Attr{},
		xlog.Error(errors.New("boom")),
	)
	require.NoError(t, h.Handle(context.Background(), r))

	var msg map[string]any
	require.NoError(t, json.Unmarshal(<-f.queue, &msg))
	assert.Equal(t, map[string]any{
		"version":       "1.1",
		"host":          "h",
		"short_message": "msg",
		"level":         7.0,
		"__id":          "x",
		"_req_p_th":     "/",
		"_req_ok":       "true",
		"_req_nan":      "NaN",
		"_req_d":        1e9,
		"_req_t":        "1970-01-01T00:00:00Z",
		"_req_inline":   "x",
		"_req_error":    "boom",
	}, msg)
}

func TestHandler_Enabled(t *testing.T) {
	h := &handler{level: slog.LevelWarn}
	assert.False(t, h.Enabled(context.Background(), slog.LevelInfo))
	assert.True(t, h.Enabled(context.Background(), slog.LevelWarn))
}

func TestSyslogLevel(t *testing.T) {
	for l, expected := range map[slog.Level]int{
		slog.LevelDebug - 4: 7,
		slog.LevelDebug:     7,
		slog.LevelInfo:      6,
		slog.LevelWarn - 1:  6,
		slog.LevelWarn:      4,
		slog.LevelError:     3,
		slog.LevelError + 4: 3,
	} {
		assert.Equal(t, expected, syslogLevel(l), l.String())
	}
}

// Package graylog ships log records from [github.com/digineo/xlog] to a
// Graylog GELF input, in addition to the regular log output.
//
// Messages are queued and sent in the background, so log calls never
// block on the network. Use [Forwarder.Forward] to attach a Forwarder to
// a logger:
//
//	gl, err := graylog.New("udp://graylog.example.com:12201")
//	if err != nil {
//		// ...
//	}
//	defer gl.Close()
//
//	log, err := xlog.New(
//		xlog.AsJSON(),
//		gl.Forward(slog.String("component", "myapp")),
//	)
package graylog

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/digineo/xlog"
)

const (
	queueSize = 1024
	ioTimeout = 5 * time.Second

	// chunkSize limits UDP datagrams to fit into common MTUs.
	chunkSize   = 1420
	chunkHeader = 12
	maxChunks   = 128
)

var errTooLarge = errors.New("graylog: message too large")

// An Option configures a [Forwarder].
type Option func(*settings)

type settings struct {
	tlsConfig    *tls.Config
	flushTimeout time.Duration
}

// TLSConfig sets the TLS configuration for "tls://" addresses, e.g. to
// trust a private CA. By default, the system roots are used.
func TLSConfig(cfg *tls.Config) Option {
	return func(s *settings) { s.tlsConfig = cfg }
}

// FlushTimeout limits how long [Forwarder.Close] waits for queued
// messages to be sent. The default is 5s.
func FlushTimeout(d time.Duration) Option {
	return func(s *settings) { s.flushTimeout = d }
}

// A Forwarder sends GELF messages to a Graylog input.
type Forwarder struct {
	host         string
	flushTimeout time.Duration
	dial         func() (net.Conn, error)
	frame        func(msg []byte) ([][]byte, error)

	mu      sync.RWMutex // guards closed and closing the queue
	closed  bool
	queue   chan []byte
	done    chan struct{}
	dropped atomic.Uint64

	conn net.Conn // owned by run
}

// New creates a Forwarder for addr, which has the form
// "udp://host:port", "tcp://host:port" or "tls://host:port". Messages
// sent via UDP and TCP are not encrypted.
//
// The connection is established lazily, and re-established after
// errors, so an unavailable Graylog does not prevent the application
// from starting. Call [Forwarder.Close] to flush queued messages.
func New(addr string, opt ...Option) (*Forwarder, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, fmt.Errorf("graylog: invalid address: %w", err)
	}
	if u.Hostname() == "" || u.Port() == "" {
		return nil, fmt.Errorf("graylog: invalid address %q: missing host or port", addr)
	}

	s := settings{flushTimeout: 5 * time.Second}
	for _, o := range opt {
		o(&s)
	}

	var dial func() (net.Conn, error)
	frame := tcpFrames
	switch u.Scheme {
	case "udp":
		dial = func() (net.Conn, error) { return net.DialTimeout("udp", u.Host, ioTimeout) }
		frame = udpFrames
	case "tcp":
		dial = func() (net.Conn, error) { return net.DialTimeout("tcp", u.Host, ioTimeout) }
	case "tls":
		d := &tls.Dialer{NetDialer: &net.Dialer{Timeout: ioTimeout}, Config: s.tlsConfig}
		dial = func() (net.Conn, error) { return d.Dial("tcp", u.Host) }
	default:
		return nil, fmt.Errorf("graylog: invalid address %q: unsupported scheme", addr)
	}

	host, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("graylog: %w", err)
	}

	return newForwarder(host, s.flushTimeout, dial, frame), nil
}

func newForwarder(host string, flushTimeout time.Duration, dial func() (net.Conn, error), frame func([]byte) ([][]byte, error)) *Forwarder {
	f := &Forwarder{
		host:         host,
		flushTimeout: flushTimeout,
		dial:         dial,
		frame:        frame,
		queue:        make(chan []byte, queueSize),
		done:         make(chan struct{}),
	}
	go f.run()
	return f
}

// Forward returns an [xlog.Option] which sends log records to Graylog in
// addition to the regular output, and flushes queued messages before
// [xlog.Logger.Fatal] exits the program. The attrs are added to every
// message sent to Graylog, but not to the regular output.
func (f *Forwarder) Forward(attrs ...slog.Attr) xlog.Option {
	return xlog.Options(
		xlog.Tee(func(cfg *xlog.HandlerConfig) slog.Handler {
			h := &handler{fwd: f, level: cfg.Level, source: cfg.Source}
			return h.WithAttrs(attrs)
		}),
		// the program exits right after, so there is nobody to report to
		xlog.OnExit(func() { _ = f.Close() }),
	)
}

// Dropped reports the number of messages which could not be sent,
// because the queue was full, sending failed, or the Forwarder was
// already closed.
func (f *Forwarder) Dropped() uint64 {
	return f.dropped.Load()
}

// Close sends the queued messages and closes the connection. It waits at
// most for the flush timeout (see [FlushTimeout]) and returns an error
// if messages are still pending then. Messages logged after Close are
// dropped.
func (f *Forwarder) Close() error {
	f.mu.Lock()
	if !f.closed {
		f.closed = true
		close(f.queue)
	}
	f.mu.Unlock()

	select {
	case <-f.done:
		return nil
	case <-time.After(f.flushTimeout):
		return errors.New("graylog: flush timed out")
	}
}

func (f *Forwarder) enqueue(msg []byte) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.closed {
		f.dropped.Add(1)
		return
	}
	select {
	case f.queue <- msg:
	default:
		f.dropped.Add(1)
	}
}

func (f *Forwarder) run() {
	defer close(f.done)
	for msg := range f.queue {
		if err := f.send(msg); err != nil {
			f.dropped.Add(1)
		}
	}
	if f.conn != nil {
		_ = f.conn.Close() // nothing left to lose
	}
}

func (f *Forwarder) send(msg []byte) error {
	frames, err := f.frame(msg)
	if err != nil {
		return err
	}
	if f.conn == nil {
		if f.conn, err = f.dial(); err != nil {
			return err
		}
	}
	for _, b := range frames {
		if err = f.conn.SetWriteDeadline(time.Now().Add(ioTimeout)); err == nil {
			_, err = f.conn.Write(b)
		}
		if err != nil {
			_ = f.conn.Close() // reconnect with the next message
			f.conn = nil
			return err
		}
	}
	return nil
}

// tcpFrames terminates msg with a null byte, as GELF TCP inputs expect.
func tcpFrames(msg []byte) ([][]byte, error) {
	return [][]byte{append(msg, 0)}, nil
}

// udpFrames compresses msg and splits it into GELF chunks, if needed.
func udpFrames(msg []byte) ([][]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(msg); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	data := buf.Bytes()
	if len(data) <= chunkSize {
		return [][]byte{data}, nil
	}

	const size = chunkSize - chunkHeader
	n := (len(data) + size - 1) / size
	if n > maxChunks {
		return nil, errTooLarge
	}

	id := rand.Uint64()
	frames := make([][]byte, n)
	for i := range n {
		part := data[i*size : min((i+1)*size, len(data))]
		b := make([]byte, 0, chunkHeader+len(part))
		b = append(b, 0x1e, 0x0f)
		b = binary.BigEndian.AppendUint64(b, id)
		b = append(b, byte(i), byte(n))
		frames[i] = append(b, part...)
	}
	return frames, nil
}

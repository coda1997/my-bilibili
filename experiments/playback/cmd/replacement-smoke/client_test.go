package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"
)

type failedConnection struct {
	net.Conn
	err      error
	response io.Reader
	short    bool
	deadline time.Time
}

func (c *failedConnection) SetDeadline(d time.Time) error { c.deadline = d; return nil }
func (c *failedConnection) Read(p []byte) (int, error)    { return c.response.Read(p) }
func (c *failedConnection) Write(p []byte) (int, error) {
	if c.err != nil {
		return 0, c.err
	}
	if c.short {
		return len(p) - 1, nil
	}
	return len(p), nil
}

func TestAutomationWriteFailurePreservesSafeCause(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"broken-pipe", &net.OpError{Op: "write", Net: "unix", Addr: &net.UnixAddr{Name: "private-locator", Net: "unix"}, Err: syscall.EPIPE}},
		{"connection-reset", syscall.ECONNRESET},
		{"closed", net.ErrClosed},
		{"timeout", context.DeadlineExceeded},
		{"short-write", io.ErrShortWrite},
		{"io-error", errors.New("private-header-value")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newClient(&failedConnection{err: tc.err})
			err := c.call(context.Background(), "act", map[string]string{"key": "speed-double"}, nil)
			if err == nil || !strings.Contains(err.Error(), "cause="+tc.name) || !strings.Contains(err.Error(), "method=act") || !strings.Contains(err.Error(), "request=1") {
				t.Fatalf("missing bounded transport diagnosis: %v", err)
			}
			if strings.Contains(err.Error(), "private") {
				t.Fatal("transport diagnostic disclosed private data")
			}
		})
	}
}

func TestAutomationResponseFailureAndShortWrite(t *testing.T) {
	for _, tc := range []struct {
		name string
		conn *failedConnection
	}{
		{"eof", &failedConnection{response: strings.NewReader("")}},
		{"oversized", &failedConnection{response: strings.NewReader(strings.Repeat("x", 8<<20) + "\n")}},
		{"short-write", &failedConnection{short: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			trace := &automationTrace{}
			c := newClient(tc.conn)
			c.trace = trace
			err := c.call(context.Background(), "tree", nil, nil)
			if err == nil || !strings.Contains(err.Error(), "cause="+tc.name) || len(trace.Calls) != 1 || trace.Calls[0].Cause != tc.name {
				t.Fatalf("lost transport failure: %v, trace %+v", err, trace)
			}
		})
	}
}

func TestAutomationTraceBoundedAndSanitized(t *testing.T) {
	var responses bytes.Buffer
	for i := 1; i <= 2050; i++ {
		fmt.Fprintf(&responses, "{\"id\":%d,\"result\":{}}\n", i)
	}
	conn := &failedConnection{response: &responses}
	c := newClient(conn)
	trace := &automationTrace{}
	c.trace = trace
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	end, _ := ctx.Deadline()
	for i := 0; i < 2050; i++ {
		if err := c.call(ctx, "private-method", map[string]string{"header": "private-value"}, nil); err != nil {
			t.Fatal(err)
		}
		if !conn.deadline.Equal(end) {
			t.Fatal("transport extended the enclosing deadline")
		}
	}
	if len(trace.Calls) != 2048 || trace.Dropped != 2 {
		t.Fatalf("unbounded trace: %d calls, %d dropped", len(trace.Calls), trace.Dropped)
	}
	b, err := json.Marshal(trace)
	if err != nil || bytes.Contains(b, []byte("private")) || trace.Calls[0].Method != "unknown" {
		t.Fatal("trace disclosed caller-controlled data")
	}
}

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

type timeoutObservedConn struct {
	net.Conn
	timedOut chan struct{}
	once     sync.Once
}

func (c *timeoutObservedConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		c.once.Do(func() { close(c.timedOut) })
	}
	return n, err
}

// The first post-down tree reply crosses the real parent I/O deadline. A
// partial reply tests that recovery preserves bytes as well as the request ID.
func delayedSliderReply(t *testing.T, partial bool, change func(int, []byte) []byte) (*client, <-chan struct{}) {
	t.Helper()
	a, b := net.Pipe()
	conn := &timeoutObservedConn{Conn: a, timedOut: make(chan struct{})}
	released, done := make(chan struct{}, 1), make(chan struct{})
	t.Cleanup(func() {
		_ = a.Close()
		_ = b.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("delayed reply server did not join")
		}
	})
	go func() {
		defer close(done)
		defer b.Close()
		down, delayed := false, false
		scan := bufio.NewScanner(b)
		for scan.Scan() {
			var request struct {
				ID     int
				Method string
				Params struct{ Events []struct{ Type string } }
			}
			if json.Unmarshal(scan.Bytes(), &request) != nil {
				return
			}
			response := map[string]any{"id": request.ID, "result": map[string]bool{"ok": true}}
			if request.Method == "tree" {
				response["result"] = sliderFrame(0)
			}
			for _, event := range request.Params.Events {
				if event.Type == "pointerDown" {
					down = true
				}
				if event.Type == "pointerUp" {
					released <- struct{}{}
				}
			}
			data, _ := json.Marshal(response)
			data = append(data, 10)
			if down && request.Method == "tree" && !delayed {
				delayed = true
				if change != nil {
					data = change(request.ID, data)
				}
				if partial {
					cut := len(data) / 2
					if _, err := b.Write(data[:cut]); err != nil {
						return
					}
					data = data[cut:]
				}
				select {
				case <-conn.timedOut:
				case <-time.After(4 * time.Second):
					return
				}
			}
			if _, err := b.Write(data); err != nil {
				return
			}
		}
	}()
	return newClient(conn), released
}

func TestSliderReleaseAfterTimedOutCompleteOrPartialReply(t *testing.T) {
	for _, partial := range []bool{false, true} {
		name := "complete"
		if partial {
			name = "partial"
		}
		t.Run(name, func(t *testing.T) {
			c, released := delayedSliderReply(t, partial, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
			defer cancel()
			err := c.slider(ctx, "volume-slider", ".35")
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("original deadline lost: %v", err)
			}
			select {
			case <-released:
			default:
				t.Fatalf("pointerUp not received after timed-out %s tree reply: %v", name, err)
			}
			if err.Error() != context.DeadlineExceeded.Error() {
				t.Fatalf("release failed instead of preserving only the original timeout: %v", err)
			}
		})
	}
}

func TestLateReplyCannotSkipIdentityOrMalformedData(t *testing.T) {
	for name, change := range map[string]func(int, []byte) []byte{
		"wrong-id": func(id int, _ []byte) []byte {
			b, _ := json.Marshal(map[string]any{"id": id + 1, "result": sliderFrame(0)})
			return append(b, 10)
		},
		"malformed": func(int, []byte) []byte { return []byte("private-hostile-response\n") },
		"oversized": func(int, []byte) []byte { return []byte(strings.Repeat(" ", 8<<20) + "\n") },
	} {
		t.Run(name, func(t *testing.T) {
			c, released := delayedSliderReply(t, true, change)
			ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
			defer cancel()
			err := c.slider(ctx, "volume-slider", ".35")
			if !errors.Is(err, context.DeadlineExceeded) || err.Error() == context.DeadlineExceeded.Error() || strings.Contains(err.Error(), "private") {
				t.Fatalf("late protocol failure or original timeout was hidden: %v", err)
			}
			select {
			case <-released:
				t.Fatal("an invalid unacknowledged reply was skipped to issue another RPC")
			default:
			}
		})
	}
}

type timedRequestWrite struct {
	bytes   int
	timeout bool
}

type deadlineWriteConn struct {
	net.Conn
	parent   context.Context
	trees    int
	partial  bool
	trace    chan timedRequestWrite
	deadline time.Time
}

func (c *deadlineWriteConn) SetDeadline(t time.Time) error {
	if err := c.Conn.SetDeadline(t); err != nil {
		return err
	}
	c.deadline = t
	return nil
}

func (c *deadlineWriteConn) Write(data []byte) (int, error) {
	var request struct{ Method string }
	if json.Unmarshal(data, &request) != nil || request.Method != "tree" {
		return c.Conn.Write(data)
	}
	c.trees++
	if c.trees != 2 {
		return c.Conn.Write(data)
	}
	written := 0
	if c.partial {
		n, err := c.Conn.Write(data[:1])
		if err != nil {
			return n, err
		}
		written = n
	}
	<-c.parent.Done()
	// The context and net.Pipe fire separate timers. Reapply the caller's
	// deadline so the pipe observes its expiry before the peer can read.
	if err := c.Conn.SetWriteDeadline(c.deadline); err != nil {
		return written, err
	}
	n, err := c.Conn.Write(data[written:])
	var timeout net.Error
	c.trace <- timedRequestWrite{bytes: written + n, timeout: errors.As(err, &timeout) && timeout.Timeout()}
	return written + n, err
}

func TestSliderReleaseAfterZeroByteTimedOutRequestWrite(t *testing.T) {
	c, released := sliderFrameClient(t, func(int) any { return sliderFrame(0) }, false)
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	trace := make(chan timedRequestWrite, 1)
	conn := &deadlineWriteConn{Conn: c.conn, parent: ctx, trace: trace}
	c.conn = conn
	err := c.slider(ctx, "volume-slider", ".35")
	select {
	case outcome := <-trace:
		if outcome.bytes != 0 || !outcome.timeout {
			t.Fatal("zero-byte timed-out write was not established", outcome)
		}
		t.Log("actual post-down tree write returned zero bytes and an I/O timeout")
	default:
		t.Fatal("intended zero-byte write schedule was not reached")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("original timeout lost: %v", err)
	}
	if conn.trees != 2 {
		t.Fatal("timed-out tree request was retried")
	}
	if frames := requireSliderRelease(t, released); frames != 0 {
		t.Fatal("server received the timed-out request or a retry")
	}
	if err.Error() != context.DeadlineExceeded.Error() {
		t.Fatalf("zero-byte write poisoned cleanup: %v", err)
	}
}

func TestPartialTimedOutRequestCannotReuseUncertainFraming(t *testing.T) {
	c, released := sliderFrameClient(t, func(int) any { return sliderFrame(0) }, false)
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	trace := make(chan timedRequestWrite, 1)
	c.conn = &deadlineWriteConn{Conn: c.conn, parent: ctx, partial: true, trace: trace}
	err := c.slider(ctx, "volume-slider", ".35")
	select {
	case outcome := <-trace:
		if outcome.bytes != 1 || !outcome.timeout {
			t.Fatal("partial timed-out write not established", outcome)
		}
	default:
		t.Fatal("partial write schedule was not reached")
	}
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "response stream unavailable") {
		t.Fatalf("partial framing or original timeout was hidden: %v", err)
	}
	select {
	case <-released:
		t.Fatal("another request was appended to a partial JSON request")
	default:
	}
}

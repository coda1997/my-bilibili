package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"time"

	"my-bilibili/experiments/playback/contract"
)

type client struct {
	conn   net.Conn
	scan   *bufio.Scanner
	id     int
	window int
	trace  *automationTrace
}

// Keep only bounded protocol metadata. Never record request/response bodies,
// raw network errors, socket paths, media locators, or header values.
type automationTrace struct {
	Calls   []automationCall `json:"calls"`
	Dropped int              `json:"dropped"`
}

type automationCall struct {
	ID            int           `json:"id"`
	Method        string        `json:"method"`
	Phase         string        `json:"phase"`
	Cause         string        `json:"cause"`
	Context       string        `json:"context"`
	Budget        time.Duration `json:"budget_ns"`
	Elapsed       time.Duration `json:"elapsed_ns"`
	Written       int           `json:"written_bytes"`
	ResponseBytes int           `json:"response_bytes"`
}

func safeMethod(method string) string {
	switch method {
	case "windows", "tree", "act", "input", "capture":
		return method
	default:
		return "unknown"
	}
}

func transportCause(err error) string {
	var networkError net.Error
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, syscall.EPIPE):
		return "broken-pipe"
	case errors.Is(err, syscall.ECONNRESET):
		return "connection-reset"
	case errors.Is(err, net.ErrClosed):
		return "closed"
	case errors.Is(err, io.EOF):
		return "eof"
	case errors.Is(err, io.ErrShortWrite):
		return "short-write"
	case errors.Is(err, bufio.ErrTooLong):
		return "oversized"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &networkError) && networkError.Timeout():
		return "timeout"
	default:
		return "io-error"
	}
}

func newClient(conn net.Conn) *client {
	s := bufio.NewScanner(conn)
	s.Buffer(make([]byte, 4096), 8<<20)
	return &client{conn: conn, scan: s}
}

func (c *client) call(ctx context.Context, method string, params any, result any) error {
	started := time.Now()
	d := started.Add(3 * time.Second)
	if end, ok := ctx.Deadline(); ok && end.Before(d) {
		d = end
	}
	c.id++
	event := automationCall{ID: c.id, Method: safeMethod(method), Phase: "deadline", Cause: "none", Budget: d.Sub(started)}
	defer func() {
		event.Elapsed = time.Since(started)
		event.Context = transportCause(ctx.Err())
		if c.trace != nil {
			if len(c.trace.Calls) < 2048 {
				c.trace.Calls = append(c.trace.Calls, event)
			} else {
				c.trace.Dropped++
			}
		}
	}()
	failure := func(err error) error {
		event.Cause = transportCause(err)
		return fmt.Errorf("automation %s failed (request=%d method=%s cause=%s context=%s)", event.Phase, event.ID, event.Method, event.Cause, transportCause(ctx.Err()))
	}
	if err := c.conn.SetDeadline(d); err != nil {
		return failure(err)
	}
	event.Phase = "encode"
	b, err := json.Marshal(struct {
		ID     int    `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params,omitempty"`
	}{c.id, method, params})
	if err != nil {
		return failure(err)
	}
	event.Phase = "write"
	event.Written, err = c.conn.Write(append(b, '\n'))
	if err != nil {
		return failure(err)
	}
	if event.Written != len(b)+1 {
		return failure(io.ErrShortWrite)
	}
	event.Phase = "read"
	if !c.scan.Scan() {
		err := c.scan.Err()
		if err == nil {
			err = io.EOF
		}
		return failure(err)
	}
	event.ResponseBytes = len(c.scan.Bytes())
	event.Phase = "decode"
	var response struct {
		ID     int             `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(c.scan.Bytes(), &response) != nil || response.ID != c.id {
		event.Cause = "invalid-response"
		return errors.New("invalid automation response")
	}
	if response.Error != nil {
		event.Cause = "rejected"
		return errors.New("automation rejected " + event.Method)
	}
	if result != nil && json.Unmarshal(response.Result, result) != nil {
		event.Cause = "invalid-result"
		return errors.New("invalid automation result")
	}
	event.Phase = "complete"
	return nil
}

type node struct {
	Key    any      `json:"key"`
	Label  string   `json:"label"`
	Value  string   `json:"value"`
	States []string `json:"states"`
	Bounds struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
		W float64 `json:"width"`
		H float64 `json:"height"`
	} `json:"bounds"`
}

func (c *client) tree(ctx context.Context) ([]node, error) {
	var r struct {
		Nodes []node `json:"nodes"`
	}
	err := c.call(ctx, "tree", map[string]any{"window": c.window}, &r)
	if len(r.Nodes) > 4096 {
		return nil, errors.New("too many automation nodes")
	}
	return r.Nodes, err
}

func (c *client) act(ctx context.Context, key string) error {
	return c.call(ctx, "act", map[string]any{"window": c.window, "key": key, "action": "press"}, nil)
}

func (c *client) openUntil(ctx context.Context, key string, limit time.Duration, predicate func(observed) bool) (observed, error) {
	previous, err := c.snapshot(ctx)
	if err != nil {
		return previous, err
	}
	if err = c.act(ctx, key); err != nil {
		return previous, err
	}
	// An action reply can precede its next UI frame. Never reuse an earlier
	// session's identical ready/loading/error state as replacement evidence.
	return c.until(ctx, limit, func(s observed) bool {
		return s.Generation > previous.Generation && predicate(s)
	})
}

type observed struct {
	contract.Snapshot
	Owned        bool               `json:"owned"`
	ControlError contract.ErrorCode `json:"control_error"`
	WindowHidden bool               `json:"window_hidden"`
}

func (c *client) snapshot(ctx context.Context) (observed, error) {
	nodes, err := c.tree(ctx)
	if err != nil {
		return observed{}, err
	}
	for _, n := range nodes {
		if strings.HasPrefix(n.Label, "snapshot ") {
			var s observed
			if json.Unmarshal([]byte(strings.TrimPrefix(n.Label, "snapshot ")), &s) != nil {
				return s, errors.New("invalid snapshot")
			}
			return s, nil
		}
	}
	return observed{}, errors.New("snapshot element absent")
}

func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (c *client) until(ctx context.Context, limit time.Duration, predicate func(observed) bool) (observed, error) {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	end, _ := ctx.Deadline()
	var s observed
	for {
		if err := ctx.Err(); err != nil {
			return s, err
		}
		if !time.Now().Before(end) {
			return s, context.DeadlineExceeded
		}
		var err error
		s, err = c.snapshot(ctx)
		if err != nil {
			return s, err
		}
		matched := predicate(s)
		// A buffered reply or scheduling delay must not turn a late result into a pass.
		if err := ctx.Err(); err != nil {
			return s, err
		}
		if !time.Now().Before(end) {
			return s, context.DeadlineExceeded
		}
		if matched {
			return s, nil
		}
		if err = wait(ctx, 50*time.Millisecond); err != nil {
			return s, err
		}
	}
}

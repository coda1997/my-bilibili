package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"time"

	"my-bilibili/internal/previewbundle"
)

type client struct {
	conn          net.Conn
	reader        *bufio.Reader
	partial       []byte
	pendingID     int
	pendingMethod string
	pendingResult any
	broken        bool
	clientMode    bool
	id, window    int
	callLimit     time.Duration
	report        *report
	phase         string
	cycle         int
	completion    fullscreenCompletion
}

func newClient(conn net.Conn) *client {
	return &client{conn: conn, reader: bufio.NewReaderSize(conn, 4096), callLimit: 3 * time.Second}
}
func (c *client) call(ctx context.Context, method string, params, result any) (err error) {
	started := time.Now()
	defer func() { c.recordRPC(started, method, params, result, err) }()
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.broken {
		return errors.New("automation response stream unavailable")
	}
	deadline := time.Now().Add(c.callLimit)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	if c.conn.SetDeadline(deadline) != nil {
		return errors.New("automation deadline failed")
	}
	ioFailure := func(stage string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !time.Now().Before(deadline) {
			return context.DeadlineExceeded
		}
		return fmt.Errorf("automation %s %s failed", method, stage)
	}
	// A timed-out read still owns exactly one response. Complete and validate
	// it under this call's existing deadline before writing another request.
	// In particular, pointerUp must not oppose the server's old reply write.
	if c.pendingID != 0 {
		data, err := c.readFrame()
		if err != nil {
			return ioFailure("read")
		}
		if err := c.acceptReply(data); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	c.id++
	data, err := json.Marshal(struct {
		ID     int    `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params,omitempty"`
	}{c.id, method, params})
	if err != nil {
		return errors.New("automation request invalid")
	}
	n, err := c.conn.Write(append(data, '\n'))
	if err != nil || n != len(data)+1 {
		// Zero bytes leave the stream aligned for a distinct cleanup action.
		// A partially/uncertainly sent request cannot be retried or reused.
		if n != 0 {
			c.broken = true
		}
		return ioFailure("write")
	}
	c.pendingID, c.pendingMethod, c.pendingResult = c.id, method, result
	data, err = c.readFrame()
	if err != nil {
		return ioFailure("read")
	}
	if err := c.acceptReply(data); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return nil
}

// ReadSlice preserves bounded framing across a timeout. Scanner instead makes
// an I/O error terminal and can expose a partial last line as a complete token.
func (c *client) readFrame() ([]byte, error) {
	for {
		part, err := c.reader.ReadSlice('\n')
		if len(part) > (8<<20)-len(c.partial) {
			c.broken, c.partial = true, nil
			return nil, errors.New("automation response exceeds bound")
		}
		c.partial = append(c.partial, part...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			var timeout net.Error
			if !errors.As(err, &timeout) || !timeout.Timeout() {
				c.broken, c.partial = true, nil
			}
			return nil, err
		}
		data := c.partial
		c.partial = nil
		return data, nil
	}
}

func (c *client) acceptReply(data []byte) error {
	id, method, result := c.pendingID, c.pendingMethod, c.pendingResult
	c.pendingID, c.pendingMethod, c.pendingResult = 0, "", nil
	var response struct {
		ID     *int            `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if json.Unmarshal(data, &response) != nil || response.ID == nil || *response.ID != id {
		c.broken = true
		return fmt.Errorf("automation %s response invalid", method)
	}
	if len(response.Error) > 0 && string(response.Error) != "null" {
		var remote struct{ Code string }
		if json.Unmarshal(response.Error, &remote) != nil || remote.Code == "" {
			return fmt.Errorf("automation %s response invalid", method)
		}
		category := "remote_error"
		switch remote.Code {
		case "parse_error", "invalid_request", "unknown_method", "no_window", "no_element", "unsupported", "timeout", "failed":
			category = remote.Code
		}
		return fmt.Errorf("automation %s rejected (%s)", method, category)
	}
	if result != nil && (len(response.Result) == 0 || string(response.Result) == "null" || json.Unmarshal(response.Result, result) != nil) {
		return fmt.Errorf("automation %s result invalid", method)
	}
	return nil
}

type rectangle struct {
	X, Y          float64
	Width, Height float64
	complete      bool
}

// Keep field presence for fullscreen observations: a missing origin must not
// become a valid (0,0). Other readers retain their existing validation rules.
func (r *rectangle) UnmarshalJSON(data []byte) error {
	var wire struct{ X, Y, Width, Height *float64 }
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*r = rectangle{}
	if wire.X != nil {
		r.X = *wire.X
	}
	if wire.Y != nil {
		r.Y = *wire.Y
	}
	if wire.Width != nil {
		r.Width = *wire.Width
	}
	if wire.Height != nil {
		r.Height = *wire.Height
	}
	r.complete = wire.X != nil && wire.Y != nil && wire.Width != nil && wire.Height != nil
	return nil
}

type node struct {
	ID     string    `json:"id"`
	Parent string    `json:"parent"`
	Key    any       `json:"key"`
	Label  string    `json:"label"`
	Value  string    `json:"value"`
	Bounds rectangle `json:"bounds"`
	States []string  `json:"states"`
	Now    *float64  `json:"now"`
}
type window struct {
	ID            int
	Title, Kind   string
	ContentBounds rectangle
	Visible       bool
}

type windowResult struct{ Windows []window }
type treeResult struct{ Nodes []node }

func (c *client) identify(ctx context.Context) (window, error) {
	var windows windowResult
	if err := c.call(ctx, "windows", nil, &windows); err != nil {
		return window{}, err
	}
	title := "My Bilibili Preview"
	if c.clientMode {
		title = "My Bilibili"
	}
	if len(windows.Windows) != 1 || windows.Windows[0].ID <= 0 || windows.Windows[0].Title != title || windows.Windows[0].Kind != "content" || !windows.Windows[0].Visible {
		return window{}, errors.New("owned preview window unavailable")
	}
	c.window = windows.Windows[0].ID
	return windows.Windows[0], nil
}
func (c *client) tree(ctx context.Context) ([]node, error) {
	var tree treeResult
	if err := c.call(ctx, "tree", map[string]any{"window": c.window}, &tree); err != nil {
		return nil, err
	}
	if len(tree.Nodes) == 0 || len(tree.Nodes) > 4096 {
		return nil, errors.New("invalid preview tree")
	}
	return tree.Nodes, nil
}
func uniqueNode(nodes []node, key string) (node, error) {
	var found node
	count := 0
	for _, n := range nodes {
		if n.Key == key {
			found = n
			count++
		}
	}
	if count != 1 {
		return node{}, errors.New("required unique preview node unavailable")
	}
	return found, nil
}
func observation(nodes []node) (previewbundle.Observation, error) {
	n, err := uniqueNode(nodes, "e2e-observation")
	if err != nil {
		return previewbundle.Observation{}, err
	}
	return previewbundle.ParseObservation([]byte(n.Label))
}
func (c *client) snapshot(ctx context.Context) (previewbundle.Observation, error) {
	nodes, err := c.tree(ctx)
	if err != nil {
		return previewbundle.Observation{}, err
	}
	return observation(nodes)
}
func (c *client) act(ctx context.Context, key string) error {
	if err := c.call(ctx, "act", map[string]any{"window": c.window, "key": key, "action": "press"}, nil); err != nil {
		return fmt.Errorf("action %q: %w", key, err)
	}
	return nil
}
func (c *client) selectPopup(ctx context.Context, menu, item string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := c.act(ctx, menu); err != nil {
		return err
	}
	for {
		ready, err := c.popupItemReady(ctx, item)
		if err != nil {
			return fmt.Errorf("popup item %q: %w", item, err)
		}
		if ready {
			return c.act(ctx, item)
		}
		if err := wait(ctx, 25*time.Millisecond); err != nil {
			return fmt.Errorf("popup item %q: %w", item, err)
		}
	}
}

func (c *client) popupItemReady(ctx context.Context, key string) (bool, error) {
	// Popup availability needs complete geometry, including explicit zero
	// coordinates. Decode pointers locally so missing fields cannot appear ready.
	type popupNode struct {
		Key    any      `json:"key"`
		States []string `json:"states"`
		Bounds *struct {
			X, Y, Width, Height *float64
		} `json:"bounds"`
	}
	var tree struct{ Nodes []*popupNode }
	if err := c.call(ctx, "tree", map[string]any{"window": c.window}, &tree); err != nil {
		return false, err
	}
	if len(tree.Nodes) == 0 || len(tree.Nodes) > 4096 {
		return false, errors.New("invalid popup tree")
	}
	var target *popupNode
	for _, n := range tree.Nodes {
		if n == nil {
			return false, errors.New("invalid popup node")
		}
		if n.Key == key {
			if target != nil {
				return false, errors.New("duplicate popup item")
			}
			target = n
		}
	}
	if target == nil {
		return false, nil
	}
	b := target.Bounds
	if b == nil || b.X == nil || b.Y == nil || b.Width == nil || b.Height == nil || *b.Width < 0 || *b.Height < 0 {
		return false, errors.New("invalid popup item geometry")
	}
	if *b.Width == 0 || *b.Height == 0 {
		return false, nil
	}
	for _, state := range target.States {
		if state == "disabled" || state == "offscreen" {
			return false, nil
		}
	}
	return true, nil
}
func (c *client) input(ctx context.Context, events ...map[string]any) error {
	return c.call(ctx, "input", map[string]any{"window": c.window, "events": events}, nil)
}
func (c *client) key(ctx context.Context, key string) error {
	return c.input(ctx, map[string]any{"type": "key", "key": key})
}
func (c *client) slider(ctx context.Context, key, value string) (err error) {
	minimum, maximum := 0.0, 1.0
	switch key {
	case "seek-slider":
		if c.clientMode {
			s, err := c.clientSnapshot(ctx)
			if err != nil {
				return err
			}
			maximum = s.Playback.Duration.Seconds()
		} else {
			s, err := c.snapshot(ctx)
			if err != nil {
				return err
			}
			maximum = s.State.Playback.Duration.Seconds()
		}
	case "danmaku-font-size":
		minimum, maximum = 16, 40
	case "danmaku-speed":
		minimum, maximum = .5, 2
	}
	v, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(v) || maximum <= minimum || v < minimum || v > maximum {
		return errors.New("invalid slider target")
	}
	nodes, err := c.tree(ctx)
	if err != nil {
		return err
	}
	n, err := uniqueNode(nodes, key)
	if err != nil {
		return err
	}
	// The pinned mygo slider has a 20pt thumb and 10pt content inset.
	// Drive actual pointer capture and release rather than a command seam.
	if n.Bounds.Width <= 20 || n.Bounds.Height <= 0 {
		return errors.New("slider geometry unavailable")
	}
	x, y := sliderCoordinates(n.Bounds, (v-minimum)/(maximum-minimum))
	if err = c.input(ctx, map[string]any{"type": "pointerMove", "x": x, "y": y}, map[string]any{"type": "pointerDown", "x": x, "y": y}); err != nil {
		return err
	}
	defer func() {
		release, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		err = errors.Join(err, c.input(release, map[string]any{"type": "pointerUp", "x": x, "y": y}))
	}()
	// Visible-window trees can retain the previous frame. Keep capture until a
	// rendered slider value proves that a frame consumed the pointer position.
	hold, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	for {
		nodes, err = c.tree(hold)
		if err != nil {
			return err
		}
		n, err = uniqueNode(nodes, key)
		if err != nil {
			return err
		}
		if n.Now == nil {
			return errors.New("slider observed value unavailable")
		}
		if math.Abs(*n.Now-v) <= 1e-6 {
			return nil
		}
		if err = wait(hold, 25*time.Millisecond); err != nil {
			return err
		}
	}
}

func sliderCoordinates(bounds rectangle, fraction float64) (float64, float64) {
	return bounds.X + 10 + (bounds.Width-20)*fraction, bounds.Y + bounds.Height/2
}
func wait(ctx context.Context, delay time.Duration) error {
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
func (c *client) until(ctx context.Context, limit time.Duration, predicate func(previewbundle.Observation) bool) (previewbundle.Observation, error) {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	end, _ := ctx.Deadline()
	var s previewbundle.Observation
	for {
		var err error
		if err = ctx.Err(); err != nil {
			return s, err
		}
		s, err = c.snapshot(ctx)
		if err != nil {
			return s, err
		}
		if err := ctx.Err(); err != nil {
			return s, err
		}
		if !time.Now().Before(end) {
			return s, context.DeadlineExceeded
		}
		if predicate(s) {
			return s, nil
		}
		if err = wait(ctx, 25*time.Millisecond); err != nil {
			return s, err
		}
	}
}

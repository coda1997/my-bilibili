package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net"
	"strings"
	"time"

	"my-bilibili/experiments/playback/contract"
	"my-bilibili/experiments/playback/mpvprobe"
)

type automationCall struct {
	ID      int                `json:"id"`
	Method  string             `json:"method"`
	Elapsed time.Duration      `json:"elapsed_ns"`
	Result  contract.ErrorCode `json:"result"`
}
type client struct {
	conn       net.Conn
	scan       *bufio.Scanner
	id, window int
	report     *report
}

func newClient(conn net.Conn, r *report) *client {
	scan := bufio.NewScanner(conn)
	scan.Buffer(make([]byte, 4096), 1<<20)
	return &client{conn: conn, scan: scan, report: r}
}
func (c *client) call(ctx context.Context, method string, params, result any) (err error) {
	if method != "windows" && method != "tree" && method != "act" {
		return contract.Invalid
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	started := time.Now()
	deadline := started.Add(3 * time.Second)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	c.id++
	id := c.id
	defer func() {
		if c.report != nil {
			if len(c.report.Automation) < 2048 {
				c.report.Automation = append(c.report.Automation, automationCall{id, method, time.Since(started), safeCode(err)})
			} else {
				c.report.DroppedCalls++
			}
		}
	}()
	if c.conn.SetDeadline(deadline) != nil {
		return contract.Media
	}
	// Wake an in-flight socket read/write on cancellation, and quiesce the
	// callback before the next request can reset the connection's deadline.
	stopped := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = c.conn.SetDeadline(time.Now()); close(stopped) })
	defer func() {
		if !stop() {
			<-stopped
		}
	}()
	transportError := func() error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !time.Now().Before(deadline) {
			return contract.Timeout
		}
		return contract.Media
	}
	b, e := json.Marshal(struct {
		ID     int    `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params,omitempty"`
	}{id, method, params})
	if e != nil {
		return contract.Invalid
	}
	n, e := c.conn.Write(append(b, '\n'))
	if e != nil || n != len(b)+1 {
		return transportError()
	}
	if !c.scan.Scan() {
		return transportError()
	}
	var response struct {
		ID     int             `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if json.Unmarshal(c.scan.Bytes(), &response) != nil || response.ID != id || (len(response.Error) > 0 && string(response.Error) != "null") {
		return contract.Media
	}
	if result != nil && json.Unmarshal(response.Result, result) != nil {
		return contract.Media
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if !time.Now().Before(deadline) {
		return contract.Timeout
	}
	return nil
}

type node struct {
	Key    any    `json:"key"`
	Role   string `json:"role"`
	Label  string `json:"label"`
	Bounds struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
		W float64 `json:"width"`
		H float64 `json:"height"`
	} `json:"bounds"`
}
type observed struct {
	contract.Snapshot
	Owned        bool               `json:"owned"`
	ControlError contract.ErrorCode `json:"control_error"`
	WindowHidden bool               `json:"window_hidden"`
}

func validCode(c contract.ErrorCode) bool {
	switch c {
	case contract.NoError, contract.Invalid, contract.Unsupported, contract.Canceled, contract.Timeout, contract.Media, contract.IsClosed:
		return true
	}
	return false
}
func validSnapshot(s observed) bool {
	switch s.State {
	case contract.Idle, contract.Loading, contract.Ready, contract.Playing, contract.Paused, contract.Ended, contract.Failed, contract.Closed:
	default:
		return false
	}
	return validCode(s.Error) && validCode(s.ControlError) && s.Position >= 0 && s.Duration >= 0 && s.Position <= 24*time.Hour && s.Duration <= 24*time.Hour && !math.IsNaN(s.Rate) && !math.IsInf(s.Rate, 0) && s.Rate >= 0 && s.Rate <= 16 && !math.IsNaN(s.Volume) && !math.IsInf(s.Volume, 0) && s.Volume >= 0 && s.Volume <= 1
}
func (c *client) tree(ctx context.Context) ([]node, error) {
	var r struct {
		Nodes []node `json:"nodes"`
	}
	err := c.call(ctx, "tree", map[string]any{"window": c.window}, &r)
	if err != nil {
		return nil, err
	}
	if len(r.Nodes) > 4096 {
		return nil, contract.Media
	}
	if c.report != nil {
		c.report.observe(r.Nodes)
	}
	return r.Nodes, nil
}
func (c *client) identify(ctx context.Context) error {
	var r struct {
		Windows []struct {
			ID    int    `json:"id"`
			Title string `json:"title"`
		} `json:"windows"`
	}
	if err := c.call(ctx, "windows", nil, &r); err != nil {
		return err
	}
	if len(r.Windows) != 1 || r.Windows[0].ID <= 0 || r.Windows[0].Title != "Authorized Target Playback Probe" {
		return contract.Media
	}
	c.window = r.Windows[0].ID
	return nil
}
func (c *client) act(ctx context.Context, key string) error {
	switch key {
	case "open", "play", "pause", "seek", "volume", "rate", "cancel", "close", "quit":
	default:
		return contract.Invalid
	}
	return c.call(ctx, "act", map[string]any{"window": c.window, "key": key, "action": "press"}, nil)
}
func (c *client) snapshot(ctx context.Context) (observed, error) {
	nodes, err := c.tree(ctx)
	if err != nil {
		return observed{}, err
	}
	return snapshotNodes(nodes)
}
func snapshotNodes(nodes []node) (observed, error) {
	var s observed
	found := false
	for _, n := range nodes {
		if strings.HasPrefix(n.Label, "snapshot ") {
			if found || !completeObject(strings.TrimPrefix(n.Label, "snapshot "), []string{"generation", "state", "position", "duration", "rate", "volume", "error", "owned", "control_error", "window_hidden"}, &s) || !validSnapshot(s) {
				return observed{}, contract.Media
			}
			found = true
		}
	}
	if !found {
		return observed{}, contract.Media
	}
	return s, nil
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
func (c *client) until(ctx context.Context, predicate func(observed) bool) (observed, error) {
	var s observed
	for {
		if ctx.Err() != nil {
			return s, ctx.Err()
		}
		var err error
		s, err = c.snapshot(ctx)
		if err != nil {
			return s, err
		}
		if ctx.Err() != nil {
			return s, ctx.Err()
		}
		if end, ok := ctx.Deadline(); ok && !time.Now().Before(end) {
			return s, context.DeadlineExceeded
		}
		if s.State == contract.Closed && s.Error == contract.Canceled && predicate(s) {
			return s, nil
		}
		if s.State == contract.Failed || s.Error != contract.NoError {
			if s.Error == contract.NoError {
				return s, contract.Media
			}
			return s, s.Error
		}
		if predicate(s) {
			return s, nil
		}
		if err = wait(ctx, 25*time.Millisecond); err != nil {
			return s, err
		}
	}
}
func (c *client) actionUntil(ctx context.Context, key string, limit time.Duration, predicate func(observed) bool) (observed, error) {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	if err := c.act(ctx, key); err != nil {
		return observed{}, err
	}
	return c.until(ctx, predicate)
}
func (c *client) resources(ctx context.Context) (mpvprobe.Resources, error) {
	nodes, err := c.tree(ctx)
	if err != nil {
		return mpvprobe.Resources{}, err
	}
	return resourceNodes(nodes)
}
func resourceNodes(nodes []node) (mpvprobe.Resources, error) {
	var result mpvprobe.Resources
	found := false
	for _, n := range nodes {
		if strings.HasPrefix(n.Label, "resources ") {
			if found || !completeObject(strings.TrimPrefix(n.Label, "resources "), []string{"Cores", "Renderers", "Workers"}, &result) || result.Cores < 0 || result.Renderers < 0 || result.Workers < 0 {
				return mpvprobe.Resources{}, contract.Media
			}
			found = true
		}
	}
	if !found {
		return result, contract.Media
	}
	return result, nil
}

var errPicturePending = errors.New("picture not yet rendered")

func (c *client) pictureTracks(ctx context.Context, generation uint64) (observed, error) {
	nodes, err := c.tree(ctx)
	if err != nil {
		return observed{}, err
	}
	s, err := snapshotNodes(nodes)
	if err != nil {
		return observed{}, err
	}
	if !active(s, generation) || s.State != contract.Ready {
		return observed{}, contract.Media
	}
	image, tracks := false, false
	for _, n := range nodes {
		if key, ok := n.Key.(string); ok && key == "target-video" && n.Role == "image" && n.Bounds.W > 0 && n.Bounds.H > 0 && n.Bounds.W <= 16384 && n.Bounds.H <= 16384 {
			image = true
		}
		if strings.HasPrefix(n.Label, "tracks ") {
			if tracks {
				return observed{}, contract.Media
			}
			var t mpvprobe.Tracks
			if json.Unmarshal([]byte(strings.TrimPrefix(n.Label, "tracks ")), &t) != nil || !t.Video || !t.Audio || !t.ExternalAudio {
				return observed{}, contract.Media
			}
			tracks = true
		}
	}
	if !tracks {
		return observed{}, contract.Media
	}
	if !image {
		return observed{}, errPicturePending
	}
	return s, nil
}

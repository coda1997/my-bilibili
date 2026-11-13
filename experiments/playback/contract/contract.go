// Package contract contains the framework-free boundary for the M1 experiment.
package contract

import (
	"context"
	"maps"
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type ErrorCode string

const (
	NoError     ErrorCode = ""
	Invalid     ErrorCode = "invalid-input"
	Unsupported ErrorCode = "unsupported"
	Canceled    ErrorCode = "canceled"
	Timeout     ErrorCode = "timeout"
	Media       ErrorCode = "media-failure"
	IsClosed    ErrorCode = "closed"
)

func (e ErrorCode) Error() string { return string(e) }

type State string

const (
	Idle    State = "idle"
	Loading State = "loading"
	Ready   State = "ready"
	Playing State = "playing"
	Paused  State = "paused"
	Ended   State = "ended"
	Failed  State = "failed"
	Closed  State = "closed"
)

type Input struct {
	FilePath string
	URL      string
	AudioURL string
	Headers  map[string]string
}

func clean(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsFunc(s, unicode.IsControl)
}

func loopback(s string) bool {
	if s == "" || len(s) > 4096 || !clean(s) {
		return false
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Fragment != "" || strings.Contains(s, "#") || u.Opaque != "" {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return false
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return false
		}
	} else if strings.HasSuffix(u.Host, ":") {
		return false
	}
	decoded, err := url.QueryUnescape(s)
	if err != nil || !clean(decoded) {
		return false
	}
	return true
}

func headerName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", r)) {
			return false
		}
	}
	return true
}

func (in Input) Validate() error {
	if (in.FilePath == "") == (in.URL == "") || len(in.FilePath) > 4096 || !clean(in.FilePath) {
		return Invalid
	}
	if in.URL != "" && !loopback(in.URL) || in.AudioURL != "" && (in.URL == "" || !loopback(in.AudioURL)) {
		return Invalid
	}
	if len(in.Headers) > 8 || in.FilePath != "" && len(in.Headers) != 0 {
		return Invalid
	}
	n, seen := 0, map[string]bool{}
	for k, v := range in.Headers {
		n += len(k) + len(v)
		key := strings.ToLower(k)
		if !headerName(k) || !clean(v) || seen[key] || n > 4096 {
			return Invalid
		}
		seen[key] = true
	}
	return nil
}

type CommandKind string

const (
	Play   CommandKind = "play"
	Pause  CommandKind = "pause"
	Seek   CommandKind = "seek"
	Volume CommandKind = "volume"
	Rate   CommandKind = "rate"
)

type Command struct {
	Kind     CommandKind
	Position time.Duration
	Value    float64
}

func (c Command) Validate() error {
	if math.IsNaN(c.Value) || math.IsInf(c.Value, 0) || c.Position < 0 {
		return Invalid
	}
	switch c.Kind {
	case Play, Pause, Seek:
	case Volume:
		if c.Value < 0 || c.Value > 1 {
			return Invalid
		}
	case Rate:
		if c.Value < .25 || c.Value > 3 {
			return Invalid
		}
	default:
		return Invalid
	}
	return nil
}

type Snapshot struct {
	Generation uint64        `json:"generation"`
	State      State         `json:"state"`
	Position   time.Duration `json:"position"`
	Duration   time.Duration `json:"duration"`
	Rate       float64       `json:"rate"`
	Volume     float64       `json:"volume"`
	Error      ErrorCode     `json:"error"`
}

type Engine interface {
	Snapshot() Snapshot
	Command(Command) error
	Close() error
}

// InputPolicy is an explicit opt-in validator. Nil retains fixture-only validation.
type InputPolicy interface{ Validate(Input) error }

type Opener interface {
	Open(context.Context, Input) (Engine, error)
}

const PollInterval = 50 * time.Millisecond

// Controller is confined to its caller's UI thread. It never creates goroutines.
type Controller struct {
	opener   Opener
	policy   InputPolicy
	now      func() time.Time
	engine   Engine
	ctx      context.Context
	cancel   context.CancelFunc
	deadline time.Time
	nextPoll time.Time
	s        Snapshot
}

func New(opener Opener, now func() time.Time) *Controller {
	return NewWithPolicy(opener, now, nil)
}

func NewWithPolicy(opener Opener, now func() time.Time, policy InputPolicy) *Controller {
	if now == nil {
		now = time.Now
	}
	return &Controller{opener: opener, policy: policy, now: now, s: Snapshot{State: Idle, Rate: 1, Volume: 1}}
}

func safe(err error) ErrorCode {
	if code, ok := err.(ErrorCode); ok {
		switch code {
		case Invalid, Unsupported, Canceled, Timeout, Media, IsClosed:
			return code
		}
	}
	return Media
}

func (c *Controller) dispose() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.engine != nil {
		_ = c.engine.Close()
		c.engine = nil
	}
	c.ctx = nil
	c.deadline, c.nextPoll = time.Time{}, time.Time{}
}

func (c *Controller) stop(state State, code ErrorCode) {
	c.dispose()
	c.s.State, c.s.Error = state, code
}

func (c *Controller) Open(ctx context.Context, in Input) error {
	in.Headers = maps.Clone(in.Headers)
	validate := in.Validate
	if c.policy != nil {
		validate = func() error { return c.policy.Validate(in) }
	}
	if err := validate(); err != nil {
		return safe(err)
	}
	if ctx == nil {
		return Invalid
	}
	c.dispose()
	c.s = Snapshot{Generation: c.s.Generation + 1, State: Loading, Rate: 1, Volume: 1}
	c.ctx, c.cancel = context.WithCancel(ctx)
	if c.ctx.Err() != nil {
		if c.policy != nil && c.ctx.Err() == context.DeadlineExceeded {
			c.stop(Failed, Timeout)
			return Timeout
		}
		c.stop(Closed, Canceled)
		return Canceled
	}
	if c.opener == nil {
		c.stop(Failed, Unsupported)
		return Unsupported
	}
	// Give the opener its own copy, separate even from validator-owned metadata.
	in.Headers = maps.Clone(in.Headers)
	e, err := c.opener.Open(c.ctx, in)
	if err != nil || e == nil {
		if e != nil {
			_ = e.Close()
		}
		code := safe(err)
		c.stop(Failed, code)
		return code
	}
	c.engine = e
	c.deadline = c.now().Add(10 * time.Second)
	c.nextPoll = time.Time{}
	return nil
}

func (c *Controller) Tick() {
	if c.engine == nil {
		return
	}
	if c.ctx.Err() != nil {
		if c.policy != nil && c.ctx.Err() == context.DeadlineExceeded {
			c.stop(Failed, Timeout)
			return
		}
		c.stop(Closed, Canceled)
		return
	}
	now := c.now()
	if now.Before(c.nextPoll) {
		return
	}
	c.nextPoll = now.Add(PollInterval)
	if c.s.State == Loading && !now.Before(c.deadline) {
		c.stop(Failed, Timeout)
		return
	}
	s := c.engine.Snapshot()
	if s.State == Failed || s.Error != NoError {
		code := Media
		if s.Error != NoError {
			code = safe(s.Error)
		}
		c.stop(Failed, code)
		return
	}
	s.Generation = c.s.Generation
	c.s = s
}

func (c *Controller) Command(cmd Command) error {
	if err := cmd.Validate(); err != nil {
		return err
	}
	c.Tick()
	if c.engine == nil {
		return IsClosed
	}
	if cmd.Kind == Seek && c.s.Duration > 0 {
		cmd.Position = min(cmd.Position, c.s.Duration)
	}
	if err := c.engine.Command(cmd); err != nil {
		code := safe(err)
		c.stop(Failed, code)
		return code
	}
	return nil
}

func (c *Controller) Snapshot() Snapshot { return c.s }
func (c *Controller) Engine() Engine     { return c.engine }
func (c *Controller) Active() bool       { return c.engine != nil }
func (c *Controller) Close() error {
	c.stop(Closed, NoError)
	return nil
}

// Package covers presents normalized covers with one bounded worker and cache.
package covers

import (
	"context"
	"errors"
	"image"
	"math"
	"slices"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/catalog"
)

const pendingLimit = 20
const bitmapLimit = 12

type State struct {
	Generation                       uint64
	Cached, Pending, Active, Workers int
	Error                            catalog.Code
	Closed                           bool
}

type bitmapEntry struct {
	bitmap *gui.Bitmap
	order  uint64
}

type request struct {
	id      string
	episode uint64
	ctx     context.Context
	cancel  context.CancelFunc
}

type result struct {
	request  request
	cover    catalog.Cover
	err      error
	consumed chan struct{}
}

// Presenter is UI-thread-only. Start owns the waiting worker, SetVisible changes
// intent, and Tick accepts results and submits work. Render never requests data.
type Presenter struct {
	source          catalog.CoverSource
	ctx             context.Context
	cancel          context.CancelFunc
	started, closed bool
	retiring        bool
	generation, age uint64
	visible, wanted []string
	episodes        map[string]uint64
	attempted       map[string]uint64
	failures        map[string]catalog.Code
	cache           map[string]bitmapEntry
	pending         []string
	active          *request
	requests        chan request
	results         chan result
	done            chan struct{}
	makeBitmap      func(image.Image) *gui.Bitmap
}

func New(source catalog.CoverSource) (*Presenter, error) {
	if source == nil {
		return nil, catalog.Invalid
	}
	return &Presenter{source: source, episodes: map[string]uint64{}, attempted: map[string]uint64{},
		failures: map[string]catalog.Code{}, cache: map[string]bitmapEntry{},
		requests: make(chan request, 1), results: make(chan result, 1), done: make(chan struct{}),
		makeBitmap: gui.NewBitmap}, nil
}

func (p *Presenter) Start(ctx context.Context) error {
	if p.closed {
		return catalog.Closed
	}
	if ctx == nil {
		return catalog.Invalid
	}
	if p.started {
		return nil
	}
	p.started = true
	p.ctx, p.cancel = context.WithCancel(ctx)
	go p.work(p.ctx, p.source)
	return nil
}

func (p *Presenter) SetVisible(ids []string) error {
	if p.closed {
		return catalog.Closed
	}
	if len(ids) > pendingLimit {
		return catalog.Invalid
	}
	visible := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" && !slices.Contains(visible, id) {
			visible = append(visible, id)
		}
	}
	if slices.Equal(visible, p.visible) {
		return nil
	}
	p.generation++
	p.visible = visible
	p.wanted = slices.Clone(visible[:min(len(visible), bitmapLimit)])
	episodes, attempted, failures := map[string]uint64{}, map[string]uint64{}, map[string]catalog.Code{}
	for _, id := range p.wanted {
		if old := p.episodes[id]; old != 0 {
			episodes[id], attempted[id], failures[id] = old, p.attempted[id], p.failures[id]
		} else {
			episodes[id] = p.generation
		}
	}
	p.episodes, p.attempted, p.failures = episodes, attempted, failures
	if p.active != nil && p.episodes[p.active.id] != p.active.episode {
		p.active.cancel()
	}
	p.queue()
	return nil
}

func (p *Presenter) queue() {
	p.pending = nil
	for _, id := range p.wanted {
		if _, cached := p.cache[id]; !cached && p.attempted[id] != p.episodes[id] {
			p.pending = append(p.pending, id)
		}
	}
}

func (p *Presenter) RetryVisible() error {
	if p.closed {
		return catalog.Closed
	}
	if !p.started || p.active != nil {
		return catalog.Invalid
	}
	p.generation++
	for _, id := range p.wanted {
		p.episodes[id] = p.generation
		delete(p.attempted, id)
		delete(p.failures, id)
	}
	p.queue()
	return nil
}

func safeCode(err error) catalog.Code {
	var code catalog.Code
	if errors.As(err, &code) {
		switch code {
		case catalog.Invalid, catalog.Unsupported, catalog.Canceled, catalog.Timeout, catalog.Network,
			catalog.InvalidResponse, catalog.Unavailable, catalog.Restricted, catalog.LoginRequired,
			catalog.SessionExpired, catalog.VerificationRequired, catalog.RateLimited,
			catalog.ServiceUnavailable, catalog.QualityUnavailable, catalog.Closed:
			return code
		}
	}
	return catalog.Unavailable
}

func (p *Presenter) accept(value result) {
	defer close(value.consumed)
	defer value.request.cancel()
	p.active = nil
	id := value.request.id
	if p.closed || value.request.ctx.Err() != nil || p.episodes[id] != value.request.episode {
		p.queue()
		return
	}
	if value.err != nil {
		p.failures[id] = safeCode(value.err)
		return
	}
	cover := value.cover
	if cover.Width < 1 || cover.Width > 320 || cover.Height < 1 || cover.Height > 180 ||
		len(cover.RGBA) != cover.Width*cover.Height*4 {
		p.failures[id] = catalog.InvalidResponse
		return
	}
	if len(p.cache) >= bitmapLimit {
		oldest := ""
		var age uint64
		for key, entry := range p.cache {
			if !slices.Contains(p.wanted, key) && (oldest == "" || entry.order < age) {
				oldest, age = key, entry.order
			}
		}
		delete(p.cache, oldest)
	}
	p.age++
	bitmap := p.makeBitmap(&image.RGBA{Pix: cover.RGBA, Stride: cover.Width * 4, Rect: image.Rect(0, 0, cover.Width, cover.Height)})
	p.cache[id] = bitmapEntry{bitmap: bitmap, order: p.age}
	delete(p.failures, id)
}

func (p *Presenter) Tick() {
	if !p.started || p.closed {
		return
	}
	if p.ctx.Err() != nil {
		p.Close()
		return
	}
	select {
	case value := <-p.results:
		p.accept(value)
	default:
	}
	if p.active == nil && len(p.pending) > 0 {
		id := p.pending[0]
		p.pending = p.pending[1:]
		ctx, cancel := context.WithCancel(p.ctx)
		value := request{id: id, episode: p.episodes[id], ctx: ctx, cancel: cancel}
		p.attempted[id] = value.episode
		p.active = &value
		p.requests <- value
	}
}

func (p *Presenter) State() State {
	state := State{Generation: p.generation, Cached: len(p.cache), Pending: len(p.pending), Closed: p.closed}
	if p.started {
		select {
		case <-p.done:
		default:
			state.Workers = 1
			if p.active != nil || p.retiring {
				state.Active = 1
			}
		}
	}
	for _, id := range p.wanted {
		if code := p.failures[id]; code != catalog.None {
			state.Error = code
			break
		}
	}
	return state
}

func (p *Presenter) Render(c *gui.Context, id string, width, height float32) {
	if width <= 0 || height <= 0 || math.IsNaN(float64(width)) || math.IsNaN(float64(height)) ||
		math.IsInf(float64(width), 0) || math.IsInf(float64(height), 0) {
		return
	}
	if entry, found := p.cache[id]; found && !p.closed {
		gui.Image(c, entry.bitmap).Size(width, height).Fit(gui.Contain).Label("视频封面")
		return
	}
	gui.Box(c).Size(width, height).Background(c.Theme().Surface).Role(gui.RoleImage).Label("暂无封面").
		AlignItems(gui.Center).Justify(gui.Center).Children(func() {
		gui.Text(c, "暂无封面").FontSize(12).TextColor(c.Theme().TextMuted)
	})
}

func (p *Presenter) Close() {
	if p.closed {
		return
	}
	p.closed = true
	p.generation++
	p.retiring = p.active != nil
	if p.active != nil {
		p.active.cancel()
	}
	p.active, p.source = nil, nil
	p.visible, p.wanted, p.pending = nil, nil, nil
	p.cache, p.episodes, p.attempted, p.failures = nil, nil, nil, nil
	if p.cancel != nil {
		p.cancel()
	} else {
		close(p.done)
	}
	p.ctx, p.cancel = nil, nil
}

func (p *Presenter) Done() <-chan struct{} { return p.done }

func (p *Presenter) work(ctx context.Context, source catalog.CoverSource) {
	defer func() {
		select {
		case <-p.results:
		default:
		}
		select {
		case <-p.requests:
		default:
		}
		close(p.done)
	}()
	for {
		var value request
		select {
		case <-ctx.Done():
			return
		case value = <-p.requests:
		}
		out := result{request: value, consumed: make(chan struct{})}
		if err := value.ctx.Err(); err != nil {
			out.err = err
		} else {
			out.cover, out.err = source.Cover(value.ctx, value.id)
		}
		select {
		case <-ctx.Done():
			return
		case p.results <- out:
		}
		select {
		case <-ctx.Done():
			return
		case <-out.consumed:
		}
	}
}

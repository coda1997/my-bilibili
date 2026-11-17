package covers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/catalog"
)

type gate struct{ entered, release chan struct{} }

func newGate() *gate { return &gate{make(chan struct{}), make(chan struct{})} }

type fakeSource struct {
	mu              sync.Mutex
	calls           []string
	active, maximum int
	gate            *gate
	err             error
	bad             bool
}

func (s *fakeSource) Cover(_ context.Context, id string) (catalog.Cover, error) {
	s.mu.Lock()
	s.calls = append(s.calls, id)
	s.active++
	s.maximum = max(s.maximum, s.active)
	blocked, err, bad := s.gate, s.err, s.bad
	s.gate = nil
	s.mu.Unlock()
	if blocked != nil {
		close(blocked.entered)
		<-blocked.release
	}
	s.mu.Lock()
	s.active--
	s.mu.Unlock()
	if err != nil {
		return catalog.Cover{}, err
	}
	if bad {
		return catalog.Cover{Width: 321, Height: 2, RGBA: []byte{1}}, nil
	}
	return catalog.Cover{Width: 2, Height: 2, RGBA: []byte{230, 30, 50, 255, 230, 30, 50, 255, 230, 30, 50, 255, 230, 30, 50, 255}}, nil
}
func (s *fakeSource) count() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.calls) }
func pump(t *testing.T, p *Presenter, condition func(State) bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		p.Tick()
		if condition(p.State()) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("cover state did not settle: %+v", p.State())
}
func settle(t *testing.T, p *Presenter) {
	t.Helper()
	pump(t, p, func(s State) bool { return s.Active == 0 && s.Pending == 0 })
}
func closePresenter(t *testing.T, p *Presenter) {
	t.Helper()
	p.Close()
	select {
	case <-p.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("cover worker did not retire")
	}
}
func presenter(t *testing.T, source *fakeSource) *Presenter {
	t.Helper()
	p, err := New(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closePresenter(t, p) })
	return p
}

func TestPureConstructionStateRenderAndExplicitTick(t *testing.T) {
	if _, err := New(nil); !errors.Is(err, catalog.Invalid) {
		t.Fatal(err)
	}
	source := &fakeSource{}
	p, _ := New(source)
	if p.State().Workers != 0 {
		t.Fatal("constructor started worker")
	}
	if err := p.SetVisible([]string{"opaque-one"}); err != nil {
		t.Fatal(err)
	}
	ui := gui.NewTester(func(c *gui.Context) { p.Render(c, "opaque-one", 80, 45) }, 120, 80)
	for range 10 {
		ui.Frame()
		p.State()
	}
	if source.count() != 0 || p.State().Cached != 0 {
		t.Fatal("Render/State performed I/O")
	}
	if err := p.Start(nil); err != catalog.Invalid {
		t.Fatal(err)
	}
	if err := p.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if source.count() != 0 {
		t.Fatal("Start eagerly requested cover")
	}
	ui.Frame()
	if p.State().Active != 0 || source.count() != 0 {
		t.Fatal("Render submitted pending cover work")
	}
	settle(t, p)
	if source.count() != 1 || p.State().Cached != 1 {
		t.Fatal("explicit Tick did not load")
	}
	ui.Frame()
	pixel := ui.Image().RGBAAt(40, 22)
	if pixel.R != 230 || pixel.G != 30 || pixel.B != 50 {
		t.Fatalf("synthetic bitmap not rasterized: %+v", pixel)
	}
	if strings.Contains(fmt.Sprint(ui.AccessAll()), "opaque-one") {
		t.Fatal("opaque identity in accessibility")
	}
	closePresenter(t, p)
	ui.Frame()
	if pixel := ui.Image().RGBAAt(40, 22); pixel.R == 230 && pixel.G == 30 && pixel.B == 50 {
		t.Fatal("closed renderer retained picture")
	}
	if p.source != nil || p.ctx != nil || p.cache != nil || p.active != nil || len(p.requests) != 0 || len(p.results) != 0 {
		t.Fatal("close retained owned references")
	}
}

func TestBoundedAdmissionNoRepaintRefetchAndEvictBeforeAdd(t *testing.T) {
	source := &fakeSource{}
	p := presenter(t, source)
	ids := make([]string, 20)
	for i := range ids {
		ids[i] = fmt.Sprintf("cover-%d", i)
	}
	if err := p.SetVisible(append(slices.Clone(ids), "over-limit")); err != catalog.Invalid || p.State().Pending != 0 {
		t.Fatal("pending-ID bound lost", err)
	}
	if err := p.SetVisible(ids); err != nil {
		t.Fatal(err)
	}
	ids[0] = "caller-mutated"
	if p.wanted[0] != "cover-0" {
		t.Fatal("demand aliases caller")
	}
	bitmap := p.makeBitmap
	p.makeBitmap = func(img image.Image) *gui.Bitmap {
		if len(p.cache) >= 12 {
			t.Fatal("allocated before evicting")
		}
		return bitmap(img)
	}
	settle(t, p)
	if state := p.State(); state.Cached != 12 || state.Pending > 20 || source.count() != 12 {
		t.Fatalf("initial admission %+v calls %d", state, source.count())
	}
	ids[0] = "cover-0"
	ui := gui.NewTester(func(c *gui.Context) {
		gui.Column(c).Children(func() {
			for _, id := range ids {
				p.Render(c, id, 12, 8)
			}
		})
	}, 100, 220)
	generation := p.State().Generation
	for range 100 {
		p.SetVisible(ids)
		p.Tick()
		ui.Frame()
	}
	if source.count() != 12 || p.State().Cached != 12 || p.State().Generation != generation {
		t.Fatal("repaint caused refetch/eviction loop")
	}
	next := append([]string{"detail"}, ids[:11]...)
	p.SetVisible(next)
	settle(t, p)
	if _, ok := p.cache["detail"]; !ok || source.count() != 13 || p.State().Cached != 12 {
		t.Fatal("new demand not admitted")
	}
	if _, ok := p.cache["cover-11"]; ok {
		t.Fatal("retained displaced entry")
	}
	p.SetVisible(ids[:12])
	settle(t, p)
	if source.count() != 14 {
		t.Fatal("genuine re-entry did not allow loading")
	}
}

func TestStaleCompletionSingleCallAndSameIDReentry(t *testing.T) {
	blocked := newGate()
	source := &fakeSource{gate: blocked}
	p := presenter(t, source)
	p.SetVisible([]string{"a"})
	p.Tick()
	select {
	case <-blocked.entered:
	case <-time.After(time.Second):
		t.Fatal("request missing")
	}
	p.SetVisible([]string{"b"})
	p.SetVisible([]string{"a"})
	p.Tick()
	if source.count() != 1 || p.State().Active != 1 {
		t.Fatal("second call before previous returned")
	}
	if p.active.ctx.Err() == nil {
		t.Fatal("obsolete demand not canceled")
	}
	second := newGate()
	source.mu.Lock()
	source.gate = second
	source.mu.Unlock()
	close(blocked.release)
	pump(t, p, func(state State) bool { return source.count() == 2 && state.Active == 1 })
	select {
	case <-second.entered:
	case <-time.After(time.Second):
		t.Fatal("replacement request missing")
	}
	if p.State().Cached != 0 {
		t.Fatal("obsolete result published before current completion")
	}
	close(second.release)
	settle(t, p)
	if source.count() != 2 || p.State().Cached != 1 {
		t.Fatal("old request published as new episode")
	}
	source.mu.Lock()
	maximum := source.maximum
	source.mu.Unlock()
	if maximum != 1 {
		t.Fatal("concurrent cover calls", maximum)
	}
}

func TestFailureLatchShapeBoundsAndExplicitRetry(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		t.Run(fmt.Sprint(malformed), func(t *testing.T) {
			source := &fakeSource{bad: malformed}
			if !malformed {
				source.err = errors.New("https://private.invalid/cover-sentinel")
			}
			p := presenter(t, source)
			p.SetVisible([]string{"opaque"})
			settle(t, p)
			if p.State().Error == catalog.None || p.State().Cached != 0 {
				t.Fatal("failure became a bitmap")
			}
			for range 30 {
				p.SetVisible([]string{"opaque"})
				p.Tick()
			}
			if source.count() != 1 {
				t.Fatal("failed image automatically retried")
			}
			p.SetVisible([]string{"opaque", "another"})
			settle(t, p)
			if source.count() != 2 {
				t.Fatal("unchanged visible failure retried when another ID entered")
			}
			p.SetVisible([]string{"opaque"})
			data, _ := json.Marshal(p.State())
			if strings.Contains(string(data), "private.invalid") || strings.Contains(string(data), "opaque") {
				t.Fatal("unsafe cover state")
			}
			source.mu.Lock()
			source.bad = false
			source.err = nil
			source.mu.Unlock()
			if err := p.RetryVisible(); err != nil {
				t.Fatal(err)
			}
			settle(t, p)
			if source.count() != 3 || p.State().Cached != 1 || p.State().Error != catalog.None {
				t.Fatal("explicit retry failed")
			}
		})
	}
}

func TestCloseJoinsUncooperativeCallAndDropsRetiredResults(t *testing.T) {
	blocked := newGate()
	source := &fakeSource{gate: blocked}
	p, _ := New(source)
	p.Start(context.Background())
	p.SetVisible([]string{"private-opaque"})
	p.Tick()
	select {
	case <-blocked.entered:
	case <-time.After(time.Second):
		t.Fatal("request missing")
	}
	if err := p.RetryVisible(); err != catalog.Invalid {
		t.Fatal("busy retry accepted", err)
	}
	p.Close()
	p.Tick()
	state := p.State()
	if !state.Closed || state.Cached != 0 || state.Pending != 0 || state.Workers != 1 || state.Active != 1 {
		t.Fatalf("premature zero ownership %+v", state)
	}
	select {
	case <-p.Done():
		t.Fatal("Done before call returned")
	default:
	}
	if p.source != nil || p.ctx != nil || p.active != nil {
		t.Fatal("UI references retained")
	}
	close(blocked.release)
	closePresenter(t, p)
	if state = p.State(); state.Workers != 0 || state.Active != 0 || state.Cached != 0 || len(p.results) != 0 || len(p.requests) != 0 {
		t.Fatalf("terminal ownership %+v", state)
	}
	if err := p.SetVisible([]string{"new"}); err != catalog.Closed {
		t.Fatal(err)
	}
	if err := p.RetryVisible(); err != catalog.Closed {
		t.Fatal(err)
	}
	if err := p.Start(context.Background()); err != catalog.Closed {
		t.Fatal(err)
	}
	unstarted, _ := New(&fakeSource{})
	closePresenter(t, unstarted)
}

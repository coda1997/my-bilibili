package main

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"my-bilibili/internal/account"
	catalogapi "my-bilibili/internal/adapters/bilibili/catalog"
	"my-bilibili/internal/adapters/bilibili/sessiondata"
	"my-bilibili/internal/adapters/bilibili/transport"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/history"
	"my-bilibili/internal/playback"
)

type coreTestRT struct{}

func (coreTestRT) RoundTrip(*http.Request) (*http.Response, error) { return nil, catalog.Network }

type coreAccountStore struct {
	mu                    sync.Mutex
	material              account.Material
	failSave              bool
	loads, saves, deletes atomic.Int32
}

func (s *coreAccountStore) Load() (account.Material, error) {
	s.loads.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.material.Empty() {
		return account.Material{}, account.Missing
	}
	return s.material, nil
}
func (s *coreAccountStore) Save(m account.Material) error {
	s.saves.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failSave {
		return account.Storage
	}
	s.material = m
	return nil
}
func (s *coreAccountStore) Delete() error {
	s.deletes.Add(1)
	s.mu.Lock()
	s.material = account.Material{}
	s.mu.Unlock()
	return nil
}

type coreGateway struct {
	material account.Material
	verifies atomic.Int32
}

func (g *coreGateway) Verify(context.Context, account.Material) error { g.verifies.Add(1); return nil }
func (g *coreGateway) Begin(context.Context) (account.Attempt, error) {
	return coreAttempt{g.material}, nil
}

type coreAttempt struct{ material account.Material }

func (coreAttempt) QR() account.QR { return account.NewQR("SYNTHETIC_CLIENT_CHALLENGE") }
func (a coreAttempt) Poll(context.Context) (account.PollResult, error) {
	return account.PollResult{Status: account.PollComplete, Material: a.material}, nil
}
func (coreAttempt) Close() {}

func coreMaterial(t *testing.T) account.Material {
	t.Helper()
	material, err := sessiondata.Parse([]string{"SESSDATA=SYNTHETIC_CLIENT_ONLY; Domain=bilibili.com; Path=/; Secure; Max-Age=3600"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return material
}

type coreHistoryStore struct {
	mu                             sync.Mutex
	records                        []history.Record
	saveGate                       <-chan struct{}
	closeFailure                   bool
	acquired, closed               bool
	acquires, loads, saves, closes atomic.Int32
}

func (s *coreHistoryStore) Acquire() error {
	s.acquires.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acquired = true
	return nil
}
func (s *coreHistoryStore) Load() ([]history.Record, error) {
	s.loads.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.records), nil
}
func (s *coreHistoryStore) Save(records []history.Record) error {
	s.saves.Add(1)
	s.mu.Lock()
	gate := s.saveGate
	s.mu.Unlock()
	if gate != nil {
		<-gate
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.acquired || s.closed {
		return history.Closed
	}
	s.records = slices.Clone(records)
	return nil
}
func (s *coreHistoryStore) Close() error {
	s.closes.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closeFailure {
		return history.Storage
	}
	s.closed = true
	return nil
}
func (s *coreHistoryStore) snapshot() []history.Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.records)
}

type coreSource struct {
	id                        catalog.VideoID
	detailGate, resolveGate   <-chan struct{}
	resolveQuality            app.QualityID
	detailCalls, resolveCalls atomic.Int32
}

func (s *coreSource) Detail(context.Context) (catalog.Video, error) {
	s.detailCalls.Add(1)
	if s.detailGate != nil {
		<-s.detailGate
	}
	return catalog.Video{ID: s.id, Title: "Synthetic client video", Creator: "Synthetic creator", Parts: []catalog.Part{{ID: "101", Title: "First", Duration: 30 * time.Second}, {ID: "102", Title: "Second", Duration: 30 * time.Second}}}, nil
}
func (s *coreSource) Resolve(ctx context.Context, r app.ResolveRequest) (app.Resolution, error) {
	s.resolveCalls.Add(1)
	if s.resolveGate != nil && r.Quality == s.resolveQuality {
		<-s.resolveGate
	}
	qualities := []app.Quality{{ID: "32", Label: "Synthetic low", Width: 640, Height: 360}, {ID: "64", Label: "Synthetic high", Width: 1280, Height: 720}}
	selected := qualities[1]
	if r.Quality == "32" {
		selected = qualities[0]
	}
	return app.Resolution{Input: playback.Input{FilePath: "/tmp/my-bilibili-synthetic-" + string(r.Part)}, Quality: selected, Qualities: qualities}, nil
}

type coreSegments struct{}

func (coreSegments) Metadata(context.Context, catalog.PartRef) (danmaku.Metadata, error) {
	return danmaku.Metadata{Known: true, Available: true}, nil
}
func (coreSegments) Segment(_ context.Context, _ catalog.PartRef, index int) (danmaku.Segment, error) {
	return danmaku.Segment{Index: index, Encoded: 1, Entries: []danmaku.SegmentEntry{{ID: "synthetic-one", Entry: danmaku.Entry{At: time.Second, Text: "合成弹幕", Mode: danmaku.ScrollRTL, Color: 0xffffff}}}}, nil
}

type coreEngine struct {
	state            playback.Snapshot
	closed, holdSeek bool
	seeks            []time.Duration
}

func (e *coreEngine) Snapshot() playback.Snapshot { return e.state }
func (e *coreEngine) Close() error                { e.closed = true; e.state.State = playback.Closed; return nil }
func (e *coreEngine) Command(command playback.Command) error {
	switch command.Kind {
	case playback.Play:
		e.state.State = playback.Playing
	case playback.Pause:
		e.state.State = playback.Paused
	case playback.Seek:
		e.seeks = append(e.seeks, command.Position)
		if !e.holdSeek {
			e.state.Position = command.Position
			e.state.State = playback.Paused
		}
	case playback.Volume:
		e.state.Volume = command.Value
	case playback.Rate:
		e.state.Rate = command.Value
	}
	return nil
}

type coreOpener struct {
	engines  []*coreEngine
	holdNext bool
}

func (o *coreOpener) Open(context.Context, playback.Input) (playback.Engine, error) {
	e := &coreEngine{state: playback.Snapshot{State: playback.Ready, Duration: 30 * time.Second, Rate: 1, Volume: 1}, holdSeek: o.holdNext}
	o.holdNext = false
	o.engines = append(o.engines, e)
	return e, nil
}

type coreHarness struct {
	core         *clientCore
	accountStore *coreAccountStore
	historyStore *coreHistoryStore
	gateway      *coreGateway
	opener       *coreOpener
	sources      map[string]*coreSource
}

func newCoreHarness(t *testing.T, signed bool) *coreHarness {
	t.Helper()
	h := &coreHarness{accountStore: &coreAccountStore{}, historyStore: &coreHistoryStore{}, gateway: &coreGateway{}, opener: &coreOpener{}, sources: map[string]*coreSource{}}
	h.gateway.material = coreMaterial(t)
	if signed {
		h.accountStore.material = h.gateway.material
	}
	service := transport.New(coreTestRT{})
	cfg := clientConfig{gateway: h.gateway, store: h.accountStore, service: service, historyStore: h.historyStore, opener: h.opener, coverTransport: coreTestRT{},
		newSource: func(_ *transport.Scope, _ *catalogapi.Client, entry catalog.Entry) (app.Source, error) {
			source := h.sources[entry.ID]
			if source == nil {
				source = &coreSource{id: catalog.VideoID(entry.ID)}
				h.sources[entry.ID] = source
			}
			return source, nil
		}, newSegments: func(*transport.Scope, catalog.Video) (danmaku.SegmentSource, error) { return coreSegments{}, nil }}
	var err error
	h.core, err = newClientCore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.core.RequestClose(false); waitCore(t, h.core, func() bool { return h.core.closed }) })
	return h
}
func waitCore(t *testing.T, c *clientCore, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		c.Tick()
		if predicate() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("client state did not converge")
		}
		time.Sleep(time.Millisecond)
	}
}
func startCore(t *testing.T, h *coreHarness) {
	t.Helper()
	if err := h.core.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitCore(t, h.core, func() bool {
		return !h.core.account.Snapshot().Busy && h.core.history.State().Phase == history.Ready && h.core.group != nil && !h.core.group.retiring && !h.core.group.scope.State().Closed
	})
}
func openCore(t *testing.T, h *coreHarness, id string) {
	t.Helper()
	if err := h.core.OpenEntry(catalog.Entry{Kind: catalog.BVEntry, ID: id}); err != nil {
		t.Fatal(err)
	}
	waitCore(t, h.core, func() bool {
		return h.core.group != nil && h.core.group.video != nil && !h.core.group.video.retiring && h.core.group.video.session.State().PlaybackPart != "" && h.core.group.video.session.State().Playback.State == playback.Ready
	})
}

func TestClientConstructionAndCloseBeforeStartArePure(t *testing.T) {
	h := newCoreHarness(t, false)
	if h.accountStore.loads.Load() != 0 || h.historyStore.acquires.Load() != 0 || h.gateway.verifies.Load() != 0 || len(h.opener.engines) != 0 {
		t.Fatal("construction performed work")
	}
	h.core.RequestClose(false)
	waitCore(t, h.core, func() bool { return h.core.closed })
	if h.historyStore.loads.Load() != 0 || h.historyStore.acquires.Load() != 0 || h.accountStore.loads.Load() != 0 || h.core.PendingOwners() != 0 {
		t.Fatal("unstarted close acquired data or kept owners")
	}
	if !errors.Is(h.core.Start(context.Background()), catalog.Closed) {
		t.Fatal("closed client restarted")
	}
}

func TestClientLatestNavigationWaitsForActualOldSourceAndKeepsSearchUsable(t *testing.T) {
	h := newCoreHarness(t, false)
	gate := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(gate) }) })
	a := &coreSource{id: "BV0000000001", detailGate: gate}
	h.sources[string(a.id)] = a
	startCore(t, h)
	if err := h.core.OpenEntry(catalog.Entry{Kind: catalog.BVEntry, ID: string(a.id)}); err != nil {
		t.Fatal(err)
	}
	waitCore(t, h.core, func() bool { return a.detailCalls.Load() == 1 })
	if err := h.core.OpenEntry(catalog.Entry{Kind: catalog.BVEntry, ID: "BV0000000002"}); err != nil {
		t.Fatal(err)
	}
	if err := h.core.OpenEntry(catalog.Entry{Kind: catalog.BVEntry, ID: "BV0000000003"}); err != nil {
		t.Fatal(err)
	}
	if err := h.core.group.browser.Search("synthetic"); err != nil {
		t.Fatal(err)
	}
	waitCore(t, h.core, func() bool { return h.core.group.browser.State().Phase == catalog.SearchFailed })
	if len(h.opener.engines) != 0 || h.sources["BV0000000002"].detailCalls.Load() != 0 || h.sources["BV0000000003"].detailCalls.Load() != 0 {
		t.Fatal("replacement started before old source ended")
	}
	once.Do(func() { close(gate) })
	waitCore(t, h.core, func() bool { return h.core.group.video != nil && h.core.group.video.session.State().PlaybackPart != "" })
	if got := h.core.group.video.session.State().Video.ID; got != "BV0000000003" {
		t.Fatal("obsolete navigation accepted")
	}
	if h.core.config.metrics.source.snapshot().peak != 1 || h.sources["BV0000000002"].detailCalls.Load() != 0 {
		t.Fatal("source ownership overlapped or replaced request ran")
	}
}

func TestClientQualityResumeCannotEraseHistoryOrCreateAnotherVisit(t *testing.T) {
	h := newCoreHarness(t, false)
	startCore(t, h)
	openCore(t, h, "BV0000000001")
	v := h.core.group.video
	if len(h.core.history.State().Records) != 0 {
		t.Fatal("detail-only history row")
	}
	if err := v.session.Play(); err != nil {
		t.Fatal(err)
	}
	e := h.opener.engines[0]
	e.state.Position = 7 * time.Second
	waitCore(t, h.core, func() bool {
		s := h.core.history.State()
		return len(s.Records) == 1 && s.Records[0].Position == 7*time.Second
	})
	if err := v.session.Pause(); err != nil {
		t.Fatal(err)
	}
	waitCore(t, h.core, func() bool { return v.session.State().Playback.State == playback.Paused })
	visit := h.core.visit
	h.opener.holdNext = true
	if err := v.session.SelectQuality("32"); err != nil {
		t.Fatal(err)
	}
	waitCore(t, h.core, func() bool {
		return len(h.opener.engines) == 2 && v.session.State().Resuming && len(h.opener.engines[1].seeks) == 1
	})
	h.opener.engines[1].state.State = playback.Paused
	waitCore(t, h.core, func() bool { return v.session.State().Playback.State == playback.Paused })
	if h.core.history.State().Records[0].Position != 7*time.Second || h.core.visit != visit {
		t.Fatal("unconfirmed resume erased history or changed visit")
	}
	h.opener.engines[1].state.Position = 7 * time.Second
	waitCore(t, h.core, func() bool { return !v.session.State().Resuming })
	ref := h.core.history.State().Records[0].Ref
	if err := h.core.history.Delete(ref); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		h.core.Tick()
	}
	if len(h.core.history.State().Records) != 0 {
		t.Fatal("same visit resurrected deleted history")
	}
	if err := v.session.SelectPart("101"); err != nil {
		t.Fatal(err)
	}
	waitCore(t, h.core, func() bool { return h.core.visit > visit && len(h.core.history.State().Records) == 1 })
}

func TestClientNavigationDeadlineIncludesWaitingForOldOwner(t *testing.T) {
	h := newCoreHarness(t, false)
	gate := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(gate) }) })
	source := &coreSource{id: "BV0000000001", detailGate: gate}
	h.sources[string(source.id)] = source
	startCore(t, h)
	if err := h.core.OpenEntry(catalog.Entry{Kind: catalog.BVEntry, ID: string(source.id)}); err != nil {
		t.Fatal(err)
	}
	waitCore(t, h.core, func() bool { return source.detailCalls.Load() == 1 })
	if err := h.core.OpenEntry(catalog.Entry{Kind: catalog.BVEntry, ID: "BV0000000002"}); err != nil {
		t.Fatal(err)
	}
	h.core.pending.deadline = time.Now().Add(-time.Millisecond)
	once.Do(func() { close(gate) })
	waitCore(t, h.core, func() bool { return h.core.navigationError == catalog.Timeout })
	if h.sources["BV0000000002"].detailCalls.Load() != 0 || len(h.opener.engines) != 0 || h.core.pending != nil {
		t.Fatal("expired queued navigation started work")
	}
	if err := h.core.RetryNavigation(); err != nil {
		t.Fatal(err)
	}
	waitCore(t, h.core, func() bool { return h.core.group.video != nil && h.core.group.video.session.State().PlaybackPart != "" })
	if h.core.group.video.session.State().Video.ID != "BV0000000002" {
		t.Fatal("explicit retry did not retain requested video")
	}
}

func TestClientNewVideoRetiresTimelineAndKeepsUserSettings(t *testing.T) {
	h := newCoreHarness(t, false)
	startCore(t, h)
	openCore(t, h, "BV0000000001")
	old := h.core.group.video
	settings := old.overlay.Settings()
	settings.Opacity = .4
	if err := old.overlay.Configure(settings); err != nil {
		t.Fatal(err)
	}
	openCore(t, h, "BV0000000002")
	current := h.core.group.video
	if current == old || !old.overlay.State().Closed || !h.opener.engines[0].closed || current.overlay.Settings().Opacity != .4 {
		t.Fatal("video lifetime or settings were reused incorrectly")
	}
	if current.session.State().Playback.Generation != 1 {
		t.Fatal("new session generation did not restart as expected")
	}
}

func TestClientRevocationRetiresPendingNavigationAndDoesNotReopenAnonymously(t *testing.T) {
	h := newCoreHarness(t, true)
	startCore(t, h)
	openCore(t, h, "BV0000000001")
	if !h.core.group.scope.State().Authenticated {
		t.Fatal("test account not published")
	}
	old := h.core.group
	engine := h.opener.engines[0]
	if err := h.core.OpenEntry(catalog.Entry{Kind: catalog.BVEntry, ID: "BV0000000002"}); err != nil {
		t.Fatal(err)
	}
	if err := (clientAccountController{h.core}).Logout(); err != nil {
		t.Fatal(err)
	}
	h.core.Tick()
	if !engine.closed || h.core.pending != nil {
		t.Fatal("revocation retained native or pending navigation")
	}
	waitCore(t, h.core, func() bool {
		state := h.core.account.Snapshot()
		return h.core.group != nil && h.core.group != old && !h.core.group.retiring && state.Phase == account.SignedOut && !state.Busy
	})
	if h.core.group.video != nil || len(h.opener.engines) != 1 || h.core.group.browser.State().Query != "" || h.accountStore.deletes.Load() != 1 {
		t.Fatal("revoked content reopened or query resubmitted")
	}
}

func TestClientCloseJoinsHeldSourceAndHistoryWithoutPaint(t *testing.T) {
	h := newCoreHarness(t, false)
	startCore(t, h)
	openCore(t, h, "BV0000000001")
	saveGate, sourceGate := make(chan struct{}), make(chan struct{})
	var saveOnce, sourceOnce sync.Once
	t.Cleanup(func() { saveOnce.Do(func() { close(saveGate) }); sourceOnce.Do(func() { close(sourceGate) }) })
	h.historyStore.mu.Lock()
	h.historyStore.saveGate = saveGate
	h.historyStore.mu.Unlock()
	v := h.core.group.video
	h.sources["BV0000000001"].resolveGate = sourceGate
	h.sources["BV0000000001"].resolveQuality = "32"
	if err := v.session.Play(); err != nil {
		t.Fatal(err)
	}
	h.opener.engines[0].state.Position = 5 * time.Second
	waitCore(t, h.core, func() bool { return h.historyStore.saves.Load() > 0 && h.core.history.State().Busy })
	if err := v.session.SelectQuality("32"); err != nil {
		t.Fatal(err)
	}
	waitCore(t, h.core, func() bool { return h.sources["BV0000000001"].resolveCalls.Load() == 2 })
	h.core.RequestClose(false)
	if h.core.closed || h.core.PendingOwners() == 0 || !h.opener.engines[0].closed {
		t.Fatal("close fabricated completion or left native open")
	}
	sourceOnce.Do(func() { close(sourceGate) })
	waitCore(t, h.core, func() bool { return channelDone(v.session.Done()) })
	if h.core.closed || h.core.PendingOwners() == 0 {
		t.Fatal("held file operation was not owned")
	}
	saveOnce.Do(func() { close(saveGate) })
	waitCore(t, h.core, func() bool { return h.core.closed })
	if h.core.PendingOwners() != 0 || h.historyStore.closes.Load() != 1 || len(h.opener.engines) != 1 || len(h.historyStore.snapshot()) != 1 {
		t.Fatal("close did not save/join exactly the accepted owners")
	}
}

func TestClientCleanupFailureRequiresExplicitRetryAndCannotBeDiscarded(t *testing.T) {
	h := newCoreHarness(t, false)
	startCore(t, h)
	h.historyStore.mu.Lock()
	h.historyStore.closeFailure = true
	h.historyStore.mu.Unlock()
	h.core.RequestClose(false)
	waitCore(t, h.core, func() bool { s := h.core.history.State(); return s.Phase == history.Failed && !s.Busy })
	h.core.RequestClose(true)
	for i := 0; i < 20; i++ {
		h.core.Tick()
	}
	if h.core.closed || h.historyStore.closes.Load() != 1 || h.core.PendingOwners() == 0 {
		t.Fatal("cleanup failure retried or bypassed implicitly")
	}
	h.historyStore.mu.Lock()
	h.historyStore.closeFailure = false
	h.historyStore.mu.Unlock()
	if err := h.core.history.Retry(); err != nil {
		t.Fatal(err)
	}
	waitCore(t, h.core, func() bool { return h.core.closed })
	if h.historyStore.closes.Load() != 2 {
		t.Fatal("cleanup retry did not retain original ownership")
	}
}

func TestClientPendingCloseUnsavedDiscardRetainsStoredData(t *testing.T) {
	h := newCoreHarness(t, false)
	startCore(t, h)
	h.accountStore.mu.Lock()
	h.accountStore.failSave = true
	h.accountStore.mu.Unlock()
	action := clientAccountController{h.core}
	if err := action.Begin(); err != nil {
		t.Fatal(err)
	}
	waitCore(t, h.core, func() bool { s := action.Snapshot(); return s.Phase == account.SaveFailed && !s.Busy })
	h.core.RequestClose(false)
	if h.core.closed {
		t.Fatal("unconfirmed account save silently discarded")
	}
	if err := action.DiscardUnsaved(); err != nil {
		t.Fatal(err)
	}
	waitCore(t, h.core, func() bool { return h.core.closed })
	if h.accountStore.saves.Load() != 1 || h.accountStore.deletes.Load() != 0 {
		t.Fatal("discard mutated stored account material")
	}
}

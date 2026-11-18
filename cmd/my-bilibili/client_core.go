package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/account"
	catalogapi "my-bilibili/internal/adapters/bilibili/catalog"
	danmakuapi "my-bilibili/internal/adapters/bilibili/danmaku"
	mediaapi "my-bilibili/internal/adapters/bilibili/playback"
	"my-bilibili/internal/adapters/bilibili/transport"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/desktop"
	catalogview "my-bilibili/internal/desktop/catalog"
	"my-bilibili/internal/desktop/covers"
	danmakuview "my-bilibili/internal/desktop/danmaku"
	"my-bilibili/internal/history"
	"my-bilibili/internal/playback"
)

// clientConfig selects the composition root's actual external capabilities.
// Construction transfers their ownership without starting network or file work.
type clientConfig struct {
	gateway        account.Gateway
	store          account.Store
	closeGateway   func()
	service        *transport.Client
	coverTransport http.RoundTripper
	historyStore   history.Store
	opener         playback.Opener
	policy         playback.InputPolicy
	fixtureAudio   bool
	newSource      func(*transport.Scope, *catalogapi.Client, catalog.Entry) (app.Source, error)
	newSegments    func(*transport.Scope, catalog.Video) (danmaku.SegmentSource, error)
	metrics        *clientMetrics
}

type navigationRequest struct {
	entry    catalog.Entry
	options  app.Options
	revision uint64
}

type clientVideo struct {
	session                                *app.Session
	player                                 *desktop.View
	overlay                                *danmakuview.View
	loader                                 *danmaku.Loader
	cancel                                 context.CancelFunc
	ctx                                    context.Context
	deadline                               time.Time
	navigation                             uint64
	retiring                               bool
	expectedAudio                          bool
	loaderError                            catalog.Code
	loaderAttempted                        bool
	presentationGeneration, windowRevision uint64
	loaderSample                           playback.Snapshot
	firstComment                           time.Duration
	hasComment                             bool
}

type clientScope struct {
	scope                   *transport.Scope
	catalog                 *catalogapi.Client
	browser                 *catalog.Browser
	covers                  *covers.Presenter
	view                    *catalogview.View
	video                   *clientVideo
	retiring, coverRetiring bool
	catalogPainted          bool
}

type scopeCoverView struct{ owner *clientScope }

func (v scopeCoverView) Render(c *gui.Context, id string, w, h float32) {
	v.owner.catalogPainted = true
	if v.owner.covers != nil {
		v.owner.covers.Render(c, id, w, h)
	}
}

// Mutable composition state and its methods belong to the UI thread. Tick
// accepts completed retirement without waiting on channels, HTTP or files.
type clientCore struct {
	config                                                clientConfig
	account                                               *account.Manager
	history                                               *history.Manager
	group                                                 *clientScope
	pending                                               *clientVideo
	lastRequest                                           *navigationRequest
	ctx                                                   context.Context
	cancel                                                context.CancelFunc
	started, closing, closed                              bool
	navigation, visit, watchedNavigation, watchedSequence uint64
	settings                                              danmaku.Settings
	coverDemand                                           []string
	navigationError                                       catalog.Code
	closeError                                            error
	lastPlayback                                          playback.Snapshot
	joined                                                <-chan struct{}
	done                                                  chan struct{}
}

func newClientCore(config clientConfig) (*clientCore, error) {
	if config.gateway == nil || config.store == nil || config.service == nil || config.historyStore == nil || config.opener == nil {
		return nil, catalog.Invalid
	}
	h, err := history.New(config.historyStore)
	if err != nil {
		return nil, err
	}
	if config.metrics == nil {
		config.metrics = &clientMetrics{}
	}
	if config.newSource == nil {
		config.newSource = func(scope *transport.Scope, details *catalogapi.Client, entry catalog.Entry) (app.Source, error) {
			return mediaapi.NewSource(scope, details, entry)
		}
	}
	if config.newSegments == nil {
		config.newSegments = func(scope *transport.Scope, video catalog.Video) (danmaku.SegmentSource, error) {
			return danmakuapi.New(scope, video)
		}
	}
	return &clientCore{config: config, account: account.NewWithPublisher(config.gateway, config.store, config.service), history: h,
		settings: danmaku.DefaultSettings(), done: make(chan struct{}), lastPlayback: playback.Snapshot{State: playback.Idle, Rate: 1, Volume: 1}}, nil
}

func (c *clientCore) Start(parent context.Context) error {
	if c.closing {
		return catalog.Closed
	}
	if parent == nil || parent.Err() != nil {
		return catalog.Canceled
	}
	if c.started {
		return nil
	}
	c.started = true
	c.ctx, c.cancel = context.WithCancel(parent)
	if err := c.history.Start(); err != nil {
		c.RequestClose(false)
		return err
	}
	if err := c.account.Restore(); err != nil {
		c.RequestClose(false)
		return err
	}
	c.bindScope()
	return nil
}

func (c *clientCore) bindScope() {
	bound, err := c.config.service.Bind()
	if err != nil {
		c.navigationError = contentCode(err)
		return
	}
	g := &clientScope{scope: bound}
	g.catalog = catalogapi.New(bound, c.config.coverTransport)
	g.browser, err = catalog.NewBrowser(&countedSearch{source: g.catalog, metrics: c.config.metrics})
	if err == nil {
		g.covers, err = covers.New(g.catalog)
	}
	if err == nil {
		g.view, err = catalogview.New(g.browser, c.OpenEntry, scopeCoverView{g})
	}
	c.group = g
	if err == nil {
		err = g.browser.Start(bound.Context())
	}
	if err == nil {
		err = g.covers.Start(bound.Context())
	}
	if err != nil {
		c.navigationError = contentCode(err)
		c.retireScope()
	}
}

func (c *clientCore) OpenEntry(entry catalog.Entry) error {
	return c.Open(entry, app.Options{InitialPage: entry.Page})
}

func (c *clientCore) Open(entry catalog.Entry, options app.Options) error {
	if c.closing {
		return catalog.Closed
	}
	g := c.group
	if !c.started || g == nil || g.retiring || g.scope.State().Closed {
		return catalog.Canceled
	}
	deadline := time.Now().Add(10 * time.Second)
	source, err := c.config.newSource(g.scope, g.catalog, entry)
	if err != nil {
		return contentCode(err)
	}
	v := &clientVideo{deadline: deadline, overlay: danmakuview.New()}
	v.ctx, v.cancel = context.WithCancel(g.scope.Context())
	options.InputPolicy = c.config.policy
	opener := observedOpener{delegate: c.config.opener, metrics: c.config.metrics, accepted: func(input playback.Input) {
		v.expectedAudio = c.config.fixtureAudio || input.AudioURL != ""
	}}
	controller := playback.NewWithPolicy(opener, nil, c.config.policy)
	wrapped := &queuedDetailSource{source: &countedSource{source: source, metrics: c.config.metrics}, deadline: deadline}
	v.session, err = app.NewWithOptions(wrapped, controller, options)
	if err != nil {
		v.cancel()
		v.overlay.Close()
		return contentCode(err)
	}
	settings := c.settings
	if g.video != nil && !g.video.retiring {
		settings = g.video.overlay.Settings()
	}
	if err = v.overlay.Configure(settings); err != nil {
		v.cancel()
		v.overlay.Close()
		_ = v.session.Close()
		return catalog.Unavailable
	}
	v.player = &desktop.View{Player: v.session, Danmaku: v.overlay}
	if c.pending != nil {
		c.retireVideo(c.pending)
		c.pending = nil
	}
	c.navigation++
	v.navigation = c.navigation
	c.pending = v
	c.lastRequest = &navigationRequest{entry: entry, options: options, revision: g.scope.State().Revision}
	c.navigationError = catalog.None
	if g.video != nil && !g.video.retiring {
		c.observeHistory(g.video)
		c.retireVideo(g.video)
	}
	c.retireCovers()
	return nil
}

func (c *clientCore) Continue(record history.Record) error {
	if record.Ref.Video == "" || record.Ref.Part == "" || record.Completed {
		return catalog.Invalid
	}
	return c.Open(catalog.Entry{Kind: catalog.BVEntry, ID: string(record.Ref.Video)}, app.Options{InitialPart: record.Ref.Part, StartAt: record.Position, Autoplay: true})
}
func (c *clientCore) Replay(record history.Record) error {
	if record.Ref.Video == "" || record.Ref.Part == "" {
		return catalog.Invalid
	}
	return c.Open(catalog.Entry{Kind: catalog.BVEntry, ID: string(record.Ref.Video)}, app.Options{InitialPart: record.Ref.Part, Autoplay: true})
}
func (c *clientCore) RetryNavigation() error {
	if c.lastRequest == nil || c.navigationError == catalog.None || c.group == nil || c.lastRequest.revision != c.group.scope.State().Revision {
		return catalog.Invalid
	}
	request := *c.lastRequest
	return c.Open(request.entry, request.options)
}

func (c *clientCore) retireCovers() {
	c.coverDemand = nil
	if c.group != nil && c.group.covers != nil && !c.group.coverRetiring {
		c.group.covers.Close()
		c.group.coverRetiring = true
	}
}
func (c *clientCore) retireVideo(v *clientVideo) {
	if v == nil || v.retiring {
		return
	}
	v.retiring = true
	c.settings = v.overlay.Settings()
	v.overlay.Close()
	if v.loader != nil {
		v.loader.Close()
	}
	if err := v.session.Close(); err != nil && c.closeError == nil {
		c.closeError = err
	}
	c.lastPlayback = v.session.State().Playback
	v.cancel()
}
func (c *clientCore) retireScope() {
	g := c.group
	if g == nil || g.retiring {
		return
	}
	g.retiring = true
	if g.video != nil {
		c.observeHistory(g.video)
		c.retireVideo(g.video)
	}
	if g.browser != nil {
		g.browser.Close()
	}
	if g.view != nil {
		g.view.Close()
	}
	c.retireCovers()
	g.catalog.Close()
}
func videoDone(v *clientVideo) bool {
	return v == nil || channelDone(v.session.Done()) && (v.loader == nil || channelDone(v.loader.Done()))
}
func channelDone(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}
func scopeDone(g *clientScope) bool {
	return g == nil || (g.browser == nil || channelDone(g.browser.Done())) && (g.covers == nil || channelDone(g.covers.Done())) && videoDone(g.video)
}

func (c *clientCore) Tick() {
	if c.closed {
		return
	}
	c.history.Tick()
	if c.closing {
		if c.joined != nil && channelDone(c.joined) {
			c.group, c.pending, c.coverDemand, c.lastRequest = nil, nil, nil, nil
			c.closed = true
			close(c.done)
		}
		return
	}
	if !c.started {
		return
	}
	if c.ctx.Err() != nil {
		c.RequestClose(false)
		return
	}
	state := c.config.service.State()
	if state.ExpiredEpoch != 0 {
		c.account.SessionExpired(state.ExpiredEpoch)
		state = c.config.service.State()
	}
	if g := c.group; g != nil && (g.scope.State().Closed || g.scope.State().Revision != state.Revision) {
		if c.pending != nil {
			c.retireVideo(c.pending)
			c.pending = nil
		}
		c.lastRequest = nil
		c.navigationError = catalog.None
		c.retireScope()
	}
	if c.group != nil && c.group.retiring {
		if !scopeDone(c.group) {
			return
		}
		c.group = nil
	}
	if c.group == nil && !state.Closed {
		c.bindScope()
	}
	g := c.group
	if g == nil || g.retiring {
		return
	}
	g.browser.Tick()
	if g.video != nil && g.video.retiring {
		if !videoDone(g.video) {
			return
		}
		g.video = nil
	}
	if g.coverRetiring {
		if !channelDone(g.covers.Done()) {
			return
		}
		g.covers = nil
		g.coverRetiring = false
	}
	if c.pending != nil && g.video == nil {
		v := c.pending
		c.pending = nil
		if !time.Now().Before(v.deadline) || v.ctx.Err() != nil {
			c.retireVideo(v)
			c.navigationError = catalog.Timeout
		} else if err := v.session.Start(v.ctx); err != nil {
			c.retireVideo(v)
			c.navigationError = contentCode(err)
		} else {
			g.video = v
		}
	}
	if g.covers == nil {
		presenter, err := covers.New(g.catalog)
		if err == nil {
			err = presenter.Start(g.scope.Context())
		}
		if err != nil {
			if presenter != nil {
				presenter.Close()
			}
			c.navigationError = contentCode(err)
			return
		}
		g.covers = presenter
	}
	if err := g.covers.SetVisible(c.coverDemand); err != nil {
		c.navigationError = contentCode(err)
	}
	g.covers.Tick()
	if g.video != nil {
		g.video.session.Tick()
		c.bindSegments(g.video)
		c.observeHistory(g.video)
	}
}

func (c *clientCore) SetCoverDemand(ids []string) error {
	if c.closing {
		return catalog.Closed
	}
	if len(ids) > 20 {
		return catalog.Invalid
	}
	c.coverDemand = append(c.coverDemand[:0], ids...)
	return nil
}

func (c *clientCore) bindSegments(v *clientVideo) {
	s := v.session.State()
	if s.DetailPhase != app.DetailReady || s.PlaybackPart == "" || s.Playback.Generation == 0 {
		return
	}
	if s.Playback.Generation != v.presentationGeneration {
		if err := v.overlay.Reset(s.Playback.Generation); err != nil {
			v.loaderError = catalog.Unavailable
			return
		}
		v.presentationGeneration = s.Playback.Generation
		v.windowRevision = 0
		v.hasComment = false
	}
	if !v.loaderAttempted {
		v.loaderAttempted = true
		source, err := c.config.newSegments(c.group.scope, s.Video)
		if err == nil {
			v.loader, err = danmaku.NewLoader(source)
		}
		if err == nil {
			err = v.loader.Start(v.ctx)
		}
		if err != nil {
			v.loaderError = contentCode(err)
			return
		}
	}
	if v.loader == nil {
		return
	}
	// A first history resume does not fetch segment one while its initial seek
	// is unresolved. Same-part quality replacement keeps its valid cache.
	ref := catalog.PartRef{Video: s.Video.ID, Part: s.PlaybackPart}
	old := v.loader.State()
	if s.Resuming && old.Ref != ref {
		return
	}
	for _, part := range s.Video.Parts {
		if part.ID == s.PlaybackPart {
			if err := v.loader.Bind(ref, part.Duration, s.Playback.Generation); err != nil {
				v.loaderError = contentCode(err)
				return
			}
			break
		}
	}
	if !s.Resuming && s.ResumeError == catalog.None || s.Playback.State == playback.Loading {
		v.loaderSample = s.Playback
	}
	v.loader.Tick(v.loaderSample)
	state := v.loader.State()
	v.loaderError = state.Error
	if state.Revision != v.windowRevision {
		window := v.loader.Window()
		if window.Generation == s.Playback.Generation {
			if err := v.overlay.SetEntries(window.Generation, window.Entries); err != nil {
				v.loaderError = catalog.Unavailable
			}
			v.windowRevision = window.Revision
			v.hasComment = len(window.Entries) != 0
			if v.hasComment {
				v.firstComment = window.Entries[0].At
			}
		}
	}
}
func (c *clientCore) RetryDanmaku() error {
	if c.closing || c.group == nil || c.group.video == nil || c.group.video.retiring {
		return catalog.Closed
	}
	v := c.group.video
	if v.loader == nil {
		v.loaderAttempted, v.loaderError = false, catalog.None
		return nil
	}
	return v.loader.Retry()
}

func (c *clientCore) observeHistory(v *clientVideo) {
	if v == nil || v.retiring {
		return
	}
	s := v.session.State()
	if s.WatchSequence == 0 || s.Resuming || s.ResumeError != catalog.None || s.PlaybackPart == "" {
		return
	}
	if c.watchedNavigation != v.navigation || c.watchedSequence != s.WatchSequence {
		c.visit++
		c.watchedNavigation, c.watchedSequence = v.navigation, s.WatchSequence
	}
	if c.history.State().Phase != history.Ready {
		return
	}
	for _, part := range s.Video.Parts {
		if part.ID == s.PlaybackPart {
			err := c.history.Observe(history.Sample{Visit: c.visit, Playback: s.Playback.State,
				Record: history.Record{Ref: catalog.PartRef{Video: s.Video.ID, Part: part.ID}, Title: s.Video.Title, PartTitle: part.Title, Position: s.Playback.Position, Duration: s.Playback.Duration}})
			if err != nil && c.closeError == nil {
				c.closeError = err
			}
			return
		}
	}
}

func (c *clientCore) ownerChannels() []<-chan struct{} {
	owners := []<-chan struct{}{c.account.Done(), c.history.Done(), c.config.service.Done()}
	if g := c.group; g != nil {
		if g.browser != nil {
			owners = append(owners, g.browser.Done())
		}
		if g.covers != nil {
			owners = append(owners, g.covers.Done())
		}
		if g.video != nil {
			owners = append(owners, g.video.session.Done())
			if g.video.loader != nil {
				owners = append(owners, g.video.loader.Done())
			}
		}
	}
	return owners
}
func (c *clientCore) PendingOwners() int {
	if !c.closing {
		return 0
	}
	count := 0
	for _, done := range c.ownerChannels() {
		if !channelDone(done) {
			count++
		}
	}
	if c.joined != nil && !channelDone(c.joined) {
		count++
	}
	return count
}
func (c *clientCore) Done() <-chan struct{} { return c.done }

func (c *clientCore) RequestClose(discardHistory bool) {
	if c.closed {
		return
	}
	if c.closing {
		if discardHistory {
			c.history.RequestClose(true)
		}
		return
	}
	c.closing = true
	if c.pending != nil {
		c.retireVideo(c.pending)
		c.pending = nil
	}
	c.retireScope()
	c.account.RequestClose(false)
	c.history.RequestClose(discardHistory)
	c.config.service.Close()
	if c.cancel != nil {
		c.cancel()
	}
	owners := c.ownerChannels()
	joined := make(chan struct{})
	c.joined = joined
	go func() {
		for _, done := range owners {
			<-done
		}
		if c.config.closeGateway != nil {
			c.config.closeGateway()
		}
		close(joined)
	}()
}
func (c *clientCore) KeepAccountAndClose() {
	s := c.account.Snapshot()
	if c.closing && s.Phase == account.DeleteFailed && !s.Busy {
		c.account.RequestClose(true)
	}
}

func contentCode(err error) catalog.Code {
	if errors.Is(err, context.DeadlineExceeded) {
		return catalog.Timeout
	}
	if errors.Is(err, context.Canceled) {
		return catalog.Canceled
	}
	var code catalog.Code
	if errors.As(err, &code) {
		switch code {
		case catalog.Invalid, catalog.Unsupported, catalog.Canceled, catalog.Timeout, catalog.Network, catalog.InvalidResponse, catalog.Unavailable, catalog.Restricted, catalog.LoginRequired, catalog.SessionExpired, catalog.VerificationRequired, catalog.RateLimited, catalog.ServiceUnavailable, catalog.QualityUnavailable, catalog.Closed:
			return code
		}
	}
	return catalog.Unavailable
}

// Include initial navigation queue wait without turning a request deadline into
// a video lifetime. Later explicit RetryDetail gets a fresh ordinary deadline.
type queuedDetailSource struct {
	source   app.Source
	deadline time.Time
	used     bool
}

func (s *queuedDetailSource) Detail(parent context.Context) (catalog.Video, error) {
	if s.used {
		return s.source.Detail(parent)
	}
	s.used = true
	ctx, cancel := context.WithDeadline(parent, s.deadline)
	defer cancel()
	return s.source.Detail(ctx)
}
func (s *queuedDetailSource) Resolve(ctx context.Context, request app.ResolveRequest) (app.Resolution, error) {
	return s.source.Resolve(ctx, request)
}

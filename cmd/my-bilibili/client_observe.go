package main

import (
	"my-bilibili/internal/adapters/mpv"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/playback"
	"my-bilibili/internal/previewbundle"
)

// Only this private snapshot contains product data. The e2e/public receipt
// mapping below exports fixed enums, booleans and numeric counters.
func (a *clientApplication) verificationSnapshot() verificationSnapshot {
	c := a.core
	s := verificationSnapshot{Account: c.account.Snapshot(), History: c.history.State(),
		Search: catalog.SearchState{Phase: catalog.SearchInitial}, Player: app.State{DetailPhase: app.DetailInitial, Playback: c.lastPlayback},
		Danmaku: danmaku.LoadState{Phase: danmaku.LoadInitial}, Navigation: c.navigation, ScopeRevision: c.config.service.State().Revision,
		PendingOwners: a.pendingOwners(), Resources: mpv.Owned(), Closing: c.closing, Closed: a.closed, Error: c.navigationError}
	if a.window != nil && !a.closed {
		s.WindowVisible = a.window.IsVisible() && !a.window.IsMinimized()
	}
	if a.shell != nil {
		s.PlayerMounted = a.shell.PlayerVisible()
	}
	api, cover := c.config.metrics.api.snapshot(), c.config.metrics.cover.snapshot()
	s.APIActive, s.CoverActive = api.active, cover.active
	if g := c.group; g != nil {
		if g.browser != nil {
			s.Search = g.browser.State()
		}
		if v := g.video; v != nil {
			s.Player = v.session.State()
			tracks := mpv.Inspect(v.session.Engine())
			s.VideoTrack, s.AudioTrack, s.ExpectedAudio = tracks.Video, tracks.Audio, v.expectedAudio
			s.PicturePresented = mpv.PicturePresented(v.session.Engine())
			if v.loader != nil {
				s.Danmaku = v.loader.State()
				s.DanmakuWindowAccepted = s.Danmaku.Phase == danmaku.LoadReady && v.windowRevision > 0 &&
					v.windowRevision == s.Danmaku.Revision && s.Danmaku.Generation == s.Player.Playback.Generation
			}
			frame := v.overlay.State()
			s.VisibleComments, s.TextLayouts = frame.Active, frame.Layouts
			s.FirstComment, s.HasFirstComment = v.firstComment, v.hasComment
		}
	}
	if c.closed {
		s.Search.Phase, s.Danmaku.Phase = catalog.SearchClosed, danmaku.LoadClosed
		// There is no Session when the user closes without opening a video.
		// core.closed proves no retained native owner in either path.
		s.Player.Playback.State = playback.Closed
	}
	return s
}

func (a *clientApplication) clientObservation() previewbundle.ClientObservation {
	s := a.verificationSnapshot()
	c := a.core
	service := c.config.service.State()
	source, search := c.config.metrics.source.snapshot(), c.config.metrics.search.snapshot()
	api, cover := c.config.metrics.api.snapshot(), c.config.metrics.cover.snapshot()
	o := previewbundle.ClientObservation{
		AccountPhase: s.Account.Phase, AccountError: s.Account.Error, AccountEpoch: s.Account.SessionEpoch, ScopeRevision: service.Revision,
		Navigation: c.navigation, Visit: c.visit, Authenticated: service.Authenticated, Stored: s.Account.Stored, Restored: s.Account.Restored,
		SearchPhase: s.Search.Phase, SearchError: s.Search.Error, SearchGeneration: s.Search.Generation, Results: len(s.Search.Results), Page: s.Search.Page, LoadingMore: s.Search.LoadingMore, AtLimit: s.Search.AtLimit,
		DetailPhase: s.Player.DetailPhase, DetailError: s.Player.DetailError, SelectionError: s.Player.SelectionError, PlaybackRetryError: s.Player.PlaybackRetryError, ResumeError: s.Player.ResumeError,
		Parts: len(s.Player.Video.Parts), Qualities: len(s.Player.Qualities), Resolving: s.Player.ResolvingPart != "", Resuming: s.Player.Resuming, DesiredPlaying: s.Player.DesiredPlaying,
		Playback: s.Player.Playback, VideoTrack: s.VideoTrack, AudioTrack: s.AudioTrack, PicturePresented: s.PicturePresented,
		SourceCalls: source.calls, SourceActive: source.active, SourcePeak: source.peak, SearchCalls: search.calls, SearchActive: search.active, SearchPeak: search.peak,
		APIRequests: api.calls, APIActive: api.active, APIPeak: api.peak, CoverRequests: cover.calls, CoverActive: cover.active,
		DanmakuPhase: s.Danmaku.Phase, DanmakuError: s.Danmaku.Error, DanmakuGeneration: s.Danmaku.Generation, DanmakuRevision: s.Danmaku.Revision,
		Segment: s.Danmaku.Segment, CachedSegments: s.Danmaku.CachedSegments, CachedEntries: s.Danmaku.Entries, CachedTextBytes: s.Danmaku.TextBytes, DanmakuActiveRequests: s.Danmaku.ActiveRequests, VisibleComments: s.VisibleComments, TextLayouts: s.TextLayouts,
		HistoryPhase: s.History.Phase, HistoryError: s.History.Error, HistoryCount: len(s.History.Records), HistoryRevision: s.History.Revision, HistorySavedRevision: s.History.SavedRevision, HistoryBusy: s.History.Busy, HistoryDirty: s.History.Dirty,
		PendingOwners: s.PendingOwners, Resources: s.Resources, Minimized: a.minimized, Closing: s.Closing, Closed: a.closed,
	}
	if a.window != nil && !a.closed {
		o.Minimized = a.window.IsMinimized()
	}
	for i, part := range s.Player.Video.Parts {
		if part.ID == s.Player.SelectedPart {
			o.SelectedPart = i + 1
		}
		if part.ID == s.Player.PlaybackPart {
			o.PlaybackPart = i + 1
		}
	}
	for i, quality := range s.Player.Qualities {
		if quality.ID == s.Player.SelectedQuality {
			o.SelectedQuality = i + 1
		}
		if quality.ID == s.Player.PlaybackQuality {
			o.PlaybackQuality = i + 1
		}
	}
	if g := c.group; g != nil && g.covers != nil {
		o.CoverRetained = g.covers.State().Cached
	}
	return o
}

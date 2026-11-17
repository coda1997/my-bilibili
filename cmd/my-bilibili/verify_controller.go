package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"my-bilibili/internal/account"
	"my-bilibili/internal/adapters/mpv"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/history"
	"my-bilibili/internal/playback"
	"my-bilibili/internal/previewbundle"
)

type verificationSnapshot struct {
	Account                                                 account.State
	Search                                                  catalog.SearchState
	Player                                                  app.State
	History                                                 history.State
	Danmaku                                                 danmaku.LoadState
	Navigation, ScopeRevision                               uint64
	VideoTrack, AudioTrack, ExpectedAudio, PicturePresented bool
	PlayerMounted, WindowVisible                            bool
	DanmakuWindowAccepted                                   bool
	VisibleComments                                         int
	FirstComment                                            time.Duration
	HasFirstComment                                         bool
	PendingOwners                                           int
	Resources                                               mpv.Resources
	APIActive, CoverActive, TextLayouts                     int
	Closing, Closed, ForcedCleanup                          bool
	Error                                                   catalog.Code
}

func (verificationSnapshot) Format(s fmt.State, _ rune) {
	_, _ = io.WriteString(s, "[private verification snapshot]")
}
func (verificationSnapshot) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

type verificationActions struct {
	Snapshot      func() verificationSnapshot
	Search        func(string) error
	Open          func(catalog.Entry, app.Options) error
	SelectPart    func(catalog.PartID) error
	SelectQuality func(app.QualityID) error
	Play, Pause   func() error
	Seek          func(time.Duration) error
	RequestClose  func()
}

// Only UI Tick owns the controller. Report transfers a copied bounded result
// to the existing off-UI finalizer; it never runs file work or joins an owner.
type verificationController struct {
	run                                                                               *verificationRun
	actions                                                                           verificationActions
	phase, stage                                                                      string
	stopped, reported, complete                                                       bool
	failure                                                                           string
	receipts                                                                          []previewbundle.LiveReceipt
	restored, stored, historyRestored, samePart, resumed, danmakuLoaded, historySaved bool
	epoch, revision, beforeSearch, beforeNavigation, navigation                       uint64
	video                                                                             catalog.VideoID
	ref                                                                               catalog.PartRef
	quality                                                                           app.QualityID
	previousGeneration, progressGeneration                                            uint64
	progressPosition                                                                  time.Duration
	progressSeen, resumeConfirmed                                                     bool
	resumeAt, replacementFloor                                                        time.Duration
	seekUsed, seekPending, seekNeedsProgress                                          bool
	seekAt                                                                            time.Duration
	seekSubmitted                                                                     time.Time
}

func (verificationController) Format(s fmt.State, _ rune) {
	_, _ = io.WriteString(s, "[private verification controller]")
}
func (verificationController) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

func newVerificationController(run *verificationRun, actions verificationActions) (*verificationController, error) {
	if run == nil || run.budget == nil || run.history == nil || actions.Snapshot == nil || actions.Search == nil || actions.Open == nil || actions.SelectPart == nil || actions.SelectQuality == nil || actions.Play == nil || actions.Pause == nil || actions.Seek == nil || actions.RequestClose == nil {
		return nil, catalog.Invalid
	}
	return &verificationController{run: run, actions: actions, phase: "restore", stage: "restore"}, nil
}
func (v *verificationController) Stopped() bool { return v.stopped }

func verificationError(err error) string {
	if err == nil {
		return ""
	}
	var content catalog.Code
	if errors.As(err, &content) {
		switch content {
		case catalog.None, catalog.Invalid, catalog.Unsupported, catalog.Canceled, catalog.Timeout, catalog.Network, catalog.InvalidResponse, catalog.Unavailable, catalog.Restricted, catalog.LoginRequired, catalog.SessionExpired, catalog.VerificationRequired, catalog.RateLimited, catalog.ServiceUnavailable, catalog.QualityUnavailable, catalog.Closed:
			return string(content)
		}
	}
	var login account.Code
	if errors.As(err, &login) {
		switch login {
		case account.None, account.Invalid, account.Network, account.ServiceRejected, account.ExpiredSession, account.Storage, account.Missing, account.Busy, account.Canceled, account.Timeout, account.Closed:
			return string(login)
		}
	}
	var stored history.Code
	if errors.As(err, &stored) {
		switch stored {
		case history.None, history.Invalid, history.Storage, history.InUse, history.LockUnsupported, history.Corrupt, history.UnsupportedVersion, history.Uncertain, history.Closed:
			return string(stored)
		}
	}
	var native playback.ErrorCode
	if errors.As(err, &native) {
		switch native {
		case playback.NoError, playback.Invalid, playback.Unsupported, playback.Canceled, playback.Timeout, playback.Media, playback.IsClosed:
			return string(native)
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return string(catalog.Timeout)
	}
	if errors.Is(err, context.Canceled) {
		return string(catalog.Canceled)
	}
	return string(catalog.Unavailable)
}

func verificationStateError(s verificationSnapshot) string {
	for _, err := range []error{s.Error, s.Account.Error, s.Search.Error, s.Player.DetailError, s.Player.SelectionError, s.Player.PlaybackRetryError, s.Player.ResumeError, s.Player.Playback.Error, s.History.Error, s.Danmaku.Error, s.Danmaku.PrefetchError} {
		if code := verificationError(err); code != "" {
			return code
		}
	}
	if s.Player.DetailPhase == app.DetailFailed || s.Player.Playback.State == playback.Failed || s.Search.Phase == catalog.SearchFailed || s.Danmaku.Phase == danmaku.LoadFailed {
		return string(catalog.Unavailable)
	}
	if s.History.Phase == history.Failed {
		return string(history.Storage)
	}
	switch s.Account.Phase {
	case account.Generating, account.Waiting, account.Scanned, account.Saving, account.SaveFailed, account.SigningOut, account.DeleteFailed:
		return string(account.ServiceRejected)
	case account.SignedOut:
		return string(catalog.LoginRequired)
	case account.Expired:
		return string(catalog.SessionExpired)
	case account.Failed, account.VerifyFailed:
		return string(account.ServiceRejected)
	}
	return ""
}

func (v *verificationController) receipt(s verificationSnapshot, stage, result, code string) previewbundle.LiveReceipt {
	counts := v.run.budget.snapshot()
	return previewbundle.LiveReceipt{Version: 1, Event: "stage", Stage: stage, Result: result, Error: code,
		AccountRestored: v.restored, Stored: v.stored, SearchCount: len(s.Search.Results), Parts: len(s.Player.Video.Parts), Qualities: len(s.Player.Qualities),
		VideoTrack: s.VideoTrack, AudioTrack: s.AudioTrack, PicturePresented: s.PicturePresented, NativeGeneration: s.Player.Playback.Generation,
		PositionMS: int64(s.Player.Playback.Position / time.Millisecond), DurationMS: int64(s.Player.Playback.Duration / time.Millisecond),
		DanmakuLoaded: v.danmakuLoaded, VisibleComments: s.VisibleComments, HistorySaved: v.historySaved, HistoryRestored: v.historyRestored, SamePart: v.samePart, Resumed: v.resumed,
		APIRequests: counts.APIRequests, BootstrapRequests: counts.BootstrapRequests, CoverRequests: counts.CoverRequests, MediaOpens: counts.MediaOpens,
		PendingOwners: s.PendingOwners, NativeCores: int(s.Resources.Cores), NativeRenderers: int(s.Resources.Renderers), NativeWorkers: int(s.Resources.Workers), ForcedCleanup: s.ForcedCleanup}
}
func (v *verificationController) mark(s verificationSnapshot, stage, result, code string) {
	v.receipts = append(v.receipts, v.receipt(s, stage, result, code))
}
func (v *verificationController) fail(s verificationSnapshot, code string) {
	if v.stopped {
		return
	}
	if code == "" {
		code = string(catalog.Unavailable)
	}
	v.failure = code
	v.mark(s, v.stage, "fail", code)
	v.stopped = true
	v.run.Stop()
	v.actions.RequestClose()
}
func (v *verificationController) action(s verificationSnapshot, action func() error) bool {
	if err := action(); err != nil {
		v.fail(s, verificationError(err))
		return false
	}
	return true
}

func verificationNativeBounds(s verificationSnapshot) bool {
	p := s.Player.Playback
	return p.Position >= 0 && p.Duration >= 0 && p.Duration <= 7*24*time.Hour && p.Position <= p.Duration && s.Resources.Cores >= 0 && s.Resources.Cores <= 1 && s.Resources.Renderers >= 0 && s.Resources.Renderers <= 1 && s.Resources.Workers >= 0 && s.Resources.Workers <= 2 && s.PendingOwners >= 0 && s.PendingOwners <= 9 && s.APIActive >= 0 && s.APIActive <= 4 && s.CoverActive >= 0 && s.CoverActive <= 1 && s.VisibleComments >= 0 && s.VisibleComments <= danmaku.MaxLanes && s.TextLayouts >= 0 && s.TextLayouts <= danmaku.MaxLanes
}

func (v *verificationController) Tick(now time.Time) {
	if v.stopped || v.reported {
		return
	}
	s := v.actions.Snapshot()
	if now.Before(v.run.started) || !now.Before(v.run.started.Add(verificationProcessLimit)) {
		v.fail(s, string(catalog.Timeout))
		return
	}
	if code := v.run.budget.snapshot().Error; code != catalog.None {
		v.fail(s, string(code))
		return
	}
	if code := verificationStateError(s); code != "" {
		v.fail(s, code)
		return
	}
	if code := v.run.history.proof().Error; code != history.None {
		v.fail(s, string(code))
		return
	}
	if !verificationNativeBounds(s) || s.ForcedCleanup {
		v.fail(s, string(catalog.InvalidResponse))
		return
	}
	if s.Closing || s.Closed {
		v.fail(s, string(catalog.Canceled))
		return
	}
	if v.restored && (s.Account.Phase != account.SignedIn || s.Account.SessionEpoch != v.epoch || s.ScopeRevision != v.revision) {
		v.fail(s, string(catalog.SessionExpired))
		return
	}
	if v.navigation != 0 && (s.Navigation != v.navigation || s.Player.Video.ID != v.video) {
		v.fail(s, string(catalog.InvalidResponse))
		return
	}
	switch v.phase {
	case "restore":
		v.restore(s)
	case "search":
		v.search(s)
	case "detail", "resume-detail":
		v.detail(s)
	case "play-initial", "play-part", "play-quality", "resume":
		v.play(s)
	case "danmaku":
		v.comments(s, now)
	case "history":
		v.saved(s)
	}
}

func (v *verificationController) restore(s verificationSnapshot) {
	if s.Account.Phase != account.SignedIn || !s.Account.Stored || !s.Account.Restored || s.Account.Unsaved || s.Account.Busy || s.Account.SessionEpoch == 0 || s.ScopeRevision == 0 || s.History.Phase != history.Ready || s.History.Busy || !v.run.history.proof().Loaded {
		return
	}
	v.restored, v.stored = true, true
	v.epoch, v.revision = s.Account.SessionEpoch, s.ScopeRevision
	v.mark(s, "restore", "pass", "")
	if v.run.plan.Launch == 1 {
		v.beforeSearch = s.Search.Generation
		v.phase, v.stage = "search", "search"
		v.action(s, func() error { return v.actions.Search(v.run.plan.Query) })
		return
	}
	c := v.run.continuation
	record, found := verificationRecord(s.History.Records, c.ref())
	if !found || record.Completed || record.Position != time.Duration(c.PositionMS)*time.Millisecond {
		v.fail(s, string(history.Corrupt))
		return
	}
	v.historyRestored = true
	v.video, v.ref, v.resumeAt = c.ref().Video, c.ref(), record.Position
	v.beforeNavigation = s.Navigation
	v.phase, v.stage = "resume-detail", "detail"
	v.action(s, func() error {
		return v.actions.Open(catalog.Entry{Kind: catalog.BVEntry, ID: string(v.video)}, app.Options{InitialPart: v.ref.Part, StartAt: v.resumeAt, Autoplay: true})
	})
}

func (v *verificationController) search(s verificationSnapshot) {
	if s.Search.Phase != catalog.SearchReady || s.Search.Generation <= v.beforeSearch {
		return
	}
	if s.Search.Query != v.run.plan.Query || s.Search.Page != 1 || s.Search.LoadingMore || len(s.Search.Results) > 1000 {
		v.fail(s, string(catalog.InvalidResponse))
		return
	}
	id := v.run.plan.VideoID
	if v.run.plan.SelectedResult != 0 {
		if v.run.plan.SelectedResult > len(s.Search.Results) {
			v.fail(s, string(catalog.Invalid))
			return
		}
		id = string(s.Search.Results[v.run.plan.SelectedResult-1].ID)
	}
	if !verificationBV(id) {
		v.fail(s, string(catalog.InvalidResponse))
		return
	}
	v.video = catalog.VideoID(id)
	v.mark(s, "search", "pass", "")
	v.beforeNavigation = s.Navigation
	v.phase, v.stage = "detail", "detail"
	v.action(s, func() error { return v.actions.Open(catalog.Entry{Kind: catalog.BVEntry, ID: id}, app.Options{}) })
}

func (v *verificationController) detail(s verificationSnapshot) {
	if s.Navigation <= v.beforeNavigation || s.Player.DetailPhase != app.DetailReady {
		return
	}
	if s.Player.Video.ID != v.video || len(s.Player.Video.Parts) == 0 || len(s.Player.Video.Parts) > 1000 {
		v.fail(s, string(catalog.InvalidResponse))
		return
	}
	v.navigation = s.Navigation
	if v.phase == "resume-detail" {
		found := false
		for _, part := range s.Player.Video.Parts {
			if part.ID == v.ref.Part {
				found = true
				break
			}
		}
		if !found || s.Player.SelectedPart != v.ref.Part {
			v.fail(s, string(catalog.Invalid))
			return
		}
		v.mark(s, "detail", "pass", "")
		v.phase, v.stage = "resume", "resume"
		return
	}
	v.ref = catalog.PartRef{Video: v.video, Part: s.Player.Video.Parts[0].ID}
	if s.Player.SelectedPart != v.ref.Part || v.run.history.allow(v.ref) != nil {
		v.fail(s, string(catalog.InvalidResponse))
		return
	}
	v.mark(s, "detail", "pass", "")
	v.phase, v.stage = "play-initial", "play"
	v.action(s, v.actions.Play)
}

func (v *verificationController) playing(s verificationSnapshot) bool {
	p := s.Player
	n := p.Playback
	if p.Video.ID != v.ref.Video || p.PlaybackPart != v.ref.Part || p.ResolvingPart != "" || p.Resuming || n.Generation == 0 || n.Generation <= v.previousGeneration || n.State != playback.Playing || n.Duration <= 0 || n.Position <= 0 || n.Position >= n.Duration || !p.DesiredPlaying || len(p.Qualities) == 0 || len(p.Qualities) > 16 || p.PlaybackQuality == "" || p.SelectedQuality != p.PlaybackQuality || v.quality != "" && p.PlaybackQuality != v.quality || !s.VideoTrack || s.AudioTrack != s.ExpectedAudio || !s.PicturePresented || !s.PlayerMounted || !s.WindowVisible || s.Resources.Cores != 1 || s.Resources.Renderers != 1 || s.Resources.Workers == 0 {
		return false
	}
	member := false
	for _, quality := range p.Qualities {
		if quality.ID == p.PlaybackQuality {
			member = true
			break
		}
	}
	if !member {
		return false
	}
	if !v.progressSeen || v.progressGeneration != n.Generation {
		v.progressSeen, v.progressGeneration, v.progressPosition = true, n.Generation, n.Position
		return false
	}
	return n.Position > v.progressPosition
}

func (v *verificationController) play(s verificationSnapshot) {
	n := s.Player.Playback
	if v.phase == "resume" && !v.resumeConfirmed && s.Player.PlaybackPart == v.ref.Part && n.Generation != 0 && !s.Player.Resuming && (n.State == playback.Ready || n.State == playback.Paused || n.State == playback.Playing) {
		if n.Duration <= v.resumeAt || n.Position < v.resumeAt-250*time.Millisecond || n.Position > v.resumeAt+250*time.Millisecond {
			v.fail(s, string(catalog.InvalidResponse))
			return
		}
		v.resumeConfirmed = true
	}
	if v.phase == "resume" && !v.resumeConfirmed {
		return
	}
	if v.phase == "play-quality" && n.Generation > v.previousGeneration && !s.Player.Resuming && n.State == playback.Playing && n.Position < v.replacementFloor-250*time.Millisecond {
		v.fail(s, string(catalog.InvalidResponse))
		return
	}
	if !v.playing(s) {
		return
	}
	phase := v.phase
	if phase == "resume" {
		v.samePart, v.resumed = true, true
	}
	v.mark(s, v.stage, "pass", "")
	switch phase {
	case "play-initial":
		v.part(s)
	case "play-part":
		v.chooseQuality(s)
	case "play-quality":
		v.phase, v.stage = "danmaku", "danmaku"
	case "resume":
		v.pauseForHistory(s)
	}
}

func (v *verificationController) part(s verificationSnapshot) {
	page := v.run.plan.PartPage
	if page > len(s.Player.Video.Parts) {
		v.fail(s, string(catalog.Invalid))
		return
	}
	if page == 0 || s.Player.Video.Parts[page-1].ID == v.ref.Part {
		v.mark(s, "play", "not-tested", "")
		v.chooseQuality(s)
		return
	}
	v.ref.Part = s.Player.Video.Parts[page-1].ID
	if v.run.history.allow(v.ref) != nil {
		v.fail(s, string(history.Invalid))
		return
	}
	v.previousGeneration, v.progressSeen = s.Player.Playback.Generation, false
	v.phase = "play-part"
	v.action(s, func() error { return v.actions.SelectPart(v.ref.Part) })
}
func (v *verificationController) chooseQuality(s verificationSnapshot) {
	index := v.run.plan.QualityIndex
	if index > len(s.Player.Qualities) {
		v.fail(s, string(catalog.Invalid))
		return
	}
	if index == 0 || s.Player.Qualities[index-1].ID == s.Player.PlaybackQuality {
		v.mark(s, "play", "not-tested", "")
		v.phase, v.stage = "danmaku", "danmaku"
		return
	}
	v.quality = s.Player.Qualities[index-1].ID
	v.previousGeneration, v.progressSeen = s.Player.Playback.Generation, false
	v.replacementFloor = s.Player.Playback.Position
	v.phase = "play-quality"
	v.action(s, func() error { return v.actions.SelectQuality(v.quality) })
}

func (v *verificationController) comments(s verificationSnapshot, now time.Time) {
	if s.Danmaku.Phase != danmaku.LoadReady || s.Danmaku.Ref != v.ref || s.Danmaku.Generation != s.Player.Playback.Generation || s.Danmaku.Segment <= 0 || s.Danmaku.CachedSegments < 1 || !s.DanmakuWindowAccepted {
		return
	}
	if s.Danmaku.CachedSegments > 2 || s.Danmaku.Entries < 0 || s.Danmaku.Entries > danmaku.MaxEntries || s.Danmaku.TextBytes < 0 || s.Danmaku.TextBytes > danmaku.MaxWindowBytes || s.VisibleComments > s.Danmaku.Entries {
		v.fail(s, string(catalog.InvalidResponse))
		return
	}
	if v.seekPending {
		if !now.Before(v.seekSubmitted.Add(5 * time.Second)) {
			v.fail(s, string(catalog.Timeout))
			return
		}
		position := s.Player.Playback.Position
		if now.Before(v.seekSubmitted.Add(playback.PollInterval)) || position < v.seekAt-250*time.Millisecond || position > v.seekAt+250*time.Millisecond {
			return
		}
		v.seekPending, v.seekNeedsProgress, v.progressSeen = false, true, false
	}
	if v.seekNeedsProgress {
		if !v.playing(s) {
			return
		}
		v.seekNeedsProgress = false
	}
	if s.VisibleComments > 0 && v.playing(s) {
		v.danmakuLoaded = true
		v.mark(s, "danmaku", "pass", "")
		v.pauseForHistory(s)
		return
	}
	if !v.seekUsed && s.HasFirstComment && s.VisibleComments == 0 {
		if s.FirstComment < 0 || s.FirstComment >= s.Player.Playback.Duration {
			v.fail(s, string(catalog.InvalidResponse))
			return
		}
		v.seekUsed, v.seekPending, v.seekAt, v.seekSubmitted = true, true, s.FirstComment, now
		v.action(s, func() error { return v.actions.Seek(v.seekAt) })
	}
}
func (v *verificationController) pauseForHistory(s verificationSnapshot) {
	v.phase, v.stage = "history", "history"
	v.action(s, v.actions.Pause)
}
func (v *verificationController) saved(s verificationSnapshot) {
	if s.Player.PlaybackPart != v.ref.Part || s.Player.Playback.State != playback.Paused || s.Player.DesiredPlaying || s.History.Phase != history.Ready || s.History.Busy || s.History.Dirty || s.History.Revision == 0 || s.History.SavedRevision != s.History.Revision {
		return
	}
	proof := v.run.history.proof()
	record, found := verificationRecord(proof.Records, v.ref)
	if !proof.Saved || !found || record.Completed || record.Position <= 0 || record.Position >= record.Duration || record.Position != s.Player.Playback.Position/time.Millisecond*time.Millisecond || record.Duration != s.Player.Playback.Duration/time.Millisecond*time.Millisecond {
		return
	}
	v.historySaved = true
	v.mark(s, "history", "pass", "")
	v.complete, v.stopped = true, true
	v.run.Stop()
	v.actions.RequestClose()
}

func verificationClosed(s verificationSnapshot) bool {
	return s.Closed && !s.ForcedCleanup && s.PendingOwners == 0 && s.Resources == (mpv.Resources{}) && s.APIActive == 0 && s.CoverActive == 0 && s.Danmaku.ActiveRequests == 0 && s.Danmaku.CachedSegments == 0 && s.Danmaku.Entries == 0 && s.Danmaku.TextBytes == 0 && !s.DanmakuWindowAccepted && s.TextLayouts == 0 && s.VisibleComments == 0 && !s.VideoTrack && !s.AudioTrack && !s.PicturePresented && s.Player.ResolvingPart == "" && !s.Player.Resuming && !s.Player.DesiredPlaying && (s.Player.Playback.State == playback.Closed || s.Player.Video.ID == "" && s.Player.Playback.Generation == 0 && (s.Player.Playback.State == "" || s.Player.Playback.State == playback.Idle))
}

type verificationReport struct {
	receipts         []previewbundle.LiveReceipt
	ref              catalog.PartRef
	complete, closed bool
	failure          string
}

func (verificationReport) Format(s fmt.State, _ rune) {
	_, _ = io.WriteString(s, "[private verification report]")
}
func (verificationReport) MarshalJSON() ([]byte, error) { return []byte("null"), nil }
func (v *verificationController) Report(s verificationSnapshot) verificationReport {
	if v.reported {
		return verificationReport{failure: string(catalog.Closed)}
	}
	v.reported, v.stopped = true, true
	v.run.Stop()
	code := v.failure
	if code == "" {
		for _, err := range []error{s.Error, s.History.Error, s.Player.Playback.Error} {
			if value := verificationError(err); value != "" {
				code = value
				break
			}
		}
	}
	if code == "" && (!v.complete || !verificationClosed(s)) {
		code = string(catalog.Unavailable)
	}
	result := "pass"
	if code != "" {
		result = "fail"
	}
	closed := v.receipt(s, "close", result, code)
	closed.Event = "closed"
	receipts := append([]previewbundle.LiveReceipt(nil), v.receipts...)
	receipts = append(receipts, closed)
	return verificationReport{receipts: receipts, ref: v.ref, complete: v.complete, closed: verificationClosed(s), failure: code}
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

type verifyMachine struct {
	t                                                               *testing.T
	r                                                               *verificationRun
	h                                                               *verificationHistory
	store                                                           *verifyHistorySpy
	v                                                               *verificationController
	s                                                               verificationSnapshot
	now                                                             time.Time
	searches, opens, parts, qualities, plays, pauses, seeks, closes int
	entry                                                           catalog.Entry
	options                                                         app.Options
	seek                                                            time.Duration
	engines                                                         []*verificationFakeEngine
	generation                                                      uint64
}

func newVerifyMachine(t *testing.T, r *verificationRun) *verifyMachine {
	t.Helper()
	h, spy := verifyTestOwnedHistory(t, r)
	if h.Acquire() != nil {
		t.Fatal("machine Store acquire failed")
	}
	records, err := h.Load()
	if err != nil {
		t.Fatal("machine Store load failed")
	}
	return verifyMachineForSnapshot(t, r, spy, records)
}

// Playback and navigation cases start after history has loaded. Only the
// filesystem integration tests acquire a Store and establish this proof via I/O.
func newVerifyActionMachine(t *testing.T, fields map[string]any) *verifyMachine {
	t.Helper()
	plan, err := parseVerificationPlan(verifyTestJSON(t, fields))
	if err != nil {
		t.Fatal(err)
	}
	r := verifyTestBudget(t, verificationCounts{}, plan.Launch)
	r.plan = plan
	r.runDirectory = t.TempDir()
	r.continuationPath = filepath.Join(r.runDirectory, "continuation.json")
	spy := &verifyHistorySpy{}
	if _, err := r.HistoryStore(spy); err != nil {
		t.Fatal(err)
	}
	r.history.last.Loaded = true
	return verifyMachineForSnapshot(t, r, spy, nil)
}

func verifyMachineForSnapshot(t *testing.T, r *verificationRun, spy *verifyHistorySpy, records []history.Record) *verifyMachine {
	t.Helper()
	m := &verifyMachine{t: t, r: r, h: r.history, store: spy, now: r.started.Add(time.Millisecond)}
	r.now = func() time.Time { return m.now }
	m.s = verificationSnapshot{Account: account.State{Phase: account.SignedIn, Stored: true, Restored: true, SessionEpoch: 1}, Search: catalog.SearchState{Phase: catalog.SearchInitial}, History: history.State{Phase: history.Ready, Records: records}, ScopeRevision: 1, PlayerMounted: true, WindowVisible: true}
	actions := verificationActions{Snapshot: func() verificationSnapshot { return m.s },
		Search: func(query string) error {
			m.searches++
			m.s.Search = catalog.SearchState{Phase: catalog.SearchReady, Query: query, Generation: 1, Page: 1, Results: []catalog.Summary{{ID: verifyTestVideo, Title: verifyTestPrivate}}}
			return nil
		},
		Open: func(entry catalog.Entry, options app.Options) error {
			m.opens++
			m.entry, m.options = entry, options
			m.s.Navigation++
			m.s.Player = app.State{DetailPhase: app.DetailReady, Video: catalog.Video{ID: catalog.VideoID(entry.ID), Title: verifyTestPrivate, Creator: "Original creator", Parts: []catalog.Part{{ID: "101", Title: "Original first", Duration: time.Minute}, {ID: "102", Title: "Original second", Duration: time.Minute}}}, SelectedPart: "101", Volume: 1, Rate: 1}
			if options.InitialPart != "" {
				m.s.Player.SelectedPart = options.InitialPart
			}
			if options.Autoplay {
				m.native(m.s.Player.SelectedPart, "80", 0)
				m.s.Player.Resuming = true
			}
			return nil
		},
		SelectPart: func(part catalog.PartID) error { m.parts++; m.native(part, "80", 0); return nil },
		SelectQuality: func(quality app.QualityID) error {
			m.qualities++
			position := m.s.Player.Playback.Position
			m.native(m.s.Player.PlaybackPart, quality, position)
			return nil
		},
		Play: func() error { m.plays++; m.native(m.s.Player.SelectedPart, "80", 0); return nil },
		Pause: func() error {
			m.pauses++
			m.s.Player.DesiredPlaying = false
			m.s.Player.Playback.State = playback.Paused
			return nil
		},
		Seek:         func(at time.Duration) error { m.seeks++; m.seek = at; return nil },
		RequestClose: func() { m.closes++ },
	}
	v, err := newVerificationController(r, actions)
	if err != nil {
		t.Fatal("machine controller constructor failed")
	}
	m.v = v
	return m
}
func (m *verifyMachine) native(part catalog.PartID, quality app.QualityID, position time.Duration) {
	if len(m.engines) > 0 {
		if m.engines[len(m.engines)-1].Close() != nil {
			m.t.Fatal("synthetic old engine close failed")
		}
	}
	e := &verificationFakeEngine{}
	opener := m.r.Opener(verificationOpenFunc(func(context.Context, playback.Input) (playback.Engine, error) { return e, nil }))
	if _, err := opener.Open(context.Background(), playback.Input{URL: "https://video.example.test/synthetic-only.m4s", AudioURL: "https://audio.example.test/synthetic-only.m4s"}); err != nil {
		m.t.Fatal("machine exceeded admitted native budget")
	}
	m.engines = append(m.engines, e)
	m.generation++
	m.s.Player.SelectedPart, m.s.Player.PlaybackPart = part, part
	m.s.Player.SelectedQuality, m.s.Player.PlaybackQuality = quality, quality
	m.s.Player.Qualities = []app.Quality{{ID: "80", Label: "1080P", Width: 1920, Height: 1080}, {ID: "64", Label: "720P", Width: 1280, Height: 720}}
	m.s.Player.Playback = playback.Snapshot{Generation: m.generation, State: playback.Paused, Position: position, Duration: 30666 * time.Millisecond, Rate: 1, Volume: 1}
	m.s.Player.DesiredPlaying = true
	m.s.Player.Resuming = false
	m.s.VideoTrack, m.s.AudioTrack, m.s.ExpectedAudio, m.s.PicturePresented = true, true, true, false
	m.s.Resources = mpv.Resources{Cores: 1, Renderers: 1, Workers: 1}
	m.s.Danmaku = danmaku.LoadState{}
	m.s.DanmakuWindowAccepted = false
	m.s.VisibleComments, m.s.TextLayouts = 0, 0
}
func (m *verifyMachine) tick() { m.now = m.now.Add(100 * time.Millisecond); m.v.Tick(m.now) }
func (m *verifyMachine) playing(position time.Duration) {
	m.s.Player.Playback.State = playback.Playing
	m.s.Player.Playback.Position = position
	m.s.PicturePresented = true
	m.tick()
}
func (m *verifyMachine) beginFirst() {
	m.tick()
	m.tick()
	m.tick()
	if m.v.phase != "play-initial" {
		m.t.Fatal("first launch did not reach explicit Play")
	}
}
func (m *verifyMachine) firstProof() { m.playing(time.Second); m.playing(1200 * time.Millisecond) }
func (m *verifyMachine) comments(visible int) {
	m.s.Danmaku = danmaku.LoadState{Phase: danmaku.LoadReady, Ref: catalog.PartRef{Video: m.s.Player.Video.ID, Part: m.s.Player.PlaybackPart}, Generation: m.s.Player.Playback.Generation, Revision: 1, Segment: 1, CachedSegments: 1, Entries: 1, TextBytes: 12}
	m.s.DanmakuWindowAccepted = true
	m.s.VisibleComments, m.s.TextLayouts = visible, visible
}
func (m *verifyMachine) save() {
	record := verifyTestRecord()
	record.Ref = catalog.PartRef{Video: m.s.Player.Video.ID, Part: m.s.Player.PlaybackPart}
	record.Position = m.s.Player.Playback.Position
	record.Duration = m.s.Player.Playback.Duration
	record.UpdatedAt = m.now.UTC()
	rows := append([]history.Record(nil), m.s.History.Records...)
	replaced := false
	for i := range rows {
		if rows[i].Ref == record.Ref {
			rows[i] = record
			replaced = true
			break
		}
	}
	if !replaced {
		rows = append(rows, record)
	}
	if m.h.Save(rows) != nil {
		m.t.Fatal("machine checkpoint failed")
	}
	m.s.History.Records = m.h.proof().Records
	m.s.History.Revision++
	m.s.History.SavedRevision = m.s.History.Revision
	m.s.History.Busy, m.s.History.Dirty = false, false
}
func (m *verifyMachine) closeActual() error {
	if len(m.engines) > 0 {
		if m.engines[len(m.engines)-1].Close() != nil {
			m.t.Fatal("synthetic current engine close failed")
		}
	}
	err := m.h.Close()
	m.s.Closing = true
	m.s.Closed = err == nil
	m.s.PendingOwners = 0
	if err != nil {
		m.s.PendingOwners = 1
		m.s.History.Phase = history.Failed
		m.s.History.Error = history.Storage
	} else {
		m.s.History.Phase = history.Done
	}
	m.s.Account.Phase = account.Done
	m.s.Account.SessionEpoch = 0
	m.s.Search.Phase = catalog.SearchClosed
	m.s.Search.Results = nil
	m.s.Player.Playback.State = playback.Closed
	m.s.Player.DesiredPlaying, m.s.Player.Resuming = false, false
	m.s.Player.ResolvingPart = ""
	m.s.Resources = mpv.Resources{}
	m.s.VideoTrack, m.s.AudioTrack, m.s.PicturePresented = false, false, false
	m.s.WindowVisible, m.s.PlayerMounted = false, false
	m.s.Danmaku = danmaku.LoadState{Phase: danmaku.LoadClosed}
	m.s.DanmakuWindowAccepted = false
	m.s.VisibleComments, m.s.TextLayouts = 0, 0
	return err
}
func (m *verifyMachine) finish() ([]previewbundle.LiveReceipt, []byte, error) {
	report := m.v.Report(m.s)
	var output bytes.Buffer
	err := m.r.Finish(report, &output)
	rows, parseErr := previewbundle.ParseLiveReceipts(output.Bytes())
	if parseErr != nil {
		m.t.Fatal("machine emitted malformed/incomplete receipt stream")
	}
	return rows, output.Bytes(), err
}
func (m *verifyMachine) completeFirst() {
	m.beginFirst()
	m.firstProof()
	if m.v.phase == "play-part" {
		m.playing(time.Second)
		m.playing(1200 * time.Millisecond)
	}
	if m.v.phase == "play-quality" {
		m.playing(1300 * time.Millisecond)
		m.playing(1400 * time.Millisecond)
	}
	if m.v.phase != "danmaku" {
		m.t.Fatal("fixed bounded replacements did not finish")
	}
	m.comments(1)
	m.tick()
	if m.v.phase != "history" || m.pauses != 1 {
		m.t.Fatal("danmaku acceptance did not pause for observed progress")
	}
	m.save()
	m.tick()
	if !m.v.Stopped() || !m.v.complete || m.closes != 1 {
		m.t.Fatal("useful saved progress did not request exactly one close")
	}
}
func (m *verifyMachine) completeSecond() {
	m.tick()
	m.tick()
	if m.v.phase != "resume" {
		m.t.Fatal("second launch did not use private exact Continue")
	}
	m.s.Player.Resuming = false
	m.s.Player.Playback.Position = m.options.StartAt
	m.s.Player.Playback.State = playback.Paused
	m.tick()
	m.playing(m.options.StartAt + 100*time.Millisecond)
	m.playing(m.options.StartAt + 200*time.Millisecond)
	if m.v.phase != "history" || !m.v.resumed {
		m.t.Fatal("same-part confirmed progression did not prove resume")
	}
	m.save()
	m.tick()
	if !m.v.complete || m.closes != 1 {
		m.t.Fatal("second progress did not close")
	}
}

func TestVerificationFixedTwoLaunchFlowUsesExactCheckpointAndSafeReceipts(t *testing.T) {
	f := verifyTestPaths(t)
	p := verifyTestPlan(1)
	p["query"] = verifyTestPrivate
	p["part_page"], p["quality_index"] = 2, 2
	f.writePlan(t, p)
	first := newVerifyMachine(t, f.prepare(t))
	first.completeFirst()
	if first.opens != 1 || first.searches != 1 || first.plays != 1 || first.parts != 1 || first.qualities != 1 || first.r.budget.snapshot().MediaOpens != 3 {
		t.Fatal("first launch actions or attempt bounds changed")
	}
	if first.closeActual() != nil {
		t.Fatal("first actual cleanup failed")
	}
	rows, output, err := first.finish()
	if err != nil || rows[len(rows)-1].Result != "pass" {
		t.Fatal("first complete loop failed")
	}
	for _, e := range first.engines {
		if e.closed != 1 {
			t.Fatal("first native engines were not actually retired")
		}
	}
	if bytes.Contains(output, []byte(verifyTestPrivate)) || bytes.Contains(output, []byte(verifyTestVideo)) || bytes.Contains(output, []byte("https://")) {
		t.Fatal("public receipts leaked identity/content/location")
	}
	data, err := os.ReadFile(first.r.continuationPath)
	if err != nil || len(data) > 4096 {
		t.Fatal("owned continuation missing or too large")
	}
	c, err := parseVerificationContinuation(data)
	if err != nil || c.PartID != "102" || c.PositionMS != 1400 || c.MediaOpens != 3 || bytes.Contains(data, []byte(verifyTestPrivate)) {
		t.Fatal("continuation did not use exact last successfully saved progress")
	}
	info, err := os.Stat(first.r.continuationPath)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("continuation was not private")
	}
	p = verifyTestPlan(2)
	p["video_id"] = "BV0000000002"
	p["part_page"], p["quality_index"] = 1000, 16
	f.writePlan(t, p)
	second := newVerifyMachine(t, f.prepare(t))
	second.completeSecond()
	if second.searches != 0 || second.opens != 1 || second.plays != 0 || second.parts != 0 || second.qualities != 0 || second.entry.ID != verifyTestVideo || second.options.InitialPart != "102" || second.options.StartAt != 1400*time.Millisecond || !second.options.Autoplay || second.r.budget.snapshot().MediaOpens != 4 {
		t.Fatal("launch2 substituted plan/latest row or reset whole-plan budget")
	}
	if second.closeActual() != nil {
		t.Fatal("second cleanup failed")
	}
	rows, _, err = second.finish()
	if err != nil || !rows[len(rows)-1].HistoryRestored || !rows[len(rows)-1].SamePart || !rows[len(rows)-1].Resumed || rows[len(rows)-1].Result != "pass" {
		t.Fatal("second close lost actual resume evidence")
	}
	if _, err := os.Stat(second.r.continuationPath); !os.IsNotExist(err) || !verificationDirectory(second.r.runDirectory) {
		t.Fatal("successful second launch removed wrong ownership marker")
	}
	if run, err := prepareVerification(f.planPath, f.directory, f.executable); err == nil || run != nil {
		t.Fatal("completed run reused")
	}
}

func TestVerificationPlayRequiresPictureVisibilityExpectedAudioAndProgress(t *testing.T) {
	for _, kind := range []string{"no-picture", "not-mounted", "minimized", "missing-audio", "unexpected-audio", "no-progress", "no-core", "stale-generation"} {
		t.Run(kind, func(t *testing.T) {
			m := newVerifyActionMachine(t, verifyTestPlan(1))
			m.beginFirst()
			for i := 0; i < 3; i++ {
				m.s.Player.Playback.State = playback.Playing
				m.s.Player.Playback.Position = time.Second + time.Duration(i)*100*time.Millisecond
				m.s.PicturePresented = true
				switch kind {
				case "no-picture":
					m.s.PicturePresented = false
				case "not-mounted":
					m.s.PlayerMounted = false
				case "minimized":
					m.s.WindowVisible = false
				case "missing-audio":
					m.s.AudioTrack = false
				case "unexpected-audio":
					m.s.ExpectedAudio = false
				case "no-progress":
					m.s.Player.Playback.Position = time.Second
				case "no-core":
					m.s.Resources.Cores = 0
				case "stale-generation":
					m.s.Player.Playback.Generation = 0
				}
				m.tick()
			}
			if m.v.phase != "play-initial" || m.pauses != 0 || m.v.complete {
				t.Fatal("tracks/time or command intent replaced actual picture/progress proof")
			}
			m.now = m.r.started.Add(verificationProcessLimit)
			m.v.Tick(m.now)
			if !m.v.Stopped() || m.closes != 1 || m.v.failure != string(catalog.Timeout) {
				t.Fatal("missing native proof escaped bounded stop")
			}
		})
	}
}

func TestVerificationOptionalIndicesAndRefusalsNeverSubstituteOrRetry(t *testing.T) {
	for _, kind := range []string{"result-index", "part-index", "quality-index", "refusal", "epoch", "navigation", "unknown-action-error"} {
		t.Run(kind, func(t *testing.T) {
			p := verifyTestPlan(1)
			if kind == "result-index" {
				p["video_id"], p["selected_result"] = "", 2
			}
			if kind == "part-index" {
				p["part_page"] = 3
			}
			if kind == "quality-index" {
				p["quality_index"] = 3
			}
			m := newVerifyActionMachine(t, p)
			if kind == "unknown-action-error" {
				m.v.actions.Search = func(string) error { return errors.New(verifyTestPrivate) }
			}
			m.tick()
			m.tick()
			if kind != "result-index" && kind != "unknown-action-error" {
				m.tick()
				switch kind {
				case "refusal":
					m.s.Player.SelectionError = catalog.Restricted
				case "epoch":
					m.s.Account.SessionEpoch++
				case "navigation":
					m.s.Navigation++
				}
				m.firstProof()
			}
			if !m.v.Stopped() || m.closes != 1 || m.v.failure == "" || strings.Contains(m.v.failure, verifyTestPrivate) {
				t.Fatal("unavailable selection/refusal/obsolete state did not stop safely")
			}
			before := m.r.budget.snapshot().MediaOpens
			m.tick()
			m.tick()
			if m.closes != 1 || m.r.budget.snapshot().MediaOpens != before {
				t.Fatal("stopped plan retried actions or media")
			}
		})
	}
}

func TestVerificationDanmakuSeekRequiresObservedPositionPictureAndNewProgress(t *testing.T) {
	m := newVerifyActionMachine(t, verifyTestPlan(1))
	m.beginFirst()
	m.firstProof()
	m.comments(0)
	m.s.HasFirstComment = true
	m.s.FirstComment = 5 * time.Second
	m.tick()
	if m.seeks != 1 || m.seek != 5*time.Second || m.v.phase != "danmaku" {
		t.Fatal("fixed first-comment seek was not bounded and explicit")
	}
	m.s.VisibleComments = 1
	m.s.TextLayouts = 1
	m.s.Player.Playback.Position += 100 * time.Millisecond
	m.tick()
	m.s.Player.Playback.Position += 100 * time.Millisecond
	m.tick()
	if m.pauses != 0 {
		t.Fatal("requested seek target accepted before native position")
	}
	m.s.Player.Playback.Position = 5 * time.Second
	m.s.PicturePresented = false
	m.tick()
	m.s.Player.Playback.Position += 100 * time.Millisecond
	m.tick()
	if m.pauses != 0 {
		t.Fatal("seek advanced without current painted picture")
	}
	m.s.PicturePresented = true
	m.tick()
	m.s.Player.Playback.Position += 100 * time.Millisecond
	m.tick()
	if m.seeks != 1 || m.pauses != 1 || !m.v.danmakuLoaded {
		t.Fatal("confirmed seek/paint/progression did not accept actual visible comments")
	}
	for _, kind := range []string{"bad-time", "unconfirmed", "old-window"} {
		m := newVerifyActionMachine(t, verifyTestPlan(1))
		m.beginFirst()
		m.firstProof()
		m.comments(0)
		m.s.HasFirstComment = true
		m.s.FirstComment = 5 * time.Second
		if kind == "bad-time" {
			m.s.FirstComment = m.s.Player.Playback.Duration
		}
		if kind == "old-window" {
			m.s.Danmaku.Generation++
		}
		m.tick()
		if kind == "unconfirmed" {
			m.now = m.now.Add(5 * time.Second)
			m.v.Tick(m.now)
		}
		if kind == "old-window" {
			if m.seeks != 0 || m.v.danmakuLoaded {
				t.Fatal("old window supplied a live seek or proof")
			}
		} else if !m.v.Stopped() || m.pauses != 0 {
			t.Fatal("invalid/unconfirmed seek became successful comment proof")
		}
	}
}

func TestVerificationResumeRejectsNoPictureZeroOrWrongPart(t *testing.T) {
	for _, kind := range []string{"no-picture", "zero-position", "wrong-part"} {
		t.Run(kind, func(t *testing.T) {
			r, f := verifyTestRun(t)
			first := newVerifyMachine(t, r)
			first.completeFirst()
			if first.closeActual() != nil {
				t.Fatal("first cleanup failed")
			}
			if _, _, err := first.finish(); err != nil {
				t.Fatal("first fixture finalization failed")
			}
			f.writePlan(t, verifyTestPlan(2))
			m := newVerifyMachine(t, f.prepare(t))
			m.tick()
			m.tick()
			m.s.Player.Resuming = false
			if kind != "zero-position" {
				m.s.Player.Playback.Position = m.options.StartAt
				m.s.Player.Playback.State = playback.Paused
				m.tick()
			}
			for i := 1; i <= 3; i++ {
				m.s.Player.Playback.State = playback.Playing
				m.s.Player.Playback.Position = m.options.StartAt + time.Duration(i)*100*time.Millisecond
				m.s.PicturePresented = true
				switch kind {
				case "no-picture":
					m.s.PicturePresented = false
				case "zero-position":
					m.s.Player.Playback.Position = time.Duration(i) * 100 * time.Millisecond
				case "wrong-part":
					m.s.Player.PlaybackPart = "999"
				}
				m.tick()
			}
			if m.v.resumed || m.v.complete || m.pauses != 0 {
				t.Fatal("resume passed without actual same-part position/picture")
			}
			if !m.v.Stopped() {
				m.now = m.r.started.Add(verificationProcessLimit)
				m.v.Tick(m.now)
			}
			if m.closes != 1 {
				t.Fatal("failed resume did not stop exactly once")
			}
			if _, err := os.Stat(m.r.continuationPath); err != nil {
				t.Fatal("failed second launch destroyed private continuation")
			}
		})
	}
}

type verificationFailWriter struct{}

func (verificationFailWriter) Write([]byte) (int, error) { return 0, errors.New(verifyTestPrivate) }

func TestVerificationFinalizationPreservesCleanupDiskAndReceiptFailures(t *testing.T) {
	for _, kind := range []string{"pending-owner", "native-resource", "old-picture", "old-window", "store-close", "continuation-write", "deadline", "writer"} {
		t.Run(kind, func(t *testing.T) {
			r, _ := verifyTestRun(t)
			m := newVerifyMachine(t, r)
			m.completeFirst()
			if kind == "store-close" {
				m.store.closeErr = errors.New(verifyTestPrivate)
			}
			closeErr := m.closeActual()
			if kind != "store-close" && closeErr != nil {
				t.Fatal("fixture close failed")
			}
			switch kind {
			case "pending-owner":
				m.s.PendingOwners = 1
			case "native-resource":
				m.s.Resources.Workers = 1
			case "old-picture":
				m.s.PicturePresented = true
			case "old-window":
				m.s.DanmakuWindowAccepted = true
			case "continuation-write":
				r.writeContinuation = func(string, []byte) error { return errors.New(verifyTestPrivate) }
			case "deadline":
				m.now = r.started.Add(verificationProcessLimit)
			}
			report := m.v.Report(m.s)
			var output bytes.Buffer
			var writer io.Writer = &output
			if kind == "writer" {
				writer = verificationFailWriter{}
			}
			if err := r.Finish(report, writer); err == nil {
				t.Fatal("cleanup/disk/receipt uncertainty became successful verification")
			} else {
				verifyTestSafe(t, err)
			}
			if kind != "writer" {
				rows, err := previewbundle.ParseLiveReceipts(output.Bytes())
				if err != nil || rows[len(rows)-1].Result != "fail" {
					t.Fatal("final failure was not retained in strict closed receipt")
				}
			}
			if r.Finish(report, &bytes.Buffer{}) != catalog.Closed {
				t.Fatal("finalization retried an owned completion")
			}
		})
	}
}

func TestVerificationSecondLaunchPreservesOwnedContinuationOnFinalFailure(t *testing.T) {
	for _, kind := range []string{"remove-error", "writer-error", "replaced-file"} {
		t.Run(kind, func(t *testing.T) {
			r, f := verifyTestRun(t)
			first := newVerifyMachine(t, r)
			first.completeFirst()
			if first.closeActual() != nil {
				t.Fatal("first close failed")
			}
			if _, _, err := first.finish(); err != nil {
				t.Fatal("first finalization failed")
			}
			original, err := os.ReadFile(r.continuationPath)
			if err != nil {
				t.Fatal("private fixture read failed")
			}
			f.writePlan(t, verifyTestPlan(2))
			second := newVerifyMachine(t, f.prepare(t))
			second.completeSecond()
			if second.closeActual() != nil {
				t.Fatal("second close failed")
			}
			var output bytes.Buffer
			var writer io.Writer = &output
			switch kind {
			case "remove-error":
				second.r.removeContinuation = func(path string) error {
					if os.Remove(path) != nil {
						t.Error("synthetic unlink failed")
					}
					return errors.New(verifyTestPrivate)
				}
			case "writer-error":
				writer = verificationFailWriter{}
			case "replaced-file":
				if os.Remove(second.r.continuationPath) != nil || os.WriteFile(second.r.continuationPath, original, 0600) != nil {
					t.Fatal("synthetic identity replacement failed")
				}
			}
			if err := second.r.Finish(second.v.Report(second.s), writer); err == nil {
				t.Fatal("failed final action became successful second launch")
			}
			data, err := os.ReadFile(second.r.continuationPath)
			if err != nil || !bytes.Equal(data, original) {
				t.Fatal("failed second launch lost or replaced private continuation")
			}
		})
	}
}

func TestVerificationPrivateSnapshotsAndErrorsCannotBecomeDiagnostics(t *testing.T) {
	m := newVerifyActionMachine(t, verifyTestPlan(1))
	r := m.r
	m.s.Search.Query = verifyTestPrivate
	m.s.Player.Video.Title = verifyTestPrivate
	for _, value := range []any{m.s, m.v, verificationReport{ref: catalog.PartRef{Video: verifyTestVideo, Part: "101"}}, r} {
		if strings.Contains(fmt.Sprintf("%#v", value), verifyTestPrivate) || strings.Contains(fmt.Sprintf("%+v", value), verifyTestVideo) {
			t.Fatal("private controller state formatted")
		}
		data, err := json.Marshal(value)
		if err != nil || string(data) != "null" {
			t.Fatal("private controller state serialized")
		}
	}
	if verificationError(errors.New(verifyTestPrivate)) != string(catalog.Unavailable) || verificationError(catalog.Code(verifyTestPrivate)) != string(catalog.Unavailable) {
		t.Fatal("unknown error escaped fixed receipt boundary")
	}
	if _, err := os.Stat(filepath.Join(r.runDirectory, "continuation.json")); !os.IsNotExist(err) {
		t.Fatal("unstarted controller created continuation")
	}
}

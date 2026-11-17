package previewbundle

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"reflect"
	"time"

	"my-bilibili/internal/account"
	"my-bilibili/internal/adapters/mpv"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/history"
	"my-bilibili/internal/playback"
)

// ClientObservation is the complete token-free client evidence schema. It does
// not carry product State, identities, text, media inputs or account material.
type ClientObservation struct {
	AccountPhase                                                 account.Phase
	AccountError                                                 account.Code
	AccountEpoch, ScopeRevision, Navigation, Visit               uint64
	Authenticated, Stored, Restored                              bool
	SearchPhase                                                  catalog.SearchPhase
	SearchError                                                  catalog.Code
	SearchGeneration                                             uint64
	Results, Page                                                int
	LoadingMore, AtLimit                                         bool
	DetailPhase                                                  app.DetailPhase
	DetailError, SelectionError, PlaybackRetryError, ResumeError catalog.Code
	Parts, SelectedPart, PlaybackPart                            int
	Qualities, SelectedQuality, PlaybackQuality                  int
	Resolving, Resuming, DesiredPlaying                          bool
	Playback                                                     playback.Snapshot
	VideoTrack, AudioTrack                                       bool
	PicturePresented                                             bool
	SourceCalls, SourceActive, SourcePeak                        int
	SearchCalls, SearchActive, SearchPeak                        int
	APIRequests, APIActive, APIPeak                              int
	CoverRequests, CoverActive, CoverRetained                    int
	DanmakuPhase                                                 danmaku.LoadPhase
	DanmakuError                                                 catalog.Code
	DanmakuGeneration, DanmakuRevision                           uint64
	Segment, CachedSegments, CachedEntries, CachedTextBytes      int
	DanmakuActiveRequests, VisibleComments, TextLayouts          int
	HistoryPhase                                                 history.Phase
	HistoryError                                                 history.Code
	HistoryCount                                                 int
	HistoryRevision, HistorySavedRevision                        uint64
	HistoryBusy, HistoryDirty                                    bool
	PendingOwners                                                int
	Resources                                                    mpv.Resources
	Minimized, Closing, Closed                                   bool
}

func ParseClientObservation(data []byte) (ClientObservation, error) {
	var o ClientObservation
	if !decodeClientValue(data, &o) || !validClientObservation(o) {
		return ClientObservation{}, errors.New("invalid client observation")
	}
	return o, nil
}

func decodeClientValue(data []byte, target any) bool {
	if len(data) == 0 || len(data) > 65536 {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(data))
	if requiredValue(d, reflect.TypeOf(target).Elem()) != nil {
		return false
	}
	if _, err := d.Token(); err != io.EOF {
		return false
	}
	return json.Unmarshal(data, target) == nil
}

func clientCount(n, maximum int) bool { return n >= 0 && n <= maximum }

func clientCalls(total, active, peak, cap int) bool {
	return clientCount(total, math.MaxInt32) && clientCount(active, cap) &&
		peak >= active && peak <= cap && peak <= total
}

func validClientObservation(o ClientObservation) bool {
	if !clientPhases(o) || !clientAccountCode(o.AccountError) || !clientHistoryCode(o.HistoryError) {
		return false
	}
	for _, code := range []catalog.Code{o.SearchError, o.DetailError, o.SelectionError, o.PlaybackRetryError, o.ResumeError, o.DanmakuError} {
		if !clientContentCode(code) {
			return false
		}
	}
	s := o.Playback
	if !clientPlayback(s) || !clientCount(o.Results, 1000) || !clientCount(o.Page, 50) ||
		!clientCount(o.Parts, 1000) || !clientCount(o.SelectedPart, o.Parts) || !clientCount(o.PlaybackPart, o.Parts) ||
		!clientCount(o.Qualities, 16) || !clientCount(o.SelectedQuality, o.Qualities) || !clientCount(o.PlaybackQuality, o.Qualities) {
		return false
	}
	if o.Authenticated != (o.AccountEpoch != 0) ||
		o.PlaybackPart > 0 && (s.Generation == 0 || o.PlaybackQuality == 0) ||
		o.PlaybackPart == 0 && (o.PlaybackQuality != 0 || o.Qualities != 0) {
		return false
	}
	if !clientCalls(o.SourceCalls, o.SourceActive, o.SourcePeak, 1) ||
		!clientCalls(o.SearchCalls, o.SearchActive, o.SearchPeak, 1) ||
		!clientCalls(o.APIRequests, o.APIActive, o.APIPeak, 4) ||
		!clientCount(o.CoverRequests, math.MaxInt32) || !clientCount(o.CoverActive, 1) ||
		o.CoverActive > o.CoverRequests || !clientCount(o.CoverRetained, 12) {
		return false
	}
	if !clientCount(o.Segment, 1680) || !clientCount(o.CachedSegments, 2) ||
		!clientCount(o.CachedEntries, o.CachedSegments*danmaku.MaxSegmentEntries) ||
		!clientCount(o.CachedTextBytes, o.CachedSegments*danmaku.MaxSegmentTextBytes) ||
		!clientCount(o.DanmakuActiveRequests, 1) || !clientCount(o.VisibleComments, danmaku.MaxLanes) ||
		!clientCount(o.TextLayouts, danmaku.MaxLanes) {
		return false
	}
	if o.CachedSegments > 0 && o.Segment == 0 || o.CachedTextBytes < o.CachedEntries ||
		o.VisibleComments > 0 && (o.DanmakuGeneration != s.Generation || o.PlaybackPart == 0) {
		return false
	}
	if !clientCount(o.HistoryCount, 500) || o.HistorySavedRevision > o.HistoryRevision ||
		!clientCount(o.PendingOwners, 9) || !o.Closing && !o.Closed && o.PendingOwners != 0 {
		return false
	}
	r := o.Resources
	if r.Cores < 0 || r.Cores > 1 || r.Renderers < 0 || r.Renderers > 1 || r.Workers < 0 || r.Workers > 2 {
		return false
	}
	if o.Closed {
		return o.PendingOwners == 0 && r == (mpv.Resources{}) && s.State == playback.Closed &&
			!o.VideoTrack && !o.AudioTrack && !o.PicturePresented && !o.Authenticated &&
			o.SourceActive == 0 && o.SearchActive == 0 && o.APIActive == 0 && o.CoverActive == 0 &&
			o.DanmakuActiveRequests == 0 && o.VisibleComments == 0 && o.TextLayouts == 0 &&
			o.CachedSegments == 0 && o.CachedEntries == 0 && o.CachedTextBytes == 0 && o.CoverRetained == 0 &&
			!o.Resolving && !o.Resuming && !o.DesiredPlaying && !o.HistoryBusy &&
			o.AccountPhase == account.Done && o.SearchPhase == catalog.SearchClosed &&
			o.DanmakuPhase == danmaku.LoadClosed && o.HistoryPhase == history.Done
	}
	return true
}

func clientPlayback(s playback.Snapshot) bool {
	switch s.State {
	case playback.Idle, playback.Loading, playback.Ready, playback.Playing, playback.Paused, playback.Ended, playback.Failed, playback.Closed:
	default:
		return false
	}
	switch s.Error {
	case playback.NoError, playback.Invalid, playback.Unsupported, playback.Canceled, playback.Timeout, playback.Media, playback.IsClosed:
	default:
		return false
	}
	return s.Position >= 0 && s.Position <= 7*24*time.Hour && s.Duration >= 0 && s.Duration <= 7*24*time.Hour &&
		!math.IsNaN(s.Rate) && !math.IsInf(s.Rate, 0) && s.Rate >= 0 && s.Rate <= 3 &&
		!math.IsNaN(s.Volume) && !math.IsInf(s.Volume, 0) && s.Volume >= 0 && s.Volume <= 1
}

func clientPhases(o ClientObservation) bool {
	switch o.AccountPhase {
	case account.Initial, account.Restoring, account.SignedOut, account.Generating, account.Waiting, account.Scanned,
		account.Saving, account.SaveFailed, account.Verifying, account.VerifyFailed, account.SignedIn, account.Expired,
		account.Failed, account.Canceling, account.SigningOut, account.DeleteFailed, account.Closing, account.Done:
	default:
		return false
	}
	switch o.SearchPhase {
	case catalog.SearchInitial, catalog.SearchLoading, catalog.SearchReady, catalog.SearchFailed, catalog.SearchClosed:
	default:
		return false
	}
	switch o.DetailPhase {
	case app.DetailInitial, app.DetailLoading, app.DetailReady, app.DetailFailed:
	default:
		return false
	}
	switch o.DanmakuPhase {
	case danmaku.LoadInitial, danmaku.LoadLoading, danmaku.LoadReady, danmaku.LoadFailed, danmaku.LoadClosed:
	default:
		return false
	}
	switch o.HistoryPhase {
	case history.Initial, history.Loading, history.Ready, history.Failed, history.Closing, history.Done:
		return true
	}
	return false
}

func clientContentCode(code catalog.Code) bool {
	switch code {
	case catalog.None, catalog.Invalid, catalog.Unsupported, catalog.Canceled, catalog.Timeout, catalog.Network,
		catalog.InvalidResponse, catalog.Unavailable, catalog.Restricted, catalog.LoginRequired, catalog.SessionExpired,
		catalog.VerificationRequired, catalog.RateLimited, catalog.ServiceUnavailable, catalog.QualityUnavailable, catalog.Closed:
		return true
	}
	return false
}

func clientAccountCode(code account.Code) bool {
	switch code {
	case account.None, account.Invalid, account.Network, account.ServiceRejected, account.ExpiredSession,
		account.Storage, account.Missing, account.Busy, account.Canceled, account.Timeout, account.Closed:
		return true
	}
	return false
}

func clientHistoryCode(code history.Code) bool {
	switch code {
	case history.None, history.Invalid, history.Storage, history.InUse, history.LockUnsupported, history.Corrupt,
		history.UnsupportedVersion, history.Uncertain, history.Closed:
		return true
	}
	return false
}

package app

import (
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

func (s *Session) initialSelection() {
	index := max(0, s.options.InitialPage-1)
	if s.options.InitialPart != "" {
		index = s.partIndex(s.options.InitialPart)
	}
	if index < 0 || index >= len(s.video.Parts) {
		s.selected = s.options.InitialPart
		s.selectionError = catalog.Unavailable
		return
	}
	s.selected = s.video.Parts[index].ID
	s.selectedQuality = ""
	s.selectionError, s.selectionRetry = catalog.None, ""
	s.submitTarget(selectionRequest, ResolveRequest{Part: s.selected}, s.options.Autoplay, s.options.StartAt, true)
}

// SelectQuality replaces only the accepted part. On acceptance it carries an
// unconfirmed seek forward, or takes the latest position if playback has resumed.
func (s *Session) SelectQuality(id QualityID) error {
	if err := s.detailReady(); err != nil {
		return err
	}
	if !s.hasInput || s.selected != s.playback || s.controller.Snapshot().State == playback.Ended ||
		s.pending != nil && s.pending.request.part != s.playback {
		return playback.Invalid
	}
	found := false
	for _, quality := range s.qualities {
		found = found || id == quality.ID
	}
	if !found {
		return catalog.QualityUnavailable
	}
	if s.pending != nil && s.pending.request.purpose == qualityRequest && s.pending.request.quality == id {
		return nil
	}
	intent := s.intent
	if s.pending != nil {
		intent = s.pending.intent
	}
	s.selectedQuality = id
	s.selectionError, s.selectionRetry = catalog.None, ""
	if id == s.playbackQuality {
		if s.pending != nil && s.pending.request.purpose == qualityRequest {
			s.retireRequest()
		}
		return nil
	}
	s.submitTarget(qualityRequest, ResolveRequest{Part: s.playback, Quality: id}, intent, 0, false)
	return nil
}

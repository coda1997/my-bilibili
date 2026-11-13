package main

import (
	"encoding/json"
	"io"
	"strings"
	"time"

	"my-bilibili/experiments/playback/contract"
	"my-bilibili/experiments/playback/mpvprobe"
)

// Phases come only from driver control flow, never from RPC or caller strings.
type driverPhase string

const (
	phaseDescriptor      driverPhase = "descriptor"
	phasePreflight       driverPhase = "preflight"
	phaseStart           driverPhase = "start"
	phaseConnect         driverPhase = "connect"
	phaseInitialSnapshot driverPhase = "initial-snapshot"
	phaseReady           driverPhase = "ready"
	phasePictureTracks   driverPhase = "picture-tracks"
	phaseReadyResources  driverPhase = "ready-resources"
	phaseProgress        driverPhase = "progress"
	phasePause           driverPhase = "pause"
	phaseSeek            driverPhase = "seek"
	phaseVolume          driverPhase = "volume"
	phaseRate            driverPhase = "rate"
	phaseResume          driverPhase = "resume"
	phaseCancel          driverPhase = "playing-cancel"
	phaseCancelResources driverPhase = "cancel-resources"
	phaseCleanupConnect  driverPhase = "cleanup-connect"
	phaseCleanupQuit     driverPhase = "cleanup-quit"
	phaseCleanupExit     driverPhase = "cleanup-exit"
	phaseCleanupSocket   driverPhase = "cleanup-socket"
)

type sample[T any] struct {
	Phase   driverPhase   `json:"phase"`
	Elapsed time.Duration `json:"elapsed_ns"`
	Value   T             `json:"value"`
}

type failureEvidence struct {
	Phase         driverPhase                 `json:"phase"`
	Code          contract.ErrorCode          `json:"code"`
	Elapsed       time.Duration               `json:"elapsed_ns"`
	LastSnapshot  *sample[observed]           `json:"last_snapshot,omitempty"`
	LastResources *sample[mpvprobe.Resources] `json:"last_resources,omitempty"`
}

func (r *report) setPhase(phase driverPhase) {
	if r.started.IsZero() {
		r.started = time.Now()
	}
	r.phase = phase
}

func (r *report) failure(err error) *failureEvidence {
	return &failureEvidence{r.phase, safeCode(err), time.Since(r.started), r.lastSnapshot, r.lastResources}
}

func (r *report) sessionFailed(err error) {
	if err != nil && r.SessionFailure == nil {
		r.SessionFailure = r.failure(err)
	}
}

// Keep only complete validated values from an already received tree. Independent
// samples keep their own time/phase; an invalid later tree cannot re-date them.
func (r *report) observe(nodes []node) {
	if r.started.IsZero() {
		return // No assigned driver phase; never invent provenance.
	}
	elapsed := time.Since(r.started)
	if s, err := snapshotNodes(nodes); err == nil {
		r.lastSnapshot = &sample[observed]{r.phase, elapsed, s}
	}
	if resources, err := resourceNodes(nodes); err == nil {
		r.lastResources = &sample[mpvprobe.Resources]{r.phase, elapsed, resources}
	}
}

// Select exact required fields before typed decoding. Unknown fields (including
// case aliases) cannot override a safe value or enter retained evidence. Missing,
// null or duplicate fields cannot masquerade as observed zero values.
func completeObject(text string, fields []string, out any) bool {
	d := json.NewDecoder(strings.NewReader(text))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return false
	}
	seen := make(map[string]bool)
	selected := make(map[string]json.RawMessage, len(fields))
	for d.More() {
		token, err = d.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] {
			return false
		}
		seen[key] = true
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return false
		}
		for _, field := range fields {
			if key == field {
				if string(value) == "null" {
					return false
				}
				selected[key] = value
			}
		}
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') || len(selected) != len(fields) {
		return false
	}
	if _, err = d.Token(); err != io.EOF {
		return false
	}
	b, err := json.Marshal(selected)
	return err == nil && json.Unmarshal(b, out) == nil
}

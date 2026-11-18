package main

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"my-bilibili/internal/app"
	"my-bilibili/internal/playback"
	"my-bilibili/internal/previewbundle"
)

func closedSourceObservation(t *testing.T) observed {
	t.Helper()
	o := completeObservation(t)
	o.Source = previewbundle.SourceObservation{DetailRequests: 1, ResolveRequests: 1, PeakActive: 1}
	return o
}

func TestSourceClosedRequiresActualEndedOwnership(t *testing.T) {
	for name, mutate := range map[string]func(*observed){
		"valid":         func(*observed) {},
		"native core":   func(o *observed) { o.Resources.Cores = 1 },
		"source active": func(o *observed) { o.Source.Active = 1 },
		"concurrency":   func(o *observed) { o.Source.PeakActive = 2 },
		"no detail":     func(o *observed) { o.Source.DetailRequests = 0 },
		"still closing": func(o *observed) { o.State.Closing = true },
		"still pending": func(o *observed) { o.State.ResolvingPart = "part-2" },
		"failed close":  func(o *observed) { o.State.Playback.Error = playback.Media },
	} {
		t.Run(name, func(t *testing.T) {
			o := closedSourceObservation(t)
			mutate(&o)
			receipts := newSourceReceipts()
			receipts.values[previewbundle.SourceWindowClosed] = o
			_, err := sourceClosed(&ownedProcess{sourceReceipts: receipts})
			if (err == nil) != (name == "valid") {
				t.Fatal("close evidence did not discriminate ownership", err)
			}
		})
	}
	if _, err := sourceClosed(&ownedProcess{}); err == nil {
		t.Fatal("missing receipt became successful cleanup")
	}
}

func TestMinimizedSourceCloseNeedsProofWithoutAnotherRPC(t *testing.T) {
	for name, mutate := range map[string]func(*observed){
		"valid":         func(*observed) {},
		"not minimized": func(o *observed) { o.Minimized = false },
		"not pending":   func(o *observed) { o.Source.Active = 0 },
		"parallel":      func(o *observed) { o.Source.PeakActive = 2 },
		"detail changed": func(o *observed) {
			o.State.DetailPhase = app.DetailLoading
		},
		"part changed":       func(o *observed) { o.State.ResolvingPart = "part-1" },
		"generation changed": func(o *observed) { o.State.Playback.Generation++ },
	} {
		t.Run(name, func(t *testing.T) {
			before := closedSourceObservation(t)
			before.State.ResolvingPart, before.Source.Active = "part-2", 1
			minimized := before
			minimized.Minimized = true
			mutate(&minimized)
			receipts := newSourceReceipts()
			receipts.values[previewbundle.SourceWindowMinimized] = minimized
			receipts.values[previewbundle.SourceWindowClosed] = closedSourceObservation(t)
			p := &ownedProcess{done: make(chan error, 1), sourceReceipts: receipts}
			var calls atomic.Int32
			c := pipeClient(t, func(_ int, method string) any {
				if calls.Add(1) == 1 && method == "act" {
					p.done <- nil
				}
				return map[string]bool{"ok": true}
			})
			r := &report{}
			err := c.sourceMinimizedClose(t.Context(), p, r, before)
			if (err == nil) != (name == "valid") {
				t.Fatal("minimized evidence did not discriminate the assigned state", err)
			}
			if calls.Load() != 1 || !p.closeRequested || !p.reaped {
				t.Fatal("close sent another RPC or did not retain process ownership")
			}
			if name != "valid" && len(r.Checks) != 0 {
				t.Fatal("invalid minimized evidence recorded a pass")
			}
		})
	}
}

func TestPreviouslyRequestedCloseRetainsExitAndSocketChecks(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "absent.sock")
	p := &ownedProcess{closeRequested: true, reaped: true}
	if err := closeOwned(p, socket, &report{}); err != nil {
		t.Fatal("already requested close attempted another connection", err)
	}
	p.closeRequested = false
	if err := closeOwned(p, socket, &report{}); err == nil {
		t.Fatal("unexpected process exit passed as an explicit close")
	}
	p.closeRequested, p.exit = true, errors.New("failed")
	if err := closeOwned(p, socket, &report{}); err == nil {
		t.Fatal("unsuccessful exit passed cleanup")
	}
	p.exit = nil
	if err := os.WriteFile(socket, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := closeOwned(p, socket, &report{}); err == nil {
		t.Fatal("remaining automation socket passed cleanup")
	}
}

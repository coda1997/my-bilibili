package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"my-bilibili/internal/app"
	"my-bilibili/internal/playback"
	"my-bilibili/internal/previewbundle"
)

func sourceClosed(p *ownedProcess) (observed, error) {
	o, err := p.sourceReceipts.get(previewbundle.SourceWindowClosed)
	if err != nil {
		return observed{}, err
	}
	code := o.State.Playback.Error
	if code != playback.NoError && code != playback.Canceled || !disposed(o, code) || o.State.Closing || o.State.ResolvingPart != "" || o.Source.Active != 0 || o.Source.PeakActive > 1 || o.Source.DetailRequests < 1 {
		return observed{}, errors.New("source or native ownership survived window close")
	}
	return o, nil
}

func (c *client) sourceNode(ctx context.Context, key string) error {
	nodes, err := c.tree(ctx)
	if err != nil {
		return err
	}
	n, err := uniqueNode(nodes, key)
	if err != nil || strings.TrimSpace(n.Label) == "" {
		return errors.New("source loading or retry UI missing")
	}
	return nil
}

func (c *client) sourceDetailPending(ctx context.Context) (observed, error) {
	return c.until(ctx, 3*time.Second, func(o observed) bool {
		return o.State.DetailPhase == app.DetailLoading && o.State.Playback.Generation == 0 && o.Source.DetailRequests == 1 && o.Source.Active == 1 && o.Source.PeakActive == 1
	})
}

func (c *client) resizeDuringSource(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := c.act(ctx, "e2e-narrow"); err != nil {
		return err
	}
	for {
		w, err := c.identify(ctx)
		if err != nil {
			return err
		}
		if w.ContentBounds.Width == 800 && w.ContentBounds.Height == 600 {
			return nil
		}
		if err := wait(ctx, 25*time.Millisecond); err != nil {
			return err
		}
	}
}

func (c *client) sourcePartPending(ctx context.Context) (observed, error) {
	before, err := c.startPlaying(ctx)
	if err != nil {
		return observed{}, err
	}
	if err := c.act(ctx, "list-part-part-2"); err != nil {
		return observed{}, err
	}
	return c.until(ctx, 3*time.Second, func(o observed) bool {
		return o.State.SelectedPart == "part-2" && o.State.ResolvingPart == "part-2" && o.State.PlaybackPart == "part-1" && o.State.Playback.Generation == before.State.Playback.Generation && o.State.Playback.State == playback.Playing && o.Source.Active == 1 && o.Source.PeakActive == 1
	})
}

func (c *client) sourceMinimizedClose(ctx context.Context, p *ownedProcess, r *report, before observed) error {
	if p.exited() {
		return errors.New("source process exited before minimized close request")
	}
	p.closeRequested = true
	if err := c.act(ctx, "e2e-minimized-close"); err != nil {
		return err
	}
	// No further RPC/tree/picture call may pump a minimized frame and mask a
	// paint-dependent close bug. Only await the owned process and its receipts.
	if !p.await(5*time.Second) || p.exit != nil {
		return errors.New("minimized source window did not finish owned close")
	}
	minimized, err := p.sourceReceipts.get(previewbundle.SourceWindowMinimized)
	if err != nil {
		return err
	}
	if !minimized.Minimized || minimized.Source.Active != 1 || minimized.Source.PeakActive != 1 || minimized.State.DetailPhase != before.State.DetailPhase || minimized.State.ResolvingPart != before.State.ResolvingPart || minimized.State.Playback.Generation != before.State.Playback.Generation {
		return errors.New("native minimize did not occur with the assigned source pending")
	}
	if _, err := sourceClosed(p); err != nil {
		return err
	}
	r.pass("minimized-pending-source-close", "actual OnMinimize/IsMinimized receipt precedes close; no subsequent RPC or paint request; clean process exit", &minimized)
	return nil
}

func runSource(ctx context.Context, executable, dir string, r *report) error {
	scenarios := []struct {
		name, seam string
		exercise   func(context.Context, *client, *ownedProcess) error
	}{
		{"source-detail-release", "source-detail", func(ctx context.Context, c *client, p *ownedProcess) error {
			o, err := c.sourceDetailPending(ctx)
			if err != nil {
				return err
			}
			if err = c.sourceNode(ctx, "detail-state"); err != nil {
				return err
			}
			if err = c.resizeDuringSource(ctx); err != nil {
				return err
			}
			r.pass("detail-loading-responsive", "detail still held while native window resizing and loading UI respond", &o)
			if err = c.act(ctx, "e2e-release-source"); err != nil {
				return err
			}
			o, err = c.until(ctx, 10*time.Second, ready)
			if err != nil {
				return err
			}
			if o.State.DetailPhase != app.DetailReady || o.State.DesiredPlaying || o.Source.DetailRequests != 1 || o.Source.ResolveRequests != 1 || o.Source.Active != 0 {
				return errors.New("detail completion did not resolve exactly one paused initial part")
			}
			r.pass("detail-completion", "validated fixture metadata leads to one paused P1 native attempt", &o)
			return nil
		}},
		{"source-detail-retry", "source-detail-failure", func(ctx context.Context, c *client, p *ownedProcess) error {
			o, err := c.until(ctx, 3*time.Second, func(o observed) bool {
				return o.State.DetailPhase == app.DetailFailed && o.State.DetailError == playback.Media && o.Source.Active == 0
			})
			if err != nil {
				return err
			}
			if err = c.sourceNode(ctx, "retry-detail"); err != nil {
				return err
			}
			r.pass("detail-failure-visible", "safe failure and explicit retry shown before any playback generation", &o)
			if err = c.act(ctx, "retry-detail"); err != nil {
				return err
			}
			o, err = c.until(ctx, 10*time.Second, ready)
			if err != nil {
				return err
			}
			if o.Source.DetailRequests != 2 || o.Source.ResolveRequests != 1 || o.State.DetailError != playback.NoError {
				return errors.New("detail retry did not recover once")
			}
			r.pass("detail-retry", "explicit retry recovered metadata and paused initial playback", &o)
			return nil
		}},
		{"source-detail-minimized-close", "source-detail", func(ctx context.Context, c *client, p *ownedProcess) error {
			o, err := c.sourceDetailPending(ctx)
			if err != nil {
				return err
			}
			return c.sourceMinimizedClose(ctx, p, r, o)
		}},
		{"source-resolve-pending-intent", "source-resolve", func(ctx context.Context, c *client, p *ownedProcess) error {
			before, err := c.sourcePartPending(ctx)
			if err != nil {
				return err
			}
			if err = c.sourceNode(ctx, "resolving-part"); err != nil {
				return err
			}
			if err = c.act(ctx, "play-toggle"); err != nil {
				return err
			}
			o, err := c.until(ctx, 2*time.Second, func(o observed) bool {
				return o.State.ResolvingPart == "part-2" && !o.State.DesiredPlaying && o.State.Playback.State == playback.Paused && o.Source.Active == 1
			})
			if err != nil {
				return err
			}
			r.pass("pending-resolution-pause", "accepted P1 and pending P2 both retain the newer pause intent", &o)
			if err = c.act(ctx, "e2e-release-source"); err != nil {
				return err
			}
			o, err = c.until(ctx, 10*time.Second, func(o observed) bool {
				return ready(o) && o.State.PlaybackPart == "part-2" && o.State.Playback.Generation > before.State.Playback.Generation
			})
			if err != nil {
				return err
			}
			if o.State.DesiredPlaying || o.State.ResolvingPart != "" || o.State.Playback.Position > 150*time.Millisecond || o.Source.Active != 0 {
				return errors.New("resolution completion lost pause intent")
			}
			r.pass("pending-intent-at-new-generation", "new P2 remained paused at its beginning after delayed resolution", &o)
			return nil
		}},
		{"source-resolve-retry", "source-resolve-failure", func(ctx context.Context, c *client, p *ownedProcess) error {
			before, err := c.startPlaying(ctx)
			if err != nil {
				return err
			}
			if err = c.act(ctx, "list-part-part-2"); err != nil {
				return err
			}
			o, err := c.until(ctx, 3*time.Second, func(o observed) bool {
				return o.State.SelectedPart == "part-2" && o.State.SelectionError == playback.Media && o.State.PlaybackPart == "part-1" && o.State.Playback.Generation == before.State.Playback.Generation && o.State.Playback.State == playback.Playing
			})
			if err != nil {
				return err
			}
			r.pass("resolution-failure-preserves-playback", "P2 failure retained P1's accepted generation and advancing native playback", &o)
			if err = c.act(ctx, "retry-selection"); err != nil {
				return err
			}
			o, err = c.until(ctx, 10*time.Second, func(o observed) bool {
				return o.State.PlaybackPart == "part-2" && o.State.Playback.Generation > before.State.Playback.Generation && o.State.Playback.State == playback.Playing && o.State.SelectionError == playback.NoError
			})
			if err != nil {
				return err
			}
			r.pass("resolution-retry", "explicit retry opened and played the requested P2", &o)
			return nil
		}},
		{"source-resolve-minimized-close", "source-resolve", func(ctx context.Context, c *client, p *ownedProcess) error {
			o, err := c.sourcePartPending(ctx)
			if err != nil {
				return err
			}
			return c.sourceMinimizedClose(ctx, p, r, o)
		}},
		{"source-timeout", "source-timeout", func(ctx context.Context, c *client, p *ownedProcess) error {
			before, err := c.sourcePartPending(ctx)
			if err != nil {
				return err
			}
			o, err := c.until(ctx, 12*time.Second, func(o observed) bool {
				return o.State.SelectionError == playback.Timeout && o.State.ResolvingPart == "" && o.Source.Active == 0
			})
			if err != nil {
				return err
			}
			if o.State.PlaybackPart != "part-1" || o.State.Playback.Generation != before.State.Playback.Generation || o.State.Playback.State != playback.Playing || o.Source.ResolveRequests != 2 {
				return errors.New("source timeout replaced accepted playback or retried automatically")
			}
			r.pass("source-deadline-preserves-playback", "ten-second source deadline surfaced Timeout without native replacement or automatic retry", &o)
			return nil
		}},
	}
	for _, scenario := range scenarios {
		if err := invocation(ctx, executable, scenario.name, scenario.seam, dir, r, scenario.exercise); err != nil {
			return err
		}
	}
	return nil
}

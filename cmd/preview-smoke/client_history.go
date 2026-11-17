package main

import (
	"context"
	"errors"
	"time"
)

func runClientHistory(ctx context.Context, executable, historyDir, dir string, r *report) error {
	var savedPosition time.Duration
	err := scenarioInvocation(ctx, executable, "history-write", "", "history", historyDir, dir, r, func(ctx context.Context, c *client, _ *ownedProcess) error {
		initial, err := c.clientStart(ctx, true)
		if err != nil {
			return err
		}
		if initial.HistoryCount != 0 {
			return errors.New("synthetic history directory was not fresh")
		}
		if _, err = c.clientOpen(ctx, "BV0000000001", 1); err != nil {
			return err
		}
		if _, err = c.clientPlay(ctx); err != nil {
			return err
		}
		if _, err = c.clientSeek(ctx, 7*time.Second); err != nil {
			return err
		}
		o, err := c.clientUntil(ctx, 5*time.Second, func(o clientObserved) bool {
			return clientHistorySaved(o, 1) && absDuration(o.Playback.Position-7*time.Second) <= 100*time.Millisecond
		})
		if err != nil {
			return err
		}
		savedPosition = o.Playback.Position
		r.passClient("client-history-save-before-restart", "one canonical original part checkpointed at confirmed native7s; latest revision durably acknowledged before orderly close", o)
		return nil
	})
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	err = scenarioInvocation(ctx, executable, "history-restart", "", "history", historyDir, dir, r, func(ctx context.Context, c *client, _ *ownedProcess) error {
		initial, err := c.clientStart(ctx, true)
		if err != nil {
			return err
		}
		if initial.HistoryCount != 1 || initial.PlaybackPart != 0 {
			return errors.New("restart did not restore exactly one isolated history row without auto-open")
		}
		if err = c.clientAction(ctx, "client-history"); err != nil {
			return err
		}
		if err = c.clientAction(ctx, "history-continue-0"); err != nil {
			return err
		}
		o, err := c.clientUntil(ctx, 10*time.Second, func(o clientObserved) bool {
			return clientPlaying(o) && o.PlaybackPart == 1 && o.SelectedPart == 1 && o.Playback.Position >= 6*time.Second &&
				o.Playback.Position >= savedPosition-250*time.Millisecond && o.Playback.Position <= savedPosition+3*time.Second
		})
		if err != nil {
			return err
		}
		o, err = c.clientProgress(ctx, o)
		if err != nil {
			return err
		}
		r.passClient("client-history-restart-continue", "same isolated directory reloaded in a fresh process; same part resumes confirmed progress with current-engine picture and advancing native time", o)
		if _, err = c.clientPause(ctx); err != nil {
			return err
		}
		if _, err = c.clientUntil(ctx, 5*time.Second, func(o clientObserved) bool { return clientHistorySaved(o, 1) }); err != nil {
			return err
		}
		if err = c.clientAction(ctx, "client-history"); err != nil {
			return err
		}
		if err = c.clientConfirm(ctx, "history-delete-0", "history-delete-confirmation", true); err != nil {
			return err
		}
		o, err = c.clientUntil(ctx, 5*time.Second, func(o clientObserved) bool { return clientHistorySaved(o, 0) })
		if err != nil {
			return err
		}
		r.passClient("client-history-delete-confirmed", "scoped visible confirmation deletes the current visit and saves its newer snapshot", o)
		if err = c.clientAction(ctx, "client-player"); err != nil {
			return err
		}
		before, err := c.clientUntil(ctx, 3*time.Second, clientReady)
		if err != nil {
			return err
		}
		o, err = c.clientQuality(ctx, before)
		if err != nil {
			return err
		}
		if err = wait(ctx, 200*time.Millisecond); err != nil {
			return err
		}
		o, err = c.clientSnapshot(ctx)
		if err != nil {
			return err
		}
		if !clientHistorySaved(o, 0) || o.Visit != before.Visit {
			return errors.New("same-visit quality replacement recreated deleted history")
		}
		r.passClient("client-history-deletion-survives-quality", "same visit survives quality generation change without recreating the deleted record", o)
		o, err = c.clientPartTwo(ctx, o)
		if err != nil {
			return err
		}
		if _, err = c.clientPause(ctx); err != nil {
			return err
		}
		o, err = c.clientUntil(ctx, 5*time.Second, func(o clientObserved) bool { return clientHistorySaved(o, 1) && o.Visit > before.Visit })
		if err != nil {
			return err
		}
		if err = c.clientAction(ctx, "client-history"); err != nil {
			return err
		}
		if err = c.clientConfirm(ctx, "history-clear", "history-clear-confirmation", true); err != nil {
			return err
		}
		o, err = c.clientUntil(ctx, 5*time.Second, func(o clientObserved) bool { return clientHistorySaved(o, 0) })
		if err != nil {
			return err
		}
		r.passClient("client-history-new-visit-and-clear", "explicit part selection permits a fresh visit; scoped Clear confirmation persists an empty isolated history", o)
		return nil
	})
	if err != nil {
		return err
	}
	return ctx.Err()
}

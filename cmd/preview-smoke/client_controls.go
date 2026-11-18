package main

import (
	"context"
	"errors"
	"time"

	"my-bilibili/internal/playback"
)

func validClientScenario(scene string) bool {
	switch scene {
	case "search", "playback", "segments", "history", "account-expiry", "account-signout", "history-corrupt", "history-in-use", "close-held":
		return true
	}
	return false
}

func clientReady(o clientObserved) bool {
	s := o.Playback
	return (s.State == playback.Ready || s.State == playback.Paused) && s.Duration > 0 && s.Error == playback.NoError &&
		s.Generation > 0 && o.PlaybackPart > 0 && o.PlaybackQuality > 0 &&
		o.VideoTrack && o.AudioTrack && o.PicturePresented &&
		o.Resources.Cores > 0 && o.Resources.Renderers > 0 && o.Resources.Workers > 0 &&
		!o.Minimized && !o.Closed && !o.Closing && !o.Resuming
}

func (c *client) clientText(ctx context.Context, key, value string) error {
	if key != "client-search-text" && key != "client-open-text" {
		return errors.New("unknown client text input")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := c.clientControl(ctx, key); err != nil {
		return err
	}
	if err := c.call(ctx, "act", map[string]any{"window": c.window, "key": key, "action": "setValue", "text": value}, nil); err != nil {
		return err
	}
	for {
		nodes, err := c.tree(ctx)
		if err != nil {
			return err
		}
		n, err := uniqueNode(nodes, key)
		if err != nil {
			return err
		}
		if n.Value == value {
			return nil
		}
		if err := wait(ctx, 25*time.Millisecond); err != nil {
			return err
		}
	}
}

// Visible-window automation trees can still describe the previous frame. Reuse
// the strict rendered-node availability parser before a newly mounted control.
func (c *client) clientControl(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	for {
		ready, err := c.popupItemReady(ctx, key)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		if err := wait(ctx, 25*time.Millisecond); err != nil {
			return err
		}
	}
}

func (c *client) clientAction(ctx context.Context, key string) error {
	switch key {
	case "client-catalog", "client-player", "client-history", "client-account":
		return c.clientTab(ctx, key)
	}
	if err := c.clientControl(ctx, key); err != nil {
		return err
	}
	return c.act(ctx, key)
}

// A tab action retires the player's mounted control identities. Observe the
// selected tab in a rendered frame before using those newly mounted controls.
func (c *client) clientTab(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := c.clientControl(ctx, key); err != nil {
		return err
	}
	if err := c.act(ctx, key); err != nil {
		return err
	}
	for {
		nodes, err := c.tree(ctx)
		if err != nil {
			return err
		}
		n, err := uniqueNode(nodes, key)
		if err != nil {
			return err
		}
		// These four navigation controls are native buttons. Their rendered
		// automation nodes use checked to identify the active page.
		for _, state := range n.States {
			if state == "checked" || state == "selected" {
				return nil
			}
		}
		if err := wait(ctx, 25*time.Millisecond); err != nil {
			return err
		}
	}
}

func (c *client) clientPopup(ctx context.Context, menu, item string) error {
	if err := c.clientControl(ctx, menu); err != nil {
		return err
	}
	return c.selectPopup(ctx, menu, item)
}

func scopedClientAction(nodes []node, popover, action string) (string, bool, error) {
	var parent node
	parents := 0
	byID := make(map[string]node, len(nodes))
	for _, n := range nodes {
		if n.Key == popover {
			parent, parents = n, parents+1
		}
		if n.ID != "" {
			if _, ok := byID[n.ID]; ok {
				return "", false, errors.New("duplicate client node identity")
			}
			byID[n.ID] = n
		}
	}
	if parents == 0 {
		return "", false, nil
	}
	if parents != 1 || parent.ID == "" {
		return "", false, errors.New("invalid client confirmation scope")
	}
	if parent.Bounds.Width <= 0 || parent.Bounds.Height <= 0 {
		return "", false, nil
	}
	var target node
	matches := 0
	for _, n := range nodes {
		if n.Key != action {
			continue
		}
		current, seen := n.Parent, map[string]bool{}
		for current != "" {
			if seen[current] {
				return "", false, errors.New("cyclic client confirmation ancestry")
			}
			seen[current] = true
			if current == parent.ID {
				target, matches = n, matches+1
				break
			}
			ancestor, ok := byID[current]
			if !ok {
				return "", false, errors.New("incomplete client confirmation ancestry")
			}
			current = ancestor.Parent
		}
	}
	if matches == 0 {
		return "", false, nil
	}
	if matches != 1 || target.ID == "" {
		return "", false, errors.New("ambiguous client confirmation")
	}
	if !target.Bounds.complete || target.Bounds.Width <= 0 || target.Bounds.Height <= 0 {
		return "", false, nil
	}
	for _, state := range target.States {
		if state == "disabled" || state == "offscreen" {
			return "", false, nil
		}
	}
	return target.ID, true, nil
}

func (c *client) clientConfirm(ctx context.Context, opener, popover string, confirm bool) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := c.clientAction(ctx, opener); err != nil {
		return err
	}
	action := "cancel"
	if confirm {
		action = "confirm"
	}
	for {
		nodes, err := c.tree(ctx)
		if err != nil {
			return err
		}
		id, ready, err := scopedClientAction(nodes, popover, action)
		if err != nil {
			return err
		}
		if ready {
			if err := c.call(ctx, "act", map[string]any{"window": c.window, "id": id, "action": "press"}, nil); err != nil {
				return err
			}
			for {
				nodes, err := c.tree(ctx)
				if err != nil {
					return err
				}
				shown := false
				for _, n := range nodes {
					if n.Key == popover && n.Bounds.Width > 0 && n.Bounds.Height > 0 {
						shown = true
					}
				}
				if !shown {
					return nil
				}
				if err := wait(ctx, 25*time.Millisecond); err != nil {
					return err
				}
			}
		}
		if err := wait(ctx, 25*time.Millisecond); err != nil {
			return err
		}
	}
}

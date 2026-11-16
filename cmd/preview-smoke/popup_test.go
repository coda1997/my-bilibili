package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

type popupRequest struct {
	ID     int
	Method string
	Params struct{ Key string }
}

type popupTrace struct {
	mu       sync.Mutex
	requests []popupRequest
}

func (p *popupTrace) count(method, key string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, r := range p.requests {
		if r.Method == method && (key == "" || r.Params.Key == key) {
			n++
		}
	}
	return n
}

func popupClient(t *testing.T, reply func(popupRequest) map[string]any) (*client, *popupTrace) {
	t.Helper()
	a, b := net.Pipe()
	t.Cleanup(func() { a.Close(); b.Close() })
	trace := &popupTrace{}
	go func() {
		defer b.Close()
		s := bufio.NewScanner(b)
		for s.Scan() {
			var request popupRequest
			if json.Unmarshal(s.Bytes(), &request) != nil {
				return
			}
			trace.mu.Lock()
			trace.requests = append(trace.requests, request)
			trace.mu.Unlock()
			response := reply(request)
			if response == nil {
				return
			}
			response["id"] = request.ID
			data, _ := json.Marshal(response)
			if _, err := b.Write(append(data, '\n')); err != nil {
				return
			}
		}
	}()
	return newClient(a), trace
}

func popupNode(key string, states ...string) map[string]any {
	return map[string]any{"key": key, "bounds": map[string]any{"x": 20, "y": 30, "width": 100, "height": 28}, "states": states}
}

func popupTree(nodes ...any) map[string]any {
	return map[string]any{"result": map[string]any{"nodes": nodes}}
}

func popupOK() map[string]any { return map[string]any{"result": map[string]any{}} }

func TestPopupSelectionWaitsForAvailableItem(t *testing.T) {
	for _, pair := range [][2]string{{"speed-menu", "speed-2"}, {"parts-menu", "part-part-1"}} {
		t.Run(pair[0], func(t *testing.T) {
			frames := 0
			c, trace := popupClient(t, func(r popupRequest) map[string]any {
				if r.Method == "tree" {
					frames++
					switch frames {
					case 1:
						return popupTree(popupNode(pair[0]))
					case 2:
						return popupTree(popupNode(pair[1], "disabled"))
					case 3:
						return popupTree(popupNode(pair[1], "offscreen"))
					case 4:
						n := popupNode(pair[1])
						n["bounds"].(map[string]any)["width"] = 0
						return popupTree(n)
					default:
						return popupTree(popupNode(pair[1]))
					}
				}
				if r.Params.Key == pair[1] && frames < 5 {
					return map[string]any{"error": map[string]any{"code": "no_element", "message": "target is not ready"}}
				}
				return popupOK()
			})
			if err := c.selectPopup(context.Background(), pair[0], pair[1]); err != nil {
				t.Fatalf("item action preceded its available frame: %v", err)
			}
			if trace.count("act", pair[0]) != 1 || trace.count("act", pair[1]) != 1 || trace.count("tree", "") != 5 {
				t.Fatal("expected exactly one menu action and one item action after five observations")
			}
		})
	}
}

func TestPopupAbsentItemPreservesDeadlineWithoutActionRetry(t *testing.T) {
	c, trace := popupClient(t, func(r popupRequest) map[string]any {
		if r.Method == "tree" {
			return popupTree(popupNode("speed-menu"))
		}
		return popupOK()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	err := c.selectPopup(ctx, "speed-menu", "speed-2")
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "speed-2") {
		t.Fatalf("missing bounded target deadline/context: %v", err)
	}
	if trace.count("act", "speed-menu") != 1 || trace.count("act", "speed-2") != 0 {
		t.Fatal("absent target caused an item action or repeated menu action")
	}
}

func TestPopupRejectsMalformedOrDuplicateTreeImmediately(t *testing.T) {
	missingWidth := popupNode("speed-2")
	delete(missingWidth["bounds"].(map[string]any), "width")
	invalidWidth := popupNode("speed-2")
	invalidWidth["bounds"].(map[string]any)["width"] = "private payload"
	negativeWidth := popupNode("speed-2")
	negativeWidth["bounds"].(map[string]any)["width"] = -1
	for name, response := range map[string]map[string]any{
		"duplicate":      popupTree(popupNode("speed-2"), popupNode("speed-2")),
		"null node":      popupTree(nil),
		"missing width":  popupTree(missingWidth),
		"invalid width":  popupTree(invalidWidth),
		"negative width": popupTree(negativeWidth),
		"null tree":      {"result": nil},
	} {
		t.Run(name, func(t *testing.T) {
			c, trace := popupClient(t, func(r popupRequest) map[string]any {
				if r.Method == "tree" {
					return response
				}
				return popupOK()
			})
			err := c.selectPopup(context.Background(), "speed-menu", "speed-2")
			if err == nil || strings.Contains(err.Error(), "private payload") {
				t.Fatalf("malformed tree accepted or payload leaked: %v", err)
			}
			if trace.count("tree", "") != 1 || trace.count("act", "speed-2") != 0 || trace.count("act", "speed-menu") != 1 {
				t.Fatal("malformed/duplicate tree was retried or acted on")
			}
		})
	}
}

func TestPopupTransportFailureStopsWithoutRetry(t *testing.T) {
	c, trace := popupClient(t, func(r popupRequest) map[string]any {
		if r.Method == "tree" {
			return nil
		}
		return popupOK()
	})
	err := c.selectPopup(context.Background(), "speed-menu", "speed-2")
	if err == nil || !strings.Contains(err.Error(), "speed-2") || !strings.Contains(err.Error(), "tree read failed") {
		t.Fatalf("lost safe target/transport context: %v", err)
	}
	if trace.count("tree", "") != 1 || trace.count("act", "speed-2") != 0 {
		t.Fatal("transport failure retried or sent item action")
	}
}

func TestPopupItemActionUsesSharedDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, trace := popupClient(t, func(r popupRequest) map[string]any {
		if r.Method == "tree" {
			cancel()
			return popupTree(popupNode("speed-2"))
		}
		return popupOK()
	})
	if err := c.selectPopup(ctx, "speed-menu", "speed-2"); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost canceled context identity: %v", err)
	}
	if trace.count("act", "speed-2") != 0 {
		t.Fatal("item action ignored expired popup context")
	}
}

func TestPopupWaitHasTwoSecondLimit(t *testing.T) {
	c, trace := popupClient(t, func(r popupRequest) map[string]any {
		if r.Method == "tree" {
			return popupTree(popupNode("speed-menu"))
		}
		return popupOK()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	start := time.Now()
	err := c.selectPopup(ctx, "speed-menu", "speed-2")
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 2500*time.Millisecond {
		t.Fatalf("popup availability did not enforce its two-second bound: %v, %v", err, time.Since(start))
	}
	if trace.count("act", "speed-menu") != 1 || trace.count("act", "speed-2") != 0 {
		t.Fatal("popup timeout repeated menu or acted on absent item")
	}
}

func TestPopupActionFailureIsNeverRetried(t *testing.T) {
	for _, failedKey := range []string{"speed-menu", "speed-2"} {
		t.Run(failedKey, func(t *testing.T) {
			c, trace := popupClient(t, func(r popupRequest) map[string]any {
				if r.Method == "tree" {
					return popupTree(popupNode("speed-2"))
				}
				if r.Params.Key == failedKey {
					return map[string]any{"error": map[string]any{"code": "failed", "message": "private native payload"}}
				}
				return popupOK()
			})
			err := c.selectPopup(context.Background(), "speed-menu", "speed-2")
			if err == nil || !strings.Contains(err.Error(), failedKey) || strings.Contains(err.Error(), "private") {
				t.Fatalf("lost safe failure context: %v", err)
			}
			if trace.count("act", failedKey) != 1 || failedKey == "speed-menu" && trace.count("tree", "") != 0 {
				t.Fatal("action failure retried or continued")
			}
		})
	}
}

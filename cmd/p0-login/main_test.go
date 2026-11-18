package main

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"my-bilibili/internal/account"
)

func TestLoginOptionsGateModes(t *testing.T) {
	for _, args := range [][]string{{"--synthetic"}, {"--begin-login", "--restore-check"}, {"--cookies=PRIVATE"}, {"PRIVATE"}, {"--begin-login", "PRIVATE"}} {
		if _, err := parseOptions(args, false); err == nil {
			t.Fatal("production flags accepted unsafe mode")
		}
	}
	for _, args := range [][]string{{}, {"--begin-login"}, {"--restore-check"}} {
		if _, err := parseOptions(args, false); err != nil {
			t.Fatal("normal mode rejected")
		}
	}
	for _, args := range [][]string{{}, {"--begin-login"}, {"--restore-check"}, {"--synthetic", "--begin-login"}, {"--synthetic", "--restore-check"}} {
		if _, err := parseOptions(args, true); err == nil {
			t.Fatal("e2e accepted live mode")
		}
	}
	if _, err := parseOptions([]string{"--synthetic"}, true); err != nil {
		t.Fatal("synthetic mode rejected")
	}
}
func TestSafeReceiptExcludesSecrets(t *testing.T) {
	const sentinel = "PRIVATE_ACCOUNT_SENTINEL"
	s := account.State{Phase: account.Phase(sentinel), Error: account.Code(sentinel), QR: account.NewQR(sentinel), Stored: true}
	encoded, err := json.Marshal(safeReceipt("state", s))
	if err != nil || bytes.Contains(encoded, []byte(sentinel)) || bytes.Contains(encoded, []byte("QR")) {
		t.Fatal("receipt includes private/unknown state")
	}
	r := safeReceipt("state", account.State{Phase: account.SignedIn, Stored: true, Restored: true})
	if !r.Verified || !r.Stored || !r.Restored {
		t.Fatal("verified state not preserved")
	}
	var out, diagnostics bytes.Buffer
	if run([]string{"--cookie=" + sentinel}, &out, &diagnostics) != 2 || bytes.Contains(diagnostics.Bytes(), []byte(sentinel)) || out.Len() != 0 {
		t.Fatal("argument error leaks or runs backend")
	}
}

func TestRestoreCheckRequiresCompletedVerification(t *testing.T) {
	a := application{options: options{restoreCheck: true}}
	if a.resultCode() == 0 {
		t.Fatal("early close reported restore success")
	}
	a.restoreFinished = true
	if a.resultCode() != 0 {
		t.Fatal("successful completion rejected")
	}
	a.exitCode = 1
	if a.resultCode() == 0 {
		t.Fatal("failed verification reported success")
	}
}

func TestBackgroundStateCompletionWithoutRender(t *testing.T) {
	var mu sync.Mutex
	state := account.State{Phase: account.Closing, Busy: true}
	events := make(chan struct{}, 8)
	snapshots := 0
	snapshot := func() account.State { mu.Lock(); defer mu.Unlock(); snapshots++; return state }
	stop := watchState(snapshot, func() { events <- struct{}{} }, 5*time.Millisecond)
	defer stop()
	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("initial state not published")
	}
	mu.Lock()
	state.Phase = account.Done
	state.Busy = false
	mu.Unlock()
	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("completed close waits for a render")
	}
	stop()
	stop()
	mu.Lock()
	before := snapshots
	state.Phase = account.Waiting
	mu.Unlock()
	time.Sleep(15 * time.Millisecond)
	mu.Lock()
	after := snapshots
	mu.Unlock()
	if after != before {
		t.Fatal("state watcher outlived its owner")
	}
	select {
	case <-events:
		t.Fatal("stopped state watcher dispatched")
	default:
	}
}

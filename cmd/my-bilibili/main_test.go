package main

import (
	"context"
	"errors"
	"testing"

	"my-bilibili/internal/app"
	"my-bilibili/internal/fixture"
	"my-bilibili/internal/playback"
)

func TestWindowClosePreservesStartupFailure(t *testing.T) {
	source, err := fixture.New("../../internal/fixture/assets")
	if err != nil {
		t.Fatal(err)
	}
	session, err := app.New(context.Background(), source, playback.New(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	a := &application{session: session, closeError: playback.Unsupported}
	a.onClose()
	if !errors.Is(a.closeError, playback.Unsupported) {
		t.Fatal("window close erased startup failure", a.closeError)
	}
	if session.State().Playback.State != playback.Closed {
		t.Fatal("playback was not disposed")
	}
}

func TestDisposeBeforeStartIsTerminalAndIdempotent(t *testing.T) {
	source, err := fixture.New("../../internal/fixture/assets")
	if err != nil {
		t.Fatal(err)
	}
	session, err := app.New(context.Background(), source, playback.New(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	a := &application{session: session}
	if err := a.dispose(); err != nil {
		t.Fatal(err)
	}
	if err := a.dispose(); err != nil {
		t.Fatal(err)
	}
	if s := session.State(); s.Playback.State != playback.Closed {
		t.Fatal(s)
	}
	if err := session.Play(); err != playback.IsClosed {
		t.Fatal(err)
	}
}

package main

import (
	"errors"
	"testing"

	"my-bilibili/experiments/playback/contract"
	"my-bilibili/internal/app"
	"my-bilibili/internal/fixture"
)

func TestWindowClosePreservesStartupFailure(t *testing.T) {
	source, err := fixture.New("../../internal/fixture/assets")
	if err != nil {
		t.Fatal(err)
	}
	session, err := app.New(source, contract.New(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	a := &application{session: session, closeError: contract.Unsupported}
	a.onClose()
	if !errors.Is(a.closeError, contract.Unsupported) {
		t.Fatal("window close erased startup failure", a.closeError)
	}
	if session.State().Playback.State != contract.Closed {
		t.Fatal("playback was not disposed")
	}
}

func TestDisposeBeforeStartIsTerminalAndIdempotent(t *testing.T) {
	source, err := fixture.New("../../internal/fixture/assets")
	if err != nil {
		t.Fatal(err)
	}
	session, err := app.New(source, contract.New(nil, nil))
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
	if s := session.State(); s.Playback.State != contract.Closed {
		t.Fatal(s)
	}
	if err := session.Play(); err != contract.IsClosed {
		t.Fatal(err)
	}
}

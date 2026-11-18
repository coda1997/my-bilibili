package main

import (
	"bytes"
	"errors"
	"sync"

	"my-bilibili/internal/previewbundle"
)

// Only named synthetic client scenes capture stdout. Raw lines are discarded;
// only these two exact parsed observations can become cleanup evidence.
type clientReceipts struct {
	mu        sync.Mutex
	remaining int
	pending   []byte
	values    map[string]clientObserved
	closed    bool
	invalid   bool
}

func newClientReceipts() *clientReceipts {
	return &clientReceipts{remaining: 65536, values: make(map[string]clientObserved)}
}

func (r *clientReceipts) Write(data []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := len(data)
	if n > r.remaining {
		r.invalid, r.remaining, r.pending = true, 0, nil
		return n, nil
	}
	r.remaining -= n
	r.pending = append(r.pending, data...)
	for {
		end := bytes.IndexByte(r.pending, '\n')
		if end < 0 {
			break
		}
		line := r.pending[:end]
		r.pending = r.pending[end+1:]
		if !bytes.HasPrefix(bytes.TrimSpace(line), []byte("{")) {
			continue
		}
		receipt, err := previewbundle.ParseClientReceipt(line)
		if err != nil || r.closed {
			r.invalid = true
			continue
		}
		if _, ok := r.values[receipt.Event]; ok {
			r.invalid = true
			continue
		}
		r.values[receipt.Event] = receipt.Observation
		r.closed = receipt.Event == previewbundle.ClientWindowClosed
	}
	return n, nil
}

func (r *clientReceipts) get(event string) (clientObserved, error) {
	if r == nil {
		return clientObserved{}, errors.New("client receipts unavailable")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.invalid || len(r.pending) != 0 {
		return clientObserved{}, errors.New("client receipt stream invalid or incomplete")
	}
	o, ok := r.values[event]
	if !ok {
		return clientObserved{}, errors.New("required client receipt missing")
	}
	return o, nil
}

func clientClosed(p *ownedProcess) (clientObserved, error) {
	o, err := p.clientReceipts.get(previewbundle.ClientWindowClosed)
	if err != nil {
		return clientObserved{}, err
	}
	return o, nil // ParseClientReceipt already requires every closed-owner invariant.
}

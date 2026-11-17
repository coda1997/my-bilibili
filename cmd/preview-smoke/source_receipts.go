package main

import (
	"bytes"
	"errors"
	"sync"

	"my-bilibili/internal/previewbundle"
)

// Only an explicitly synthetic source scenario captures stdout. Retain the two
// validated receipts, never raw native output or arbitrary decoded fields.
type sourceReceipts struct {
	mu        sync.Mutex
	remaining int
	pending   []byte
	values    map[string]observed
	invalid   bool
}

func newSourceReceipts() *sourceReceipts {
	return &sourceReceipts{remaining: 65536, values: make(map[string]observed)}
}

func (s *sourceReceipts) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(data)
	if n > s.remaining {
		s.invalid, s.remaining, s.pending = true, 0, nil
		return n, nil
	}
	s.remaining -= n
	s.pending = append(s.pending, data...)
	for {
		end := bytes.IndexByte(s.pending, '\n')
		if end < 0 {
			break
		}
		line := s.pending[:end]
		s.pending = s.pending[end+1:]
		// Unstructured platform diagnostics are not evidence and are discarded.
		if !bytes.HasPrefix(bytes.TrimSpace(line), []byte("{")) {
			continue
		}
		receipt, err := previewbundle.ParseSourceReceipt(line)
		if err != nil {
			s.invalid = true
			continue
		}
		if _, duplicate := s.values[receipt.Event]; duplicate {
			s.invalid = true
			continue
		}
		s.values[receipt.Event] = receipt.Observation
	}
	return n, nil
}

func (s *sourceReceipts) get(event string) (observed, error) {
	if s == nil {
		return observed{}, errors.New("source receipts unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.invalid || len(s.pending) != 0 {
		return observed{}, errors.New("source receipt stream invalid or incomplete")
	}
	value, ok := s.values[event]
	if !ok {
		return observed{}, errors.New("required source receipt missing")
	}
	value.State.Video.Parts = append(value.State.Video.Parts[:0:0], value.State.Video.Parts...)
	return value, nil
}

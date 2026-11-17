package main

import (
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/history"
)

type verificationHistoryProof struct {
	Loaded, Saved, Closed bool
	Digest                string
	Records               []history.Record
	Error                 history.Code
}

// Store calls belong to history.Manager's one I/O worker. The small memory
// mutex protects only copied proof/allowed refs; it never covers delegated I/O
// or hashing, so UI acceptance cannot block on a filesystem syscall.
type verificationHistory struct {
	base                               history.Store
	directory, canonical               string
	expected                           *verificationContinuation
	acquired, loaded, terminal, closed bool
	baseline                           []history.Record
	mu                                 sync.Mutex
	allowed                            map[catalog.PartRef]bool
	last                               verificationHistoryProof
}

var _ history.Store = (*verificationHistory)(nil)

func (r *verificationRun) HistoryStore(base history.Store) (history.Store, error) {
	if base == nil || r.history != nil {
		return nil, history.Invalid
	}
	h := &verificationHistory{base: base, directory: r.directory, expected: r.continuation, allowed: make(map[catalog.PartRef]bool)}
	if r.continuation != nil {
		h.allowed[r.continuation.ref()] = true
	}
	r.history = h
	return h, nil
}

func verificationHistoryCode(err error) history.Code {
	if code, ok := err.(history.Code); ok {
		switch code {
		case history.Invalid, history.Storage, history.InUse, history.LockUnsupported, history.Corrupt, history.UnsupportedVersion, history.Uncertain, history.Closed:
			return code
		}
	}
	return history.Storage
}
func (h *verificationHistory) fail(err error) error {
	code := verificationHistoryCode(err)
	h.mu.Lock()
	if h.last.Error == history.None {
		h.last.Error = code
	}
	h.mu.Unlock()
	return code
}
func (h *verificationHistory) proof() verificationHistoryProof {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := h.last
	result.Records = append([]history.Record(nil), h.last.Records...)
	return result
}
func (h *verificationHistory) allow(ref catalog.PartRef) error {
	if !verificationRef(ref) {
		return history.Invalid
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.allowed[ref] && len(h.allowed) >= 2 {
		return history.Invalid
	}
	for old := range h.allowed {
		if old.Video != ref.Video {
			return history.Invalid
		}
	}
	h.allowed[ref] = true
	return nil
}
func verificationQuantize(records []history.Record) []history.Record {
	result := append([]history.Record(nil), records...)
	for i := range result {
		result[i].Position = result[i].Position / time.Millisecond * time.Millisecond
		result[i].Duration = result[i].Duration / time.Millisecond * time.Millisecond
	}
	return result
}
func verificationRecord(records []history.Record, ref catalog.PartRef) (history.Record, bool) {
	for _, record := range records {
		if record.Ref == ref {
			return record, true
		}
	}
	return history.Record{}, false
}

func (h *verificationHistory) Acquire() error {
	if h.terminal {
		return history.Closed
	}
	if h.acquired {
		if h.canonical == "" {
			return history.Storage
		}
		return nil
	}
	if err := h.base.Acquire(); err != nil {
		return h.fail(err)
	}
	// The underlying Store now holds its canonical lock. Resolve our same
	// injected directory only after that ownership was actually obtained.
	h.acquired = true
	canonical, err := filepath.EvalSymlinks(h.directory)
	if err != nil || !verificationDirectory(canonical) {
		return h.fail(history.Storage)
	}
	h.canonical = canonical
	return nil
}
func (h *verificationHistory) Load() ([]history.Record, error) {
	if h.terminal {
		return nil, history.Closed
	}
	if !h.acquired || h.canonical == "" {
		return nil, h.fail(history.Storage)
	}
	records, err := h.base.Load()
	if err != nil {
		return nil, h.fail(err)
	}
	digest, exists, err := verificationDigest(filepath.Join(h.canonical, "history.json"), verificationHistoryLimit, true, true)
	if err != nil || !exists && len(records) != 0 {
		return nil, h.fail(history.Storage)
	}
	if h.expected != nil {
		record, found := verificationRecord(records, h.expected.ref())
		if !exists || digest != h.expected.HistorySHA256 || !found || record.Completed || record.Position != time.Duration(h.expected.PositionMS)*time.Millisecond || record.Duration <= record.Position {
			return nil, h.fail(history.Corrupt)
		}
	}
	h.loaded = true
	h.baseline = verificationQuantize(records)
	h.mu.Lock()
	h.last.Loaded = true
	h.mu.Unlock()
	return append([]history.Record(nil), records...), nil
}

func (h *verificationHistory) Save(records []history.Record) error {
	if h.terminal {
		return history.Closed
	}
	if !h.acquired || !h.loaded || h.canonical == "" {
		return h.fail(history.Storage)
	}
	if len(records) == 0 || len(records) > 500 {
		return h.fail(history.Invalid)
	}
	h.mu.Lock()
	allowed := make(map[catalog.PartRef]bool, len(h.allowed))
	for ref := range h.allowed {
		allowed[ref] = true
	}
	h.mu.Unlock()
	owned := verificationQuantize(records)
	byRef := make(map[catalog.PartRef]history.Record, len(owned))
	for _, record := range owned {
		if _, duplicate := byRef[record.Ref]; duplicate {
			return h.fail(history.Invalid)
		}
		byRef[record.Ref] = record
		old, found := verificationRecord(h.baseline, record.Ref)
		if !allowed[record.Ref] && (!found || !reflect.DeepEqual(old, record)) {
			return h.fail(history.Invalid)
		}
	}
	for _, old := range h.baseline {
		if _, found := byRef[old.Ref]; !found {
			return h.fail(history.Invalid)
		}
	}
	if len(allowed) == 0 {
		return h.fail(history.Invalid)
	}
	if err := h.base.Save(records); err != nil {
		return h.fail(err)
	}
	digest, exists, err := verificationDigest(filepath.Join(h.canonical, "history.json"), verificationHistoryLimit, true, false)
	if err != nil || !exists {
		return h.fail(history.Storage)
	}
	h.baseline = owned
	h.mu.Lock()
	h.last.Saved, h.last.Digest = true, digest
	h.last.Records = append([]history.Record(nil), owned...)
	h.mu.Unlock()
	return nil
}

func (h *verificationHistory) Close() error {
	h.terminal = true
	if h.closed {
		return nil
	}
	if err := h.base.Close(); err != nil {
		return h.fail(err)
	}
	h.closed = true
	h.mu.Lock()
	h.last.Closed = true
	h.mu.Unlock()
	return nil
}

package history

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

const checkpointInterval = 5 * time.Second
const recordLimit = 500

// Manager owns history intent on the UI thread. Its one worker owns every
// Store call; Tick accepts copied results and acknowledges their completion.
type Manager struct {
	store Store
	state State
	now   func() time.Time

	requests chan storeRequest
	results  chan storeResult
	retired  chan struct{}
	done     chan struct{}
	worker   bool
	started  bool
	acquired bool
	loaded   bool
	pending  *storeRequest
	failed   storeOperation

	visit          uint64
	ref            catalog.PartRef
	suppressed     uint64
	playing        bool
	useful         bool
	lastCheckpoint time.Time
	closing        bool
	cleaning       bool
	closeAccepted  bool
}

// New transfers the Store to this Manager without calling it or starting I/O.
func New(store Store) (*Manager, error) {
	if store == nil {
		return nil, Invalid
	}
	return &Manager{
		store: store, state: State{Phase: Initial, Records: []Record{}}, now: time.Now,
		requests: make(chan storeRequest, 1), results: make(chan storeResult, 1),
		retired: make(chan struct{}), done: make(chan struct{}),
	}, nil
}

func (m *Manager) State() State {
	state := m.state
	state.Records = slices.Clone(state.Records)
	return state
}

func (m *Manager) Done() <-chan struct{} { return m.done }

func (m *Manager) startWorker() {
	if !m.worker {
		m.worker = true
		go runStore(m.store, m.requests, m.results, m.retired)
	}
}

func (m *Manager) Start() error {
	if m.closing {
		return Closed
	}
	if m.started {
		return nil
	}
	m.started = true
	m.startWorker()
	m.submit(storeRequest{operation: loadStore})
	return nil
}

func (m *Manager) mutable() error {
	if m.closing {
		return Closed
	}
	if !m.loaded {
		return Invalid
	}
	return nil
}

// Observe consumes actual accepted native samples, never a seek command target.
// Identity and plain titles were established by catalog conversion.
func (m *Manager) Observe(sample Sample) error {
	if err := m.mutable(); err != nil {
		return err
	}
	if sample.Visit == 0 || sample.Visit < m.visit || sample.Record.Ref.Video == "" || sample.Record.Ref.Part == "" ||
		sample.Visit == m.visit && sample.Record.Ref != m.ref {
		return Invalid
	}
	transition := sample.Visit != m.visit
	m.visit, m.ref = sample.Visit, sample.Record.Ref
	m.playing = sample.Playback == playback.Playing
	if transition {
		m.useful = false
	}
	if sample.Visit == m.suppressed {
		return nil
	}
	// A paused zero after useful progress is an actual seek sample. A fresh
	// engine's Ready-at-zero sample must not erase that progress while its
	// quality/resume seek is still pending.
	useful := sample.Record.Position > 0 || m.playing || sample.Playback == playback.Ended ||
		m.useful && sample.Playback == playback.Paused
	if sample.Record.Duration <= 0 || !useful {
		if transition {
			m.checkpoint()
		}
		return nil
	}
	m.useful = true
	record := sample.Record
	record.Position = min(max(record.Position, 0), record.Duration)
	record.Title, record.PartTitle = truncateTitle(record.Title), truncateTitle(record.PartTitle)
	record.Completed = sample.Playback == playback.Ended
	index := slices.IndexFunc(m.state.Records, func(r Record) bool { return r.Ref == record.Ref })
	changed := transition || index < 0
	if index >= 0 {
		previous := m.state.Records[index]
		record.UpdatedAt = previous.UpdatedAt
		changed = changed || record != previous
	}
	if changed {
		record.UpdatedAt = m.now().UTC()
		if index >= 0 {
			m.state.Records[index] = record
		} else {
			m.state.Records = append(m.state.Records, record)
		}
		m.orderRecords()
		m.changed()
	}
	if transition || index < 0 || sample.Playback == playback.Ended || sample.Playback == playback.Paused {
		m.checkpoint()
	} else {
		m.periodic()
	}
	return nil
}

func truncateTitle(text string) string {
	if len(text) <= 256 {
		return text
	}
	end := 256
	for !utf8.RuneStart(text[end]) {
		end--
	}
	prefix := text[:end]
	if strings.TrimSpace(prefix) == "" {
		// The byte cap fell before the first visible rune. Keep the title instead
		// of saving a whitespace-only prefix; apply the same cap to the remainder.
		return truncateTitle(strings.TrimSpace(text[end:]))
	}
	return prefix
}

func (m *Manager) changed() {
	m.state.Revision++
	m.state.Dirty = true
}

func (m *Manager) orderRecords() {
	slices.SortFunc(m.state.Records, func(a, b Record) int {
		if order := b.UpdatedAt.Compare(a.UpdatedAt); order != 0 {
			return order
		}
		if order := strings.Compare(string(a.Ref.Video), string(b.Ref.Video)); order != 0 {
			return order
		}
		return strings.Compare(string(a.Ref.Part), string(b.Ref.Part))
	})
	if len(m.state.Records) > recordLimit {
		m.state.Records = m.state.Records[:recordLimit:recordLimit]
	}
}

func (m *Manager) Delete(ref catalog.PartRef) error {
	if err := m.mutable(); err != nil {
		return err
	}
	if ref.Video == "" || ref.Part == "" {
		return Invalid
	}
	if ref == m.ref {
		m.suppressed = m.visit
	}
	before := len(m.state.Records)
	m.state.Records = slices.DeleteFunc(m.state.Records, func(record Record) bool { return record.Ref == ref })
	if len(m.state.Records) != before {
		m.changed()
		m.checkpoint()
	}
	return nil
}

func (m *Manager) Clear() error {
	if err := m.mutable(); err != nil {
		return err
	}
	m.suppressed = m.visit
	if len(m.state.Records) > 0 {
		m.state.Records = []Record{}
		m.changed()
		m.checkpoint()
	}
	return nil
}

func (m *Manager) ResetCorrupt() error {
	if m.closing {
		return Closed
	}
	if !m.acquired || m.loaded || m.state.Busy || m.failed != loadStore {
		return Invalid
	}
	m.state.Records = []Record{}
	m.changed()
	m.failed = noOperation
	m.submit(storeRequest{operation: resetStore, records: []Record{}, revision: m.state.Revision})
	return nil
}

func (m *Manager) Retry() error {
	if m.state.Phase == Done || m.closeAccepted {
		return Closed
	}
	if m.state.Busy || m.failed == noOperation {
		return Invalid
	}
	operation := m.failed
	m.failed = noOperation
	request := storeRequest{operation: operation}
	if operation == saveStore || operation == resetStore {
		request.records = slices.Clone(m.state.Records)
		request.revision = m.state.Revision
	}
	m.submit(request)
	return nil
}

func (m *Manager) submit(request storeRequest) {
	m.state.Busy = true
	if request.operation == loadStore || request.operation == resetStore {
		m.state.Phase = Loading
	} else if m.closing {
		m.state.Phase = Closing
	}
	// Only the UI thread submits, and Busy stays true through acknowledgment.
	// The previous request has left the capacity-one mailbox before acceptance.
	m.requests <- request
}

func (m *Manager) periodic() {
	if m.playing && m.now().Sub(m.lastCheckpoint) >= checkpointInterval {
		m.checkpoint()
	}
}

func (m *Manager) checkpoint() {
	if !m.loaded || !m.state.Dirty || m.failed != noOperation || m.cleaning {
		return
	}
	request := storeRequest{operation: saveStore, records: slices.Clone(m.state.Records), revision: m.state.Revision}
	m.lastCheckpoint = m.now()
	if m.state.Busy {
		m.pending = &request
		return
	}
	m.submit(request)
}

func historyError(err error) Code {
	var code Code
	if errors.As(err, &code) {
		switch code {
		case Invalid, Storage, InUse, LockUnsupported, Corrupt, UnsupportedVersion, Uncertain, Closed:
			return code
		}
	}
	return Storage
}

func (m *Manager) accept(result storeResult) {
	defer close(result.consumed)
	m.state.Busy = false
	operation := result.request.operation
	if operation == loadStore {
		m.acquired = result.acquired
	}
	if result.err != nil {
		m.state.Phase, m.state.Error = Failed, historyError(result.err)
		m.failed = operation
		m.pending = nil
		if operation == closeStore {
			m.state.Error = Storage
		}
		if m.closing && operation == loadStore {
			m.beginCleanup()
		}
		return
	}
	m.state.Phase = Ready
	if operation == loadStore {
		m.loaded = true
		m.state.Records = slices.Clone(result.records)
		trimmed := len(m.state.Records) > recordLimit
		m.orderRecords()
		m.state.Error = None
		if trimmed {
			m.changed()
			m.checkpoint()
		}
	} else if operation == saveStore || operation == resetStore {
		m.loaded = true
		m.state.SavedRevision = result.request.revision
		m.state.Dirty = m.state.Revision != m.state.SavedRevision
		if !m.state.Dirty {
			m.state.Error = None
		}
	} else if operation == closeStore {
		m.closeAccepted = true
		m.state.Phase = Closing
		return
	}
	if m.pending != nil && !m.state.Busy {
		request := *m.pending
		m.pending = nil
		if request.revision > m.state.SavedRevision {
			m.submit(request)
		}
	}
	if m.closing && !m.state.Busy {
		if m.state.Dirty {
			m.checkpoint()
		} else {
			m.beginCleanup()
		}
	}
}

// Tick never blocks on I/O and does not create a goroutine. It must also be
// pumped during minimized shutdown so cleanup acceptance does not need paint.
func (m *Manager) Tick() {
	if m.state.Phase == Done {
		return
	}
	select {
	case result := <-m.results:
		m.accept(result)
	default:
	}
	if m.closeAccepted {
		select {
		case <-m.retired:
			m.state.Phase, m.state.Error = Done, None
			close(m.done)
		default:
		}
		return
	}
	if !m.closing {
		m.periodic()
	}
}

func (m *Manager) beginCleanup() {
	m.cleaning = true
	m.failed = noOperation
	m.pending = nil
	m.submit(storeRequest{operation: closeStore})
}

func (m *Manager) RequestClose(discardUnsaved bool) bool {
	if m.state.Phase == Done {
		return true
	}
	if discardUnsaved && m.state.Busy {
		return false
	}
	m.closing = true
	m.startWorker()
	if m.state.Busy || m.closeAccepted || m.cleaning {
		return false
	}
	if discardUnsaved {
		m.pending = nil
		m.state.Dirty = false
		m.beginCleanup()
	} else if m.state.Dirty {
		m.checkpoint()
	} else {
		m.beginCleanup()
	}
	return false
}

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"my-bilibili/internal/adapters/localhistory"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/history"
)

type verifyHistorySpy struct {
	base                                   history.Store
	acquires, loads, saves, closes         int
	acquireErr, loadErr, saveErr, closeErr error
	afterLoad, afterSave, afterClose       func()
}

func (s *verifyHistorySpy) Acquire() error {
	s.acquires++
	if s.acquireErr != nil {
		return s.acquireErr
	}
	return s.base.Acquire()
}
func (s *verifyHistorySpy) Load() ([]history.Record, error) {
	s.loads++
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	records, err := s.base.Load()
	if s.afterLoad != nil {
		s.afterLoad()
	}
	return records, err
}
func (s *verifyHistorySpy) Save(records []history.Record) error {
	s.saves++
	if s.saveErr != nil {
		return s.saveErr
	}
	err := s.base.Save(records)
	if s.afterSave != nil {
		s.afterSave()
	}
	return err
}
func (s *verifyHistorySpy) Close() error {
	s.closes++
	if s.closeErr != nil {
		return s.closeErr
	}
	err := s.base.Close()
	if s.afterClose != nil {
		s.afterClose()
	}
	return err
}
func verifyTestRecord() history.Record {
	return history.Record{Ref: catalog.PartRef{Video: verifyTestVideo, Part: "101"}, Title: "Original synthetic video", PartTitle: "First synthetic part", Position: 1234567890 * time.Nanosecond, Duration: 30666777777 * time.Nanosecond, UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}
func verifyTestSeed(t *testing.T, directory string, records []history.Record) {
	t.Helper()
	s, err := localhistory.New(directory)
	if err != nil {
		t.Fatal("synthetic history constructor failed")
	}
	if s.Acquire() != nil || s.Save(records) != nil || s.Close() != nil {
		t.Fatal("synthetic history seed failed")
	}
}
func verifyTestOwnedHistory(t *testing.T, r *verificationRun) (*verificationHistory, *verifyHistorySpy) {
	t.Helper()
	base, err := localhistory.New(r.HistoryDirectory())
	if err != nil {
		t.Fatal("synthetic store failed")
	}
	spy := &verifyHistorySpy{base: base}
	wrapped, err := r.HistoryStore(spy)
	if err != nil {
		t.Fatal("history wrapper failed")
	}
	h := wrapped.(*verificationHistory)
	t.Cleanup(func() { spy.closeErr = nil; _ = h.Close() })
	return h, spy
}

func TestVerificationHistoryOwnershipDigestAndPersistedMilliseconds(t *testing.T) {
	r, _ := verifyTestRun(t)
	h, spy := verifyTestOwnedHistory(t, r)
	if h.Acquire() != nil {
		t.Fatal("owned Acquire failed")
	}
	competitor, err := localhistory.New(r.directory)
	if err != nil {
		t.Fatal("competing store failed")
	}
	defer competitor.Close()
	spy.afterLoad = func() {
		if competitor.Acquire() != history.InUse {
			t.Error("Load/hash boundary lost actual filesystem ownership")
		}
	}
	spy.afterSave = func() {
		if competitor.Acquire() != history.InUse {
			t.Error("Save/hash boundary lost actual filesystem ownership")
		}
	}
	if records, err := h.Load(); err != nil || len(records) != 0 {
		t.Fatal("owned missing file was not empty")
	}
	record := verifyTestRecord()
	if h.allow(record.Ref) != nil || h.Save([]history.Record{record}) != nil {
		t.Fatal("authorized useful progress did not save")
	}
	proof := h.proof()
	if !proof.Loaded || !proof.Saved || proof.Closed || proof.Error != history.None || len(proof.Records) != 1 || proof.Records[0].Position != 1234*time.Millisecond || proof.Records[0].Duration != 30666*time.Millisecond {
		t.Fatal("proof did not match persisted integer milliseconds")
	}
	digest, exists, err := verificationDigest(filepath.Join(r.directory, "history.json"), verificationHistoryLimit, true, false)
	if err != nil || !exists || digest != proof.Digest {
		t.Fatal("saved digest did not bind exact owned bytes")
	}
	proof.Records[0].Position = 0
	if h.proof().Records[0].Position == 0 {
		t.Fatal("proof aliased caller slice")
	}
	if h.Close() != nil || !h.proof().Closed {
		t.Fatal("actual final close not recorded")
	}
	if _, err := h.Load(); err != history.Closed || h.Save([]history.Record{record}) != history.Closed || h.Acquire() != history.Closed {
		t.Fatal("terminal wrapper delegated later I/O")
	}
	if spy.loads != 1 || spy.saves != 1 || spy.acquires != 1 || spy.closes != 1 {
		t.Fatal("wrapper performed unowned or post-close operations")
	}
	if competitor.Acquire() != nil {
		t.Fatal("final close did not release real history ownership")
	}
}

func TestVerificationHistoryCannotReadWithoutAcquireOrBypassInUse(t *testing.T) {
	for _, kind := range []string{"unacquired", "in-use", "unsupported", "acquire-error"} {
		t.Run(kind, func(t *testing.T) {
			r, _ := verifyTestRun(t)
			h, spy := verifyTestOwnedHistory(t, r)
			var owner *localhistory.Store
			if kind == "in-use" {
				owner, _ = localhistory.New(r.directory)
				if owner.Acquire() != nil {
					t.Fatal("test owner failed")
				}
				defer owner.Close()
			}
			if kind == "unsupported" {
				spy.acquireErr = history.LockUnsupported
			}
			if kind == "acquire-error" {
				spy.acquireErr = errors.New(verifyTestPrivate)
			}
			if kind != "unacquired" {
				if err := h.Acquire(); err == nil {
					t.Fatal("acquisition refusal bypassed")
				} else {
					verifyTestSafe(t, err)
				}
			}
			if records, err := h.Load(); err == nil || records != nil {
				t.Fatal("unowned history published")
			}
			if h.Save([]history.Record{verifyTestRecord()}) == nil || spy.loads != 0 || spy.saves != 0 {
				t.Fatal("failed acquisition reached filesystem operations")
			}
		})
	}
}

func TestVerificationHistoryPreservesUnrelatedRowsAndRejectsDeletionResetOrEviction(t *testing.T) {
	for _, kind := range []string{"delete", "clear", "change-other", "new-other", "duplicate", "evict", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			r, _ := verifyTestRun(t)
			other := verifyTestRecord()
			other.Ref = catalog.PartRef{Video: "BV0000000002", Part: "201"}
			other.Title = "Existing synthetic row"
			initial := []history.Record{other}
			if kind == "evict" || kind == "oversize" {
				initial = nil
				for i := 1; i <= 500; i++ {
					row := other
					row.Ref.Part = catalog.PartID(strconv.Itoa(i))
					initial = append(initial, row)
				}
			}
			verifyTestSeed(t, r.directory, initial)
			before, err := os.ReadFile(filepath.Join(r.directory, "history.json"))
			if err != nil {
				t.Fatal("synthetic original read failed")
			}
			h, spy := verifyTestOwnedHistory(t, r)
			if h.Acquire() != nil {
				t.Fatal("test acquire failed")
			}
			rows, err := h.Load()
			if err != nil {
				t.Fatal("test load failed")
			}
			target := verifyTestRecord()
			if h.allow(target.Ref) != nil {
				t.Fatal("test target permission failed")
			}
			changed := append(append([]history.Record(nil), rows...), target)
			switch kind {
			case "delete":
				changed = []history.Record{target}
			case "clear":
				changed = nil
			case "change-other":
				changed[0].Title = verifyTestPrivate
			case "new-other":
				extra := other
				extra.Ref.Part = "999"
				changed = append(changed, extra)
			case "duplicate":
				changed = append(changed, target)
			case "evict":
				changed = changed[1:]
			}
			if err := h.Save(changed); err != history.Invalid || spy.saves != 0 {
				t.Fatal("verification changed/deleted unplanned history")
			}
			after, err := os.ReadFile(filepath.Join(r.directory, "history.json"))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("rejected history mutation changed bytes")
			}
		})
	}
	r, _ := verifyTestRun(t)
	h, _ := verifyTestOwnedHistory(t, r)
	if h.allow(catalog.PartRef{Video: verifyTestVideo, Part: "101"}) != nil || h.allow(catalog.PartRef{Video: verifyTestVideo, Part: "102"}) != nil || h.allow(catalog.PartRef{Video: verifyTestVideo, Part: "103"}) != history.Invalid || h.allow(catalog.PartRef{Video: "BV0000000002", Part: "201"}) != history.Invalid {
		t.Fatal("unbounded/different-video progress authorization")
	}
}

func TestVerificationHistoryHashReadAndCloseFailuresCannotClaimSavedCleanup(t *testing.T) {
	for _, kind := range []string{"load-error", "load-hash-error", "save-error", "save-hash-error", "close-error"} {
		t.Run(kind, func(t *testing.T) {
			r, _ := verifyTestRun(t)
			record := verifyTestRecord()
			verifyTestSeed(t, r.directory, []history.Record{record})
			h, spy := verifyTestOwnedHistory(t, r)
			if h.Acquire() != nil {
				t.Fatal("test acquire failed")
			}
			if kind == "load-error" {
				spy.loadErr = errors.New(verifyTestPrivate)
			}
			if kind == "load-hash-error" {
				spy.afterLoad = func() {
					if os.Chmod(filepath.Join(r.directory, "history.json"), 0644) != nil {
						t.Error("test mode injection failed")
					}
				}
			}
			_, err := h.Load()
			if kind == "load-error" || kind == "load-hash-error" {
				if err == nil || h.proof().Loaded {
					t.Fatal("failed load/hash became published records")
				}
				return
			}
			if err != nil || h.allow(record.Ref) != nil {
				t.Fatal("test load/target failed")
			}
			if kind == "save-error" {
				spy.saveErr = errors.New(verifyTestPrivate)
			}
			if kind == "save-hash-error" {
				spy.afterSave = func() {
					if os.Chmod(filepath.Join(r.directory, "history.json"), 0644) != nil {
						t.Error("test mode injection failed")
					}
				}
			}
			err = h.Save([]history.Record{record})
			if kind == "save-error" || kind == "save-hash-error" {
				if err == nil || h.proof().Saved {
					t.Fatal("failed save/hash became saved proof")
				}
				return
			}
			if err != nil {
				t.Fatal("test save failed")
			}
			spy.closeErr = errors.New(verifyTestPrivate)
			if h.Close() != history.Storage || h.proof().Closed || h.proof().Error != history.Storage {
				t.Fatal("release uncertainty became clean close")
			}
			if _, err = h.Load(); err != history.Closed || h.Save([]history.Record{record}) != history.Closed {
				t.Fatal("close error allowed reopening data")
			}
			spy.closeErr = nil
			if h.Close() != nil || !h.proof().Closed || h.proof().Error != history.Storage || spy.loads != 1 || spy.saves != 1 {
				t.Fatal("cleanup retry read/saved again or erased first verification failure")
			}
		})
	}
}

func TestVerificationLaunchTwoChecksExactOwnedDigestAndRecordBeforePublication(t *testing.T) {
	for _, kind := range []string{"matching", "changed-bytes", "different-ref", "different-position", "in-use"} {
		t.Run(kind, func(t *testing.T) {
			r, f := verifyTestRun(t)
			h, _ := verifyTestOwnedHistory(t, r)
			record := verifyTestRecord()
			if h.Acquire() != nil {
				t.Fatal("first acquire failed")
			}
			if _, err := h.Load(); err != nil {
				t.Fatal("first load failed")
			}
			if h.allow(record.Ref) != nil || h.Save([]history.Record{record}) != nil || h.Close() != nil {
				t.Fatal("first saved checkpoint failed")
			}
			proof := h.proof()
			c := verifyTestContinuation(r)
			c.PositionMS = int64(proof.Records[0].Position / time.Millisecond)
			c.HistorySHA256 = proof.Digest
			if kind == "different-ref" {
				c.PartID = "999"
			}
			if kind == "different-position" {
				c.PositionMS++
			}
			if verificationCreate(r.continuationPath, verifyTestJSON(t, c)) != nil {
				t.Fatal("continuation fixture failed")
			}
			f.writePlan(t, verifyTestPlan(2))
			second := f.prepare(t)
			if kind == "changed-bytes" {
				changed := record
				changed.Title = "Another original synthetic title"
				verifyTestSeed(t, r.directory, []history.Record{changed})
			}
			var owner *localhistory.Store
			if kind == "in-use" {
				owner, _ = localhistory.New(r.directory)
				if owner.Acquire() != nil {
					t.Fatal("competing owner failed")
				}
				defer owner.Close()
			}
			secondStore, spy := verifyTestOwnedHistory(t, second)
			if kind == "in-use" {
				if secondStore.Acquire() != history.InUse || spy.loads != 0 {
					t.Fatal("resume bypassed live history owner")
				}
				return
			}
			if secondStore.Acquire() != nil {
				t.Fatal("second acquire failed")
			}
			rows, err := secondStore.Load()
			if kind == "matching" {
				if err != nil || len(rows) != 1 || rows[0].Ref != record.Ref || rows[0].Position != proof.Records[0].Position {
					t.Fatal("matching restart record not restored")
				}
			} else if err != history.Corrupt || rows != nil || secondStore.proof().Loaded {
				t.Fatal("hash/ref/progress mismatch published records")
			}
			if spy.saves != 0 {
				t.Fatal("restart verification rewrote existing history")
			}
		})
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"time"

	"my-bilibili/internal/account"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/history"
	"my-bilibili/internal/playback"
	"my-bilibili/internal/previewbundle"
)

func verificationResultError(code string) error {
	if code == "" {
		return nil
	}
	for _, candidate := range []error{catalog.Code(code), account.Code(code), history.Code(code), playback.ErrorCode(code)} {
		if verificationError(candidate) == code {
			return candidate
		}
	}
	return catalog.Unavailable
}

func (r *verificationRun) continuationOwned() bool {
	info, err := os.Lstat(r.continuationPath)
	if err != nil || r.continuationIdentity == nil || !os.SameFile(info, r.continuationIdentity) {
		return false
	}
	data, err := verificationRead(r.continuationPath, verificationPlanLimit, true)
	return err == nil && bytes.Equal(data, r.continuationData)
}
func (r *verificationRun) preserveContinuation() error {
	data, err := verificationRead(r.continuationPath, verificationPlanLimit, true)
	if err == nil {
		if bytes.Equal(data, r.continuationData) {
			return nil
		}
		return history.Storage
	}
	if _, statErr := os.Lstat(r.continuationPath); !os.IsNotExist(statErr) {
		return history.Storage
	}
	return verificationCreate(r.continuationPath, r.continuationData)
}

// Finish runs only after Report transfers UI ownership and the actual normal
// app has joined its owners. It reads no history file here: the last digest was
// captured by the Store's I/O worker before it released its real lock.
func (r *verificationRun) Finish(report verificationReport, out io.Writer) error {
	if r.finished {
		return catalog.Closed
	}
	r.finished = true
	r.Stop()
	if out == nil || len(report.receipts) == 0 || len(report.receipts) > 16 || report.receipts[len(report.receipts)-1].Event != "closed" {
		return catalog.Invalid
	}
	code := report.failure
	fail := func(next string) {
		if code == "" {
			code = next
		}
	}
	if !report.complete || !report.closed {
		fail(string(catalog.Unavailable))
	}
	if !r.now().Before(r.started.Add(verificationProcessLimit)) {
		fail(string(catalog.Timeout))
	}
	budget := r.budget.snapshot()
	if budget.Error != catalog.None {
		fail(string(budget.Error))
	}
	if budget.APIActive != 0 || budget.CoverActive != 0 {
		fail(string(catalog.Unavailable))
	}
	var proof verificationHistoryProof
	if r.history != nil {
		proof = r.history.proof()
	}
	if proof.Error != history.None {
		fail(string(proof.Error))
	}
	if !proof.Closed || !proof.Saved || !proof.Loaded {
		fail(string(history.Storage))
	}
	record, found := verificationRecord(proof.Records, report.ref)
	if !found || !verificationRef(report.ref) || record.Completed || record.Position <= 0 || record.Position >= record.Duration || !verificationHex(proof.Digest, 64) {
		fail(string(history.Invalid))
	}
	removed := false
	if code == "" {
		if r.plan.Launch == 1 {
			c := verificationContinuation{Version: 1, ExecutableSHA256: r.executableHash, VideoID: string(report.ref.Video), PartID: string(report.ref.Part), PositionMS: int64(record.Position / time.Millisecond), HistorySHA256: proof.Digest, APIRequests: budget.APIRequests, BootstrapRequests: budget.BootstrapRequests, CoverRequests: budget.CoverRequests, MediaOpens: budget.MediaOpens}
			data, err := json.Marshal(c)
			if err != nil {
				fail(string(history.Storage))
			} else if _, err = parseVerificationContinuation(data); err != nil {
				fail(string(history.Invalid))
			} else if r.writeContinuation(r.continuationPath, data) != nil {
				fail(string(history.Storage))
			}
		} else if r.continuation == nil || report.ref != r.continuation.ref() || !r.continuationOwned() {
			fail(string(history.Corrupt))
		} else if r.removeContinuation(r.continuationPath) != nil {
			fail(string(history.Storage))
			// Unlink may have succeeded before directory sync failed. Preserve the
			// original private bytes where possible; never claim that it rolled back.
			if r.preserveContinuation() != nil {
				fail(string(history.Storage))
			}
		} else {
			removed = true
		}
	}
	last := &report.receipts[len(report.receipts)-1]
	last.APIRequests, last.BootstrapRequests, last.CoverRequests, last.MediaOpens = budget.APIRequests, budget.BootstrapRequests, budget.CoverRequests, budget.MediaOpens
	last.HistorySaved = proof.Saved && proof.Closed && proof.Error == history.None && found
	if code != "" {
		last.Result, last.Error = "fail", code
	}
	var stream bytes.Buffer
	encoder := json.NewEncoder(&stream)
	for _, receipt := range report.receipts {
		if encoder.Encode(receipt) != nil {
			return catalog.InvalidResponse
		}
	}
	if _, err := previewbundle.ParseLiveReceipts(stream.Bytes()); err != nil {
		if removed && r.preserveContinuation() != nil {
			return history.Storage
		}
		return catalog.InvalidResponse
	}
	n, err := out.Write(stream.Bytes())
	if err != nil || n != stream.Len() {
		if removed && r.preserveContinuation() != nil {
			return history.Storage
		}
		return catalog.Unavailable
	}
	return verificationResultError(code)
}

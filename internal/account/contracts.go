// Package account owns the framework-free web sign-in lifecycle.
package account

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"
)

type Code string

const (
	None            Code = ""
	Invalid         Code = "invalid"
	Network         Code = "network"
	ServiceRejected Code = "service-rejected"
	ExpiredSession  Code = "expired"
	Storage         Code = "storage"
	Missing         Code = "missing"
	Busy            Code = "busy"
	Canceled        Code = "canceled"
	Timeout         Code = "timeout"
	Closed          Code = "closed"
)

func (c Code) Error() string { return string(c) }

// Material is opaque to views and state reports. Bytes is for adapters only.
type Material struct{ data []byte }

func NewMaterial(v []byte) Material           { return Material{data: bytes.Clone(v)} }
func (m Material) Bytes() []byte              { return bytes.Clone(m.data) }
func (m Material) Empty() bool                { return len(m.data) == 0 }
func (Material) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[private credential]") }
func (Material) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

// QR is ephemeral presentation data. Text is used only by the native encoder.
type QR struct{ text string }

func NewQR(v string) QR                 { return QR{text: v} }
func (q QR) Text() string               { return q.text }
func (q QR) Empty() bool                { return q.text == "" }
func (QR) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[private QR]") }
func (QR) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

type Phase string

const (
	Initial      Phase = "initial"
	Restoring    Phase = "restoring"
	SignedOut    Phase = "signed-out"
	Generating   Phase = "generating"
	Waiting      Phase = "waiting"
	Scanned      Phase = "scanned"
	Saving       Phase = "saving"
	SaveFailed   Phase = "save-failed"
	Verifying    Phase = "verifying"
	VerifyFailed Phase = "verify-failed"
	SignedIn     Phase = "signed-in"
	Expired      Phase = "expired"
	Failed       Phase = "failed"
	Canceling    Phase = "canceling"
	SigningOut   Phase = "signing-out"
	DeleteFailed Phase = "delete-failed"
	Closing      Phase = "closing"
	Done         Phase = "closed"
)

type State struct {
	Phase                                           Phase
	Error                                           Code
	QR                                              QR
	Generation                                      uint64
	SessionEpoch                                    uint64
	ExpiresAt                                       time.Time
	Stored, Unsaved, Busy, CloseRequested, Restored bool
}

type PollStatus string

const (
	PollWaiting  PollStatus = "waiting"
	PollScanned  PollStatus = "scanned"
	PollExpired  PollStatus = "expired"
	PollComplete PollStatus = "complete"
)

type PollResult struct {
	Status   PollStatus
	Material Material
}
type Attempt interface {
	QR() QR
	Poll(context.Context) (PollResult, error)
	Close()
}
type Gateway interface {
	Begin(context.Context) (Attempt, error)
	Verify(context.Context, Material) error
}

// SessionPublisher is a bounded memory-only leaf transaction. Manager may
// call it while locked; it must never call Manager, UI, HTTP or native storage.
type SessionPublisher interface {
	Activate(epoch uint64, material Material) error
	Revoke(epoch uint64)
}

// Store is synchronous: native Keychain calls cannot be canceled once started.
// Manager serializes them on its owned worker, outside the UI thread.
type Store interface {
	Load() (Material, error)
	Save(Material) error
	Delete() error
}
type Controller interface {
	Snapshot() State
	Restore() error
	Begin() error
	Cancel() error
	RetrySave() error
	RetryVerify() error
	Logout() error
	RequestClose(discardUnsaved bool) bool
}

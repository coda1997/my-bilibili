# Shared Go contract for the persistent-login slice

Freeze before implementation split. Only standard-library types cross account
ports. Controller writes internal/account/contracts.go first; workers use these
names/signatures. Domain implementation may add private helpers, not change these
ports or lifetime semantics without review.

~~~go
package account

import "context"
import "time"

type Code string // implements error; fixed safe values only
// Codes: None(""), Invalid("invalid"), Network("network"),
// ServiceRejected("service-rejected"), ExpiredSession("expired"),
// Storage("storage"), Missing("missing"), Busy("busy"),
// Canceled("canceled"), Timeout("timeout"), Closed("closed").

type Material struct { data []byte }
func NewMaterial([]byte) Material // copies input
func (Material) Bytes() []byte    // returns a copy to adapter/store only
func (Material) Empty() bool
// Material fmt formatting and JSON marshaling redact rather than exposing bytes.

type QR struct { text string }
func NewQR(string) QR
func (QR) Text() string // deliberate in-memory presentation-only access
func (QR) Empty() bool
// QR fmt formatting/JSON marshaling redact; never raw text in state receipts.

type Phase string
// Initial, Restoring, SignedOut, Generating, Waiting, Scanned, Saving,
// SaveFailed, Verifying, VerifyFailed, SignedIn, Expired, Failed,
// Canceling, SigningOut, DeleteFailed, Closing, Done.
// Serialized values are lower-case kebab-case; Done="closed".

type State struct {
    Phase Phase
    Error Code
    QR QR
    Generation uint64
    ExpiresAt time.Time
    Stored, Unsaved, Busy, CloseRequested, Restored bool
}

type PollStatus string
// PollWaiting("waiting"), PollScanned("scanned"),
// PollExpired("expired"), PollComplete("complete").
type PollResult struct { Status PollStatus; Material Material }
type Attempt interface {
    QR() QR
    Poll(context.Context) (PollResult, error)
    Close() // clears attempt-owned references; no remote sign-out
}
type Gateway interface {
    Begin(context.Context) (Attempt, error)
    Verify(context.Context, Material) error
}
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
// Business worker implements:
func New(gateway Gateway, store Store) *Manager
// *Manager implements Controller. Non-nil ports supplied by composition.
~~~

Store methods deliberately have no context: native Keychain calls cannot be
canceled once started. Manager owns one active operation worker and serializes
store calls. No mutex is held across gateway/store calls. UI polls safe Snapshot;
all mygo calls and QR bitmap creation/destruction happen on UI thread.

Action/lifetime contract:

- Constructor is side-effect-free, Initial. Composition calls Restore before
  Begin. Restore is allowed from Initial/Failed storage recovery and quiescent
  SignedOut; native missing becomes SignedOut, error Failed+Storage, loaded
  material proceeds to Verifying. No auto-delete/relogin on network/parse error.
- Begin allowed from quiescent SignedOut/Expired (and Failed after a failed Begin
  with no unsaved/stored unknown material). It starts a180s attempt, fixed2s serial
  polls, maximum90 polls,10s HTTP child deadlines. Only one attempt/worker exists.
- Cancel allowed for Generating/Waiting/Scanned and network verification only
  when no unsaved material can be lost. Invalidate generation/clear QR immediately,
  cancel context and wait worker before another Begin. Late completion never saves
  or publishes credentials. Saving/Restoring/SigningOut cannot be canceled; Busy
  is returned, and the UI explains the native operation rather than pretending
  context cancellation stopped it.
- PollComplete requires gateway-validated nonempty Material. Clear QR, atomically
  enter Saving before the store call. Native Save plus readback confirmation runs
  before nav validation to retain the one scan across transient network failure.
  Only then Stored=true, Unsaved=false, Verifying; Verify success SignedIn.
  Invalid/expired session never counts as SignedIn.
- Save failure: SaveFailed, Unsaved=true, keep material in memory, RetrySave
  repeats saving without Begin/Poll/new QR. Never plaintext fallback. Do not
  claim durable success until adapter confirms write+readback. Unknown write
  completion remains failure, not a fabricated rollback/credential deletion.
- Verify network/format/service failure: VerifyFailed, retain stored material,
  RetryVerify makes only nav; expiry becomes Expired. Neither deletes the store
  nor automatically asks for another scan. Restored=true only after successful
  restore verification; user-visible identity/text remains token-free.
- Logout allowed only when quiescent and stored (SignedIn/VerifyFailed/Expired)
  or DeleteFailed retry. It invalidates generation and clears live material/QR
  before serialized Delete. Success SignedOut with Stored=false; failure
  DeleteFailed, Stored=true meaning possibly still persisted, safe error visible.
  No new login/restore until successful retry or explicit close acknowledging
  retained storage. No late Set exists across Logout because Saving is Busy.
- RequestClose(false) is nonblocking. Clear/cancel pending HTTP and wait owned
  work to finish; never abandon a native store operation or lose unsaved material
  silently. While native Save is running keep window/event loop alive; successful
  save may finish close without nav, preserving the item for next restore.
  SaveFailed/Unsaved and DeleteFailed require explicit disposition: false refuses
  close with safe UI explanation. RequestClose(true) only when quiescent explicitly
  discards unsaved memory or acknowledges possibly retained stored login; it does
  NOT delete saved credentials. A close request during network validation keeps
  saved credentials. Done only after worker quiescence. Root also gates Quit.
- UI treats account expiry/cancel/retry/close as different actions. Ordinary close
  of SignedIn preserves Keychain. No actual user Logout during validation; only
  synthetic tests exercise destructive cleanup. OS force-termination is outside
  orderly-close guarantees and not simulated with the real logged-in process.

Errors crossing ports are fixed Code values; unknown errors sanitize to Network
or Storage at the owning boundary. Do not log wrapped transport/native errors.
No cookie/poll key or Material occurs in State; QR is the deliberate ephemeral
presentation exception. All Format/JSON redaction has sentinel tests.

Concrete adapter/view composition names (for parallel implementers):

~~~go
// internal/adapters/bilibili/account, import alias accountapi:
func New(transport http.RoundTripper) *Gateway // nil selects fixed production HTTP
func (*Gateway) Close() // close owned idle transport resources, no remote logout
// internal/adapters/credentials:
func New() *Store // fixed production service/key, no I/O until port call
func NewSynthetic(namespace string) (*Store, error) // exactly32 lowercase hex;
// service is fixed productionservice+".test."+namespace, never caller-selected
// real service. Same key web-session-v1; used solely by native smoke.
// internal/desktop/account, import alias accountview:
func New(controller account.Controller, requestClose func(discard bool)) *View
func (*View) Render(*ui.Context)
~~~

Normal composition uses these constructors, creates Manager on App.WhenReady,
then calls Restore. View exposes only normal actions; root owns window close/
quit. SIGTERM/interrupt also request orderly close through UI dispatch and never
call Logout. During store-critical operations root waits with responsive UI;
explicit forced OS termination cannot be given a fake completion guarantee.
E2E composition substitutes whole gateway/store ports, not live constructors.

Store.Save checks size/nonempty, calls native Set then Get and compares in memory.
Failure emits only fixed Storage error and retains user material in Manager;
no secret hash/value is returned. Native Get failure is fixed Storage except
actual ErrSecretNotFound -> Missing. Native values larger than2560 or empty are
invalid stored data and are not treated as missing or silently replaced.
Save/Get/Delete unit tests use an injected private backend; native smoke calls
NewSynthetic only. Smoke CLI requires --namespace and --phase=write|restore-delete;
invalid/absent flags make no storage call. Write phase asserts missing, saves a
fixed synthetic1 payload, checks it, replaces with fixed synthetic2, checks it.
Restore-delete phase in a second identical process checks synthetic2, deletes,
checks Missing and duplicate delete success. No supplied secret/value argument.

Normal launcher never imports automation. In e2e builds, only --synthetic is
accepted (and may select documented synthetic scenarios); production --begin-login
and --restore-check are rejected. No real store/gateway constructor reachable
from that composition. E2E hidden controls/status contain synthetic data only.

LOG-REV-001 response: Gateway only publishes Material after every retained
known cookie explicitly grants parent bilibili.com Domain (optional one leading
dot, ASCII case-insensitive) and exact root Path=/; no host-only/passport-only
or default/non-root path promotion. Version1 opaque encoding has mandatory
fixed scope="bilibili.com/". Verify rejects missing/unknown/extra scope metadata
before making nav requests. Store remains opaque and cannot widen scope.

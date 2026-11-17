# One real, resumable Bilibili client

Spec ID: 0027-p0-real-client
Status: revision 2 proposal answering architecture review 1, 2026-11-17.
Full lane. This is a proposed implementation contract, not approval or a claim
that the client works. The owner explicitly requested completion of P0.

The original planning references and proposal remain preserved. This bounded
revision answers P0R-ARCH-001 and P0R-ARCH-002 from architecture review 1 against
source14ce0ce6d374e5a655169348d5f7dfd76cf87efa. Before publication / freeze,
the controller records the completed 0025 and 0026 candidates and their
independent/native dispositions in a new assignment. Do not edit their frozen
inputs or count their evidence as evidence for this future combined candidate.

## User outcome, prerequisites and scope

The normal Mac application restores the existing login, searches for videos or
opens a supported identifier/link, shows details and parts, plays ordinary UGC
with the available supported quality, displays read-only Chinese danmaku, and
stores useful local progress. Continue watching after restart resolves fresh
media and resumes the same part. Failures are understandable and retryable;
obsolete work cannot replace a newer video or survive account revocation/close.

Required prerequisites are the completed real-login restoration, reviewed async
Source ownership from 0025, and actual Chinese glyph/timebase/fullscreen proof
from 0026. They remain dependencies, not implied by this proposal. The owner
has authorized progression beyond the fixture slice. M2 formal milestone
acceptance and P0 owner acceptance remain separate from this technical work.

Included: account publication/revocation, fixed read-only web APIs, WBI and
ordinary browser-cookie setup, search/pagination/BV/av/canonical link entry,
details/covers/parts, explicit HTTPS DASH resolution and quality replacement,
segmented ordinary danmaku, local JSON history/resume/delete/clear, feature
views, normal-app wiring, controlled offline/native verification and one bounded
real-service verification plan. Preserve the retained Keychain item and format.

Excluded: personalized feeds, posting, comment/danmaku sending, downloads,
cloud history, favorites/subscriptions, live rooms/PGC/paid/interactive content,
advanced video/audio formats, browser-profile import, challenge solving,
automatic cookie renewal, endpoint fallback chains, SDKs, a new database,
generic event bus/framework, new dependencies, Windows native claims, updater,
runtime redistribution/signing/notarization or public release. No CI/check-policy
change or remote publication belongs to this task.

## Boundaries and integration shape

Retain one module and the existing modular monolith. This refines, rather than
replaces, the original 0004 draft's composition-root orchestration:

| Owner | Responsibility and dependencies |
| --- | --- |
| internal/account | Existing login/store worker, live account epoch, publication and expiry acceptance; standard library only |
| internal/catalog | Plain identity/details/search/cover models, input parsing and bounded Browser; standard library and existing allowed plain packages only |
| internal/app | One video's Source, part/quality requests, transport intent, initial/resume seek; only catalog, playback and standard library |
| internal/playback | Existing native controller contract plus explicit HTTPS input policy; no account or HTTP client |
| internal/danmaku | Preserve 0026 timeline; add a bounded segment loader consuming a plain source and sampled playback state |
| internal/history | Device-local record/checkpoint behavior and Store port; catalog/playback/plain standard-library data only |
| adapters/bilibili/sessiondata | Single private cookie codec, preserving existing stored schema and scope validation |
| adapters/bilibili/transport | Fixed service routes, scoped private credentials, WBI/browser setup, cancellation and safe transport failures |
| adapters/bilibili/catalog, playback, danmaku | Feature DTO conversion into consuming contracts; private URL/ID mappings stay in adapters |
| adapters/localhistory | Exclusive filesystem ownership, versioned bounded JSON file, private permissions and atomic replacement |
| internal/desktop and child views | Native rendering/input/owned image and overlay references; no transport DTOs |
| cmd/my-bilibili | Navigation, account propagation, assembly, per-video lifetime and orderly whole-app close |

Do not move forbidden app/account dependencies into an unguarded helper.
There is no new generic client/coordinator package. Source stays single-video;
Browser, Loader and history.Manager own their feature behavior. Composition
only connects their explicit actions, state and lifetimes. No domain imports a
UI, native handle, HTTP adapter or experiments package. Existing app/catalog and
account import tests therefore remain unchanged and continue to run. The guard
currently checks multiple GOOS/CGO combinations; the older research's narrower
coverage description is not a request to modify it. Any actual need to change a
check or policy returns with a concrete diff for separate owner review.

All new UI-facing methods below are UI-thread-only unless explicitly stated.
Workers return immutable owned data; only Tick accepts it. State/Window methods
copy slices, perform no I/O and do not start playback. UI construction is pure.
Bounded worker cancellation/acknowledgment follows the established 0025 pattern;
do not introduce a generic worker framework merely to share that pattern.

## Identity, catalog and safe content failures

The existing VideoID/PartID stay opaque to consumers. Bilibili adapters return
canonical BVID and decimal CID. Part order is its slice order, never CID order.
AV entry canonicalizes through a matching detail response. Existing fixture
identifiers remain valid in fixture Sources.

Add the following catalog-owned plain types and ports (existing Video/Part
fields stay, with validation clarified below):

~~~go
type EntryKind uint8
const (BVEntry EntryKind = iota + 1; AVEntry)
type Entry struct { Kind EntryKind; ID string; Page int } // Page 0 = first
type PartRef struct { Video VideoID; Part PartID }
type Code string
const (
    None Code = ""
    Invalid Code = "invalid-input"
    Unsupported Code = "unsupported"
    Canceled Code = "canceled"
    Timeout Code = "timeout"
    Network Code = "network"
    InvalidResponse Code = "invalid-response"
    Unavailable Code = "unavailable"
    Restricted Code = "restricted"
    LoginRequired Code = "login-required"
    SessionExpired Code = "session-expired"
    VerificationRequired Code = "verification-required"
    RateLimited Code = "rate-limited"
    ServiceUnavailable Code = "service-unavailable"
    QualityUnavailable Code = "quality-unavailable"
    Closed Code = "closed"
)
func (c Code) Error() string
func ParseEntry(text string) (Entry, error)
func ValidateQuery(text string) (string, error)
func ValidateVideo(video Video) error
type Summary struct {
    ID VideoID
    Title, Creator, CoverID string
    Duration time.Duration
}
type Page struct { Number, TotalPages, TotalResults int; Videos []Summary }
type SearchSource interface {
    Search(context.Context, string, int) (Page, error)
}
type Cover struct { Width, Height int; RGBA []byte }
type CoverSource interface { Cover(context.Context, string) (Cover, error) }
type SearchPhase string
const (
    SearchInitial SearchPhase = "initial"
    SearchLoading SearchPhase = "loading"
    SearchReady SearchPhase = "ready"
    SearchFailed SearchPhase = "failed"
    SearchClosed SearchPhase = "closed"
)
type SearchState struct {
    Phase SearchPhase
    Query string
    Generation uint64
    Results []Summary
    Page, TotalPages, TotalResults int
    LoadingMore, AtLimit bool
    Error Code
}
func NewBrowser(source SearchSource) (*Browser, error)
func (b *Browser) Start(context.Context) error
func (b *Browser) Search(string) error
func (b *Browser) NextPage() error
func (b *Browser) Retry() error
func (b *Browser) Tick()
func (b *Browser) State() SearchState
func (b *Browser) Close()
func (b *Browser) Done() <-chan struct{}
~~~

Entry parsing is local, with no request on rejection. Bound input to 2048 raw
UTF-8 bytes before trimming outer Unicode whitespace; reject remaining control
characters. Accept BV followed by ten ASCII alphanumeric characters; lowercase
av plus 1..19 decimal digits representing a positive signed-int64 AID; or
HTTPS www.bilibili.com/video/<one such ID> with an optional trailing slash.
The canonical URL has no credentials, fragment, nondefault port, escaped path
pieces or extra path components. Parse its query strictly; one optional p is
digits, positive and <=1000. Discard other well-formed tracking parameters;
never forward them or look for an embedded ID in an arbitrary URL. Bare IDs
have Page=0. b23.tv and other URLs produce Unsupported, not an implicit search.
BV syntax is not existence/entitlement validation.

The search box and Open link/BV action are distinct explicit actions. Searches
are trimmed, nonempty, valid UTF-8, <=128 Unicode scalars and <=512 UTF-8 bytes,
with no control characters. No keystroke network requests or query telemetry.
Browser owns one active call, one latest request and one acknowledged result;
ten seconds starts at submission including queue wait. New query invalidates
prior query/page results and replaces page 1. Failed append preserves accepted
results and page number; Retry repeats precisely that failed query/page. Page
buttons cannot queue duplicates. Canonical VideoID dedupe preserves first-seen
order. Cap at 50 requested pages or 1000 unique retained summaries and show
AtLimit, rather than claiming the whole service result set. Empty page 1 is a
valid empty state. A malformed page or denied request is never empty success.

Validate bounded decoded strings after highlight removal/entity decoding:
title 1..512 UTF-8 bytes, creator 1..256, description 0..65536, part title
1..512. Empty descriptions are legitimate and no longer rejected by app's
old fixture-only check. CoverID may be empty (placeholder). All fields are
plain text without remaining controls; IDs are nonempty; parts are unique,
ordered, 1..1000, with positive duration <=7 days. Adapter detail validates
response identity against the requested BV or AID, and page numbers/order.
Move shared catalog invariants to ValidateVideo; do not make competing validators.
Unknown JSON fields may be ignored, but selected critical fields cannot be
missing/null, duplicate, incorrectly typed or contradictory. Bound nesting
and raw JSON before conversion; exact evidence/local-file schemas remain strict.

Search accepts only video rows. Normalize documented em highlight tags and
HTML entities into text, never render markup or follow arcurl/redirect_url.
Other literal title characters remain literal text. Response page must equal
the requested page; nonnegative finite reported totals are bounded to int32.
Empty first pages may report zero total pages. Do not assume a dynamic service
will return disjoint pages; the Browser deduplicates them.

Catalog.Code is a safe content-operation category, not raw server text. App's
DetailError, SelectionError and PlaybackRetryError become catalog.Code; native
State.Playback.Error remains playback.ErrorCode. App maps a controlled fixture
playback.Invalid/Unsupported/Canceled/Timeout/Media/IsClosed source error into
the corresponding content code (Media maps to Unavailable); unexpected errors
become Unavailable. No arbitrary Error() or service message reaches UI/evidence.

## Account publication and service scopes

Preserve service com.coda.mybilibili.account, key web-session-v1 and version-1
material containing the existing bilibili.com/ scope and four allowed cookie
names. Neither this feature nor a test rewrites/deletes the real retained item.
Share the existing strict codec in adapters/bilibili/sessiondata; do not copy
credential getters into feature views, Sources or history. Keep New(gateway,
store) behavior for the standalone login app and add these account ports:

~~~go
type SessionPublisher interface {
    Activate(epoch uint64, material Material) error
    Revoke(epoch uint64)
}
func NewWithPublisher(Gateway, Store, SessionPublisher) *Manager
// State gains SessionEpoch uint64: positive only for a currently usable login.
func (m *Manager) SessionExpired(expectedEpoch uint64) bool
func (m *Manager) DiscardUnsaved() error
func (m *Manager) Done() <-chan struct{}
~~~

Each successful Verify allocates a strictly increasing private account epoch,
publishes it, and only then exposes SignedIn/SessionEpoch. Failed activation
is VerifyFailed with a fixed safe error, not usable login. Manager lock may
call only these bounded memory-only publisher methods: lock order is Manager
then publisher. Publisher must never call Manager, UI, HTTP, Keychain or any
callback. No gateway/store operation occurs under the manager lock. This leaf
memory transaction prevents activation racing Logout or RequestClose without
adding asynchronous credential handoff workers.

Revoke synchronously retires the matching active epoch, clears the publisher's
private material/caches and cancels its request scope before Logout starts
Delete or close returns. Repeated/stale revocations are no-ops; an old epoch
cannot revoke a newer one. SessionExpired accepts only the current positive
authenticated epoch, revokes it, sets Expired, retains the Keychain item and
returns whether accepted. Network/403/malformed/challenge failures do not
expire a session. Done closes only after the existing account worker and
terminal RequestClose finish. Existing unsaved/delete-failure close decisions
remain; do not invent interruptibility of a native store syscall.

DiscardUnsaved is explicit, allowed only when not Busy and Unsaved, drops the
in-memory attempt without Store.Delete, and moves to Expired if Stored or
SignedOut otherwise. It cannot discard an in-flight save. Account pane dismissal
is distinct from whole-app RequestClose: hiding pending QR cancels that attempt
and drops QR bitmap references; saving/verification may finish in background.
The pane uses the same Manager for the application's lifetime. Fix existing
LOG-ADV-001 while touching its copy: say persistence could not be confirmed,
not that a failed readback proves no credential was saved. No new scan starts
merely because restore fails or the pane opens.

The shared concrete service adapter exposes these adapter-only contracts:

~~~go
// All credential and signing fields in these concrete types are private.
type SessionState struct {
    Revision, AccountEpoch, ExpiredEpoch uint64
    Authenticated, Closed bool
}
type Route uint8
const (Nav Route = iota + 1; Search; Detail; PlayURL; DanmakuView; DanmakuSegment)
func New(roundTripper http.RoundTripper) *Client
func (c *Client) Activate(uint64, account.Material) error
func (c *Client) Revoke(uint64)
func (c *Client) State() SessionState
func (c *Client) Bind() (*Scope, error)
func (c *Client) Close()
func (c *Client) Done() <-chan struct{}
func (s *Scope) State() SessionState
func (s *Scope) Context() context.Context
func (s *Scope) JSON(context.Context, Route, url.Values) ([]byte, error)
func (s *Scope) Protobuf(context.Context, Route, url.Values) ([]byte, error)
~~~

Client initially owns one anonymous scope. Every activation/revocation creates
a new positive Revision and cancels the previous scope. Bind returns the current
opaque scope, not credential bytes. Feature adapters bind once to that scope;
they do not read whichever account happens to be current later. AccountEpoch
is zero for anonymous scopes. On unequivocal authenticated nav -101 or
isLogin=false, the client atomically revokes only the matching scope and records
ExpiredEpoch; composition passes that token to Manager.SessionExpired. Stale
nav responses cannot expire a newer account. Anonymous nav -101 can still carry
WBI metadata and means signed out. Other endpoint -101 means LoginRequired
unless its independently reviewed contract establishes expiry; no extra nav
probe/retry is silently invented in response.

Scope cancellation is combined with the operation context for every request,
including preparation, queue wait and body read. Revocation prevents new
authenticated leases and cancels already submitted ones; it is not a claim
that already sent network bytes can be recalled. Recheck scope identity and
context before accepting data or issuing a dependent request. Context callbacks
have explicit stop/join ownership. Client.Close cancels all scopes and waits
for preparation workers via Done; feature owners join their own callers.

## Fixed transport, signing and bounds

All API routes use HTTPS api.bilibili.com, GET only, fixed path/query keys.
User input supplies validated values, never an endpoint. Reject API/cover/
bootstrap redirects. Disable environment proxies and native/browser imports;
use the reviewed fixed public Referer and an honest fixed application User-Agent.
No account cookie is sent to www bootstrap, covers, media or arbitrary hosts.
Codec Apply refuses a non-HTTPS/non-api.bilibili.com credential destination.

| Route | Fixed path | Preparation and bounds |
| --- | --- | --- |
| Nav | /x/web-interface/nav | Current scope cookie if authenticated; 1 MiB JSON |
| Search | /x/web-interface/wbi/search/type | WBI, normal browser cookie; search_type=video, keyword, page, order=totalrank; 1 MiB JSON |
| Detail | /x/web-interface/view | Exactly one bvid or aid; 1 MiB JSON |
| PlayURL | /x/player/wbi/playurl | WBI; bvid, validated cid, fnver=0, fnval=16, qn; 1 MiB JSON |
| DanmakuView | /x/v2/dm/web/view | type=1, oid; authenticated scope only; 1 MiB protobuf |
| DanmakuSegment | /x/v2/dm/web/seg.so | type=1, oid, segment_index; 4 MiB protobuf |

The unsigned web segment route is an intentional endpoint choice in this
proposal, matching the prior observed probe and pinned protocol reference. It
is not a fallback from the research's untested WBI proposal. No XML/app/alternate
endpoint is attempted when it fails. Optional pid is omitted; CID membership
is established by the validated video-bound Source/composition, not user input.

Every feature operation has ten seconds from submission. Child HTTP calls use
the earlier caller deadline, never extend it; raw HTTP also has a ten-second
limit. Client caps all API/bootstrap requests at four concurrent RoundTrips,
with cancelable semaphore wait. Limit decompressed bytes while reading, close
every response body, and return fixed codes for read/close/context failures.
Unknown HTML/nonprotocol responses are InvalidResponse/ServiceUnavailable;
only an observed challenge marker, nonempty v_voucher or -412 yields
VerificationRequired. HTTP429 is RateLimited, HTTP5xx ServiceUnavailable,
generic HTTP403 Restricted. No automatic retry loop, challenge workaround or
raw body/error/header in diagnostics.

Before a search needs browser cookies, issue one normal GET to
https://www.bilibili.com/ without account material. Retain only validated
buvid3 and optional b_nut in memory (at most two values, each <=1024 bytes),
respecting explicit bilibili.com/ scope, Secure and expiry. No manufactured
fingerprint or alternate acquisition route. Missing required buvid3 is an
explicit ServiceUnavailable preparation result, not a search pass.

WBI reads key names from nav's wbi_img URLs without fetching those images.
Validate the two 32-character hexadecimal basenames, derive the standard
documented 32-character mixin via its pinned permutation, and sign sorted
parameters plus Unix-second wts with MD5. Remove the documented !'()* characters
from values; encode UTF-8 and spaces as percent escapes (%20, not plus). Use
independent synthetic expected vectors for ASCII/Chinese/space/punctuation,
key order and timestamp; do not copy an SDK or examples containing live data.
Cache one keyset per scope for at most 15 minutes. This is a client reuse bound,
not a service promise. Revocation clears signing and browser state.

Preparation is one scope-owned worker, not a goroutine per waiter. It serves
one current attempt with merged needed flags (WBI/browser) and one completion
signal. Its ten-second deadline begins at preparation submission and is bounded
by scope lifetime. Each caller waits cancelably under its own deadline; canceling
one waiter does not incorrectly cancel another caller's preparation. Scope
close cancels/joins it. A failed preparation is returned to its waiting calls;
only a later explicit operation may create a new attempt. A single Get cannot
repeat failed preparation or retry an endpoint. Missing/malformed nav data is
a safe error, never a hard-coded signing key.

Sessiondata extraction preserves every current cookie-scope, duplicate, type,
expiry, length and unknown-field regression. The standalone account Gateway
continues its reviewed QR routes and limits. It may keep its own small HTTP
client; sharing the cookie codec does not require migrating a working QR flow
into the new content transport or silently changing its protocol.

## Detail covers and supported content

Catalog adapter constructors are pure and scope-bound:

~~~go
func New(scope *transport.Scope, coverTransport http.RoundTripper) *Client
func (c *Client) Search(context.Context, string, int) (catalog.Page, error)
func (c *Client) Detail(context.Context, catalog.Entry) (catalog.Video, error)
func (c *Client) Cover(context.Context, string) (catalog.Cover, error)
func (c *Client) Close()
~~~

The adapter privately maps opaque CoverID values to returned HTTPS cover URLs,
bounded to 1001 entries (1000 search results plus current detail). No URL enters
catalog.Video, history, UI state or evidence. Empty/bad covers use a placeholder
without losing otherwise valid details. Cover requests omit account/browser
cookies, reject redirects, allow only validated HTTPS locations and fixed public
metadata, read <=2 MiB, and inspect image dimensions before decoding. Accept
JPEG/PNG/GIF first image, <=1024x1024 source pixels; reject unsupported or larger
images without decoding them. Normalize into owned RGBA <=320x180 preserving
aspect ratio. One cover request/decode is in flight; at most 20 visible IDs are
pending. The desktop retains <=12 normalized bitmaps (<=2.64 MiB pixels), evicts
before adding, and drops outgoing/account/window references on retirement.
Decode buffers and mygo global caches are not falsely reported as that cache.

Only ordinary public/full-access UGC is supported. Preserve explicit restricted
or unsupported results: nonempty redirect_url (PGC/film redirection), positive
rights.movie/pay/ugc_pay/is_stein_gate, true is_chargeable_season,
is_upower_exclusive or is_upower_preview reject this content class. Nonzero
rights.ugc_pay_preview or rights.arc_pay has uncertain documented semantics and
is conservatively Unsupported, not a guessed entitlement rule. Required
rights fields used for ordinary-content classification must have known types;
unknown advanced/paid markers do not enable a bypass. Do not use VIP status,
URL shape or duration heuristics to claim a full-access grant. These supported
markers are from the pinned detail reference; new service shapes require a
bounded observed contract decision, not a speculative fallback.

## Quality resolution and generation-bound resume

Keep Session single-video. Extend its reviewed port deliberately, updating all
known normal/e2e fixtures, tests and strict observation consumers in the serial
shared-contract checkpoint before parallel workers start:

~~~go
type QualityID string // empty requests a default; nonempty is an opaque choice
type Quality struct { ID QualityID; Label string; Width, Height int }
type ResolveRequest struct { Part catalog.PartID; Quality QualityID }
type Source interface {
    Detail(context.Context) (catalog.Video, error)
    Resolve(context.Context, ResolveRequest) (Resolution, error)
}
type Resolution struct {
    Input playback.Input
    ExpiresAt time.Time
    Quality Quality
    Qualities []Quality
    RefreshOnRetry bool
}
type Options struct {
    InputPolicy playback.InputPolicy
    InitialPart catalog.PartID
    InitialPage int
    StartAt time.Duration
    Autoplay bool
}
func NewWithOptions(Source, *playback.Controller, Options) (*Session, error)
// New remains the zero-Options fixture convenience constructor.
// Player gains SelectQuality(QualityID) error.
// State gains SelectedQuality, PlaybackQuality QualityID; Qualities []Quality;
// Resuming bool; ResumeError catalog.Code; WatchSequence uint64.
// Its source error fields use catalog.Code.
~~~

Options permits either InitialPart or a nonnegative InitialPage, not both;
StartAt is nonnegative, bounded to 7 days, and remains private until detail/
native duration is known. Page=0 means first. The exact requested CID must exist
in fresh detail before resolving; missing/changed history part or out-of-range
page shows a selection error and opens nothing. It never silently resumes a
different part. An explicit part selection clears that initial resume target.

Default real Open/result/link starts paused like the existing preview; Play or
part selection is explicit. Continue/Replay from history requests Autoplay=true,
with StartAt=0 for completed replay. Each accepted resolution carries copied,
unique 1..16 choices and an actual chosen member. Fixture uses one fixed
fixture choice; no service claim. Source resolves a nonempty explicit request
exactly, or returns QualityUnavailable. State's choices/PlaybackQuality belong
to accepted playback; SelectedQuality describes the requested replacement.

The production Source is composed from a scope-bound catalog adapter and media
resolver. Detail establishes its member CIDs once on its serial worker; Resolve
rejects unknown CID or mismatched/retired scope before service access. Its
constructor does no network work. Adapter boundary:

~~~go
func NewSource(scope *transport.Scope, details *catalogapi.Client,
    entry catalog.Entry) (*Source, error)
~~~

Request standard DASH fnval=16, fnver=0, with qn=64 for default and the selected
code for explicit choices. Supported video quality IDs are 16,32,64,74,80,112,116
only when actually returned as AVC/H.264 (codecid=7 and avc1 codec) MP4 streams,
with positive dimensions <=1920x1080 and positive bounded bandwidth. Choose the
highest supported returned quality by default. Deduplicate same-quality AVC
entries by highest bandwidth, then first response index. Do not use only
accept_quality or the top-level quality to claim availability. Labels derive
from this fixed supported table and actual dimensions, never untrusted format
HTML. No 4K/HDR/AV1/HEVC/Dolby/flac request or support claim.

Require <=64 video and <=32 audio entries and bounded quality metadata. Choose
the highest-bandwidth returned AAC-LC mp4a.40.2 MP4 audio among IDs30216/30232/
30280. An explicitly null/empty audio list means silent video; a nonempty list
with no supported audio is Unsupported, not silently discarded. Required
fields cannot be absent/invalid. Recognize baseUrl/base_url and mimeType/
mime_type documented aliases; if both exist they must match. Select primary
URLs only; do not walk backup hosts after errors or upgrade HTTP URLs to HTTPS.
Validate the complete chosen video/audio pair before publication. A nonempty
v_voucher or an explicit returned preview/paid refusal is not playable success.

Add playback.HTTPSPolicy implementing InputPolicy, injected identically into
Session, Controller and mpv.Opener. Defaults remain fixture-only. It accepts
only HTTPS URL and optional HTTPS AudioURL, no FilePath, credentials, fragments,
opaque URLs, controls, invalid escapes or invalid ports; URL/header byte bounds
remain 4096. Any syntactically valid host/port, including IP literals, is allowed
under ADR0007; do not restore a CDN/host allowlist. It requires exactly the fixed
public Referer and User-Agent, never Cookie/Authorization. URL bytes are not
rewritten. This is validation, not a firewall or a claim about native DNS/
redirect behavior. Preserve all existing native config/script/log suppression.

Remote ExpiresAt is response completion +5 minutes: an explicit conservative
client reuse cutoff, not a promise of service validity until that time. Do not
infer an expiry rule from undocumented URL query fields or the reference's
generic two-hour example. A future recognized server cutoff needs its own
observed/documented semantics before shortening this client cutoff.
Keep the 30-second acceptance margin and recheck before Open. Remote sets
RefreshOnRetry=true: explicit native retry/replay resolves afresh even inside
the client cutoff. A rejected/expired result cannot fall back to an old URL.

SelectQuality is valid only for an accepted part and a returned choice. Disable
it while a different part is selected/resolving; changing a pending same-part
quality replaces its latest request. Capture the latest accepted media position
at resolution acceptance, not at network submission, and preserve newer play/
pause, volume and rate intent. Until replacement acceptance, old playback and
its overlay remain. Failure retains old engine/quality plus the safe requested
choice error and explicit retry. Keep all 0025 selected-versus-playing rules.

SelectQuality is unavailable while accepted playback is Ended. If accepted A
reaches Ended during its same-part quality resolution, retire only that quality
request, retain the actual ended engine/quality and reset SelectedQuality to the
accepted one. It does not replay, seek to near-end and auto-start, or reopen A.
An explicit Replay is a separate action. This special same-part quality rule
does not cancel a pending different part B or a deliberately submitted Replay;
retain 0025's terminal-A-versus-pending-B behavior.

WatchSequence is a per-Session positive accepted-watch token, zero before the
first accepted open. Increment it on accepted initial playback, accepted
explicit part selection/reselection or explicit replay from Ended. A pending
or failed selection does not advance it while the old part continues. Quality
replacement, expired-input refresh and retry of a failed current attempt retain
it. A successful current-part fast-path explicit reselection may advance it
without changing native generation. Plain pause/resume does not. This records
application intent without importing history into Session; composition maps
(navigation identity, WatchSequence) to its monotonic history Visit.

Open the replacement paused. Apply volume/rate on Ready, then issue one
generation-bound seek after valid native duration is available; clamp to
[0,max(0,duration-100ms)] for continue/quality replacement. Wait for a Ready/
Paused actual sample within 250ms of the requested target before Play; target
zero needs no seek. While Resuming, Play/Pause update pending intent and never
play from zero. Native readiness remains subject to its existing ten-second
limit; seek confirmation has five seconds from command submission. Failed or
timed-out resume stays paused with ResumeError and explicit retry/part actions,
not false success. Controls disable ordinary Seek during this initial seek.
Changing part/video/account/closing retires the pending seek; stale readiness
cannot seek/play another generation. A fresh explicit part selection starts at
zero; a quality replacement does not create a new history visit.

## Segmented danmaku without changing the proven renderer

Preserve the complete 0026 timeline/view/timebase/settings/lane contracts. In
particular MaxEntries=40000, MaxWindowBytes=8 MiB, MaxTextBytes=1024, MaxLanes=16;
do not adopt the research's alternative 100-visible/60000-entry proposal. Add
separate load contracts without putting HTTP types or DTOs into the model:

~~~go
type SegmentEntry struct { ID string; Entry Entry }
type Metadata struct { Known, Available bool }
type Segment struct {
    Index int
    Entries []SegmentEntry
    Encoded, SkippedEmpty, Unsupported int
}
type SegmentSource interface {
    Metadata(context.Context, catalog.PartRef) (Metadata, error)
    Segment(context.Context, catalog.PartRef, int) (Segment, error)
}
type LoadPhase string
const (LoadInitial LoadPhase="initial"; LoadLoading LoadPhase="loading";
    LoadReady LoadPhase="ready"; LoadFailed LoadPhase="failed"; LoadClosed LoadPhase="closed")
type LoadState struct {
    Phase LoadPhase
    Ref catalog.PartRef
    Generation, Revision uint64
    Segment, CachedSegments, Entries, TextBytes, ActiveRequests int
    Error, PrefetchError catalog.Code
}
type Window struct { Generation, Revision uint64; Entries []Entry }
func NewLoader(SegmentSource) (*Loader, error)
func (l *Loader) Start(context.Context) error
func (l *Loader) Bind(catalog.PartRef, time.Duration, uint64) error
func (l *Loader) Tick(playback.Snapshot)
func (l *Loader) Retry() error
func (l *Loader) State() LoadState
func (l *Loader) Window() Window
func (l *Loader) Close()
func (l *Loader) Done() <-chan struct{}
~~~

The Bilibili danmaku adapter has pure constructor
New(scope *transport.Scope, video catalog.Video) (*Source, error), which owns a
validated copied part/duration mapping. Its Metadata/Segment implementations
reject a Ref outside that video and apply the requested interval/duration
bounds before returning normalized data. No raw AID or sender data is needed.

Metadata adapter uses dm/web/view only when its scope is authenticated; anonymous
mode explicitly returns Known=false and proceeds to the supported unsigned
segment route, never claims an empty corpus. State0 permits segments, state1
means unavailable (both Known=true), other states are Unsupported. Do not use uncertain dmSge
pageSize/total or count as proof of zero data. Structurally invalid metadata is
InvalidResponse; authenticated metadata failure is a danmaku-only failed state
until explicit Retry. Ignore special-package URLs, commandDms, settings and
sender fields. No extra asset/URL from them is fetched or executed.

Segment index is floor(position/360s)+1, bounded by ceil(part duration/360s).
Decode absolute progress, not progress relative to the segment. Allow the
documented inclusive endpoint [start,end] (also <=part duration); assign exact
boundary entries by stable ID dedupe when two adjacent sets contain them.
Omitted progress defaults to zero and is valid only in the first interval.
Preserve structurally valid empty-text skip-after-validation; it counts toward
the 20000 encoded-entry limit. Body<=4 MiB; encoded entry<=4096 bytes; text<=1024
bytes, valid UTF-8; protobuf keys/varints/wire sizes/defaults are validated with
overflow checks. Malformed data remains InvalidResponse, not end-of-video.
Implement the narrow decoder independently under the product adapter, not by
importing experiments. Unknown fields are skipped only with valid wire bounds;
known repeated singular fields are rejected as ambiguous.

Modes1/2/3 normalize to ScrollRTL,4 bottom,5 top,6 reverse. Modes7/8/9 and other
positive unsupported values are counted/skipped after structural validation;
mode0 is invalid. No advanced command execution. Color is uint24; presentation
font size remains the user's 0026 setting. Ignore sender hashes/IDs and do not
retain or hash them. Identity uses consistent positive idStr/id if present;
fallback is a bounded hash of normalized timestamp/mode/color/text plus the
ordinal of an identical tuple in that segment. Duplicate real IDs with identical
presentation collapse; contradictory duplicate IDs are InvalidResponse.

Use exactly two cache slots, fitting the proven 40000-entry/8 MiB window. For
the first 16 seconds of a segment retain previous+current; afterward retain
current+next and prefetch next. At most one fetch/metadata call is active with
one latest pending request and one acknowledged result. Current work outranks
prefetch. Seek/part/scope changes cancel obsolete work, but retain its ownership
until return. Every attempt has a ten-second submission deadline. Failed slots
are latched until explicit Retry or a deliberate new seek/bind after eviction;
Tick cannot create a retry loop. Retry replaces a slot and cannot append another
copy. If prefetch fails, accepted current comments continue and PrefetchError is
separate; when needed as current, it becomes the visible explicit-retry error.

Cap each retained segment's owned text at 4 MiB and the pair at 8 MiB. Reject
overflow explicitly before publication. Window returns a copied entry slice in
stable (timestamp, identity) order, excluding duplicate IDs, only when queried;
composition fetches it when Revision changes, not on every paint. Immutable
strings may be shared until the proven timeline takes its own copies. Worker
decode/result memory is separately bounded by one 4 MiB response/result, not
falsely counted as zero or as part of the retained-cache cap.

Bind uses accepted PlaybackPart and native generation, never a pending selected
part. Same-part quality replacement updates presentation generation and reuses
valid segment cache; a new part/scope clears it. Cross-video Session native
generations can restart at one, so composition closes the old timeline/view and
creates a new one, carrying only user settings; it must not Reset a terminal old
timeline with a lower generation. On a new Window revision, replace entries in
the current view; errors become a safe danmaku-only message. Pause/rate/seek use
actual sampled position and the unchanged 0026 deterministic schedule.

## History, persistence and resume

History is device-local and survives sign-out. It stores no account ID, cookie,
query, cover/media URL or transport DTO. Add a plain history port/manager:

~~~go
type Code string
const (None Code=""; Invalid Code="invalid-history"; Storage Code="history-storage";
    InUse Code="history-in-use"; LockUnsupported Code="history-lock-unsupported";
    Corrupt Code="history-corrupt"; UnsupportedVersion Code="history-version";
    Uncertain Code="history-durability-uncertain"; Closed Code="history-closed")
func (c Code) Error() string
type Record struct {
    Ref catalog.PartRef
    Title, PartTitle string
    Position, Duration time.Duration
    UpdatedAt time.Time
    Completed bool
}
type Store interface {
    Acquire() error
    Load() ([]Record, error)
    Save([]Record) error
    Close() error
}
type Sample struct {
    Visit uint64
    Record Record
    Playback playback.State
}
type Phase string
const (Initial Phase="initial"; Loading Phase="loading"; Ready Phase="ready";
    Failed Phase="failed"; Closing Phase="closing"; Done Phase="closed")
type State struct {
    Phase Phase
    Records []Record
    Revision, SavedRevision uint64
    Busy, Dirty bool
    Error Code
}
func New(Store) (*Manager, error)
func (m *Manager) Start() error
func (m *Manager) Observe(Sample) error
func (m *Manager) Delete(catalog.PartRef) error
func (m *Manager) Clear() error
func (m *Manager) ResetCorrupt() error
func (m *Manager) Retry() error
func (m *Manager) Tick()
func (m *Manager) State() State
func (m *Manager) RequestClose(discardUnsaved bool) bool
func (m *Manager) Done() <-chan struct{}
~~~

Store is an exclusive lifetime, not a reusable stateless file helper. Successful
history.New transfers that Store to one Manager; callers must not reuse it in
another Manager or call it concurrently. A failed New does not transfer it.
Constructors acquire nothing and read no files. The one owned I/O worker first
calls Acquire, then Load only after successful acquisition, serializes every
Save, and finally calls Close after the last store operation. No caller reads
history before this ownership step. Acquire is nonblocking with respect to a
competing owner. It returns InUse when that location is held, Storage for other
acquisition errors, or LockUnsupported on an unsupported lock platform. It must
not return success on a lock error. An unsuccessful Acquire leaves no owned
descriptor and permits explicit retry; Load/Save without acquisition fail
without touching the history file. A successful repeated Acquire on the same
still-owned Store is idempotent and never obtains a second descriptor.

Start submits the Acquire/Load operation once, with one latest pending snapshot
and one acknowledged result. Until a valid Load is accepted, Observe/Delete/
Clear are rejected, no mutable record snapshot is created, and no deferred save
can later overwrite the just-loaded file. A losing Manager exposes Failed/InUse
and fixed wording that another application is using history, with Retry/Close;
this is not an empty-history success. Search/playback may remain available with
that visible unsaved-history state. Explicit Retry repeats acquisition then
loads the latest bytes, never resumes an earlier unowned snapshot. A load error
after successful acquisition keeps the lock while the user chooses Retry or
corruption recovery; ResetCorrupt cannot run after an acquisition failure.

No goroutine or disk write per Tick. Normal file syscalls are not claimed
cancelable. State mutations and revision acceptance are UI-thread-only; worker
results are immutable. Failed save keeps Dirty/Error visible; a stale save
completion cannot claim a later revision saved. A successful current save
clears only its matching uncertainty. The local-file adapter enforces ownership
before reading, creating a temporary, replacing, or resetting data; Manager
serialization alone is not the cross-process exclusion mechanism.

Composition allocates a positive monotonic Visit when an accepted
(navigation identity, WatchSequence) changes, independent of native generation.
Open/Continue/Replay/part-selection therefore start a visit only when accepted;
pending B cannot recreate deleted accepted A. Quality replacement and automatic
refresh retain that Visit. Record only accepted sampled playback
with positive native duration, after progress or Playing/Ended; no zero-progress
detail-only visits. Clamp observed position to [0,duration]. Completed is true
only after native Ended, not a guessed near-end threshold. Canonical (BVID,CID)
is the record key, so AV/BV aliases converge after detail. Store titles as plain
UTF-8 truncated at rune boundaries to 256 bytes each.

Checkpoints coalesce every five seconds of changed Playing progress (a monotonic
clock seam), on a changed paused sample, part/visit transition, Ended and orderly
close. A playing seek is captured by the next <=5-second checkpoint; a paused
seek is immediate after its actual changed sample. Never persist an unconfirmed
seek command target. This explicitly refines the research's optional per-seek
checkpoint idea without requiring a second playback event consumer.

Delete/Clear mutates the owned record set and queues a newer serialized snapshot.
If the currently observed visit was deleted/cleared, suppress recreation for
that same Visit, including later quality generations. A fresh deliberate play
action can create a new record. Late/in-flight saves run before the deletion
snapshot; no stale completion can undo it or clear its Dirty flag. Keep at most
500 most-recently-updated records, evicting oldest only after a valid load.

The local adapter constructor is localhistory.New(directory string)
(*Store, error), where Store implements history.Store. It validates syntax only;
empty directory selects os.UserConfigDir()/my-bilibili during Acquire, and a
nonempty injected directory must be absolute. No directory or file access takes
place on the UI thread. The data basename is always history.json. Tests inject
only a temporary private directory.

Acquire creates/checks the private directory, makes its path absolute/clean and
resolves existing directory symlinks with filepath.EvalSymlinks before selecting
the data and lock paths. Failure to establish this canonical directory fails
closed. Aliases of the same directory therefore use the same lock location.
Open the separate stable history.lock file read/write/create without truncate,
with0600 permissions and no-follow semantics; reject symlink/nonregular lock
files. On darwin and linux use the observed standard-library syscall.Flock
with LOCK_EX|LOCK_NB on the retained descriptor. EWOULDBLOCK/EAGAIN means InUse;
all other failures preserve history bytes and return Storage. No dependency,
PID-age heuristic, lock-directory deletion or mygo SingleInstance fallback.
Other GOOS builds provide the same compiling port but Acquire returns
LockUnsupported before any history read/write; Windows native remains P2
not-tested, not silently protected by a process-local mutex.

The stable lock inode is never renamed, replaced or unlinked, including on
normal close, failed acquisition and crash recovery. Do not lock history.json:
atomic replacement deliberately changes that file's inode. Hold the exclusive
descriptor across Load, corruption/retry decisions, every checkpoint/delete/
clear/reset and final save. Close runs only after no queued/in-flight operation
can write again; it releases the advisory lock and closes its descriptor, with
no intervening read/reacquire/save. Kernel process exit releases the OS lock;
the harmless lock file remains for the next process. Directory permissions and
cooperating application instances are the boundary; this is not a distributed
lock or protection against another same-user program deliberately replacing
files behind the application.

Version1 JSON has only version and records.
Each record has exactly video_id, part_id, title, part_title, position_ms,
duration_ms, updated_at and completed; position/duration are integer
milliseconds, updated_at is UTC RFC3339Nano with Z. IDs are bounded nonempty
plain identifiers established by catalog conversion; e2e uses original
synthetic canonical-looking IDs. Bound file to 1 MiB before
decode, reject unsupported version, duplicate/unknown/null keys, invalid IDs,
timestamps/durations and duplicate record keys. Missing file is empty. Corrupt/
unreadable/oversized/version-unknown input leaves the original bytes untouched
and disables automatic saving; explicit ResetCorrupt after confirmation may
replace it with empty valid history. Plain Retry retries a read or a failed save,
never resets a corrupt file. Do not silently backfill defaults or overwrite it.

Use private directory0700/file0600 and a bounded temporary sibling. Write fully,
sync, close, atomically rename and sync the directory where supported. Clean up
owned pre-rename temporaries. Distinguish failures before replacement from
post-rename sync uncertainty; never claim rollback after replacement. Reject
symlinks/nonregular target files rather than following them. No history in the
checkout, external synchronized folder or account cloud service.

Close freezes observation mutations and submits the latest checkpoint. It waits
off the UI thread, retaining the file lock through final I/O. If save fails,
the window stays open
with Retry or an explicit close-without-confirmed-save action; RequestClose(true)
is available only when no store call is Busy, then discards pending unsaved
snapshots and still calls Store.Close without rewriting an older file. Close
before Start or after failed Acquire also retires the Store without acquiring
or loading it. After Close begins, Acquire/Load/Save return Closed; the Store
can never be reused to publish a stale snapshot. Close is idempotent after
success. A cleanup error is a safe Storage result, not success; Close retries
only unfinished release/descriptor cleanup and never reopens the data or lock.

Manager accepts a close completion only after successful Store.Close and worker
retirement. Then Done closes. A release/close failure keeps Done open, exposes
Failed/Storage, and Retry performs only that unfinished cleanup on the owned
worker. The explicit unsaved-data discard action cannot waive a cleanup error
or manufacture zero ownership. Acquisition/load failures and all close paths
must have this terminal cleanup, including minimized shutdown; composition pumps
the owned result acceptance through Window.Update, without waiting for paint.
Successful close never silently ignores Dirty data. This is ordinary product
UX, not a new task approval requirement.

Continue reads a copied record, opens fresh detail with exact InitialPart and
StartAt, and uses the generation-bound ready/seek protocol above. Completed
records offer explicit Replay at zero. Missing/changed part, restricted content
or service failure remains visible; no different-part/time substitution.

## Native views and whole-app lifecycle

Default cmd/my-bilibili becomes the real shell. Keep an explicit --fixture
normal mode for the existing developer preview; e2e defaults to controlled
fixture composition so full/source/danmaku stages do not touch an account or
network. Normal builds expose no automation endpoint. e2e has a separate fixture-
only constructor and cannot select the real account/content composition. The
normal executable's explicit bounded verification option below uses the same
final binary and normal wiring, without an RPC socket or arbitrary input driver.

The shell has Search/Open, History, account status and the current player/detail
area. At the existing 800x600 minimum, controls wrap/stack vertically; preserve
native keyboard focus and the current fullscreen/player gestures. Existing
mygo TextInput.Submitted and Button actions submit bounded commands; no global
shortcut steals typing. Feature views own no network or disk operation. UI
uses fixed safe Chinese wording for failure categories, with explicit Retry;
empty search, loading, denied, unavailable and signed-out are distinguishable.

Add a quality popover beside existing parts/speed/danmaku controls. Popovers are
mutually exclusive, hold controls visible and dismiss before fullscreen Escape.
Selected pending quality and actual accepted quality remain distinguishable.
The account pane embeds the existing account view with a dismissal callback;
no duplicate login implementation. The view drops QR references when hidden,
and whole-app close alone calls Manager.RequestClose. No QR/account screenshot
or private accessibility dump belongs to verification output.

On a new video, checkpoint old accepted history, retire old app/Loader/cover
view, close native playback, cancel the old scope-bound video context and join
their Done signals off UI. Retain one latest pending navigation request; only
after old video ownership ends create its replacement. This deliberately does
not promise A continues while a different video B loads; within a video's
part/quality resolution the 0025 preservation rules still apply. Search and
history views remain usable during this handoff. No per-click goroutine growth.

On a service Revision change, immediately cancel/revoke old account-bound
search/video/danmaku/cover work and close authenticated native playback on the
UI thread. Retire queued navigation from that revision. Create a fresh Browser/
source scope only after the old relevant owners stop. History records survive.
Do not silently re-open the old video anonymously or auto-submit a failed query;
the next explicit action uses the new scope. The small account state observer
dispatches through Window.Update, including minimized windows, as in the
existing login composition. Do not rely on repaint for close or revocation.

One app close coordinator stops accepting actions, requests account/history
close, closes Source/Loader/Browser/cover owners and native resources, cancels
transport setup, and waits off UI for all Done signals. It then dispatches one
Window.Close via Window.Update. Failed unsaved account/history decisions keep
the window open and visible, with fixed retry/discard choices. It never joins
network/file work on the UI thread or reports zero ownership for a live worker.
Retain existing fixture 200ms native cancellation and eight-second live
post-disposal survival and minimized pending-source close checks unchanged.

## Native picture observation

P0R-AC-05 already requires an actual native picture separately from selected
tracks/time. Freeze these small adapter-owned UI-thread ports; P0R-010 owns
their implementation and consumer together:

~~~go
func DrawTargetSized(c *gui.Context, plain playback.Engine, displayWidth, displayHeight float32)
func PicturePresented(plain playback.Engine) bool
~~~

DrawTargetSized uses the existing double buffers, framesMu.TryLock, bitmap
creation/update, Contain sizing and native teardown unchanged. It labels the
real target correctly, while existing Draw/DrawSized fixture labels and
DrawTarget fixed-size compatibility remain. It does not allocate another video
buffer, add a render worker, capture pixels or make a network call. Reject
nonfinite/nonpositive presentation sizes without creating a picture observation.

PicturePresented starts false for every engine. It becomes true only after a
successful render-produced pending frame has been consumed into that engine's
bitmap and that bitmap's Image has reached the actual mygo paint pass at finite
positive dimensions. Use the public Image.DrawOver callback, which the inspected
mygo paint path invokes after the image drawing command, to record only this
idempotent boolean. Constructing a node, setting an accessibility label, creating
a bitmap or observing tracks/progress is insufficient. The callback captures
the prepared bitmap identity and checks the same engine is not closed and still
owns that bitmap; it never schedules, allocates, changes playback or touches
native rendering. Retained old callbacks after Close cannot set the bit.

Read PicturePresented only on the UI thread for the currently accepted engine
and navigation/native generation. It returns false for nil, a non-mpv engine
or a closed engine; Close clears it. A new accepted engine starts false even
when native generation numbering restarts in a new Session. Normal live play/
resume gates require this result while the target player is mounted in a
nonminimized visible window, plus their existing readiness/tracks/advancing-time
requirements. This proves native bitmap submission in the application's paint
path, not compositor completion, calibrated physical pixels or audibility.
The existing synthetic native fixture pixel gates remain unchanged.

Add synthetic gui.Tester raster checks and a no-picture case with valid selected
tracks/advancing time but no successful pending render or paint. Also cover
prepared-but-not-painted, nil/closed engine and a retained callback after Close.
A compiling control that substitutes track/time readiness for this predicate
must fail the no-picture gate. Only the boolean enters observations/receipts;
never export bitmap pointers, frame bytes, hashes of live pixels or screenshots.

## Acceptance criteria

### P0R-AC-01

The shared ports, exact baseline and protected storage encoding are frozen after
independent architecture review; required 0025/0026 evidence is recorded. Domain
imports and default fixture-only playback validation remain intact.

### P0R-AC-02

Restored login is published only after verification. Revoke/expiry/close retire
the correct scope and cancel account-bound work; late A cannot expire/reopen B.
Existing QR/save/readback/cancel/retry behavior and Keychain bytes remain usable.

### P0R-AC-03

Fixed transport/WBI/browser preparation has bounded work, deadlines, safe errors
and destination-specific credentials. No Cookie reaches covers/media/bootstrap
or redirects; unknown/denied/expired data cannot become success or a retry loop.

### P0R-AC-04

Search, paging and supported direct entry reach matching canonical details and
parts with cover/placeholder, correct empty/error states, bounded retention and
latest-only result acceptance. Invalid input makes no request.

### P0R-AC-05

Permitted ordinary UGC resolves actual supported DASH choices and correct A/V
inputs through all three explicit HTTPS validators. Quality changes and fresh
retry preserve identity/intent/position; unavailable qualities are truthful.

### P0R-AC-06

Initial history resume and quality replacement wait for the intended generation's
readiness/seek confirmation before play. Missing part, seek timeout, rapid
switching/account revocation and stale native/source results have defined safe
outcomes without zero-position autoplay or an obsolete seek.

### P0R-AC-07

Real segmented danmaku is normalized into the proven native renderer, keeps
absolute media time/identity, bounded two-slot work and explicit retry, and
cannot leak previous part/account text or duplicate retry data. Failure leaves
video playback usable and does not execute special content.

### P0R-AC-08

History persists useful canonical progress, restores after restart, resumes the
same part, and supports delete/clear without late-save resurrection. Corrupt/
unwritable/uncertain storage remains visible and preserves prior data until an
explicit recovery decision. Exactly one Store owns each canonical location
from before Load through final cleanup; a competing instance fails visibly and
cannot read/save/reset stale data. Ownership releases after orderly close.

### P0R-AC-09

One usable macOS shell connects account, discovery, playback, overlays and history,
including keyboard/fullscreen/min-size behavior, resource ownership and
paint-independent account revocation/close. Synthetic destructive tests do not
read or delete the real account or history.

### P0R-AC-10

The exact combined candidate passes repository/race/tagged checks, required
synthetic native stages and a bounded actual restored-account service/playback/
danmaku/history-restart loop, then independent code/native/privacy review.
Per-capability pass/fail/not-tested evidence, remaining limits and owner actions
are recorded without claiming Windows, remote CI or a public release.

## Verification and delivery evidence

| AC | Required discriminating checks | Native / evidence |
| --- | --- | --- |
| 01 | Existing import/fixture-policy gates; exact types/defaults; unchanged cookie encoding roundtrip and old login tests | Final base/spec/task hashes in controller run; no policy diff |
| 02 | Activate-before-SignedIn, publisher failure, scope revocation, stale expiry, close/activation and sign-out/success races; no IO under Manager lock; fake Delete failures | Synthetic account UI and minimized close; real restore only, no real logout/delete |
| 03 | Destination/header capture; revoked contexts; redirect/oversize/body-close failures; WBI expected vectors and TTL; preparation waiter cancellation/close/bounds; missing buvid3; denial classification | Sanitized API stage/count/status receipt; no raw request, body or cookie |
| 04 | Entry/query vectors, requested/returned ID/CID mismatch, empty description, duplicate pages, late query/page, failed append, cover bounds/decoding/copy/eviction | Synthetic native search/Open/link/cover/empty/retry at minimum size; actual one search/detail path |
| 05 | Actual vs advertised quality, aliases/codec/audio rules, explicit choice unavailable, HTTPS default rejection and opt-in at Session/Controller/Opener, expiry/deadline/fresh retry; pending-frame/paint observation and tracks-plus-progress-without-picture rejection | Synthetic multi-quality A/V and original pixel gates; actual current-engine picture_presented plus accepted target streams/quality and selected-track readiness, no live capture |
| 06 | Latest position/intent during pending quality; seek exactly once after Ready, timeout stays paused, missing CID, late ready after B/sign-out; native generation reuse across videos | Synthetic native quality/resume with known positions; real restart Continue loop |
| 07 | Second-segment >360000ms, boundary dedupe, protobuf/default/empty/unsupported/cap cases; canceled fetch and retry replacement, two-slot/prefetch/seek/quality behavior | Unchanged 0026 Chinese pixel/timebase stage; synthetic segment transition; actual service segment accepted and visible current-frame count, no live capture |
| 08 | Real temporary-file restart, missing/corrupt/large/version/permissions, write/sync/rename/dir-sync failures, stale save/delete/clear suppression and later explicit new visit; independent owners/helper process, canonical-directory aliases, failed Acquire/Save/Reset, close/error/release and lock-removal control | Synthetic native restart/Continue/delete/clear, busy-store Retry and minimized close with isolated file; actual normal history reload without deleting existing user records |
| 09 | Cross-feature controlled callbacks, rapid navigation/epoch change, close while each worker pending, focus and popover tests, normal/e2e isolation | Rebuilt macOS full/source/danmaku plus client stage; minimized closure and zero owned resources |
| 10 | Full check --race; normal/e2e and verification-option offline tests; independently reviewed exact commit/artifacts/evidence | Scoped normal/e2e developer bundles, synthetic client stages, bounded real loop with the identical normal executable; Windows P2 not-tested |

New guards/limits require a regression that fails when the mechanism is locally
reverted. Consolidate a compiling negative-control matrix for epoch acceptance,
credential destination, WBI encoding, queue/deadline bounds, exact quality,
initial seek gating, segment identity/window caps, history deletion/write
ordering and cross-process acquisition, and picture observation separate from
tracks/time. For file ownership use two independent Stores/Managers and a helper
process against one temporary canonical directory, not just concurrent calls to
one Manager. After A's save/delete/clear, B's failed acquisition and attempted
stale snapshot cannot change the bytes; after A closes, B explicitly acquires
and loads those latest bytes before any save. Include alias paths, stable lock
inode across data replacements, release after final save and a compiling guard-
removal control that admits B and breaks that exclusion assertion. Tests inject
acquisition and cleanup errors too; no error path publishes clean-close success.
Do not consume rounds on wording or request unrelated test expansion.
Preserve failing evidence; repeat a full check only after material changes or
an unresolved failure requires it.

Required synthetic native work uses the existing mygo application-owned e2e
harness, not global desktop keystrokes or a live account. Keep old strict fixture
observations and checks. Add a separate strict ClientObservation/key for client
scenarios rather than weakening fixture ID/track proofs. It contains fixed
phase/error enums, token-free generation IDs, bounded numeric request/cache/
resource/history counters, confirmed quality/position and boolean feature
readiness only; no query/title/comment/cover/URL/header/credential values.
Use the following complete plain shape, not the unrestricted product State:

~~~go
type ClientObservation struct {
    AccountPhase account.Phase
    AccountError account.Code
    AccountEpoch, ScopeRevision, Navigation, Visit uint64
    Authenticated, Stored, Restored bool
    SearchPhase catalog.SearchPhase
    SearchError catalog.Code
    SearchGeneration uint64
    Results, Page int
    LoadingMore, AtLimit bool
    DetailPhase app.DetailPhase
    DetailError, SelectionError, PlaybackRetryError, ResumeError catalog.Code
    Parts, SelectedPart, PlaybackPart int // one-based index; zero = none
    Qualities, SelectedQuality, PlaybackQuality int // same indexing rule
    Resolving, Resuming, DesiredPlaying bool
    Playback playback.Snapshot
    VideoTrack, AudioTrack bool
    PicturePresented bool
    SourceCalls, SourceActive, SourcePeak int
    SearchCalls, SearchActive, SearchPeak int
    APIRequests, APIActive, APIPeak int
    CoverRequests, CoverActive, CoverRetained int
    DanmakuPhase danmaku.LoadPhase
    DanmakuError catalog.Code
    DanmakuGeneration, DanmakuRevision uint64
    Segment, CachedSegments, CachedEntries, CachedTextBytes int
    DanmakuActiveRequests, VisibleComments, TextLayouts int
    HistoryPhase history.Phase
    HistoryError history.Code
    HistoryCount int
    HistoryRevision, HistorySavedRevision uint64
    HistoryBusy, HistoryDirty bool
    PendingOwners int // owned Done channels not closed after close request
    Resources mpv.Resources
    Minimized, Closing, Closed bool
}
~~~

Require every exact field (no missing/null/duplicates/case aliases/unknown keys),
all fixed enum values and finite bounded positions/settings. Validate counts
against the spec limits, nonnegative active/cumulative counts with peak>=active,
one-based indexes against present item counts and no accepted native part with
generation0. While not closing PendingOwners=0 is only a not-yet-joining marker,
not cleanup evidence. On Closed require every actual joined owner done, zero
native resources/tracks/picture observation/active operations/overlay layouts/
cache references and
no resolving/resuming/desired-play state. Never coerce unknown/positive ownership
to zero. Bounds are assertions too: a value beyond a resource cap remains a
failure, not a truncated valid observation. Native readiness still requires
actual nonzero ownership and expected selected tracks, not an Idle flag; the
separate picture gate additionally requires PicturePresented for that engine.

Stable UI keys are client-search-text, client-search-submit, client-open-text,
client-open-submit, client-next-page, client-retry-search, client-result-N
(zero-based visible result), client-history, client-account, history-continue-N,
history-replay-N, history-delete-N, history-clear, history-reset, quality-menu
and quality-N. Confirmations use explicit confirm/cancel keys, scoped by their
own popover. Existing player/danmaku keys remain. Synthetic text entry uses the
application-owned mygo e2e text-input action on those nodes, not OS keystrokes.

The existing driver gains -stage client with a 240-second per-stage deadline;
normal/e2e build identity, zero/forced cleanup distinction and fresh output
requirements stay. Synthetic client scenes include search+append/retry, direct
multi-part entry, quality/resume, segment seek, restart with the same isolated
history, fake account expiry/sign-out, corrupt/in-use history recovery and minimized
close while HTTP/store work is held. Synthetic media/comments are original.
The client e2e composition uses fake RoundTrippers and synthetic account stores
with the actual catalog/transport/segment conversion code. For native playback,
an e2e-only Source wrapper may first run the actual resolver on its synthetic
envelope, then replace only that scenario's known video/audio inputs with the
existing hash-verified fixture files. No arbitrary path/media URL is accepted,
and the app/Controller/mpv use their fixture-only policy in this composition.
This verifies replacement/seek/intent on native media, not a real resolution or
an encoded-picture-quality claim. Actual returned stream selection and HTTPS
mapping have their adapter tests and the required real verification separately.

For real verification, reuse the final exact normal executable for both launches,
as the existing login --restore-check verification does. Its optional
--verify-readonly <absolute-plan-path> mode submits only the reviewed bounded
read-only actions through the same UI-facing contracts and emits a strict safe
receipt. There is no e2e/RPC endpoint, arbitrary script, endpoint, cookie/header
argument or runtime control channel. The mode cannot be combined with --fixture
and e2e rejects it. A single fixed verification controller has ordinary owned
lifetime and starts only with that explicit invocation option; default launches
remain the normal user application. Tests inject fake ports and prove the mode
cannot Begin QR, Save or Delete account material; actual verification uses a
restore-only Store wrapper and disables those account actions. No automatic QR.

Before invoking it, the controller freezes a small public-input plan: one query,
one selected returned or canonical ordinary video, optional second part/quality
if actually available, and at most two process launches for restart. No live
query/video identity is printed into its receipt. Per whole plan, cap API calls
at24, homepage bootstrap at2, covers at4, native media opens at4, wall time at
240 seconds per process, and stop on first unexpected denial/shape/ownership
failure. Media's bounded duration/read behavior remains the native player;
this request cap does not claim to count its HTTP range requests. Do not dump
HTTP/native stderr, live window screenshots or private danmaku. Actual receipt
proves restoration, search/detail acceptance, current-engine picture presentation,
selected video/audio and advancing position, supported available quality,
segment parse/window acceptance, saved
progress, clean close and matching-part resume on the second launch. It records
counts/booleans/positions, not identifiers or content. The local plan's exact
version1 fields are version, run_id, query, video_id, selected_result, part_page,
quality_index and launch (1 or2). run_id is exactly32 lowercase hexadecimal
characters, a nonsecret owned-run identity. selected_result is one-based, or0
when a nonempty canonical supported BV video_id is used; those two selectors
are mutually exclusive. part_page/quality_index are zero for first/default or
one-based existing choices. Indices are bounded by already accepted lists. No unknown/null/
duplicate fields or raw locations are accepted. The controller records its hash
locally and passes a file path, not credential environment/arguments. On launch2
the verified private history record supplies canonical video/part/progress;
the plan cannot substitute a different identifier or time. The first launch
writes one owned <=4KiB private continuation file under the app's verification/
run_id directory, containing only version, executable hash, canonical Ref,
saved position, history-file hash and cumulative request/open counts. It contains
no titles, queries, credentials or URLs and never becomes a report attachment.
Second launch verifies the identical executable and history bytes, resolves the
stored exact Ref, and uses the remaining whole-plan budgets. The verification-
only Store wrapper computes the required file digest immediately after a
successful delegated Load/Save on that same owned worker, while the filesystem
lock remains held. On launch2 it checks the digest before publishing the loaded
records; on launch1 the continuation uses the last successfully saved snapshot
and matching digest after final save/close succeeds. It cannot inspect history
before Acquire, hash a moving unlocked file as the saved checkpoint, bypass an
InUse failure or use a wrapper to Save after Close. Hash/read failure fails
verification without replacing data. No new public history observation port or
persisted schema field is needed. It does not choose
an arbitrary latest history row. Delete only that owned continuation file on
successful completion; preserve a failed one privately for diagnosis. Refuse
reuse of an already completed run_id (the fresh run directory remains as the
nonsecret completion marker). This is a fixed two-launch verification
receipt, not a general automation/persistence layer or a history schema change.

Live receipt is exactly version, event, stage, result, error, account_restored,
stored, search_count, parts, qualities, video_track, audio_track, picture_presented,
native_generation, position_ms, duration_ms, danmaku_loaded, visible_comments,
history_saved, history_restored, same_part, resumed, api_requests,
bootstrap_requests, cover_requests, media_opens, pending_owners, native_cores,
native_renderers, native_workers and forced_cleanup. version=1; event is stage
or closed; stage is restore/search/detail/play/danmaku/history/resume/close;
result is pass/fail/not-tested; error is a fixed content/account/history/native
safe code from the reviewed sets. Boolean fields are booleans; counts and
durations are nonnegative bounded integers. No optional free-text field exists.
For a passing play/resume stage picture_presented must be true for the current
accepted engine; tracks/position alone cannot satisfy that stage. It is false
after engine disposal, and does not retain a frame or certify physical display.
Parse/collect this exact schema with a bounded receipt stream; incomplete or
malformed output cannot become pass. Fresh evidence directories retain strict
receipts, hashes and aggregate results only, never raw native stdout/stderr.

Part/quality changes are required with synthetic native fixtures; exercise them
on the real target if it offers multiple supported choices. If it does not,
report those real-target capabilities not-tested without inventing another
entitlement or weakening the synthetic feature checks. At least one real video
must play with accepted ordinary danmaku and resume after restart. A denied
bootstrap or missing required real-loop evidence blocks that acceptance; do
not call a fixture-only integration complete. Any recovery investigation has
its observed failure and remaining budget recorded before another attempt.

The rebuilt final normal bundle may cause a native Keychain permission prompt for a changed
binary. Keep actual credentials in the app/Keychain; if owner interaction is
needed, present that concrete built app through Cindy's private handoff and
continue independent work. Do not read secrets through shell tools or solve
system prompts by an alternate input path. No new permission question is needed
for the already-authorized implementation or ordinary read-only verification.

## Review, risks, rollback and continuation

Independent architecture review assesses account/transport lock ordering and
scope epochs, exact error/capability classification, async worker ownership,
quality/seek lifetime, two-slot renderer compatibility, exclusive file ownership,
durability and the picture observation's UI-thread/lifetime semantics, and
the task DAG. It must not infer API behavior from a success-only SDK example.
Independent exact-candidate code/native/privacy review happens after actual
required evidence. Architecture round1/global17 returned P0R-ARCH-001 BLOCKING
and P0R-ARCH-002 ADVISORY; this revision is contract correction1 and awaits the
same reviewer's bounded round2 response disposition. Round3 is reserved for
the actual combined code/native review, with no fourth round or counter reset.
No self-review or unavailable evidence is an approval.

This proposal makes no change to trusted checks/CI/workflow policy. Its narrower
composition dependencies are justified by the existing single-video Session
and original P0 composition-root boundary, not by a desire to bypass tests.
If implementation needs a new edge/policy, stop that affected change with the
actual diff and proposed trade-off; other assigned work can continue.

Service contracts are not stable guarantees. Unsupported paid/interactive/
advanced formats are explicit P0 boundaries, not promises that every Bilibili
video is playable. The first signed search/media/metadata requests still need
actual verification; previous login and unsigned-segment probes do not prove
them. Keep each unavailable capability fail/not-tested and identify the concrete
next action. Do not reduce mandatory login/danmaku/history criteria to finish.

Retained counters: formal reviews1/3 (architecture1/global17), contract
correction1/3 in progress, implementation repairs0 and repeated product failures0.
The controller assigns the next global number when the original reviewer
receives this complete response; author edits/schema checks are not another
formal round. Preserve all prior login, async and overlay history. Shared remote
issues remain0/5; no remote filing is
authorized. Controller inventory: .agent/p0-start/inventory.md. Workers write
their assigned scratch only; root owns .agent/runs. Worktree isolation is not a
security/approval mechanism. Local implementation commits follow existing owner
authority; this task grants no push/PR/merge/release operation.

Rollback preserves existing real credentials and history bytes and reverts only
this candidate; never roll back by deleting the Keychain item or resetting a
corrupt file automatically. History version changes require future migration
review. Required final output: reports/0039-p0-real-client.md, updated STATUS,
architecture navigation and a concrete preview artifact, with claim/evidence/
result/scope tables. Record candidate-specific local/native results, original
findings/dispositions, all owner-only actions and found-not-done exclusions.
The controller may propose P0 acceptance only after the integrated criteria;
a succeeded run never grants human acceptance, CI, merge or release authority.

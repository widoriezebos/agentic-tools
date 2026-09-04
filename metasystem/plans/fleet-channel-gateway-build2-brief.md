Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-04

# Goal

Build step 2 of goal fleet-channel-gateway (tier 3; design
metasystem/plans/fleet-channel-gateway-design.md at revision 4; step 1
landed). Step 2 is FCG-BUILD-13 (2): the provider contract (Ack,
Confirm, ErrBusy, the HTTP deadlines), the telegram and slack adapters
under it, the fake's controls and tokens, and StripCode plus the
masking as functions — while the old Poll keeps passing its saved
cursor, never calls Confirm and still binds through SplitTOTP, so what
Telegram is told and what binds are byte-for-byte what they were. The
proof that nothing moved is the existing channel fixtures
(scripts/agents/channel-fixtures.sh) and internal/channel tests
staying green against the extended fake.

# Workspace

The dispatch workspace as given, on the branch given. Do not stage or
commit; the seat lands the chain.

- May touch: metasystem/internal/channel/channel.go (the contract)
- May touch: metasystem/internal/channel/channel_test.go
- May touch: metasystem/internal/channel/telegram/telegram.go
- May touch: metasystem/internal/channel/telegram/telegram_test.go
- May touch: metasystem/internal/channel/slack/slack.go
- May touch: metasystem/internal/channel/fake/fake.go
- May touch: metasystem/internal/channel/fake/fake_test.go
- May touch: metasystem/internal/channel/totp.go
- May touch: metasystem/internal/config/validate.go (the numeric-knob list at 449, two keys)
- May touch: metasystem/cmd/metasystem/channel_verbs.go (only if the Provider interface change forces a compile fix; no new verb or flag)
- May write: metasystem/internal/channel/mask.go (only if masking does not fit totp.go)
- May write: metasystem/internal/channel/mask_test.go

Nothing under internal/goal, internal/steward, internal/channel/phase
or poll.go changes except what the interface change forces to compile
(poll.go's Receive call keeps its `old.Cursor` argument and ignores
`Ack`; it gains no Confirm call). No plan, record or docs change.

# Inputs

Read FCG-RECEIVE-03 (design lines 382-463), FCG-POLL-04's HTTP and
409 paragraphs (464-531), FCG-POST-08's deadline paragraph (660-672),
FCG-SECRET-15 (886-933), FCG-EVIDENCE-12's first paragraph on the
fake (934-965) and FCG-BUILD-13 (1089-1120). The cites below were
re-read on main after 757a31f8; internal/channel has not changed since.

## The contract (internal/channel/channel.go:18-43)

- `Inbound` gains `Ack Cursor` (the cursor whose confirmation
  acknowledges this item and everything delivered before it) and
  `UpdateID int64` (0 for providers without one).
- `Provider` gains `Confirm(ctx context.Context, dest DestinationConfig, c Cursor) error`.
  Receive keeps its signature and its second return (the batch
  cursor).
- New `ErrorKind` `Busy ErrorKind = "busy"` with `ErrBusy(problem)`
  shaped like the existing constructors.
- Deadlines: the adapters bound every request with a per-request
  context deadline of `channel.http-timeout-sec` (default 30) — the
  long poll uses T + 15 s where T is the `timeout` it sends. Read the
  knob the way `channel.poll-timeout-sec` is read
  (cmd/metasystem/channel_verbs.go:34-41; phase.Get) and carry it on
  DestinationConfig as `HTTPTimeout time.Duration` (0 → the default),
  so a caller that builds the config today needs no change. Add
  `channel.http-timeout-sec` and `channel.long-poll-sec` to the
  positive-integer knob list in internal/config/validate.go:449; no
  other config change (long-poll-sec is read by step 4; it is
  validated now so a conf line written early is not a typo later).

## Telegram (internal/channel/telegram/telegram.go)

- `updates()` (203-218) keeps sending the offset only when the cursor
  is non-empty — unchanged — and with an empty cursor sends no offset,
  limit 100, `timeout` = the long-poll seconds the caller passes (the
  old Poll's path keeps timeout 0: add the parameter with 0 from
  every existing caller). `allowed_updates` unchanged.
- Each returned Inbound carries `UpdateID` and `Ack = update_id+1`
  as a decimal Cursor. The filters at 181-183 (other chat, the bot's
  own messages) still drop items; their acknowledgement rides the
  next returned item's Ack or the batch cursor, as today.
- `Confirm(c)`: getUpdates with `offset` c, `timeout` 0, `limit` 1,
  discard the result; an empty c is a no-op returning nil.
- 409 (83-85): the description "terminated by other getUpdates
  request" becomes `ErrBusy` with that text; "webhook is active" keeps
  today's ErrReceiveFailed message. Read the description from the
  response body; an unrecognised 409 body stays ErrReceiveFailed.
- `New(nil)` (20-25) keeps http.DefaultClient; the deadline is the
  request context's, applied in `request()` from
  DestinationConfig.HTTPTimeout, so a caller-supplied client with its
  own timeout still works.

## Slack (internal/channel/slack/slack.go:96-146)

Each reply's Inbound carries `Ack` = its `ts` (UpdateID 0); `Confirm`
is a no-op returning nil; the per-root cursor stays as it is.

## The fake (internal/channel/fake/fake.go)

- Tokens: `/bot<token>/<method>` accepts any token of the form
  `fake-telegram-token-<listener>` and takes `<listener>` as the
  caller's name; the bare `fake-telegram-token` (36-40) stays accepted
  with listener "" so today's fixtures run unchanged. `provider()`
  mints the token from `channel.fake.listener` when the conf.local of
  the checkout sets it (read through phase.Get like the other keys;
  absent → the bare token). `channel.fake.listener` is not a numeric
  knob; add nothing for it to validate.go.
- One confirmed offset and one update stream across all tokens.
  getUpdates without `offset` returns only updates at or above the
  confirmed offset; with `offset` c it first sets the confirmed
  offset to max(current, c), then returns from c. Today's behaviour
  (278-300) for a caller that always sends its saved offset is
  therefore unchanged, and the first call of a fresh clone (no
  cursor) sees the whole unconfirmed stream as before.
- `timeout` T > 0 blocks up to T seconds until an update is
  available (poll the scripted file at 100 ms), then returns.
- The journal (`journal()` at 199, journal.jsonl) gains `listener`
  and a monotonic `sequence` on every row; the confirming getUpdates
  (one that carries an offset) is journaled with method `confirm` as
  well as the raw row — one row, `method: "confirm"`, the raw method
  in `raw`.
- Control file `<fake root>/control.json`, re-read per request,
  absent = no controls: `conflict: [{listener, remaining,
  description}]` (the next `remaining` getUpdates from that listener
  answer 409 `{ok:false, error_code:409, description}` and decrement
  the count in memory), `deliverOnlyTo: {"<updateId>": [listener]}`
  (other listeners never see that update), `pauseBefore: [{listener,
  method, until}]` (hold that listener's next request of that method
  — `getUpdates`, `sendMessage`, or `confirm` for the offset-carrying
  getUpdates — until the file at `until` exists, polling at 100 ms,
  then serve it and drop the pause). Malformed control.json → every
  request answers 500 with the parse error in `description`, so a
  fixture typo is loud.

## StripCode and masking (internal/channel/totp.go)

`StripCode(text string) (clean, code string, present bool)` exactly as
FCG-SECRET-15 states it, as a NEW function beside SplitTOTP;
SplitTOTP stays and every existing caller keeps calling it (the
cut-over is step 3). `MaskCodes(clean, secret string, at time.Time)
string`: every whitespace-delimited field of clean whose trailing
`.,;:!?` run is trimmed to exactly six ASCII digits and which
VerifyTOTP(secret, field, at) accepts is replaced by the literal
`[code]` (the trimmed punctuation kept after it), byte positions
otherwise untouched; a six-digit field the secret does not produce
stays. No caller in this step.

# Tests

internal/channel: StripCode's four cases ("approve 123456." →
("approve","123456",true); "order 123456 now" → no code; "123456"
alone → ("", "123456", true); "approve 12345" → no code); MaskCodes
on a repeated code, a mid-sentence code with a comma, and a six-digit
fact that is not a code; ErrBusy classification of the two 409
bodies (telegram_test.go, against an httptest server); Ack and
UpdateID on returned items and the batch cursor unchanged; Confirm's
request shape (offset c, timeout 0, limit 1) and empty-cursor no-op;
the request deadline is applied (a server that sleeps past a 1-s
HTTPTimeout returns a context error classified ReceiveFailed);
fake_test.go: two tokens share one stream and one offset, the
confirmed offset forgets below c, deliverOnlyTo hides an update from
the other listener, a conflict row counts down, pauseBefore holds
the request until the file exists, the journal carries listener and
sequence. Existing tests and scripts/agents/channel-fixtures.sh must
pass unmodified except for the compile-level Inbound/Provider
changes (a test that constructs an Inbound literal or implements
Provider gains the new members and nothing else).

# Verification

`go build ./...`, `gofmt -l` clean, `go vet ./internal/channel/...
./cmd/...`, `go test ./internal/channel/... ./internal/config/...
-count=1`, `go run honnef.co/go/tools/cmd/staticcheck@2025.1
./internal/channel/... ./cmd/...`, and
`scripts/agents/channel-fixtures.sh` green. Every path in your return
(diffBoundary, files) is relative to the repository root, so it starts
with `metasystem/`.

# Constraints

Wall-clock budget: 45 minutes. No verb or flag is added; the two
config keys above are the only config change; nothing writes under
plans/channel/; Poll never calls Confirm; SplitTOTP keeps every
caller; the steward and recovery are untouched.

# Gap Rule

Stop and report a gap only for a law-changing contract: a new
authority, a new refusal the design does not name, a landing bar, or
a fleet-read schema the design does not give. A mechanical choice —
a helper's name, a test fixture's shape, a field's exact tag where the
design names the key, an error's wording beyond the kinds the design
fixes — is made from what the tree does nearest the seam, recorded
in the return under `decisions`, and built; a choice recorded in the
return is not silent. Where an example in this brief contradicts the
tree's existing law, the law wins, the choice is recorded under
`decisions`, and the item is built.

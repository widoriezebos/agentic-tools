Working Mode: implement
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal alert-escalation-channel, overnight envelope R-38-m3)
Date: 2026-09-01

# Goal

Slice 1 of the alert channel, implemented exactly per the ACCEPTED
design plans/alert-channel-design.md (revision 5, landed 1a0fdcc3,
critique register closed with zero findings): the alert path with the
working unthreaded Telegram adapter — purely additive, no legacy
behavior changes.

# Workspace

The dispatch-created job worktree, branched from main. The design's
section 11 slice 1 defines the boundary of what exists when you are
done; the design's sections bind exactly as written — R-25b-m1: the
design is carried whole, and any deviation, simplification, or scope
cut you find necessary is a GAP to report, never a silent choice.

Expected new/touched areas (declare every touched path in
diffBoundary WITH the metasystem/ prefix): a new internal channel
package (contract, journal/transport split, sender flock, section 5a
completion merge, redaction), the Telegram adapter, configuration
key wiring per section 4, the health verdict line's undelivered
count, and their tests/fixtures. The legacy NotifyCommand path,
launch gate, and pending queue are UNTOUCHED — the design says
byte-for-byte.

# What binds (by design section)

- §2-3: the contract types — single-chunk Send live (SendResult one
  outcome; chunking dormant until slice 3); ConversationID accepted
  but the Telegram adapter ships unthreaded.
- §4: destination configuration for the alert class; credentials
  resolved from local config or environment only, with the
  secret-layer skip; unconfigured channel reports unconfigured and
  blocks nothing.
- §5 + §5a: journal phase under the alert lock with no network
  beneath it; transport after release behind the non-blocking
  single-flight sender flock; completion as reload-and-merge with the
  CONCURRENT-WRITER FIXTURE the design names (an acknowledgment
  recorded mid-transport survives; completion without its pending
  stamped attempt refuses and journals).
- §10: the redaction invariant with its KNOWN-BAD FIXTURE (a token
  never appears in journals, errors, or health text); failed sends
  journal and retry next pass; undelivered count reaches the health
  verdict line through existing plumbing.
- §11 slice 1: nothing else. Slices 2-6 do not exist yet.

# Constraints

- KNOWN SANDBOX LIMIT: no network in your sandbox — the Telegram
  adapter is proven against the design's fake endpoint seam, never a
  live call; environment-limited evidence is reported, not faked.
- Fixed constants and laws come from the design verbatim (including
  emailReferencesMaxBytes staying OUT of this slice — email is not
  slice 1).
- No test weakened; gofmt/vet clean.
- Wall-clock budget: 45 minutes.

# Expected Return

Version-2 implementer JSON; complete diffBoundary; evidence commands
replayable from the worktree root including:
- `go build ./...`
- `go test ./internal/... -run Channel -count=1` (or the package's
  actual test names — name them exactly)
- `gofmt -l` over touched Go files
- `go vet ./...`

# Gap Rule

stop and report a gap; never fill it silently — especially anywhere
the design underdetermines an implementation choice.

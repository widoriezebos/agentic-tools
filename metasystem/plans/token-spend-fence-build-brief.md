Working Mode: implement
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal token-spend-fence)
Date: 2026-09-03

# Goal

Goal token-spend-fence, step 1 (alert mode), Wido's word R-61-m1,
implemented exactly per the ACCEPTED design
metasystem/plans/token-spend-fence-design.md (revision 2; the one Sol
review, the fold and the closing review recorded in
metasystem/plans/token-spend-fence-dispositions.md, whose named test
obligations bind beside the design). Spend measured in
tokens and money per goal, machine and day from the runtimes' usage
records; the seat's own spend from the machine's transcripts; ceilings
in metasystem.conf; one health line every tick; one alert episode per
crossing; NOTHING REFUSED.

# Workspace

The dispatch-created job worktree, branched from main. The design's
sections bind exactly as written — ruling R-25b-m1: carried whole; any
deviation, simplification or scope cut you find necessary is a GAP to
report, never a silent choice. Under R-60-m1 the dispositions may name
test obligations that stand beside the design; they bind too.

Expected touched paths (declare every touched path in diffBoundary WITH
the metasystem/ prefix): a new package metasystem/internal/spend/
(Measure, the ledger writer, the transcript reader, tests and the
fixture bed testdata/bed-20260902/), metasystem/internal/mission/fence.go
(the lifted `JobUsageAt` returning the unreadable outcome; AggregateUsage
rewritten to call it and keeping its observable behaviour; the existing
test unchanged and green), metasystem/internal/config/ (the
six spend keys and the price prefix registered in the validator, the
committed-root law for them, defaults, tests),
metasystem/internal/steward/health.go (RoleSpendFence, checkSpendFence,
the role order, the role staying alive on a crossing, tests),
metasystem/internal/steward/alert_episode.go (the Owner field, the two
loops skipping spend-owned episodes, submitEpisode factored out,
UpdateSpendEpisodes called by the tick after the health updater; tests),
metasystem/internal/steward/tick.go (the one call),
metasystem/metasystem.conf (the keys commented, no price rows).
Nothing in the design's section 9 (non-goals) moves:
no adapter usage writer, no admission path, no goal or goalbudget
package, no new CLI verb.

# What binds (by design section)

- §2: the reader's rules in order; unavailable, pending-death-proof and
  UNREADABLE records enter no total and the unreadable ones are explicit
  ledger entries naming file and error; health goes unknown only when
  the jobs directory cannot be listed; day is the UTC date of
  startedAt; the ledger file per UTC day under
  artifacts/agents/steward/spend/, content-equal rewrites skipped.
- §3: the delegate session set from every readable job record
  (sessionId, resumedSessionId) excludes those transcripts in any
  launch mode, the worktree-cwd rule as a second guard; the 48-hour age
  filter for the DAY scope only, every present file for the goal scope;
  requestId dedupe last-wins; the token class mapping; goal "seat"; the
  explicit unattributed, codex-unmeasured, shape-failure and
  aged-file counts in the ledger and the health line.
- §4: native cost wins; derived cost per priced class; unpriced beside
  tokens, never zero; foreign currency counted beside; no price rows in
  the shipped conf.
- §5: the six keys, committed-root law, `spend.mode=enforce` refused by
  name, the derived defaults exactly as stated.
- §6 (revision 2): the exact health-line bytes; the spend role stays
  ALIVE on a crossing (the reason carries the CROSSED prefix and the
  named remedy; the spend-owned episodes of §7 deliver the alert, so the
  health digest never double-alerts); unknown only when the jobs
  directory cannot be listed, the ledger or the conf is unreadable,
  naming the path.
- §7: per-crossing episodes owned by the spend role inside the existing
  store, identity (scope-id, ceiling, multiple), submitted once, a new
  episode per further multiple, resolved and cleared when the crossing
  alone clears whatever the other roles' status; the Message carries the
  five facts; delivery through the existing `Deliver`.
- §8: every test by name and the `go list -deps` proof
  that no admission package imports spend; plus any test obligations
  the dispositions add.

# Test obligations from the closing review (bind beside the design)

Recorded in metasystem/plans/token-spend-fence-dispositions-closing.md;
each resolves one closing finding whose shape the design left
underdetermined, and the code review checks both:

1. TestTickCarriesSpendObservationAndUnknownDoesNotClearEpisodes: one
   typed spend observation (the crossings plus a measurement-valid
   flag) travels from checkSpendFence to UpdateSpendEpisodes through the
   tick — no re-measurement, no parsing of the health line — and an
   invalid or unknown measurement clears no episode.
2. TestSpendFenceHigherMultipleRearmsWhileLowerMultipleRemainsCrossed:
   a spend-owned episode persists its structured identity (scope-id,
   ceiling, multiple) in the episode store and is cleared per multiple
   when that multiple is no longer crossed, whatever lower multiples
   still are; a return to the higher multiple opens a new episode.

# Constraints

- KNOWN SANDBOX LIMIT: the full validation suite needs real process
  visibility your sandbox denies; run the focused proofs named below and
  report anything environment-limited as such, never faked. The
  transcript reader is tested on the fixture transcript only; never read
  the real ~/.claude in a test.
- No test weakened; gofmt and go vet clean; coverage floors for touched
  packages hold (metasystem/scripts/agents/coverage-ratchet.json).
- Wall-clock budget: 75 minutes.

# Expected Return

Version-2 implementer JSON; complete diffBoundary; evidence commands
replayable from the worktree root including:
- `go build ./...`
- `go test ./internal/spend/ ./internal/mission/ ./internal/config/ ./internal/steward/ -count=1`
- `go list -deps ./internal/dispatch ./internal/goal ./internal/goalbudget | grep -c internal/spend` (expected 0)
- `gofmt -l internal/spend internal/mission internal/config internal/steward`, `go vet` over the same

# Gap Rule

stop and report a gap; never fill it silently — especially anywhere
the design underdetermines an implementation choice.

Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal fleet-slack-channel)
Date: 2026-09-03

# Goal

The one code review of the fleet conversation channel build (job
fsc-build-2, Sol), tier 3 under R-54-m1: after this the code lands
through the chain. The build's worktree holds one commit, a3929f3e (21 files, 2537 insertions, 31 deletions), against
metasystem/plans/fleet-slack-channel-design.md revision 4 (§2–§8 are the
law; §5 and §8 carry the two reviews' obligations by test name) and
the build brief metasystem/plans/fleet-slack-channel-build-brief.md (landed). The orchestrator's
host gate on an export of that commit is recorded below; do not re-run it.

# The standard for a finding (R-60-m1, binding)

A finding is material only if it changes what gets built AND names the
artifact (file, function, test). At the end of this review every disputed
point is either a one-hunk fix the orchestrator folds through a fix build
or a named test obligation; never a raise for another review. Zero
material findings is a closing answer if the reading supports it.

# Attack surface, in priority order

1. The reply path (internal/channel/poll.go and totp.go): can any inbound
   envelope other than one whose UserID equals the configured human id AND
   whose text ends with a valid, unconsumed TOTP step reach the ledger as
   `actor=human:wido`? Trace the consumption row: written (fsync, rename)
   BEFORE attribution, under the flock, with the ALL-fields resume
   exception only. Is the ±1 window computed once per envelope, and is the
   step compared as an integer, not a code string?
2. Crash safety (poll.go, the phase machine): after each of MATCHED,
   RECORDED, RECEIPTED, CLOSED a re-poll must yield exactly one history op
   and one ANSWERED line. Is the op id allocated at MATCHED and persisted
   before the goal transaction? Is the history op and the next-step line
   ONE transaction? Is the destination cursor persisted only after every
   envelope of the pass is durably rejected, unmatched, or ≥ matched — and
   what happens to the envelopes beyond the five-disposition budget?
3. The grammar (internal/goal): `AUTHENTICATED_CHANNEL_WORD` with exactly
   the four keys channelProvider, channelUser, channelRef, channelStep in
   that order; the parser still rejects any other key; the strict token
   sits in the reason field where RecordedNormApproval scans; a repeated
   op id is a no-op.
4. The consumers (metasystem/cmd/metasystem/goalsync_mutations.go,
   metasystem/internal/humanauthority/authority.go,
   metasystem/internal/governance/types.go): `resume --approved-ref`
   and `set-obligation --approved-ref` accept ONLY an
   AUTHENTICATED_CHANNEL_WORD op on the SAME goal; set-budget and
   enroll-terminal unchanged; no existing refusal weakened.
5. Secrets (§6, alert §10): the bot token and TOTP secret are read only
   from environment or metasystem.conf.local; every resolved secret is
   literally scrubbed from every error string and log line, including
   HTTP errors from the Slack adapter; a committed secret-named key is
   reported and ignored.
6. The tick (metasystem/internal/steward/runner.go and the standalone
   tick in metasystem/cmd/metasystem/steward_verbs.go):
   the channel phase is last, after DeliverPending, outside RunTick and
   the arbitration lock, under one 15-second context; a failure is a count
   and a stderr line, never an error return; nothing in the drivers waits
   on it.
7. The fake (internal/channel/fake) and the fixture
   (scripts/agents/channel-fixtures.sh): the fake speaks Slack's JSON
   shapes and never invents an operation the Slack adapter does not make;
   the fixture asserts the rejection post, the `answer actor=human:wido`
   history line, the close post, and the claim with `--approved-ref`. Any
   sleep for ordering is a finding (R-35).
8. Tests: every test named in design §8 exists by that exact name and
   asserts what its name says, not a tautology. Any hunk outside the
   brief's diffBoundary is a finding. Any weakening of an existing refusal
   or assertion is a finding. R-31: no benchmarks.

# Host gate (recorded by the orchestrator)

On a clean export of a3929f3e: go-build.sh, gofmt -l (empty), go vet
clean; go test -count=1 over internal/channel/... (all 24 named tests pass
by name), internal/goal, internal/humanauthority, internal/governance,
internal/steward, cmd/metasystem: ok. channel-fixtures.sh: PASSED.
goal-cli-fixtures.sh: PASSED. The build's return declares no gaps.

# Return

Code-critic schema. Findings first, each naming the file, the function or
test, and the one-line change. Then one line: "agreed parts land as
written" or the disputed points as fixes or test obligations by name.

# Constraints

Wall-clock budget: 20 minutes. Your sandbox is read-only; verify by
reading. No redesign; the design's decisions are closed.

# Gap Rule

stop and report a gap; never fill it silently.

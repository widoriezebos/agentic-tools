Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal codex-handshake-budget-load-fragile)
Date: 2026-09-02

# Goal

Revision 2 of metasystem/plans/codex-handshake-design.md (revision 1
landed, in your worktree). Sol's round-1 register is
metasystem/records/misc/codex-handshake-critique-r1.md: six material
findings, every one a specification gap in Part 2 (sections 3, 4, 5); Part
1 (section 2) drew nothing and is not touched. Edit in place; diffBoundary
is that one file. Mark each closure with the finding identifier. Decisions,
not options. Keep it under fifteen minutes; you do not need to re-read the
code beyond the lines each finding cites.

# The folds, by id

1. CHS-R1-SNAPSHOT-01 (high). The snapshot is immutable evidence; an old
   snapshot selected into a new job after the upgrade must not be silently
   reinterpreted. Decide a snapshot-level discriminator and state it
   exactly. Recommended shape: the adapters write a NEW capability
   `handshakeProgressBoundSec` (codex 30, claude 20, devin 30) beside the
   old field; `capability.Select` (metasystem/internal/capability/select.go)
   reads the new field when present and otherwise DEFAULTS the old field's
   value into the same slot with the launch-anchored meaning recorded on
   the job record (a field such as `handshakeBound: "launch"|"progress"`),
   so an old snapshot keeps its old semantics until it is re-probed; state
   which value `metasystem/internal/dispatch/build.go` copies and how the
   waiter, custodian and `ComputeReapFacts` branch on it. If you choose
   another shape, say why it is smaller. Whatever you choose fixes D2.3 and
   the section-4 file rows (the three adapter scripts change after all).
2. CHS-R1-EXIT-02 (high). The critic fold must not erase the exit status.
   Choose: keep `handshake_failed:exit=N` unfolded for every role (state
   what the register consumer does with it — read
   metasystem/internal/adapter/adjudicate.go `criticFailureFold` lines
   155-179 and name the consumer that needed `protocol_error`), or fold to a
   parameterised `protocol_error handshake_failed:exit=N`. Fix D2.5, the
   verdict table, the dispatcher outcome line and the exit-before-session
   fixture so all four agree for a design-critic shaped job.
3. CHS-R1-DEADLINE-03 (high). One equality boundary. Define it as: the
   waiter refuses on the first poll where `now >= handshakeDeadline`
   (integer seconds), the custodian at `now >= handshakeDeadline +
   HandshakeCustodianGraceSec`, the reaper's `HandshakeWaiting` as
   `now < handshakeDeadline + HandshakeBackstopGraceSec` (unchanged). Write
   the algorithm and D2.6 with that exact comparison, and state the
   consequence: refusal lands within one poll interval after the bound.
4. CHS-R1-PROGRESS-04 (medium). Say exactly whether launch (P1) writes
   `handshakeProgressAt`. Recommended: yes — `BuildOwnershipPatch`
   (metasystem/internal/dispatch/ownership.go) writes both fields when it
   writes the deadline, so the discriminator is present from launch; then
   D2.4 names THREE writers, and ownership.go joins section 4 with its test
   (`TestOwnershipPatchPersistsTheNativeExactIdentity` in
   metasystem/internal/dispatch/exact_identity_test.go is the existing pin).
5. CHS-R1-FIXTURE-05 (high). `no-signal` must relate the verdict to the
   last-progress deadline: assert the record's final `handshakeDeadline`
   equals the refreshed value (not the launch stamp) and that the failure
   was written no earlier than that deadline (compare the job log's
   `handshake-timeout recorded` timestamp, or a `failedAt` field if the
   record has one — read metasystem/internal/dispatch/record.go for the
   field names) — state the exact assertion. `hang-gone-dispatcher`: every
   wait names its scaled ceiling from
   metasystem/scripts/agents/fixture-budget.sh (`harness_fixture_cap`);
   replace "wait for it" with the named helper and cap.
6. CHS-R1-FILES-06 (low). Section 4 lists every test file section 5 needs:
   the dispatch package's test files for `CustodyAdd`
   (metasystem/internal/dispatch/stage4_custody_test.go) and
   `ComputeReapFacts` (metasystem/internal/dispatch/decisions_test.go) and
   a new `handshake_progress_test.go` owned by the new source file.
7. Evidence note (no finding, orchestrator's fold). Goal
   codex-handshake-budget (m1b) recorded 8-14 s starts on m1b where
   "plugins disabled changed nothing", against m1's 1-second `plugins={}`
   measurement. Add one sentence to section 1: Part 1 is expected to fix
   m1 and is unproven on m1b; Part 2 is the fix that covers both.
   Also replace "twelve inspected" with a note that the specimen list is
   the design-critic jobs on m0b whose events.jsonl begins with
   `thread.started` (Sol's gap).

Consistency pass over the verdict table, section 4 and section 5 only
where these folds touch them; bump the header to revision 2 naming the
round. R-31: no benchmarks.

# Constraints

Wall-clock budget: 15 minutes. Design only; edit nothing but the design
file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the design file.

# Gap Rule

stop and report a gap; never fill it silently.

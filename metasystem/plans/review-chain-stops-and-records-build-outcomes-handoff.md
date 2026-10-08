# Build outcomes: unfinished handoff

Implementation stopped at the brief's threshold: 255 changed production lines,
counting additions and deletions and the new Go file. The change is uncommitted
and is not ready for acceptance. No local configuration was opened, and no
commit, push, rebase, stash or reset was performed.

The retained change freezes the builder's diff before checks, records empty and
oversized results, retains the builder's message path, and connects a gap
correction to the retained plan and gap. A person's size acceptance goes through
the existing authority and impact recorder and resumes the same round's checks.
The shared step driver retains physical execution ids, permits one environment
or declared-flake repeat, and stops deadline and unattributed failures.

The five questions currently have these owners:

- Production caller: UnitRunner.advanceRunning calls freezeBuildOutcome;
  intentInvocation.reviewUnit calls AcceptBuildSize.
- Freshness: freezeBuildOutcome retains worktree.diff; AcceptBuildSize holds the
  run lock and compares a fresh diff with that retained change.
- Actor: reviewUnit calls actingAs with actorHuman before recording the impact.
- Remedy: allowCorrection and reviseDecided admit a gap against its retained
  plan; AcceptBuildSize resumes the existing round rather than rebuilding.
- Unreadable input: I/O failures return errors; stepDriver.endStep defaults a
  failed command to unclassified. Full failure-question synchronization and
  unknown-input stop handling remain incomplete.

Would split into two units:

1. Build result holds and their public remedies: finish gap/size question and
   recovery handling, then repair the affected fixtures and message wording.
2. Failure attribution and execution accounting: run the exact failed command
   on the retained base once, recognize registered flakes, handle tree movement,
   and connect launch ids to the reservation owner's idempotent charge release.

Baseline comparison, demonstrated own attribution, launch-id reservation
reconciliation, build-failure questions and moved-tree retry handling have not
been implemented. No main incident is fabricated from a red branch base.

Verification observed so far:

- go build ./...: exit 0.
- go vet ./...: exit 0.
- Four new public-command tests: exit 0, including after the final source edit.
- internal/launch package: exit 1. Read partition fixtures exceed their declared
  size; real-Git proof fixtures have empty builder results; cause expectations
  also need reconciliation with the changed contract.
- Message and instruction selection: exit 1. New summaries contain the technical
  word "proof", and one physical-write refusal has no executable next command.
- Mutation control in a temporary source copy: exit 0. Moving the gap check after
  checks, excluding test lines from size, charging environment failures and
  assuming own attribution each produced exit 1 in the corresponding new test.
- Broad command selection: exit 1 after 1292.459 seconds. The failed groups are
  TestIntentConnectedJourneyRealClose, TestIntentBuiltUnitToLanding,
  TestIntentBuildResume, TestAuditMessagesAPersonReads, TestAuditMessagesTraced,
  and TestIntentUnitOnlyDecidesAfterARead/failed_checks_can_be_corrected.
- go run ./cmd/devgate static: exit 0.
- git diff --check: exit 0.

Commands issued for the final checks (each used METASYSTEM_TESTING_WORKERS=9):

```text
go build ./...
go vet ./...
go test -count=1 -timeout 30m ./internal/launch/
go test -count=1 -timeout 30m -run '^TestIntentBuild(EmptyStopsBeforeProofAndRetainsGap|SizeAcceptanceResumesProofAfterRecordedImpact|EnvironmentRetriesOnlyTheFailedStepOnce|UnattributedRedAndDeadlineStartNoRetry)$' ./cmd/metasystem/
go test -count=1 -timeout 30m -run 'TestLanding|TestWork|TestIntent|TestGoal|TestReview|TestUnit|TestClaim|TestSession|TestHelm|TestPolicy|TestQuestion|TestKeeper|TestDispatch|TestEvery|TestAudit|TestInstruction' ./cmd/metasystem/
go test -count=1 -timeout 30m -run 'TestAudit|TestInstruction' ./cmd/metasystem/
go run ./cmd/devgate static
```

git diff --stat reports 10 tracked files changed, 125 insertions and 24 deletions.
It excludes the new unit_build_outcome.go (122 lines), the new public-command
test file (230 lines), and this handoff. All three remain untracked.

All new tests and subtests call t.Parallel(). Tests use synthetic settings and
injected clocks. Existing fixture helpers now supply a nonempty builder diff and
expect it to be frozen before checks. Check logs and mutation exits are retained
under /tmp/build-outcomes-*. No receipt was appended because the brief forbids
changes to memory/ and records/.

Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Follow-up brief, slice 1 round 13: the third critique's three material findings, decided

Chain phd-build1-20260910. The third code critique
(phd-build1-crit4-20260910, reviewing round 12, tree
ca14dbc6129f24cff17781d286348b80174fcf5e) found three material defects
and two notes. Decided below. Report `round` as 13. Do not fetch or
rebase the worktree.

## Decisions

D-R13-1 (PHD-15, host-wait row decided by a sample count). The fixture
seam on `supervisorOptions` gains `OnReading func(reading string)`
beside `OnVerdict`, nil by default, invoked once each time a
nonterminal reading (`stopped`, `waiting on the host`) is first
recorded. The host-wait row keeps serving waiting samples until
OnReading has reported the reading, then serves the non-waiting sample
and closes the helper; the stopped row may use the same seam in place
of its scripted count. No row decides by how many samples it served.

D-R13-2 (PHD-16, the Linux counter decreases when a non-root member
reaps its own child). On Linux every live member counts its own utime
and stime plus its own cutime and cstime (fields 14 to 17), with no
retention: a child's seconds stay readable while it is a zombie and
move into its reaper's children figure when it is reaped, so the total
is continuous and never counted twice; the group counter stays
high-water. Darwin keeps retention (it exposes no children figure). The
Linux reaped-child row (build-tagged) adds a grandchild reaped by a
member other than the root and asserts the total never drops across
the reap. This replaces D-R11-5.

D-R13-3 (PHD-17, Darwin retention untested). A scripted row proves
retention: a member at 1.0 s vanishes; the next sample shows only the
root at a small figure; the counter keeps the vanished member's figure
and a later rise of one tick by the root registers as consumption, so
no `dead` verdict follows within the window; removing retention must
fail this row.

D-R13-4 (PHD-18, folded as a note): stage-result growth marks output
with the later of the tick time and the existing last-output time, so
the last-output time never moves backwards.

PHD-19 (note): accepted; proof row 4 asks for the wall comparison and
the measured share precondition guards it.

## Mandate for this round

1. Implement D-R13-1 to D-R13-4. Production changes: the OnReading seam
   (one invocation point, nil in production), the Linux accounting, the
   later-of-two output mark. Everything else is fixtures.
2. Keep every proof-standard test name in testing.json in step.
3. Nothing else changes.

## Proof

go build ./..., go vet ./..., gofmt -l; go test -count=1 and
go test -count=1 -race on internal/proofrun and internal/testpolicy,
green; CGO_ENABLED=0 GOOS=linux go test -c on internal/proofrun. The
seat gate runs the same with the real reader. Report the round as your
own.

## Constraints

Wall-clock budget: 30 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.

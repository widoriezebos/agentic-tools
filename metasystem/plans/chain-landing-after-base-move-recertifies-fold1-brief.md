Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal chain-landing-after-base-move-recertifies)
Date: 2026-09-08

# Fold brief: round 2 of chain clbm-build2

The closing read of round 1 (job clbm-crit2) returned three material
findings, none high, and two notes. Its register landed on trunk after
this worktree was cut, so everything is restated below rather than
cited. The specification is unchanged:
metasystem/plans/chain-landing-after-base-move-recertifies-design.md,
revision 2, final.

Round 1 is good work, and the read says so where it counts: it found no
way for unreviewed code to reach main through the new path, and it says
inside two of its own findings that the chain's content stays bound to
the merged tree. Fold these four items and change nothing else.

## D1 - CLC-01, medium - the recertified merge must run the whole merge gate

The ordinary merge stage loads the path-class manifest and refuses
unless every declared runtime instruction file carries the behavior
class. `mergeStage` in
`metasystem/internal/validate/conformance.go` does this at roughly lines
460-468. Its new sibling `mergeRecertified`, at roughly lines 512-545 of
the same file, never loads the manifest, so that refusal cannot fire on
the recertified path, and its waiver branch is missing too.

Nothing unreviewed reaches main because of this. What is wrong is that
one path passes a version of a gate that is missing a check the other
enforces. Add the same manifest check, and the same waiver branch, so
the two paths refuse the same things for the same reasons. If you find
any other check present in one and absent in the other, add it and name
it in the return; the read says this was the only one it found reading
both functions end to end.

## D2 - CLC-02, medium - the overlap canary must be able to fail

The design made one canary the arbiter of the overlap decision and
required each overlap case to show both offending base ranges. The
assertion in `internal/gittree/disjoint_merge_test.go`, at
roughly lines 86-89, fails only if the error does not convert, its kind
is not overlap, its path is not the fixture file, or either range start
is below zero. `HunkRange.Start` is an int whose zero value is zero, and
every value reaching it comes from an unsigned parse of ASCII digits, so
both range clauses are always false. An implementation that raised the
right refusal with both ranges empty would pass.

Replace those two clauses with assertions on the actual expected
ranges, so the test fails when a range is empty or wrong.

Then do this, and report it in the return: make the production refusal
site stop populating one of the two ranges, run the canary, and confirm
it fails. Restore the production code afterwards. An assertion nobody
has watched fail is not yet a test.

This is the fourth canary in this program that could not fail for the
right reason, so apply the same scrutiny to every other assertion you
touched in round 1: any clause that cannot be false is a defect, and
arithmetic vacuity like this one is the easiest kind to miss.

## D3 - CLC-03, low - the park path must not die under set -u

In `metasystem/scripts/agents/land.sh`, the branch that runs when the
candidate receipt check fails reads the gate-width variable with no
default, at roughly line 513. The only assignment is inside a
pattern-guarded block at roughly line 176, so a chain name outside that
pattern leaves it unset, and under `set -u` bash aborts with an
unbound-variable message before the park verb runs. The result is an
ending a reader cannot distinguish from a crash: no parked line, no
park-failed line, no durable park record, which is what the headless
failure contract exists to prevent.

Read it with an empty default and make the branch behave sensibly when
the width is unknown. Add a `bash -n` clean check and, if it can be done
cheaply, a case that exercises this branch with a chain name outside the
pattern.

## D4 - CLC-04, recorded as a note, cheap enough to fix

The design says blob comparisons run outside attribute-bearing
worktrees with no repository-local diff configuration loaded. The
implementation builds its comparison directory with the standard
temporary-directory call, which honours `TMPDIR`, so an operator whose
`TMPDIR` sits inside a Git repository would get a directory the design
wanted excluded. The read found no behaviour that changes as a result
and graded it non-material.

Fix it anyway if it is one call: create the comparison directory
somewhere the design's condition is guaranteed, or assert the condition
and refuse when it does not hold. If it turns out not to be cheap, say
so in the return and leave it.

## Not in this round

CLC-05 stays recorded and unactioned: the park's ten-second ceiling has
no channel through which an enclosing limit could shorten it, and no
caller passes one, so the clause has no effect today. Do not add a
parameter nothing uses.

## Scope

These four and their tests. Do not touch the decision, the proof, the
binding, the accounting, or anything in the design's out-of-scope list.
Do not weaken a test to make a change pass.

## Verification

Canary first, each reported individually:

- The three design canaries by their exact commands.
- The deliberate-failure demonstration required by D2, with what you
  changed, what the canary printed, and confirmation that you restored
  the production code.
- `go build ./...`
- `go test` once for `./internal/gittree ./internal/landing ./internal/validate ./internal/dispatch ./cmd/metasystem`
- `bash -n scripts/agents/land.sh` and `bash -n scripts/agents/commit.sh`
- `scripts/agents/go-gate.sh --fast`, once, at the end.

The `cmd/metasystem` process-group ownership probe cannot observe its
own spawned group inside a delegate sandbox; report it as
sandbox-limited, and the orchestrator runs that package outside the
sandbox. Do not run the fixture beds or a battery.

Gap rule: stop and report a gap; never fill it silently.

# Closing read: coordinator-context slice 3, unit C, after the scope cut and fold

Reader: Opus 5, second independent read. No edit to the worktree, no commit, no fixture bed.
Worktree: `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccb-s3/metasystem`
Base: `origin/main` at `113b3bcf`. HEAD equals the base; unit C is entirely working tree.
Round 1: `artifacts/reports/opus-read-c.md` (nine material findings F-1 to F-9).
Builder fold: `artifacts/reports/codex-ccb-slice3-result.md`, section "Scope and read fold round".

Surface read in full: the tracked diff (`internal/goal/project.go`, `internal/refusal/register.go`,
`internal/steward/revive.go`, `internal/usage/retention.go`) and the three untracked files
`internal/steward/handoff_capture.go` (1,145), `handoff_capture_test.go` (1,374),
`handoff_retention.go` (285). Re-read for context: `handoff.go`, `handoff_state.go`, `stage.go`,
`intervene.go`, `internal/usage/retention.go`, `internal/run/waiter.go`,
`internal/dispatch/record.go`, `internal/refusal/register_test.go`, `scripts/agents/go-gate.sh`.

Checks I ran myself, in two scratch copies (`scratchpad/mut2`, `scratchpad/probe2`; the worktree was
never modified):

- `go build ./...` PASS, `go vet ./internal/steward ./internal/goal ./internal/refusal ./internal/usage` PASS.
- Full `go test -count=1 -cover ./internal/steward/` PASS, 199.7 s, **coverage 80.1%** against the
  74.0 floor at `scripts/agents/coverage-ratchet.json:73`. No ratchet risk.
- **My own mutation battery: 40 single-rule mutations, each applied alone and reverted.**
  Five did not compile and four of those were reformulated and rerun. Of the 35 that ran,
  **28 were KILLED and 7 SURVIVED.** Every one of the twenty rows in the builder's corrected
  mutation table was independently reproduced: each named rule, deleted or inverted alone,
  went red in the test the table names. The table is now true.
- Four behavioural probes written as throwaway tests and deleted afterwards: the exact F-2
  retention scenario at 1d/7d/13d/14d/15d; the 14-day boundary; interrupted-publication residue;
  foreign-session damage on the Stop path.

## Answers to the seven questions

**1. Is the unbriefed journal subsystem genuinely gone, and is what remains briefed?**
Yes. `grep` over `internal/steward` finds no cancellation or retirement journal, no recovery state
machine, and no journal namespaces; the remaining `journal` hits are the pre-existing
`alert_episode.go` and `ledgerattention.go` subsystems. The shared-revival wiring is gone: the whole
`revive.go` diff is now the `prepareIntentUnderLock` extraction the brief asked for, plus liveness-
specific wording on the receipt-failure path. `handoff_retention.go` is 285 lines and every symbol in
it serves prune: directory sync, the canonical-parent walk, the nonce-in-use check across the six
lifecycle namespaces, the owned-member walk, the protected set, and `PruneContext` itself. I
inventoried every function in both new files against the brief and found nothing unbriefed. Unit C
now creates exactly one new store, `artifacts/agents/context/handoffs/<nonce>/` (three directory
creations in the whole unit: the canonical parent chain, the nonce directory, and its `references/`),
which is exactly the brief's cleanup table. Round 1's F-9 is gone with the code that caused it, and
the scope half of F-8 is closed.

**2. F-1, too-large before minting?** Fixed and proven. `buildHandoffState` now takes a `write bool`;
`Handoff` runs it once against a fixed 16-character preview nonce with `write=false`
(`handoff_capture.go:944`) before `createHandoffDirectory`, so the fit refusal happens before any
mint. The preview writes nothing. `TestHandoffRefusals/state_cannot_fit` asserts `mintCalls == 0`,
an empty result and zero directories and intents; deleting the preview call is KILLED by it (MA1).
Publication failures after the directory exists now clean the directory and say so
(`:958`-`:967`), proven by `TestHandoffCleansDirectoryPublicationFailure`. The one residue path
left is a crash or kill between `os.Mkdir` and `state.json`, which the brief deliberately keeps and
reports; see G-6.

**3. F-2, does prune retire at the documented default?** Fixed and proven. `PruneContext` now clamps
only the **usage** cutoff to usage's own floor (`handoff_retention.go:216`-`:220`) and keeps the
caller's cutoff for handoffs. My probe on a cancelled handoff aged sixty days, on the real wall
clock: `older-than=1d, 7d, 13d, 14d, 15d` each retired the directory with `err=<nil>` and
`handoffs=1`. The boundary behaves as the brief says: exactly 14 days old is retained (cutoff
equality), 14 days plus a minute is retired. The composition is deterministic rather than lucky:
steward's floor is `time.Now() - 14d - 1ns`, sampled strictly before usage samples its own, so the
strict inequality in `internal/usage/retention.go:78` holds by construction. Deleting the clamp is
KILLED by `TestContextPruneDefaultAgeComposesWithUsageFloor` (MB2).

**4. F-3, F-4, F-5.**
F-3 is fixed: the bad-bound and zero-clock refusals are now asserted on a clean bed with exact
messages before any damage is seeded (`handoff_capture_test.go:1254`-`:1263`). Replacing either
refusal with a silent clamp is KILLED (MA3, MA4).
F-4 is fixed in behaviour on both sides: waiters and jobs are now decoded cheaply and filtered by
owner and by goal **before** the stable double read (`:379`, `:433`), so an unrelated record never
reaches the stability check. Job and waiter records are both written atomically
(`internal/dispatch/record.go:651`, `internal/run/waiter.go:533`), so no torn read can refuse
either. The waiter half is proven (MA5 KILLED); the job half is not (MA6 SURVIVED) — see G-2.
F-5 is fixed and proven: `LiveHandoffForSession` takes no lock, creates nothing and runs no
recovery. Re-adding the arbitration acquire or a directory creation is KILLED by
`TestLiveHandoffReadDoesNotWaitOrWrite` (MA7, MA8), which drives the lookup while another holder
owns arbitration and asserts no `artifacts` tree appears under an empty root.

**5. F-6 and F-7, proof claims.** The two false claims are gone. The scratch fixture now writes six
bytes over six bytes and restores mtime, so only the byte comparison can catch it: deleting
`!bytes.Equal(first, second)` is now KILLED (MA9), where round 1 recorded it SURVIVED. The
C-CANCEL revival-routing claim is gone with the code. I re-ran the whole corrected table myself
rather than trusting it: all twenty rows reproduce. Of the twenty-two rules round 1 listed as
unproven, the brief-named ones now have failing tests (nonce collision across lifecycle namespaces,
incomplete authorization, predecessor verification, both duplicate-session guards, exclusive
creation, publication re-verification, stop-on-usage-error, the nonce-directory shape check, the
tree-member and canonical-parent symlink refusals, the pre-removal recheck, both caps, the
malformed-nonce report, the bound-handoff cancel guard). Seven rules still survive mutation; two of
them matter (G-2), five are redundant or defence in depth (G-8).

**6. Was anything the first read verified disturbed?** Yes, one thing. The expiry question is still
untouched: the only consultation is `handoff_capture.go:1086` with a zero clock, and
`TestLiveHandoffUsesNoExpiryClock` pins one call with `observedAt.IsZero()`. But **all eleven
refusal-register Site lines are now stale** — every one of them points at a line that does not emit
its code (G-1). Round 1 verified those eleven exact and warned that they hold only if the file lands
byte-identical; the fold moved the file and the register was not re-synced. Nothing in the suite
checks a Site line number, so the gate stays green. On size: I mapped every function in both new
source files to a brief obligation and found nothing hidden; the remaining 2,849 lines are the
briefed implementation plus the proof this review's own corrections demanded (G-3).

**7. Scope.** Clean. `git status --porcelain --ignored=matching` lists exactly the four tracked
modifications and the three untracked files, with no ignored residue beyond the pre-existing
`artifacts/` and `bin/`. `git diff 113b3bcf` against `testing.json`, both coverage-ratchet files,
`scripts/agents/roles/` and `docs/` is empty. `memory/receipts.log` is unmodified: no receipt row was
appended and no commit was made. Nothing writes outside the resolved state root, and the two extra
stores round 1 objected to are gone. The one new exported symbol outside steward is
`usage.CallRetentionWindow`, a two-line constant export that the F-2 fix needs and that the brief's
usage-retention permission covers; `goal.OwnedClaim` is the narrow accessor the brief's warning about
joining a later projection calls for, and it is exercised through the state-file test.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| G-1 | medium | yes | Every one of the eleven refusal-register rows unit C adds now names the wrong line. The register's documented contract is "The file and line of one emitting site" (`internal/refusal/register.go:16`), and the repository honours it exactly (`CONTEXT_EVIDENCE_RETIRED` -> `contextreport.go:59` emits that code on that line). The fold moved `handoff_capture.go` and the rows were not re-synced, so the change would land a register index in which no new row resolves. Round 1 verified these eleven exact and warned they hold only if the file lands byte-identical; this is a regression introduced by the correction round, and no test can catch it. | Checked every row programmatically. Register says LAUNCH_IN_FLIGHT 421 (real 453), NOT_FOUND 1049 (1110), NOT_HOLDER 266 (268), NOT_LIVE 951 (1033), NO_GOAL 340 (342), OTHER_PENDING 879 (937), REFERENCE 210 (212), STATE_MISMATCH 1087 (1142), STATE_TOO_LARGE 741 (776), UNOBSERVABLE 295 (297), WAIT_IN_FLIGHT 380 (396). `internal/refusal/register_test.go` checks that every code has a row and every override names a real verb; nothing checks a Site line, so `go-gate --fast` passes over it. |
| G-2 | low | yes | Two rules the fold depends on have no failing test. (a) The job-side pre-filter, which is this round's own F-4 correction, survives deletion: `TestHandoffIgnoresConcurrentUnrelatedRecords` fires its hook at `handoffCandidateRead`, which runs before the stable double read begins, so its job leg passes with or without the filter, while its failure message claims it covers "unrelated waiter completion or job progress". (b) The size/mtime/SameFile stability clause is now unproven, because narrowing the scratch fixture to a byte-only change (the correct fix for F-6a) removed the only case that exercised it; round 1 recorded that clause KILLED, it now SURVIVES. The clause is not redundant: bytes.Equal cannot see a change that reverts between the two reads or a same-content replacement on a new inode. | MA6 SURVIVED (24 s) and MA10 SURVIVED (26 s) in my battery. Hook at `handoff_capture_test.go:711`-`:718`; rules at `handoff_capture.go:433` and `:244`. Both close with one fixture change each: fire the job hook from `handoffSourceAfterRead` for a held-goal record, and add a second scratch case that changes mtime or size without changing bytes. |
| G-3 | low | yes | The size mismatch from round 1's F-8 is still open: 2,849 changed lines against the brief's "keep each unit near or below 1,500 changed lines". The builder disclosed it and did not call the unit certifiable. It is a brief rule that is not met, and it stays material until the brief's owner adjudicates it. | My own count matches the builder's: 45 tracked (43 insertions, 2 deletions) + 1,145 + 285 + 1,374 = 2,849. Non-test source is 1,475, inside the ceiling; the overrun is the proof surface. See the recommendation below for why splitting does not fix it. |
| G-4 | low | no | `context prune --older-than 1d` now silently retires usage pairs at the 14-day floor instead of at one day, and prints only a count. The clamp is the right direction (it always retires less, never more, than the caller asked) and usage's floor is not overridable, but nothing tells the operator that the usage half did less than requested. | `handoff_retention.go:216`-`:220`; unit D prints `pruned call-sessions=<N>`, an honest count of what was retired. Worth one sentence in unit D's output or the orchestration text, not a code change. |
| G-5 | low | no | `PruneContext` now reads the wall clock itself (`time.Now()`), so it is no longer a pure function of its injected `now`, and it re-derives usage's strictness rule in the caller with a `- time.Nanosecond` fudge. This couples steward to a comparison that lives in usage. | `handoff_retention.go:217`. The ordering is deterministic rather than load-fragile, and `TestContextPruneDefaultAgeComposesWithUsageFloor` fails if usage's rule moves (MB2 KILLED), so the coupling is caught. A `usage.CallRetentionFloor()` accessor would be cleaner than an exported constant plus a fudge. |
| G-6 | low | no | An interrupted publication (kill, OOM, reboot between `os.Mkdir` and `state.json`) still leaves a nonce directory that `PruneContext` reports on every run and never removes, which unit D will map to exit 1 forever with no verb to clean it. This is the brief's chosen behaviour ("Keep malformed or incomplete directories and report them for inspection"), so it is a note, not a defect. | Probe: three consecutive prunes over a bare nonce directory each returned `keep incomplete handoff 7c00000000000001: handoff state is not a regular file ...` with the directory still present. The operator remedy is a manual `rm` of the named directory; unit D's documentation should say so. |
| G-7 | low | no | Nothing pins the preview nonce to the same length as a minted nonce, and the fit computation is exact only because both are 16 characters. Shortening the preview constant leaves every test green. The consequence if it ever broke is mild: the refusal moves back after directory creation, where the cleanup path removes the directory anyway. | MA28 SURVIVED. `handoff_capture.go:944` versus `randomHandoffNonce` at `:108`. A one-line assertion (`len(previewNonce) == len(minted)`) would pin it. |
| G-8 | low | no | Four more rules survive mutation, all redundant or defence in depth: the post-write `validateManifestReference` (the pre-mint state verifier already covers it, which is why no test can kill it), the `.tmp.json` job exclusion, the waiter schema-2 check, and the delegate's `status == "running"` requirement (already implied by `ConsumedActiveJob`). | MA26, MB35, MB36, MB38 all SURVIVED. Recorded, not actioned. |
| G-9 | low | no | The `prepareIntentUnderLock` extraction also changed the shared revival failure message into three liveness-specific branches; the two new branches are untested and affect ordinary `seatIdle` revival, not only handoffs. Message-only on an already-failing path, and aligned with the brief's "never claim the old handoff is still live". | `internal/steward/revive.go:74`-`:82`; no test change in the tracked diff, `revive_test.go` untouched. |
| G-10 | low | no | Carried from round 1's F-13 and unchanged: a foreign session's drifted handoff turns this session's Stop lookup into an error rather than "no allowance", because every live handoff is verified before the session comparison. The brief makes this the right outcome ("An unreadable handoff store produces the ordinary infrastructure verdict, not an allowance"). | Probe: with another session's `state.json` overwritten, `LiveHandoffForSession(root, "a totally different session")` returned `err=handoff state digest mismatch expected=... found=...` rather than `live=false, err=nil`. `handoff_capture.go:1083`. |

## Mutation battery detail

KILLED (28): preview fit before minting; usage-floor composition; bad-bound refusal; zero-clock
refusal; waiter pre-filter; Stop-path lock-free read; Stop-path no directory creation; second-read
byte comparison; stop-on-usage-error; nonce-directory shape; tree-member symlink refusal; canonical-
parent redirect refusal; pre-removal recheck; nonce-in-use across lifecycle namespaces; exclusive
creation; publication re-verification; strictly-older cutoff; protected set; incomplete-authorization
error; predecessor verification before supersession; single same-session predecessor; single live
match on the Stop path; bound-handoff cancel guard; landing-history cap; receipt cap; machine-versus-
claimant equality; removal reported before the parent sync; malformed-nonce report.

SURVIVED (7): job-side pre-filter (G-2a); size/mtime/SameFile clause (G-2b); post-write manifest
validation, `.tmp.json` exclusion, waiter schema check, delegate running-status check (G-8); preview
nonce length (G-7).

## Verdict

VERDICT: not certifiable as it stands, on three material findings (G-1, G-2, G-3), none of which is a
defect in behaviour. The nine material findings of round 1 are genuinely closed: I reproduced the
correction for each one with my own mutations and probes rather than reading the report's table. The
unbriefed subsystem is gone with its wiring, the too-large refusal precedes minting and leaves
nothing behind, prune retires at the documented default, the refuse-rather-than-clamp rule and the
lock-free Stop read are proven, unrelated records no longer refuse a handoff, and the false proof
claims are corrected and true.

What stands against landing is one stale index, one test-fixture gap, and the size mismatch. G-1 is a
mechanical re-sync of eleven line numbers and must be done before the file lands, because the numbers
are only stable once it does. G-2 is two fixture changes of a line or two each. Neither needs another
builder round; both are inside the fix-forward lane.

On the size: I do not think 2,849 lines is a reason to split rather than land, and I would say so to
Wido plainly. The brief's own suggested split ("state encoding and references, then admission and
publication") divides the source, but the tests follow the code: the prune half comes to roughly 595
changed lines and the capture, admission and publication half to roughly 2,254, so the larger half
would still be half again over the ceiling. The split therefore does not buy conformance. It also
runs into the brief's own rule that each unit must compile and pass its focused checks: an
intermediate unit holding the capture helpers without their caller would fail the gate's
`staticcheck ./...` on unused unexported functions, so the first unit would have to carry `Handoff`
anyway. The ceiling exists to bound review cost, and that cost has now been paid twice by a reader
who found the unit coherent both times. Record the deviation, adjudicate it, and land.

# Critique trail — metasystem full-system review

The review report (`2026-08-12-full-system-review.md`) is being critiqued by codex
`gpt-5.6-sol` at xhigh reasoning, per the standing critique discipline, looping until
neither side has an objection that would change the execution backlog. This file records
each round verbatim, followed by the adjudication of every point.

## Round 1 — Codex critique (verbatim)

VERDICT: REVISE

1. **MATERIAL — Claim:** `foundations-1` adequately fixes evidence GC by separating checkout namespaces and honoring the mirror path.
   **Evidence:** A manifest records the pre-mirror semantic hash (internal/dispatch/mirror.go:200), but close-check subsequently adds `chainClosed`/`runnerClosed` without remirroring (scripts/agents/dispatch.sh:1316). GC checks only manifest presence, not whether its hash matches the current record (internal/evidence/gc.go:365). It can therefore delete the only final-state record while preserving a stale pre-close mirror.
   **Change:** Elevate this to W1/high and revise the target: mirror after the closing CAS, or make closure part of the mirror protocol; require current semantic-hash equality before pruning. Amend the report's "no data loss" conclusion.

2. **MATERIAL — Claim:** The existing dispatch owner lock is safe to reuse for cap authority in `script-orchestration-01`.
   **Evidence:** A live process with unreadable argv produces `Alive=true, ArgvKnown=false` (internal/identity/identity.go:39). `holderState` ignores `ArgvKnown`, interprets the empty command as stale, and permits takeover (internal/dispatch/ownerlock.go:54, internal/dispatch/ownerlock.go:104). Existing tests cover readable-live and dead processes, not unreadable argv (internal/dispatch/ownerlock_direct_test.go:42).
   **Change:** Add a W1 prerequisite: inject process probing and treat every live-but-unverifiable holder as busy. Do not expand owner-lock use until that behavior is tested.

3. **MATERIAL — Claim:** `lease-census-1` says malformed job state already refuses loudly, so only filesystem census errors require correction.
   **Evidence:** Job-record read failures, JSON errors, and missing job IDs are silently skipped (internal/lease/classify.go:222). Process identity and parent lookup errors are also collapsed to false (internal/lease/identity.go:40, internal/identity/enumerate_linux.go:46), after which classification can return HUMAN and bypass lease restrictions (internal/lease/verbs.go:185).
   **Change:** Expand W1.1 to fail closed on malformed/unreadable job records and represent identity/parent results as known, absent, or unknown. Test each unknown path.

4. **MATERIAL — Claim:** `lease-census-2` is executable by rejecting unreadable or unparseable sweep state.
   **Evidence:** Valid JSON with missing/noninteger epochs or missing/unknown statuses is accepted as non-owning without error (internal/lease/sweep.go:64). During process-group inspection, per-process `getpgid` and argv failures are skipped, yet ownership is still declared provably false (internal/lease/sweep.go:126).
   **Change:** Require schema-valid states and propagate any member-inspection uncertainty as "unprovable." Fold the later uncertainty tests into W1.2 rather than treating them as separate hardening.

5. **MATERIAL — Claim:** `mission-contract-2` should run gate and guards in one materialized worktree.
   **Evidence:** Gate execution permits an arbitrary command in the candidate worktree (internal/contract/contract.go:845); guards then inspect that same filesystem model (internal/contract/measure.go:141). A gate can modify or delete guard-observed files, so sharing one worktree does not guarantee one candidate snapshot.
   **Change:** Pin the candidate SHA once but use independent clean worktrees, or reset and clean between gate and guard execution. Add a test where the gate mutates a guarded path.

6. **MATERIAL — Claim:** `cli-1` is primarily a policy-location/testability problem suitable for W3.
   **Evidence:** The arming scan silently skips unreadable or malformed job, fence, and reservation records (cmd/metasystem/supervise_arming.go:42, cmd/metasystem/supervise_arming.go:61), then returns success if no readable blocker remains (cmd/metasystem/supervise_arming.go:89). Moving unchanged code merely relocates a fail-open correctness defect.
   **Change:** Split the item: W1 must distinguish ENOENT races from read/parse/schema failures and refuse unsafe arming; W3 can then relocate the tested policy.

7. **MATERIAL — Claim:** `mission-contract-3` covers the correctness-relevant unbounded subprocesses.
   **Evidence:** Contract and mission still have bare `gitTry` execution (internal/contract/helpers.go:94, internal/mission/anchor.go:41). Gate and guard commands use `CommandContext` without process-group termination (internal/contract/contract.go:887, internal/contract/measure.go:248). Adapter signature discovery is fully unbounded and lies on lease classification paths (internal/census/fingerprint.go:43).
   **Change:** Replace W1.5 with a production-wide subprocess inventory. At minimum cover mission/contract git calls, gate/guard process groups, and census fingerprinting; preserve existing exit-code semantics.

8. **MATERIAL — Claim:** `lease-census-9` sufficiently addresses record-lock blocking by bounding the sweep lock.
   **Evidence:** The canonical dispatch record lock uses an unbounded blocking flock (internal/dispatch/record.go:90). Multiple dispatch record operations execute through `lease_run_held` (scripts/agents/dispatch.sh:999, scripts/agents/dispatch.sh:1324), while `RunHeld` owns the global lease lock (internal/lease/verbs.go:270).
   **Change:** W1 must also bound or redesign the canonical record-lock acquisition and test a held record lock under `run-held`.

9. **MATERIAL — Claim:** Evidence namespacing is completely covered by the checkout-segment correction in `foundations-1`.
   **Evidence:** Durable event archives still use only `basename(checkoutRoot)` (internal/evidence/gc.go:627). A same-name divergent destination is replaced before the local source is deleted (internal/evidence/gc.go:657); archive names are timestamp-plus-PID (scripts/agents/arm-supervision.sh:156). Same-basename checkouts can overwrite one another's durable evidence.
   **Change:** Apply the same stable checkout segment to event archives and manifests, with an explicit legacy-ownership rule and a two-checkout collision test.

10. **MATERIAL — Claim:** The report's release calibration—"nothing threatens … a broken release today"—is compatible with the adoption findings' W4 placement.
    **Evidence:** Adoption copies scripts and a locally built binary but not Go source (scripts/adopt.sh:162, scripts/adopt.sh:248); the binary is ignored, while the installed workflow runs validation (scripts/adopt.sh:311). Validation skips rebuilding without source and then invokes a shim requiring the absent binary (scripts/validate-metasystem.sh:65, scripts/audit-metasystem.sh:11). A fresh adopted CI checkout is therefore red by construction.
    **Change:** Call this an adoption/release blocker and move the required owner decision ahead of W4 implementation.

11. **MATERIAL — Claim:** `architecture-1` is a high-severity, executable W2 item.
    **Evidence:** The report separately downgrades the same duplicated Devin usage policy under `cli-3` because both implementations are identical and tested. The proposed target also permits either host importing adapter or a new leaf package, while W2's stated boundary goal requires mission/fence to stop importing adapter (internal/mission/fence.go:18).
    **Change:** Consolidate the duplicate findings at medium severity, place them behind live correctness work, and select one leaf policy owner rather than presenting architecturally incompatible alternatives.

12. **MATERIAL — Claim:** W6 retirement items may delete obsolete scripts immediately after parity.
    **Evidence:** Adoption ships the script tree (scripts/adopt.sh:162), and validation/canary entry points invoke these scripts directly (scripts/validate-metasystem.sh:306, scripts/canary.sh:46). Whole-file retirement therefore changes the shipped command surface, but those deletions are absent from the sign-off decisions.
    **Change:** Gate every shipped-entry-point retirement on owner sign-off, or preserve the filename as a thin compatibility wrapper.

13. **MATERIAL — Claim:** `dispatch-supervise-6` is an executable W1 task.
    **Evidence:** The implementation counts only the group leader (internal/supervise/proc.go:166), and the ceiling consumes that count (internal/supervise/owner.go:270). The report's Target offers mutually exclusive outcomes: implement real enumeration, or merely document the ceiling as fixture-only.
    **Change:** Choose real group enumeration and keep it in W1, or choose documentation-only and move it to W7.

MISSED — Material omissions are:

- Live owner-lock takeover when argv is unreadable.
- Loss of post-mirror closure state and cross-checkout event-archive collisions.
- Fail-open arming scans on malformed or unreadable blocker state.
- Unbounded canonical dispatch record locks while the global lease lock is held.
- Unbounded subprocess paths beyond the two identified by `mission-contract-3`.

## Round 1 — Adjudication

Every code-level claim was independently verified against the tree by three
verification agents before acceptance (the same discipline the original findings went
through). Result: 12 of 13 points accepted, 1 partially accepted with the data-loss
half rebutted. Report deltas: six new findings (codex-1 … codex-6), four expansions,
two fix-target revisions, one severity consolidation, three backlog/process changes.

1. **ACCEPTED → codex-1 (high/M), leads W1.** Verified end to end: mirroring happens
   only on the transition to terminal; the close CAS re-mirrors nothing; GC prunes on
   manifest presence alone after the grace window, and the stop hook runs GC every turn
   end. chainClosed/runnerClosed are permanently lost. The report's "no data loss"
   verdict wording was amended.
2. **ACCEPTED → codex-2 (high/S), W1.4, prerequisite of W4.1.** Verified: both probers
   return Alive+ArgvKnown=false on argv-read failure; holderState never consults
   ArgvKnown and classifies the holder stale; takeover proceeds. Directly contradicts
   ownerlock.go's own header and the identity doctrine; the repo fixed the same class
   in identity.Custodian (B1 row 5) — holderState is the missed consumer. Test gap
   confirmed.
3. **ACCEPTED → lease-census-1 expanded + codex-3 (high/M).** Verified with one line
   correction (the skip legs are classify.go:224-227 and :234). Two genuinely new legs
   beyond the original finding: corrupt job records fail open (only reads were
   covered), and the identity/ancestry error collapse (psIdentity conflates ESRCH with
   EPERM at census/verbs.go:44-47; ParentPid collapses errors to false; the truncated
   walk lands on ClassHuman). Consequence (HUMAN bypasses the holder gate) was already
   the original finding's; inputs widened.
4. **ACCEPTED → lease-census-2 expanded (W1.3).** Verified: schema-invalid records
   (missing/noninteger claimEpoch, unknown status, sweep.go:72-75) take the same silent
   skip as legitimately terminal jobs, violating the function's own contract comment;
   groupOwnsTag returns "provably not owned" despite skipped members on non-ESRCH
   errors (:131-140). Both outside the original target's read/unmarshal scope.
5. **ACCEPTED → mission-contract-2 target revised.** Verified: gate and guards are
   arbitrary bash with cwd inside the worktree; guards read the working tree; a shared
   worktree lets the gate pollute guard readings and defeat the frozen-instruments
   restore (gateRef is also resolved twice). New target: pin candidateSHA and gateRef
   once; independent clean worktrees at the pinned SHAs.
6. **ACCEPTED → codex-4 (high/S) + cli-1 split (fix in W1.5, relocate in W3.1).**
   Verified with one precision the report keeps: the fence-side job-status lookup is
   already fail-closed (status "" blocks) and must be preserved; the fail-open legs are
   job-carried reservations and unreadable/corrupt fence files.
7. **ACCEPTED → mission-contract-3 expanded (W1.8).** Verified inventory: seven sites —
   the two original, both gitTry copies, census.SignatureText (worst: hangs watcher
   passes and lease classification), and the two CommandContext execs bounded in name
   only (no process group; grandchildren holding the pipe block Run() past the
   ceiling, and the deferred worktree cleanup waits with it).
8. **ACCEPTED → codex-5 (medium/M, W1.20).** Verified: withRecordLock is a second,
   separate unbounded flock; record ops run as children of RunHeld while it holds the
   global lease lock; a wedged holder stalls every claim/renew/succession. Calibrated
   medium on the verified blast radius: persistent refusal (bounded acquires elsewhere
   refuse loudly), wedge propagation rather than deadlock, consistent lock ordering.
9. **PARTIALLY ACCEPTED → codex-6 (low/S).** The basename-keyed events directory and
   the replace-before-delete order are real, and the mirror-style segmentation is the
   right fix. The data-loss framing was REBUTTED: archives are named
   events-UTCsecond-PID[-n].jsonl, so a divergent same-name collision requires two
   simultaneously live processes sharing a PID in the same second — sub-second PID
   wraparound. Filed as consistency hardening, not a reachable data-loss bug.
10. **ACCEPTED (calibration).** Verdict wording amended; the adopted-engine-delivery
    ruling (r10) is now marked decide-first, ahead of the whole W4 stream.
11. **ACCEPTED (calibration).** The triple-reported usage duplication consolidates at
    medium (the cli-3 verifier adjudication was right: both copies tested and identical
    today). Owner decision recorded: a small leaf package, because mission/fence.go
    must also come off its adapter import — host-imports-adapter fixes half the graph.
12. **ACCEPTED (process).** W6 gains the gate: adoption ships scripts/, so every
    whole-file retirement is itself a sign-off item or the filename survives as a thin
    wrapper. Added to Decisions A.
13. **ACCEPTED (resolution).** dispatch-supervise-6 commits to real enumeration —
    AllPids + Getpgid inside processGroupMembers, precedent already shipped at
    internal/census/production.go:41, ~10 lines, no import cycle. The
    documentation-only option is withdrawn.

MISSED-list disposition: all five entries correspond to points 2, 1/9, 6, 8, and 7
above — four accepted as new or expanded findings, the archive half of the second one
rebutted as above.

## Round 2 — Codex critique (verbatim)

VERDICT: REVISE

1. **MATERIAL — `codex-1` offers an unsafe fix alternative.** Re-mirroring after the close CAS is insufficient by itself: the CAS has already persisted closure state (scripts/agents/dispatch.sh:1324), while mirroring can fail before updating the manifest (scripts/agents/dispatch.sh:725). The old manifest remains, and GC still deletes by manifest presence and age alone (internal/evidence/gc.go:381). Even if `close` reports failure, a later GC loses the already-written closure state.
   **Change:** Make current-record/manifest semantic-hash equality mandatory before pruning. Post-close re-mirroring may be added for prompt durability, but not presented as an alternative to the GC guard. Test close → forced mirror failure → GC retention.

2. **MATERIAL — `codex-3` can still authorize an erroneous classification other than HUMAN.** The revised target permits merely refusing HUMAN after uncertainty. But when identity reads fail for an intermediate adapter/supervision ancestor, `Classify` skips that classification and continues upward (internal/lease/classify.go:282); it can then recognize a higher authenticated main at line 279 and return `ClassMain`. `RequireHolder` can authorize that result as the checkout holder (internal/lease/verbs.go:212).
   **Change:** Any non-absence identity, command, start-time, or parent-read uncertainty encountered before classification completes must abort classification; it must not continue to a more privileged higher ancestor. Add a child → unreadable adapter → authenticated-main test.

3. **MATERIAL — the chosen `dispatch-supervise-6` implementation leaves process-count uncertainty fail-open.** Real enumeration makes `GroupCount` capable of returning errors, but `Owner.Cycle` ignores the error and retains the independently computed observation, which can remain Healthy and reset the breaker (internal/supervise/owner.go:270). The cited census precedent also converts every `Getpgid` error into PGID zero (internal/census/production.go:41). A process-table denial can therefore undercount while supervision continues normally.
   **Change:** Specify that only ESRCH is an absent member; `AllPids` or any other `Getpgid` failure makes the count indeterminable. Map that error to `Indeterminable` in `Owner.Cycle`, preventing reset or relaunch, and test both error paths.

4. **MATERIAL — the "full verified" seven-site subprocess inventory remains incomplete.** Dispatch still runs bare, unbounded git commands (internal/dispatch/gitcmd.go:13), used while building records (internal/dispatch/build.go:127). The shell acquires the chain and global cap-authority locks first (scripts/agents/dispatch.sh:1002), then invokes `build-record` (scripts/agents/dispatch.sh:1058). A hung git therefore blocks dispatch and arming indefinitely. Other omitted bare git helpers remain in conformance and frontier processing (internal/validate/conformance.go:77, internal/report/frontier.go:153).
   **Change:** Expand W1.8 to an actual production `exec.Command` inventory. Add dispatch's locked `build-record` path to W1, with a hanging-git test, and classify the remaining intentional long-lived versus improperly unbounded calls explicitly.

5. **MATERIAL — accepted revisions were added as annotations without replacing contradictory formal targets.** Most importantly, the revision says `mission-contract-2` must not share a worktree (report:413), while its formal Target still instructs one shared worktree (report:422); arbitrary gate execution makes the old target unsafe (internal/contract/contract.go:890). The same patch seam remains for `mission-contract-3`, `lease-census-1`, `lease-census-2`, `dispatch-supervise-6`, and `architecture-1`: their notes/backlog contain the accepted resolution, but their formal Targets retain the superseded scope or alternative.
   **Change:** Rewrite each canonical Current/Target/Why and severity line to contain the adjudicated result, then remove the superseded annotation. An implementer must never have to decide whether the italic note or the designated Target governs.

6. **MATERIAL — W1 delays the test seam until after the high-risk lease fixes.** W1.2–W1.3 implement new fail-closed classification and sweep behavior, but the process-table seam and refusal tests remain W1.25. Today `groupOwnsTag` hardwires `AllPids`, `Getpgid`, and `ProcessCommand` (internal/lease/sweep.go:126); existing tests cover only real-kernel success and known absence (internal/lease/sweep_test.go:43). This contradicts the accepted request to fold uncertainty tests into the correctness fix.
   **Change:** Move the necessary seams and refusal tests into W1.2/W1.3 and require them in the same changes. Leave W1.25 only for additional coverage not required to prove those fixes.

## Round 2 — Adjudication

All six points ACCEPTED. The genuinely new code claims were verified directly before
acceptance: internal/dispatch/gitcmd.go is a bare exec.Command with stderr discarded,
called from build.go:127/131 on the locked build-record path; bare git helpers
confirmed at validate/conformance.go:78/87 and report/frontier.go:154 (round 1's
verification swept only contract, mission, and census — the scope gap was real);
Owner.Cycle runs the ceiling check only under err == nil; census production
enumeration collapses Getpgid errors to PGID 0 and skips prober errors entirely; the
classification walk demonstrably skips an ancestor whose ProcessCommand or StartedAt
read fails and continues upward, and RequireHolder's MAIN path grants holder authority
and even auto-claims an absent lease.

Report deltas: codex-1's GC hash guard made mandatory (re-mirror demoted to
supplement); codex-3's target now aborts classification on any pre-completion
uncertainty, with the MAIN-escalation path documented in Current/Why; the
dispatch-supervise-6 spec gained the uncertainty mapping (only ESRCH is absence;
count errors map to Indeterminable in Owner.Cycle); mission-contract-3's inventory
expanded to ten sites plus a classify-the-rest sweep and a hanging-git test, effort
M-L; all six annotated findings (mission-contract-2, -3, lease-census-1, -2,
dispatch-supervise-6, architecture-1, plus adapter-host-registry-1) had their
canonical Current/Target/Why and severity lines rewritten to the adjudicated content
and the italic annotations removed; W1.2/W1.3 now carry their seams and refusal tests
in the same change, with W1.25 reduced to residual coverage. No new findings, no
severity changes.

## Round 3 — Codex critique (verbatim)

VERDICT: AGREE

## Convergence

The loop converged after three rounds on 2026-08-13. Round 3 carried this side's
stated position (converged; remaining differences wording-only) and Codex's verdict on
the revised report was AGREE with no further points. Totals across the loop: 19
critique points raised, 18 accepted (6 new findings, 6 expansions or fix-target
revisions, 3 calibration changes, 3 process/ordering changes), 1 partially accepted
with its data-loss half rebutted on evidence. Every accepted code claim was verified
against the tree before acceptance; no point was accepted on assertion alone.

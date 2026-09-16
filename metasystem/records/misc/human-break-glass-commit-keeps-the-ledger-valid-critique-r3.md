# Design critique: human-break-glass-commit-keeps-the-ledger-valid, revision 3

Review round: 3 of 3  
Reviewed commit: `7df4c50878dffe35f3f8e33826fb7909e5148efb` (`origin/main`)  
Material findings: 4

Evidence level is `read`. I read the complete revision, brief, goal record, revision-2 critique, binding rulings, and the complete fleet-doctor revision 3 from the S5 worktree. I checked the cited implementation on current `origin/main`; the only paths changed between the design's `5cd5d313c` fact anchor and current `origin/main` are goal records, not the cited code. No tests, builds, fixture beds, commits, or process-control commands ran. The allowed `bin/metasystem goal show --id human-break-glass-commit-keeps-the-ledger-valid` read could not run because this worktree contains no `bin/metasystem`; the goal record itself was read and still records tier 3 and a three-round review budget (`metasystem/plans/goals/human-break-glass-commit-keeps-the-ledger-valid.md:3-15`).

The code-fact pass confirmed the design's ledger digest and parser facts (`metasystem/internal/goal/file.go:475-488`, `metasystem/internal/goal/file.go:1745-1766`), commit readers and validation order (`metasystem/internal/goal/validate.go:550-566`, `metasystem/internal/goal/validate.go:620-642`, `metasystem/internal/goal/channel.go:137-197`), goal transaction and compare-and-swap boundaries (`metasystem/internal/goal/txn.go:225-295`, `metasystem/internal/goal/txn.go:345-374`, `metasystem/internal/goal/txn.go:469-498`, `metasystem/internal/goal/txn.go:604-777`), human classification and fixture grants (`metasystem/cmd/metasystem/process_verbs.go:319-364`, `metasystem/internal/fixtureauth/fixtureauth.go:48-71`, `metasystem/internal/lease/classify.go:458-472`), hook installation and chaining (`metasystem/cmd/metasystem/goalsync_verbs.go:67-112`, `metasystem/cmd/metasystem/goalsync_verbs.go:132-175`, `metasystem/cmd/metasystem/goalsync_verbs.go:207-263`), guard predicates (`metasystem/scripts/agents/pre-commit-guard.sh:26-112`), receipt clock injection (`metasystem/internal/receipt/receipt.go:27-85`, `metasystem/internal/receipt/receipt.go:217-240`), and materialized-base advancement (`metasystem/internal/goal/reconcile.go:241-271`). The defects below are therefore design-contract defects, not stale citations.

## Revision-2 material findings

| Prior finding | Closed by revision 3? | Evidence in revision 3 |
|---|---|---|
| BGCR2-01, candidate identity was self-referential and changed on confirmation | Yes | BG-TREE-03 keeps the candidate index and BG-ANNOUNCE-06 makes the full payload-tree identifier exclude the generated record and receipt, then adds those bytes only after consent (`metasystem/artifacts/reports/breakglass-design-r3.md:132-190`, `metasystem/artifacts/reports/breakglass-design-r3.md:277-319`). |
| BGCR2-02, hook location and caller result did not describe the installed chain | Yes | BG-GUARD-05 resolves the configured hooks directory, locates the local hook beside the primary hook, removes the invented caller field, and records only the chain's actual output (`metasystem/artifacts/reports/breakglass-design-r3.md:226-275`). The new package-boundary defect in that implementation map is BGCR3-03 below, not a reopening of the requested behavior. |
| BGCR2-03, the ordinary index remained at the parent | Yes | Repair landing now has `AdvanceIndex`, specifies preservation of unrelated staging, and gives a command-level HEAD/index/worktree witness (`metasystem/artifacts/reports/breakglass-design-r3.md:165-190`). |
| BGCR2-04, hook observation could inspect a steered repository or hang | Yes | The observer now has a scrubbed environment, copied index, one timeout, terminal outcomes, capped stderr, and injected fake-execution witnesses (`metasystem/artifacts/reports/breakglass-design-r3.md:226-275`). |
| BGCR2-05, scope cleaning dereferenced the named symlink | Yes | BG-SCOPE-02 resolves only the parent directory and its command witness commits the outward-pointing symlink entry without touching its target (`metasystem/artifacts/reports/breakglass-design-r3.md:104-130`). |
| BGCR2-06, repair never searched for ledger paths missing at HEAD | Yes | BG-REPAIR-08 step 4b searches a bounded first-parent history and covers deleted backlog, deleted referenced-goal, archive-move, and no-ancestor outcomes (`metasystem/artifacts/reports/breakglass-design-r3.md:351-416`). |
| BGCR2-07, no-push, real mutation, required flags, and the R-115 read rule lacked witnesses | Yes | The revision adds the no-remote argv/ref witness, separate `--why` and `-m` witnesses, actual post-restamp and post-repair `goal.Publish` mutations, and the per-unit proof-floor review sentence (`metasystem/artifacts/reports/breakglass-design-r3.md:81-102`, `metasystem/artifacts/reports/breakglass-design-r3.md:176-190`, `metasystem/artifacts/reports/breakglass-design-r3.md:431-472`). |
| BGCR2-08, candidate read failure had no outcome | Yes | `ValidateCommitProblems` returns problems separately from read failure, and BG-06-UNREADABLE-CANDIDATE proves the distinct announcement, record, consent, and landing outcomes (`metasystem/artifacts/reports/breakglass-design-r3.md:277-319`). |

## BGCR3-01: The two pages define different canonical restore-and-publish primitives

Severity: critical  
Material: yes  
Evidence: read

The seat required the break-glass goal to build the shared primitive first and the doctor to consume `RepairBuild`, `Candidate.Add`, and `RepairLand` with `LandCanonical`. This page declares exactly those names, but defines `LandCanonical` as one direct `PublishCAS` and says the doctor consumes it (`metasystem/artifacts/reports/breakglass-design-r3.md:132-174`, `metasystem/artifacts/reports/breakglass-design-r3.md:388-393`). The doctor page explicitly rejects that interface: it adds a separate `PublishRequest.RepairTip` mode, invokes `goal.Publish` directly, and says that if `LandCanonical` remains bare `PublishCAS`, the doctor will not call it (`metasystem/artifacts/reports/doctor-design-r3.md:103-113`, `metasystem/artifacts/reports/doctor-design-r3.md:192-195`, `metasystem/artifacts/reports/doctor-design-r3.md:210-215`).

Those are not interchangeable aliases in the current code. `PublishCAS` performs one local ref update or one remote push and returns only the compare outcome (`metasystem/internal/goal/txn.go:345-374`). `goal.Publish` creates and advances the journal, recaptures and rebuilds on each competing tip through `Mutate`, runs acceptance and sync-mode gates, validates the candidate, classifies unknown publication, verifies the transaction trailer after refetch, and advances the accepted ref (`metasystem/internal/goal/txn.go:469-498`, `metasystem/internal/goal/txn.go:521-568`, `metasystem/internal/goal/txn.go:604-777`). The proposed `RepairLand(..., c *Candidate, mode LandMode, ...) error` has neither a mutation callback for retry nor the result states the doctor requires. The doctor units therefore cannot consume U2/U3 as promised, and U3's bare canonical CAS cannot prove the doctor's DR-9 and DR-10 transaction witnesses.

Required correction: make `RepairLand` with `LandCanonical` the one repair transaction owner, with a rebuild callback based on `RepairBuild`/`Candidate.Add` and explicit transaction outcomes, or change both pages to another single interface that still bears the seat-mandated names. Remove the doctor's direct bypass. Its witness must show the shared entry point retains every `goal.Publish` gate and re-plans after a lost compare.

## BGCR3-02: The shared ledger-write authority contract disagrees on `humanauthority.Prove`

Severity: critical  
Material: yes  
Evidence: read

The break-glass page gates its command with arm's lease classifier, refuses `FixtureGranted`, and passes an internal `Proof{Class, FixtureGranted, By}`; neither the shared primitive nor its record contract requires `humanauthority.Prove` (`metasystem/artifacts/reports/breakglass-design-r3.md:81-102`, `metasystem/artifacts/reports/breakglass-design-r3.md:132-149`, `metasystem/artifacts/reports/breakglass-design-r3.md:321-340`). The doctor page requires a valid `humanauthority.Prove` for every canonical ledger repair and records its grade, outcome, terminal reference, and invoker reference (`metasystem/artifacts/reports/doctor-design-r3.md:103-113`, `metasystem/artifacts/reports/doctor-design-r3.md:115-123`). Current arm proof really does stop at the lease classification and returns the fixture bit (`metasystem/cmd/metasystem/process_verbs.go:331-364`); `humanauthority.Prove` is a different enrolled-terminal proof (`metasystem/internal/humanauthority/authority.go:800-829`).

The page's statement that the two designs agree because both refuse `FixtureGranted` closes only half of the authority seam. An implementer of the break-glass API has nowhere to receive, validate, or record the doctor's enrolled proof, while an implementer of the doctor page must invent a second wrapper around the allegedly shared landing primitive. That changes both the authority boundary and record schema.

Required correction: define one canonical-ledger authority input owned by `LandCanonical` (or its single caller contract): a freshly observed, valid `humanauthority.Proof`, plus an unconditional fixture-grant refusal before any ledger write. Make both pages name the same proof fields and the same witness. The local commit-only `LandRef` path may retain the goal's arm-equivalent proof, but that distinction must be explicit.

## BGCR3-03: U7 cannot call either helper it is specified to reuse

Severity: high  
Material: yes  
Evidence: read

BG-GUARD-05 puts the observer in `internal/breakglass`, tells it to classify the composer with `isOurComposer`, and tells it to run with `environWithoutGitSteering` (`metasystem/artifacts/reports/breakglass-design-r3.md:226-255`). U7a and U7b allocate only `internal/breakglass` files (`metasystem/artifacts/reports/breakglass-design-r3.md:456-457`). On current `origin/main`, `isOurComposer` is an unexported function in the command's `package main` (`metasystem/cmd/metasystem/goalsync_verbs.go:1`, `metasystem/cmd/metasystem/goalsync_verbs.go:233-264`), and `environWithoutGitSteering` is unexported in `internal/goal` (`metasystem/internal/goal/genesis.go:123-145`). Go code in `internal/breakglass` cannot call either.

Copying both implementations would make the unit compile, but it would silently choose a new owner and create a third Git-environment scrubber; BG-05-MIRROR covers the shell predicates, not exact composer recognition or the complete steering-variable set. Moving or exporting the owners changes files, allocations, and tests absent from U7's map. The unit is therefore unbuildable as mapped, or unprovable against the installed-chain owner if the implementer guesses duplication.

Required correction: allocate an earlier shared hook-contract/environment owner, update the installer and observer to consume it, and add a mutation witness that changes that owner once and fails both consumers. Alternatively inject both decisions from the command package, but then U7's production wiring and tests must be reassigned to the command unit.

## BGCR3-04: Two early units claim witnesses for decisions owned only by the later verb

Severity: medium  
Material: yes  
Evidence: read

The page places path expansion and the “names nothing in the worktree or HEAD” question in the verb (`metasystem/artifacts/reports/breakglass-design-r3.md:160-163`), yet U2, which builds only `internal/goal.RepairBuild`, must pass BG-03-NOTHING-NAMED (`metasystem/artifacts/reports/breakglass-design-r3.md:448-450`). `RepairBuild` receives already-resolved `RepairEntry` values and cannot observe which absent path the human named. Likewise, detached-HEAD detection is explicitly the verb's `symbolic-ref HEAD` decision (`metasystem/artifacts/reports/breakglass-design-r3.md:165-169`), but U3, which builds only `RepairLand` and `AdvanceIndex`, claims BG-03-DETACHED (`metasystem/artifacts/reports/breakglass-design-r3.md:450`). `LandRef` already receives a ref and expected oid, so it cannot prove that the command was invoked on a detached HEAD.

Because each unit must land through the selected green groups and its independent read, these named witnesses cannot be green for the stated reason when U2 and U3 land. An implementer must either move command policy into the low-level primitive or defer the witnesses to U8; those alternatives change the named owner and test boundary.

Required correction: keep the decisions at the command boundary and reassign BG-03-NOTHING-NAMED and BG-03-DETACHED to U8, while giving U2 and U3 low-level witnesses for empty-entry handling and explicit-ref compare behavior; or deliberately move the decisions and change the APIs and prose consistently.

## Open-question recommendations

1. Accept the page's answer that none of revision 1's nine questions remains; no new seat ruling is needed.
2. Require the doctor to consume `RepairBuild`, `Candidate.Add`, and `RepairLand(LandCanonical)`; make that canonical mode own the repair-enabled `goal.Publish` transaction, require `humanauthority.Prove`, refuse `FixtureGranted`, and never call `AdvanceIndex` for the remote canonical ref.
3. Leave the cause of 20f6d460e and its `metasystem` author unestablished; keep the commit-tree fixture independent of both.

## Binding-rule conformance not otherwise defective

The seat-approved 507-line single page is not a finding. The revision keeps the goal-record budget, names the verb `metasystem break-glass`, makes the human verb commit locally without push, leaves the hook in observe mode except for the already-ruled hint text, introduces no deny mode or terminal-multiplexer dependency, uses injected clocks and fake execution in tests, says red tests are fixed rather than retried, and declares every unit at no more than 300 changed lines (`metasystem/artifacts/reports/breakglass-design-r3.md:3-13`, `metasystem/artifacts/reports/breakglass-design-r3.md:65-79`, `metasystem/artifacts/reports/breakglass-design-r3.md:444-472`). R-115-m1e's proof-floor read, R-117-m1e item 5's landing lane, R-118-m1e's observe-mode limit, and R-119-m1e's continuing gates are all carried. Findings BGCR3-01 through BGCR3-04 are the remaining reasons units cannot be built or proved as written.

## Rigor classification

| findingId | rigorClass | facts | reopeningTrigger |
|---|---|---|---|
| BGCR3-01 | severe | `{"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":true}` | Reopen when both pages name and test one canonical `RepairBuild`/`Candidate.Add`/`RepairLand(LandCanonical)` path that retains the complete repair transaction, retry, outcome, and accepted-ref contract, with no direct doctor bypass. |
| BGCR3-02 | severe | `{"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":true,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":true}` | Reopen when the canonical shared path requires and records one freshly observed valid `humanauthority.Proof`, refuses fixture-granted HUMAN before every ledger write, and both pages name the same witness. |
| BGCR3-03 | unproven | `{"local":true,"recoverable":true,"proofBoundaryCrossed":false,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}` | Reopen when U7 has an accessible single owner for composer recognition and Git-environment scrubbing, with both installer and observer proved against that owner and the unit's files and allocation updated. |
| BGCR3-04 | severe | `{"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}` | Reopen when each pre-verb unit's named witnesses are runnable at that unit's landing boundary, with the two command-only questions either reassigned to U8 or deliberately moved into the primitive APIs. |

Proposed receipt line: `<epoch>|<utc>|RECEIPT|type=other|outcome=shipped|skills=design-critique|verify=read-only|corrections=0|stop_loss=no|delegate=none|goal=human-break-glass-commit-keeps-the-ledger-valid|note=revision 3 design critique: four material build or proof defects`

VERDICT: rework

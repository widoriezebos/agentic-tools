# g1-s10 Backlog data path: Sol's design review

Reviewer: Codex on `gpt-5.6-sol`, the design reviewer under D12 and D24, read-only, 2026-09-21. Subject: the [slice design](g1-s10-backlog-data-path-design.md) at commit `f98b71003`.

**Revision 2 and the planner's second ruling, 2026-09-21.** All six are answered. The revision takes two additive exports from `internal/goal`, not the one the planner admitted: `ReadValidatedTree` as well as `NewApprovalHorizon`. That is ratified. The single-export alternative would have the interface compose the at-rest validation sequence itself and carry its own copy of the Git steering-variable list, making it a second owner of both — which is precisely the divergence M6 exists to prevent, so admitting one export and refusing the other would have applied the principle to half the problem. Both exports are additive: no existing function changes, and an obligation holds the package's diff to two new files.

Six material findings. The planner's ruling on M6: the slice may add an exported constructor to `internal/goal`, so that the approval horizon keeps one owner. That is a narrow additive change and it is preferable to a mirrored copy that can silently diverge from what claim admission uses. `internal/goal/**` is therefore not absolutely no-touch for this slice; every other package on the list stays.

## 1. Verdict build after the listed changes Riskiest: the snapshot can present a parseable b...
[codex] Turn completion inferred after the main thread finished and subagent work drained.
## 1. Verdict

build after the listed changes

Riskiest: the snapshot can present a parseable but semantically invalid accepted tree as trusted backlog data.

## 2. Findings

M1; MATERIAL; `internal/ui/snapshot` and failure handling; `goal.Project(e, false, now)` parses the tree but does not run the full at-rest validator, contrary to the design’s claim that any parse or validation problem refuses the whole tree; evidence: `metasystem/internal/goal/project.go:76-125`, `metasystem/internal/goal/verbs.go:333-363`, `metasystem/internal/goal/validate.go:198-500`, `metasystem/internal/goal/validate.go:669-691`; a parseable accepted ref with duplicate ranks, missing dependencies, cycles, or another tree-level violation would answer `read` and show records as trustworthy; fix direction: add an owner-provided read that returns one fully validated tree per tip, preserving typed problems without reading it twice.

M2; MATERIAL; `internal/backlog`, Ready; the proposed `LaneOf` calls a goal Ready after approval, expiry, and blockers only, but the engine’s final claim-admission gate also evaluates the tier-box configuration and norm coverage and can classify an otherwise eligible goal as `Refused`; evidence: the master defines Ready as “currently eligible to be claimed” at `plans/designs/user-interface-design.md:521-531`, while admission is implemented at `metasystem/internal/goal/approval.go:290-378` and consumed by `Next` at `metasystem/internal/goal/project.go:649-723`; the human could see Ready for work that every claim attempt would refuse; fix direction: obtain the owner’s per-goal admission verdict and make it an input to the shared lane projection rather than duplicating or omitting that decision.

M3; MATERIAL; Draft gap and working-tree counts; counting `plans/goals-drafts/*.md` with `ReadDir` is a read of the drafts directory, despite M6 and Gate 1 saying nothing reads it; evidence: `plans/user-interface-implementation-plan.md:143-144` and `plans/designs/user-interface-design.md:805-810`; the interface would claim “Draft not read” while reporting knowledge obtained by reading that source; fix direction: remove `DraftFiles` and the count until the drafts owner exists, or obtain a master amendment explicitly permitting an existence-only read.

M4; MATERIAL; snapshot caching; the design caches the sync-mode verdict with the tip by skipping both `ResolveEndpoint` and `Project` on a cache hit, even though the Git configuration can change independently; evidence: endpoint configuration is read at `metasystem/internal/goal/txn.go:58-72`, the sync-mode gate runs at `metasystem/internal/goal/project.go:94-110`, and the accepted plan permits caching only immutable tree data at `plans/user-interface-implementation-plan.md:184`; after a configuration flip, Refresh could continue showing `read` until the accepted tip moves; fix direction: cache only the parsed validated tree and re-evaluate endpoint/configuration-dependent gates on every observation.

M5; MATERIAL; `AcceptedAt`; the proposed direct `exec.Command("git", "-C", ...)` inherits the interface server’s environment, whereas the engine explicitly strips Git steering variables because `-C` does not defeat them; evidence: the server child inherits its environment at `metasystem/internal/ui/lifecycle/launch.go:95-102`, while the engine’s protected execution explains and implements the scrub at `metasystem/internal/goal/genesis.go:89-140` and `metasystem/internal/goal/genesis.go:163-185`; `acceptedAt` could be read from another repository or become spuriously unknown; fix direction: expose commit time through the Git/goal owner or use the same scrubbed execution contract with a poisoned-environment test.

M6; MATERIAL; approval horizon; the four-line mirror matches today, but it creates a second owner for the observation used to decide approval expiry; evidence: the existing owner is `approvalHorizon` at `metasystem/internal/goal/approval.go:253-259`, and `ApprovalHorizon` is documented as the complete observation at `metasystem/internal/goal/file.go:256-262`; a later change to the engine’s horizon construction could make browser and claim admission disagree; fix direction: export the owner’s horizon constructor and compose it here, which requires removing `internal/goal/**` from this slice’s absolute no-touch list.

M7; MATERIAL; absent and no-ledger panes; the snapshot proves only that the ref is absent or that the named commit lacks `backlog.md`; it cannot prove “because no goal verb has run,” that the step is one-time, or that a no-ledger tip predates the ledger, and the fixed `from metasystem/: bin/metasystem ... --root .` instruction is self-hosted-layout-specific; evidence: `AcceptedLedgerTip` only performs those probes at `metasystem/internal/goal/attention.go:541-555` and `metasystem/internal/goal/txn.go:158-203`, while served roots vary by layout at `metasystem/internal/ui/lifecycle/roots.go:25-68`; a restored, manually removed, or wrongly pointed ref—and an adopted project—would receive false causal or operational guidance; fix direction: state only the observed condition and derive any exact recovery command from the served checkout and installation.

M8; MATERIAL; parked-from; `Displaced` proves only that a foreign claim was displaced, not every transition from claimed, while an own-pair park retains an `Episode` that also proves a prior claim; evidence: `ParkRecord.Displaced` is defined at `metasystem/internal/goal/file.go:405-415`, own-pair episode retention is at `metasystem/internal/goal/verbs.go:437-456`, and ordinary parking sets `Displaced` only for a foreign pair at `metasystem/internal/goal/verbs.go:2370-2400`; an own-pair parked goal would falsely say its source was not recorded; fix direction: derive the prior claimed state from `Displaced` or `Episode`, while keeping execution phase separately “not recorded.”

M9; MATERIAL; Waiting precedence; “Waiting first” conflicts with the table’s expired-approval-before-blocker ordering, and the verification matrix has no combined expired-plus-open-blocker case; evidence: `plans/designs/user-interface/g1-s10-backlog-data-path-design.md:60-71`, the master’s qualified precedence at `plans/designs/user-interface-design.md:521-531`, and `Next` evaluating expiry before blockers at `metasystem/internal/goal/project.go:688-720`; a builder must guess whether that record belongs in To Do or Waiting; fix direction: decide the combined case explicitly and add it to the `LaneOf` table test.

M10; NOT MATERIAL; Scale; the under-500-KB estimate and cited field-bound premise do not survive—a read-only calculation found roughly 1.2 MB in Intent, Next step, and Concluded text alone, and the bounds at `metasystem/internal/goal/goal.go:26-39` apply to the legacy `Goal`, not `GoalFile`, whose parser is at `metasystem/internal/goal/file.go:528-818`; this does not yet justify pagination at 585 loopback rows, but the walkthrough must measure the real payload, initial paint, and closed-toggle paint.

M11; NOT MATERIAL; Unknown lane; an unknown `GoalFile.State` cannot reach this read path today because parsing rejects anything outside the closed six-state set at `metasystem/internal/goal/file.go:520-525` and `metasystem/internal/goal/file.go:590-595`; keeping a defensive fallback is harmless and satisfies the master’s future-facing unknown-state requirement.

## 3. The absent-ledger pane

Naming the fetch command is the honest choice, not a breach of M9. Backlog is projected in this slice; the accepted ref is a missing prerequisite for that live view, not a section whose projection has not been built. M9’s no-action rule applies to the latter at `plans/designs/user-interface-design.md:433-438`, while unavailable capabilities should name a concrete path at `plans/designs/user-interface-design.md:447-449`.

It remains an uncovered workflow, exactly as the master says at `plans/designs/user-interface-design.md:725-735`. Present it as transitional recovery guidance, not as a completed interface action. Remove the unproved causal wording and make the command layout-correct. Fetch must still run only on explicit human instruction because it fetches and advances refs; `plans/user-interface-implementation-plan.md:7-14`.

## 4. The design's own claims

The following did not survive:

- “A tree with any problem refuses whole”: `Project` performs parsing, not `ValidateTree`; `metasystem/internal/goal/project.go:83-92`, `metasystem/internal/goal/verbs.go:353-362`, `metasystem/internal/goal/validate.go:669-691`.
- “Ready means eligible”: claim admission can still refuse it; `metasystem/internal/goal/approval.go:360-378`, `metasystem/internal/goal/project.go:701-717`.
- “Waiting first”: the expired-plus-blocked case is contradictory and unspecified; `plans/designs/user-interface/g1-s10-backlog-data-path-design.md:60-71`.
- “Draft not read”: its directory would be read for the count; `plans/designs/user-interface/g1-s10-backlog-data-path-design.md:91-95`.
- “Every field is bounded” and “under 500 KB”: the cited bounds govern the legacy type, not `GoalFile`; `metasystem/internal/goal/goal.go:26-39`, `metasystem/internal/goal/file.go:22-91`.
- “Any other live state goes to Unknown”: the current parser refuses it before projection; `metasystem/internal/goal/file.go:590-595`.
- The absent/no-ledger causal explanations: the probes establish no such history; `metasystem/internal/goal/attention.go:541-555`.

These challenged claims did survive:

- `Project(e, false, now)` does not fetch or advance refs; `true` enters `FetchAdvance`, which can move the accepted ref; `metasystem/internal/goal/project.go:69-92`, `metasystem/internal/goal/fetchadvance.go:26-81`.
- The remaining proposed GET path consists of local reads; the HTTP server admits only GET and HEAD at `metasystem/internal/ui/httpd/httpd.go:104-135`.
- The approval-horizon mirror is behaviorally exact today; the finding is about duplicated ownership, not current output.
- `internal/backlog` is the correct package tier for a future CLI/browser shared projection. Today the CLI independently groups raw states and builds markers at `metasystem/cmd/metasystem/goal.go:394-443` and `metasystem/cmd/metasystem/goal_list.go:142-164`; the exported `LaneOf` shape becomes sound once it receives owner-computed admission.
- The checkout inventory recorded at `plans/designs/user-interface/g1-s10-backlog-data-path-design.md:37` survived independent read-only Git checks, including 155 goals rather than 156.

## 5. The three open questions

1. Closed-items order is the designer’s. Decide ID order now. It matches the current deterministic CLI reads at `metasystem/cmd/metasystem/goal.go:416-425` and avoids inventing another ordering rule.

2. Expired-approval placement is the designer’s. Decide To Do, matching the engine’s `Awaiting` classification, and explicitly say that expiry is evaluated before blockers; `metasystem/internal/goal/project.go:688-720`. Add the expired-plus-blocked fixture.

3. Who performs the fetch is genuinely the human’s because it is an outward-facing ref-moving operation. Recommendation: the human runs it after checking the endpoint; the planner may do so only following an explicit instruction for that execution.

## 6. Reconciliation with the concurrent Project slice

Before either build starts, settle:

- The canonical goal URL. The router is prepared for nested Backlog paths at `metasystem/internal/ui/web/_app/src/routes.ts:51-77`, and its test already uses `/backlog/goals/g1` as an example at `metasystem/internal/ui/web/_app/src/routes.test.ts:56`; this slice must not independently freeze all `/backlog/*` paths as permanently not found while the Project slice chooses the goal route.
- When `routeFor({kind:"goal"})` becomes non-null and when rows become links. Both slices must use the same kind, ID, revision, and route mapping.
- One accepted-tip snapshot owner. Project detail must consume the same validated cached tree, observation-time horizon, accepted time, and ledger-state taxonomy rather than creating another holder or another Git call.
- The API split: whether detail reads from `/api/backlog`, a goal-detail route, or both, and how a requested revision that differs from the current row is represented.
- The call-site allowlist and landing order. This slice correctly changes the current one-site guard at `metasystem/internal/ui/web/_app/src/cuts.test.ts:313-326` to two sites for its cut, but the Project slice must amend that same exact allowlist rather than independently replacing it.
- Ownership of overlapping edits to `Shell.tsx`, `routes.ts`, route tests, `cuts.test.ts`, and the accepted-tip wiring. One slice must land first or the two designs need one combined integration contract.

## 7. Not checked

- The concurrent Project design was not read.
- No files were created, changed, or deleted.
- `metasystem`, goal verbs, repository scripts, npm, and npx were not run.
- `metasystem/metasystem.conf.local` was not read.
- The running server was not changed or exercised.
- No proposed implementation exists, so exact serialized response size, browser paint time, closed-toggle latency, and narrow-screen behavior were not executed.

---

# The freshness loop, terminal review, 2026-09-21

On revision 3, scoped to the loop and the rewritten ledger states. Verdict: safe to build with three obligations, all on the builder rather than the design. The no-ref invariant holds and its proof is adequate, all seven ledger situations match source, and `CaptureTip` really does keep the loop off `refs/remotes/*`.

The planner extends its earlier ruling: if a bounded advance requires another additive export from `internal/goal`, take it, on the same reasoning as `ReadValidatedTree`. A timeout the interface implements itself would be a second owner of the transport's lifetime, which is the divergence these rulings exist to prevent.

## Verdict safe to build with the listed obligations ## The six items 1. confirmed; the propo...
[codex] Turn completion inferred after the main thread finished and subagent work drained.
## Verdict

safe to build with the listed obligations

## The six items

1. confirmed; the proposed request call graph is read-only and its proof is adequate.
2. cancellation is incomplete: non-overlap, wake coalescing, and deadline arithmetic hold, but a ready timer may win over cancellation and start one final tick.
3. not acceptable unbounded; the builder must use a bounded fetch before shipment.
4. confirmed; all seven ledger situations and the remaining terminal instruction match source.
5. lifecycle ownership is wrong: CAS prevents partial accepted-ref updates, but the loop can start before server exclusivity and outlive a failed listener.
6. confirmed; `CaptureTip` passes `--refmap=` with only its per-operation refspec (`metasystem/internal/goal/txn.go:115-150`).

## The no-ref invariant

Inference from the specified implementation: it holds. `ServeHTTP` reaches the backlog observation only (`plans/designs/user-interface/g1-s10-backlog-data-path-design.md:187-203`); `Observe` changes cadence and sends a wake but neither calls nor waits for fetch (`plans/designs/user-interface/g1-s10-backlog-data-path-design.md:162-171`). The mutating entry points are visibly separate: `Project(true)` fetches (`metasystem/internal/goal/project.go:69-82`), `FetchAdvance` validates then advances (`metasystem/internal/goal/fetchadvance.go:30-81`), and `RepairAcceptRemote` is explicitly mutating (`metasystem/internal/goal/accepted.go:16-82`).

The grep rule, fake-fetch counter, and byte-identical `for-each-ref` test, combined with the route test asserting one `Observe`, adequately prove the boundary (`plans/designs/user-interface/g1-s10-backlog-data-path-design.md:272-274,282`). Code review must reject any additional request-reachable goal call not covered by that allowlist.

## Findings

L1; MATERIAL; the loop starts before `lifecycle.Serve`, but exclusivity is acquired inside `Serve`; therefore a second server attempt can perform its immediate tick before being refused, and if the listener fails, `Serve` returns without cancelling the signal context, so waiting for the loop hangs; evidence: design wiring at `plans/designs/user-interface/g1-s10-backlog-data-path-design.md:177,258`, lock acquisition at `metasystem/internal/ui/lifecycle/serve.go:100-120`, listener return at `metasystem/internal/ui/lifecycle/serve.go:168-185`, signal-context lifetime at `metasystem/cmd/metasystem/ui.go:122-155`; builder obligation: start exactly once only after lifecycle ownership is established, cancel a derived loop context on every `Serve` return, and wait; code review checks that `AlreadyRunning` performs zero fetches and listener failure terminates the loop.

L2; MATERIAL; bare `FetchAdvance` can hang forever in `CaptureTip`, making the loop remain `running` and preventing graceful shutdown; evidence: `FetchAdvance` calls `CaptureTip` directly (`metasystem/internal/goal/fetchadvance.go:30-39`), whose fetch uses an unbounded command run (`metasystem/internal/goal/txn.go:81-112,119-150`); builder obligation: use an engine-owned bounded advance that kills the transport process group, cleans its temporary ref, reports `failed`, and enters backoff. Existing bounded machinery demonstrates the required behavior (`metasystem/internal/goal/attention.go:623-719`).

L3; MATERIAL; inference: the specified `select` gives cancellation no priority, so simultaneously ready timer and cancellation channels may select the timer and start a fetch after cancellation; evidence: `plans/designs/user-interface/g1-s10-backlog-data-path-design.md:181-185`; builder obligation: check cancellation immediately before every tick and add a deterministic cancelled-context-plus-ready-timer test asserting zero fetch calls.

The ledger claims hold. Genuine first fetch skips `AcceptanceGates` because the accepted ref does not resolve (`metasystem/internal/goal/fetchadvance.go:48-62`), so foreign-ledger and rewind refusals cannot appear. Missing `backlog.md` is rejected by parsing and validation before creation (`metasystem/internal/goal/validate.go:93-152,672-691`), so `no-ledger` cannot be caused by this loop. Repair also refuses an accepted tip without readable identity (`metasystem/internal/goal/accepted.go:36-56`); deleting that externally created ref is therefore the correct remaining terminal recovery.

## Not checked

No implementation exists. I ran no server, tests, scripts, npm, goal verb, or fetch; I did not read `metasystem.conf.local` or re-review revision 2’s findings. No files were changed.

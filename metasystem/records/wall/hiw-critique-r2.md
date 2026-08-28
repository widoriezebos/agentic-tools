R2 has not converged. It faithfully carries both D100 rulings—no self-work exception and defect-detector tier—but only partially folds four of the nine Round 1 findings. The design is not yet implementation-ready: an implementer would still have to invent safety-critical authorization, composition, recovery, and taint semantics.

Evidence level: checked by reading the design, current schemas, prompts, runner paths, conformance code, and benchmark extractor. Proposed contracts below are reasoned recommendations. No files were modified and no tests were run.

## Fold fidelity

| Round 1 finding | R2 status | Assessment |
|---|---|---|
| HIW-R1-01, conformance-issued authorization | Partial | Durable/content-addressed issuance is stated, but issue-time patch↔base↔reviewed-tree joining and adjudicated consumption are absent. |
| HIW-R1-02, exact byte equation | Carried | Exact ordered patches and fresh conformance/park are present, though the executable composition protocol remains undefined. |
| HIW-R1-03, complete trustworthy snapshot | Partial | The isolated-index primitive and recovery-before-new-snapshot rule are carried. Gitlinks, pre-existing dirty state, object reachability, and recovery ordering remain unspecified. |
| HIW-R1-04, exact default-deny paths | Partial | Blanket `plans/**` is correctly rejected, but “principally under `artifacts/agents/**`” is a category, not an exact enumeration; host output files lack a declaration contract. |
| HIW-R1-05, durable taint and mission park | Carried | The required outcome and gate precedence are present, but not the state machine that makes them durable. |
| HIW-R1-06, self-work ruling | Faithful | The residual and all exception machinery are deleted, matching D100. |
| HIW-R1-07, hardened ownership split | Carried | Dispatch, conformance, missionrunner, gate, and floor responsibilities are assigned correctly. |
| HIW-R1-08, mission-only boundary | Faithful | Runner-created missions are covered; interactive direct implementation remains outside the wall. |
| HIW-R1-09, hardened delegation floor | Partial | R2 says “adjudicated facts,” but does not require a nonempty authorized patch that was actually consumed into the accepted tree. |

The D100 rulings are faithfully represented in [the r2 invariant](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/host-implementer-wall-design.md:29>) and [the recorded decision](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/reviews/2026-08-13-delegated-decisions.md:901>). The known stale “Ruling 2 below…” sentence is editorial and is not counted as a finding.

## Material findings

### HIW-R2-01 — STRUCTURAL: The authorization is not a replay-safe, consumable fact

The proposed binding set—job, mission, stream, role, patch digest, reviewed tree, changed paths, and base—is insufficient.

It must additionally bind:

- The immutable job-record digest.
- Mission incarnation or signed contract/genesis digest, not merely a possibly reused mission identifier.
- Dispatch turn and authorization-issuance turn.
- An unambiguous input/base tree and output/reviewed tree.
- Schema version and canonical authorization digest.
- Supersession status for earlier rounds in the same chain.

Replay prevention cannot simply require “authorization turn equals consuming turn.” Delayed certification of landed returns is an existing legitimate behavior, as [landed.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/mission/landed.go:15>) documents. The runner instead needs a durable, one-time consumption ledger such as `authorizationDigest → consumedByTurnId`. An earlier unconsumed authorization may remain usable if it is current, unsuperseded, and still applies; a consumed or superseded authorization must be rejected.

Issue-time conformance must atomically prove:

`apply(exact bound patch, exact bound base) = reviewed tree`

and derive the patch digest and changed paths from those same bytes. Current conformance builds `diff.patch` and `review.json`, but merge success does not consume either record before printing acceptance; [conformance.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/validate/conformance.go:349>) therefore is not yet the required authorization join.

Dispatch provenance also needs real plumbing: current job records have no structured stream, and mission/turn are not among the immutable record fields in [record.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/record.go:53>). Returned certification is still copied raw into the turn record rather than adjudicated in [turnio.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/turnio.go:118>).

### HIW-R2-02 — STRUCTURAL: “Ordered exact patches” still lacks a total tree-composition contract

The isolated-index primitive is appropriate: `read-tree`, `add -A`, and `write-tree` capture deletions, regular-file executable bits, symlink target blobs, binary content, and superproject gitlink object identifiers. They do not capture arbitrary POSIX metadata, ignored untracked files, or dirty/untracked content inside a submodule. Those boundaries must be stated and tested; “mode” should mean Git mode, with repository configuration such as `core.fileMode` pinned.

The design must also specify:

- Where the authoritative patch order is durably recorded before integration or recovery.
- That only the final authorized round of a delegate chain is eligible, or how cumulative rounds are superseded.
- Whether overlap means any common changed path or overlapping hunks. The conservative rule is to reject changed-path intersection unless a new combined authorization is issued.
- Exact apply semantics: no three-way merge, rejects, fuzz, or host-created conflict resolution.
- A failed/non-clean application requires a new authorization against the current expected tree or mission park.
- The initial-baseline rule: require a clean shippable tree or a human-sealed initial pre-tree. Otherwise pre-existing dirty product bytes are silently grandfathered.
- Durable reachability of `write-tree` objects. Recording only a tree object identifier does not prevent garbage collection; recovery needs a runner-owned ref/commit or equivalent durable object store.

Without a durable consumption order, crash recovery cannot reconstruct the expected tree and must either guess patch order or trust a later host claim.

### HIW-R2-03 — STRUCTURAL: The machine-owned metadata term is incoherent inside a shippable-tree equation

As written, the term should not exist as a category-level exemption.

In this repository, `artifacts/` is ignored by [.gitignore](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/.gitignore:1>), and Git administrative state is already outside the shippable Git tree. Their contribution to the equation is therefore empty. If an adopting repository tracks runner metadata, those bytes are shippable and must be represented as exact deterministic tree deltas—not exempted because their paths look machine-owned.

“Mission-declared host output files” are a different class: they are legitimate host-authored, shippable design/control artifacts, not machine metadata. R2 mentions them in prose but omits them from the equation. The complete contract should either:

- Keep runner/admin/ignored state outside the projection and add a distinct exact host-artifact delta; or
- Represent every tracked runner transform and host artifact as separately authorized exact bytes.

The declaration grammar must use canonical repository-relative files, default deny, no directory/glob grants, no symlink traversal, and protected-contract precedence. “[P]rincipally under `artifacts/agents/**`” in [the design](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/host-implementer-wall-design.md:80>) is not an implementable exact list.

### HIW-R2-04 — STRUCTURAL: Taint is an outcome, not yet a state machine

R2 leaves the writer, transitions, crash order, and typed clearance contract undefined.

The hash-chained mission state should carry a runner-owned `openTurn` and monotonic `workspaceTaint`. The lease-holding missionrunner should be the only ordinary protocol writer; Wido’s resolution should use a separate human-reserved operation. The current public `mission state-write` accepts any proposed state plus the current hash in [mission.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/mission.go:71>), so “the host cannot forge” overstates the chosen detector tier. Under D100 it should say “runner-owned and tamper-evident under the cooperative posture.” Genuine unforgeability would require the rejected isolation/security tier.

Required ordering is:

1. Snapshot and durably anchor `openTurn.preTree` before host launch.
2. After every host exit—normal, capped, nonzero, malformed return, or recovery—inspect the tree before measurement or conclusion.
3. On mismatch, persist evidence, taint, and mission park before any gate-success path.
4. On resume, verify state/anchor; if tainted, stop. Otherwise inspect any unfinished open turn before reserved-cycle healing or a new baseline.

That ordering matters because current recovery heals a reserved cycle using current `HEAD` without inspecting the unfinished workspace in [loop.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/loop.go:424>), and current fault conclusion can complete when the gate passes in [cycle.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/cycle.go:201>).

Typed resolution needs two explicit variants:

- `RESTORE`: name a recorded safe tree; the runner verifies exact equality before clearing.
- `ADOPT_DISPUTED_TREE`: bind the taint identifier and observed tree, record Wido’s identity/reason and the exact attribution claims waived.

Adoption clears the operational taint but must not fabricate authorization, erase the violation, or earn delegation-floor credit. Generic free-text `answer` must never clear it.

### HIW-R2-05 — STRUCTURAL: The prompt change does not identify all live authorities or the interim rule

No legitimate mission product activity depends on the exception. Host duties already have separate authority: design/brief work, adjudication, decisive verification, integration of exact authorized patches, receipts, and certification. Tiny mission fixes can use the ordinary implementer → code critic → conformance authorization → exact integration path until small-change-lane ships.

The allowance currently appears in both:

- [host-turn-instruction.md](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/templates/host-turn-instruction.md:8>)
- [orchestrator.md](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/roles/orchestrator.md:94>)

Both are assembled into every mission prompt by [prompt.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/mission/prompt.go:439>), while the role bytes are validated exactly by [turnprompt.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/validate/turnprompt.go:140>). Canonical [orchestration.md](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/orchestration.md:25>) also carries the allowance and generic “When Not to Delegate” language.

The interim replacement should state:

> Inside a runner-created mission, the host never authors product bytes, regardless of size or urgency. A mechanically small change may omit a separate design artifact only when the existing contract permits it; implementation still requires an implementer job, critic closure, conformance-issued integration authorization, and exact authorized-patch integration. Until small-change-lane ships, use that ordinary path. A fence refusal parks through the runner; it never authorizes host implementation. Interactive work outside missionrunner is unaffected.

The broad “Repository work … is yours” opening in the orchestrator role should also be narrowed to enumerate the legitimate host duties.

All four benchmark manifests need the discipline sentence in `completionGate.command`, because [provision.sh](</Users/wido/LocalStorage/GitHub/agentic-tools/benchmark/provision.sh:438>) copies only that field into the mission contract. Updating rationale or an unwired field would be dead text.

### HIW-R2-06 — STRUCTURAL: The delegation floor still permits sham evidence

“Consumes runner-adjudicated certification facts” is not enough. A qualifying stream must have at least one:

- Validated implementer-role job.
- Nonempty conformant patch.
- Unsuperseded integration authorization.
- Authorization actually consumed into the accepted post-tree.

Rejected, empty, replayed, superseded, unapplied, or human-adopted evidence must not count. The current extractor only joins a completed job against raw dispatched/certified identifiers in [extractor.py](</Users/wido/LocalStorage/GitHub/agentic-tools/benchmark/extractor.py:744>), so R1-09 is not fully folded until that outcome rule is explicit.

### HIW-R2-07 — MECHANICAL-GRAIN: Required implementation artifacts remain unnamed

Once the structural contracts above are settled, the design still needs exact schemas and locations for:

- Integration authorization, canonical digest, version, storage, mirroring, and chain-close retention.
- Structured immutable dispatch stream and job-record identity.
- `certified` authorization reference and runner-expanded adjudicated fact.
- Turn wall evidence: pre-tree, expected tree, post-tree, authorization order, and verdict.
- Mission-state `openTurn`, taint, and typed-resolution records.
- Exact host-artifact declarations and protected-path table.
- Durable refs or equivalent storage for recovery trees and cleanup.
- Events such as authorization issued/consumed/refused, wall passed/violated, recovery inspected, and taint resolved.
- Migration or explicit refusal behavior for in-flight legacy missions.
- One shared isolated-index/tree-composition owner rather than separate validator and runner implementations.

Events should remain observability; state and authorization records are the authority.

### HIW-R2-08 — MECHANICAL-GRAIN: The goal handoff still contradicts D100

`goal next` currently tells the next session to design exemptions for `plans/`, `artifacts/`, and “too-small-to-delegate declarations” in [goals.md](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/goals.md:3>). Because `goal next` is the universal handoff, this should be reconciled through the goal verbs when the design is amended. This is distinct from the known stale “Ruling 2” sentence.

## Required design-obligation rows

The full matrix is mandatory because this design introduces an invariant, owner split, lifecycle, state transitions, and failure paths. The gate requires every critical/high row to name an owner, code target, and focused test target before implementation ([design-obligation-gate.md](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/design/design-obligation-gate.md:19>)).

| ID | Severity | Required behavior | Owner/code target | Focused proof target |
|---|---|---|---|---|
| HIW-O1 | CRITICAL | Persist and anchor reachable pre-tree/open-turn state before host launch; inspect it before recovery healing or rebasing. | Missionrunner + mission state | Crash and object-GC fixtures at every snapshot boundary. |
| HIW-O2 | CRITICAL | Issue authorization only after exact patch/base/reviewed-tree and critic-closure join. | Conformance + dispatch evidence | Mutated/stale patch, wrong base, empty patch, and critic mismatch fixtures. |
| HIW-O3 | CRITICAL | Verify immutable mission/lineage/stream/role/job provenance and consume authorization once while allowing legitimate delayed landed returns. | Dispatch + adjudication | Cross-mission, cross-stream, duplicate, replay, superseded, and later-turn fixtures. |
| HIW-O4 | CRITICAL | Deterministically compose exact patches and compare the full Git tree. | Shared Git-tree primitive + missionrunner | Deletion, executable mode, symlink, binary, gitlink, order, overlap, and nonapplication fixtures. |
| HIW-O5 | CRITICAL | Every host-exit path checks the wall; violation taints and parks before measurement or completion. | Missionrunner lifecycle/state | Green completion gate after host mutation must still park. |
| HIW-O6 | CRITICAL | No new baseline while tainted; only typed restore/adopt can clear it, without manufacturing floor credit. | Mission resolution path + state | Generic answer refusal, restore mismatch, exact adoption, and crash-recovery fixtures. |
| HIW-O7 | CRITICAL | Tree partition is equation-complete and default-deny. | Mission contract parser + wall | Protected path, path escape, symlink ancestry, directory/glob grant, and tracked metadata fixtures. |
| HIW-O8 | HIGH | Mission prompts remove every self-work license while interactive direct work remains unaffected. | Prompt assembler, role/template, orchestration doctrine | Assembled prompt byte test plus interactive boundary test. |
| HIW-O9 | HIGH | Delegation floor counts only nonempty, actually consumed adjudicated authorization per stream. | Benchmark extractor/evidence schema | Empty, sham, replayed, unapplied, and adopted-tree fixtures. |
| HIW-O10 | HIGH | Authorization, wall, recovery, and taint evidence is durable, mirrored, and observable. | Dispatch mirror/close + event registry | Restart/readback, missing-record close failure, and event-payload fixtures. |
| HIW-O11 | HIGH | Legacy mission state is migrated safely or refused explicitly. | Mission state loader | Old-schema resume fixture. |

Proposed review-only receipt:

`scripts/receipt.sh add --type review --outcome reworked --skills design-critique --verify skipped --corrections 0 --stop-loss no --note "Host-implementer-wall round 2: partial fold; structural authorization, tree-composition, taint-recovery, prompt, and delegation-floor findings remain."`

REVISE — structural findings remain

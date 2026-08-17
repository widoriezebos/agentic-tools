D87’s “no new coverage” claim is false. The park remains directionally sound, but its named dependency and the r2 fold both require structural revision.

### Finding 1 — STRUCTURAL: WatchdogReport does not implement the promised liveness verdict

The watchdog overlaps with the proposed steward check, but it is not equivalent.

| r2 promise | Current watchdog behavior |
|---|---|
| Typed `ok / unknown / breach` | Returns human-readable lines or silence. Missing/unreadable evidence is presented as `SUPERVISION DOWN`, not `unknown`. |
| “A session is active” precondition | `WatchdogReport(repo, now)` has no session input or activity predicate. A Stop call merely implies a turn reached that hook; advisor Stops bypass the watchdog entirely. |
| Canonical armed-and-fresh predicate | Uses a different predicate: census freshness is up to `min(2× interval, 180s)`, while `ArmedNow` allows one interval. |
| Complete armed-state verification | Does not read component heartbeats, verify generation, validate the watcher cap, require both watcher and reaper, or check instance tags. `ArmedNow` does all of these. |
| Reliable end-of-turn surfacing | A blocking turn discards watchdog extras after the digest has already been recorded as surfaced. |
| No-hook or between-turn coverage | No automatic caller exists beyond Stop. The report itself is not persisted. |

The differences are visible directly in [watchdog.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/supervise/watchdog.go:27), compared with [verifyarmed.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/supervise/verifyarmed.go:36). The Stop path calls the watchdog at [supervision-hook.sh](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/supervision-hook.sh:119), but suppresses its text on blocking outcomes at [supervision-hook.sh](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/supervision-hook.sh:148).

Therefore, a supervise-owned typed verdict would add real predicate and outcome coverage. A steward could also add durable no-hook visibility once scanned. Between-turn detection would require the still-unspecified cadence/freshness owner; it is potential coverage, not something the current steward text actually guarantees.

D87 sharpened r2’s defensible conclusion—“largely duplicates the existing warning”—into the unsupported claim “adds no new coverage” at [D87](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/reviews/2026-08-13-delegated-decisions.md:2520).

### Finding 2 — STRUCTURAL: The park is coherent, but its recorded unblock condition is not

Waiting for owner-produced typed evidence remains sound. Building a scanner that reinterprets raw records would create a competing policy owner.

D89 invalidates the specific sequencing judgment that the janitor orphan verdict is the cheapest prerequisite. Sound process-orphan detection needs process-group identity and death proof; custody-list death is insufficient, as recorded at [D89](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/reviews/2026-08-13-delegated-decisions.md:2594).

The records now disagree:

- The design’s top-level unblock condition is generic: any sound typed verdict for a currently unwatched invariant ([process-steward-design.md](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/process-steward-design.md:9)).
- The same design and the goal ledger still hard-bind resumption to the janitor orphan verdict ([process-steward-design.md](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/process-steward-design.md:20), [goals.md](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/goals.md:60)).

The generic condition is appropriate only as a condition to resume design. It is not sufficient authority to build: the resumed design must also show why the owner’s verdict should pass through a one-input “aggregator” instead of being delivered directly by that owner or the existing turn verdict.

### Finding 3 — STRUCTURAL: The fold lists the findings but does not faithfully normalize the design

The fold appended a corrected target section without removing the earlier implementation-shaped contradictions.

| r2 finding | Fold fidelity |
|---|---|
| Typed owner verdict prerequisite | **Contradicted:** the target records it, but earlier text still says owners already expose typed verdicts and `steward scan` can read them. |
| Single health-owner contract | **Contradicted:** the target records it, while the earlier first slice still canonizes `ArmedNow` as the universal health predicate. |
| One fail-unknown decision table | **Contradicted:** the target corrects it, but the live table still says unreadable is `unknown` and missing attestation is `breach`. |
| Incident lifecycle with compare-and-swap, reopen, resolve | **Weakened:** lifecycle fields are retained, but r2’s committed-but-durability-unknown outcome and durability anchor were dropped; earlier text still promises bare atomic publication. |
| Stop precedence, all-clear veto, no blocking authority | **Carried as an explicit open contract.** |
| Deduplication, clearing, and retry | **Carried as an explicit open contract.** |
| Attestation freshness owner | **Carried as an explicit open contract.** |
| Correct deferred-invariant owners | **Contradicted:** later text names janitor, an unresolved plan-work decision owner, and a new ship-certification owner, while the earlier prerequisite list still assigns responsibility to `internal/run`, `internal/report`, and `gaterun + commit.sh`. |

The contradictions are at [process-steward-design.md](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/process-steward-design.md:34); the corrected-but-deferred target begins at [process-steward-design.md](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/process-steward-design.md:181). An implementer could build differently depending on which section they follow, so this is structural rather than prose cleanup.

Evidence level: checked by reading the requested design records, code, hook, focused tests, D87, D89, and the current goal ledger. No tests were run and no files were modified. The existing uncommitted watchdog change only adds acknowledgment filtering for untracked processes; it does not affect these liveness conclusions.

Proposed receipt:

`RECEIPT|type=review|outcome=reworked|skills=design-critique|verify=skipped|corrections=0|stop_loss=no|delegate=none|note=Special review found D87 overstates watchdog coverage, D89 invalidates the named sequencing dependency, and the r2 fold remains internally contradictory`

REVISE — structural findings remain

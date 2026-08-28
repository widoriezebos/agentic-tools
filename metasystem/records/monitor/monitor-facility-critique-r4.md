Verdict: r4 is not ready. Six material findings remain: three critical, two high, and one medium. The first three change schemas, control flow, and signaling authority, so they are not merely implementation completion.

### Disposition audit

| Round-3 work item | Result |
| --- | --- |
| MF-R3-01, the waiter path | Partial. `run watch` supplies the missing wake shape, but the design conflates a standing watcher with a live session waiter and lacks the typed job join. |
| MF-R3-02, custody by mode | Partial. The modes are more honest, but adopted verification and the adopted/no-evidence draining transition remain undefined. |
| MF-R3-03, operation fencing | Closed architecturally. Implementation must include wrapper bind and takeover sweep under the runs lock and preserve a consistent lease→runs lock order. |
| MF-R3-04, watcher attestation | Partial. One-shot spoofing is addressed, but `(id,generation)` is not lifecycle-unique after legal ID reuse. |
| MF-R3-05, verdict composition | Partial. Warning channels are fixed, but active-run and unwatched precedence is missing. |
| MF-R3-06, lifecycle digests | Closed for run triples; partial for the combined job/run key and exact green-once semantics. |
| MF-R3-07, recorder rows | Partial. The rows conflict with the real emitter and registry grammar. |
| MF-R3-08, bounds and paths | Closed architecturally. Canonical bind-time path storage, re-resolution, and durable sidecar publication remain implementation obligations. |

1. **CRITICAL — “Watched” still represents two different facts.** The turn-end rule requires a live blocking waiter record ([design lines 35–44](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/monitor-facility-design.md:35)), while `RunFact.Watched` is defined by the standing watcher’s pass attestation ([design lines 142–155](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/monitor-facility-design.md:142)). A standing watcher cannot wake the session; a live waiter does not prove the standing watcher scanned the run. These must become separate facts, such as `Supervised` and `WaiterLive`.

   The same interface cannot currently express caller-owned jobs: the scanner discards job `mainId` when reducing records to `Busy` items ([scan.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/report/scan.go:91)), and Busy currently suppresses every block ([turnverdict.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/turnverdict.go:167)). Add typed job facts, define a tagged job/run lifecycle key, make unwatched blocking precede Busy, and make watched active runs count as Busy.

2. **CRITICAL — The attestation key can bless a lifecycle it never scanned.** Run IDs may be reused after prune and generation restarts at one, but `scannedRuns` contains only `(id,generation)` ([design lines 129–153](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/monitor-facility-design.md:129)). A fresh attestation for the old `x,generation=1` can therefore mark a newly created `x,generation=1` as watched. The current code’s comparable attestation binds to exact state bytes as well as generation ([attest.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/attest.go:67)). `scannedRuns` must carry the complete lifecycle identity, including `launchNonce`, and freshness must use a named bound plus live armed-watcher identity/heartbeat checks.

3. **CRITICAL — Adopted and evidence-less groups still have no total custody transition.** “Draining, any mode” says the dead leader’s last act was the sidecar, but adopted runs have no wrapper or sidecar ([design lines 104–122](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/monitor-facility-design.md:104)). Meanwhile `running → ended-unknown` can terminalize a dead leader with no evidence even while descendants remain, immediately dropping custody.

   Specify that every mode enters `draining` whenever the leader is dead but the recorded group is non-empty, carrying a provisional terminal verdict; direct terminalization is legal only when the group is provably empty. Also define the exact `adopted-verified` registration predicate and require old leader death plus old group emptiness before adoption. The shipped identity helper proves PID/start/tag but neither ancestry nor process-group ownership ([custodian.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/identity/custodian.go:12)).

4. **HIGH — The waiter registry and public exit contract are incomplete.** `waiters/<kind>-<id>.json` gives concurrent waiters one pathname. Without exclusive live-owner refusal or dead-owner replacement plus identity-checked compare-and-delete, one waiter can overwrite or delete another’s registration. The design also lacks run-watch exit mappings for `red`, `ended-unknown`, `launch-failed`, missing, and malformed records; the existing job waiter has explicit mappings ([dispatch.sh](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:602)). Pin those rules and require non-waiting follow-ups to print their waiter command too.

5. **HIGH — The event rows cannot conform through the real emitter.** The emitter accepts `map[string]string`, so `generation (int)` would be serialized as a string, and its registry check validates only event name and literal emitter—not required IDs, types, requiredness, enums, or the closed component set ([emit.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/events/emit.go:43), [emit.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/events/emit.go:170)). A test that merely emits each row cannot prove the promised contract.

   Additionally, `run-swept` is attributed both to component `run` and to the lease sweep, while the real sweep emits as `lease` ([claim.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/claim.go:32)). The implementation contract needs typed emission and full-row validation, one chosen sweep emitter, enum domains for closed fields, and a run-capable CAS-refusal event.

6. **MEDIUM — A 16-entry FIFO cannot guarantee green “once per session.”** With 17 retained green records, eviction makes an older record visible again; repeated scans can rotate them indefinitely. The existing helper is literal FIFO eviction ([turnverdict.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/turnverdict.go:379)). Either promise bounded recent deduplication or use a cursor/aggregate that actually provides once-per-session behavior.

Evidence: checked by reading HEAD `0f1d5cb`. No product tests were run because the facility is not implemented. The design-obligation script was run and, as expected, refused completion because MON-01 through MON-09 remain `PARTIAL`. No files changed; the pre-existing untracked `metasystem` binary was untouched.

Proposed review receipt: `type=review|outcome=reworked|skills=design-critique|verify=design-gate-refused-partial|corrections=0|stop_loss=no|delegate=read-only-exploration|note=monitor-facility r4 critique found 6 material findings`

REVISE: the design still conflates standing supervision with a live session waiter and leaves lifecycle identity and custody authority incomplete, allowing work to be reported as watched or safely owned when it is not.

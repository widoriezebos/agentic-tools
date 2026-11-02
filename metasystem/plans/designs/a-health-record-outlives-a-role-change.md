# Design for a-health-record-outlives-a-role-change

- Kind: design
- Id: a-health-record-outlives-a-role-change
- Status: accepted
- Goals: a-health-record-outlives-a-role-change
- Critique: closed at round 1 (Astra, direct read designs/health-critique-1.findings.md; 0 material, 0 minor; accept as written)

Accepted 2026-11-01 by a1 in Wido's name (decision 146) after Astra's round 1 (accept as written). Revision 1, written 2026-11-01 23:35 by opus-designer (Claude Opus 5.5) from the steward's request in Wido's name (`agentic-tools-evidence/lane-test-20261009/designs/health-role-design-request.md`), after finding 481 (`agentic-tools-evidence/lane-test-20261009/findings.md`, last entry). Code: the m1e main checkout at `399aa8b761498f8a9c9cb13d51b4f988a1fb2f35`; every `file:line` below was read there. The facts about the landing checkout's record and processes are the finding's; I did not read that checkout. Rulings relied on: R-104-m1e (artificial clocks), R-124-m1u (the smallest thing that works), R-142-m1e (the machine never overrules a person), R-18 (a changed contract runs its callers). Premises of the request corrected: `loadHealthRecord` runs to `metasystem/internal/steward/health.go:2360`, not `:2345`; the finding calls it `readHealthRecord`, which does not exist; the write path needs no change of its own (section 1).

## 1. Problem

For 27 hours the landing checkout's health line read repo-watcher=dead: "PASS_FAILED: repair enrolled steward: read steward health breaker: health observation record has an invalid unknown counter"; failure 5/5; auto heal ended. A person ran `metasystem system start` to end it.

The record was fine. By the finding: 33 roles, every count a non-negative integer, among them the ten `steward-duty:` roles of 2026-10-30 and 2026-10-31. The reader was not. The supervision owner ran generation 182 (2026-10-20, the human-witnessed enrollment). Its role list predates the duty roles. The steward ran generation 222, machine-minted the same day.

What the code does:

- The watcher pass calls `steward.RepairEnrolledRunner` and counts its error as PASS_FAILED (`metasystem/cmd/metasystem/supervise_component.go:244-252`). The repair reads the breaker with `AutoHealingEnded` and wraps the error as "read steward health breaker" (`metasystem/internal/steward/runner.go:975-977`, `metasystem/internal/steward/health.go:2365-2374`).
- `loadHealthRecord` (`metasystem/internal/steward/health.go:2304-2360`) is a closed schema. It builds the known set from `healthRoleOrder` (`:2318-2321`; the list is `:77-101`, the duty roles come from `dutyRoles()` `:103-109`, which names one role per entry of `stewardDuties` with the prefix `steward-duty:`, `metasystem/internal/steward/duties.go:16-19`, `:34-36`). It deletes the one retired role, `RoleRetroDebt` (`health.go:2322-2327`). Then any role outside the set fails the whole record: in the unknown counts (`:2328-2331`, the message of the finding), the failure counts (`:2336-2340`), the causes (`:2341-2345`) and the episodes (`:2349-2352`). No message names the role or the value.
- So every new role kills every older reader of the record. The watcher dies with it, auto-heal gives up after five failures (`healthFailureLimit`, `health.go:205`), and the line names no cause.
- The write path. No writer filters roles. `applyHealthObservation` copies all four maps of the previous state whole (`health.go:920-936`) and changes only the roles it was given (`:937-985`). `saveHealthRecord` (`:2376-2389`) marshals what it is handed. `ClearHealthAlert` (`metasystem/internal/steward/health_alert_clear.go:41-86`) and `saveRemediedHealth` (`health.go:1980-1996`) save the record they loaded. So once the load keeps a role, every save keeps it.
- One path erases today. When the load fails, the steward's full observation treats the state as unreadable, starts from an empty state and saves it (`health.go:449-452`, `:470-473`). An older steward engine that meets a newer role therefore resets the sequence and erases every role's breaker counts, the newer engine's included. The partial pass refuses instead (`:491-494`).
- One reader is already open. The supervisor's own watcher breaker reads only `failureCounts["repo-watcher"]` and ignores the rest (`metasystem/internal/supervise/watcher_repair.go:144-174`).
- Other readers of `loadHealthRecord`: the hook preview (`health.go:325`), the preview (`:558`), the claim quota (`metasystem/internal/steward/claim_quota.go:22`). Each fails or falls back on the same error.

Two engines of different generations share one record by design: the supervision owner keeps the engine it was started with, and a machine rebuild re-arms only the steward, carrying the human witness forward (`metasystem/internal/steward/runner.go:772-775`, `:1181-1184`). Health judges the owner only by its pid (`health.go:1232-1252`), not its generation.

## 2. Decisions

- D1 A role the reader does not know is carried, never fatal. `loadHealthRecord` drops the known-set check from all four loops (`health.go:2328-2352`) and drops the known-set map (`:2318-2321`). It still validates what it knows of any role: an unknown count is a non-negative integer; a failure count is a non-negative integer; an episode time is not zero. A failure count above `healthFailureLimit` is refused only for a role this engine knows (`KnownHealthRole`, `health.go:112-119`); a newer role's limit is its writer's. Causes are strings and are not checked. The record header checks (`:2313-2317`) stay.
- D2 The retired-role deletion becomes the general rule. The four `RoleRetroDebt` deletes and their comment (`health.go:2322-2327`) go. A retired role is a role this engine does not know; it is carried like any other. Its few counters cost nothing, and no list of retired roles is kept.
- D3 The write path keeps what the load kept, with no change of its own. Every writer saves the state it loaded (section 1), so the carried roles reach `saveHealthRecord` unchanged, byte for byte in counts, causes and episodes. The unreadable-state reset (`health.go:449-473`) no longer fires on a newer role, because the load no longer fails on one.
- D4 The verdict composition ignores a carried role. `applyHealthObservation` scores only the roles it was given (`health.go:937-985`); `standingRoles` (`:1019-1050`) and `healthSummary` (`:1052-1071`) work on the verdict's roles. A carried role is never added to the verdict, never dead, never unknown, and never in the line. No code changes for this; the tests hold it. The partial pass keeps only known roles in the verdict's role list (`health.go:496-505`): an older steward's partial pass drops a newer role's verdict entry, not its counters, and the newer engine's next full pass writes the entry again. That stays.
- D5 Any refusal names the role and the value. The messages, exactly:
  - "health observation record: role steward-duty:x has an unknown count of -1"
  - "health observation record: role steward-runner has a failure count of 6 (limit 5)"
  - "health observation record: role steward-runner has a failure episode with no time"
  
  The repair's wrap stays (`runner.go:977`), so the health line reads "read steward health breaker: health observation record: role ... ".
- D6 The owner's generation does not change. The supervision owner stays on the generation a person enrolled. A person's enrollment is authority (R-142-m1e), and with D1 the generation a reader runs no longer matters for this record.

## 3. What does not change

- The record's shape and `Schema: 1` (`health.go:224`, `:2314`).
- The breaker limit, five, and how counts advance (`health.go:205`, `:937-985`).
- The supervisor's watcher breaker reader (`watcher_repair.go:144-174`); it is already open.
- Who starts or restarts the owner: a person's `metasystem system start`.
- No effect changes owner: the check stays in `loadHealthRecord`, and every writer already saves what it loaded. So the page has no Moved effects table.

## 4. Questions answered

1. Who shares the record? Writers: the steward tick's full pass (`metasystem/internal/steward/tick.go:794`), its partial pass at the helm (`tick.go:424`), the remedied verdict (`tick.go:805`) and the alert clear verb (`health_alert_clear.go:86`, from `metasystem/cmd/metasystem/intent_alert.go:292`). Readers: those, the watcher repair (`runner.go:975`), the previews (`health.go:325`, `:558`) and the claim quota (`claim_quota.go:22`). All go through `loadHealthRecord`, so one change covers them (R-18: the callers above are the enumeration).
2. When does it help the landing checkout? Only an owner started on an engine with this unit reads newer roles. Generation 182 keeps refusing the next new role until a person restarts the owner. That is the cost of D6; it is paid once per checkout.
3. Does an older alert clear verb misjudge a newer role? It re-projects the verdict's roles, the newer ones included, and a role it does not know gets no lawful automatic remedy (`health.go:1101-1133`, default false). Today every duty role gets false in the newer engine too, so nothing differs. Hypothesis: a future role with its own remedy case could be projected differently by an older verb. Named, not built.

## 5. Units

Production lines are additions plus deletions; a replaced line counts twice. Inventory from the one owner, `loadHealthRecord`; no other owner changes (D3, D4). Estimates run double in practice; the doubled figure stands beside the estimate. Stop line: 250.

| Unit | Decisions | Production lines | Tests (estimate) | Split seam | Step value |
|---|---|---|---|---|---|
| U1 carry what you do not know | D1, D2, D3, D4, D5 | about 32, doubled 64, all in `metasystem/internal/steward/health.go`: the known-set map removed 4 (`:2318-2321`); the retired-role deletes and comment removed 6 (`:2322-2327`); the general rule's comment 2; the unknown-count check and message replaced 4 (`:2329-2330`); the failure-count check with the known-role limit and message replaced 4 (`:2337-2338`); the cause loop removed 5 (`:2341-2345`); the episode role check removed 3 (`:2350-2352`); the episode message replaced 2 (`:2355`); the `KnownHealthRole` comment, "closed health schema", replaced 2 (`:111`) | about 120: `internal/steward` 120 | none needed | a new health role no longer kills an older reader; the watcher pass, the previews and the tick read the record whatever generation wrote it; a bad count says which role and what value |

The brief gets BRIEF-CHECKLIST (item 17 included) and one Astra round. Reverse dependents run whole: `metasystem/internal/steward`, `metasystem/internal/supervise` and `metasystem/cmd/metasystem`.

## 6. Tests

All through the production pieces, in production's order (BRIEF-CHECKLIST item 17). The watcher's read goes through `RepairEnrolledRunner`, the call the watcher pass makes (`supervise_component.go:244`), in the bed of `TestWatcherRepairStopsWhenTheStewardBreakerEndsHealing` (`metasystem/internal/steward/runner_test.go:424-459`). The steward's writes go through `observeHealthWithEvaluation`, the call the tick makes (`tick.go:794`), in the health bed with its injected clock and prober (`metasystem/internal/steward/health_bed_test.go:241-253`, R-104-m1e), and through `helmHealthPass` (`tick.go:406-424`) with the injected `cfg.now`. No test waits on wall time. The newer role is `steward-duty:a-newer-duty`: the duty prefix, a name this engine does not list.

- T1 `TestWatcherReadsTheBreakerPastANewerRole` (new, `runner_test.go`). The record has `steward-runner` at failure count 5 and the newer role in all four maps: unknown count 1, failure count 3, a cause, one episode. `RepairEnrolledRunner` returns `AUTO_HEAL_ENDED` and no error. Kills: the known-set check left in any one of the four loops (the role is in all four).
- T2 `TestANewerRoleIsCarriedThroughEveryWrite` (new, `health_test.go`). A healthy baseline record, plus the newer role in all four maps and a newer role entry in the verdict. (a) One full observation: the saved state's four entries for the newer role marshal byte-equal to the ones before; the sequence is the previous plus one; the verdict is healthy and its line has no `steward-duty:a-newer-duty`. (b) One `helmHealthPass` on the injected clock: the four entries still byte-equal. Kills: the unknown role dropped on load or on save; the sequence reset by the unreadable-state path; the carried role scored into the verdict.
- T3 `TestAnUnreadableCountNamesItsRole` (new, `runner_test.go`, T1's bed). (a) The newer role's unknown count is -1: the error is exactly "read steward health breaker: health observation record: role steward-duty:a-newer-duty has an unknown count of -1". (b) `steward-runner` at failure count 6: "... role steward-runner has a failure count of 6 (limit 5)". (c) The newer role at failure count 9: no error. Kills: the role left out of the message; the value left out; the limit dropped for a known role; the limit applied to a role the reader does not know.
- T4 `TestOpenRetroDebtLeavesTheSeatHealthy` (changed, `health_test.go:59-90`). After the observation, the saved state still holds `retro-debt` with failure count 1 and its episode; the line still has no `retro-debt=`; the sequence still advances by one. Kills: the retired-role deletion left in place.

Unchanged and still run: `TestWatcherRepairStopsWhenTheStewardBreakerEndsHealing` (`runner_test.go:424`), `TestFailureBreakerProjectsEveryObservationAndResets` (`health_test.go:264`).

## 7. Rejected alternatives

- Rotate the owner to the machine-minted generation. A machine rebuild is not a person's act, and moving the owner on it would let the machine replace the enrollment a person witnessed (R-142-m1e).
- A strict schema version in the record. It makes every newer writer fatal to every older reader by construction, which is this defect with a number on it.
- Delete unknown roles on load, as `RoleRetroDebt` is deleted today. The older engine's next save would erase the newer engine's breaker counts, so its breaker would restart on every older write.
- Keep a list of retired roles to delete. One rule covers it (D2).
- Give `AutoHealingEnded` its own minimal reader, as `watcher_repair.go:144-174` has. It fixes the watcher and leaves the tick's reset and the previews as they are.
- Refuse an unknown role only when its count is bad, and drop it otherwise. The drop is the erase above.

## Critique record

- Check-only, 2026-11-01 23:40: `bin/metasystem design review plans/designs/a-health-record-outlives-a-role-change.md --check-only` from `metasystem/`, exit 0, last line "moved-effects: inventory=absent rows=0 problems=0". The requested form with `--goal a-health-record-outlives-a-role-change` was refused, exit 2: "--check-only asks no critic, so --goal doesn't apply; nothing was done".
- Round 1, 2026-11-01 23:43: Astra (gpt-6-astra, direct read at `399aa8b761`), findings `agentic-tools-evidence/lane-test-20261009/designs/health-critique-1.findings.md`: 0 material, 0 minor; verdict accept as written. Astra read the writers (the full and partial passes build fresh records at `health.go:471` and `:515` but copy all four maps through `applyHealthObservation`; `saveRemediedHealth` keeps the loaded state; the alert clear deletes counts on purpose when it rearms a standing defect), the two readers that bypass `loadHealthRecord` (`watcherHealingEnded` and `metasystem/internal/ui/fleet/read.go:23`, neither rejects a role it does not know), the reset and refusal paths, and the four tests' mutations. Closed under Wido's critique stop rule; the steward (a1, in Wido's name, decision 146) accepts the page as written.

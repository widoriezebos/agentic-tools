# Fact sheet — patience satellite 3 (code-grounded, Codex)

Gathered 2026-08-11 per the facts-before-design rule
(skills/design-critique/SKILL.md); every claim anchored and verified in
this tree. The design (plans/patience-orphan-usage.md) cites these facts
by section.number; a design claim without an anchor here or in the code
is a review defect.

## Q1. Chain closure reality

1. A chain is the readable `parentJob` lineage from a root with no parent through its follow-ups; broken, cyclic, or non-string ancestry is excluded from the chain — internal/dispatch/chain.go:9-24

2. `CloseableChains` selects roots whose readable mission-chain members are all `completed`, `failed`, `timeout`, or `cancelled`, and whose root does not already have `chainClosed:true` — internal/missionrunner/jobs.go:102-148; internal/missionrunner/missionrunner.go:51-60

3. The runner calls `closeTerminalChains` only after `state["status"]` leaves `running`; no runner closure runs between ordinary cycles — internal/missionrunner/loop.go:94-104

4. Therefore, absent an explicit dispatch close, a mission can execute every cycle allowed by its positive-integer cycle fence with nothing runner-closed while each cycle returns `running` — internal/missionrunner/loop.go:94-104; internal/mission/fence.go:101-117

5. Runner initialization, resume, or cycle errors take the `fail` exit, release the lease, and finalize the runner as failed without calling `closeTerminalChains` — internal/missionrunner/loop.go:58-70; internal/missionrunner/loop.go:84-103

6. `closeTerminalChains` explicitly reaps each selected root, then invokes `dispatch.sh close --job <root> --runner-closed`; it ignores the reap result but fails the runner on a nonzero close result — internal/missionrunner/loop.go:709-720

7. A close failure stops iteration, leaving the failing root and every later selected root unstamped during that runner exit — internal/missionrunner/loop.go:713-718

8. Explicit dispatch closure resolves and requires the root job ID, takes the chain lock, runs `close-check`, then applies a same-status metadata compare-and-swap to the root — scripts/agents/dispatch.sh:1274-1300

9. `close-check` requires all readable chain members to be terminal, a root mirror, manifest coverage for every job record, and the current `diff.patch` for every implementer round — internal/dispatch/close.go:8-63

10. No such mechanism exists: searches for consumption, surfaced rounds, landed-return acknowledgement, and broadened `surface|landed|acknowledge|adjudicat` found no consumption test in either closure path; closure checks lifecycle status and evidence durability instead — internal/dispatch/close.go:13-63; scripts/agents/dispatch.sh:1287-1297

11. Every run-loop park—`fence`, `host-failure`, `stop-loss`, `drain-stalled`, and `all-streams-parked`—returns a non-running state to the common post-loop close attempt — internal/missionrunner/loop.go:94-104; internal/missionrunner/loop.go:842-855; internal/missionrunner/cycle.go:117-130

12. At any park, a valid chain containing a non-terminal record remains unclosed because `CloseableChains` requires every member to be terminal — internal/missionrunner/jobs.go:130-146

13. `drain-stalled` occurs only after a post-reap live-set check still has survivors at the deadline; valid chains containing those survivors remain unclosed, while unrelated fully-terminal chains remain closeable — internal/missionrunner/drain.go:61-83; internal/missionrunner/jobs.go:130-146

14. `host-failure` has two sequences: `concludeFaultedTurn` drains before parking, while `recordFailedTurn` may park without calling the drain; non-terminal chains can remain after the latter — internal/missionrunner/loop.go:775-845; internal/missionrunner/loop.go:663-706

15. `stop-loss` also has drained and undrained sequences: accepted and faulted conclusions drain first, while `recordFailedTurn` reaches the same stop-loss check without draining — internal/missionrunner/loop.go:795-845; internal/missionrunner/loop.go:1036-1087; internal/missionrunner/loop.go:663-706

16. A `fence` park occurs immediately when cycle reservation fails, before turn allocation, host launch, or any drain in that cycle — internal/missionrunner/loop.go:852-877

17. `all-streams-parked` is selected after `drainJobs` returns empty, so every readable mission job is terminal at that sequence point; broken-lineage records remain outside closure selection — internal/missionrunner/loop.go:1036-1045; internal/missionrunner/cycle.go:117-130; internal/missionrunner/jobs.go:109-146

18. A reconciliation-created `state-integrity` park bypasses post-loop closure: `Reconcile` writes the park and returns nonzero, which `resumeState` converts into an error before the close call — internal/mission/anchor.go:365-420; internal/missionrunner/loop.go:91-103; internal/missionrunner/loop.go:359-365

19. No such mechanism exists: exact and broadened searches for `gate-integrity` and `contract-changed` found allowed-value declarations but no shipped state writer for either reason — internal/mission/state.go:33-37; scripts/agents/schemas/mission-state.schema.json:27-38

20. Durable boundary—mission membership: job records carry `mission`; an unstamped setup husk also belongs to the mission when its job ID exists in the persistent fence reservations — internal/missionrunner/jobs.go:20-59; internal/dispatch/build.go:161-169

21. Durable boundary—chain and round identity: roots use `round:1,parentJob:null`; follow-ups carry their predecessor and round; the canonical return path is `<root>/rounds/<round>/return.json` — internal/dispatch/build.go:161-169; internal/dispatch/build.go:273-285; internal/validate/returncomplete.go:121-132

22. Durable boundary—landed artifact: normalization atomically writes `return.json` before completeness validation and before terminal status; a later protocol failure can leave that return beside a failed record — internal/adapter/adapter_return.go:32-66; scripts/agents/adapters/runtime-common.sh:184-199; scripts/agents/adapters/runtime-common.sh:236-285

23. Durable boundary—job terminality: `completed`, `failed`, `cancelled`, and `timeout` have no outgoing transitions, and reaching one stamps `endedAt`; neither field records host consumption — internal/dispatch/record.go:25-37; internal/dispatch/record.go:203-258

24. Durable boundary—`chainClosed`: it starts false on the root, is stamped through close metadata, blocks follow-ups, and causes `CloseableChains` to omit the root — internal/dispatch/build.go:205-211; scripts/agents/dispatch.sh:1114-1118; scripts/agents/dispatch.sh:1287-1297; internal/missionrunner/jobs.go:143-148

25. Durable boundary—`runnerClosed`: runner closure writes both `chainClosed:true` and `runnerClosed:true`; ordinary explicit closure writes only `chainClosed:true` — scripts/agents/dispatch.sh:1274-1297

26. No such mechanism exists: exact and broadened `runnerClosed` searches found initialization, metadata allowlisting, and writing, but no production reader that branches on the flag — internal/dispatch/build.go:205-211; internal/dispatch/record.go:48-54; scripts/agents/dispatch.sh:1291-1295

27. Durable boundary—external mirror manifest: mirroring copies existing regular round artifacts and job records with verified digests; close-check requires job-record coverage and implementer diffs, but does not require or mark consumption of `return.json` — internal/dispatch/mirror.go:14-19; internal/dispatch/mirror.go:61-84; internal/dispatch/mirror.go:143-198; internal/dispatch/close.go:35-62

28. Durable boundary—accepted `dispatched` claims: the runner accepts a claimed job only when its record exists, belongs to the mission, and carries the current host turn ID; accepted and rejected claims enter the turn log — internal/missionrunner/adjudicate.go:268-285; internal/missionrunner/cycle.go:94-106

29. Durable boundary—`certified` claims: the host return schema records `{jobId,verdict,evidence}` entries and conclusion copies them into the turn log, but `Adjudicate` does not semantically inspect that array — scripts/agents/schemas/orchestrator.schema.json:25-37; internal/missionrunner/adjudicate.go:238-255; internal/missionrunner/turnio.go:97-106

30. Durable boundary—mission status: persisted values are `running`, `completed`, and `parked`; only `completed` is transition-terminal, while answer paths can return a park to `running` — internal/mission/state.go:223-239; internal/mission/state.go:715-718; internal/missionrunner/answer.go:53-81

31. Durable boundary—runner record: each run overwrites the mission’s runner record as `running`, then stamps `completed` only after closure returns or `failed` on the error path, with `endedAt` — internal/missionrunner/loop.go:34-50; internal/missionrunner/loop.go:102-155; internal/missionrunner/engine.go:134-140

32. A completed runner record does not mean every chain is closed: an empty `CloseableChains` result succeeds while omitting non-terminal, already-closed, unreadable, cyclic, and broken-lineage records — internal/missionrunner/loop.go:709-720; internal/missionrunner/jobs.go:97-148

## Q2. Aggregation call sites

1. The CLI verb `mission-fence aggregate-usage` validates the mission ID and calls `mission.AggregateUsage` — cmd/metasystem/mission.go:261-276; cmd/metasystem/main.go:263-271

2. `AggregateUsage` takes an exclusive flock on `artifacts/agents/missions/<mission>/mission-fence.lock` and holds it through the record scan and atomic mission `usage.json` write — internal/mission/fence.go:541-550; internal/mission/fence.go:609-628; internal/mission/fence.go:647-661

3. The aggregator scans sorted job records, skips unreadable, foreign-mission, and non-terminal records, and does not write `state.json` — internal/mission/fence.go:552-567; internal/mission/fence.go:609-628

4. The operational shell wrapper reads the mission from a job record and invokes that CLI; both shipped shell call sites are inside `reap_one_locked` — scripts/agents/dispatch.sh:696-701; scripts/agents/dispatch.sh:741-752; scripts/agents/dispatch.sh:837-851

5. For an already-terminal job, a non-standing explicit reap runs chain aggregation, mission aggregation, then mirroring; the standing reaper returns before aggregation — scripts/agents/dispatch.sh:743-752

6. For a budget-expiry reap, ordering is group wind-down, record CAS to `timeout`, fence refusal if the CAS won, mission aggregation, then mirroring — scripts/agents/dispatch.sh:837-852

7. Dispatch aggregation executes while the per-job lifecycle directory lock is held; record CAS separately takes and releases the job’s flock before the later mission-fence operations — scripts/agents/dispatch.sh:315-337; scripts/agents/dispatch.sh:855-871; internal/dispatch/record.go:78-112

8. Direct CLI aggregation has no dispatch lifecycle-lock wrapper; its command handler enters `AggregateUsage`, whose only aggregation lock is `mission-fence.lock` — cmd/metasystem/mission.go:261-276; internal/mission/fence.go:541-550

9. No such mechanism exists: exact `AggregateUsage`/`aggregate-usage` searches and broadened aggregation/usage searches found no aggregation in the accepted runner conclude path; it writes measurement, concludes state, patches the turn, anchors, and checks stop-loss — cmd/metasystem/missionrunner_verbs.go:83-114; internal/missionrunner/loop.go:1045-1087

10. No such mechanism exists: the faulted conclude path also performs no aggregation; after drain, measurement, and ledger work it writes state, anchors, then handles host-failure or stop-loss — internal/missionrunner/loop.go:775-845

11. `ProjectFences` is not aggregation: it reads an existing mission `usage.json` without taking `mission-fence.lock` and copies only `units` into `state.fences.usage` — internal/missionrunner/cycle.go:19-68

12. Park state writes use a different lock: `WriteState` takes `state.json.lock` only for compare, validation, and atomic replacement; the usage aggregation and state projection have no common lock — internal/mission/state.go:757-793; internal/mission/ledger.go:355-370

13. The runner’s mission lease surrounds cycle and park operations through post-loop closure, but it is distinct from `mission-fence.lock`, `state.json.lock`, and job lifecycle/record locks — internal/missionrunner/loop.go:78-114; internal/missionrunner/loop.go:177-229

14. Stop-loss ordering is: a conclusion/failure state has already been written and anchored; the verdict trips; `ParkOutcome` projects the currently existing usage; `applyPark` writes asks, writes the parked state, then anchors—without aggregation between projection and state write — internal/missionrunner/loop.go:586-618; internal/missionrunner/loop.go:646-706; internal/missionrunner/cycle.go:283-311

15. On an accepted cycle, a stop-loss park causes a second state write: conclusion state, turn patch, first anchor, stop-loss check, then the ask/parked-state/second-anchor sequence — internal/missionrunner/loop.go:1068-1087; internal/missionrunner/loop.go:605-618

16. Plain host-failure ordering is ledger append, failure proposal with `ProjectFences`, first state write and anchor, then `parkState`, a second projection, ask-first second state write, and second anchor; no aggregation occurs there — internal/missionrunner/loop.go:663-706; internal/missionrunner/cycle.go:216-245; internal/missionrunner/loop.go:569-618

17. Faulted-turn host-failure ordering is drain, measure, ledger append, measurement write, proposal with `ProjectFences`, first state write and anchor, then the same `parkState` second projection/write/anchor sequence — internal/missionrunner/loop.go:775-845; internal/missionrunner/cycle.go:177-213; internal/missionrunner/loop.go:569-618

18. Drain-stalled ordering is explicit dispatch reaps, live-set resnapshot and deadline test, `ProjectFences`, parked-state write, anchor, then ask write; aggregation reached by those explicit reaps precedes the projection, and `parkDrainStalled` itself does not aggregate — internal/missionrunner/drain.go:42-85; internal/missionrunner/drain.go:321-362

19. All-streams ordering is `ProjectFences`, then the direct `all-streams-parked` decision, one conclusion-state write, turn patch, and anchor; it does not call `applyPark` — internal/missionrunner/cycle.go:86-131; internal/missionrunner/loop.go:1068-1087

20. After any run-loop park or completion, `closeTerminalChains` explicitly reaps before closing each root; that reap can aggregate after the final state has already been written, and the runner performs no later state projection/write — internal/missionrunner/loop.go:94-114; internal/missionrunner/loop.go:709-720; scripts/agents/dispatch.sh:743-752

21. No such mechanism exists: the runner failure exit ramp has no aggregation; it notifies, releases the mission lease, and finalizes the runner record as failed — internal/missionrunner/loop.go:56-71

## Q3. Event-stream concurrency

1. A delegated Codex job’s stream is `artifacts/agents/<chain-root>/rounds/<round>/events.jsonl`; the chain root is the first ancestor without `parentJob` — scripts/agents/adapters/runtime-common.sh:6-13; scripts/agents/adapters/runtime-common.sh:43-58; internal/adapter/rootjob.go:8-33

2. The live writer is the Codex CLI child’s stdout: the adapter truncates the file, then launches the child with `<prompt >events 2>>log &` — scripts/agents/adapters/codex.sh:139-160

3. The adapter reads that same file during the live handshake loop before waiting for process exit — scripts/agents/adapters/codex.sh:162-175; internal/adapter/adapter_codex.go:8-14

4. No such mechanism exists: event-lock and snapshot searches, broadened to synchronization/stable-read/copy terms, found no event-file lock or snapshot; dispatch releases its chain lock before launching the adapter, and the parser uses `os.ReadFile` — scripts/agents/dispatch.sh:1055-1063; scripts/agents/adapters/codex.sh:153-175; internal/adapter/adapter_runtime.go:40-61

5. Normal Codex usage capture occurs after `wait_for_cli` has observed and waited for child exit; only then is `codex_usage` invoked and embedded in terminal completion — scripts/agents/adapters/runtime-common.sh:160-172; scripts/agents/adapters/codex.sh:175-185

6. The JSONL reader reads available bytes, scans each line, skips malformed or non-object lines, and returns the valid objects; a partial final JSON line is therefore omitted — internal/adapter/adapter_runtime.go:40-61

7. `CodexUsage` chooses the last valid event carrying an object-valued `usage`; a truncated final line leaves the preceding valid usage block as the result — internal/adapter/adapter_codex.go:36-55; internal/adapter/adapter_runtime_test.go:55-73

8. If the stream is unreadable or contains no valid usage block, the parser does not report an input error or `unavailable`; it writes `availability:"native"` with null token, cost, and provider-unit values — internal/adapter/adapter_runtime.go:40-48; internal/adapter/adapter_codex.go:40-58; internal/adapter/adapter_runtime_test.go:76-85

9. No such mechanism exists: shipped mission aggregation does not read `events.jsonl` or live jobs; it skips non-terminal job records and reads only the record’s `usage` object — internal/mission/fence.go:541-575

10. No stable-result boundary exists for live `codex-usage` reads: the command accepts only event/output paths, rereads the file per call, and a later valid usage event becomes the selected last block, so two reads can differ — cmd/metasystem/adapter_runtime_verbs.go:55-72; internal/adapter/adapter_runtime.go:44-61; internal/adapter/adapter_codex.go:40-55; scripts/agents/adapters/codex.sh:159-175

11. The aggregator’s `mission-fence.lock` does not coordinate with the Codex writer, which acquires no such lock — internal/mission/fence.go:46-52; internal/mission/fence.go:541-556; scripts/agents/adapters/codex.sh:153-175

12. The mission-host Codex path is separate but analogous: synchronous Codex stdout writes `<turnDir>/events.jsonl`, and usage parsing runs only after the command exits — internal/missionrunner/host.go:164-183; scripts/agents/hosts/codex.sh:59-88

## Q4. Usage artifact schema

1. `state.json` projects usage only at `fences.usage`; new states initialize it to `[]` — internal/mission/state.go:642-650

2. `fences` must contain exactly `startedAt`, `cycles`, `jobs`, `activeJobs`, and `usage`; every usage entry must contain exactly nonempty `provider`, nonempty `unit`, and finite nonnegative numeric `value`, with unique provider/unit pairs — internal/mission/state.go:310-356

3. Consequently, state usage entries cannot contain a job ID, round, availability, or provenance without changing the strict state shape — internal/mission/state.go:310-356

4. The projection source is `artifacts/agents/missions/<mission>/usage.json`; if present, only its `units` array is copied. An absent file or non-array `units` leaves prior projected usage unchanged, while an unreadable existing file errors — internal/missionrunner/cycle.go:58-67

5. The mission aggregate currently emits `schemaVersion`, `missionId`, `units`, `unavailableJobs`, and `updatedAt`; `units` is sorted by provider then unit and contains `{provider,unit,value}` — internal/mission/fence.go:609-628

6. Mission unit names are `tokens.<token-field>`, `cost.<currency>`, and `provider.<name>`; provider is the record’s runtime or `unknown` — internal/mission/fence.go:577-603

7. Aggregation includes only terminal mission jobs and uses `jobId`, falling back to the record filename stem — internal/mission/fence.go:31-33; internal/mission/fence.go:554-571

8. `unavailableJobs` is the sorted list of terminal jobs with no object-valued usage or no measured value; `availability:"unavailable"` suppresses token fields, but valid cost or provider units still count as measured — internal/mission/fence.go:572-605; internal/mission/fence.go:623-626

9. No such mechanism exists: exact and case-insensitive `unavailableJobs` searches found only its writer; `ProjectFences` ignores it, so it never enters `state.json` — internal/mission/fence.go:624-628; internal/missionrunner/cycle.go:58-66

10. Typed per-round writers use seven fields: `availability`, four token fields, `cost`, and `providerUnits`; `cost` is null or `{amount,currency}`, and `providerUnits` is null or `{name,value}` — internal/adapter/adapter_codex.go:36-59; internal/adapter/adapter_claude.go:75-96; internal/adapter/adapter_devin.go:227-255; internal/adapter/adapter_devin.go:278-289

11. Real delegate adapters name the artifact `rounds/<n>/usage.json`; the fake adapter uses `fake-usage.json` — scripts/agents/adapters/codex.sh:131-136; scripts/agents/adapters/claude.sh:97-105; scripts/agents/adapters/devin.sh:259-269; scripts/agents/adapters/fake.sh:95-101

12. `WriteResultPatch` embeds any decoded regular usage file verbatim into the job record, uses null for an absent/nonregular path, and performs no exact-key validation of the nested value — internal/adapter/patch.go:29-48

13. The complete shipped production consumer set for job-record `usage` is mission aggregation, chain aggregation, and adapter self-test; each selects named fields and ignores additional nested keys — internal/mission/fence.go:572-605; internal/dispatch/usage.go:19-75; internal/adapter/adapter_selftest.go:30-58

14. Chain aggregation writes `{chainUsage:{tokens,cost,providerUnits}}`: tokens map runtime to the four token fields, cost maps currency to totals, and provider units map runtime then unit name to totals; there is no unavailable-job or per-round member — internal/dispatch/usage.go:8-12; internal/dispatch/usage.go:19-87

15. The only production read of `chainUsage` is `ChainUsage`’s whole-value equality check used to suppress an unchanged patch; tests and fixtures inspect its token, cost, and provider-unit members — internal/dispatch/usage.go:78-87; scripts/agents/dispatch.sh:681-694; internal/dispatch/decisions_test.go:87-114; scripts/validate-metasystem.sh:2251-2260

16. The only production consumer of the mission aggregate file is `ProjectFences`, which reads `units`; the Go and shell verification consumers also inspect only `units` — internal/missionrunner/cycle.go:58-66; internal/mission/fence_test.go:138-151; scripts/validate-metasystem.sh:2599-2604; scripts/validate-metasystem.sh:2721-2728

17. No such mechanism exists: exact and broadened searches for derived usage, event-stream provenance, and usage provenance found no shipped field or availability value; shipped availability values are `native` and `unavailable` — internal/adapter/adapter_codex.go:47-55; internal/adapter/adapter_devin.go:227-235; internal/adapter/adapter_devin.go:278-286

18. A concrete additive per-round location with zero current consumer changes is an extra key such as `provenance` inside each typed round usage object: embedding is verbatim and all job-usage consumers select named fields — internal/adapter/patch.go:39-46; internal/mission/fence.go:572-603; internal/dispatch/usage.go:30-74; internal/adapter/adapter_selftest.go:35-58

19. A second concrete additive location with zero current consumer changes is a top-level collection such as `rounds` in mission `usage.json`; the existing reader selects only `units` and already ignores `schemaVersion`, `missionId`, `unavailableJobs`, and `updatedAt` — internal/mission/fence.go:624-628; internal/missionrunner/cycle.go:58-66

20. No additive provenance field can be placed inside `state.fences` or its usage entries without changing strict consumers; both levels use exact-key checks, and the state top level rejects unknown keys — internal/mission/state.go:174-195; internal/mission/state.go:310-336

21. Host-turn `turns/<turn>/usage.json` is separate: `ResultWrite` embeds it into the five-field host result, the runner checks only those five outer keys, conclusion ignores nested host usage, and mission aggregation scans delegate job records instead — internal/host/result.go:5-24; internal/missionrunner/adjudicate.go:86-96; internal/missionrunner/turnio.go:67-106; internal/mission/fence.go:554-572

## Q5. Prompt section grammar

1. The assembler emits a records section as an exact heading line, `<<<DATA>>>`, tab-joined records, then `<<<END>>>`; zero records become the single content line `(none)` — internal/mission/prompt.go:228-247

2. Before joining, null/empty fields become literal `none`, booleans become `yes`/`no`, CR/LF/tab become spaces, and embedded fence markers are defanged — internal/mission/prompt.go:28-63

3. The shipped assembler emits exactly four records sections—`## Ledger Tail`, `## Open Asks`, `## Streams`, and `## Reconciliation`—between Mission Contract and This Turn, joined by `\n\n` with a final LF — internal/mission/prompt.go:503-520

4. The validator recognizes exactly six headings in fixed order: Mission Contract, Ledger Tail, Open Asks, Streams, Reconciliation, This Turn; mismatch produces “the six required headings are missing, duplicated, or out of order” — internal/validate/turnprompt.go:29-33; internal/validate/turnprompt.go:183-189

5. A heading is recognized only by exact whole-line equality and only outside a data fence — internal/validate/turnprompt.go:161-178; internal/validate/turnprompt.go:309-315

6. Exact whole-line `<<<DATA>>>` and `<<<END>>>` markers may not nest, be unmatched, or remain unclosed — internal/validate/turnprompt.go:161-181

7. Every recognized section except the last must end with exactly one blank separator line; otherwise the validator reports that it “is not separated from the next block by exactly one blank line” — internal/validate/turnprompt.go:191-210

8. A records body must be exactly one fixed fence with at least one content line; an empty fence reports “has an empty data fence; use (none)” — internal/validate/turnprompt.go:325-335

9. `(none)` is valid only as the fence’s sole content; mixing it with records fails — internal/validate/turnprompt.go:336-343

10. Every record is split only on tab characters, must contain the declared field-count range, and may contain no empty field; failure reports “must contain … non-empty tab-separated fields” — internal/validate/turnprompt.go:344-363

11. Declared counts are Ledger Tail four or five fields, Open Asks four, Streams four, and Reconciliation three — internal/validate/turnprompt.go:212-215; internal/validate/turnprompt.go:245-245; internal/validate/turnprompt.go:270-270; internal/validate/turnprompt.go:292-292

12. Ledger Tail additionally requires a positive decimal cycle, a known classification, SHA `none` or 40–64 lowercase hexadecimal characters, no `(none)` observed field, optional `yes|no`, and strictly increasing unique cycles — internal/validate/turnprompt.go:35-44; internal/validate/turnprompt.go:219-243

13. Open Asks requires valid IDs, stream ID or `none`, a known prompt reason, no `(none)` question, and sorted unique ask IDs — internal/validate/turnprompt.go:35-39; internal/validate/turnprompt.go:245-268

14. Known prompt reasons are `reserved-decision`, `red-test`, `merge-conflict`, `host-failure`, `fence`, `stop-loss`, and `drain-stalled` — internal/missionrunner/missionrunner.go:27-49

15. Streams requires a valid stream ID, one of four stream states, no `(none)` goal/reason, and sorted unique stream IDs — internal/validate/turnprompt.go:35-39; internal/validate/turnprompt.go:51-53; internal/validate/turnprompt.go:270-290

16. Reconciliation requires a valid turn ID and outcome/detail unequal to `(none)` — internal/validate/turnprompt.go:292-304

17. The assembler limits Ledger Tail to the configured last 1–50 cycles and emits at most one Reconciliation row; asks and streams enumerate their available records — internal/mission/prompt.go:90-127; internal/mission/prompt.go:192-225; internal/mission/prompt.go:441-451

18. No such mechanism exists: row-count searches and broadened `len(records|ledger|asks|streams|reconciliation|content)` searches found no validator maximum per records section; after the `(none)` case it validates every supplied line — internal/validate/turnprompt.go:332-363

19. No such mechanism exists: exact `Landed Returns` and broadened landed/unreadable-chain/chain-root/return-path searches found no shipped assembler block, recognized heading, parser call, semantic rule, or test for that section — internal/mission/prompt.go:503-515; internal/validate/turnprompt.go:29-33; internal/validate/turnprompt.go:212-304

20. A syntax-compatible unreadable-chain row for a newly declared three-field generic records parser could be `chain-id\tunreadable\tnone`, with `\t` representing actual tabs: all three fields are nonempty and `none` is not the section sentinel `(none)`; no shipped Landed Returns semantic rule assigns meanings to those fields — internal/validate/turnprompt.go:325-363

Codex session ID: 019ff0f6-7ceb-7f10-a19b-d04004053253
Resume in Codex: codex resume 019ff0f6-7ceb-7f10-a19b-d04004053253

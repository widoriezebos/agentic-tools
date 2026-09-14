# Independent read: coordinator-context slice 3, unit D1b (Stop allowance under a recorded handoff)

Reader: Fable 5.1, code-critique skill, two layers in order. Worktree: /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccb-d1b/metasystem. Subject: the uncommitted diff, three files, 189 insertions, 0 deletions (git diff --stat). Nothing committed, nothing edited except temporary mutations restored and checksum-verified (shasum before and after, identical: turnverdict.go 4579e4fd, turnverdict_test.go e719a07a, goal.go 114fdf09).

Materiality test applied to every finding: would the change ship a defect, violate its brief, or damage what certifies it?

Material finding count: 0.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | low | no | The command-wiring proof is a source-text assertion, not a wiring proof. It stays green when the closure is commented out, and cmd/metasystem still builds in that state. | internal/goal/turnverdict_test.go:273-286 reads cmd/metasystem/goal.go through runtime.Caller and checks strings.Contains for two lines. My mutation: replaced goal.go:710-712 with the same three lines prefixed by `//`, ran `go test -count=1 ./internal/goal/ -run 'TestTurnVerdictAllowsTheStopUnderARecordedHandoff/command_supplies_steward_lookup' -v`: PASS. `go build ./cmd/metasystem/`: ok. Restored, shasum identical. Runtime certification of the wiring rests on D2's supervision-hook fixture leg named in CCB-3-08 (design line 295). A cmd/metasystem test of runReportTurnVerdict with a staged live seatHandoff intent would close this; that file is outside D1b's boundary. |
| F-2 | low | no | No guard for a seam that returns live=true with an empty nonce. The display would read `handoff recorded: ; end this session`. | Mutation row 9 output: `Display:"handoff recorded: ; end this session"`. The landed steward cannot produce it: internal/steward/handoff_capture.go:1094 returns `found, found != "", nil`. Optional hardening only. |
| F-3 | low | no | The early return skips the per-session full-turn-verdict text artifact, while the stop report's "Existing records" section still names that file. On an allowance turn the file is absent or stale from the last blocking turn. | Skipped write: internal/goal/turnverdict.go:419-429 (inside the lock path). Report line: internal/report/stoppresentation.go:1258 names `artifacts/agents/supervision/stop-verdicts/<session>.txt`. No Go or shell reader of that file exists (grep turnVerdictArtifactPath and stop-verdicts across cmd/, internal/, scripts/). The closed-fence path (turnverdict.go:279-294) has the same shape; the brief told the builder to follow it. D2 must grep the report's "Original turn verdict and frozen judgment" JSON section (stoppresentation.go:1248), where `"display"` carries the sentence, not the .txt. |
| F-4 | low | no | The allowance turn skips verdict-state touch and save, so SurfaceWatchdog is false and this turn's watchdog digest is neither surfaced nor stored; brain status write and BrainStatusDue are also skipped. | turnverdict.go:333 (touch), :347 (watchdog), :349-362 (brain status), :365 (save) are all inside the lock path the allowance returns before. Hook consequence: supervision-hook.sh:1452 surfaces watchdog text only when surfaceWatchdog is true. Same as the fence path. A brain seat cannot hold a live handoff (the handoff verb refuses HANDOFF_NO_GOAL without a held goal), so the brain status skip is moot. |
| F-5 | low | no | A human stop marker for the session stays unspent when the handoff allowance wins. | inspectSessionStop (turnverdict.go:319) and consumeSessionStopForVerdict (:382) are inside the lock path. The test asserts HumanStopConsumed false as the brief requires (turnverdict_test.go:322-324). Bookkeeping only; the session ends. |
| F-6 | info | no | The design page still shows the two-result seam; the build follows the three-result seam from the v2 brief. | plans/coordinator-context-stays-under-budget-design.md:253 (D3-5): `HandoffRecorded func(session string) (string, bool)`. ccb-s3 codex-ccb-slice3-brief-v2.md:274: `(string, bool, error)`, "This corrects the old two-result seam". The D1b brief restates the three-result form. The design page wants one amendment line under 8c so the accepted design matches what landed. |
| F-7 | info | no | Race with a concurrent CancelHandoff: the seam reads without arbitration, so the outcome is either an allowance or no allowance, never a spurious error. An allowance can be granted for a handoff cancelled a moment later. | LiveHandoffForSession takes no AcquireArbitration (handoff_capture.go:1071-1095); readLiveHandoffIntents tolerates a vanished file (:880-883, os.IsNotExist continue); CancelIntent removes the live file first (intervene.go:99). The canceller is the seat itself (reason "cancelled by the seat", handoff_capture.go:1054), so the stale allowance is the seat's own act. Landed steward code, outside this unit. |
| F-8 | info | no | Out of scope, landed unit C: the seam verifies every live handoff's state file, including other sessions'. A foreign session's drifted state file turns this session's Stop into a non-blocking infrastructure verdict instead of the open-work refusal. | handoff_capture.go:1082-1085 returns the verify error before the session comparison at :1086. turnverdict.go:298-300 maps it to infrastructureVerdict (ShouldBlock false, class infrastructure, LedgerStatus degraded, turnverdict.go:528-535). The v2 brief accepts this ("An unreadable handoff store produces the ordinary infrastructure verdict", line 276) and the hook labels it degraded (supervision-hook.sh:1409-1418). Named for the seat; no change in this unit. |

## Layer 1: conformance to the brief

Every requirement checked against the diff.

- Seam on goal.TurnVerdictOptions as a function value: turnverdict.go:201. No steward import in internal/goal: `command grep steward internal/goal/*.go` finds none. Yes.
- Supplied from runReportTurnVerdict after the state root resolves: cmd/metasystem/goal.go:709-712, inside the `rootErr == nil` branch. Yes.
- Consulted after the checkout fence path and before the open-work rules: turnverdict.go:296 follows the fence block ending at :295 and precedes `s.withLock` at :318. Yes.
- Live same-session binding: SchemaVersion 1, class seat-actionable, ShouldBlock false, LedgerStatus ok, display exactly `handoff recorded: <nonce>; end this session`: turnverdict.go:302-309. Yes.
- Foreign session, cancelled, consumed, absent, nil seam give no allowance: only `live` authorizes (:301); nil seam skips the block (:296). Tests cover each (turnverdict_test.go:218-247, 265-271). Yes.
- Seam error is the ordinary infrastructure verdict: :298-300 with the new producer prefix at :525. Yes.
- Frozen facts through freezeTurnVerdictFacts with the same display, FullDisplay equal, empty action list, no fabricated ledger read (workRead false), no consumed human stop (false): :310-313. Test asserts each (turnverdict_test.go:288-327). Yes.
- Boundary: `git status --short` lists exactly the three files. 189 changed lines, under the 400 ceiling. No new file.
- Nothing else changed: the diff is additions only; the fence block (:268-295), the lock path and open-work rules, and the provider response (no script changed) are untouched. gofmt -l on the three files: empty. go vet on both packages: clean.
- Focused tests as instructed, METASYSTEM_BIN and METASYSTEM_CONTEXT_COST_PROOF unset: `go test -race -count=1 ./internal/goal/ -run 'TestTurnVerdict|TestHandoff'` ok 5.144s; `go test -race -count=1 ./cmd/metasystem/ -run 'TestReportTurnVerdict|TestTurnVerdict|TestStop'` ok 402.135s.
- Rules with no failing test: none found. Every rule in the diff has a row below that fails when removed. The one weak row is row 2 (F-1).

## Layer 2: adversarial questions asked by the seat

- Ordering against the checkout fence. A closed checkout with a live handoff gets the fence verdict (LedgerStatus stopped, display `<description>; run: <command>`), never the handoff sentence, and the seam is not even called (test `closed checkout keeps precedence` asserts lookedUp false). Both branches allow the stop; only the display differs. This matches D3-5 ("right after the fence check") and the v2 brief ("The checkout fence retains precedence"). Should it: yes. The fence's command is the seat's instruction on a closed checkout; the held intent stays staged and launches on observed death either way.
- Session identity. The hook normalizes once (supervision-hook.sh:554-565: safe shape or sha256 hex) and passes that value both to `up --session` (:936, the announcement the lease records, internal/lease/verbs.go:184) and to `report turn-verdict --session` (:1357). TurnVerdict normalizes again at turnverdict.go:250; idempotent, because a 64-char hex matches the safe shape. The seam receives that value (:297). LiveHandoffForSession normalizes once more (handoff_capture.go:1080) and compares with Handoff.Session, which the steward stored as goal.NormalizeSession(capture.authority.Session) (:835). Same function on both sides; consistent. The one link outside this diff: the handoff verb must fill HandoffCaller.Session with the announced SessionId. That verb is not in this worktree (main.go registers only context status and report), so it belongs to the verb unit.
- Empty nonce with live=true: F-2. Not reachable from the landed steward.
- Reachable on a turn that is not a Stop: no. The hook calls the verb only inside `if [[ "$event" == stop ]]` (supervision-hook.sh:1254-1485, call at :1356); the only production Go caller is goal.go:718. Fixture scripts call it directly, which is their purpose.
- Race with CancelHandoff: F-7.
- Display byte-for-byte. turnverdict.go:302 builds `"handoff recorded: " + nonce + "; end this session"`. Identical to design line 69, D3-5 (line 253), CCB-3-08 (line 295) and v2 brief line 276. The hook may prefix durable-wait lines (goal.go:723-743); the sentence remains one whole line. The stop report embeds the verdict JSON under "Original turn verdict and frozen judgment" (stoppresentation.go:1248), so `"display": "handoff recorded: <nonce>; end this session"` is greppable there; the two-line provider response will not carry it, and the v2 brief already tells D2 not to grep that.
- Side effects skipped by the early return: F-3, F-4, F-5. Idle notifications (RecordIdleIncident, RaiseIdleAlarm) fire only from enforceIdleBacklog on a refusal, so skipping them on an allowance is correct. LedgerStatus is not persisted by TurnVerdict anywhere; it rides the verdict JSON and the facts.

## Reproduced mutation table

Each row: the single mutation from the builder's table applied alone, the named test run with `go test -count=1 ./internal/goal/ -run <test>`, the file restored, the checksum verified identical. Line numbers are the current worktree's.

| Row | Rule | Reproduced | Observed line |
| --- | --- | --- | --- |
| 1 | seam field exists | yes | `internal/goal/turnverdict.go:295:13: options.HandoffRecorded undefined (type TurnVerdictOptions has no field or method HandoffRecorded)` (build failed) |
| 2 | command supplies the lookup | yes | `turnverdict_test.go:285: runReportTurnVerdict does not supply the state-root-scoped steward handoff lookup` (see F-1: a comment-out does not fail it) |
| 3 | closed checkout precedes lookup | yes | `turnverdict_test.go:213: closed-checkout precedence changed: lookedUp=true ... Display:"handoff recorded: handoff-nonce; end this session"` |
| 4 | lookup gets the current session | yes | `turnverdict_test.go:228: foreign handoff changed the current session: ... Display:"handoff recorded: foreign-nonce; end this session"` |
| 5 | nil seam means no allowance | yes | `panic: runtime error: invalid memory address or nil pointer dereference` |
| 6 | error becomes infrastructure verdict | yes | `turnverdict_test.go:261: unreadable handoff verdict = ... Class:"seat-actionable", ShouldBlock:true ... Display:"OPEN WORK (1)...` |
| 7 | producer prefix `handoff record` | yes | `turnverdict_test.go:261: ... CauseCode:"handoff-lookup", Component:"handoff-record" ... Display:"handoff lookup: fixture handoff read failed"` |
| 8 | only live authorizes, not a nonce | yes | `turnverdict_test.go:246: cancelled handoff changed the verdict: ... Display:"handoff recorded: cancelled-nonce; end this session"` |
| 9 | absent does not authorize | yes | `turnverdict_test.go:246: absent handoff changed the verdict: ... Display:"handoff recorded: ; end this session"` |
| 10 | live returns before open work | yes | `turnverdict_test.go:188: same-session handoff verdict = ... ShouldBlock:true ... Display:"OPEN WORK (1)...` |
| 11 | schema version 1 | yes | `turnverdict_test.go:188: ... SchemaVersion:0` |
| 12 | does not block | yes | `turnverdict_test.go:188: ... ShouldBlock:true` |
| 13 | class seat-actionable | yes | `turnverdict_test.go:188: ... Class:"infrastructure"` |
| 14 | ledger status ok | yes | `turnverdict_test.go:188: ... LedgerStatus:"degraded"` |
| 15 | exact display | yes | `turnverdict_test.go:188: ... Display:"handoff recorded: handoff-nonce; continue this session"` |
| 16 | fact owner populates the return | yes | `internal/goal/turnverdict.go:310:4: declared and not used: stamp` (build failed) |
| 17 | identity names the installation | yes | `turnverdict_test.go:309: handoff identity facts = goal.TurnFactsIdentity{Installation:"", Session:"handoff-session", MainId:"main-1", ...}` |
| 18 | FullDisplay equals the display | yes | `turnverdict_test.go:314: handoff verdict facts = ...` (FullDisplay empty) |
| 19 | scan preserved | yes | `turnverdict_test.go:317: handoff scan facts = goal.ScanResult{Open:[]goal.Item{}, ...}` |
| 20 | no fabricated ledger read | yes | `turnverdict_test.go:320: handoff allowance fabricated a ledger read: work=goal.TurnWorkFacts{ReadSucceeded:true, ...}` |
| 21 | no consumed human stop | yes | `turnverdict_test.go:323: handoff allowance consumed a human stop: ... HumanStopConsumed:true` |
| 22 | no action to continue this session | yes | `turnverdict_test.go:326: handoff allowance offered an action to continue this session: []goal.TurnAction{{Kind:"open-plan", TargetId:"handoff-plan", Instruction:"continue this session", ...}}` |

22 of 22 reproduce. Row line numbers differ from the builder's by the one-line shift the builder noted (the added `runtime` import); the messages match. After the table: `go test -count=1 ./internal/goal/ -run 'TestTurnVerdictAllowsTheStopUnderARecordedHandoff|TestHandoffAllowanceCarriesFrozenFacts'` ok.

## Verdict

Land. Zero material findings. Carry F-1 and F-3 into D2's brief: the fixture leg is the runtime proof of the command wiring, and it must grep the sentence from the stop report's judgment JSON, not the per-session .txt. Add one amendment line under 8c for the three-result seam (F-6).

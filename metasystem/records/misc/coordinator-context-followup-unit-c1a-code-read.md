VERDICT: send back

Material findings: 3 (F-1, F-2, F-3). All three are missing witnesses in metasystem/internal/steward/handoff_capture_test.go. The built code behaves correctly in every case probed. Conformance is clean: six files, all inside the Boundary; 359 additions and 29 deletions (388); no Non-goal body changed; no existing assertion changed. The seat's verification log is green.

### F-1. Severity medium. Material yes.

Claim: the A4 witness cannot detect the failure its row names. If the class and proof steps are judged before the live read, every C1a test still passes, and a consumed nonce then gets a human token instead of `HANDOFF_NOT_LIVE`, which D15-7 forbids.

Evidence:
- Mutant A4b inserts, after `defer arbitration.Release()` (handoff_capture.go:1118) and before the live read (:1119), a call to `handoffCancelReason` for any act whose class is not HUMAN or whose proof fails `TerminalValidFor(stateRoot)`. For live nonces its output is identical to the built code.
- Command, in a private copy: `go test -count=1 -timeout 40m ./internal/steward -run '^(TestCancelHandoffAdmitsAProvenHumanAct|TestCancelHandoffRefusesAnUnprovenHumanAct|TestCancelHandoffHumanActIsHolderBound|TestCancelHandoffHumanActRequiresNameAndIdentity)$'` gives `ok ... 9.757s`. The broad sweep `-run '(?i)(handoff|cancel)'` gives `ok ... 37.449s`.
- Probe, consumed nonce, `Caller: handoffMainCaller()`, `Human: &HandoffHumanAct{By: "Wido"}`. Built code: `HANDOFF_NOT_LIVE nonce=6400000000000001`. Under A4b: `HANDOFF_HUMAN_UNPROVEN nonce=6400000000000001 by=Wido caller=MAIN human=unattempted`.
- Cause: the `consumed nonce` subtest (its assertion is at handoff_capture_test.go:839) uses the valid base act, as page row A4 says. A valid act produces no token at steps 1 or 2, so this ordering fault cannot show. The builder's own A4 mutation (a human refusal returned unconditionally on the not-exist path) is killed, but that is a different fault.
- Artifact to change: the `consumed nonce` subtest should cancel with an act that step 1 or step 2 refuses, for example `Caller: handoffMainCaller()`, and still assert `HANDOFF_NOT_LIVE nonce=<n>`. That is about 2 numstat lines. It departs from the row's words "then the base act", so the seat rules on it.

### F-2. Severity low. Material yes.

Claim: four text rules of D15-4 have no failing test. They are the reason's `session=none`, `human-ticks=<n>` and `human-boot=<id>` fields, and the refusal's `caller=none` for an empty class. Each can be removed with all C1a tests green.

Evidence:
- Each mutant was run against the four new steward tests and then the broad `(?i)(handoff|cancel)` sweep. All runs passed:
  - R1: always `goal.NormalizeSession(act.HolderSession)`, the guard at handoff_capture.go:1092-1095 dropped. `ok 12.583s`, broad `ok 37.846s`.
  - R2: boot always empty (:1096). `ok 8.736s`, broad `ok 34.946s`.
  - R3: ticks printed as 0 (:1101). `ok 9.099s`, broad `ok 32.398s`.
  - R4: the `none` substitution at :1050-1052 dropped. `ok 8.581s`, broad `ok 35.780s`.
- Probe with `HolderSession: ""` and a proof from `identity.Exact{Pid: 4711, StartedAt: time.Unix(1700000000, 0), StartTicks: 5, BootID: "boot-fixture"}`:
  - Built code: `cancelled: cancelled by human by=Wido holder=main-1 epoch=3 session=none human-pid=4711 human-started=1700000000 human-ticks=5 human-boot=boot-fixture`.
  - Under R1+R2+R3: `... session=e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 human-pid=4711 human-started=1700000000 human-ticks=0 human-boot=none`.
- Probe with `HandoffCanceller{Human: &HandoffHumanAct{By: "Wido"}}`:
  - Built code: `HANDOFF_HUMAN_UNPROVEN nonce=6400000000000001 by=Wido caller=none human=unattempted`.
  - Under R4: `... caller= human=unattempted`.
- Why no test sees them: the only reason witness (`attended human`, handoff_capture_test.go:810) fixes `session / original`, ticks 0 and boot `none`. The only class witness (`recorder with --by`, :846) uses MAIN.
- `session=none` is the tombstone D15-7 names for the dead-recorder case the human leg exists for.
- Artifact to change: one more attended-human subtest (empty `HolderSession`, a proof with ticks and a boot id, an exact outcome), plus one `caller=none` refusal row. That is about 15 numstat lines.

### F-3. Severity medium. Material yes.

Claim: the malformed-lease refusal of D15-4 step 3 has no failing test. If `ReadHolderLease` decodes the lease without `currentSessionStopLease`'s validity check, all C1a tests and the goal tests pass, and a malformed lease authorizes a human cancellation. Row A21's statement "a changed body fails the goal package's own tests or a steward row" does not hold for this body.

Evidence:
- Mutant A21a replaces the call at sessionstop.go:443 (inside `ReadHolderLease`, :442-448) with `os.ReadFile(sessionStopLeasePath(root))` and an unchecked `json.Unmarshal` into `sessionStopLease`.
- Results: four new steward tests `ok 9.082s`. `go test -count=1 -timeout 40m ./internal/goal -run '(?i)(stop|verdict|lease)'` (62 tests) `ok 153.119s`. Broad steward sweep `ok 45.870s`.
- Probe with the lease `{"holderMainId": "main-1", "pid": 0, "pidStartedAt": 100, "claimEpoch": 3}`:
  - Built code: `handoff 6400000000000001 cannot be cancelled by a human act: the checkout holder lease cannot be proved: checkout lease is malformed`.
  - Under A21a: `<nil>`, and the handoff is cancelled.
- Coverage today: A9 (`absent lease`, :878) covers only a missing file. No goal test calls `ReadHolderLease`.
- Artifact to change: a `malformed lease` subtest in `TestCancelHandoffHumanActIsHolderBound` that writes a lease with pid 0 and uses `failedCancel` with the full text above. That is about 5 numstat lines.

### F-4. Severity low. Material no.

Claim: the nil-human constant `cancelled by the seat` (handoff_capture.go:1058) has no failing test, but the bytes are unchanged from base, and the part of A1 that C1a adds is proven.

Evidence:
- Mutant A1a changes the constant to `"cancelled by the seat (mutant)"`. The ten existing steward cancel tests, the four new ones, `go test ./cmd/metasystem -run '^TestContextVerifyAndCancel$'` and the broad sweep all pass.
- Base passed the same literal at the old call site, and it was untested there too: `grep -rn "cancelled by the seat"` over internal, cmd and scripts finds it only at :1058. The page gives its witness to B6 in C1b.
- A1's own rule, "no judgment of the caller on the nil branch", is proven. Mutant A1b (the nil branch refuses a zero caller) is killed: `context_verbs_test.go:699: cancel = code 9 stdout "" stderr "HANDOFF_NOT_HOLDER\n"`.
- Probe with `HandoffCanceller{}` writes `cancelled: cancelled by the seat`.

### F-5. Severity low. Material no.

Claim: a name with `key=value` words passes `ValidateHumanName` and makes the reason ambiguous. The format is exactly D15-4 step 8's, and nothing parses it.

Evidence:
- Probe with `By: "Wido holder=main-9 epoch=9"` records `cancelled: cancelled by human by=Wido holder=main-9 epoch=9 holder=main-1 epoch=3 session=4fde8990... human-pid=4711 ...`.
- A name with a newline is refused: `the human name must be at most 200 bytes and contain no control characters`.
- `session stop` keeps `By` in its own JSON field. No reader of the Outcome text exists: grep for `cancelled by human` finds only handoff_capture.go:1100 and its test.

### F-6. Severity low. Material no.

Claim: the builder's own return cannot certify the unit. The seat's run does.

Evidence:
- codex-ccb-c1a-result.md is still the first build's 401-line stop report.
- In the session record, the continuation's `go test -race` (ord 849) gave `ok internal/goal 1244.307s` and `ok internal/refusal 2.810s`. internal/steward failed on nested builds downloading golang.org/x/sys. cmd/metasystem failed on `httptest ... bind: operation not permitted`. go-gate --fast never ran, and the turn was aborted at ord 867.
- The seat's log ccb-c1a-verify.log shows: build-rc=0; vet-rc=0; `ok internal/goal 1345.831s`, `ok internal/steward 224.312s`, `ok internal/refusal 2.368s`, `ok cmd/metasystem 400.512s`, test-rc=0; `go gate: fast mode passed`, gate-rc=0.

## Table 1. The builder's evidence per row, as found in the session record

- Record: /Users/wido/.codex/sessions/2026/09/15/rollout-2026-09-15T00-15-11-01a0a1fd-1053-7de0-835c-dd882b8863da.jsonl.
- Commands: each mutation ran `go test -count=1 -timeout 40m ./internal/steward -run '^<Test>$/^<subtest>$'` (A20 used ./internal/refusal, A21 used ./internal/goal).
- Restores: each was checked with `shasum -a 256` against f9fa7870 (handoff_capture.go), d36a30a0 (sessionstop.go) or 40841fce (register.go).
- These final hashes equal the live worktree's hashes now.
- The builder applied its mutations to the live worktree between about 22:18:24Z (ord 124) and 22:26:32Z (ord 588). Its final restores match.
- Every row except A19 has a recorded mutation and failure line. A1 and A21 have only a crash or compile-failure line, and A2's run mutation is not the untrimmed-name removal.

| Row | Builder's mutation (ordinal) | Observed failure line in the record | Quality |
| --- | --- | --- | --- |
| A1 | `if false && canceller.Human == nil` (124) | `--- FAIL: TestCancelHandoffReportsItsExactPartialOutcome` with `panic: runtime error: invalid memory address or nil pointer dereference`; TestContextVerifyAndCancel panicked at context_verbs_test.go:697 (134) | Crash only; the reader's A1b is behavioural |
| A2 | `goal.ValidateHumanName(act.By + "!")` (157); the `act.By` edit at 150 was never run | `handoff_capture_test.go:810: outcome="cancelled: cancelled by human by=Wido ! ..." want="... by=Wido ..."` (167) | Indirect; the reader's A2 removes the trim use directly |
| A3 | `TerminalValidFor(root)` (183) | `handoff_capture_test.go:825: HANDOFF_HUMAN_UNPROVEN nonce=6400000000000001 by= Wido  caller=HUMAN human=other-root` (191) | Good |
| A4 | unconditional human refusal on the not-exist path (205) | `handoff_capture_test.go:839: consumed cancellation=HANDOFF_HUMAN_UNPROVEN ... human=unobserved` (213) | Partial, see F-1 |
| A5 | `if false && canceller.Caller.Class != handoffClassHuman` (227) | `handoff_capture_test.go:846: ... caller=MAIN human=unobserved want="... human=unattempted"` (235) | Good |
| A6 | outcome branch `false &&` (249) | `handoff_capture_test.go:852: ... human=unobserved want="... human=AGENT_IN_AUTHORITY_CHAIN"` (257) | Good |
| A7 | `if false && act.Proof.Valid()` (272) | `handoff_capture_test.go:859: ... human=unobserved want="... human=other-root"` (280) | Good |
| A8 | `token := "other-root"` (294) | `handoff_capture_test.go:865: ... human=other-root want="... human=unobserved"` (302) | Good |
| A9 | `if false && err != nil` after ReadHolderLease (316) | `handoff_capture_test.go:878: ... the supplied holder coordinates do not match the current checkout lease want="... the checkout holder lease cannot be proved: "` (326) | Good; malformed branch unwitnessed, see F-3 |
| A10 | holder term dropped (340) | `handoff_capture_test.go:891: ... it was recorded by main-1 and the current holder is main-2 want="... do not match the current checkout lease"` (348) | Good |
| A11 | epoch term dropped (362) | `handoff_capture_test.go:891: cancellation error=<nil> want="... do not match the current checkout lease"` (370) | Good |
| A12 | `if false && intent.Handoff.MainId != lease.HolderMainId` (384) | `handoff_capture_test.go:899: cancellation error=<nil> want="... it was recorded by main-1 and the current holder is main-2"` (392) | Good |
| A13 | `if false && by == ""` (410) | `handoff_capture_test.go:912: cancellation error=<nil> want="... a non-blank human name is required"` (418) | Good |
| A14 | `len(by) > 200` dropped (432) | `handoff_capture_test.go:912: cancellation error=<nil> want="... at most 200 bytes and contain no control characters"` (440) | Good |
| A15 | control predicate forced false (461) | `handoff_capture_test.go:912: cancellation error=<nil> want="... no control characters"` (469) | Good |
| A16 | `ref.StartedAtSec < 1` dropped (483) | `handoff_capture_test.go:919: cancellation error=<nil> want="... the attended-human process identity is invalid"` (491) | Good |
| A17 | pid term inserted before the proof check (505) | `handoff_capture_test.go:925: cancellation refusal=handoff ... the attended-human process identity is invalid want="HANDOFF_HUMAN_UNPROVEN ... human=unobserved"` (513) | Good |
| A18 | Mode term dropped (527) | `handoff_capture_test.go:931: cancellation error=<nil> want="... the attended-human process identity is invalid"` (535) | Good |
| A19 | moved to C1b by the page's overflow rule; `TestHCL03HandoffCancelRows` removed (44), numstat 359/29 (51) | not applicable | Expected |
| A20 | HANDOFF_HUMAN_UNPROVEN Site 1053 to 1052 (549) | `register_test.go:84: row HANDOFF_HUMAN_UNPROVEN site "handoff_capture.go:1052" is not an emission line` (557) | Good |
| A21 | ValidateHumanName deleted (571) | `internal/goal/sessionstop.go:123:13: undefined: ValidateHumanName` [build failed] (581) | Compile failure only; see F-3 |

## Table 2. The mutations the reader ran

No mutation was applied to the live worktree. Where and when the reader's work ran:

- Copy: all mutations ran in a private rsync copy of the worktree's metasystem module, scratchpad/opus-c1a-copy, made at 22:57:48Z. Its six changed files matched the live sha256 before any mutation.
- Probe copy: a second private copy, about 22:58Z to 23:00Z, ran a no-mutation probe of the built code and was then deleted.
- Driver: the mutation driver (scratchpad/opus-c1a-mutate.py, log scratchpad/opus-c1a-mutations-run.log) ran from 23:01:47Z to 23:27:19Z. The mutant probes ran about 23:27Z to 23:29Z.
- Isolation: GOCACHE=/tmp/opus-c1a-gocache, GOTMPDIR=/tmp/opus-c1a-tmp, METASYSTEM_BIN unset.
- Live worktree: its six files keep mtimes from 22:09:51Z to 22:26:32Z, the builder's last restore. Their hashes and numstat were unchanged at the end of this read.
- Restores: every restore in the copy matched its sha256: handoff_capture.go f9fa78705ac5dbcd, sessionstop.go d36a30a09342ad14, register.go 40841fcea442794a.

Suites, all `go test -count=1 -timeout 40m` in the copy:
- STN: `./internal/steward -run '^(TestCancelHandoffAdmitsAProvenHumanAct|TestCancelHandoffRefusesAnUnprovenHumanAct|TestCancelHandoffHumanActIsHolderBound|TestCancelHandoffHumanActRequiresNameAndIdentity)$'`.
- STO: `./internal/steward` over the ten existing cancel tests of section 4.
- GO: `./internal/goal -run '(?i)(stop|verdict|lease)'`, 62 tests.
- RF: `./internal/refusal -run '^TestHCL03'`.
- CM: `./cmd/metasystem -run '^TestContextVerifyAndCancel$'`.
- BROAD: `./internal/steward -run '(?i)(handoff|cancel)'`, run only when a mutant passed its suites.

| Id | Row | Mutation | Suites | Observed | Result |
| --- | --- | --- | --- | --- | --- |
| BASE | none | unmutated copy | STN, STO, GO, RF, CM | all `ok` (goal 141.411s, refusal includes TestHCL03EveryCodeRowed) | green |
| A1a | A1, D15-2 | :1058 constant to `"cancelled by the seat (mutant)"` | STO, STN, CM, BROAD | all `ok` | survived (F-4) |
| A1b | A1 | nil branch refuses a zero caller with `HANDOFF_NOT_HOLDER` | STO, CM | `context_verbs_test.go:699: cancel = code 9 stdout "" stderr "HANDOFF_NOT_HOLDER\n"` | killed |
| A2 | A2 | reason prints untrimmed `act.By` (:1101) | STN | `handoff_capture_test.go:810: outcome="cancelled: cancelled by human by= Wido  holder=main-1 ..." want="... by=Wido holder=main-1 ..."` | killed |
| A3 | A3 | `TerminalValidFor(root)` (:1064) | STN | `handoff_capture_test.go:825: HANDOFF_HUMAN_UNPROVEN nonce=6400000000000001 by= Wido  caller=HUMAN human=other-root` (attended_human and absent_lease also fail on this host) | killed |
| A4 | A4 | builder's form, human refusal on the not-exist branch (:1121) | STN | `handoff_capture_test.go:839: consumed cancellation=HANDOFF_HUMAN_UNPROVEN nonce=6400000000000001 by= Wido  caller=HUMAN human=unobserved` | killed |
| A4b | A4, D15-7 | class and proof judged before the live read (after :1118) | STN, BROAD | all `ok`; probe `HANDOFF_HUMAN_UNPROVEN nonce=6400000000000001 by=Wido caller=MAIN human=unattempted` for a consumed nonce | survived (F-1) |
| A5 | A5 | `false &&` on the class check (:1061) | STN | `handoff_capture_test.go:846: cancellation refusal=HANDOFF_HUMAN_UNPROVEN nonce=6400000000000001 by=Wido caller=MAIN human=unobserved want="... human=unattempted"` | killed |
| A6 | A6 | outcome branch `false` (:1068) | STN | `handoff_capture_test.go:852: ... human=unobserved want="... human=AGENT_IN_AUTHORITY_CHAIN"` | killed |
| A7 | A7 | `Valid()` branch `false` (:1066) | STN | `handoff_capture_test.go:859: ... human=unobserved want="... human=other-root"` | killed |
| A8 | A8 | default token `""` (:1065) | STN | `handoff_capture_test.go:865: ... human= want="... human=unobserved"` (and :925 missing_pid) | killed |
| A6to8 | A4 to A8 | whole proof check off (:1064) | STN | `handoff_capture_test.go:859: cancellation refusal=<nil> want="... human=other-root"`; refused_walk and zero_proof fall to the identity error | killed |
| A9 | A9 | `false && err != nil` after ReadHolderLease (:1074) | STN | `handoff_capture_test.go:878: cancellation error=... the supplied holder coordinates do not match the current checkout lease want="... the checkout holder lease cannot be proved: "` | killed |
| A10 | A10 | holder term dropped (:1077) | STN | `handoff_capture_test.go:891: cancellation error=... it was recorded by main-1 and the current holder is main-2 want="... do not match the current checkout lease"` | killed |
| A11 | A11 | epoch term dropped (:1077) | STN | `handoff_capture_test.go:891: cancellation error=<nil> want="... do not match the current checkout lease"` | killed |
| A12 | A12 | binding check `false` (:1080) | STN | `handoff_capture_test.go:899: cancellation error=<nil> want="... it was recorded by main-1 and the current holder is main-2"` | killed |
| A13 | A13 | sessionstop.go:110 `if by == ""` to `if false` | STN, GO | `handoff_capture_test.go:912: cancellation error=<nil> want="... a non-blank human name is required"`; GO `ok 142.045s` | killed |
| A14 | A14 | sessionstop.go:113 `len(by) > 200 ||` dropped | STN, GO | `handoff_capture_test.go:912: cancellation error=<nil> want="... at most 200 bytes and contain no control characters"`; GO `ok 156.606s` | killed |
| A15 | A15 | sessionstop.go:113 control predicate `< -1` | STN, GO | `handoff_capture_test.go:912: cancellation error=<nil> want="... no control characters"` (control_character); GO `ok 138.957s` | killed |
| A16 | A16 | `ref.StartedAtSec < 1 ||` dropped (:1089) | STN | `handoff_capture_test.go:919: cancellation error=<nil> want="... the attended-human process identity is invalid"` | killed |
| A17 | A17 | pid term inserted before the proof check | STN | `handoff_capture_test.go:925: cancellation refusal=handoff 6400000000000001 cannot be cancelled by a human act: the attended-human process identity is invalid want="HANDOFF_HUMAN_UNPROVEN ... human=unobserved"` | killed |
| A18 | A18 | `|| ref.Mode() == identity.CompareInvalid` dropped (:1089) | STN | `handoff_capture_test.go:931: cancellation error=<nil> want="... the attended-human process identity is invalid"` | killed |
| A20a | A20 | register.go:47 Site 1053 to 1052 | RF | `register_test.go:84: row HANDOFF_HUMAN_UNPROVEN site "handoff_capture.go:1052" is not an emission line` | killed |
| A20b | A20 | register.go:58 HANDOFF_WAIT_IN_FLIGHT Site 415 to 396 (its base line) | RF | `register_test.go:84: row HANDOFF_WAIT_IN_FLIGHT site "handoff_capture.go:396" is not an emission line` | killed |
| A21a | A21, D15-4 step 3 | ReadHolderLease without the malformed-lease check (sessionstop.go:443) | STN, GO, BROAD | all `ok` (GO 153.119s); probe with a pid-0 lease gives `<nil>` | survived (F-3) |
| A21b | A21 | ValidateHumanName without TrimSpace (sessionstop.go:109) | STN, GO | `handoff_capture_test.go:810: outcome="... by= Wido  ..."` and `:912 ... want="... a non-blank human name is required"`; GO `ok 168.182s` | killed by steward rows only |
| R1 | D15-4 step 8 | `session=none` guard dropped (:1092-1095) | STN, BROAD | all `ok` | survived (F-2) |
| R2 | D15-4 step 8 | boot always empty (:1096) | STN, BROAD | all `ok` | survived (F-2) |
| R3 | D15-4 step 8 | ticks printed as 0 (:1101) | STN, BROAD | all `ok` | survived (F-2) |
| R4 | D15-4 refusal | `caller=none` substitution dropped (:1050-1052) | STN, BROAD | all `ok`; probe `HANDOFF_HUMAN_UNPROVEN nonce=6400000000000001 by=Wido caller= human=unattempted` | survived (F-2) |

## Checks that found nothing

Conformance:
- Scope: the six changed files are exactly the Boundary, with no untracked files, and 359 plus 29 is 388, under the 400 ceiling.
- Non-goal bodies: byte-identical to ddab438d, compared by extracting each body from base and worktree:
  - handoff_capture.go: `admitHandoffCaller`, `activeDelegateCaller`, `HandoffCaller`, `HandoffResult`, `Handoff` (with its supersession path), `cancelHandoffUnderLock`, `readExactLiveIntent`, `LiveHandoffForSession`, `VerifyHandoffState` and `refusal`.
  - Other steward files: `CancelIntent`, `HandoffBinding`, `decideForHandoff`, `StageHandoffIntent` and `PruneContext`.
  - sessionstop.go: `WriteSessionStop`, `currentSessionStopLease`, `sessionStopLease` and `inspectSessionStop`.
- Untouched paths: nothing under internal/lease, internal/humanauthority, internal/identity, session_stop.go, main.go, scripts, docs or context_verbs_test.go changed.
- Existing tests: the eleven existing steward call sites change only their argument, and no assertion changed.
- Verb: context_verbs.go changes only the call at :163, as D15-6 says for C1a.
- Spec match: the types, signature, helper names and every text match D15-1, D15-2 and D15-4.

The human leg, probed on the built code in a private copy:
- An agent-classified caller with a valid proof is refused: `HANDOFF_HUMAN_UNPROVEN nonce=6400000000000001 by= Wido  caller=DELEGATE human=unattempted`, and the record stays live.
- A blank name from a MAIN caller is refused `unattempted` before name validation, as D15-4 orders it.
- A whitespace name from a proven human fails with the plain name error (A13).
- A stale holder, a moved epoch, a foreign nonce, an absent lease and a malformed lease are all refused (A9 to A12, F-3 probe).
- A proof for another checkout gives `other-root` (A7).
- A consumed nonce gives `HANDOFF_NOT_LIVE`.

Arbitration: the lease is read once, at :1073 inside `handoffCancelReason`. That function is called at :1129, after `AcquireArbitration` (:1114) and the live read (:1119) and before `cancelHandoffUnderLock` (:1133), under the same arbitration. The lease is not read before arbitration.

Nil caller: the nil branch returns `cancelled by the seat` (:1058), the literal base passed, and the verb passes `HandoffCanceller{}`.

Register:
- New row: register.go:47 is `{Code: "HANDOFF_HUMAN_UNPROVEN", Owner: "internal/steward", Site: "handoff_capture.go:1053", Shape: Identity}`. It has no Override, zero Commands, and sits in alphabetical order, as D15-10 requires.
- Its Site: handoff_capture.go:1053 is its only `refusal(...)` call.
- Existing rows: each of the eleven `Site` lines names the first emission line of its code. Each moved by exactly the lines inserted above it: +19 above :1048, +75 for `HANDOFF_NOT_LIVE`, +79 below `CancelHandoff`.
- Tests: the TestHCL03 suite, including TestHCL03EveryCodeRowed, is green.
- A19: absent, as the overflow rule says.

Fold size: folding F-1 to F-3 as separate subtests adds about 22 numstat lines. The unit has 12 lines of headroom (388 of 400), so the fold must be compact, or the seat applies the page's overflow rule.

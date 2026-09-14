# Opus read: coordinator-context slice 3, unit D1a (context handoff, verify, prune)

Reader: Claude Fable 5.1, independent of the builder. Date: 2026-09-14.
Subject: the uncommitted diff in /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccb-d1a/metasystem, three files, 399 insertions, 0 deletions (`git diff --stat`). Baseline shasums: context_verbs.go fa7acf079bb1c82584d82b14427c7cfbc03df121, context_verbs_test.go bbde175c565c05bcbcc0b8074668267051b55f95, main.go d8ac2f06494c30693df052acb2e2e85f78676979. Every mutation below was restored and the three shasums re-checked equal to the baseline at the end of each sweep and at the end of this read. Nothing was committed. No fixture bed was run. METASYSTEM_BIN and METASYSTEM_CONTEXT_COST_PROOF were unset in every shell.
Authorities read: artifacts/reports/codex-ccb-slice3-d1a-brief.md; artifacts/reports/codex-ccb-slice3-d1a-result.md; plans/coordinator-context-stays-under-budget-design.md section 8c (D3-1 to D3-7 at lines 245 to 257, rows CCB-3-09 and CCB-3-10 at lines 296 to 297); the D section of the ccb-s3 worktree's artifacts/reports/codex-ccb-slice3-brief-v2.md (lines 258 to 300); internal/steward/handoff_capture.go and handoff_retention.go; internal/lease/classify.go, verbs.go, hook_delegate.go; internal/steward/context.go.
Scratch evidence: mutate.sh and mutations.log (first sweep, 45 rows), mutate2.sh and mutations2.log (anchored re-runs and the delegate proof), ms-clean (a binary built from the unmutated tree before any sweep) and fx/ (the empty probe root), all under this scratchpad.

## Subject drift after this read (read this first)

This read, its mutation tables and its verdict cover the diff at the baseline shasums above (399 insertions). After my last verified restoration the worktree changed under a different hand: context_verbs.go was rewritten at 19:15:18 (now d5f52bc2d27c39e874aabe8d22bb21a944211873) and context_verbs_test.go at 19:15:40 (8b840e4b1ece452b9581a140daa9a833b9be6f66), then the test file changed again during my read-only re-run (abcb0d9e53d7ab0225c8c3c4cbd2ae29a4f67c54, unread at the time of writing). main.go is unchanged. I ran nothing but reads and `go test` after the last restoration, so these are a builder or seat follow-up, and I did not touch or restore them.

The delta from my baseline to d5f52bc2 plus 8b840e4b is exactly an F-1 correction: a third seam `hookContextHandoffDelegate = lease.HookDelegate`; in the DELEGATE branch, `ConsumedActiveJob` then `HookDelegate(stateRoot, stateRoot, jobID, ppid)`, and `caller.JobId = jobID` only when `Delegate && JobID == jobID`, otherwise JobId stays empty and the steward refuses; a "wrong delegate" case in TestContextHandoffVerb (classification pid 9999, hook seam answering Delegate false, asserting exit 9 and `HANDOFF_NOT_HOLDER`); the helper's seam save and cleanup extended. Read-only check of that state: `go vet ./cmd/metasystem/` exit 0; the three named tests PASS without race (0.88 s, 0.63 s, 0.37 s) and with race (3.4 s). The diff is now 424 insertions, past the 400 ceiling. Two read-only remarks on the delta, not a critique round: a `HookDelegate` error (for example an ancestor pid it cannot authenticate) exits 1, not 9, which matches "operational errors 1"; and the unit test proves the verb honours the seam's verdict, not HookDelegate itself, which is the brief's allowed seam shape. The test file then settled at 2e8ce2ff7aa0f80dad97089fd69526040e28ad0b (19:17:49; diff 421 insertions). That state adds `TestContextVerbUsage`, a table of sixteen exit-2 vectors that includes all seven F-2 rules (verify without nonce or root, handoff without root, positional arguments on handoff and prune, prune without root, unknown scratch key) plus the already-tested cancel, scratch and duration rejections, and a `captureContextVerb` helper that shortens the existing tests. Read-only check with the tree stable across the run (shasums identical before and after, 19:18:33 to 19:18:41): `go vet` exit 0; `go test -race` on TestContextHandoffVerb, TestContextVerifyAndCancel, TestContextPruneVerb and TestContextVerbUsage all PASS (3.7 s). So at d5f52bc2 plus 2e8ce2ff both material findings have a correction and a test in place. The tree moved twice more and settled: context_verbs.go 63b1f9bb760f97eea85e67625301ced04d7cafae, context_verbs_test.go 3c6a1054a9229dd1f86dd2344db5092757bc7b26, main.go unchanged, 398 insertions (back under the ceiling: the JSON struct became a four-key `map[string]string`, and the delegate block was compacted to `if err == nil && delegate.Delegate && delegate.JobID == jobID { caller.JobId = jobID }; return caller, err`). The seat's fold brief (artifacts/reports/codex-ccb-slice3-d1a-fold-brief.md, 19:11) and read-1 (my interim message) explain the hand: this is the Codex fold of F-1 and F-2. At 19:19:52 I copied the whole module to a scratch directory at exactly that state, and at 19:21:52 the live tree still had the same three shasums, so the second-round rows below, run in the copy, are attributable to the current live tree. I mutated nothing in the worktree after my baseline sweeps. The verdict line at the end applies to the baseline; the second-round section gives the rows the seat needs to adjudicate the fold.

## Findings

Materiality test applied to each: would the change ship a defect, violate its brief, or damage what certifies it?

### F-1. Any DELEGATE caller is admitted as the active steward continuation

Severity: medium. Material: yes.

Claim. The verb never binds a delegate caller to the job it names. For every DELEGATE classification it sets `caller.JobId` to whatever `steward.ConsumedActiveJob` returns, so the steward's own check `caller.JobId != jobID` (the "wrong delegate" refusal) is satisfied by construction. The design's rule at D3-3, "class DELEGATE whose job id is ConsumedActiveJob" (design line 249), is asserted, not established.

Evidence.
- cmd/metasystem/context_verbs.go:201-203: `if classified.Class == lease.ClassDelegate { caller.JobId, _, err = steward.ConsumedActiveJob(stateRoot); return caller, err }`.
- internal/lease/classify.go:419: a delegate classifies as `Classification{Class: ClassDelegate, Pid: current}` with no JobId; only ADAPTER-SUPERVISOR carries one (classify.go:437).
- internal/steward/handoff_capture.go, `activeDelegateCaller`: refuses `HANDOFF_NOT_HOLDER` only when `caller.JobId == "" || caller.JobId != jobID`; then takes the predecessor identity from the job record, never from the caller. So the caller's own process is compared with nothing.
- Proof by test mutation (mutations2.log): row D01 changed only the test seam's delegate pid from 4343 to 9999 (context_verbs_test.go:672); TestContextHandoffVerb still passed. Row D02 changed only the job record's custodian to `"pid": 7777, "pidStartedAt": 555` (context_verbs_test.go:667); the test still passed. A delegate that provably is not the continuation's process is admitted and a handoff is recorded in the continuation's name.
- Command: `bash mutate2.sh` (perl single-rule edits, `go test -count=1 ./cmd/metasystem/ -run '^TestContextHandoffVerb$'`, restore, shasum).

Consequence. An implementer or critic job under the same checkout that runs `metasystem context handoff --root <installation>` while a continuation is active stages a `seatHandoff` intent bound to the continuation's custodian identity. When that custodian dies, a successor launches that no holder requested. It cannot supersede the seat's own live handoff, because a different session refuses HANDOFF_OTHER_PENDING. The damage is a spurious launch authorization, not data loss.

Fix inside the boundary. `lease.HookDelegate(stateRoot, installationRoot, jobID, callerPID)` (internal/lease/hook_delegate.go:54) walks the caller's ancestry against the named job's recorded pid, pidStartedAt, ticks and boot id; the Stop path already uses it (cmd/metasystem/proof_run.go:385 and :484). In `contextHandoffCaller`, after `ConsumedActiveJob` returns `(jobID, true)`, call `HookDelegate(stateRoot, stateRoot, jobID, int64(os.Getppid()))` through a package seam beside `classifyContextHandoffCaller`, and set `caller.JobId = jobID` only when `result.Delegate && result.JobID == jobID`; otherwise leave it empty so the steward refuses. Add the D01 case as a real test (a delegate whose pid does not match refuses with exit 9). This is roughly 8 code lines and 10 test lines and takes the diff past the 400-line ceiling; the seat decides the ceiling.

### F-2. Seven rules in the diff have no failing test and no mutation row

Severity: low. Material: yes (the brief's proof rule: "for every rule you add, a test that fails when that rule alone is removed ... A rule without such a row is not done").

Claim. The usage paths the brief maps to exit 2, and the scratch key whitelist, survive removal.

Evidence (mutations.log, each row: perl edit, `go test -count=1 ./cmd/metasystem/ -run '^<Test>$'`, restore, shasum):
- X04 verify without `--nonce` (context_verbs.go:248, drop `*nonce == ""`): SURVIVED.
- X05 verify without `--root` (same line, drop `*root == ""`): SURVIVED.
- X06 handoff without `--root` (context_verbs.go:154): SURVIVED.
- X07 handoff with a positional argument (same line, drop `flags.NArg() != 0`): SURVIVED.
- X08 prune with a positional argument (context_verbs.go:272): SURVIVED.
- X09 prune without `--root` (same line): SURVIVED.
- X16 scratch key whitelist (context_verbs.go:226, replace the key check with `false` so `foo=bar` is accepted): SURVIVED.
The sibling status verb does test its usage line (context_verbs_test.go:208 and :338), so the convention exists. Behavior is correct today: on the clean binary `context verify --root R` and `context verify --nonce N` both exit 2 with `usage: metasystem context verify --root ROOT --nonce NONCE`; positional arguments exit 2 for all three verbs; `context prune` with no root exits 2. This is a proof gap, not a behavior defect.

Fix. One table-driven loop over the seven argument vectors asserting exit 2 and the usage prefix, about 10 test lines, plus the seven rows in the table.

### F-3. The --json intentPath is built from the unresolved root and hardcodes the steward's layout

Severity: low. Material: no.

Claim. `intentPath` joins the caller's `--root` (after ResolveStateRoot, no symlink resolution) with `artifacts/agents/steward/intents/<nonce>.json` (context_verbs.go:183-184), while `statePath` comes from the steward, which resolves symlinks (handoff_capture.go `handoffCanonicalRoot`, handoff_state.go:154 `canonicalExistingPath`). Row R14's captured output shows `"statePath":"/private/var/folders/..."` beside `"intentPath":"/var/folders/..."` in one object. Both resolve to the same file, so nothing is wrong at runtime, and the projection stays bounded (four scalar fields, no state body; the test asserts `len == 4` and no `disposable` text, context_verbs_test.go:681-682). The command layer now knows where the steward keeps intents; the steward exports `BriefPath` (stage.go:23) but no intent path helper. Record for a later steward unit: export the intent path, or derive the root from `result.StatePath`.

### F-4. Four brief rules are tested but missing from the builder's mutation table

Severity: low. Material: no (the proofs exist; only the return's table is incomplete).

- X01 `--cancel` rejects `--scratch` (context_verbs.go:154): KILLED, `context_verbs_test.go:736: invalid cancellation = code 9 stderr "HANDOFF_NOT_LIVE nonce=<n>"`.
- X02 verify resolves the root through goal.ResolveStateRoot (context_verbs.go:252): KILLED, `context_verbs_test.go:707: verify = code 9 ... HANDOFF_NOT_FOUND`.
- X03 prune resolves the root through goal.ResolveStateRoot (context_verbs.go:276): KILLED, `context_verbs_test.go:754: prune = code 0 stdout "pruned call-sessions=0\n"` (the handoff line vanished).
- X10 the current holder's main id is projected into the caller (context_verbs.go:213): KILLED, `context_verbs_test.go:655: text handoff = code 9 ... HANDOFF_NOT_HOLDER`.

### F-5. Cancellation needs no caller identity

Severity: medium. Material: no for this unit (out of scope; the verb conforms to its brief).

`context handoff --cancel NONCE` calls `steward.CancelHandoff(stateRoot, nonce)` (context_verbs.go:163), whose landed signature carries no caller (handoff_capture.go:1031). Any process that can run the binary against the root, a delegate or a person's shell, cancels the seat's live handoff. Design D3-4 (line 251) describes cancel as the seat's act ("cancelled by the seat"). The D1a brief says "calls steward.CancelHandoff" and forbids steward changes, so this is a design and unit C gap for the seat to carry to the page, not a correction for this build.

### F-6. Identity resolution follows the design, not the brief's phrase (no defect)

Severity: info. Material: no.

The D1a brief says "the same identity the context status verb resolves today". Context status resolves the lease holder's announcement regardless of who calls (internal/steward/context.go `resolveContextIdentity`). The verb instead classifies the caller with `lease.ClassifyAt(stateRoot, stateRoot, ppid)` and reads `lease.CurrentHolder` for `HolderMainId` (context_verbs.go:192-215), which is D3-3's rule. This is the right reading: reusing status's resolution would let an advisor main hand off in the holder's name. Passing `stateRoot` as both roots matches how status calls `ContextBudgetLine(stateRoot, stateRoot, ...)`; the sibling helper `classifyVerbCaller` (process_verbs.go:35) resolves the installation from the executing engine instead, and the two agree for the self-hosted and template layouts ResolveStateRoot produces. The `os.Getppid()` caller matches the convention of every other classifying verb in cmd/metasystem. The mutation X11 (installation argument changed to the container) is KILLED by the test seam's root check (context_verbs_test.go:838).

### F-7. Prerequisites the verb imposes on the D2 fixture legs (no blocker)

Severity: info. Material: no.

- The verb passes `<stateRoot>/memory/receipts.log`; the steward refuses with `handoff receipt file must be ...` (exit 1) when `memory/` does not exist, because it canonicalizes the parent. A fixture installation must stage `memory/`. Row X12 shows the exact message.
- `steward.Handoff` resolves the continuation roster from `metasystem.conf` and reads the role, requirements, schema and permissions files (the test stages them at context_verbs_test.go:805-811). A fixture must stage the same.
- `ClassifyAt` reads fixture probes from `<installation>/metasystem.conf`, so a fixture-granted announced main works; the test seams never exercise real classification, which the brief allowed.
- `context verify` on a drifted file prints `HANDOFF_STATE_MISMATCH expected=<d> found=<f> detail=<text>`; the design's text (D3-6) names two tokens. A fixture should match the prefix, not the whole line.
- The verb takes no lock of its own. Handoff, cancel, verify and prune each take the steward arbitration lock inside the steward; `LiveHandoffForSession` takes none. No new contention with the Stop path.

### F-8. Misleading message on a bad root (exit code right)

Severity: info. Material: no.

`ms-clean context handoff --root /nonexistent/place` exits 1 with "no machine nickname is enrolled", because classification tolerates a missing root and `goal.ResolveMachine` fails first (context_verbs.go:192-199). The exit code is the brief's operational 1. Cosmetic.

### F-9. Race run time

Severity: info. Material: no.

`go test -race -count=1 ./cmd/metasystem/ -run 'TestContext'` passed in 441.8 s on this machine against the builder's 8.3 s. The three new tests take 0.80 s, 0.62 s and 0.37 s without race. The time sits in the pre-existing TestContext* tests. Per-test race timing is in the appendix.

## Layer 1 conformance summary

- Boundary: `git status --short` lists exactly cmd/metasystem/context_verbs.go, context_verbs_test.go, main.go. 399 insertions, 0 deletions.
- Non-goals: no change under internal/, scripts/, plans/. The verb writes no file, store or lock of its own; the lock files that appeared under the probe root (context/maintenance.lock, context/sessions.jsonl.lock, steward/arbitration.flock) come from the landed usage and steward code. The 14d default is D3-1's window; no retention or expiry rule was added; the Stop surface and provider response are untouched.
- Handlers registered under the `context` family (main.go:434-436); rows R01 to R03.
- Flag syntax, output lines, exit mapping 0/1/2/9, root resolution, caller projection, cancel rules, prune duration forms, partial-error counts, bounded JSON: every rule has a killing test except the seven in F-2.
- Every one of the builder's 29 mutation rows reproduces (table below).

## Edge probes on the clean binary (ms-clean, empty template root fx/)

| Case | Exit | First line |
| --- | --- | --- |
| verify without --nonce | 2 | usage: metasystem context verify --root ROOT --nonce NONCE |
| verify without --root | 2 | same usage line |
| verify unknown nonce | 9 | HANDOFF_NOT_FOUND nonce=abcdefabcdefabcd |
| verify malformed nonce `zzz` | 9 | HANDOFF_NOT_FOUND nonce=zzz |
| verify positional arg | 2 | usage line |
| handoff --json --cancel N | 9 | HANDOFF_NOT_LIVE nonce=0000000000000000 (text, not JSON; brief silent) |
| handoff positional arg | 2 | usage line |
| handoff --cancel with no value | 2 | flag needs an argument: -cancel |
| handoff --scratch ... --cancel N | 2 | usage line |
| handoff --scratch= (empty) | 2 | --scratch must be purpose=P,path=REL[,required=true\|false] |
| handoff --scratch trailing comma | 2 | same |
| prune default on empty root | 0 | pruned call-sessions=0 |
| prune 1.5d, +14d, 14D, 0d, -1d, 1d2h, "14 d", "", 14 | 2 | usage: metasystem context prune --root ROOT [--older-than 14d] |
| prune 106751d | 0 | pruned call-sessions=0 (largest representable day count) |
| prune 106752d | 2 | usage line (would wrap negative) |
| prune 1ns, 1h30m, 0.5h, 9223372036854775807ns | 0 | pruned call-sessions=0 |
| prune 9223372036854775808ns | 2 | usage line |
| prune --older-than= | 2 | usage line |
| prune positional, prune without --root | 2 | usage line |

Error wrapping: `contextVerbError` uses `errors.As`, which sees through `%w` and `errors.Join`; the steward wraps refusals with `%w` only (for example `capture job record %s: %w`). No path turns a HandoffRefusal into exit 1. Rows R20b and R21 exercise the mapping and the one-line rule.

## Reproduced mutation table

Method per row: one perl edit of the named rule, `go test -count=1 ./cmd/metasystem/ -run '^<Test>$'`, restore from the pre-sweep copy, shasum compared with the baseline. H = TestContextHandoffVerb, C = TestContextVerifyAndCancel, P = TestContextPruneVerb. Observed lines are trimmed; nonces, digests and temp paths vary.

Builder's rows:

| Row | Builder's rule | Test | Reproduced | Observed line |
| --- | --- | --- | --- | --- |
| R01 | Register handoff (main.go:434) | H | yes | test.go:655: text handoff = code 2 stderr "metasystem context: unknown verb \"handoff\"" |
| R02 | Register verify (main.go:435) | C | yes | test.go:707: verify = code 2 ... unknown verb "verify" |
| R03 | Register prune (main.go:436) | P | yes | test.go:754: prune = code 2 ... unknown verb "prune" |
| R04 | Containing checkout resolves to its state root | H | yes | test.go:838: handoff classified roots (".../001", ".../001"), want ".../001/metasystem" |
| R05 | Goal machine projected into the caller | H | yes | test.go:655: text handoff = code 9 stderr "HANDOFF_NOT_HOLDER" (my edit empties the machine; the builder substituted a wrong name; both kill) |
| R06 | Announcement's strong process identity preserved | H | yes | test.go:662: captured command state = ... StartTicks:0 BootID: ... |
| R07 | Delegate bound to the consumed active job | H | yes | test.go:679: unexpected end of JSON input |
| R08 | Repeated scratch arguments accumulate | H | yes | test.go:662: captured command state = ... one Scratch entry (Purpose:second) |
| R09 | required=true preserved | H | yes | test.go:662: captured command state ... Required:false on the first scratch (the builder's run reached line 690 first; same rule, same test) |
| R10 | Explicit required=false preserved | H | yes | test.go:662: captured command state ... Required:true on the second scratch |
| R11 | Duplicate scratch keys rejected | H | yes | test.go:694: malformed scratch "purpose=,purpose=bad,path=proof.txt" = code 0 |
| R12 | Explicitly empty required rejected | H | yes | test.go:694: malformed scratch "purpose=bad,path=proof.txt,required=" = code 0 |
| R13 | Exact handoff success shape | H | yes | test.go:655: text handoff = code 0 stdout "recorded: <nonce> state=... sha256=..." |
| R14 | JSON limited to four fields | H | yes | test.go:683: JSON handoff = code 0 stdout "{...,\"intentPath\":...,\"extra\":\"\"}" |
| R15 | Explicitly empty cancellation is usage | C | yes | test.go:736: invalid cancellation = code 9 stderr "HANDOFF_NOT_LIVE nonce=" |
| R16 | Exact cancellation success shape | C | yes | test.go:713: cancel = code 0 stdout "cancelled: <nonce>" |
| R17 | Cancel refusal for a consumed intent propagates | C | yes | test.go:729: consumed cancel = code 0 stderr "" |
| R18 | Exact verification digest shape | C | yes | test.go:707: verify = code 0 stdout "ok digest=<digest>" |
| R19 | Drifted handoff state rejected | C | yes | test.go:723: drifted verify = code 0 stderr "" |
| R20 | Handoff refusal maps to exit 9 | H | yes (anchored re-run R20b; my first regex hit runContextReport's `return 9`) | test.go:689: refused handoff = code 1 stderr "HANDOFF_REFERENCE path=missing file expected=readable-regular found=missing" |
| R21 | Refusal output stays on one line | H | yes | test.go:689: refused handoff = code 9 stderr "HANDOFF_REFERENCE path=missing\nfile expected=..." |
| R22 | Prune defaults to 14 days (edited to 16d) | P | yes | test.go:754: prune = code 0 stdout "pruned call-sessions=0\n" |
| R23 | Positive integer-day durations accepted | P | yes | test.go:754: prune = code 2 stderr "usage: metasystem context prune ..." |
| R24 | Positive Go durations accepted | P | yes | test.go:764: Go duration = code 2 stderr "usage: ..." |
| R25 | Zero rejected as usage | P | yes | test.go:771: invalid duration "0" = code 1 stderr "... older-than must be positive" |
| R26 | Negative rejected as usage | P | yes | test.go:771: invalid duration "-1h" = code 1 stderr "... must be positive" |
| R27 | Representable day overflow rejected before multiplication | P | yes | test.go:771: invalid duration "106752d" = code 1 stderr "... must be positive" |
| R28 | Count and removed-handoff line shapes | P | yes | test.go:754: prune = code 0 stdout "call-sessions=0\npruned handoff=<path>" |
| R29 | Completed counts printed before a partial error | P | yes | test.go:779: partial prune = code 1 stdout "" stderr "... malformed nonce \"malformed\"" |

Rows I added (rules in the brief or the diff without a builder row):

| Row | Rule | Test | Result | Observed line |
| --- | --- | --- | --- | --- |
| X01 | --cancel rejects --scratch | C | KILLED | test.go:736: invalid cancellation = code 9 stderr "HANDOFF_NOT_LIVE nonce=<n>" |
| X02 | verify root resolution | C | KILLED | test.go:707: verify = code 9 stderr "HANDOFF_NOT_FOUND nonce=<n>" |
| X03 | prune root resolution | P | KILLED | test.go:754: prune = code 0 stdout "pruned call-sessions=0\n" |
| X04 | verify --nonce required | C | SURVIVED | ok |
| X05 | verify --root required | C | SURVIVED | ok |
| X06 | handoff --root required | H | SURVIVED | ok |
| X07 | handoff rejects positional args | H | SURVIVED | ok |
| X08 | prune rejects positional args | P | SURVIVED | ok |
| X09 | prune --root required | P | SURVIVED | ok |
| X10 | holder main id projected into the caller | H | KILLED | test.go:655: text handoff = code 9 stderr "HANDOFF_NOT_HOLDER" |
| X11 | ClassifyAt installation argument is the state root | H | KILLED | test.go:838: handoff classified roots (".../metasystem", ".../001"), want ".../metasystem" |
| X12 | receipts path is <root>/memory/receipts.log | H | KILLED | test.go:655: code 1 stderr "handoff receipt file must be .../memory/receipts.log" |
| X13 | nonzero clock passed to the steward | H | KILLED | test.go:655: code 1 stderr "handoff requires one nonzero clock observation" |
| X14b | --json branch honoured (anchored; the first regex hit runContextStatus) | H | KILLED | test.go:679: invalid character 'h' looking for beginning of value |
| X15 | malformed --scratch exits 2, not 1 | H | KILLED | test.go:694: malformed scratch "...required=1" = code 1 |
| X16 | scratch key whitelist (purpose, path, required only) | H | SURVIVED | ok |
| D01 | delegate whose classified pid (9999) is not the job's custodian (4343) must refuse | H | SURVIVED (defect, F-1) | ok |
| D02 | job record custodian changed to pid 7777; classification unchanged; must refuse | H | SURVIVED (defect, F-1) | ok |

## Second round: the fold, swept in a scratch copy

Swept state: a full copy of the module taken at 19:19:52 with context_verbs.go 63b1f9bb760f97eea85e67625301ced04d7cafae, context_verbs_test.go 3c6a1054a9229dd1f86dd2344db5092757bc7b26, main.go d8ac2f06494c30693df052acb2e2e85f78676979, `git diff --numstat` total 398. Scripts mutate3.sh (all baseline rows re-run plus the new N rows) and mutate4.sh (rows re-keyed to the new usage table); logs mutations3.log and mutations4.log; both restored the copy to its shasums. The copy's own baseline passed all four tests before any row ran.

Conformance of the fold to the seat's fold brief (artifacts/reports/codex-ccb-slice3-d1a-fold-brief.md):
- F-1: `hookContextHandoffDelegate = lease.HookDelegate` seam; in the DELEGATE branch `ConsumedActiveJob`, then `HookDelegate(stateRoot, stateRoot, jobID, ppid)`, JobId set only on `Delegate && JobID == jobID`; a "wrong delegate" case in TestContextHandoffVerb with the hook seam answering Delegate false, asserting exit 9 and `HANDOFF_NOT_HOLDER`. Present.
- F-2: `TestContextVerbUsage`, a table of sixteen exit-2 vectors: the six usage vectors and the scratch key whitelist the read asked for, plus the cancel, scratch and duration rejections that previously lived inline in the three original tests. The fold moved those inline assertions into the table (they are no longer in TestContextHandoffVerb, TestContextVerifyAndCancel or TestContextPruneVerb); the table uses a plain fixture root and the checks it exercises all run before any state is read, so the proof is equivalent. It is a move, not a cut, which the fold brief allowed to fit the ceiling. Sweep 4 confirms every moved rule still kills under the new test.
- Ceiling: 398 at the swept state, 400 at 19:23:55, both within the brief's 400.
- Not asked for: the JSON struct was replaced by a four-key `map[string]string` in the swept state (same keys, no behavior change) and restored to the struct at 19:23. F-3 and F-5 were not acted on, as instructed.
- The builder's Round 2 section in artifacts/reports/codex-ccb-slice3-d1a-result.md was not yet written at 19:23:55 (file mtime 18:45).

Rows on the swept state (H, C, P as before; U = TestContextVerbUsage):

| Row | Rule | Test | Result | Observed line |
| --- | --- | --- | --- | --- |
| N01 | HookDelegate guard removed (JobId always set) | H | KILLED | test.go:689: wrong delegate = code 0 stderr "" |
| N05 | wrong-delegate assertion inverted (test is live) | H | KILLED | test.go:689: wrong delegate = code 9 stderr "HANDOFF_NOT_HOLDER" |
| N02 | `delegate.JobID == jobID` dropped, `Delegate` kept | H | SURVIVED | ok; redundant guard: HookDelegate was narrowed to jobID, so JobID equals jobID whenever Delegate is true. Not material. |
| N03 | hook still consulted when no job is active | H | SURVIVED | ok; JobId stays empty either way and the steward refuses. Not material. |
| N04 | HookDelegate error ignored (exit 9 instead of 1) | H | SURVIVED | ok; the hook error path has no test; operational-error exit code only. Not material, one table row if the seat wants it. |
| D01 | delegate seam pid changed to 9998 | H | SURVIVED | expected: HookDelegate is a seam in unit tests; the real ancestry binding is proven by internal/lease/hook_delegate_test.go (four tests) |
| D02 | job record custodian changed to 7777 | H | SURVIVED | expected, same reason |
| R01-R06, R08-R10, R13, R14, R16-R19, R20b, R21-R24, R28, R29 | baseline rules unchanged | H/C/P | all KILLED | same assertion lines as round 1, shifted by the refactor (653, 660, 675, 679, 699, 703, 711, 715, 730, 738, 743, 832) |
| R07 | replaced by N01 (the old delegate block no longer exists) | | | |
| R11u, R12u, R15u, R25u, R26u, R27u, X01u, X15u | rules whose assertions moved into the table | U | all KILLED | test.go:772: args [...] = code 0/1/9, want exit 2 |
| Rreq1u | required value limited to true/false | U | KILLED | test.go:772: args [... required=1] |
| X02, X03, X10-X13, X14b | unchanged | C/P/H | all KILLED | as round 1 |
| X04-X09, X16 | the seven F-2 rules | U | all KILLED | test.go:772: e.g. `args [] = code 0` (prune without root ran on the cwd), `args ["--nonce" ...] = code 9`, `args [... "foo=bar"] = code 0` |
| N06 | usage-table assertion inverted | U | did not apply (assertion text differs from my regex); the seven X kills above already prove the table is live |

Tally on the swept state: 47 rows killed (38 in sweep 3, 9 in sweep 4, the latter re-keying the 8 moved rules plus Rreq1u), 5 survivors (N02, N03, N04, D01, D02), none material, one row not applicable (N06). Both material findings of round 1 are corrected and proven on that state.

After the sweep the live tree moved again (19:23, context_verbs.go 4f74531edc4239375abd03997f1e2df6a22c33ad, context_verbs_test.go bcd40ce2eb0e184dd2e8872ca0a99da8f20f5105, 400 changed lines). The delta from the swept state is: the JSON struct restored in place of the map; the wrong-delegate case no longer re-classifies with pid 9999 and relies on the hook seam alone (the same proof, since the seam decides); two cosmetic line merges. Nothing in that delta touches a rule, but I have not swept it. The builder was still working, so the final certification belongs to a read of the tree the builder returns: copy it, run mutate3.sh and mutate4.sh against the copy (about five minutes), and check the Round 2 table against the rows above.

## Verification commands

From the worktree's metasystem directory with METASYSTEM_BIN and METASYSTEM_CONTEXT_COST_PROOF unset:
1. `go build ./...`: exit 0, no output.
2. `go vet ./cmd/metasystem/`: exit 0, no output.
3. `go test -race -count=1 ./cmd/metasystem/ -run 'TestContext'`: `ok github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem 441.798s`.
4. `go test -count=1 ./cmd/metasystem/ -run '^(TestContextHandoffVerb|TestContextVerifyAndCancel|TestContextPruneVerb)$' -v`: PASS 0.80 s, 0.62 s, 0.37 s.
The full package run and go-gate were not run here; the seat runs them.

## Verdict

Round 1 (baseline 399): send back. In order: (1) F-1, bind a DELEGATE caller to the active continuation job through `lease.HookDelegate` behind a seam, leave `JobId` empty when the caller does not match, and add the D01 case as a failing test; (2) F-2, add one table-driven test for the six usage vectors and the scratch key whitelist, and add their rows plus the four X01, X02, X03, X10 rows to the mutation table. F-3 and F-5 are recorded for the seat and the design page, not for this build.

Round 2 (the fold, swept at 63b1f9bb / 3c6a1054, 398 lines): both corrections are present, conform to the fold brief, and are proven by the rows above; zero material findings remain on that state; the three survivors N02, N03, N04 are not material. The tree moved again after the sweep (4f74531e / bcd40ce2, 400 lines) with no rule touched; land after the builder's Round 2 return, once the seat re-runs the two scripts against a copy of the final tree and reads the Round 2 table.

## Appendix: per-test race timing

`go test -race -count=1 ./cmd/metasystem/ -run 'TestContext' -v`, second run, PASS:

| Test | Seconds |
| --- | --- |
| TestContextStatusVerbPrintsTheRoleLine | 464.53 |
| TestContextHandoffVerb | 0.84 |
| TestContextTestingContractSelectsProof | 0.81 |
| TestContextVerifyAndCancel | 0.65 |
| TestContextStatusJSONOmitsCursorHistory | 0.51 |
| TestContextPruneVerb | 0.40 |
| TestContextReportVerbPublishesTheWeek | 0.14 |
| every other TestContext* test | under 0.1 |
| TestContextStopFitsDurationBudget | skipped (opt-in flag unset) |

The whole 441 s of the first run and 464 s of the second sit in `TestContextStatusVerbPrintsTheRoleLine` (context_verbs_test.go:161, landed in bd7a6cbc, slice 2b part D). It predates this diff and the diff does not touch the status path. The test body has no sleep of its own; it dispatches `context status --root <root>` and the time is inside that call under race on this machine. The builder reported 8.3 s for the same selection in its sandbox, so the cost is machine or environment dependent. Not a D1a finding; recorded for the seat, who may want to time that one test without race and with the artificial-clock rule in mind.

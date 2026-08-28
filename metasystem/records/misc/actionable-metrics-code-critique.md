# actionable-metrics — slice-one code critique record (2026-08-27)

Chain: `actionable-metrics-code` (critique-round.sh, gpt-5.6-sol at
xhigh, chain goal file: actionable-metrics). Budget: 3 rounds,
declared in the round-1 brief. This critique is mandatory: the
design chain exited through its declared early-close, so the O1-O20
fixtures and this chain are the arbiter.

## Round 1 (r1-output.md): 14 material + 1 info

Dispositions (orchestrator, m1 coordinator):

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F-1 | accepted | Verified: data.go git calls run inside metasystem/ with `-- .`; the repo git root is the parent, so repo-wide landings undercount payload and a commit pairing plans/metrics/ with paths outside metasystem/ reads metrics-only and is falsely excluded. | Payload and self-exclusion computed over the FULL repository diff (git root); correction task 1 |
| F-2 | accepted | compute.go counts timing-less jobs usable at zero duration; 8 live terminal records lack startedAt. Confident zero violates D-C/O2. | Jobs without both timestamps are timing-REJECTED for wall-clock (named), excluded from wall_hours; overhead with zero timed jobs prints unavailable; O2 fixture extended; task 2 |
| F-3 | accepted | Per-goal attributed/total coverage and the UNATTRIBUTED bucket exist only as one aggregate jobs line — the attribution ruling half-implemented. | Every per-goal process value carries its own attributed/total per source; unattributed records land in a printed UNATTRIBUTED bucket; task 3 |
| F-4 | accepted | O1 asserts coverage verbatim for one metric of nine. | O1 asserts value AND coverage for all nine rows; task 4 |
| F-5 | accepted | Neither real `goal done` route is exercised; deleting a hook keeps O12 green. | O12 fixture drives both CLI routes end to end (legacy and synced) through the real binary or main-path harness; task 5 |
| F-6 | accepted | thresholds.go discards ParseFloat errors: "bogus" becomes an enabled zero band — the exact D-D violation the validity contract forbids. | Unparseable floats disable the threshold with the "threshold invalid" row; O17 fixture gains the string case; task 6 |
| F-7 | refuted | The design's attribution law is DECLARATION, not time: O13 "metrics attribute on exact goal-id match only"; D-I's lifecycle clause governs the report's reading window, not attribution membership. A goal= row after conclusion is that goal's declared work (fix-forward). Checked design text at plans/actionable-metrics-design.md (O13, D-I). | none |
| F-8 | accepted | The unattributed first round writes no sentinel, so a concurrent attributed r1 can bind the chain in the gap. | Every first-round caller atomically records the attribution decision (goal file or explicit unattributed marker, both O_EXCL/noclobber); later callers must match; fixture covers the loser path; task 7 |
| F-9 | accepted | The builder's own disclosed choice 9 (unavailable + loud missing-goal coverage) is not what report.go does: a normal report publishes with global rows. | Unknown --goal renders an unavailable goal report whose coverage names the missing goal; no global values leak in; task 8 |
| F-10 | accepted | Sub-second windows collide on second-truncated filenames and rendered bounds differ from computed. | Inputs truncate to whole seconds at parse (documented), so computation, rendering, and filenames agree; same-second distinct windows cannot exist; task 9 |
| F-11 | accepted | A read/permission error on the chain goal file falls through to lawful absence — attribution silently lost, D-C requires REJECTED by name. | Unreadable goal file → chain lands in REJECTED with the error named; task 10 |
| F-12 | refuted | An empty debt list over a present, readable ledger is a TRUE zero, not a coverage failure: D-C's unavailable fires on zero USABLE INPUTS, and the input population is the ledger's live goals (found and parsed). Making "no parked/unsized goals" print unavailable would claim the metric could not compute when it did. Checked compute.go:509-535 against D-C. | none |
| F-13 | accepted | Second-precision ledger timestamps make claim==landing==done reachable; 0/0 yields NaN and a false "within" judgment. | Zero-duration lifecycle prints share as a labelled degenerate case (no NaN, no judgment); O-fixture added; task 11 |
| F-14 | accepted | O15 asserts internal flags only; both-goals payload counting and the shared label are unproven. | O15 fixture computes both per-goal reports and asserts the shared label and per-goal payload; task 12 |
| F-15 | noted (not material) | Sandbox denied go's work dir; the orchestrator ran the full focused set outside the sandbox: vet rc=0, four packages ok, fixture script rc=0 (this session, 2026-08-27 ~10:10). | none |

Round-1 close: 12 accepted → one focused correction follow-up to the
SAME implementer session; 2 refuted with evidence; round 2 recomputes
the whole diff and runs both layers again.

Correction-pass note: the first correction pass (codex companion
resume) double-fired at launch and later ZOMBIED mid-work (status
running, process dead; caught by the coordinator's monitor). Bytes
audit found 7 of 12 fixes in; a fresh direct codex exec finished the
residual six. This incident drove the delegate-job-liveness custody
arc re-scope (Wido approved, 2026-08-27).

## Round 2 (r2-output.md): 8 material + 2 info, falling from 14

Dispositions (orchestrator, m1 coordinator) — all eight accepted:

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F2-1 | accepted | attributed/total + UNATTRIBUTED helpers used only by Overhead/Rework/Delegates; Waiting and Cost still emit generic coverage. | Both metrics gain per-goal attribution coverage; fix task 1 |
| F2-2 | accepted | The pass time-bounded attributed records in Overhead and Cost — violating the standing F-7 refutation (attribution is declaration, never time). | Membership by exact goal-id only; lifecycle bounds only lifecycle-derived splits; task 2 |
| F2-3 | accepted | Cost prints wall_hours=0.000 with only timing-less jobs and labels them usable. | Cost wall-clock unavailable without timed jobs; timing-less excluded from wall sums; task 3 |
| F2-4 | accepted | Non-terminal jobs (no endedAt yet) are lawful in-flight records, not REJECTED rot. | Timing rejection applies to TERMINAL records only; in-flight jobs scope out by having no completion time, unlabelled as rot; task 4 |
| F2-5 | accepted | O2's test never forbids the Overhead false zero it exists to prevent. | O2 asserts wall_hours=unavailable and forbids 0.000/spend=0.000; task 5 |
| F2-6 | accepted | The attribution lock has no owner record or stale recovery — a died process strands the chain for 200 retries then refusal. | Lock carries owner pid; stale-owner steal + trap cleanup; fixture covers the dead-owner path (bash 3.2); task 6 |
| F2-7 | accepted | Single-decode JSON accepts trailing garbage as usable. | Decoder verifies end-of-stream; O8 gains the trailing-garbage leg; task 7 |
| F2-8 | accepted | gofmt -l lists internal/metrics/data.go; the landing gate refuses it. ALSO: the residual pass reported GOFMT_RC=0 — false verification evidence, noted. | gofmt the file; re-run gofmt -l over internal+cmd as the check; task 8 |
| F2-9 | noted (not material) | Critic sandbox denies go test; the orchestrator's own runs are the executable evidence. | none |
| F2-10 | noted (not material) | Join check passed for unknown-goal, O12 routes, zero-duration; no scope creep found. | none |

Round-2 close: 8 accepted → focused follow-up (direct codex exec);
round 3 is the budget's final round, reviewing the follow-up and the
whole recomputed diff.

## Round 3 (r3-output.md): 4 material — FIRST BUDGET EXHAUSTED

Join check: F2-1, F2-3, F2-4, F2-5, F2-7, F2-8 confirmed fixed;
F2-2 holds for Overhead/Cost; both round-1 refutations intact for
their original subjects; scope clean; gofmt/bash-syntax clean.

Dispositions — all four accepted, none refutable:

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F3-1 | accepted | The journal helper (compute.go:407-412) still requires inGoalLife — the declaration-not-time ruling's last violation, feeding Friction and Collisions context. | Exact target membership only; fix task 1 |
| F3-2 | accepted | Two waiters reading a dead owner can chain-steal: the second removes the NEW live owner (no generation compare) — both sides can then write conflicting decision files. | Steal removes the lock only if the owner record still matches the dead owner read; acquisition is always a fresh mkdir race; task 2 |
| F3-3 | accepted | Owner data written non-atomically after mkdir — SIGKILL in the gap leaves an ownerless lock recovery cannot read and cleanup cannot rmdir; chain stranded. | Atomic owner publication (stage + rename); ownerless locks past a short age are stale; task 3 |
| F3-4 | accepted | The receipt CORRECTION path validates syntax but not corrected goal=/built_by= values; an invalid effective goal escapes attributed, UNATTRIBUTED, and REJECTED alike. | Corrections validate like originals at write; metrics read defensively rejects invalid effective values by name; task 4 |

### CRITIQUE EXHAUSTION #1 (recorded per the round-budget law)

The first three-round budget closed with material findings open:
**F3-1, F3-2, F3-3, F3-4** (enumerated; nothing else is open — all
earlier findings are fixed, refuted-with-evidence, or noted). One
focused follow-up addresses exactly these four; a FRESH three-round
budget opens on the same chain (rounds 4-6). If material findings
exhaust that second budget, work stops outright and waits on the
human — no third budget exists.

## Round 4 (r4-output.md): 5 material — the generating cause is cut

The count ROSE (4→5) and four of five findings live in the lock
machinery added across the last two fixes — the classic
machinery-climbing-its-own-ladder signal. The orchestrator's
adjudication cuts the generating cause instead of patching again:
THE LOCK IS DELETED. The chain attribution decision becomes ONE
file created by atomic exclusive link — content staged complete,
`ln` into place (exclusive AND complete in one step); every caller
reads the standing decision and matches or refuses. No owner
records, no staleness aging, no steal, no PID semantics, no polling.
Death before the link leaves nothing durable; death after leaves a
complete decision — both lawful, no recovery machinery exists.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F4-1 | accepted (cause-cut) | Post-compare/pre-unlink interval real: recheck and removal are separate ops — a delayed waiter deletes a live replacement. | The lock ceases to exist; the ln-exclusive decision file has no removal path; fix task 1 |
| F4-2 | accepted (cause-cut) | Ownerless aging can replace a slow live publisher whose later mv overwrites the replacement — two owners. | Same: no aging, no owners; task 1 |
| F4-3 | accepted | A rejected-effective-provenance row stays USABLE for period Rework and Delegates — REJECTED and USABLE at once, against D-C. | Rejected rows are rejected for EVERY consumer, coverage named; period-scope test without exact-goal masking; task 2 |
| F4-4 | accepted (cause-cut) | kill -0 on a reused PID reads an unrelated process as the owner; the generation token is never liveness-checked. | Same: no PID semantics; task 1 |
| F4-5 | accepted | The fixture's fixed 500×10ms wait violates the load-scaled fixture contract (orchestration.md:199). | The deterministic ln race needs no polling; any residual wait scales via METASYSTEM_FIXTURE_CAP_SCALE and reports elapsed/cap; task 3 |
| F4-6 | noted (not material) | Joins F3-1/F3-4-write-side pass; refutations hold; scope clean; orchestrator-side executable runs remain the evidence. | none |

Second budget position: round 4 spent; rounds 5-6 remain.

## Round 5 (r5-output.md): 4 material — the protocol certifies, the proof must catch up

The critic certifies the hard-link protocol sound by adversarial
reading; the findings are proof-depth and edge-validation, not
machinery design. Trajectory 14-8-4-5-4 with the round-5 findings
changed in KIND. The coordinator's declared step-back (strip chain
attribution) technically triggered on region recurrence but the
governing condition (failure to make progress, Wido 2026-08-27) is
NOT met; the strip is PRE-COMMITTED as the round-6 branch instead:
any attribution-region material finding in round 6 removes chain
attribution from slice one outright (it returns structurally when
the severity work routes critique rounds through the job machinery).

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F5-1 | accepted | Two backgrounded full runs can serialize; the collision interleaving is never forced, so a check-then-write regression could stay green. | Deterministic barrier fixture: both callers held between existence check and ln (pause hook/FIFO), exactly one wins, loser reads a complete decision; task 1 |
| F5-2 | accepted | landing.Goals copies raw goal= values before the invalid-effective mark; Overhead/Waiting consume them — the F4-3 exclusion missed one consumer. | Landing attribution built from EFFECTIVE validated provenance only; invalid rows named once; period-scope test covers landings; task 2 |
| F5-3 | accepted | Bare `wait` twice, no cap scale, no elapsed/cap diagnostics — the fixture contract requires load-scaled bounded waits. | All fixture waits bounded and scaled via METASYSTEM_FIXTURE_CAP_SCALE with elapsed/cap on failure; task 3 |
| F5-4 | accepted | The validator follows symlinks (-f/-r after -e) and bash command substitution silently drops NUL bytes — malformed state can normalize into a valid decision; the Go reader already Lstats. | The script validator refuses symlinked decisions (lstat-equivalent test) and byte-validates content (exact grammar match on the raw bytes); fixture legs for both; task 4 |

Second budget position after round 5: round 6 is the FINAL round —
zero unrefuted material lands; any material finding exhausts budget
two and stops the work on the human.

## Round 6 (r6-output.md): 1 material — SECOND EXHAUSTION, WORK STOPPED ON THE HUMAN

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F6-1 | accepted | Real per the command-inventory contract (docs/project-rules.md:104-108): critique-round.sh:81 runs `cmp` which preflight-commands.sh never requires — an absent `cmp` makes every valid attribution decision report malformed. Fifth consecutive round with a material finding in the attribution region (races → validator edges → packaging). | Awaits the human ruling below |

### CRITIQUE EXHAUSTION #2 (the hard stop)

Both budgets are spent with a material finding open: **F6-1** (the
only open finding anywhere on the chain — everything else across six
rounds is fixed, refuted-with-evidence, or noted; the critic's own
reading certifies the rest of the slice). Per the round-budget law,
work stops outright; a human decision recorded here is the only
remedy. The standing pre-commitment (recorded before round 6 ran):
a material finding in the attribution region strips chain
attribution from slice one — it returns structurally when the
severity work routes critique rounds through the job machinery.
Options put to Wido: (a) execute the pre-committed strip and land
the slice without the script-side attribution writer (Go reader
stays, certified); (b) authorize the one-line remedy (declare cmp in
the preflight inventory, or bash-native byte compare) and land with
attribution; (c) park the slice. Coordinator recommendation: (a) —
the pre-commitment exists precisely to defeat "it's only one line
now", which every round in this region said.

**WIDO RULED (2026-08-27, in-session): EXECUTE THE STRIP.** The
script-side chain-attribution writer leaves slice one; the certified
Go reader stays (absent decision reads historical-unattributed, its
REJECTED arm still guards); chain attribution returns structurally
when the severity work routes critique rounds through the job
machinery. F6-1 resolves by removal of its subject under this
ruling; with it, ZERO unrefuted material findings remain across six
rounds and the chain CLOSES. No new critique budget opens for the
strip: it is a deletion of the never-certified region under the
recorded human remedy, and the remainder of the tree carries six
rounds of standing certification; the coordinator's executable
verification gates the landing as always.

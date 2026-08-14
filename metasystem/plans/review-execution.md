# review-execution

- Owner: Claude (Wido's delegated session, 2026-08-13 AFK rulings), main branch
- Goal and current status: execute the 101-finding backlog of
  docs/reviews/2026-08-12-full-system-review.md. W1 (26), W2 (17), W3 (8)
  complete; W4 at 17 of 25. Roughly 24 findings remain. Every delegated
  sign-off is recorded in docs/reviews/2026-08-13-delegated-decisions.md
  (through D26) for Wido's after-the-fact review.
- In flight right now: nothing in this checkout — the work is done by the
  orchestrating session itself between validation batches, not by
  dispatched jobs (the KI-34 wording for main-session work).
- Decisions made (and who made them): D15-D26 by the delegate under the
  2026-08-13 delegation; D17 (adopted-engine delivery) overrides the r10
  human-severance under that delegation and is flagged as the revert point.
- Waiting on the human (open escalations, reviews, reserved decisions):
  review of the decisions doc, especially D17 and D19; the benchmark
  series stays parked (no spend without Wido's word). Re-arming this
  checkout's supervision is BLOCKED on an ergonomics wrinkle: arming
  identity inference reads the git toplevel's metasystem.conf (which
  lists no runtimes) instead of this subdirectory checkout's — refusal
  "cannot infer arming identity", 2026-08-14. Needs either the toplevel
  conf gaining runtimes or arming taught subdirectory conf resolution;
  the review work itself does not dispatch jobs, so it proceeds unarmed.
- PRIORITY (Wido, 2026-08-14 afternoon): the suite-time reduction
  (plans/suite-time-reduction-design.md r5, D33, critique-converged
  10-4-3-1-AGREE) implements BEFORE W5. Checkpoints: (a) go-gate.sh
  witness write/accept + snapshot-gated mode; (b) validate
  --delivery-contract + adopt-fixture handoff + drift leg retained;
  (c) engine-delivery key required + adopt stamping + -buildvcs=false
  digest stamp + smoke; (d) verification legs; boundary measures
  before/after (~45 min today) for the D33 entry.
- W5 PLAN (started 2026-08-14 midday, now queued behind D33): batch 1 = the five smalls
  (script-validate-1 awk contract-hash shadow, -2 census freshness via
  dispatch census-fresh, -3 typed arming-window exit code collapsing
  three retry loops, -8 metasystem.runtimes through the config engine,
  -9 Go-source grep pin becomes a Go unit test), canaries + gate per
  commit, boundary at batch edge; then W5.1 script-validate-4 (extract
  the two giant blocks into sub-suite scripts) SOLO — it restructures
  the suite itself; then -10 python-heredoc triage (sign-off),
  -5/-7 (sign-off, verifier-downgraded, likely documented declines),
  shellcheck toolchain question (new dependency — decide with care:
  hosts and CI must have it or the check must be conditional), and the
  -11/-12 smalls.
- Dead ends (do not retry without new evidence): none in this stream.
- In flight right now: nothing in this checkout — last detached Mac
  suite finished GREEN. (Historical: suites run DETACHED from the pinned worktree (nohup, log at the session
  scratchpad's mac-suite-015f949.log) because the harness's 10-minute
  background cap kills tracked suite runs now that D17 adds nested go
  gates; the loop polls the log. VM GREEN at 015f949 (HEAD in-log at
  launch) — the W4.22 boundary's authoritative half — and the trusted
  baseline is re-recorded there.
- D33 MEASURED AND GREEN: VM 545s (9.1 min) at e8538af vs ~20 min
  pre-D33 — 2.2x; six real defects shaken out en route (each its own
  commit; see the D33 entry). Mac sampling at e8538af flaked at
  S4-2's 36s scaled cap under load ~5 (Wido active), GREEN SOLO on
  immediate reproduction — the morning's exact signature; VM-green
  authority holds per the dossier. Baseline recorded at bcc96b7,
  memory updated. W5 BATCH 1 CLOSED (5/10; D34): -8/-9 a5bc087, -2 b3e3557, -1 aa7eae8, -3 de9e88d. Next after the batch boundary: W5.1 script-validate-4 SOLO (extract the two giant blocks into sub-suite scripts), then the W5 tail, W6, W7. Superseded: W5 batch 1
  (script-validate-1/-2/-3/-8/-9), W5.1 solo, W6, W7.
- Next step: W4 is COMPLETE (25/25 at 132953f; 015f949 boundary GREEN
  ON BOTH HOSTS). Queue tail: F5 LANDED (D31). F4 IN PROGRESS: design at r2 in
  plans/f4-orphan-window-design.md. Round-1 critique (codex
  gpt-5.6-sol at xhigh via CLI fallback — supervision dispatch still
  blocked): REVISE, ten material findings, ALL folded; the design
  pivoted from Option A (waiter adoption) to the critique's simpler
  Option C: the DETACHED ADAPTER SUPERVISOR (already the record's
  custodian, own session/group, survives waiter death) enforces the
  record's own handshakeDeadline and capDeadline over its CLI child;
  waiter and reaper unchanged; custodian-dies-leaving-grandchildren
  accepted as the stated residual. Critique CONVERGED at round 5
  (10-4-3-1-AGREE; D32). IMPLEMENTATION LANDED THROUGH CHECKPOINT 2:
  f9e554e (proc group-members = the kill domain) and 510d205 (the
  enforcement core in runtime-common's wait loop; three-leg direct
  harness proved cap-expiry, handshake-expiry, and zero-signal
  stand-down against the real code; harness lives at the session
  scratchpad's f4-harness/). F4 COMPLETE with the next commit (fixture landed, suite+canary wired; two real defects found and fixed by the fixture — see D32 addendum). Superseded notes: promote the harness
  into a committed suite fixture plus the design's other legs
  (waiter+supervisor race to one verdict; supervisor crash → reaper
  finalizes; survivor after KILL → nonterminal), D32 addendum,
  boundary suites, baseline. Original sketch: SOLO implementation per the design's
  'What changes where' — runtime-common wait-loop deadline
  enforcement (cached fail-closed deadlines, gate-wait coverage,
  before-any-signal stand-down, own-group-minus-self kill sweeps with
  death proof), fake adapter deadline-expiry behaviors, suite legs per
  the Verification section; canary supervision per commit; boundary
  suites (VM detached, Mac detached) after; D32 addendum on landing. After F4 (DESIGN pass: the orphan window — a job whose
  waiter and runner both exit has no kill-capable enforcer until
  mission end; decide between waiter adoption and a kill-capable
  supervision sweep; touches the no-kill-authority rule; design →
  named critique → implement per the standing discipline); then
  W5 (10, suite decomposition), W6 (9, fixture retirement — each
  whole-file retirement its own decisions-doc entry), W7 (7, docs);
  queue tail F4 orphan-window design and F5 reaper decline logging.
  Boundaries: 8c6a114 VM-green (Mac flaked 4x under active use; three
  real harness races fixed, dossier); e91cec4 (W4.18/D27) GREEN ON BOTH
  HOSTS, trusted baseline recorded there. W4.20 smalls landed (D28:
  -08 devin-prompt verb, -09 settle_result_identity, -12 minimal with
  the wholesale-sourcing decline, -15 resolved by D27) and W4.21/D29
  (refactor gate → validate family) both VM-validated at d47c595; the
  Mac half of that boundary was SKIPPED: the harness's 10-minute
  background cap killed the run under load ~6 (Wido active on the
  machine), kill left no orphans, VM-green authority per the dossier;
  re-attempt at the W4.22 boundary. Suite runtime will GROW at W4.22:
  every nested adopted validation now pays its own go gate (D17's
  designed cost).

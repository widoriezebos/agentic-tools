# Backlog

- Goal and current status: the standing queue of ratified-but-not-scheduled work. Items leave by becoming a design stream, a retro proposal, or a deliberate rejection recorded here.
- Next step: none
- In flight right now: nothing
- Waiting on: nothing — items here are deliberately queued

## Queued

1. **Make "a turn may not end with unwatched work" a mechanism, not a rule.**
   Raised by the human 2026-08-08 after a delegate finished at 06:50 and was
   noticed at 07:38: "can that be a metasystem thing so that we ensure this
   happens? ... And if it is in memory, I'm not so sure if this is a reliable
   way of doing it." They are right that memory and ledger prose are the wrong
   home. Shape, to be designed properly: (a) `dispatch.sh dispatch` without
   `--wait` prints the exact waiter command for the job it just created, so the
   agent never invents a polling loop; (b) `dispatch.sh watch --job <id>` is
   that waiter — it blocks until the job is terminal and exits, which is what
   every runtime's background mechanism turns into a wake-up; (c) the stop hook
   REFUSES a turn that ends with jobs this main dispatched still in flight and
   no waiter running for them, the same once-only shape as the open-work
   refusal. Detection reads job records and process state, so it is
   runtime-neutral; only the wake-up itself is per-adapter plumbing. The honest
   limit to state in the design: nothing outside a session can wake it, so the
   mechanism enforces "do not end the turn blind" rather than "you will be
   woken". Carries IL-24, which is the prose version of this.
2. **Two bars for changes: the design loop for design changes, a direct fix
   for mechanical defects.** Raised by the human 2026-08-08 after they set this
   as the rule for one unattended run and asked whether it should be standard,
   "maybe with a few conditions added". It should, and the conditions are the
   whole point: without them "it's just a bug" becomes the escape hatch that
   swallows every design change. Shape, to be designed properly:
   (a) the classification is DECLARED, not assumed -- a change states which bar
   it took and why, so the choice can be audited later;
   (b) a named list that is NEVER a direct fix, whatever it looks like: an
   invariant, a contract or schema other tools parse, an authority boundary, a
   human ruling, or a safety mechanism. This session is the case for the list.
   The bm-2 delegates failing looked exactly like a bug ("Devin jobs are
   failing"), and the actual defect was in the ownership model -- design, not
   mechanics. The same session also had genuine direct fixes (a tar pipe, a
   working-directory slip) where a design chain would have been pure ceremony;
   (c) when in doubt, the loop wins -- the default on an unclear call is to
   escalate, not to proceed;
   (d) a direct fix that grows beyond the defect stops being one and escalates;
   (e) the EVIDENCE bar does not move. A direct fix still needs a
   failing-then-passing proof and the gates. Skipping the loop skips critique,
   never proof.
   Mechanism, not memory (the human's standing objection): a rule that lives in
   prose gets forgotten under time pressure, which is exactly when it matters.
   Candidates to evaluate in the design -- the commit wrapper requiring a
   declared classification, and paths that carry invariants or schemas requiring
   a design-chain reference before they can be committed to. Whether that is
   enforceable without becoming bureaucratic is the real design question.

3. **Streaming visibility inside the per-turn lifecycle (human-ratified
   direction, 2026-08-09).** Delegates are silent until exit: a 319-second
   SWE-1.7 inference is indistinguishable from a hang, an empty reply costs
   the whole session to discover, and a dead runner went unnoticed for 4.5
   hours. The fix is NOT a lifecycle change: one process per turn stays (the
   lease, census, and reaper are built on process exit as the turn boundary).
   Each runtime's native streaming lands inside that pattern: claude
   `--output-format stream-json`; codex `exec --json` already emits JSONL we
   merely fail to tail; devin runs `devin acp` PER TURN -- the human's
   synthesis: ACP as a transport within the turn, not a persistent server --
   which also moves permission answering to our side of the wire (devin is
   the one runtime with notEnforced everywhere). Supervision gains one
   additive signal: a per-turn last-event-age heartbeat the reaper and any
   watch can read. Honest gaps to resolve in the design: ACP has no standard
   usage reporting (today ACU/tokens come from ATIF final_metrics at exit),
   the turn-end evidence boundary moves to the JSON-RPC response for devin,
   and devin's ACP loadSession support needs a probe. Sequenced AFTER the
   bm-2 cohort completes -- changing the instrument mid-cohort makes
   repetitions incomparable. Design loop: it touches the turn-end and usage
   contracts. None of this makes inference faster; it makes slowness and
   death visible while they happen. CONSOLIDATED 2026-08-09 into
   plans/adapter-streaming.md at the human's request (since folded into the flight-recorder stream, plans/flight-recorder.md, which owns sequencing) -- the full design draft
   (the abstract adapter interface, per-runtime transports, the liveness
   sidecar, ACP-per-turn with its probe questions) lives there; this entry is
   now just the queue marker.

4. **Fixtures-as-arbiter: a principled exit from fine-grained critique
   (human-ratified in one instance 2026-08-09; generalize).** During the
   flight-recorder chain the human set a close rule after six falling rounds
   (21, 15, 10, 4, 3, 2): remaining detail-grain findings are folded, then
   implementation STARTS, with the folded findings expressed as NAMED
   FIXTURES and code-critique as the next reviewer — prose review stops
   being the arbiter once the grain drops to timestamp formats and field
   caps, because at that grain the implementation's own tests judge better
   and cheaper than another prose round. The human: "I really like the
   approach... you fold the remaining issues into fixtures that then
   together with code critique become the arbiter." Not yet a standing
   rule; it lived in one plan header. Conditions that keep it from becoming
   an escape hatch, to be designed into the critique skills alongside the
   stop-rule sharpening below:
   (a) the trajectory must be FALLING and the chain past its round budget;
   (b) every remaining finding must be mechanical-grain — it names a value,
   format, or bounded choice, not an invariant or contract shape; one
   invariant-grade finding and the exit is unavailable;
   (c) the fold is 1:1 — each finding becomes a NAMED fixture obligation in
   the plan's Proof section, so nothing dissolves into prose;
   (d) code-critique of the implementation becomes MANDATORY, not the
   usual per-batch judgement call;
   (e) the switch is RECORDED in the plan header with the trajectory, as
   the flight-recorder instance was.

5. **Qualified-name sweep of agent-facing text (human-raised 2026-08-09).**
   The glossary's namespacing rule — projects own bare words in their own
   workspace; metasystem prose speaks "delegate job", "recorder event",
   "mission gate", "mission runner", "host turn" — is stated but not yet
   enforced where it matters: the dispatch prompt templates, role briefs
   (`scripts/agents/roles/*`), the skills, and docs/. Sweep them, and add
   the rule to whatever lints prose (the audit's placeholder check is the
   natural home for a collision-prone-bare-word check in agent-facing
   templates). Raised because bm-2 literally builds a task runner: a
   delegate reading "check the job record" inside that workspace cannot
   know which system is meant.

6. **Critique stop-rule sharpening (human-ratified 2026-08-07, rides the
   next retro's change gate).** A chain closes on a round with zero
   UNREFUTED material findings: each refutation carries evidence, survives
   exactly one rebuttal round, and persistent disagreement escalates to
   the human. Today the stop rule counts what the critic returns, giving
   the critic an implicit veto over closure; with IL-21 forcing refutation
   attempts, that veto will start binding. Owner doc when adopted:
   `skills/design-critique/SKILL.md` (and code-critique's mirror section);
   the join script gains the unrefuted-count check. Wido: "I like the
   sharpening and I think we should implement it."
7. **KI-23 acknowledged-process mechanism** — after the one-writer
   implementation lands (same files).
8. **Mission-completion protocol design** (plans/mission-completion-protocol.md,
   seven carried findings) — after the coexistence stream closes.
9. **Benchmark kit halves** of the validity closure (schema v2 entries,
   extractor version dispatch; human-ratified) — with the next kit-gate
   batch.
10. **Confirming benchmark re-run** (Opus critics vs luna builders, the
   0.019→0.981 effect) — with the human present, after the above.
11. **Devin integration** — UNPARKED 2026-08-07: the one-writer fix is
   implemented, proven by both gates, and pushed. Next: the probe and
   `development/devin-selftest.md`; a `hosts/devin.sh` is a separate build.

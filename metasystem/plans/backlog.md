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

3. **Critique stop-rule sharpening (human-ratified 2026-08-07, rides the
   next retro's change gate).** A chain closes on a round with zero
   UNREFUTED material findings: each refutation carries evidence, survives
   exactly one rebuttal round, and persistent disagreement escalates to
   the human. Today the stop rule counts what the critic returns, giving
   the critic an implicit veto over closure; with IL-21 forcing refutation
   attempts, that veto will start binding. Owner doc when adopted:
   `skills/design-critique/SKILL.md` (and code-critique's mirror section);
   the join script gains the unrefuted-count check. Wido: "I like the
   sharpening and I think we should implement it."
4. **KI-23 acknowledged-process mechanism** — after the one-writer
   implementation lands (same files).
5. **Mission-completion protocol design** (plans/mission-completion-protocol.md,
   seven carried findings) — after the coexistence stream closes.
6. **Benchmark kit halves** of the validity closure (schema v2 entries,
   extractor version dispatch; human-ratified) — with the next kit-gate
   batch.
7. **Confirming benchmark re-run** (Opus critics vs luna builders, the
   0.019→0.981 effect) — with the human present, after the above.
8. **Devin integration** — UNPARKED 2026-08-07: the one-writer fix is
   implemented, proven by both gates, and pushed. Next: the probe and
   `development/devin-selftest.md`; a `hosts/devin.sh` is a separate build.

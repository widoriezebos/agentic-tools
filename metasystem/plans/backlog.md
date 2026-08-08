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
2. **Critique stop-rule sharpening (human-ratified 2026-08-07, rides the
   next retro's change gate).** A chain closes on a round with zero
   UNREFUTED material findings: each refutation carries evidence, survives
   exactly one rebuttal round, and persistent disagreement escalates to
   the human. Today the stop rule counts what the critic returns, giving
   the critic an implicit veto over closure; with IL-21 forcing refutation
   attempts, that veto will start binding. Owner doc when adopted:
   `skills/design-critique/SKILL.md` (and code-critique's mirror section);
   the join script gains the unrefuted-count check. Wido: "I like the
   sharpening and I think we should implement it."
3. **KI-23 acknowledged-process mechanism** — after the one-writer
   implementation lands (same files).
4. **Mission-completion protocol design** (plans/mission-completion-protocol.md,
   seven carried findings) — after the coexistence stream closes.
5. **Benchmark kit halves** of the validity closure (schema v2 entries,
   extractor version dispatch; human-ratified) — with the next kit-gate
   batch.
6. **Confirming benchmark re-run** (Opus critics vs luna builders, the
   0.019→0.981 effect) — with the human present, after the above.
7. **Devin integration** — UNPARKED 2026-08-07: the one-writer fix is
   implemented, proven by both gates, and pushed. Next: the probe and
   `development/devin-selftest.md`; a `hosts/devin.sh` is a separate build.

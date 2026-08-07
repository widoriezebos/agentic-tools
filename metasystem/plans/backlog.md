# Backlog

- Goal and current status: the standing queue of ratified-but-not-scheduled work. Items leave by becoming a design stream, a retro proposal, or a deliberate rejection recorded here.
- Next step: none
- In flight right now: nothing
- Waiting on: nothing — items here are deliberately queued

## Queued

0. **Finish KI-24's fixture alignment, then push the batch.** The only thing
   between the current state and a green, pushed, production-ready base. The
   rule is settled and applied to the supervision and mission suites: a fixture
   that performs control-plane writes announces itself as the checkout's main
   first. Applying it surfaced seven further product defects, all fixed (see
   KI-24). After green: full suite, kit gate, push the batch, then Devin.


1. **Critique stop-rule sharpening (human-ratified 2026-08-07, rides the
   next retro's change gate).** A chain closes on a round with zero
   UNREFUTED material findings: each refutation carries evidence, survives
   exactly one rebuttal round, and persistent disagreement escalates to
   the human. Today the stop rule counts what the critic returns, giving
   the critic an implicit veto over closure; with IL-21 forcing refutation
   attempts, that veto will start binding. Owner doc when adopted:
   `skills/design-critique/SKILL.md` (and code-critique's mirror section);
   the join script gains the unrefuted-count check. Wido: "I like the
   sharpening and I think we should implement it."
2. **KI-23 acknowledged-process mechanism** — after the one-writer
   implementation lands (same files).
3. **Mission-completion protocol design** (plans/mission-completion-protocol.md,
   seven carried findings) — after the coexistence stream closes.
4. **Benchmark kit halves** of the validity closure (schema v2 entries,
   extractor version dispatch; human-ratified) — with the next kit-gate
   batch.
5. **Confirming benchmark re-run** (Opus critics vs luna builders, the
   0.019→0.981 effect) — with the human present, after the above.
6. **Devin integration** — PARKED by the human until the one-writer fix
   is implemented and proven.

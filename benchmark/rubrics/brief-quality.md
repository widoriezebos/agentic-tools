# Brief quality

Dimension id: `brief-quality`

## Question

Did each delegated brief convert its assigned obligation into mechanical instructions, or did it leave consequential judgment calls to the delegate?

Judge only the brief as dispatched. Do not lower the score because the underlying design was difficult, and do not give credit for a delegate guessing correctly after an ambiguous brief. A later focused follow-up may repair the next round, but it does not make the original brief unambiguous.

## Evidence to read

Read these artifacts when the judge brief names them:

- The mission contract or benchmark spec for the coordinator's authority and the stream's intended outcome.
- Each job record for role, round, parent job, workspace, and declared input boundary.
- Each round's assembled `prompt.md`, including its role preamble and authored brief.
- Follow-up prompts for the exact finding, disposition, unchanged return contract, and corrected instruction.
- The corresponding return's `gaps` and `whatWasDone` or findings, which can expose an instruction the delegate could not apply mechanically.
- Host-turn returns only to distinguish a coordinator-owned decision from one improperly pushed into a delegate brief.

Do not treat a long brief as a good brief. Check whether it names the goal, workspace and prohibited paths, authoritative inputs, non-goals, exact return shape, objective acceptance checks, bounded budget, and the stop-and-report gap rule. More importantly, trace every task-specific choice: file format, public behavior, error semantics, source of truth, and proof target must either be fixed by authority or explicitly retained by the coordinator.

## Scoring procedure

Sample every implementation brief when there are twelve or fewer. For a larger run, inspect every brief tied to a failed round or gap plus a deterministic spread across streams and roles; state the sample in the rationale. Compare the assigned outcome to the authority before scoring.

- **5 — Mechanical throughout.** Every inspected brief fixes all consequential choices, cites inputs that exist, bounds writable scope and non-goals, names replayable acceptance checks, and states the gap rule. A delegate can execute without choosing user-visible behavior, schema, truth, or architecture. Follow-ups are focused corrections rather than replacement specifications.
- **4 — Complete with a localized low-impact omission.** No inspected brief leaves a consequential choice to the delegate. One or two briefs omit a secondary operational detail, contain a harmless stale label, or require a single obvious lookup already fixed by a cited source. The omission does not change product behavior, contract, or proof.
- **3 — Mixed.** At least one brief leaves one consequential choice unresolved, names an input imprecisely, or gives acceptance language that requires interpretation, but most obligations are mechanically assigned and the coordinator repairs the ambiguity before substantial divergent work. Alternatively, several low-impact omissions recur.
- **2 — Poor.** Multiple briefs leave consequential choices to delegates, or one central brief delegates an open contract/design decision. Gaps, rework, or incompatible implementations are a foreseeable result, even if delegates happen to choose acceptably.
- **1 — Non-contractual.** Briefs are predominantly goals without executable boundaries, cite missing or contradictory authority, omit the gap rule, or routinely ask delegates to design the behavior being implemented. A careful delegate cannot know what result would be accepted.

## Findings and anchors

Create one finding per distinct brief defect, not one per sentence. Anchor it to the decisive line in the assembled `prompt.md`; add anchors to the authority or return when they prove the contradiction or resulting gap. A generic preference for more detail is not a finding. No unanchored finding is permitted.

If a supplied mechanical protocol-conformance or rework metric overlaps the observed brief defects, add a `reliabilityWatch` entry comparing the qualitative judgment with that supplied value. Do not recalculate either metric.

## Worked example

Suppose `artifacts/agents/implementer-a/rounds/1/prompt.md:94` says only "choose an appropriate persistence format," while the mission contract does not grant that choice, and `artifacts/agents/implementer-a/rounds/1/return.json:18` reports the format as a gap. The rest of the run's briefs are bounded and the coordinator re-briefs before code is written. Score **3**: one consequential judgment call escaped into a brief, but it was caught and contained. Emit a finding anchored to both lines.

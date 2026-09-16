# small-change-lane design fold r3 - FINAL (m1e seat, 2026-09-16 ~11:05 CEST)

You are the design author for goal small-change-lane (Claude Fable delegate for seat m1e). S5 = /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/5afd308c-3804-42b1-87c4-4547e1460fa9/scratchpad. R0 = /Users/wido/LocalStorage/GitHub/agentic-tools-m1e.

## Read
1. S5/scl/scl-r2-design-prompt.md - the frame for revision 2. Everything in it still binds (the goal record, Wido's four answers of 2026-09-16, the seat rulings, the brief and context pack it names) except where this prompt says otherwise.
2. S5/scl/small-change-lane-design-r2.md - your revision 2 (the two seat rulings in it are CONFIRMED and stay).
3. S5/scl/small-change-lane-critique-r2.md - Codex critique round 2 of 3: VERDICT rework, 12 material findings (SCL-R2-M01..M08, MOVED-EFFECTS-SCL-R2-M09, SCL-R2-M10..M12) and five prior findings Not closed (R1-M05, M08, M12, M13, M14, each mapped to an R2 finding).

## Task
Write revision 3 to S5/scl/small-change-lane-design-r3.md. This is the FINAL fold: there is no round-3 critique. The seat will check each material finding element by element against the critique's own reopening trigger (its "Rigor classification" table) and then land the page, so every fold must be checkable against that trigger text.
- Verify each finding against the code at origin/main before folding (the critique cites file:line; check them - a finding that is factually wrong is rejected with evidence, not folded).
- Fold each material finding so its reopening trigger is met in the page: the rule, the refusal code, the unit that builds it, and the named witness.
- M04 says LANE_WIDTH and LANE_SURFACES contradict Wido's decided contract (tier + 4 files/80 lines box + refusal/protected/ENGINE clause + reader). Remove them unless Wido's recorded answers in the goal file say otherwise; if you believe one is needed, keep it OUT of the page and put it in the question list below.
- M08 (critical) and M12: read scripts/agents/land.sh, internal/validate/conformance.go and metasystem/memory/rulings.md R-117-m1e item 5 yourself; design a sequence that actually lands the reviewed tree with its receipt in the same commit. For DONE (4), if an injected-clock active-seat-time record is too large for this goal, narrow the measurement honestly (and say what DONE (4) then shows) rather than keep wall elapsed time; name an eligible specimen or state that the first real lane goal is the specimen.
- M09: add the mandatory "Moved effects" section with the `| Effect | From | To | Code |` table naming one destination owner for tierOneDiffMetric.
- M11: paired same-role evidence across Codex and Claude; claim hash equality only for the same template and input.
- Keep the page's structure, units and allocations coherent after the folds; re-check the unit table sums and ordering (U0 first).
- Add a findings table at the end: | Finding | Disposition (folded/rejected) | Where in r3 (section and line) | How the reopening trigger is met |.
- If any finding needs a decision only Wido or the seat can make, do NOT decide it on the page: list it under "Questions for the seat" at the end with the smallest question and your recommendation.

Do not run tests or engine commands. Do not write outside S5/scl/. End your final message with one line: DONE scl-r3 folded=<n> rejected=<n> questions=<n>.

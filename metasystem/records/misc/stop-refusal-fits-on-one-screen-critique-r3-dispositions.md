# stop-refusal-fits-on-one-screen, critique round three: dispositions

Chain one-screen-build1b-20260909. The third critic,
one-screen-crit3-20260909 (claude-opus-5), reviewed the second fold
one-screen-build1b-20260909-r3 at tree
c13c27fa26d895cc1c1251caa3e6ebfef4baa9f6 and returned one material
finding and two notes. This was the third review round the tier-3 box
allows, so the register closes here with the material finding deferred
into a review obligation on the goal, as docs/orchestration.md
prescribes when the rounds are spent; the chain lands on this tree.
The orchestrator ran conformance, the coverage floors and the hook
suite seat-side on this exact tree before the review: internal/goal
82.4 percent (floor 82.2), internal/report 87.6 percent (floor 87.3),
scripts/agents/supervision-hook-fixtures.sh green with a rebuilt
engine.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| OSR-08 | accepted | Deferred as a review obligation on the goal, the review rounds being spent. Real as reproduced: pairing fence lines by position treats only the LAST of an odd count as text, so a stray opener that comes BEFORE a closed example slides the pairing by one. Latent: no plan under plans/ or docs/ has an odd fence count today, and for every well-formed file the landed rule is correct where main today reads fields from inside fences. The premise that the field after a stray first fence is "real" is not settled either: under CommonMark an unclosed fence runs to end of file, so that field is fenced text. | The obligation's fix, stated for the discharge: adopt the CommonMark rule (a fence opens at a fence line, closes at the next, an unclosed fence runs to end of file, so no pairing heuristic), and add one diagnostics line naming the plan and the line of the unclosed fence so the plan never vanishes from the refusal silently. The critic's two plans (stray-first and stray-last) become the tests; under the rule the stray-first plan reports nothing and is named in diagnostics. |
| OSR-09 | noted | An indented fence (one to three spaces) is a fence in Markdown and the reader's pattern is anchored at the margin; a margin-level field inside such an example would be reported. No plan in the tree has one. | Folded into the OSR-08 obligation's discharge: recognise up to three leading spaces. |
| OSR-10 | noted | A four-backtick block is not recognised; a field inside it outside an inner three-backtick pair would be reported. No plan in the tree has one. | Folded into the OSR-08 obligation's discharge: a fence is three or more backticks and closes on a fence of at least the same length. |

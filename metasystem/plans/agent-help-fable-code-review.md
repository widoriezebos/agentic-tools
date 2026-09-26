# Agent help — code critique round 1 (Fable 5.1, independent)

- Reviewed tree: 7bb5f7aa0e1a2b9598efb4e44396a5b4c0c0c426 (candidate commit 556676ff6, base 0a38b2e01)
- Full diff SHA-256: 9ce7481def7c8b798d607abee1e1a538aa84e11594f9fd72722ac14e361249cf (recomputed from `git diff base..candidate`; matches review-subject.json and full.patch)
- Material findings: 0. The loop closes at round 1 under the declared early exit.

## Conformance (design → code, read, not recited)

| Obligation | Design requirement | Where it lives | What I verified |
| --- | --- | --- | --- |
| AH-1 | Human help byte-identical; agent page keeps all public commands, ≤75 lines, <1,073 words | `intent_help.go:106-125`, `intent.go:982` | Built the candidate; bare, `help`, `help review`, `help human`, `help all` are byte-identical to the captured baselines. `help agent` is 69 lines / 748 words and lists 44 public commands with audience labels. |
| AH-2 | Same descriptors, visible flags, shared envelope; no internals; no execution; works outside a repository | `intent_help.go:147-253`, `main.go:787` | Router runs before repository resolution; the render-only invocation carries no root or owners. From `/tmp`: `help all --json` returns 44 entries; `help internal --json`, `help unit --json` (compatibility) refuse with the public envelope, exit 2; `help --json --json` is harmless; `help -h`/`help review goal extra` refuse with exit 2 and empty stdout. Hidden flags (`effort`, `lineage`, `fixture-human-authority`) never enter JSON. |
| AH-3 | Eight review forms; diagnostic changes/diff exclude submission flags; contextual option notes never change parsing | `intent_review_help.go`, `intent_delivery.go:45-47` | Form usages are the same nine constants the human page prints, in the same order. `changes`/`diff` expose only brief, goal, retry (+repo/json). Every form flag name resolves against `allFlags()`; the goal flag (`intent_delivery.go:41`) description is mode-neutral. Traced the examples against `runIntentReview` (`intent_delivery.go:516-532`) and `runIntentReviewGoal` (`intent_selection.go:463-543`): `review goal G --changes` submits, `review goal G --finding` discharges, `--retry` conflicts with `--dispositions` exactly as the goal form says, `--model` on the goal path is only consumed when a manual/built work is chosen. |
| AH-4 | Decision is a prerequisite, not always human-only; next.argv not permission; outcomes from constants | `intent_help.go:83-104` | Protocol text says decision "describes a prerequisite: supply known missing input within your authority; ask the person for human-only acts". Outcome names are the six constants; the test renders each through the real renderer and checks the exit class. |

Tests: `TestIntentAgentHelpCatalogue`, `TestIntentHelpJSON`, `TestIntentHelpRefusals`, `TestIntentHelpNeverExecutes`, `TestIntentReviewHelpForms`, `TestIntentAgentResultProtocol` plus the existing help-alias, family-help and public-discovery tests pass (`go test -count=1 -run ...`, 1.3 s). They are registered in `testing.json` under command-interface-smoke. The sentinel test panics on any handler or repository resolution, so the no-execution boundary is a real proof, not a shape check.

## Material findings

None. Every defect candidate I traced either does not exist or passes both WORKS and SAFE without a change.

## Nonmaterial notes (do not keep the loop open)

- AH-N1 `writeIntentHelpForm` (`intent_help.go:256`) hardcodes the `review` prefix in the text header. Only review owns forms today, so the output is correct; a second command's forms would need the command name threaded through. WORKS and SAFE without it.
- AH-N2 `help -h` and `help --help` now refuse as "unknown help option" instead of "unknown command". Exit stays 2, stdout stays empty, and the design allows unknown options to refuse with help guidance. Top-level `-h`/`--help` are untouched.
- AH-N3 The submit form's `--work` inherits "the goal's named work to review". In a submission it names the work item the submission is recorded under (the form's repetition line says it defaults to main). The wording is not commit-only or goal-only and does not mislead about authority; the root's stated final correction can include it if cheap.
- AH-N4 `parseIntentArgs` in `TestIntentReviewHelpForms` proves argument shape only; the handler effects I traced by reading `runIntentReview` and `runIntentReviewGoal`. The design already assigns effect proof to the existing real-owner fixtures, which the root runs.

## Limitations

- I did not run the real-owner review workflow fixtures or the full command-interface-smoke group; root owns acceptance.
- I byte-compared five human pages (bare, `help`, `help review`, `help human`, `help all`) against the captured baselines, not all 53 captured calls; the runtime summary reports 53 identical and nothing I read contradicts it.
- The focused `go test -run` reported `ok` without `-v`; I did not list per-test execution, but the regex names existing functions in the diff.
- No model benchmark or fresh-agent task trial was assessed; out of scope by brief.

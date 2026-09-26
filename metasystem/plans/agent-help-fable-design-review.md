# Design critique r1: agent-help.md (goal verbs-match-intent)

- Critic: Fable 5.1, independent, round 1 of at most 2 (round 2 is fixed by the failsafe).
- Design read: `metasystem/plans/designs/agent-help.md` at worktree `codex/agent-help-20260926`, base 0a38b2e01, 153 lines, status draft. Resolver `metasystem internal project design-of --goal verbs-match-intent` names it as the only draft; `plans/designs/verbs-match-intent.md` is superseded, three intent designs are done.
- Scope critiqued: step 1 only (compact `help agent`, public `--json` discovery, eight `help review FORM` contracts), under human-byte preservation and no execution/policy change.
- Every cited line below was read in this worktree, not recited. Ranges: `main.go:781-819`, `intent.go:30-66, 229-254, 520-562, 571-660, 895-1055`, `intent_delivery.go:40-110, 514-560, 589-591`, `intent_manual_review.go:16`, `ui_describe.go:87-95`.

## Verdict

- materialCount: 2 (AH-D1, AH-D2). Five nonmaterial clarifications.
- Step 1 WORKS as written: the router already handles help before repository resolution, the envelope and outcome constants exist, the review descriptor's nine usage strings map onto the eight forms, `--check` is the one `rest` flag, and `help --json` refuses today through `writeUnknownIntentCommand` with exit 2.
- Step 1 is SAFE as written except on two edges: the protocol never names the envelope's `decision` field (the person-only channel), and the "changes" form is not pinned to the diagnostic usage string while the identically named `--changes` flag means submit.
- The smallest direct help change does suffice. Adding form metadata as prose in the review descriptor does not create a second policy owner, because nothing reads it for enforcement; see AH-D5 for the proof wording that keeps it that way.

## Findings

### AH-D1 — MEDIUM — material: true — the result protocol omits `decision`, the only field that says "a person acts first"

Claim. The agent guide and the JSON envelope description (design lines 62-72, 80-81) name outcome, summary, data, targets and next.argv, never `decision`. The envelope has `Decision string json:"decision,omitempty"` (`intent.go:599`) and text renders it as `needed first:` (`intent.go:653`). It is set exactly where a person must act: repository not inside an installation (`intent.go:559-562`), `--effort` and `--model` refusals (`intent_delivery.go:551-558`), the legacy ledger upgrade (`intent.go:571-572`, whose sentence contains a runnable command `metasystem repair goals --upgrade --by NAME`).

Evidence. Text mode prints `run:` for next, else `needed first:` for decision (`intent.go:647-657`); JSON carries both. The guide's own rule "human help when the next action needs a person" has no field to trigger on.

Why material. An agent working from the built guide has a rule for next.argv but none for decision; the decision sentence is a person's act phrased as a command, so an agent may perform it in the person's place. That is invented authority, which the guide exists to prevent.

Amendment. Add `decision` to the envelope list (line 80) and one protocol bullet: "decision names an act reserved for a person; report it to the person, never perform or emulate it, and never treat it as next.argv." Include `decision` in the agent/root index's protocol data.

WORKS without it: yes. SAFE without it: weakened on refused results.

### AH-D2 — MEDIUM — material: true — the "changes" form is not pinned to its usage string, and its name collides with the submitting `--changes` flag

Claim. Lines 94-99 name eight forms but never state which of the nine usage strings (`intent_delivery.go:45-46`) each form owns. The code makes `review changes --brief FILE` diagnostic (`intent_delivery.go:527`), `review G --changes` and `review goal G --changes` submitting (`:517-520`), and `review changes --changes` a refusal (`:521-522`). The `--changes` flag's usage text reads "review G: submit this checkout's current changes as the goal's work" (`:74`).

Evidence. Line 103-104 says "Option definitions come from the command's existing flags, selected by exact names for each form." An implementer selecting flags for the form named `changes` by exact name can pick the flag `changes`, and the diagnostic form then lists a submitting option its own argv refuses. Nothing in the design rejects that build.

Why material. AH-3's required behavior is "distinguish diagnostic feedback from submission." A "changes" form that shows or lists `--changes` answers wrong on the one distinction the slice exists for, and the wrong answer commits and publishes work.

Amendment. Add a pinned mapping table to the "Structured discovery and focused contracts" section: goal → usage 1 and 9; submit → usage 2; finding → usage 3; design → 4; job → 5; commit → 6; changes → 7; diff → 8. State that the changes and diff forms exclude `--changes`, `--patch`, `--work`, `--dispositions`, `--finding`, `--test` and carry the sentence "a goal id before --changes or --patch submits instead; see help review submit." Require `TestIntentReviewHelpForms` to assert each form's example argv shape: diagnostic forms start with `changes` or `diff` and contain no `--changes`/`--patch`; the submit form names a goal placeholder and one of those flags. Fixture-expressible.

WORKS without it: yes if the implementer happens to choose right. SAFE without it: no guarantee.

### AH-D3 — LOW — material: false — `help review --json` shape is described two ways

Line 84 says data is "either a compact commands index, one command description, or one focused form"; line 98 says `help review --json` "lists the available form names and their purposes." Clarify: it is the ordinary command description with an added `forms` array (name, purpose, help argv). Otherwise an implementer may return a forms-only object and `help review --json` gives less than `help goals --json`.

### AH-D4 — LOW — material: false — root index scope

Line 85 says "the agent/root index." Root text help lists primary commands only (`intent.go:910-915`); the new `help agent` lists all 44. State that `help --json` and `help agent --json` return the same complete index, and `help all --json` too, so a first `help --json` call never hides thirty commands.

### AH-D5 — LOW — material: false — say what "actual parser and declared effect boundaries" can prove

The submit/diagnostic decision is inline in `runIntentReview` (`intent_delivery.go:516-531`); there is no pure classifier, and the design forbids execution-parser changes (line 123). So the test can prove flag names resolve through `lookupFlag`, usage strings parse under `parseIntentArgs`, and argv shape (AH-D2). It cannot prove routing without execution. State this so the code critic does not demand a mechanical tie and the implementer does not add a classifier, which would be the second policy owner the design rightly avoids.

### AH-D6 — LOW — material: false — `review unit RUN` is a live subject with no usage string and no form

`reviewSubjectWords` includes `unit` (`intent_manual_review.go:16`), `case "unit"` exists (`intent_delivery.go:591`), and the `--model` usage says "unit and commit review", yet no usage string mentions unit. Pre-existing gap; the forms are complete against usage, not against execution. Name it under "Deferred" so it is a known omission, not a discovered one.

### AH-D7 — LOW — material: false — Partner help line is hand-written

`ui_describe.go:92-93` hardcodes the help grammar text. It will not mention `--json` or `review FORM`. Optional to update; not part of the unchanged-human-help contract.

## Notes for the implementer, not findings

- `intent_test.go` and `intent_coverage_test.go` reference the current agent-help heading; they change with `help agent` by design.
- The refusal envelope for help must be built without an `intentInvocation`; `render` is a method on one. Marshal `intentResult` directly with `Verb: "help"`, `code 2`, and keep `Targets: []`.

## Limitations

- No binary was run for help output; the router was read instead. Baselines are root's captured files under the evidence directory (`before-help-agent.txt` and siblings), not re-measured here.
- No tests run. No subagents. 8 tool calls of 24. No product edits.

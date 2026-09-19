# retro-flags-repeated-critique-findings: design, revision 1

- Goal: retro-flags-repeated-critique-findings (tag rfc), tier 2, priority 1, sequence 8
- Revision: 1 (first draft), dated 2026-09-16
- Author: Claude Fable 5.1 design delegate, launched headless by seat m1e
- Budget used: 21 tool calls of 30; page under 2500 words
- Prior pages for this goal: none (grep of `plans/` and `records/misc/` for the goal id finds only the goal page and priority-order history lines)

## 0. The round boundary

The Intent's DONE, designed exactly and no more:

1. Every critique and code-read receipt carries the ids and normalized titles of its material findings.
2. The retro reports any finding that appears in two or more critique records of different goals, or of one goal after a fold, naming the goals, the rounds and each disposition.
3. Proof: a fixture with two synthetic critique records sharing a finding and one retro run that reports it, plus one run over differing records that reports nothing.

Interpretation, stated so it can be checked: the "critique record" the retro matches is the receipt line, not the Markdown critique page. The retro's inputs are the receipts (`skills/retro/SKILL.md:12`), and the Markdown pages have no joinable shape: a survey of the 252 critique files under `records/misc/` found more than twenty-five heading dialects (`### SSTB-101`, `## GAW-3 — high, material=True`, `### SMA-C-2 (material: True, severity: high)`, ...). The "retro run" is the retro's mechanical input, a receipt verb, because the retro itself is a skill run by an agent.

## 1. What the code does today

- A receipt is one pipe-separated line; optional fields ride between `built_by` and `critique_waived`: `internal/receipt/receipt.go:218-239`. Every free-text value is newline-flattened (`:90-93`) and pipe-replaced (`:95`).
- `read_tokens`, `read_calls`, `design_tokens`, `design_calls` are optional, numeric (`epochRe`), validated on add (`:183-194`) and on correct (`:283-287`), wired as flags in `cmd/metasystem/receipt_verbs.go:56-59`, carried in `Options` (`receipt.go:43-46`).
- `--delegate` is the one repeatable flag and already uses colon-joined triples separated by commas (`receipt_verbs.go:50-53`, `receipt.go:200-207`); `delegate=none` is the explicit empty value.
- `Stats` reads only `RECEIPT` lines since the last `RETRO` line unless `--all` (`receipt.go:350-357`), parses `key=value` fields (`:362-398`), and prints one `key=value` per line (`:400-419`). Corrections are never applied by any reader today.
- The action switch is `receipt_verbs.go:99-113`; usage text at `:24`.
- Finding ids and dispositions already have an engine vocabulary: the critic's `return.json` carries `findings[].id` and `findings[].material` (`internal/validate/critiqueclosed.go:129-145`), and the dispositions table admits `accepted`, `refuted`, `noted`, `out-of-scope` (`:246`), with `noted` refused for material findings (`:42-43`). Ids are per goal tag and encode the round (SSTB-1xx is round 1, SSTB-2xx round 2 in `records/misc/seats-spend-tokens-in-bounded-sessions-critique-r1.md:5` to `-r3.md:9`), so an id never recurs across goals and rarely across rounds. Only the title can carry recurrence.
- The retro skill mines receipts for "verification repeatedly catching the same class of issue" (`skills/retro/SKILL.md:29`) with no mechanical help.
- Today's ledger has 7 `type=review` receipts, 3 receipts with `read_tokens`, none with `design_tokens`; the two receipted critique rounds record findings only as counts in the note (`memory/receipts.log`, alert-escalation-channel lines of 2026-09-01).

## 2. Decisions

### 2.1 Receipt field shape

New optional receipt fields, in this order after `design_calls` and before `critique_waived`:

```text
|round=<n>|findings=<element>,<element>,...
```

or `|findings=none`.

- `--round <n>`: the critique or read round, numeric like the other metrics. Optional, never required.
- `--finding <id>:<disposition>:<title>`: repeatable, one per material finding. The engine splits on the first two colons, so titles may contain colons.
  - `id` matches `^[A-Za-z0-9][A-Za-z0-9._-]*$` (the critic's own stable id, `SSTB-101`, `STOP-R2-4`, `CCB-FU-1`).
  - `disposition` is one of `accepted`, `refuted`, `out-of-scope`, the validator's vocabulary minus `noted`, which a material finding may not carry (`critiqueclosed.go:42`).
  - `title` is normalized by the engine into a slug: lowercase; every run of characters outside `[a-z0-9]` becomes one hyphen; leading and trailing hyphens trimmed; empty after normalization is refused. Normalization has one owner, the engine, so two seats writing "Stop rule cannot enforce the cap" and "stop-rule cannot enforce the cap." produce the same slug.
  - Serialized element: `id:disposition:slug`. Because the slug contains only `[a-z0-9-]` and the id cannot contain `,` `:` `|`, the field parses back with a split on `,` then a split on the first two `:`.
- `--finding none` alone serializes `findings=none`, mirroring `delegate=none`. Mixing `none` with a finding is refused.
- A receipt is a critique or code-read receipt when `type=review`, or when `read_tokens` or `read_calls` is present (the e700328e1 landing receipt was `type=implement` with `read_tokens`; that is the code-read receipt the DONE names). Such a receipt without any `--finding` is refused with one message naming the flag and the `none` form. This is the by-construction half of DONE item 1; the check runs after the existing numeric validations so their messages are unchanged.
- A `--finding` other than `none` requires `--goal`: the report must name goals, and a goalless finding can be neither same-goal nor different-goal.
- `receipt correct` accepts `--field round` (numeric) and `--field findings` (same element grammar, or `none`), like `receipt.go:283-287`.

### 2.2 The matching rule

`Repeats` reads `RECEIPT` lines only, as `Stats` does. Each element of a `findings` field yields one occurrence: slug, goal, round (or none), id, disposition, epoch, line index, and whether the line is after the last `RETRO` marker (in the current period).

A slug is reported when either holds:

- **Different goals:** its occurrences name two or more distinct goals, any dispositions.
- **Same goal after a fold:** two occurrences share a goal, the earlier one (by epoch, then line order) is `accepted`, and a later one carries the same slug. A fold that did not close the finding is exactly a critique record "of one goal after a fold". Refuted-then-refuted is not reported: that is the critic repeating itself, not a defect recurring.

And, unless `--all`: at least one occurrence lies in the current period. The earlier occurrence may be from any period, because recurrence across retro periods is the pile-up the goal names.

Matching is exact slug equality. Fuzzy or stopword matching is out of scope (section 5).

### 2.3 The report line

New action `receipt repeats` (shim: `scripts/receipt.sh repeats`), stdout, one line per reported slug sorted by slug, occurrences in ledger order, then a count line:

```text
repeat|title=<slug>|<goal>:r<round>:<id>:<disposition>|<goal>:r<round>:<id>:<disposition>
repeats=<count>
```

An absent round prints as `r-`. Over records with no shared slug the whole output is `repeats=0`. `receipt stats` gains one line, `repeated_findings=<count>`, computed by the same function, so a retro that reads only `stats` sees there is something to open.

### 2.4 Prose changes

- `skills/retro/SKILL.md`: the Inputs bullet at line 12 gains "`scripts/receipt.sh repeats` when `repeated_findings` is above zero"; Step 2 (line 29) names each `repeat` line as a pattern, never an anecdote, and names the goals and dispositions in the ledger row's evidence pattern.
- `skills/design-critique/SKILL.md`, Record section (line 116): each critique round is receipted as `type=review` with `--goal`, `--round`, and one `--finding` per material finding taken from the critic's finding id, its disposition, and its claim in at most eight words. Without this sentence DONE item 1 is vacuous, because today most rounds are not receipted at all.
- `scripts/agents/templates/review-brief.md`, findings artifact section (line 40): each finding carries a stable id and a title of at most eight words naming the defect class, the strings the seat passes to `--finding`.

## 3. Rules, witnesses, mutations

| Rule | Witness (all in `internal/receipt/receipt_test.go` unless named) | Named mutation that fails it |
| --- | --- | --- |
| R1 `findings=` and `round=` are absent when not given; present in the stated position and element form when given | `TestFindingsAreOptionalAndNormalized`: first line lacks the `round=` and `findings=` keys; second line contains `design_calls=..`, then `round=2`, then `findings=SSTB-201:accepted:stop-rule-cannot-enforce-the-cap,SSTB-202:refuted:u1-omits-the-owner`, then `critique_waived=` | emit `findings=` unconditionally; serialize the raw title instead of the slug |
| R2 title normalization | `TestNormalizeFindingTitle` table: `"Stop rule: cannot enforce the CAP."` to `stop-rule-cannot-enforce-the-cap`; `"  --  "` refused `invalid --finding: empty title` | remove `ToLower`; keep double hyphens |
| R3 element grammar refusals | rows in `TestAddValidation`: `SSTB-1:noted:x`, `SSTB-1:folded:x`, `SSTB-1:accepted`, `bad id:accepted:x`, each exit 2 with `invalid --finding: <value>` | drop the disposition switch |
| R4 `none` sentinel | rows: `--finding none` alone yields `findings=none`; `none` plus a finding is refused | accept the mix |
| R5 critique and read receipts declare findings | `TestReadAndReviewReceiptsRequireFindings`: `type=review` without findings refused; `read_tokens` without findings refused; `type=implement` without read fields and without findings accepted; message names `--finding` and `none` | remove the check; check `type=design` instead |
| R6 a finding requires a goal | row: `--finding SSTB-1:accepted:x` with no `--goal` refused | remove the check |
| R7 `correct` validates `round` and `findings` | `TestCorrectFindings`: `--field findings --now garbage` exit 2; `--now none` and a valid element accepted; `--field round --now many` exit 2 | drop the fields from the `Correct` switch |
| R8a different goals | `TestRepeatsAcrossGoals`: two synthetic receipts, goals `alpha` and `beta`, both with slug `stop-rule-cannot-enforce-the-cap` and one other slug each; output is exactly one `repeat` line naming both goals, rounds, ids, dispositions, then `repeats=1` | key on id instead of slug |
| R8b same goal after a fold | `TestRepeatsAfterFold`: goal `alpha` r1 `accepted`, r2 same slug, reported; goal `alpha` r1 `refuted`, r2 `refuted`, not reported | drop the `accepted` precondition |
| R8c nothing on differing records | `TestRepeatsNothingOnDifferingRecords`: two receipts, disjoint slugs, output exactly `repeats=0` | report any slug with one occurrence |
| R8d period window | `TestRepeatsPeriodWindow`: occurrence before a `RETRO` line and one after, reported; both before, `repeats=0` without `--all`, reported with `--all` | drop the period filter |
| R9 `stats` carries the count | extension of `TestStats`: `repeated_findings=1` on the R8a ledger, `repeated_findings=0` on the R8c ledger | remove the line |
| R10 verb wiring | `cmd/metasystem/receipt_verbs_test.go` `TestReceiptRepeatsVerb`: `runReceipt` with `add` twice (with `--round`, two `--finding`, `--goal`) then `repeats` on a temp `--file`; stdout contains the `repeat` line; unknown action still prints usage | leave `repeats` out of the switch |

R8a and R8c are the two runs the DONE names; R10 runs them through the real verb.

## 4. Units in landing order

**Unit 1 (first): the field.** `internal/receipt/receipt.go` (Options fields `Round`, `Findings []string`; `parseFinding`, `normalizeFindingTitle`; validation and serialization in `Add`; `Correct` validation), `cmd/metasystem/receipt_verbs.go` (`--round`, repeatable `--finding`), tests R1 to R7. About 70 engine lines, 8 command lines, 110 test lines: under 200 changed lines. Landing alone it already makes DONE item 1 true for every receipt written from then on.

**Unit 2: the report.** `internal/receipt/receipt.go` (`Repeats`, the shared `repeatedFindings` function, the `stats` line), `cmd/metasystem/receipt_verbs.go` (`repeats` action and usage), tests R8 to R10, the three prose edits of section 2.4. About 120 engine lines, 6 command lines, 130 test lines, 10 prose lines: under 270. Unit 2 depends on Unit 1's parser and lands second.

Both units are Codex gpt-5.6-sol builds in a worktree with an Opus read; the critique of this page is by another model.

## 5. Existing tests that change, and why

- `TestReadMetricsAreOptionalAndNumeric` (`receipt_test.go:94`) adds `--finding none` to its second receipt, because R5 now refuses a read receipt without findings; its shape assertions (`:110-115`) stay true since `findings=` serializes after `design_calls`.
- `TestAddValidation` (`:185`) gains the rows of R3, R4, R6; its existing `ReadTokens = "16m"` rows keep their messages because R5 runs after the numeric checks.
- `TestStats` (`:359`) gains the `repeated_findings=` assertion.
- `TestO13ReceiptProvenanceParsesWithOldRows` (`:65`) is unchanged and is the proof that old ledger lines still parse.

## 6. Out of scope, named

- Fuzzy or stopword title matching; a `--titles` lookup that offers earlier slugs to a seat writing a receipt. Follow-up if exact matching proves too strict in the first retro that uses it.
- A `title` field in the critic's `return.json` and its agent definition; the seat takes the title from the finding's claim.
- Applying `CORRECTION` lines in `repeats`; no reader applies them today, and `repeats` mirrors `stats`.
- Backfilling `findings=` onto existing receipts.
- Same-goal repeats whose earlier disposition is `refuted` or `out-of-scope`.

## 7. Risks and open questions for the seat

1. **Landing path.** `metasystem landing receipt-line` bounds the receipt command (`docs/orchestration.md:163`). Not checked within budget: whether it allowlists flags. If it does, `--round` and `--finding` must be admitted in Unit 1 or the first landing receipt after Unit 1 is refused. The builder greps `receipt-bound` in `internal/` before writing Unit 1.
2. **R5 lands as a refusal.** Any seat still landing from a brief written before Unit 1 and passing `--read-tokens` without `--finding` gets a refusal with the exact flag to add. Alternative if the seat prefers no refusal: a `reads_without_findings=` count in `stats`, treating the gap as a retro finding (`skills/retro/SKILL.md:52`). Recommendation: the refusal, because DONE item 1 says "every".
3. **Where dispositions come from for a code read.** A closing read's findings file has no dispositions table; the seat records `accepted` for a folded finding and `refuted` for one it answered. If the seat wants that written down, it belongs in `review-brief.md` next to the title sentence in Unit 2.

Self-grade: the field shape and the report follow existing conventions line for line (`delegate` triples, `none`, numeric metrics, the `stats` window), so novelty is confined to the slug normalization and the two-clause matching rule, each with a mutation-killing witness. Reject condition: if the first real retro finds that seat-written titles never coincide across goals, the exact-match rule is too strict and the `--titles` follow-up becomes the next goal.

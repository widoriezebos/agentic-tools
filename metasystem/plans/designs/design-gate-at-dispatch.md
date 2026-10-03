# Design for design-gate-at-dispatch

- Kind: design
- Id: 01M4189Q0RH1NSPD3PNAS6G177
- Status: draft
- Goals: design-gate-at-dispatch

Revision 1 (first draft). Author: Fable (claude-fable-5-1), design delegate of seat m1k, 2026-10-03. Cites were read at main `19a4f3335`; paths are under `metasystem/`.

The goal, as Wido approved it: "Work that needs a design cannot start as a build unless an accepted, critiqued design stands behind it." Folded in on 2026-10-03 at his word ("drain the waiting lane"): the landing half (`landing-design-provenance`), and a governance record for this gate and for `commit-goal-binding`'s refusal, warning-only first (`gate-governance-records`).

## 1. What is true today

- **The live door.** `metasystem work build G` (`cmd/metasystem/intent_work.go:197-235`) runs `runIntentBuildUnit` (`:494`): goal live (`:562`), approved budget (`:567`), claim (`:572-580`), this session holds it (`:587`), then the accepted designs (`:592`), the request (`:598`) and the unit runner (`:623`). In the engine, a unit with no run yet calls the caller's `prepare`, which writes the plan and briefs, and keeps `request.json` (`internal/launch/unit_named.go:165-174`); the first round is admitted and the run id reserved at `:228-241`.
- **A live example of the gap.** Goal `landing-deploys-the-engine` is tier 2. On 2026-10-03 at 14:41 its work `engine-adapter` went through this door with `"designs": []` in its kept request, while its design was still a draft in critique. Wido accepted that design at 15:20 (`ef96e62fd`); the next work, `chain-status` at 15:34, carried it with its digest. Nothing was printed for the first build.
- **Where the request is kept (corrects the brief).** Not in the checkout: the unit store is `~/.metasystem/unit` (`internal/launch/unit_run.go:837-843`, `record.go:128-134`), one per computer, each unit's folder keyed by worktree, goal and unit (`unit_named.go:144,449-455`).
- **A second, hidden door.** `work build --plan FILE` (a hidden flag, `intent_work.go:224`) goes through `runIntentBuildPlan` (`:464-487`) and never reaches `:592`.
- **Design handling at build.** `acceptedDesignPaths` (`:877-905`) keeps the records with `Status: accepted`; with none the brief stands in (`:994-996`). A project whose records cannot be read already fails the build there (`:879-883`). The request identity stores each design's `{path, sha256}` (`:701-721`); a repeat with other request bytes is refused as `UNIT_NAMED_INPUT_CHANGED` (`unit_named.go:160-164`).
- **Tier.** `goal.GateTier` (`internal/goal/landgate.go:95-100`; no tier reads as 3) is read today only by the landing gate. Design critique exists at tiers 2 and 3, not at tier 1 (`docs/orchestration.md:69`).
- **The design record.** Head `Kind`, `Id`, `Status`, `Goals` (`docs/design/design-obligation-gate.md:19-44`). `Goals` takes several ids (`internal/project/record.go:199`); a head key the reader does not know is kept and read by nothing (`record.go:47-48,75`); status is set by hand (`project.go:56-61`). `linkedDesigns` returns id, path, status and title (`cmd/metasystem/intent_goals.go:281-301`).
- **Critique closure is prose today.** Of the 11 accepted records, four open their body with a sentence such as "Critique: Codex Astra, chain design-critic-09e9…, closed at round 4 on 0 material findings" (`plans/designs/flaky-leftovers.md:8`); the others use a revision line or name a ruling. The machine's own state, `chainClosed` (`internal/dispatch/design_chain.go:26-67`), sits in the checkout's ignored `artifacts/` and does not follow a claim.
- **Landing.** `landGoalRoute` (`cmd/metasystem/intent_delivery.go:1837`) asks `admitLanding` (`:1903`; `goal.Gate`: holds, tier, a person's word, `landgate.go:240-270`), then hands in to the lane (`:1907`) or lands by hand (`:1909`). The precedent for a computed check that does not refuse is `chain-not-design-bearing` (`internal/landing/observe.go:41-44,1724-1735`).
- **A person's exception per goal.** `goal.Permissions` (`internal/goal/permissions.go:29-38`): "a new permission is one row here and one field in the record", parsed at `file.go:1190-1195`, written at `:1735-1736`. An engine that does not know a field reports `unknown field` (`file.go:1536-1537`).
- **Warnings and refusals.** The refusal register has a `Warning` shape, "prints and records the warning, then complies" (`internal/refusal/register.go:24`); no row uses it yet. `commit-goal-binding`'s refusal is `GOAL_BRANCH_NOT_HOLDER` (`internal/goal/branch/commit.go:17,70-91`); when the claim cannot be checked it refuses as well (`:73`).
- **The sweep already does the governance half.** Each steward tick (`internal/steward/tick.go:261`) runs `sweepRulingReviews` (`ruling_sweep.go:210-241`). A row of `memory/rulings.md` whose review column is `class=… due=YYYY-MM-DD` (`internal/rulings/rulings.go:109-141`) and whose date is today or past (`ruling_sweep.go:113`) gets a lowlight in the narrator's digest: "Ruling review sweep: ID owner=… class=… due=… choice=adopt|revise|withdraw" (`:164-167`).

## 2. Step 1: one check, a warning, a record, and the same at landing

Everything new is in `cmd/metasystem/intent_design_gate.go` plus the call sites named below. As shipped it stops nothing.

**The evidence that travels.** Whoever sets `Status: accepted` adds one head line in the same edit:

```markdown
- Critique: closed at round 4 on 0 material findings (Codex Astra, chain design-critic-09e9…)
- Critique: ruled by Wido 2026-10-03, R-144-m1h
```

The first word is `closed` (the critique loop ended) or `ruled` (a person accepted the design; a person's decision is never blocked, R-142-m1e). The head grammar admits the line today, it is committed with the record, and it is as trustworthy as the hand-set status beside it. `linkedDesigns` gains the line's value. Local chain state may contradict the line; it never replaces it.

**The check.** It runs at `intent_work.go:592`, in the command and not in the engine, because the engine knows neither the tier nor the records. Its inputs: the goal's tier (`goal.GateTier`), its permissions, its design records, and this checkout's critique chains for each accepted page (`dispatch.DesignCritiqueChains`). The first matching row is the verdict:

| Verdict | When | Would refuse |
| --- | --- | --- |
| `not-design-bearing` | tier 1 | no |
| `allowed` | the goal holds `build-without-design` | no |
| `no-accepted-design` | no record with `Status: accepted` names the goal | yes |
| `critique-not-recorded` | an accepted record has no `Critique` line starting `closed` or `ruled` | yes |
| `critique-open` | the line says `closed`, and a chain for the page on this checkout is not closed | yes |
| `ok` | every accepted record carries the line, uncontradicted | no |
| `unchecked` | the check itself failed (section 3) | no |

**The warning.** A would-refuse verdict prints one warning line, with its fix on the line under it, on the build's own output, and the build goes on. `--json` carries `designGate: {verdict, wouldRefuse, mode}` in `Data`. The exact lines:

- `warning: goal G is tier 2 and has no accepted design; this build runs on its brief alone`
  fix: `accept a critiqued design whose Goals line names G; or a person runs: metasystem goal allow G build-without-design --reason TEXT`
- `warning: the accepted design NAME does not say its critique closed`
  fix: `add the head line "- Critique: closed ..." under its Goals line ("ruled ..." when a person accepted it)`
- `warning: the accepted design NAME says its critique closed, but its review here is open at round N`
  fix: `finish the review, or correct the line`
- `warning: the design check could not run (CAUSE); this build was not checked`

**The dispatch record.** `prepare` (`intent_work.go:728-775`) writes `design-gate.json` beside `plan.json`, so it is written under the unit's lock and only while the unit has no run:

```json
{"schema": 1, "goal": "G", "unit": "u", "tier": 2, "mode": "warn",
 "verdict": "ok", "wouldRefuse": false, "governedBy": "R-…",
 "designs": [{"id": "01M3Y1P7…", "path": "/…/plans/designs/x.md", "sha256": "…", "critique": "closed at round 4 …"}],
 "at": "2026-10-03T12:41:00Z"}
```

It is not a field of `request.json`: those bytes identify the request, so a permission granted mid-run would turn the next repeat into `UNIT_NAMED_INPUT_CHANGED`.

**Where a would-refuse is counted.** The same `prepare` appends one lowlight to the narrator's digest (`narratordigest.Append`, `internal/narratordigest/digest.go:153`), source `design-gate G/u`, text the warning line; an exact repeat is dropped (`:181-184`). The digest is the committed log a person already hears; the review counts its `(source: design-gate ` lines.

**The switch.** One setting, `design.gate.mode`: `warn` (the default) or `refuse`, declared as `landing.review.human-from-tier` is (`internal/config/landinggate.go:16`, `defaults.go:242`, `validate.go:624`). Under `refuse` a would-refuse verdict returns, before anything is written, a refusal with the warning's sentence ending "; nothing was built", the next step `metasystem goal allow G build-without-design --reason TEXT`, and the detail `BUILD_DESIGN_NOT_ACCEPTED goal=G verdict=V governed-by=R-…`. `unchecked` never refuses. The branch is built and tested now, and left off (question 5). The code gets a register row of shape `Warning`, the first.

**The person's way past.** `metasystem goal allow G build-without-design --reason TEXT`: one row in `goal.Permissions` (words: "building and landing without an accepted design"), one record field written as `- DesignGate: off`, one help line. It covers the goal's landing too.

**The landing half.** In `landGoalRoute`, straight after `admitLanding` (`:1903-1905`) and before the hand-in or the hand landing. That is the seat's own admission, on the computer that holds the dispatch records. It reads the goal's work (`goalWork`, `intent_selection.go:26`), each one's `design-gate.json`, and the design records as they are now:

| Verdict | When | Would refuse |
| --- | --- | --- |
| `design-missing` | a recorded design id is no longer an accepted record of the goal | yes |
| `design-changed` | the file's sha256 differs from the recorded one | yes |
| a dispatch verdict | the check above run afresh, when no design was recorded or no record exists | as above |
| `ok` | every recorded design is present, accepted and unchanged | no |

- `warning: goal G was built against NAME, which changed after the build started`
- `warning: goal G was built against NAME, which is no longer an accepted design of G`
- fix for both: `have the page reviewed again; or a person runs: metasystem goal allow G build-without-design --reason TEXT`

The landing goes on, with one digest lowlight, source `design-gate-landing G@TIP`. Under `refuse` it is refused as `LANDING_DESIGN_NOT_STANDING`, "; nothing was landed", with the same next step. A missing dispatch record alone is never a would-refuse.

**The governance records.** Two rows in `memory/rulings.md`, the register the sweep already reads, so the sweep's code does not change. The ids are minted at build under the register's own rule; the rows are Wido's to rule (question 1). Proposed:

- *Design gate.* "GATE RECORD: the design gate (`BUILD_DESIGN_NOT_ACCEPTED` at work build, `LANDING_DESIGN_NOT_STANDING` at work land), warning-only under `design.gate.mode=warn`. MUST REFUSE, once refusing: a tier-2 build of a goal that no accepted, critiqued design names. APPEAL: `metasystem goal allow G build-without-design --reason TEXT`. IF THE CHECK BREAKS: the work goes on and its output says the check did not run." Owner Wido. Review `class=experimental due=2026-11-03`.
- *Goal-holder refusal.* "GATE RECORD: `GOAL_BRANCH_NOT_HOLDER` (goal commit-goal-binding), refusing. MUST REFUSE: a session building or committing on goal G while another session holds it. APPEAL: a person runs `metasystem goal claim G --take-over --reason TEXT`. IF THE CHECK BREAKS: it refuses, says the claim cannot be checked and names `metasystem goal list`." Owner Wido. Review `class=delegated-authority due=2026-11-03`.

From the due date on, the digest carries "Ruling review sweep: R-… owner=Wido class=experimental due=2026-11-03 choice=adopt|revise|withdraw". A three-entry map `refusal.GovernedBy` (code to ruling id) gives each gate its `governed-by` detail and gives the test its ids.

**The tests, each red without its part.** Build and landing tests are in `cmd/metasystem/intent_design_gate_test.go`. Git is never run: the build half uses the work bed (`newWorkBedWith`, `intent_work_test.go:166`: stubbed branch and head, `workStarter` for launches, a temporary store), the landing half the delivery bed (`intent_delivery_test.go:56`: stubbed `landingGate` and branch state). Design records are files under the bed's `plans/designs/`; the mode is a line in its `metasystem.conf.local`; the chain reader is a function field beside `landingGate` (`landing_gate.go:107-110`).

1. `TestDesignGateWarnsAndStillBuilds`: a tier-2 goal with no design. The build is confirmed and launched, the output has the warning and its fix, `design-gate.json` says `no-accepted-design`, the digest has one line, and a repeat adds none.
2. `TestDesignGateVerdicts`: a table (tier 1; the permission; accepted without the line; `closed`; `ruled`; `closed` with an open chain), asserting verdict, warning, and the recorded id and sha256.
3. `TestDesignGateRefusesOnlyWhenSwitchedOn`: under `refuse`, the build without a design is refused, launches nothing and names `goal allow`; the same goal with a matching accepted design builds. This is the goal's done-when.
4. `TestDesignGateNeverStallsWhenItBreaks`: a failing chain reader, an invalid mode, an unwritable input folder, each under `refuse`: the build still starts.
5. `TestLandingDesignCheck`: an unchanged design lands silently; an edited page warns `design-changed`; a record set to `superseded` warns `design-missing`; under `refuse` both are refused and the matching one lands; a goal with no dispatch record lands.
6. `internal/goal`, `TestBuildWithoutDesignPermissionRoundTrips`: allow, the `- DesignGate: off` line, parse, the words `goal show` prints. `internal/config`: the default is `warn`, `refuse` is valid, anything else is a settings problem.
7. `internal/steward/ruling_sweep_test.go`, `TestRulingSweepFlagsAnOverdueGateRecord`: for each id in `refusal.GovernedBy`, this repository's register (read as `internal/rulings/rulings_test.go:262` reads it) has the row with an owner, a due date and the three labels; then the sweep, run on a copy with the clock one day past due, writes a digest line naming each id.

## 3. When the check itself fails

A broken check is the verdict `unchecked`: it prints the fourth warning above, is recorded in `design-gate.json` when that can be written, and never refuses, in either mode.

- **The design home cannot be read.** Not the gate's: `acceptedDesignPaths` already fails the build (`:879-883`); left as it is.
- **A record the project reader does not list for the goal** (a head it cannot read, no `Goals` line): to the gate there is no such design, so `no-accepted-design`. `metasystem design list --goal G` shows what the reader sees.
- **No critique evidence**: the verdict `critique-not-recorded`, not a failure. **No chain state here**: the head line stands.
- **The setting is unreadable or neither value**: `warn`; `metasystem settings check` names it.
- **The record or the digest cannot be written**: the build goes on; the landing half finds no record and runs the check afresh.
- **At landing, an unreadable work list or record**: `unchecked`; the landing goes on.

## 4. Who can be stopped wrongly

Under `warn` these are wrong warnings; each would be a wrong stop under `refuse`.

- **A claim that moved.** The evidence is in the committed head, so every checkout computes the same verdict. The dispatch record is per computer and keyed by worktree, so a claim that moved finds none: the landing half runs the fresh check and never refuses for the missing record.
- **A tier-2 goal whose design is a plain plan page outside `plans/designs/`.** The reader does not see it: `no-accepted-design`. The fix is the head and a move under `plans/designs/` (`design-obligation-gate.md:32-34`), or the person's allowance.
- **An arc of goals sharing one design.** Every goal the `Goals` line names passes. A goal of the arc it does not name gets `no-accepted-design`; the fix is adding its id to that line.
- **Designs accepted before this gate.** None of the 11 has the head line; each warns `critique-not-recorded` until its seat adds it, a one-line edit.
- **A tier-2 goal that builds from its brief alone**, lawful today: warned on every build until a person allows it (question 2).
- **A design edited after dispatch for a good reason** (a "Built" note, a person's amendment): `design-changed` at landing. Step 1 only warns; the count decides what refusal does with it.
- **Follow-up work on a goal whose design is `done`**: `acceptedDesignPaths` ignores `done`, so `no-accepted-design`.

## 5. Moved effects

None: no write, notice, count or state changes owner, so the page carries no inventory section and `design review --check-only` should report `inventory=absent rows=0 problems=0` (`cmd/metasystem/validate_verbs.go:230`). New, not moved: the command writes to the narrator's digest beside the steward (sources `design-gate`, `design-gate-landing`); `design-gate.json` appears beside each `plan.json`; the JSON listing a goal's designs gains `critique`, the build and landing results gain `designGate`; a goal record may carry `- DesignGate: off`.

## 6. Units and size

| Unit | Lines |
| --- | ---: |
| gate | 270 |
| switch | 150 |
| landing | 180 |
| governance | 60 |

About 290 production lines and 370 test lines. `gate`: check, warning, record, digest line (tests 1, 2, 4). `switch`: setting, permission, refusing branch (tests 3, 6). `landing`: test 5. `governance`: the rows, the map, test 7.

## 7. Guardrails the build must pass

- `go test ./cmd/metasystem -run 'TestDesignGate|TestLandingDesignCheck|TestIntentBuild'`: the new tests, and the existing build tests unedited. A work-bed goal without a tier reads as tier 3, so those tests may now see a warning; none may be weakened.
- `go test ./internal/refusal ./internal/goal ./internal/config ./internal/steward ./internal/rulings`: the new codes have register rows; the register keeps owners and typed reviews (`rulings_test.go:262`).
- `go run ./cmd/devgate static`: the records check reads heads carrying the `Critique` key.
- No grant of `build-without-design` before every seat runs an engine that knows the field; an older engine reports `unknown field` on that goal's record. Step 1 needs no grant: nothing is refused.
- `docs/design/design-obligation-gate.md` gains one paragraph on the `Critique` head line.

## 8. Later, when it hurts

- **Switching to refusal**: a person sets `design.gate.mode=refuse` at the review the register row schedules. First: the hidden `--plan` door calls the same check; a repeat of a running unit is judged by its `design-gate.json`, not afresh; and the trial's count says whether tier 2 stays in, and whether `design-changed` refuses or is cleared by a re-closed critique.
- **Design digests in the read attestation subject** (`internal/goal/branch/attest.go:29-35` carries the unit digest only): from the record's `designs[].sha256`.
- **The verdict travelling with the work**, as a commit trailer or in the landed line's words (`landgate.go:235-239`), so the lane and another computer can read it: the record's `verdict` and `designs`.
- **The machine writing the `Critique` line** when a chain closes, with the subject digest (`cmd/metasystem/intent_design_review.go:41-46`): the head key.
- **One daily count line from the steward** instead of a lowlight per unit: the digest source `design-gate`.
- **A reviewed row leaving the sweep** (a due row keeps rotating): the row's review column.

## 9. Open questions for Wido

1. The two gate records are rows of the rulings register, which holds only your rulings. Do you rule them as worded in section 2, with you as owner and 2026-11-03 as the first review? Recommendation: yes.
2. Design-bearing means tier 2 and 3, so a tier-2 goal that builds from its brief alone is warned on every build until you allow it. Recommendation: yes for the trial; its count shows whether tier 2 stays in once the gate refuses.
3. Each would-refuse puts one lowlight in your digest, once per goal and work. Recommendation: yes during the trial; a daily count line is on the later list.
4. The critique evidence is a hand-written head line, as trustworthy as the hand-set `Status: accepted`. Recommendation: accept it for step 1.
5. The refusing branch is built and tested now, and left off. Recommendation: yes; the goal's done-when asks for a test that refuses.

## 10. Not checked

About 60 of the 70 tool calls were used. Left unchecked, to keep to the pack:

- How the project reader treats a head it cannot read (skipped, or an error for the whole read).
- How a laid-out verb such as `work build` (`intent_work.go:197`) prints an extra line: `text` is the legacy renderer's (`cmd/metasystem/intent.go:904`), so the builder may need the attention banner (`intent.go:818-820`); test 1 asserts the output either way.
- Whether the work bed's goal has a tier, and whether a unit here exceeds the slice cap.
- Whether the lane's evaluator can see the dispatch records; the design assumes not.
- The delegate-path gates of brief facts 7 and 11, which this design leaves alone.

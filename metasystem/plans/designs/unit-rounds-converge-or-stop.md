# Design for unit-rounds-converge-or-stop

- Kind: design
- Id: 01M42XDTPHMPNGVE4H9NYAC8WK
- Status: accepted
- Goals: unit-rounds-converge-or-stop

Revision 3, 2026-10-04. Author: Fable (claude-fable-5-1) for seat m1k; revised by seat m1k (Claude Opus 5.5) after Astra round 1, Wido's answer to question 1 and his widening of 16:30 (D11 to D13, relayed by m1e). No round-1 finding was critical: no second round. Cites at main `57b0966ca`, paths under `metasystem/`.

Accepted 2026-10-04 in Wido's word after one Astra round (m1e, message d-decc63664c1b1c0df043c1c317): build D11 first, then D12, D13, the cap, the stop and the warm read; units of at most 400 lines, one committed Opus read each.

Expected effect, to measure: partner-acts 18 rounds to about 4, lane-resolve 9 to about 3, same findings; one read per unit version; one Astra round per design.

## 1. What is true today

- **Admission.** Only the goal's review-round limit refuses a round, counting every round (`internal/launch/unit_run.go:190-193`, `unit_revise.go:151`). A new work name starts at zero.
- **Rounds.** `finish` (`unit_run.go:570-577`) records an outcome; nothing marks a stop as the environment's. The read brief says "Not this read" for its confirmation section (`cmd/metasystem/intent_work.go:1019-1023`) and asks for instances (`internal/protocol/templates/review-brief.md:55-57`). The seat's `--check` argv is the whole proof (`intent_work.go:673`).
- **Reads.** Every plan has a per-round read (`internal/launch/unit_plan.go:113`), a markdown report ending in `VERDICT:`. `work review G --work U` commits the round and dispatches a cold critic (`cmd/metasystem/intent_unit_review.go:286-305`); an amended commit gets a fresh chain. Landing admits only a closed code-critic chain's attestation (`internal/goal/branch/attest.go:440-444`, `internal/landing/attested.go:47-48`).
- **Design rounds.** Deciding round 1 of a changed page dispatches round 2 up to the goal's limit (`cmd/metasystem/intent_design_review.go:225-261`). Severity is `critical`, `high`, `medium` or `low` (`internal/protocol/schemas/design-critic.schema.json:46`); nothing reads it.
- **Register fold.** The goal-level close refuses accepted material findings (`intent_review_binding.go:206-232`) before any fold, so `goal accept-risk` fails (`internal/dispatch/finding_register.go:541`). Only delegate dispatch checks engine age (`internal/delegation/infra.go:53-79`).

## 2. Step 1

**Fields.** `UnitRunRecord.CountedCap`, frozen at start from setting `launch.unit.counted.rounds` (default 6); `UnitStep.Cause`, `UnitRound.Cause`, `UnitRound.Material`; the launch's `Record.Cause`.

**Cause (D2).** The launch owner names its own stops (`process-lost`, `output-busy`); a runtime adapter names `sandbox-denied` and `provider-limit` through one optional method beside `Outcome`. `finish` copies a cause to the round only when no proof ran red and no read verdict counted; a read failed without findings is `read-no-findings`.

**Cap (D1, D3).** At both admissions, after today's test: when uncaused rounds reach `CountedCap`, refuse `UNIT_ROUND_CAP run=R counted=6 cap=6 machinery=N`, "unit U has used its 6 counted rounds; nothing was started", and name the outcome that applies:

- No material finding in the newest counted read: `work review G --work U`; the rest goes to `goal notes G --read RUN --add TEXT`.
- Material, unit not committed: split. `work build G NEW` starts from the worktree as it stands; landing does not wait on U.
- Material, unit committed: a split builds on top, and U's review closes only with a person's `goal accept-risk G --finding F` naming the successor (D9 makes F findable).

**Divergence (D4).** Each finding of a warm read carries a relation: `new`, `fold-not-holding`, or `same-rule-as N` (an optional code-critic schema field; a line in a markdown read). Both admissions refuse `UNIT_ROUND_DIVERGENT` when the unit's newest read, per-round or committed, has no fewer material findings than the one before, or any is `fold-not-holding` or `same-rule-as`: "the last two reads of U found N, then M material findings, K of them repeats; nothing was started", then take-a-step-back, land or split (question 1).

**Reader (D5, D6).** `work revise G --work U --after N --brief FILE` requires `## Decisions on round N`: per finding, `fixed` with file:lines, `refuted` with the reason, or `follow-up` with the note; a missing material row refuses `UNIT_REVISE_UNDECIDED`. The next read, per-round or committed (through its header, `internal/goal/branch/read.go:257-295`), is told round N's findings, those decisions, the diff since and the proof result: check every fold first, citing the line that proves it; a fold that does not hold is the first finding; never re-raise a refuted finding without new evidence; seek new defects in the changed lines only; label each relation. The template gains: "A finding that is one instance of a rule names every sibling place; the fix is the rule."

**Proof (D7).** The adapter interface gains `TestSteps(closure) []GateStep`: every changed unit tested whole, no name selector, planned before `--check` from the working tree's `Closure(worktree, base, "HEAD")` (`internal/landing/batch/goadapter/adapter.go:34-43`). With no adapter, `--check` alone is the proof.

**Engine age (D8).** `work build` and both revise forms refuse `BUILD_ENGINE_STALE` when the installation holds `cmd/metasystem` and `git log --format= --name-only STAMP..origin/main -- internal cmd` prints a path, naming `infra.go:79`'s rebuild. Silent on a `dev` or `witness-` stamp, no source, or a git failure.

**Fold (D9).** `closeWorkReview` calls `CritiqueRegisterAdvance` for the newest round after the decisions validate (`intent_review_binding.go:189-193`), before `:227`.

**Hand-in (D10).** `handIn` (`cmd/metasystem/landing_plain.go:69-72`) fills `plain.Line.Units` from `NamedWork` (`unit_named.go:476`): counted rounds, machinery rounds by cause, proof kind, whether the read was promoted.

**One read per version (D11, D12).** A plan has a per-round read only when the build brief says `Read each round: yes` (parsed like `intent_work.go:59`); without it a green round ends unread. A counted per-round read saying `VERDICT: land` with no material finding, on another model than the build's, is promoted: after `work review` commits, the branch read owner, where it would dispatch (`read.go:649-653`), records a closed code-critic chain whose round 1 is that read (the commit subject, a LAND return with the commit's tree, the launch's model, an empty register), as `attest.go:440-444` validates. No critic starts.

**One design round (D13).** A design chain's frozen cap (`internal/dispatch/build.go:607-617`) becomes 1, raised to 2 by deciding round 1 only when a finding is `critical`, which the design-critique brief defines as "cannot be built as written". Otherwise the decisions close it, kept as the round's file and the page's Dispositions section (`intent_design_dispositions.go:198-223`), and a second round is refused `DESIGN_ROUND_ONE`: "round 1 of D found no critical finding; its findings are folded and recorded; no second round was started". An accepted material finding closes as folded, not `cap-exhausted-human-raise` (`finding_register.go:857-865`), unless `severe` (`:842-845`).

## 3. Wrong stops and escapes

- A new work name (`gate` to `gate2`) escapes both limits like a split; the hand-in shows it (question 2).
- Red and environment-caused: counts. Only non-material findings: never divergent.
- A repeat mislabelled `new` escapes the label test, not the count or the cap.
- Older runs (`CountedCap` zero) and stale engines without source escape.
- A clean read on the build's model is not promoted.
- A critic that calls everything `critical` gets round 2, as today.

## 4. Moved effects

| Effect | From | To | Code |
|---|---|---|---|
| choosing the round's test commands | the seat's `--check` argv | the testing adapter's steps, then `--check` | `metasystem/cmd/metasystem/intent_work.go:673`, `metasystem/internal/testpolicy/adapter/adapter.go:68` |
| deciding whether a round is read | every plan | the build brief's line | `metasystem/internal/launch/unit_plan.go:113` |
| writing a unit's code-critic chain | the dispatched critic | the branch read owner, for a promoted read | `metasystem/internal/goal/branch/read.go:649` |
| starting a design's round 2 | any decided round 1 | a critical round-1 finding | `metasystem/cmd/metasystem/intent_design_review.go:225` |

## 5. Deferred

- A flake cause, once the register exists; a red proof re-run at the base: `UnitStep.Cause`.
- A successor's clean read discharging a committed predecessor's findings: `UnitRound.Material` and D9.
- Witness stamps and hosts without source: the D8 check.

## 6. Open questions for Wido

1. Answered (b), 2026-10-04: the relation labels. The alternative stays (a), the literal "findings in the last fold's lines", a cap of two.
2. Does a split inherit its predecessor's counted rounds? Recommended: no; the hand-in shows the chain.
3. D7 explains 1 of 27 measured rounds. Recommended: build it last.
4. Under D12 a re-read after a revise is warm and D11 promotes warm reads, dropping the binding's "one cold read at the end"; the guard is the cited fold. Recommended: yes. Alternative: a cold critic when the last read was warm, one more read per revised unit.
5. Is BREAKING the severity `critical`? Recommended: yes. Alternative: also rigor `severe`.

## 7. Units

Every test stubs Git. U: `newUnitFixture` (`internal/launch/unit_test.go:128`). W: `newWorkBed` (`cmd/metasystem/intent_work_test.go`, plus a stamp seam). D: `newDeliveryBed`.

|Unit|Decisions|Red without it|Lines production/test|
|---|---|---|---|
|`promote`|D11|`TestCleanReadIsPromoted`, `TestReadOnTheBuildModelIsNotPromoted`, `TestPromotedReadAttestationValidates`; `internal/goal/branch/read_test.go`; D|180/220|
|`optional-read`|D12|`TestRoundWithoutReadEndsGreen`, `TestBriefLineAsksForTheRead`; U, W|60/90|
|`design`|D13|`TestSecondDesignRoundRefusedWithoutCritical`, `TestOneRoundCloseFoldsAccepted`; `intent_design_review_test.go`; D|110/170|
|`cause`|D2|`TestLaunchNamesItsEnvironmentCause`; `internal/launch/cause_test.go`; a fake adapter|90/140|
|`cap`|D1, D3|`TestCountedCapRefusesTheNextRound`, `TestEnvironmentRoundIsNotCounted`, `TestCapNamesTheSplitForAnUncommittedUnit`; `internal/launch/unit_test.go`; U|170/230|
|`stop`|D4|`TestReviseRefusedWhenMaterialDoesNotFall`, `TestReviseRefusedOnARepeatedRule`; `unit_revise_test.go`; U|120/190|
|`warm`|D5, D6|`TestReviseRefusesUndecidedFindings`, `TestFollowUpReadIsToldThePreviousDecisions`; U. `TestReadBriefAsksForTheRule`; W|170/220|
|`records`|D9, D10|`TestRefusedCloseFoldsItsRound`; `intent_review_completion_test.go`; D. `TestHandInCarriesUnitRounds`; `landing_plain_handin_test.go`|80/150|
|`proof`|D7|`TestAdapterStepsComeBeforeTheCheck` with a fake adapter; W|110/150|
|`engine`|D8|`TestBuildRefusesAStaleEngine`; W|70/90|

## 8. Folds

Astra round 1: PROOF-CANDIDATE-CONTRACT fixed (working-tree closure); RULING answered (b); SPLIT-PREDECESSOR-CLOSURE fixed (two split cases); WARM-READ-DECISION-SOURCE fixed (the decisions section); CAUSE-DEPENDENT-REGRESSION fixed (red counts); MOVED-EFFECTS-NO-ROWS fixed (section 4). Widening: D11 to D13. Acceptance: units re-cut to at most 400 lines in m1e's order.

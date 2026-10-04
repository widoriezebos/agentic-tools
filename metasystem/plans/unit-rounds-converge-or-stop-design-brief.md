# Design brief: unit-rounds-converge-or-stop

## Revision

Revision: first draft

Reason: goal `unit-rounds-converge-or-stop` (tier 2, opened and approved 2026-10-04 by m1e in Wido's word, pinned to and claimed by seat m1k) has no design record. Step 1 is this design: at most 1,200 words, one critique round on Codex Astra, then units.

## Context pack

Read this pack first. Open another file only to check one of the cited lines below; do not widen the read beyond that check. Paths are relative to the installation `metasystem/` of `/Users/wido/LocalStorage/GitHub/agentic-tools-m1k` (main at the time of writing: `ed06d4358`).

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

The goal, verbatim in its essentials (`metasystem goal show unit-rounds-converge-or-stop --json` carries it whole):

- Intent: "A unit's build, read and revise loop converges or stops: the unit runner counts rounds per unit, excludes rounds the environment caused (a sandbox refusal, a lost process, a provider limit, a registered flake), stops at a small cap with one of two outcomes (land with the remaining non-breaking findings as follow-ups, or split the unit), and the reader reports a whole class of findings at once so the builder fixes the class, not one instance per round. Generic for any adopter and language: the loop and the cap live in the runner, the class rule in the reader's brief."
- Risk basis: "changes when every seat's unit loop stops; a wrong cap blocks delivery, the right one removes the night's largest time sink (about 25 seat-hours of rounds)".
- Next step (measured): m1l's partner-acts took 18 rounds, m1h's conflicts unit 9 rounds, m1f's root-transport 6; each round costs 20 to 30 minutes. "Generic by requirement (Wido 2026-10-03): nothing names Go or this repository; adapters supply only how a stop reason is classified."

The measured analysis, which you must read whole (about 1,300 words): `/Users/wido/LocalStorage/agentic-tools-evidence/night-review-20261003/analysis-review-rounds-2026-10-04.md` (m1e, 2026-10-04). Its verdict over 27 rounds: 9 were builders that could not run their own tests (a separate goal, `codex-jobs-run-unsandboxed-on-a-trusted-host`, fixes the sandbox; out of scope here); 7 to 8 were the critic re-run cold each round, told nothing of the previous findings and asked for instances, not the class; 7 were reads that failed on a stale engine; about 4 were design by exemption; no stop rule stopped anything.

Facts traced by the seat at `ed06d4358` (verify only what you rely on):

1. The cap today. A follow-up round is admitted at `internal/launch/unit_run.go:190-203`: refused only when `len(record.Rounds) >= record.MaxRounds` (`roundLimit`, `internal/launch/coded.go:30-31`, code `UNIT_ROUND_LIMIT`). `work revise` checks the same at `internal/launch/unit_revise.go:149-151`. `MaxRounds` is the goal box's review-round member (20 today), frozen on the unit run record at build (`cmd/metasystem/intent_work.go:782`). Every round counts, whatever ended it.
2. How a round ends. `finish` (`internal/launch/unit_run.go:570-577`) sets `round.Outcome` and leaves the run awaiting judgement. Outcomes: `build-failed` (`internal/launch/unit_run.go:355`), `proof-wrote` (`internal/launch/unit_run.go:414`), `proof-red` (`internal/launch/unit_run.go:420`), or the read sequence's outcome (`internal/launch/unit_run.go:422-429`). A failed read leaves no verdict; `declared-output-busy` is raised at `internal/launch/declared_outputs.go:77`. Nothing classifies a stop as caused by the environment. The flake register is goal `lane-records-flakes-and-routes-their-fix` (`plans/goals/lane-records-flakes-and-routes-their-fix.md`).
3. What the next round's reader is told. A follow-up round passes the previous round's outputs and decisions as launch inputs only (`internal/launch/unit_run.go:322-331`, `internal/launch/unit_run.go:401`); the reader's brief comes from `readBrief` (`cmd/metasystem/intent_work.go:1015-1058`), which cuts the template's "Scoped confirmation read after a fold" section and writes "Not this read" (`cmd/metasystem/intent_work.go:1019-1023`). The template's section is at `internal/protocol/templates/review-brief.md:73-89`. The design-critique loop's follow-up wording is the model: `internal/launch/round_task.go:23` and `internal/launch/round_task.go:38` ("Check every claim FIRST ... Do not re-raise a finding that was correctly fixed or refuted").
4. Instances, not classes. The template asks each finding to name "the file, rule, and concrete failure it causes" (`internal/protocol/templates/review-brief.md:55-57`); nothing asks for the rule's other places.
5. Clean read. `readClean` is true only when every counted read says `land` (`cmd/metasystem/intent_work.go:1180-1200`); a non-material finding alone forces another round when the reader writes `fix first`.
6. A divergence rule exists only inside a critic chain: `fallingMaterialTrajectory` (`internal/dispatch/finding_register.go:983-1000`), used by the design cap. Each committed `work review` after a revise is a fresh chain root, so it never sees the unit's earlier rounds.
7. The proof is the seat's choice. `work build G --brief FILE --check COMMAND...` (`cmd/metasystem/intent_work.go:225`, `cmd/metasystem/intent_work.go:673`, `cmd/metasystem/intent_work.go:758`): the seat may pass a name selector that skips the touched package's other tests (the analysis's partner-acts R16). The project has a risk-selected test runner (`metasystem test plan|run`, the testing contract in `testing.json`); find its selection owner before relying on it.
8. Engine age. An engine binary carries a build stamp (`internal/enginebuild/record.go:22-67`; read by `enginebuild.ReadStamp`), a commit or `witness-<12 hex>` of the engine-surface digest; the steward resolves a stamp to a landed commit at `internal/steward/rearm_resolver.go:474-500` and digests a tree's engine surface with `policy.Digest(dir, behaviorsurface.Engine)`. The deployed engine is `~/.metasystem/bin/metasystem` (`internal/enginedeploy/enginedeploy.go:64-67`). On 2026-10-04 seat m1k's `metasystem/bin/metasystem` predated item X on main and refused every `Working Mode: Implement` brief; m1h's stale binary made 7 reads fail.
9. The register fold defect (found by m1k 2026-10-04). The goal-level close `work review G --work W --dispositions FILE` refuses accepted material findings at `cmd/metasystem/intent_review_binding.go:227-232` before anything folds the round into the chain's finding register. The register stays empty, so `goal accept-risk` answers "finding F is not in critic root R" (`internal/dispatch/finding_register.go:542`), although this same close already honours a risk the goal records (`cmd/metasystem/intent_review_binding.go:196-212`). The job-level close folds first (`cmd/metasystem/intent_delivery.go:1509`); m1k used it as a workaround.

Critique findings being answered:

1. none (first draft)

Cited code excerpts: open them yourself at the lines above; the facts are stated in full there.

Example page:

`plans/designs/design-gate-at-dispatch.md` (sections 1-3 and its units table): its density, its "What is true today" with file:line, its steps with the exact test that is red without each.

## Decisions the seat has taken (build on them; argue only with evidence)

These follow m1e's scope in Wido's word (2026-10-04 10:05) and the fleet rule of 08:40 (at most six review rounds per unit).

- D1 A per-unit cap of 6 counted rounds by default, one setting (name it, default 6), enforced by the unit runner at both admissions in fact 1. The goal box's `MaxRounds` stays the outer backstop over all rounds, counted or not, so an environment loop is still bounded.
- D2 A round the environment ended is recorded with its cause and not counted: at least a sandbox denial, `declared-output-busy`, a lost process, a provider limit, a read that failed without findings, and a failure the flake register names. The runner only counts; how a stop reason is classified is an adapter's answer, so nothing in the runner names Go or this repository.
- D3 At the cap the runner refuses another round and names the two outcomes: land with the remaining non-breaking findings recorded as goal follow-up notes (`metasystem goal notes`), or split the unit. A material finding never lands silently at the cap: say what path it takes (split into a new unit with its own count, or a person's raise).
- D4 `work revise` is refused with `UNIT_ROUND_DIVERGENT` when the newest counted read's material count is not below the previous counted read's, or when its findings fall in what the previous follow-up changed (a class seen in two consecutive rounds). The refusal points to take-a-step-back, land with follow-ups, or split.
- D5 A follow-up read keeps the "Scoped confirmation read after a fold" section, filled with the previous findings, decisions and follow-up paths, worded as fact 3's model: check every folded claim first; never re-raise a correctly fixed or refuted finding; then attack the delta.
- D6 The review template asks: a finding that is one instance of a rule applied in several places names every sibling place; the fix is the rule.
- D7 The round's proof runs the whole test set of every unit of test selection the diff touches, plus the static gate, with no name selector; the adapter maps a changed path to its test set.
- D8 `work build` and `work revise` are refused when the running engine is older than main's engine source, where the installation carries that source; elsewhere the check is silent.
- D9 The goal-level close folds the terminal round into the finding register before the accepted-material refusal (fact 9), so `goal accept-risk` finds the finding.
- D10 The hand-in carries each unit's counted rounds, its machinery rounds and their causes, computed from the unit records, not typed by the seat.

## What the page must answer

1. What is true today, with file:line (confirm the facts you rely on; correct any that are wrong).
2. Step 1 (R-121, the smallest thing that works first), per decision: the exact change, where it runs, the refusal text and code, and the test that is red without it (file, test name, fixture). Behavior tests stub Git (project rule); name the seams.
3. Who can be stopped wrongly or escape the cap: at least a unit continued under a new work name after its settings changed (seat m1k's `gate` to `gate2`, 2026-10-03), a round both red and environment-caused, a reader that returns only non-material findings, and a stale engine on a host without engine source.
4. Moved effects: whether any write, notice, count or state changes owner.
5. Deferred, each with the step-1 field it builds on.
6. Open questions for Wido, each with your recommended answer; keep them to real choices.
7. A units table with honest changed-line estimates (production and test separately), at most 6 units.

## Recurring findings

none recorded

## Tool-call budget

Maximum delegate tool calls: 60

Stop when this number is reached. List anything the budget did not allow you to check.

## Page-size ceiling

Maximum page size: 1200 words

Cut a draft that exceeds this ceiling. If cutting would make the page incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique only through the context pack above. Never resume a delegate from an earlier revision.

## Page artifact and return shape

Write the page to: /Users/wido/LocalStorage/GitHub/agentic-tools-m1k/metasystem/plans/designs/unit-rounds-converge-or-stop.md

Start it with the design record head (`- Kind: design`, a fresh ULID `- Id:`, `- Status: draft`, `- Goals: unit-rounds-converge-or-stop`), then one line naming the author, the date and the commit the cites were read at.

Write for a person: plain English, expand identifiers on first use, no review-round or finding numbers in proposed source comments.

Return only these two lines:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1k/metasystem/plans/designs/unit-rounds-converge-or-stop.md
DESIGN: ready (N words, N the page's word count)
```

or:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1k/metasystem/plans/designs/unit-rounds-converge-or-stop.md
DESIGN: blocked (the reason)
```

When blocked, stop and return `BLOCKED:` with the refusal's first line, what you tried and the decision needed; your seat asks.

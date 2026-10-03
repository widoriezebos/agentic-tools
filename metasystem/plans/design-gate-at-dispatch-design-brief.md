# Design brief: design-gate-at-dispatch

## Revision

Revision: first draft

Reason: goal `design-gate-at-dispatch` (tier 2, approved by Wido 2026-10-03, claimed by seat m1k) has no design record. Its next step asks to find where the switched-on machinery first starts a design-bearing build today and put the check there, plus two halves folded in at Wido's word on 2026-10-03 ("drain the waiting lane"): the landing half and the gate governance record.

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check. Paths are relative to the
installation `metasystem/` of `/Users/wido/LocalStorage/GitHub/agentic-tools-m1k`
(main at the time of writing: `19a4f3335`).

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

The goal, verbatim (`metasystem goal show design-gate-at-dispatch --json` carries it whole):

- Intent: "What: Work that needs a design cannot start as a build unless an accepted, critiqued design stands behind it. Why: 'Design first' is checked only at the end of the work, not when it starts. The gate was aimed at the old dispatch path, and builds today go through the launch lane by hand. Pros: Stops builds that skip their design. Cons: Building a gate at a door nobody uses now would be wasted work."
- Next step: "find where the switched-on machinery starts builders and put the check there: refuse a design-bearing build that names no accepted design with a closed critique. Done when: a test refuses such a build and admits one with a matching accepted design. RETURNED 2026-10-03: ... First confirm on a live example where a design-bearing build is first dispatched today (the seat's unit build), then build the gate there. FOLDED IN 2026-10-03 at Wido's word (drain the waiting lane): step 2, from landing-design-provenance: the landing half of the gate, added to the existing landing evaluator from the design record the dispatch gate produces (tests refuse a landing with a missing or changed design, pass one with a matching accepted design). Step 3, from gate-governance-records: a governance record for this gate and for commit-goal-binding's refusal (owner, review date, an example it must refuse, how to appeal, what happens if the check itself breaks), warning-only first, and a test that the ruling sweep flags an overdue review."
- Folded goal `gate-governance-records` (records/goals/gate-governance-records.md), intent: "Each new refusal gate comes with a short record: who owns it, when it is reviewed, an example it must refuse, how to appeal, and what happens if the check itself breaks. It also runs in warning-only mode first. Why: A gate without an owner or a review date turns into ceremony nobody owns." Its done-when: "each gate has its record and a test shows the ruling sweep flags an overdue review."
- Folded goal `landing-design-provenance` (records/goals/landing-design-provenance.md), intent: "A landing that needed a design is refused unless it names the accepted design and its critique."
- Goal `commit-goal-binding` (plans/goals/commit-goal-binding.md) is a separate approved goal; only its existing refusal ("an agent's landing without a goal it holds is refused") needs a governance record here.
- Risk basis recorded on the goal: "A new refusal where every seat starts a design-bearing build; a wrong check stalls legitimate builds across the fleet or lets a build skip its design."

Facts traced by the seat on 2026-10-03 (file:line, read at `19a4f3335`; verify only what you rely on):

1. The live door. `metasystem work build G` is `cmd/metasystem/intent_work.go:197-235` -> `runIntentBuild` (:428) -> `runIntentBuildUnit` (:494). In order it checks: goal live (:562), approved budget (:567), claim acquired (:572-580), this session holds it (`branch.CheckHolder`, :587); then `acceptedDesignPaths(id)` (:592), `unitRequest(...)` (:598), `runner.AdvancePrepared(...)` (:623). The engine side is `internal/launch/unit_named.go:140` `AdvancePrepared` -> `advanceNamedLocked` (:190): `requireGoalBranch` (:228), `admitRound` (:231 -> `internal/launch/unit_run.go:215` -> `Manager.admit`, `internal/launch/admit.go:53`), then the run id is reserved (unit_named.go:236-239).
2. Refusal styles. CLI: `intentResult{Outcome: intentRefused, code: 1, Summary: "...; nothing was built", next, nextReason, Details: []string{"CODE facts"}}`, example `LAUNCH_BUILD_UNSIZED` at intent_work.go:946-949. Engine: `coded(code, facts, err)` (`internal/launch/coded.go:15` -> `refusal.New`), codes registered in `internal/refusal/register.go`, appended to the refusal store (admit.go:43-49, unit_run.go:264).
3. Today's design handling at build. `acceptedDesignPaths` (intent_work.go:877-905) lists the goal's design records with `Status == "accepted"` and nothing else; with none, the brief alone stands in (brief text at :995 and the scaffold at :1985 say so). The build request identity already stores each accepted design's `{path, sha256}` (:701-721), written to `.inputs/<key>/request.json` (unit_named.go:153-175); the designs become build and read inputs (:756-757).
4. `work build` passes no destructive reach. `DESIGN-BEARING`/`MECHANICAL` exist only on the delegate path (`internal/dispatch/hazard.go`); the CLI hardcodes them for critics (intent_delivery.go:960-961 design review, :1048-1049 code review). Goal tier is `internal/goal/landgate.go:95` `GateTier`, read today only by the landing gate. Orchestration: tier 1 has no design critique and lands as a receipted direct fix (`docs/orchestration.md:69`, `:189-190`); design critique exists at tiers 2 and 3.
5. The design record. Head `Kind/Id/Status/Goals` under `plans/designs/` of the state root (`docs/design/design-obligation-gate.md:19-44`); struct `internal/project/record.go:56-76`; statuses draft/accepted/superseded/done (`internal/project/project.go:56-61`, "maintained by hand"; no verb sets `accepted`). The record has no critique field and no digest. In practice the seat sets `Status: accepted` by hand when the critique closes, commit messages such as "design: flaky-leftovers accepted, critique closed at round 4 on 0 material" (`8b37032aa`), "seat-path-lands-without-help accepted at revision 5 (m1g; critique closed at round 4, no material finding)" (`7f9ac66f9`); some are accepted by Wido's ruling (`ef96e62fd`).
6. Critique state. Per-round subject digests: `artifacts/agents/intent-review/design-<recordid>/chain.json` (`cmd/metasystem/intent_design_review.go:41-50`). Closure: job records' `chainClosed`, read by `dispatch.DesignCritiqueChains` (`internal/dispatch/design_chain.go:26-67`, `Closed` at :50). `artifacts/` is gitignored and per checkout: a goal whose claim moves to another machine's checkout does not carry it (claims do move: `one-folder-deployed-and-evolved` moved m1g -> m1f on 2026-10-02).
7. Existing gates that are not this gate: `DESIGN_CHAIN_OPEN` (`internal/dispatch/read_admission.go:24`, :180, delegate path); `chain-not-design-bearing` (`internal/landing/observe.go:475-476`, would-refuse only, kept non-refusing at :41-44 — read this, it is the precedent for a warning-only check); `attested-not-design-bearing` (`internal/landing/attested.go:50-51`, diff deletes a file with no plan fold).
8. Landing. `runIntentLand` (intent_delivery.go:1617) -> `landGoal` (:1780) -> `landGoalRoute` (:1837); admissibility `inv.admitLanding` (:1903 -> `cmd/metasystem/landing_gate.go:106`) -> `goal.Gate` (`internal/goal/landgate.go:240-270`: holds, tier, human word; no design). Tree evaluator `landingPathObserve` (`cmd/metasystem/landing_path.go:140-170`) -> `landing.Observe` (`internal/landing/observe.go:160`), attested bar (`internal/goal/branch/attest.go`); the read attestation subject (attest.go:29-34) carries the unit digest and no design digest.
9. Exceptions a person grants per goal: `metasystem goal allow G PERMISSION --reason TEXT` (today one permission, `stop-test-changes`; a person's act at the enrolled terminal; `goal show` lists them). Find its owner in `internal/goal` and `cmd/metasystem` before using it.
10. Governance today: the steward keeps a ruling review sweep (`artifacts/agents/steward/ruling-review-sweep.json` is its state; find the owner under `internal/steward` or `internal/rulings`); rulings live in `memory/rulings.md`, parsed by `internal/rulings/rulings.go`. Whether a ruling row can carry a review date the sweep already flags is for you to establish.
11. The 2026-09-02 audit `records/misc/design-gate-audit-2026-09-02.md` proposed a `designChain` field at dispatch (D3) and `chain-without-design` at landing (D4) on the old delegate path; the old path is not the live door now.

Critique findings being answered:

1. none (first draft)

Cited code excerpts: open them yourself at the lines above; the facts are stated in full there.

Example page:

`plans/designs/reviewers-check-the-rulings.md` (read sections 1-4): its level of detail, its "What is true today" with file:line, its "Step 1" with the exact test, its guardrails and its "Later, when it hurts". Write at that density.

## Decisions the seat has taken (build on them; argue only with evidence)

- D1 The gate sits at the live door, `work build`, at the first point a build is admitted (fact 1), not on the delegate path.
- D2 A build is design-bearing when its goal's tier is 2 or 3. Tier 1 is not gated.
- D3 Warning-only first. Step 1 never refuses: it computes the verdict, prints one plain-English warning line on the build's output, and records the would-refuse durably where a person and the governance review can count them. Turning it into a refusal is a later, person's decision after a trial, by one configuration value; the page names that value and its default (warn).
- D4 "An accepted, critiqued design stands behind the build" means: a design record whose `Goals` names the goal, with `Status: accepted`, and evidence that its design critique closed. You decide what the evidence is, given fact 6 (critique state is per checkout and claims move between checkouts). Prefer evidence that travels with the committed design record over evidence in `artifacts/`.
- D5 The verdict and the design digest(s) the build ran against are recorded once at dispatch, where the landing half reads them (fact 3 already stores `{path, sha256}`). The landing half compares them with the design at landing: missing or changed -> the same warning-only treatment in step 1.
- D6 The person's lawful way past the gate for one goal is the existing `goal allow` mechanism with a new permission (name it), never a new verb.
- D7 The governance record: decide where it lives (prefer an existing register the steward's sweep already reads over a new file kind) and what the sweep flags when its review date passes. One record for this gate, one for commit-goal-binding's existing refusal.

## What the page must answer

1. What is true today, with file:line (confirm facts 1-10 you rely on; correct any that are wrong).
2. Step 1 (R-121: the smallest thing that works first): the exact check, where it runs, its inputs, its verdict values, the exact warning text, where the would-refuse is recorded, how the landing half reads the dispatch record, the `goal allow` permission, the governance records and the sweep change. For each: the test that is red without it (file, test name, the fixture). Behavior tests stub Git (project rule); say which seams they use.
3. What happens when the check itself fails (unreadable design home, malformed record, missing critique evidence): the gate must never stall a build because the check broke; say what it prints and records instead.
4. Who can be stopped wrongly: name at least the moved-claim case (fact 6), a tier-2 goal whose design is a plain plan page outside `plans/designs/`, and an arc of goals sharing one design; say what the gate does for each.
5. Moved effects: whether any write, notice, count or state changes owner (the `design review --check-only` inventory convention).
6. Deferred, each with the step-1 field or home it builds on: switching to refusal, design digests in the read attestation subject, anything else.
7. Open questions for Wido, each with your recommended answer. Keep them to real choices.

Estimate the build's changed lines honestly (production and test separately).

## Recurring findings

none recorded

## Tool-call budget

Maximum delegate tool calls: 70

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: 3200 words

Cut a draft that exceeds this ceiling. If cutting would make the page
incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique
only through the context pack above. Never resume a delegate from an earlier
revision.

## Page artifact and return shape

Write the page to: /Users/wido/LocalStorage/GitHub/agentic-tools-m1k/metasystem/plans/designs/design-gate-at-dispatch.md

Start it with the design record head (`- Kind: design`, a fresh ULID `- Id:`, `- Status: draft`, `- Goals: design-gate-at-dispatch`), then one line naming the author, the date and the commit the cites were read at.

Write for a person: plain English, expand identifiers on first use, no review-round or finding numbers in proposed source comments.

Return only these two lines:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1k/metasystem/plans/designs/design-gate-at-dispatch.md
DESIGN: ready (N words, N the page's word count)
```

or:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1k/metasystem/plans/designs/design-gate-at-dispatch.md
DESIGN: blocked (the reason)
```

When blocked, stop and return `BLOCKED:` with the refusal's first line, what you tried and the decision needed; your seat asks.

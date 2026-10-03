# Design brief: design-gate-at-dispatch

## Revision

Revision: revision 2 of plans/designs/design-gate-at-dispatch.md

Reason: Codex on Astra's critique round 1 (job design-critic-ba61e618ee427038e15c207a) returned five material findings; the seat accepted all five. Two answers given in Wido's word since revision 1 also change the page.

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check. Paths are relative to the
installation `metasystem/` of `/Users/wido/LocalStorage/GitHub/agentic-tools-m1k`.

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

The complete prior revision is the page itself, `/Users/wido/LocalStorage/GitHub/agentic-tools-m1k/metasystem/plans/designs/design-gate-at-dispatch.md` (revision 1, committed on main at `63939e24e`). Read it whole first; revise it in place. Its original brief is `plans/design-gate-at-dispatch-design-brief.md`; its decisions D1-D7 still stand except where this brief changes them.

Answers since revision 1 (Wido's word, relayed by the partner seat m1e on 2026-10-03; he can overrule):

- Yes to both governance rows in `memory/rulings.md` as revision 1 worded them, owned by Wido, first review 2026-11-03; the design gate warning-only until he sets `design.gate.mode=refuse`; the two appeals and the broken-check behaviour as drafted.
- Every warning or refusal text is two plain lines: the reason, then the one command. Revise every message in the page to that shape.
- A seat cannot land a change to `memory/rulings.md` in a build commit today (goal `records-land-through-the-lane` is open). The two rows are written into the seat's hand-in note and m1e lands them with the unit. The build therefore does not edit `memory/rulings.md`; the governance test must hold before and after the rows exist (say exactly how: for example, it reads the rows from a fixture copy for the sweep half, and asserts the live register only for ids that `refusal.GovernedBy` maps once the rows have landed, or another arrangement you can show passes on both sides).
- Build order (the partner's standing rule, from R-121): the smallest first use comes first and can land alone. Unit 1 is the warning-only check at `work build` (check, warning, dispatch record, digest line). The switch with the `goal allow` permission, the landing check, and the governance part follow as later units of this goal. Reorder section 6 so each unit lands on its own and say what each later unit needs from the earlier ones.

Ruling to respect throughout: R-142-m1e (`memory/rulings.md:201`, "the machine never overrules a person"): once the gate is switched to refuse, it may stop an agent's build or landing; a person's explicit act (a landing a person makes, an exception a person records) is warned, explained and recorded, then complies. Say which acts are a person's in both halves.

Critique findings being answered (all five accepted by the seat; the decisions file is `artifacts/agents/design-critic-ba61e618ee427038e15c207a/rounds/1/decisions.md`, whose Amendment column states the required change):

1. RULING-design-gate-at-dispatch (material, high). The design substitutes a queue-admission check for the authorized landing check. The goal explicitly requires the landing half to be "added to the existing landing evaluator from the design record the dispatch gate produces." Checking only before hand-in lets an ordinary queued branch reach main after its design changes or is withdrawn, without the trial warning. This changes the named owner and leaves the authorized landing behavior incomplete. It fails WORK and the binding goal-decision test.
   Evidence: The frozen metasystem/plans/designs/design-gate-at-dispatch.md:79 places the check before hand-in; line 161 defers carrying its evidence to the landing lane. The goal's NextStep explicitly names the landing evaluator. metasystem/cmd/metasystem/landing_plain.go:69 only queues the branch; metasystem/internal/landing/plain/push.go:50 performs publication later and has no proposed design check. The hand-landing route also rechecks its existing admission gate immediately before publication at metasystem/cmd/metasystem/intent_delivery.go:2010, but the proposed check is outside that owner.
2. RULING-R-143-m1e (material, high). The new human exception omits the required explanation and record of its impact. Ruling R-143-m1e says an override must explain "what it lets through, what risk it leaves, what it changes for later work, and how to undo it" and be recorded with that statement. Adding only a permission row, record field and help line grants a lasting build-and-landing exception without this behavior. This changes the exception's interaction and recorded evidence; it fails SAFE and the binding ruling test.
   Evidence: The frozen metasystem/plans/designs/design-gate-at-dispatch.md:77 specifies only the permission row, field and help line; line 108 tests only their round trip. metasystem/cmd/metasystem/intent_goal_permission.go:35-75 applies an allowance without presenting its impact. metasystem/internal/goal/permissions.go:98-108 records the permission name and supplied reason, and metasystem/internal/goal/verbs.go:3858 uses that text. None records the impact statement required by metasystem/memory/rulings.md:202.
3. DESIGN-GATE-LANDING-CRITIQUE (material, high). An unchanged design can silently lose its failed critique verdict at landing. Dispatch with an accepted design lacking critique evidence produces a warning and records that design. Landing then returns "ok" solely because its identity, acceptance and hash are unchanged. The same happens when its critique remains open. The warning trial therefore reports success for a condition it already classified as unacceptable. The landing outcome and its test must change; this fails WORK and SAFE through a false answer.
   Evidence: The frozen metasystem/plans/designs/design-gate-at-dispatch.md:47-48 defines missing or open critique as would-refuse verdicts. Lines 62-68 retain the accepted design in the dispatch record. Lines 83-86 rerun the dispatch check only when no design or record exists; otherwise an unchanged accepted design returns ok. Line 129 identifies existing accepted designs without the new critique line, making this an immediate rollout case. Test 5 at line 107 does not distinguish an unchanged acceptable design from an unchanged design with a failed critique verdict.
4. DESIGN-GATE-FAILURE-FIXTURE (material, medium). The required failure test cannot launch a build with an unwritable input folder. That folder contains mandatory launch inputs, not just optional gate evidence. An implementer must either change the existing launch protocol or narrow the fixture to failure of the gate-owned record or digest. The design must make that distinction explicitly. This changes a required test assertion and fails WORK because the specified acceptance test cannot pass with the preserved build protocol.
   Evidence: The frozen metasystem/plans/designs/design-gate-at-dispatch.md:106 requires a build to start with an unwritable input folder; lines 62 and 119 place optional gate evidence beside the mandatory plan and promise continuation on its write failure. metasystem/internal/launch/unit_named.go:166-173 requires creation of that folder, successful preparation and writing request.json before advancing. metasystem/cmd/metasystem/intent_work.go:763-769 writes both briefs and plan.json there and returns on any write error.
5. MOVED-EFFECTS-NO-ROWS (material, medium). The empty Moved effects section fails the mandatory inventory check. The page says the section is absent while declaring that exact heading. No actual ownership transfer was found, so the supported "No owner moves." declaration would resolve this without inventing inventory rows. This fails the required first-use check and is material under the brief's explicit moved-effects rule.
   Evidence: The frozen metasystem/plans/designs/design-gate-at-dispatch.md:134-136 declares the heading but supplies neither rows nor the recognized empty-inventory declaration. The required command exited 1 with MOVED-EFFECTS-NO-ROWS and inventory=present rows=0 problems=1. metasystem/internal/validate/movedeffects.go:92 recognizes "No owner moves."; lines 140-143 otherwise reject an empty section.

Required changes, in short (from the decisions file):

1. The landing half moves into the landing evaluator (`internal/landing/observe.go`, beside the non-refusing `chain-not-design-bearing` observation at `:41-44` and `:1724-1735`), so the lane and the hand route both run it; the hand-in check in `landGoalRoute` is removed. Establish where the evaluator can read the goal's design records and the dispatch record from (the lane runs on the same computer; the unit store is `~/.metasystem/unit`, per computer) and what it does when the dispatch record is not there.
2. `goal allow G build-without-design` prints the plain-English impact statement (what it lets through, the risk left, what it changes for later work, how to undo it) before applying, and records it with the permission (R-143-m1e). Find whether the allow verb or the goal record already has a place for such a statement; prefer extending it. A test asserts the statement is shown and recorded.
3. The landing verdict always reruns the full dispatch check on the design as it stands now, then adds `design-changed` or `design-missing` from the comparison; test 5 gains the unchanged-but-uncritiqued case.
4. The failure test narrows to the gate's own record and digest writes failing through an injected writer; the unit's input folder is not part of it.
5. The Moved effects section says `No owner moves.` (`internal/validate/movedeffects.go:92`); the list of new effects stays outside that section. Run `bin/metasystem design review /Users/wido/LocalStorage/GitHub/agentic-tools-m1k/metasystem/plans/designs/design-gate-at-dispatch.md --check-only` before returning and report its line.

Cited code excerpts: open them yourself at the lines cited above and in the findings.

Example page:

The prior revision itself, for level of detail and density.

## Recurring findings

none recorded

## Tool-call budget

Maximum delegate tool calls: 60

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: 3600 words

Cut a draft that exceeds this ceiling. If cutting would make the page
incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique
only through the context pack above. Never resume a delegate from an earlier
revision.

## Page artifact and return shape

Write the page to: /Users/wido/LocalStorage/GitHub/agentic-tools-m1k/metasystem/plans/designs/design-gate-at-dispatch.md

Keep the head (same Id, `Status: draft`). Replace the revision line with one for revision 2 naming the author, the date, the commit the cites were read at, and the five findings folded; mark each changed passage "(revision 2)". Add a short section "Dispositions of Astra's round 1" listing each finding id and what changed. Write for a person: plain English, expand identifiers on first use, no review-round or finding numbers in proposed source comments.

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

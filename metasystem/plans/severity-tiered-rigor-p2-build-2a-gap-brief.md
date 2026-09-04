Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Build slice 2a, the risk basis: round two, the four gaps answered

Round one (job str-p2-build-2a) stopped under the gap rule on four
contracts it judged law-changing and left a clean tree. The dispatching
seat answers all four here; each answer follows what the tree already
does nearest the seam and is folded into the design as revision 4.3.
Build the whole of metasystem/plans/severity-tiered-rigor-p2-build-brief-2a.md
(items 1 to 6, the revision-4 fixtures and the nine STR4-R1 obligations)
with these answers in force. No gap remains open from round one; a new
gap must be one that neither the build brief, the design nor this brief
answers.

# Answer 1: the sweep row carries the four answers (amends 006 and 02)

The classification draft keeps its three-field positional shape, `<goal-id>
<answer> <text to end of line, 200 characters at most>`, and the second
field changes meaning: it is the four scores joined by commas,
`<severity>,<novelty>,<exposure>,<accumulation>`, each 1 to 3, and the
tail is the basis text. A bare tier in the second field is refused with
`SWEEP_MALFORMED_ROW: line <n> goal <id> must be <goal-id>
<severity>,<novelty>,<exposure>,<accumulation> <basis>`. The tool derives
the tier (16's table); a human never types one in the draft. The listing
line the digest binds is rendered by the tool, one of two forms:

- `<goal-id> <s>,<n>,<e>,<a> tier=<derived> <basis>` for a goal without
  a tier, or a tiered goal whose derivation is at or above its tier;
- `<goal-id> <s>,<n>,<e>,<a> tier=<current> HUMAN-DECISION derived=<d>
  <basis>` for a tiered goal whose derivation is below its current tier:
  the confirm writes the Risk record and keeps the current tier; lowering
  it is the human's act by 004.

The sweep selects every goal without a Risk record (006); a goal with one
is refused as SWEEP_UNKNOWN_GOAL, as a tiered goal is today. The digest
of the listing lines binds the confirm exactly as it does now.

# Answer 2: the admissible refusal codes are a named list (amends 007)

metasystem/internal/dispatch/admission.go declares one exported list,
`AdmissionRefusalCodes`, and it is the whole set that `refusal:<code>`
admits: `BUDGET_UNKNOWN`, `BUDGET_REFUSED`, `HAZARD_REFUSED` and
`RISK_UNANSWERED`. The hazard refusal that today prints without a code
("the hazard needs review the tier does not have; goal edit --tier 2")
gains the `HAZARD_REFUSED: ` prefix, nothing else about it changes; its
existing tests read the message after the prefix. `RISK_UNANSWERED` is
18's code. Evidence naming any other code is refused at edit time with
`evidence refusal:<code> is not an admission refusal code; one of:
<the list>`. Adding a code to the list is the only way to admit a new
one; the fixture STR4-R1-EVIDENCE-GRAMMAR uses one listed code and one
unlisted.

# Answer 3: a raise re-binds the revision and nothing else (amends 003)

The raise does not call `bindClaim` (metasystem/internal/goal/verbs.go),
which clears the launch fence and the governed obligation. It rewrites
only the revision the claim is bound to: `Claimed.Revision` and the stop
capability's `Generation` and `Revision` move to the bumped goal
revision; `ClaimEpoch`, `StopFence` and `Obligation` stay exactly as
they are. A raise of rigor never clears a stop or an obligation: a
breach-stopped goal can be raised, its fence stays, and only `goal
resume` clears it. Write that as one narrow function beside `bindClaim`
with the comment that says why, and add one test to
STR4-R1-RAISE-TRANSACTION: a raise on a breach-stopped goal with a
governed obligation keeps both records byte-for-byte.

# Answer 4: goal-free critics read the configured ceiling (amends item 6)

`goalReviewRoundLimit` has two readers. A root bound to a goal reads the
goal's `reviewRoundLimit` tuple member through the bound goal record, as
the other tuple members are read, capped at `config.ReviewRoundMax`
(metasystem/internal/config/budget.go, key
`metasystem.budget.review-round-max`, default 3). A root with `--goal
none-explicit` reads `config.ReviewRoundMax` alone: the configured
ceiling, not a literal, so goal-free chains keep their three rounds and
the constant is gone. Item 6's test stays (a goal whose tuple says nine
rounds folds a fourth round; nine above the ceiling three is clamped to
three, so set the ceiling to nine in that test's conf); add one line to
it: a goal-free root's limit equals the conf ceiling.

# Tree drift, accepted

The counselor writer `AppendMisclassification` exists in your base (2b
landed); call it. The landing owner is `observeChain` in
metasystem/internal/landing/observe.go, not the design's
`RefuseChainMembership`; write the width check there. Live setup flows
through claim-launch; follow the tree.

# Gate and constraints

As the build brief: gofmt, go vet, go build, staticcheck silent, every
package's tests green except internal/goal, whose tests you add and touch
run by name; goal-cli-fixtures.sh and dispatch-fixtures.sh once each if
the sandbox can run them. Stage nothing, no commit wrapper, nothing
under plans, records or memory. diffBoundary as the build brief.

Wall-clock budget: 45 minutes; build items 1, 2 and 4 first, then 3, 5
and 6; return by minute 40 whatever the state with the fast gate green
and the unbuilt items and fixtures listed by name. A round that ends at
the cap without a return is charged and proves nothing. Version-2
implementer JSON.

# Gap Rule

stop and report a gap; never fill it silently. The grain is the build
brief's: a contract that would change law and that none of the three
documents answers stops you; a mechanical choice (a field mapping, a
selector, a flag name, an order of two writes, a quoting rule for a
value the tree already quotes somewhere) is chosen as the tree does it
nearest the seam, recorded under `decisions` in the return, and built.

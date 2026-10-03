# Design brief: seat-path-lands-without-help

## Revision

Revision: revision 5 of the page `/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/seat-path-lands-without-help.md`

Reason: round 3 of the design critique (Codex on Astra, job design-critic-46437605ef0c580fea7461d1-r3) returned one material finding, accepted. This revision answers it with the amendment below. Nothing else changes.

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check.

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

### The prior revision

Revision 4 is the page on disk at `/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/seat-path-lands-without-help.md` (4,384 words, draft). Read it whole, once, before writing. It is not copied into this brief. Every section stands except where the amendment below changes it. Keep the head exactly (title, Kind, Id `01M3Z6EXJH4CCXWDNZA9DWB8X2`, Status draft, Goals). The line "Revision 4: ..." becomes revision 5, with one sentence saying it answers critique round 3.

The places this touches on the page: in section 2b, the bullet "A person's word stays in force for every retry" (it becomes the rule below) and the bullet "Each wait starts at its own hand-in" (its "the gate's word read from the new line" no longer holds for the gate); in section 6, the unit that carries the gate change and its witness, and the U1b.3 witness `TestASecondHandInMovesTheMark`, whose "no earlier word stands" now applies to the review's shown state only; section 9 if an item there is answered or newly open.

The threat model is unchanged: our own agents and operators make mistakes and crash; nobody attacks.

### The accepted amendment (the seat's decision, verbatim)

RULING-R-142-m1e: "A person's word binds to its tip, never to a mark. The gate (`Gate`, `internal/goal/landgate.go:240-270`) takes the newest word given at the tip it checks, over the whole claim; a word at another tip still says nothing about this one, and a send-back at this tip still refuses. The mark's window stays for what shows the review's state (`VerdictsOf`, `ReadGate` for the card, `LandingDue`), so a new hand-in still reads as a new review there. With this, the line-before-mark order stays (it keeps a stopped goal out of the queue) but no longer carries the word's safety. Witness: `TestAThroughHandInKeepsThePersonsWordForTheTip` (a tier at or above human-from-tier, the word given at the tip, a `--through` hand-in, then the hand-in of the tip passes the gate); mutation: read the word only since the mark. It lands no later than the unit that makes every hand-in mark."

Critique findings being answered:

1. RULING-R-142-m1e (high): a `--through` hand-in marks the goal, and the mark hides the person's word for the unchanged branch tip, so the next hand-in is refused for a word the person already gave.

Cited code excerpts:

1. `internal/goal/landgate.go:208-230`

   ```text
   func since(f *GoalFile) int {
   	if index := landingIndex(f); index >= 0 {
   		return index + 1
   	}
   	return 0
   }

   func newestWord(f *GoalFile) *word {
   	if f == nil {
   		return nil
   	}
   	var newest *word
   	for _, h := range f.History[since(f):] {
   		switch h.Verb {
   		case reviewVerb:
   			if line, err := parseReviewReason(h.Reason); err == nil {
   				newest = &word{kind: line.Verdict, tip: line.Tip, by: line.By, opid: h.Opid, record: line.Record}
   			}
   		case LandWithoutSittingVerb:
   			if decided, err := parseWithoutSitting(h.Reason); err == nil {
   				newest = &word{kind: LandWithoutSittingVerb, tip: decided.Tip, by: decided.By, opid: h.Opid, because: decided.Reason}
   			}
   		}
   ```

2. `internal/goal/landgate.go:236-270`

   ```text
   // under a standing hold at every tier; at or above the tier refused unless the
   // newest human word on the landing is clear to land or land without a sitting,
   // recorded against this tip; below the tier it proceeds. It answers the words
   // the landed line carries: what the landing is under.
   func Gate(f *GoalFile, tip string, s GateSettings) (string, error) {
   	if f == nil {
   		return "", &GateRefusal{Code: GateWaitsForHuman, Reason: "the goal is not live, so there is nothing its word could be read from"}
   	}
   	if holds := HoldsOf(f); len(holds) > 0 {
   		return "", &GateRefusal{Code: GateHeldBySitting, Reason: fmt.Sprintf(
   			"goal %s is held by %s's review sitting (%s); nothing lands while a sitting stands, whatever the tier; it lands once the sitting ends, which releases it: metasystem goal review %s --release --record %s",
   			f.Id, holds[0].By, holds[0].Record, f.Id, holds[0].Record)}
   	}
   	tier := GateTier(f)
   	if !s.WaitsForHuman(f) {
   		return fmt.Sprintf("landing.review.auto-after=%s, tier %d below human-from-tier=%d", s.AutoAfterText, tier, s.HumanFromTier), nil
   	}
   	said := newestWord(f)
   	missing := func(why string) error {
   		return &GateRefusal{Code: GateWaitsForHuman, Reason: fmt.Sprintf(
   			"goal %s is tier %d, at or above landing.review.human-from-tier=%d, and waits for a person: %s; a person reviews it in a sitting that ends clear to land, or lands it without a sitting: metasystem goal land-without-sitting %s --reason TEXT",
   			f.Id, tier, s.HumanFromTier, why, f.Id)}
   	}
   	switch {
   	case said == nil:
   		return "", missing("its history carries no clear-to-land verdict and no land-without-sitting decision")
   	case said.kind == VerdictSendBack:
   		return "", missing(fmt.Sprintf("its newest verdict is %s's send-back", said.by))
   	case tip == "":
   		return "", missing("this landing names no branch tip the word could be bound to")
   	case said.tip != tip:
   		return "", missing(fmt.Sprintf("%s's word was given at %s and the branch is now at %s; a moved tip needs the word again", said.by, short(said.tip), short(tip)))
   	}
   	return said.under(tier, s), nil
   }
   ```

3. `cmd/metasystem/intent_delivery.go:1863-1873`

   ```text
   	subject, _, refusal := handLandingSubject(targets, goalID, through, state)
   	if refusal != nil {
   		return *refusal
   	}
   	if refused := inv.admitLanding(targets, goalID, state.BranchTip); refused != nil {
   		return *refused
   	}
   	if configured {
   		return inv.handIn(targets, laneInstall, goalID, subject, state.EndpointTip)
   	}
   	return inv.landByHand(targets, goalID, through, subject, state, base)
   ```

Example page:

`/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/critique-findings-need-proof.md` (structure and level of detail). Revisions 1 to 4 of this page set the tone: short, in Wido's plain English.

## Recurring findings

none recorded

## Tool-call budget

Maximum delegate tool calls: 25

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: 4500 words

Cut a draft that exceeds this ceiling. If cutting would make the page
incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique
only through the context pack above. Never resume a delegate from an earlier
revision.

## Page artifact and return shape

Write the page to: /Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/seat-path-lands-without-help.md

Do not edit any other file, run test suites, build engines, claim or release goals, or commit.

Return only these two lines, N being the page's word count:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/seat-path-lands-without-help.md
DESIGN: ready (N words)
```

or:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/seat-path-lands-without-help.md
DESIGN: blocked (the reason in one line)
```

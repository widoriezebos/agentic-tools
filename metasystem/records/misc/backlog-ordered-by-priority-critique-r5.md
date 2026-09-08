# Backlog ordered by priority: second read of slice 2 (chain bolnext-build1, round 3)

Critic bolnext-crit2 (code-critic, Opus 5) on reviewed tree 08b423f4492631d0d1dc15b65019a9d4bb781fb8. Two material findings, one high and one medium, and three notes.

Both material findings trace to the previous fold's instructions being carried out narrowly. That is worth stating first, because it is the orchestrator's fault before it is the builder's: the fold brief told the round to call the shared claim-admission owner, and it did, without saying what to do with the errors that owner returns. It told the round to bring one sentence back to the page's scope, and the round deleted a standing instruction the page never mentioned.

## BOO-01 - high - material=True

CLAIM: The frontier discards every error the shared claim-admission gate returns and does not distinguish "this goal cannot be claimed" from "whether it can be claimed could not be determined". A rejected goal joins none of the four frontier categories: not ready, not blocked, not awaiting. Two consequences. A goal whose approved budget exceeds its tier norm disappears from every frontier reader with no stated cause, while still appearing in `goal list` at its rank, so a human sees a full backlog and a machine that says there is nothing to do, with nothing anywhere naming the norm refusal. Worse, the gate can fail for reasons unrelated to the goal: it reads the repository configuration on every call and errors on a retired key, a malformed tier-budget value, a budget key in the process environment outside a fixture-authorised root, or a budget override in a sibling .local file. Such an error is attributed to each goal in turn, so every approved goal drops and the entire backlog silently vanishes. Because idle-backlog enforcement resets its refusal counter and returns without blocking whenever the claimable list is empty, one configuration error also switches off the IDLE WITH BACKLOG safety refusal for every seat, fleet-wide, with no diagnostic.

EVIDENCE: internal/goal/project.go, the approved-state arm of the frontier loop, appends to the ready list only when the admission function returns no error and has no else branch, and the enclosing function returns no error value, so the cause cannot travel. internal/goal/approval.go calls a norm-coverage helper that calls the configuration reader in internal/config/budget.go, which errors on each of the listed conditions.

## BOO-02 - medium - material=True

CLAIM: The change deletes a standing instruction from the canonical agent contract that the specification did not ask it to delete, and leaves two other documents asserting the deleted text. AGENTS.md previously bound every runtime to a turn-end read of `goal next`; the new sentence scopes the read to a free seat while keeping the clause calling it the universal transport every runtime has, which only makes sense for a read that always happens. docs/design/turn-verdict-delivery-contract.md still states that AGENTS.md instructs every main to read it at turn end, under a section certifying the hook-less delivery path, and wow.md still routes the goal thread under ending a turn. So a seat holding a claim now has no stated turn-end read, and a design document certifies a contract the contract no longer supports.

## BOO-03 - low - material=False

CLAIM: The steward's pinned-goal list also changed from alphabetical to rank order and is persisted in that order. It raises no extra attention event because the pinned list is compared as a set.

## BOO-04 - low - material=False

CLAIM: Computing the frontier now performs disk reads proportional to the number of approved goals: each readiness check triggers a configuration read, in fact several, including a retired-key scan across the configuration file and its .local sibling.

## BOO-05 - low - material=False

CLAIM: Every canary can fail for the right reason, but the proofs are not distributed the way the row names suggest, and the orchestrator should know which row carries which proof.

## Coordinator's reading (m1b, 2026-09-08)

Both material findings accepted. BOO-01 is land-blocking and the reason
is its second half rather than its first: a single configuration
typo, in a file or its .local sibling, would empty every seat's backlog
silently and simultaneously switch off the one refusal that stops a seat
idling while work waits. That converts a small local mistake into a
quiet fleet-wide stand-down with no diagnostic anywhere. It also lands
squarely on this goal's own purpose, which is that machines take work
through this verb.

BOO-04 is recorded rather than actioned but belongs beside BOO-01 in the
same repair: if the readiness check is going to read configuration once
per approved goal, the fix that stops errors vanishing should also stop
the file being read ninety-nine times to answer one question.

BOO-02 is cheap and should not be left, because two other documents now
describe a contract that no longer exists, and one of them is the design
that certifies delivery without hooks.

This is the third fold cycle on this build. Per the standing rule the
coordinator set for itself after the stop-verb loop, a third cycle does
not open on its own authority: the findings, their grading and the
recommendation go to Wido, and this register is written so the decision
has something to stand on.

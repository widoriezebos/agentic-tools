Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-09

# Fold round nine: the arc forms take the same proof; an agent found is always named

Follow-up round on chain hp-terminal-build1-20260909; your worktree
carries round eight. The seat gate on round eight is green (all four
packages, the live walk reaching process 1 through nine nodes,
proof-grades passing its agent-descended branch, the hook suite). The
sixth critic (hp-terminal-crit6-20260909) found two material defects
and one note; all three fold here.

## HPT-20 (critical): the --arc cascade forms never receive a proof

Round eight put the terminal-grade proof behind the single-goal forms
of release, park and unpark. Their cascade forms, reached with the
--arc flag every goal sub-command accepts, still decide on the bare
presence of --by. In internal/goal/verbs.go the three cascade builders
gate on the name alone: releaseArcRequest (`if !ownPair(m.Claimed,
r.Actor) && r.Actor.Human == "" { continue }`), parkArcRequest (`if
r.Actor.Human == "" && m.Origin == OriginHuman { continue }`),
unparkArcRequest (`if m.Parked != nil &&
strings.HasPrefix(m.Parked.By, "human:") && r.Actor.Human == "" {
continue }`); none calls requireHuman. In
cmd/metasystem/goalsync_mutations.go the proof-carrying builder
syncStoppingReq is chosen only when the arc flag is empty, so the
cascade path leaves VerbRequest.Authority nil. An agent shell running
`metasystem goal park --id <arc root> --because ... --by Wido --arc x`
parks goals a human opened and displaces other pairs' claims; the
unpark and release forms do the matching harm. No walk runs, no agent
is detected.

Fix: the arc forms build their request through the same
proof-carrying path as the single-goal forms, and every member act in
a cascade that the single-goal form would put behind requireHuman at
the terminal grade goes behind the same call with the same row; a
refusal on any member refuses the cascade with the same
AGENT_IN_AUTHORITY_CHAIN naming the runtime. The proof-grades scenario
gains the arc forms: agent_refuses for park, unpark and release with
--arc (exact refusal, as round six made them), and on an agent-free bed
one allow case per arc form.

## HPT-21 (medium): a walk that finds an agent must say so

Round eight remembers the matched runtime in a local variable and
reports it only at the top of the tree; if the walk exits earlier above
the agent (unreadable parent, changed parent, reused process, cycle) the
caller gets that bookkeeping outcome and never learns an agent was
found. Rounds one to seven refused AGENT_IN_AUTHORITY_CHAIN at the
matching node. Fix: an agent match is the decisive fact; on any exit
after a match, the outcome is AGENT_IN_AUTHORITY_CHAIN naming the
runtime, the proof still records the nodes read, and the later
uncertainty is recorded on the proof but does not rename the refusal.
The walk may still continue to the root after a match to record it, as
round eight does; the outcome must not depend on getting there.

## HPT-22 (low): one extra iteration in the lease classifier

internal/lease/classify.go walks parents with `for ok && !seen[current]`
and no `current > 0` guard, so the new "parent none, known" at process
1 costs one lookup against process 0 before the loop ends. Harmless;
add the guard the other callers of identity.ParentPid already have.

## Mandate

1. HPT-20: the arc forms carry and require the same proof; scenario
   cases for the three arc forms.
2. HPT-21: an agent found is always the outcome; tests for a matched
   agent below an unreadable, changed, reused or cyclic ancestor.
3. HPT-22: the guard.
4. Nothing else changes.

## Proof

Run internal/humanauthority (verbose for the live test; paste its
line), internal/goal, internal/lease, and the command-package tests
that touch park, unpark, release and session stop; gofmt, vet; bash -n
on the fixture. The orchestrator runs the full packages, the goal-cli
bed and the hook suite on the seat. Report the round as your own.

## Constraints

Wall-clock budget: 25 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.

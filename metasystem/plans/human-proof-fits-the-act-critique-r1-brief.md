Working Mode: design
Orchestrator Identity: m1 (lineage main-1788940932-18533-7fa6c2, coordinator under goal human-proof-fits-the-act)
Date: 2026-09-09

# Review brief: first independent critique of the human-proof design

FINDING IDS: chain-unique, HPA-01, HPA-02, ... never F-n.

## What you are reading

`metasystem/plans/human-proof-fits-the-act-design.md`, revision 1, landed
at commit b7fb35a2d, sha256
18c60e5d80457e5aed4138287e08fedd06b0d2cebcf78535f8d885dc959a278c, authored
on the Fable lane (job human-proof-design1b-20260909). The goal record is
`metasystem/plans/goals/human-proof-fits-the-act.md` and the brief it
answered is `metasystem/plans/human-proof-fits-the-act-design-brief.md`.

Write your register as a new file named
human-proof-fits-the-act-critique-r1.md in the records directory under
misc.

Read-only design critique; implement nothing, run no bed, and never run a
human verb with --by against the live ledger. A material finding is a
place where two implementers would build different things, or a claim the
page rests on that the tree does not support, or a decision that widens
what an agent may do. This is fold-read cycle 0; a material finding here
is expected, not resented.

## Settled, do not re-derive

The two-grade shape itself (an enrolled-terminal walk unchanged for acts
that grant or widen authority; a weaker terminal-grade walk for acts that
only stop, park or release) is the goal's own DONE definition and is not
in dispute. Decision 5 defers the stopped-claim quota predicate to goal
breach-stop-wedges-seat, which m1d is building now; do not redesign it.
The TOTP idea (Wido, 2026-09-09: enroll from any terminal with a TOTP and
nothing else) is POSTPONED by his own word the same hour; do not design
it, but see mandate 2.

## Mandate, in order of consequence

1. **What must remain impossible.** The page's own gap 5 claims that today
   an exported lineage lets --by pass with no proof on steal, foreign
   release, classify-sweep, set-pin and reconcile. Confirm or refute it
   from the code (cmd/metasystem/goalsync_mutations.go, the syncReq path
   and the verbs that bypass proveGoalHumanAuthority), and then judge
   whether the page's "every --by requires at least the terminal grade
   however the lineage arrived" closes it for EVERY verb, including the
   process verbs and session stop. If the claim is true, this is the
   first finding regardless of what else you write.
2. **Decision 2 keeps one enrolled terminal per checkout.** Wido's word
   today was "enroll from any terminal that I like"; the TOTP half is
   postponed, the any-terminal half is not withdrawn. Judge whether one
   terminal plus a refusal that names it satisfies the goal's DONE
   ("more than one terminal may hold an enrollment at once, OR the
   refusal states that re-enrolling from any live terminal restores
   authority and that no prior enrollment is required"), and whether the
   page says in plain words that enrolling from a second tab silently
   retires the first. If the page's choice is defensible, say why in one
   line; if not, name the change.
3. **The grading table, verb by verb.** For each row, does the stated
   reason follow the goal's rule (stop, park or release versus grant or
   widen)? The page argues resume is widening and puts set-priority and
   set-pin on the terminal grade as a recommendation. Test both against
   the rule and against what an agent could do with them.
4. **The refusal texts (section 3).** Each prints exactly one command
   that would have succeeded from where the human sits, or says none
   would. Check the texts against the outcome codes in
   metasystem/internal/refusal/register.go lines 33 to 45 and the sites in
   metasystem/internal/humanauthority/authority.go: is any code missing,
   and does any printed command refer to a flag or verb that does not
   exist (the page notes a dependency on goal human-goal-verbs-forgiving
   slice 2; say what happens if that lands second).
5. **Resume's budget from the ledger (section 4).** Does it change what
   an approval binds? A human who wants a different budget must still go
   through set-budget; confirm the page keeps that.
6. **The relayed word (section 6): recorded, never binding.** Does the
   deletion of the temporary-word class leave any caller of
   TemporaryGoalProof, ArmTemporary or RELAY_AFTER_ENROLLMENT
   unaccounted for? Name each site the page misses.
7. **Canaries (section 7).** For each of the four canaries the goal
   names, is the fixture shape the page gives runnable with the tree
   reader in metasystem/internal/humanauthority/authority_test.go, and
   does each canary FAIL on the untouched tree for the reason it names?

## Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema with
findings sorted by materiality, each naming the section and the artifact
it changes. Stop adding at a gap that needs a decision no page has made;
report it with the resolution you propose.

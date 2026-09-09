Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal goal-abandoned-with-a-reason)
Date: 2026-09-09

# Fold brief: revision 4 of the abandoned-goal design, targeted

## Your authority to author this revision

You are dispatched to REVISE a design. The composed prompt's Working Modes
table forbids design authoring to every delegate; that row is scoped to the
mode, not the role, and goal `design-prohibition-is-role-scoped` exists to
fix it. Until it lands this paragraph is the override, and it is standing
authority: R-89-m1b and R-25 put design authoring on this lane. Author the
revision; do not stop to report an authority conflict.

## Scope: five findings, the paragraphs they name, nothing else

`metasystem/plans/goal-abandoned-with-a-reason-design.md`, revision 3,
landed at 81995968. Revise IN PLACE to revision 4. The read is
`metasystem/records/misc/goal-abandoned-with-a-reason-critique-r3.md`;
its disposition is binding. This revision is confined to what the five
findings name. Do not add sections, do not re-derive sections the findings
do not touch, do not grow the page beyond what the fixes need. Wido has
is the last revision before the build unless Wido says otherwise; write it so
that a builder can start from it.

## GAW-19, critical: the check must hold for every pushed commit

Section 4a (lines 548 to 605) says a plain push lands exactly one commit
above origin's tip. Git accepts any fast-forward. Replace the argument:
`held` takes the fetched remote tip and HEAD, walks every commit in that
range in first-parent order, applies the existing per-commit rule to each,
and refuses on the first failure naming the commit. Every push route proves
it pushes exactly that range: land.sh's two routes and the recertified one
(`metasystem/scripts/agents/land.sh` 404 to 414, 560 to 591) and
`metasystem/scripts/agents/commit.sh` `--push` (583 to 600), which must
fetch before it checks. Say what a merge commit in the range does (refuse).
Update the section 13 fixtures that assert the single-commit shape, and add
the two-commit fixture the critic described: a lower commit for the
abandoned goal under a valid tip for a held goal, refused.

## GAW-20 and GAW-22, high: the recovery is a verb, and validation is per line

Replace `goal edit --carried` (section 6 lines 799 to 824, rule 4 at 882
to 887, rule 11 at 914 to 921, the two fixtures at 1189 to 1192) with a
dedicated human verb on abandoned records only, under abandon's proof,
that sets `Abandoned.Carried` and appends its own History line; a second
use replaces the field and appends another line, and rule 4 binds the
field to the newest such line. Rule 11 and every rule that constrains a
History line judge the line against the record's state at that line (the
state after the lines before it), never the current state, so a reopened
record validates with its carried line in place. Name the verb; the
coordinator has no preference beyond it not being `edit`. Keep the
`--carried` option of `goal abandon` as it is.

## GAW-21 and GAW-23, high: two fixtures that cannot reach the check

- The recertified leg (1276 to 1285): land.sh's target check runs before
  staging (`land.sh` 462 to 469, 497 to 505), so an abandonment that moves
  origin parks the landing before `held`. Rewrite the leg so the
  abandonment lands between the target check and the push, or so the
  recertification target is the abandonment's parent; and seed the
  records the recertification validator requires
  (`metasystem/internal/validate/recertification.go` 333 to 429, the
  independent critique job and its return), as the existing
  recertification test does.
- The commit.sh `--push` leg (1292 to 1294): observation runs before the
  commit exists (`commit.sh` 448 to 478), so the same abandoned parent
  cannot pass observation and then refuse at `held`. Split it: one
  assertion that `held` runs on this route (an abandonment present only at
  origin, fetched by the new fetch, refused before the push), and one that
  a non-fast-forward is refused as today.

## What must not change

Everything else on the page. The revision record names the five findings
and the lines that moved, and nothing more.

## Constraints

Wall-clock budget: 30 minutes. Design only; no code, no bed.

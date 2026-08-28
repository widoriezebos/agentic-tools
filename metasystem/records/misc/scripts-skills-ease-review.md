# Scripts and skills from the agent seat

agent-ease-assessment, slice three (Appetite: 4h). Same method as
slices one and two; findings fixed-in-slice where deployable.

## Fixed in this slice (the day reviewing itself)

1. battery.sh (born this morning) had NO argument discipline —
   `--help` launched the full forty-minute suite; found when this
   review's probe timed out on it. Fixed: usage on any argument,
   help exits 0, unknown 2, all three probed.
2. Neither critique skill referenced the review-brief template the
   appetite law shipped the same day — the orchestrator brief
   mandated it, the skills had not caught up. Both linked.
3. critique-round.sh answered a bare call with bash parameter noise
   ("1: chain name"); now proper usage, help 0, missing-args 2.

## Surveyed, judged fine

4. Static sweep of all 56 scripts for argument handling: the large
   argless cohort is FIXTURE SUITES, argless by design — running IS
   their intent, and the battery/validate wrappers own their env.
   No action.
5. The seven skills read current after today's linkage; refactor
   and verify carry their own entry contracts; retro and
   take-a-step-back are prose-only by design.

## Banked for later slices or items

6. commit.sh passes unrecognized leading flags to git commit (an
   agent typo becomes git's error, not the wrapper's) — cosmetic,
   note only.
7. receipt.sh and frontier.sh lack --help but their headers
   document usage; below the fix line for this slice.

## Slice summary

Three deployable fixes shipped inside the slice, all on tools born
or touched TODAY — the newest code is where entry discipline
slips, which argues for the review-at-birth habit over periodic
audits. Slice four (docs) remains tokened separately.

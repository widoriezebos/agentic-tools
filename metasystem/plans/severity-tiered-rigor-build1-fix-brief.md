Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-03

# Goal

Correction round on chain str-build1c (your reviewed tree d9402254).
The code review (str-build1c-cc1) found two material defects and six
notes. Fold the two defects and the two small notes named below; the
rest are settled.

# The defects

F-1, callers left on the old goal open surface. `goal open` now
refuses a call without `--tier`, and four fixture scripts still open
goals without it, so validate-metasystem.sh goes red at landing:
metasystem/scripts/agents/dispatch-fixtures.sh (line 1571),
metasystem/scripts/agents/supervision-fixtures.sh (line 1599),
metasystem/scripts/adopt-fixtures.sh (line 484; its non-human branch
also expects a refusal mentioning 'lease holder', which the tier check
now pre-empts, so both branches fail), and
metasystem/scripts/agents/channel-fixtures.sh (lines 60 and 98). Fix
every caller in the module: `grep -rn "goal open" scripts/ cmd/
internal/ --include='*.sh' --include='*.go'` and give each a tier
consistent with its fixture; keep the adopt fixture's lease-holder
refusal reachable by giving that call a tier. Run each touched fixture
script.

F-2, the TierLaw marker is never installed on a ledger with no
tierless goal. `classify-sweep --confirm` on an empty listing prints
"the tier law is already installed and no tierless goals remain"
without reading or writing the root record, and the marker is written
only inside the last per-goal edit. Fix: confirm reads the root
record; when the marker is absent it appends `TierLaw: since=<opid>`
as its own root-record change (the opid of the confirm act) whether or
not any goal was edited; when the marker is present the message is
true. Fixture: a ledger opened entirely with --tier, confirm on the
empty listing installs the marker, a hand-written tierless goal is
then refused by delegate.

# The two small notes to fold

F-7: a stored four-member budget parses its review-round member from
the goal's tier box (the tier-3 box while it has no tier), as
revision 3 point 03 says, not a fixed 3.

F-8: re-running the preview after an interrupted confirm lists only
the still-tierless goals: a draft row whose goal already carries a
tier is skipped silently (it is neither unknown nor duplicate), so the
gap-2 brief's recovery sentence holds; SWEEP_UNKNOWN_GOAL stays for
ids that are not goals.

# The notes, settled

- F-3 (the mission-cap obligation stays a documented skip): accepted
  as a residual of part one, recorded for m3's part two which owns
  the mission fence seam.
- F-4 (a test name overstates its leg; the follow-up inheritance
  refusal has no test): add one focused test for the follow-up
  inheritance refusal in BuildFollowRecord; renaming is optional.
- F-5 (dispatch.cap-max readable from the environment): recorded for
  the design owner; not changed in this round.
- F-6 (confirm takes a name without proof): follows the gap-2
  contract; recorded for the design owner.

# Gate

As in metasystem/plans/severity-tiered-rigor-build1c-brief.md, plus
every fixture script you touched, run green. Declare the boundary as
every file that differs from main.

# Constraints

Wall-clock budget: 60 minutes; return before it ends even if something
is red, naming it. Gap rule: stop and report a gap with your proposed
contract written out.

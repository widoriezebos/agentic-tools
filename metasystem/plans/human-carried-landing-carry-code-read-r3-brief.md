Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, coordinator under goal human-carried-landing-carry, third closing read of the carried landing, chain hcl-build3-20260911, final work round 2)
Date: 2026-09-11

# Review brief: third read of the carried-landing implementation

FINDING IDS: chain-unique, continue the carry's register: HCL-C-66, HCL-C-67, ... never F-n.

## What you are reading

Chain hcl-build3-20260911 for goal human-carried-landing-carry
(metasystem/plans/goals/human-carried-landing-carry.md), final work round
2 (job hcl-build3-20260911-r2), reviewed tree fdfcd002b69a1a39fe681956d9226e245aea7127 (the
installation subtree the conformance review names as reviewedTree). The
specification is metasystem/plans/human-carried-landing-carry-design.md
revision 6 with the coordinator's three amendments (the stop list's
empty-staging rule excludes the rerun states; single-machine mode out of
scope with `carry-remote-required`; `goal carry` speaks after its
transaction). The second read,
`metasystem/artifacts/agents/hcl-context/code-read-r2.md` (Opus,
hcl-cc2-20260911), found sixteen findings; the fold
`metasystem/artifacts/agents/hcl-context/build3-fold-r2-brief.md` holds
the coordinator's decision on each, names which page behaviours are built
now and which are recorded as follow-ups by id, and is binding. Round 2
built it; the coordinator ran every bed on the host.

Write your register as this new file: `metasystem/records/misc/human-carried-landing-carry-code-read-r3.md`
(a read-only runtime returns it in the job return; the coordinator
projects it).

This is the closing read of a DESTRUCTIVE-REACH chain. A material
finding is: a fold decision not carried out as decided; a behaviour the
design specifies that the code does not have, or one it forbids that the
code has, in the code CHANGED since the second read; a way a carry lands
what the human did not name or refuses a verified human on the machine's
own judgement; a fixture that does not test what it says. A page fixture
the fold names as a follow-up is NOT a finding; say so in one line and
move on.

## Mandate, in order

1. **The fold, item by item.** The host section's three items, defects 1
   to 4 (HCL-C-50 to -56), proofs 5 to 8 (HCL-C-57 to -60 as scoped): one
   line each, folded as decided at file:line, or not, with what differs.
   Rule: no fixture claims what it does not drive.
2. **HCL-C-50 in full.** Every `git show <rev>:<path>` and every path the
   wrapper hands to git in land.sh and commit.sh resolves under a prefixed
   installation; the `carried-prefixed` leg installs under a prefix and
   lands.
3. **The row is always closed.** Trace every exit of land.sh between the
   reservation and the push, the intent-entry interval included: each
   leaves a closed row or prints why it could not.
4. **The second carry.** The counselor register's append-only admission
   accepts only `cl-<opid>` lines of carried rows on the tree; a
   goal-less new records/counselor file is refused.
5. **The red battery.** The `carried-red-battery` leg lands a group word
   on a red battery through commit.sh's red path and writes the
   `:battery-red` obligation; the match table's rows assert the page's
   exact outcomes.
6. **What must not change.** The identity gate equals `goal set-budget`'s;
   a record failure is never carried; the temporary word is not a proof;
   nothing in the shared testing contract or the workspace receipt
   weaker; the ordinary delivery digest rule equals the base's;
   `sync-transport.sh` untouched; trunk's changes since the rebase intact.
7. **The four named checks of revision 6** (HCL-C-33, -25, -03, -34): one
   line each.

## Return

Findings by id with file and line, the design or fold section, the
claim, the evidence, and what changes if it stands; the fold lines; then
a verdict line: closable as is, or one fold naming which findings.
Wall-clock budget: 45 minutes.

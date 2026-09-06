# Account-provenance design critique — round 2 (Sol)

Chain: design-critic account-provenance-crit2, 2026-09-06, reviewing
`plans/account-provenance-design.md` revision 2 at commit 57e179ab
(sha256 d09f24f4). Two material findings, no gaps; revision 3 folds both by
id in the design's fold record.

## account-provenance-r2-setup-husks-block-retirement — medium

CLAIM: The retirement rule includes setup-only job records that neither
completed account writer can populate. `BuildSetup` publishes `pending-setup`
with `mainId` and no account; `fail_setup_husk` preserves it as `failed`.
Such a record always fails the retirement predicate, so one refused dispatch
keeps the hand-written stamp for the lifetime of that main.

DISPOSITION: accepted. The evaluator skips records whose `phase` is `setup`;
`BuildSetup` stays a non-writer; the retirement fixture gains the husk and
pending-setup cases.

## account-provenance-r2-devin-nonzero-cause — medium

CLAIM: The devin mapping records `not-logged-in` on every nonzero exit, but
the design's own observation shows nonzero exits from denied network access
and a log-file panic. Exit status alone distinguishes none of those causes;
an implementer following the table would write false provenance.

DISPOSITION: accepted. Nonzero exit is `adapter-failed`, exit 0 is
`surface-unmapped`; `not-logged-in` needs a recognized logged-out output
from the named observation record; a devin floor fixture pins both.

Noted, not findings: the six other round-1 folds were judged sufficient;
the closed six-key contract, the twenty-second bound, and the landing
evaluator's caller resolution were read and confirmed.

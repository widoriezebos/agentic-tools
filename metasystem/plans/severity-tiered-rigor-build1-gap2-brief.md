Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-03

# Goal

Answer the second gap on chain str-build1 (the classify-sweep draft
contract) and build part one per
metasystem/plans/severity-tiered-rigor-build1-brief.md with the keys of
metasystem/plans/severity-tiered-rigor-build1-gap-brief.md. Nothing
else changes.

# The classify-sweep contract (design point 02, revision 3)

`metasystem goal classify-sweep --root <checkout> --draft <file> --preview`
and `... --draft <file> --confirm <digest> --by <human>`.

The DRAFT FILE: UTF-8 text, one row per line, `<goal-id> <tier> <reason>`
separated by single spaces; `<tier>` is 1, 2 or 3; `<reason>` is the
rest of the line, non-empty, at most 200 bytes, no tabs or control
characters; blank lines and lines starting with `#` are ignored. Rows
in any order.

The LISTING the preview prints: every OPEN goal on the accepted tree
that has no tier (queued, approved, claimed or parked; done goals are
never touched), one line each, `<goal-id> <tier> <reason>` normalized
(trimmed, single spaces), sorted by goal id ascending, followed by one
line `listing-digest <sha256>` where the digest is the sha256 of the
normalized listing lines joined by newline with a trailing newline. The
digest is over the NORMALIZED listing, never the raw draft bytes.

Refusals, each naming the offending id: a draft row whose goal id is
not an open tierless goal refuses `SWEEP_UNKNOWN_GOAL`; two rows with
one id refuse `SWEEP_DUPLICATE_GOAL`; an open tierless goal absent
from the draft refuses `SWEEP_INCOMPLETE`; a malformed row refuses
`SWEEP_MALFORMED_ROW` with its line number. Confirm recomputes the
listing from the current accepted tree and the draft and refuses
`SWEEP_LISTING_CHANGED` when its digest differs from `--confirm`.

Confirm applies, under the human actor named by `--by`, one `edit`
transaction per goal that sets the tier (revision 3, point 02: admitted
on another pair's claim; re-binds the approval digest of approved
goals), in listing order, and finally appends `TierLaw: since=<opid>`
to the root record with the opid of the last edit. A confirm that is
interrupted leaves the already-edited goals tiered and the marker
absent; re-running the preview then lists only the rest. The preview
mutates nothing.

Fixtures (goal-cli-fixtures.sh): preview prints the listing and the
digest for a three-goal fixture ledger; confirm with the right digest
tiers all three and appends the marker; a changed draft refuses
SWEEP_LISTING_CHANGED; each of the four refusal codes fires once;
before the marker a tierless goal dispatches under tier-3 rules, after
it `delegate` refuses a tierless goal as revision 3 says.

# Everything else

As in the two briefs above: the change list, the four binding test
obligations, the two keys, the gate, the boundary rule, the wall-clock
budget (90 minutes from now), and the gap rule. If a further seam is
silent in the design and the briefs, report it as a gap with your
proposed contract written out, so the next answer is one word.

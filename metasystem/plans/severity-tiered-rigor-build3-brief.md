Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-04

# Goal

Build PART THREE of the tiering machinery, the tier-1 landing class,
from metasystem/plans/severity-tiered-rigor-design.md revision 3: the
sections "STR2-TIER1-PROTECTED-PATHS-12: a floor list and a hazard
rule" and "STR2-TIER1-EVIDENCE-13: a receipt bound to the tree, a diff
metric", revision 2's point 6 as they amend it, and the build list's
part three. Part one is on main (6c86953a): the Tier field, goalTier on
chain roots, the tier boxes, the hazard-under-tier-1 refusal in
admission.go (point 12's second half is landed; verify and do not
redo it). Part two (another seat, goal severity-tiered-rigor-p2) adds
`gateWidth: area|full` on chain roots per design revision 4
(metasystem/plans/severity-tiered-rigor-p2-design.md,
STR4-TIER-DERIVATION-16): part three READS that root field by name
when present and treats an absent field as `area`; it never writes it.

The closing design review's finding on this part is a BINDING TEST
OBLIGATION (R-55-m1's cap rule): the test it names must exist and the
implementation must pass it.

## STR3-TIER1-RECEIPT-PROOF-06 (critical)

Claim: The Tier 1 test receipt is only labelled with a tree; it is not proof that the command ran against that tree. A caller can supply any existing candidate SHA while executing the command in a different working-tree or index state, and observeDirectFix will accept the matching label and zero exit status. The named test obligation must change the checkout or index away from the supplied tree and require receipt creation to refuse. This changes metasystem/cmd/metasystem/landing_verbs.go and the receipt validation in metasystem/internal/landing/observe.go.

Evidence: Revision 3 lines 458-464 say the command records the supplied tree and that the observer compares that field. They do not require materializing the tree or checking the working tree and index before and after execution.

# The change (part three)

1. scripts/agents/path-classes.txt: the `floor:<path> tier-1-refused`
   line kind with the rows point 12 lists; internal/pathclass reads
   them into a `Floors` set; the four classes unchanged.
2. scripts/agents/landing-classes.json: the `tier-1` direct-fix class
   as revision 2 point 6 defines it (bounded by changed lines, the
   floor, the receipt); internal/landing/observe.go `observeDirectFix`
   for `tier-1`: refuses `tier1-floor-refused` when any changed path is
   under a floor row; requires a test receipt whose tree equals the
   candidate tree with exit status zero (the obligation above: the
   receipt must PROVE the command ran against that tree, not merely be
   labelled with it; design and test the binding so a receipt created
   while the checkout or index differs from the supplied tree is
   refused at creation); the diff metric: changed lines = added plus
   deleted from `git diff --numstat base..candidate`; a binary `-`
   count, an R or C shape from `git diff-tree -M -C --name-status`, or
   a mode change with zero lines refuses `tier1-diff-shape-refused`;
   when the root's gateWidth is `full`, the receipt's command must be
   the full battery command (name the exact string in the class row).
3. cmd/metasystem/landing_verbs.go: `metasystem landing test-receipt
   --root <r> --tree <sha> --command "<cmd>"` runs the command and
   writes artifacts/agents/landing/receipts/<tree>.json with tree,
   command, exit status, time, and whatever binding the obligation
   needs; `ObserveParams` gains `TestReceipt`.
4. scripts/agents/land.sh gains `--tests "<cmd>"` (creates the receipt
   for the candidate tree before observation); scripts/agents/commit.sh
   passes `--test-receipt` to `landing observe`; the message stamp of
   the old tier-1 rule is dropped.
5. Fixtures in scripts/agents/land-fixtures.sh (and observe_test.go):
   a tier-1 landing under the floor refuses; a receipt for a foreign
   tree refuses; a receipt created while the index differs from the
   supplied tree is refused at creation; forty-one changed lines
   refuse the bound; a binary, a rename and a mode-only change each
   refuse tier1-diff-shape-refused; a lawful tier-1 landing with a
   valid receipt passes; gateWidth full requires the full battery
   command.

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l . (empty)`;
`go test ./internal/landing/ ./internal/pathclass/ ./cmd/metasystem/ -count=1 -timeout 25m` green;
`bash scripts/agents/land-fixtures.sh` and `bash scripts/agents/static-reproof-fixtures.sh` green.
Do not run path-class-fixtures.sh (ripgrep is absent on this host).

# Constraints

Wall-clock budget: 90 minutes; return before it ends even if something
is red, naming it; run the slow tests once at the end. DESIGN-BEARING
reach. Declare the boundary as every file that differs from main, with
the metasystem/ prefix. Gap rule: stop and report a gap with your
proposed contract written out, so the answer is one word.

# Expected Return

Per the implementer schema: the boundary; the test that proves the
obligation; every gate command with its observed result; anything
still red, named.

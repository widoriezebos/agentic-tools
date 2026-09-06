Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal land-sh-omits-the-full-width-chain-receipt, tier 3, hazard DESIGN-BEARING, code critique round two of chain lsr-build1)
Date: 2026-09-06

# Review brief: land-sh-omits-the-full-width-chain-receipt, round two

Round budget: three focused rounds for the goal's tier-3 box; this is
round two, after one fold. The orchestrator adjudicates every finding; you edit nothing.

Threat model: one seat landing reviewed chains on one machine, no
adversaries. In scope: a full-width chain that lands without a valid
receipt, a receipt accepted for a candidate it does not name, a tier-1
landing changed in any way, a usage combination that silently does
something other than what it says, a landing fixture leg that passes
vacuously, and any bash-4 construct. Out of scope: hostile inputs, the
landing evaluator in the engine, and the narrator-digest race the
receipt has on a live seat (its own goal).

Scope: the computed diff of chain lsr-build1 after follow-up round two
(job lsr-build1-r2) against its base. Round one implemented
metasystem/plans/land-sh-omits-the-full-width-chain-receipt-brief.md
(landed aa04f3d9); round two folded three of your notes per
metasystem/plans/land-sh-omits-the-full-width-chain-receipt-fold-r2-brief.md
(landed 13a4cd72): every refusal the lane adds prints a "land refused:"
sentence before the usage line, an unreadable receipt says why, and the
fixture scenario gained a first leg for --tests with --test-receipt.
Both bind. The computed diff is
metasystem/artifacts/agents/lsr-build1/rounds/2/diff.patch and its
reviewed tree is 13963bf2faefc7ea642e5e04f03b30814e8a5ac8; carry that
hash into your return exactly.

# Goal

Say whether the change ships a defect, violates its brief, or damages
what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to attack

0. The fold: the three usage refusals and the unreadable-receipt
   refusal each say why before the usage line and exit 2; the checks
   are ordered so the most specific combination wins; the new fixture
   leg cannot pass with the sentence absent.

1. Flag handling in metasystem/scripts/agents/land.sh: --test-receipt
   only with --chain; --tests and --test-receipt together refused;
   usage text; the receipt path forwarded unchanged to commit.sh.
2. The early refusal: it reads gateWidth from the chain root's job
   record, fires only for width full without a receipt, names the exact
   command and flag, exits 2 before any staging or commit, and does not
   fire for area-width roots or for tier-1 landings.
3. The tree check: the receipt's tree is compared with the staged
   candidate's metasystem subtree computed the way create_test_receipt
   computes it; a mismatch names both trees; a match proceeds; the
   check happens after staging (so --staged-only and pathspec forms
   both see the real candidate).
4. The fixture in metasystem/scripts/agents/land-fixtures.sh: the new
   full-width-chain scenario has three legs (no receipt refused, wrong
   tree refused, matching receipt lands with pass bar a), is registered
   in the bed's list and count line, and cannot pass with the refusal
   messages absent.
5. Conformance: only the files under the brief's May-touch list
   changed; nothing under plans; tier-1 fixture legs untouched.

# Evidence you may run

If your runtime gives you a shell, from the reviewed worktree root (the
metasystem directory): `bash -n ./scripts/agents/land.sh`,
`bash -n ./scripts/agents/land-fixtures.sh`, `bash --version`. The land
bed itself is not yours to run; the orchestrator runs it seat-side.

# Expected Return

The code-critic role's version-3 JSON with every required property,
the reviewed tree hash carried exactly, findings only, each with a
stable id, severity, a boolean material value, a plain-English claim
and evidence marked ran, read or inferred; one rigor row per material
finding. A round with zero material findings is the closure candidate;
say so plainly and list what you checked.

# Gap Rule

stop and report a gap; never fill it silently.

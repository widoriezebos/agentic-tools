Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal turn-verdict-hardening)
Date: 2026-09-02

# Goal

Round-2 critique of metasystem/plans/turn-verdict-hardening-design.md
(revision 2, landed, in your worktree). Revision 1 drew nine material
findings from your lane
(metasystem/records/misc/turn-verdict-hardening-critique-r1.md — the TVH-R1
identifiers); revision 2 claims to close every one of them by name, and folds
two rulings from Wido recorded as R-47-m0b in metasystem/memory/rulings.md:
a relayed human word MAY mint HUMANSTOP; an unbudgeted queued goal is NOT
READY (stored budget only). Those two words are law, not findings.

# Your mandate

1. CLOSURE CHECK, one verdict per finding: for each of the nine TVH-R1
   identifiers, is the closure real (the design text now specifies what your
   finding said was missing, and the cited code agrees), partial, or
   cosmetic? Read the cited code lines — metasystem/internal/goal/verbs.go
   for the claim rule order and the ClaimAdmission extraction,
   metasystem/internal/run/run.go for the run record fields section 2.2
   cites, metasystem/internal/report/scanjobs.go and
   metasystem/internal/lease/verbs.go for the exact-identity liveness in
   section 2.1, metasystem/scripts/agents/supervision-hook.sh for the trap
   structure and pre-verdict rows of section 3.0, and
   metasystem/internal/runtimes/runtimes.go (runtime facts) for section 6.
2. ATTACK THE STOP BUDGET (section 3.2): the author names this the weakest
   claim. Is the cap table's arithmetic sound, and is killing the
   supervision-arming verb mid-way genuinely no worse than today's kill?
3. ATTACK THE FOUR-SLICE CUT (section 10): slice 1 is estimated at 85
   builder minutes under a 120 cap with the run join inside it. Is the
   estimate credible against the recorded precedent the design cites, and
   does slice 1 alone still refuse all three specimens as replayed? Slice 2
   now carries the dependency on supervision-hook-wrong-root; confirm slice
   1 has none.
4. NEW FINDINGS: anything revision 2 introduced (the Mutate reorder, the
   holder conjunct, the fenced-claim exclusion, the relay-provenance fields
   in section 5.2) that opens a fresh escape or a fresh refusal loop.

Findings material and grounded, quoting the disagreeing text or code. Your
sandbox is read-only: verify by reading, do not run go. The three GAP rows in
section 9 are declared residuals, not findings, unless one of them hides an
escape. Zero material findings is an acceptable, closing answer if that is
what the reading supports.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.

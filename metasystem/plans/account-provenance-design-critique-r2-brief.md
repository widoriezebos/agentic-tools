Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal account-provenance)
Date: 2026-09-06

# Design critique, round 2: the paying account joins the run record

FINDING IDS: chain-unique, continue the series: account-provenance-r2-<slug>, never F-n.

The design under review is metasystem/plans/account-provenance-design.md,
revision 2, landed on main as commit 946a4fd0; its SHA-256 is
d09f24f488872c74f820b872fe6e2277a9ce0cb1ac1dcff8deaaf2c3b7d8344b. The
"Declared Outputs" digest line the dispatcher stamps into your prompt is
the digest of the outputs manifest, not of the design; do not stop on
that difference. The round-1 register is
metasystem/records/misc/account-provenance-critique-r1.md (eight material
findings, all accepted); revision 2 folds each by id and ends with a
fold table. The design brief is
metasystem/plans/account-provenance-design-brief.md and the goal record
is metasystem/plans/goals/account-provenance.md. Wido's binding word
stands: the string "Wido@M0" is hand-written into m0's landing messages
until this lands.

The design's tree references (file and line) were read on 2026-09-02
and main has moved since. Judge each seam by name, not by line: a moved
line is not a finding; a seam that no longer exists, changed shape, or
no longer does what the design relies on is.

Round budget: this is the closing review of the design ladder. The
question is convergence: would an implementer working from revision 2
build the right thing, with the record contract, the capture bound, the
per-runtime grades and the retirement rule as stated? Material only if
the answer is no and you name the artifact it changes; polish is not
material.

# Mandate

1. Each round-1 fold: sufficient, or does it move the problem? Rule per
   finding id, briefly; reopen with a new id only where the fold fails.
2. The record contract: is the closed six-key object, with
   attested-requires-identity, unattested-carries-none, cause-token
   errors and the engine-stamped timestamp, enough to keep credential
   material out of every record, and does one validator serve both
   surfaces?
3. The capture bound: does the bounded executor path as described leave
   no way for capture to gate `metasystem up` or a dispatch, including a
   hung devin surface and an adapter that spawns children?
4. The retirement rule: can the landing evaluator resolve the calling
   main and its job records as described, and can the stamp retire
   before the records hold usable provenance?
5. The build box: the goal's budget is four hours, six attempts, three
   review rounds. Is the implementation as scoped (one leaf package, two
   seam verbs, four adapter verbs, two writers, one observation field,
   one printed line, and the named tests) buildable in one implementer
   chain inside that box, or must the design name a first slice?

If revision 2 converges, say so with zero material findings and the
build is dispatched from it.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema; the
declared outputs manifest names the one record you write.

# Gap Rule

stop and report a gap; never fill it silently.

Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-06

# Design critique, round 3 (closing): one word stops the metasystem

FINDING IDS: chain-unique, STOP-R3-001, STOP-R3-002, ... never F-n.

The design under review is metasystem/plans/metasystem-stop-verb-design.md,
revision 3, landed on main as commit bed2eb02; its SHA-256 is
47d0e6731446e47135b2440e3c0d80a4f2281e7d6bb1b1e8b9f04b6e3498a72d. It
supersedes metasystem/plans/metasystem-stop-design.md (revisions 1 and 2,
frozen). The "Declared Outputs" digest line the dispatcher stamps into
your prompt is the digest of the outputs manifest, not of the design; do
not stop on that difference. The round-2 register is
metasystem/records/misc/metasystem-stop-critique-r2.md (eight material
findings, all accepted, with the coordinator's dispositions); revision 3
folds each by id and opens with a table saying where. The goal record is
metasystem/plans/goals/metasystem-stop-verb.md; Wido's word in it is the
acceptance test: one intuitive word, omnipotent over what the metasystem
runs, honest about what it could not stop.

The design's tree references were read on 2026-09-06 by seam name. Judge
each seam by name, not by line; a seam that no longer exists, changed
shape, or does not do what the design relies on is a finding.

Round budget: this is the closing review of the design ladder, and the
build proceeds from revision 3 whatever you find (implementation-first
ruling: any remaining finding is carried as a build finding, not a fourth
design round). So the question is convergence: would an implementer
working from revision 3 build the right thing? Material only if the
answer is no and you name the artifact it changes; polish is not material.

# Mandate

1. Each round-2 fold, by id STOP-R2-001 to STOP-R2-008: resolved, or does
   the fold move the problem? Rule per id, briefly; reopen with a new id
   only where the fold fails.
2. The generation handshake (section 2): is the completeness argument
   sound against the real creation paths (run launch, dispatch setup,
   proof-run launch, the mission launcher and loop, the steward runner,
   the owner launch), and are the package tests it names deterministic
   proofs rather than timing tests?
3. The order of stopping (section 4) after the reorder: does stopping
   suites before runs, and the owner deadline derived from the teardown
   contract, hold against the code as it is?
4. The withdrawn fleet promise (section 8): is the directory-gone branch
   now identity-safe and honest, and does the fleet fixture prove the
   human session survives?
5. The build box: the goal's budget is one day, ten attempts, 1200
   reserved job minutes, three review rounds. Is the build as scoped in
   section 12 buildable in one implementer chain inside that box, or must
   the design name a first slice? Say which.

If revision 3 converges, say so with zero material findings and the build
is dispatched from it.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema;
write your register to the record named by the declared outputs
manifest, in the shape of the round-2 record.

# Gap Rule

stop and report a gap; never fill it silently.

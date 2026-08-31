# Two-bars landing-provenance — Sol critique round 1, with seat dispositions

Critic: codex gpt-5.6-sol (job two-bars-critique-r1), design under attack:
plans/two-bars-for-changes-design.md sha256 48e1042d (landed a2029456).
25 findings, 19 critical, all declared material. Seat dispositions follow
each finding; every ACCEPTED finding binds the revision round (R-25b: the
designer folds, the seat never edits the design).

## TB-LP-R1-01 [critical]

The claimed commit chokepoint fails open. If the installed engine is absent, non-executable, or returns a classification error, the pre-commit guard deliberately leaves a raw agent commit untouched. An honest agent using git commit and then git push can therefore land bytes with neither provenance bar.

Evidence: Section 0 and section 1.5 claim every agent commit must pass the wrapper at metasystem/plans/two-bars-for-changes-design.md:32-35 and :82-87. The live guard runs classification only when the engine is executable and explicitly passes through an unreadable classification at metasystem/scripts/agents/pre-commit-guard.sh:28-50. This also contradicts the carried raw-commit refusal at metasystem/records/two-bars/two-bars-design.md:24-31.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-02 [critical]

Every direct-fix class can escape its checked path rule during the mandatory rebase. A lagging machine checks the floor against its old HEAD and index, then fetches a tip that adds a floor entry or renames an allowed path onto a protected path. Git rebase can apply the direct commit at that new path, and the design deliberately performs no direct-fix recheck before pushing.

Evidence: The floor is evaluated before commit at metasystem/plans/two-bars-for-changes-design.md:191-204, while the design asserts that rebase cannot add paths and exempts all direct fixes from rechecking at :184-187. The real order commits before fetch and rebase at metasystem/scripts/agents/land.sh:267-272 and repeats rebases at :287-290. This invalidates prose-docs, mechanical-defect, and the content-sensitive revert-exact class under a moving fleet tip.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-03 [critical]

Bar (a) is not enforced for two entry points the design expressly claims to cover. A commit made through commit.sh --push can fail its push, be rebased later, and be pushed raw without a blob recheck; a plain wrapper commit can likewise be rebased before its later push. The pushed tree can therefore differ from the reviewed tree.

Evidence: Section 8 claims coverage of direct commit.sh --push and commit-then-push-later at metasystem/plans/two-bars-for-changes-design.md:303-309, but section 3 assigns the post-rebase check only to land.sh at :172-184. The live direct-push path neither fetches nor rebases and tells the caller to resolve and push after rejection at metasystem/scripts/agents/commit.sh:275-295. There is no push-boundary check of the outgoing commit range.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-04 [critical]

A closed chain can be rebound to unreviewed bytes by editing its machine-local review.json and diff.patch after closure. Landing-check trusts those mutable files without comparing them to the durable mirror manifest, so changing reviewedTree to the staged tree and changing the path set makes arbitrary staged bytes appear reviewed.

Evidence: Section 3 resolves both authorities from local round files at metasystem/plans/two-bars-for-changes-design.md:145-161. Close-check verifies only the current diff.patch digest at metasystem/internal/dispatch/close.go:57-83 and performs no post-close seal over review.json. The design's claim that host authorship makes the files authoritative appears at metasystem/plans/two-bars-for-changes-design.md:145-152, but both remain same-user mutable artifacts after chainClosed is written.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-05 [critical]

chainClosed does not prove an acceptable independent critique outcome. A critic can complete with material findings, and the reviewed chain can still close because hazard closure checks identity, role, freshness, and effort but never reads the critic return, its material count, or the critic chain's closure. Those rejected bytes can then satisfy bar (a).

Evidence: Section 3 relies wholly on closure as proof of critique duties at metasystem/plans/two-bars-for-changes-design.md:139-144. The actual validator at metasystem/internal/dispatch/hazard.go:261-301 never opens return.json or checks findings, verdictMaterialCount, or chainClosed. Meanwhile review.json is written before merge acceptance at metasystem/internal/validate/conformance.go:297-303 and :439-451, so its existence does not repair this omission.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-06 [critical]

The carriage exemption is an unexamined-byte lane for design and governance content. plans/*.md includes top-level design documents such as this design itself, while memory/rulings.md can have an existing ruling rewritten and narrator-digest.log contains rewriteable pending narration. Any such bytes may ride an unrelated chain or a register-carriage commit without review or class constraint.

Evidence: Section 5 exempts plans/*.md, memory/rulings.md, and the narrator digest from both bars and content verification at metasystem/plans/two-bars-for-changes-design.md:238-256. The existing ruling check permits remove-and-add rewrites retaining the same identifier at metasystem/scripts/agents/land.sh:145-173. The seat record states that pending narration is rewriteable repository content at metasystem/plans/seat-governance-record.md:123-132.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-07 [critical]

A wrong direct-fix declaration remains a complete semantic bypass. mechanical-defect accepts every non-floor path and prose-docs accepts every non-floor Markdown file, so a design or policy change outside the incomplete floor lands merely by naming it a defect. Visibility after the push does not stop the accidental misclassification the carried threat model required the mechanism to stop.

Evidence: The permissive class rules and acknowledged laundering surface are at metasystem/plans/two-bars-for-changes-design.md:206-220 and :232-236. The predecessor defines the accidental model as stopping an honest agent from misclassifying or forgetting at metasystem/records/two-bars/two-bars-design.md:78-89. Its concrete schema-producer escape remains documented at metasystem/records/two-bars/two-bars-critique-r1.md:3-11.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-08 [critical]

A wrong chain hazard declaration also bypasses examination. A design change declared MECHANICAL has no independent-critique duty; a short Markdown design change can additionally use the existing prose-under-30 conformance waiver. The resulting chain can close and satisfy bar (a), despite containing no independent examination.

Evidence: Bar (a) trusts the chain's declared hazard class at metasystem/plans/two-bars-for-changes-design.md:139-144. MECHANICAL explicitly requires no critique at metasystem/internal/dispatch/hazard.go:36-45 and returns early without one at :193-198. Non-mission Markdown changes can waive critique at metasystem/internal/validate/conformance.go:532-559 and :574-600.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-09 [critical]

The never-direct-fix enforcement surface omits code that controls the check itself. cmd/metasystem routes and parses the new verb, internal/lease decides the human exemption, internal/config reads enforcement mode, and internal/gittree would perform tree comparisons, yet none is on the explicit floor. A mechanical-defect commit can change those dependencies and disable or redirect future checks; if the candidate-built engine is used, it can judge that same commit.

Evidence: The floor enumeration is at metasystem/plans/two-bars-for-changes-design.md:191-204. Current job verbs are routed outside internal/dispatch at metasystem/cmd/metasystem/main.go:92-140 and parsed in metasystem/cmd/metasystem/dispatch_verbs.go:1341-1352. commit.sh builds and uses policy code from prospective bytes at metasystem/scripts/agents/commit.sh:111-128 and :223-231.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-10 [critical]

The design leaves the post-rebase checker executable undefined. The commit-time proof engine is temporary and deleted when commit.sh exits, while bin/metasystem is a gitignored installed binary that a source rebase does not update. Calling the installed engine can omit the new verb or use stale rules; rebuilding from rebased candidate bytes lets the enforcement change judge itself. These choices produce different safety and liveness outcomes.

Evidence: Section 8 says only that land.sh invokes an engine verb after rebase at metasystem/plans/two-bars-for-changes-design.md:310-315. The current proof engine is a temporary file with an exit cleanup at metasystem/scripts/agents/commit.sh:111-128, while land.sh currently has no engine selection at metasystem/scripts/agents/land.sh:244-290. The design never fixes which immutable engine identity owns the recheck.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-11 [critical]

The human-word requirement for class changes and observe-to-enforce promotion is prose only. Bar (a) checks closure and bytes but contains no authorization join, so the seat can dispatch a chain, change the manifest or enforcement key, close it, and land it without a recorded human authorization.

Evidence: The human-word requirements are stated at metasystem/plans/two-bars-for-changes-design.md:224-230 and :321-347, but the exhaustive bar (a) check list at :139-170 contains no human-authorization field or lookup. This conflicts with Law 2's requirement that promotion authorization be checked at the base action boundary at metasystem/plans/seat-governance-record.md:191-195.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-12 [critical]

The bootstrap and fleet cutover have an ungoverned landing. The commit that introduces the mechanism is executed by the old wrapper and therefore cannot check or stamp itself. Later, a lagging machine can start the old observe-mode land.sh, commit unproven bytes, fetch and rebase onto the enforce flip, and continue in the already-loaded old shell directly to push. The checkout contains enforce at push time, but that landing never ran enforcement.

Evidence: The rollout says to land the mechanism in observe and claims each machine picks it up at its next fetch-rebase cycle at metasystem/plans/two-bars-for-changes-design.md:321-344. The actual process commits before that fetch-rebase and then pushes without restarting at metasystem/scripts/agents/land.sh:267-290. No bootstrap exception or first-post-update fence is specified.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-13 [critical]

One-landing-per-chain is neither implementable through the cited record operation nor atomic with the push. The current terminal-record schema rejects landedCommit, the compare-and-swap compares only status rather than absence of landedCommit, and a crash after remote push but before the local patch leaves the chain reusable. Thus replay is not impossible by construction.

Evidence: The design performs the patch only after push at metasystem/plans/two-bars-for-changes-design.md:165-170. Current terminal metadata permits only mirror, closure, usage, and runnerClosed at metasystem/internal/dispatch/record.go:89-95, so landedCommit is refused at :530-535. RecordCAS compares status and then overwrites patch fields at :475-540; it cannot make a local post-push write atomic with origin.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-14 [critical]

A lawful closed-chain landing can become impossible because reviewedTree is not pinned by a durable Git reference. Conformance creates it with write-tree, and sharing an object database only makes it temporarily readable. After the disposable delegate worktree and branch are removed and Git garbage collection runs, landing-check cannot resolve the tree even though the mirrored JSON and patch survive.

Evidence: The design's entire reachability argument is object-database sharing at metasystem/plans/two-bars-for-changes-design.md:145-152. Snapshot returns an unreferenced write-tree result at metasystem/internal/gittree/gittree.go:246-276. The repository lifecycle explicitly calls job worktrees disposable and local artifacts safe to wipe at metasystem/plans/README.md:3-5. No design step creates a ref or mirrors the object database.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-15 [critical]

Section 10 silently drops the carried direct-fix evidence bar. The predecessor retained a candidate-tree-bound full-validator success witness, a local nonce with consume-on-use lifecycle, and one red-then-green assertion against immutable baseline and candidate trees. The replacement closure artifacts apply only to bar (a); bar (b) has no proof field or witness at all.

Evidence: The retained decisions are explicit at metasystem/records/two-bars/two-bars-design.md:38-49 and detailed at :163-178. The new direct-fix table requires no evidence except defect text or revert identifier at metasystem/plans/two-bars-for-changes-design.md:206-220. Section 10 nevertheless describes witness machinery as superseded wholesale at :395-400. This is an R-25b-m1 weakening, not a disposition of the direct-fix half.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-16 [critical]

Section 10 silently drops final-message hook composition. The proposed argument scan cannot ensure exactly one provenance trailer because land.sh supplies -F followed by a filename, so commit.sh scans the filename rather than the file contents; later message-producing options and hooks are likewise outside that scan. An author-supplied trailer can coexist with the machine trailer and corrupt the audit claim.

Evidence: The design promises exactly one trailer using the current argument-scan pattern at metasystem/plans/two-bars-for-changes-design.md:103-120. The live scan examines only commit argument strings at metasystem/scripts/agents/commit.sh:60-70, while land.sh always passes -F and a path at metasystem/scripts/agents/land.sh:226-232. The predecessor expressly retained composed pre-commit and commit-msg enforcement at metasystem/records/two-bars/two-bars-design.md:24-31 and :113-130.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-17 [critical]

Section 10 silently drops the durable audit join. A bare reviewed tree hash is not made reachable, root job records do not contain reviewedTree, peer records are machine-local, and the new global rule receives no instruction-ledger entry. Consequently the proposed report cannot perform its stated tree-versus-record comparison or reconstruct provenance after local evidence cleanup.

Evidence: The predecessor requires content-bound reachable references, critique closure, an instruction-ledger entry, and a defined history audit at metasystem/records/two-bars/two-bars-design.md:180-193. The new audit claim is at metasystem/plans/two-bars-for-changes-design.md:283-299, but review.json—not the root record—contains reviewedTree at metasystem/internal/validate/conformance.go:439-443. Local artifacts are gitignored and disposable under metasystem/plans/README.md:3-5.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-18 [critical]

The predecessor's implementation-readiness gate is silently absent. It prohibited building before the trailer grammar, manifest path and schema, failure behavior, proof schemas, audit-chain schema, hook lifecycle, subsystem mapping, and a design-obligation matrix were settled. The new design supplies no obligation matrix or proof section and leaves several of those contracts unnamed.

Evidence: The predecessor's binding five-step order is at metasystem/records/two-bars/two-bars-design.md:50-65 and again at :216-232. Section 10 claims complete disposition at metasystem/plans/two-bars-for-changes-design.md:379-410 but does not mention the obligation matrix, schema set, or build gate. An implementer would have to invent them, violating R-25b-m1.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-19 [critical]

The design does not remedy the seat-governance record's second open item. It preserves content-unverified narrator carriage in the same actor that performs acceptance, so the narrator-plus-accepting-custodian combination remains conduct-only. Against the stated review scope, one of the two required governance separations is knowingly left open.

Evidence: The second open item and prohibited combination are defined at metasystem/plans/seat-governance-record.md:113-148. The design explicitly says its unverified carriage is that open item's surface and deliberately does not narrow it at metasystem/plans/two-bars-for-changes-design.md:442-449. Implementation of this design therefore cannot close both open items.

DISPOSITION: NOTED (scope). The stated facts are accurate and verified, but the goal charters the landing-provenance gap (the governance record's FIRST open item); the second item — narrator plus acceptor in one actor — explicitly awaits Wido's choice at the 2026-11-30 review (R-30-m1). The revision must state this boundary explicitly: what two-bars deliberately does not remedy, and where that debt is tracked.

## TB-LP-R1-20 [high]

The fleet-wide integrity report cannot work from machine-local evidence. A report on one machine scans the shared origin history but lacks other machines' root records and review artifacts, so it reports every legitimate peer chain trailer as missing or cannot verify it at all.

Evidence: The report promises to cross-check every chain trailer against its root record at metasystem/plans/two-bars-for-changes-design.md:283-297. Migration simultaneously says all checks use machine-local state at :337-344, and bar (a) locates records in this machine's artifacts directory at :139-152. No evidence replication or machine-qualified lookup is specified.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-21 [high]

The direct-fix policy files have no implementable contract. The class manifest has no path, encoding, duplicate rule, version, or malformed-input behavior; the carriage allowlist mixes apparent exact paths with plans/*.md without defining wildcard semantics; and evaluating runtime-declared instruction files from both historical trees has no specified authority or failure behavior.

Evidence: The generating rule and floor are stated at metasystem/plans/two-bars-for-changes-design.md:191-207, and the wildcard allowlist at :238-251. The predecessor specifically required manifest path, schema, failure behavior, and initial protected set before implementation at metasystem/records/two-bars/two-bars-design.md:50-58. Different reasonable implementations would protect different paths or fail open differently.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-22 [high]

chainPaths extraction is underspecified and unsafe for valid Git filenames. The design says a binary diff.patch yields the authoritative path set but defines no parser or null-delimited path representation, and review.json records no boundary-base tree from which ChangedPaths could instead be recomputed. Quoted paths, tabs, newlines, and deletions therefore require implementer judgment.

Evidence: The path-set rule is at metasystem/plans/two-bars-for-changes-design.md:145-164. The actual review object contains only diffArtifact, implementerJob, and reviewedTree at metasystem/internal/validate/conformance.go:439-443. The canonical safe path primitive uses a null-delimited git diff at metasystem/internal/gittree/gittree.go:329-343, while diff.patch is a textual binary patch at :314-326.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-23 [high]

The trailer and revert-exact grammars leave outcome-changing choices open. The design does not define escaping or rejection for quotes, control characters, or embedded trailer syntax in defect text, and revert-of may name a reachable merge commit whose inverse is ambiguous without selecting a parent. Implementers can produce incompatible parsers or different reverted trees.

Evidence: The nominal grammar and one-line field are at metasystem/plans/two-bars-for-changes-design.md:101-120 and :209-214. The named commit is restricted only by reachability, not by parent count. No canonical byte encoding, length bound, parent rule, or malformed-field outcome is stated.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-24 [high]

The claimed replacement for the deferred defect-identity growth fuse does not exist. The design says severity-tiered-rigor already consumes repeated mechanical-defect clusters, but that design explicitly says the near-miss register does not exist and recurrence promotion is a later slice. The widest laundering class therefore ships without either the carried fuse or its asserted substitute.

Evidence: The replacement claim appears at metasystem/plans/two-bars-for-changes-design.md:216-222, :294-297, and :401-409. The cited owner says the near-miss register does not exist and slice one must not depend on it at metasystem/plans/severity-tiered-rigor-design.md:16-20. The predecessor retained the defect-identity fuse at metasystem/records/two-bars/two-bars-design.md:38-46 and :152-161.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

## TB-LP-R1-25 [high]

The observe window cannot prove that observed landings were clean. Observe mode stamps the same trailer whenever flags are present even when the check would refuse, while the failure exists only as a console line. The history-based report therefore cannot distinguish a passing observed shape from a failing one, yet that report is declared to be promotion evidence.

Evidence: Observe behavior and the report-based promotion criterion are at metasystem/plans/two-bars-for-changes-design.md:321-339. The only durable audit fields are the ordinary provenance and Machine trailers described at :101-124 and :283-293; no pass/fail or observe-verdict field is recorded.

DISPOSITION: ACCEPTED. Folds into revision round 2; spot-verified where cited (the seat checked TB-LP-R1-01's fail-open guard text and TB-LP-R1-15's predecessor evidence bar directly against the tree).

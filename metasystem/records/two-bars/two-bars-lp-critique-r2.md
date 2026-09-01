# Two-bars landing-provenance — Sol critique round 2, with seat dispositions

Critic: codex gpt-5.6-sol (job two-bars-critique-r2b) against design r2
(sha256 db65bedf, landed e0bd071c). 14 findings (11 critical, 3 high),
all material — down from round 1's 25. Every finding ACCEPTED and binding
on fold round 3; the seat spot-checked TB-LP-R2B-01 (the obligation-table
header mismatch against internal/validate/designobligations.go:18-23) and
TB-LP-R2B-02 (the terminal-field allowlist at internal/dispatch/record.go:89-95)
directly against the tree.

## TB-LP-R2B-01 [critical]

Section 11's claimed restored implementation-readiness gate is not an executable obligation matrix. The document declares that implementation is refused until the matrix passes, but its four-column table is not the canonical table the named validator recognizes; an implementer must either ignore the gate or invent a replacement. This leaves round-one finding TB-LP-R1-18 standing at critical severity. Remedy: express the obligations in the validator's canonical columns and require the actual gate command to pass before implementation.

Evidence: The design calls the table the build gate at metasystem/plans/two-bars-for-changes-design.md:785-820. The validator recognizes only a header beginning `Obligation id | Severity` and ending `Status | Next action` at metasystem/internal/validate/designobligations.go:18-23, whereas the design uses `# | Obligation | Owning subsystem | Verified by` at metasystem/plans/two-bars-for-changes-design.md:791-812. Running the exact named validator returned `no design-obligation rows found` and exit code 1.

DISPOSITION: ACCEPTED. Folds into revision round 3.

## TB-LP-R2B-02 [critical]

Section 3.3's close-time seal remains generically rewritable after closure. The design tells the implementer to add reviewedTree, baseTree, and the patch digest to terminalMetadataFields while asserting that only close writes them, but terminalMetadataFields is precisely the allowlist that generic same-status record patches may modify. Editing the record and mirrored evidence can therefore rebind the alleged seal, so round-one finding TB-LP-R1-04 is not folded. Remedy: give the sealed fields a close-owned write-once operation and refuse subsequent updates.

Evidence: The proposed schema operation and seal claim are at metasystem/plans/two-bars-for-changes-design.md:205-219. Live terminal fields are an allowlist at metasystem/internal/dispatch/record.go:89-95; RecordCAS treats a same-status operation as metadata update and accepts every terminal field in that map at metasystem/internal/dispatch/record.go:467-540. The code already has a separate dedicatedMetadataFields ownership mechanism at metasystem/internal/dispatch/record.go:77-87, but the design does not assign the seal to it.

DISPOSITION: ACCEPTED. Folds into revision round 3.

## TB-LP-R2B-03 [critical]

Section 3.2 contradicts Bar (a) and the charter by allowing an uncritiqued chain to land any non-floor path. A wrongly declared MECHANICAL chain can therefore carry a design change outside the conservative floor without the adversarial critique promised on the one-page rule. This is the original TB-LP-R1-08 bypass with a smaller path set, not a genuine fold. Remedy: require a completed clean independent critique for every Bar (a) landing.

Evidence: Section 0 promises a closed chain whose closure included an independent critique at metasystem/plans/two-bars-for-changes-design.md:18-24, but section 3 explicitly allows a chain without any completed critique to land when its paths miss the floor at metasystem/plans/two-bars-for-changes-design.md:189-204. Live MECHANICAL configuration requires no critique and returns early at metasystem/internal/dispatch/hazard.go:36-41 and :193-198. The goal says design changes take the loop at metasystem/plans/goals/two-bars-for-changes.md:4.

DISPOSITION: ACCEPTED. Folds into revision round 3.

## TB-LP-R2B-04 [critical]

Section 4 still provides a complete honest-misclassification route through prose-docs. Any Markdown design or policy outside plans/ and the enumerated floor needs neither critique nor proof; acknowledging that path checks are nonsemantic does not satisfy the predecessor's requirement to stop an honest agent from misclassifying. Round-one finding TB-LP-R1-07 therefore remains open. Remedy: remove semantically unconstrained documents from direct-fix eligibility or give them a mechanically decisive direct-fix contract.

Evidence: The prose-docs rule admits every non-floor `.md` path outside plans/ and requires no evidence at metasystem/plans/two-bars-for-changes-design.md:330-344 and :375-379. The design concedes that its classes cannot distinguish a design change from a mechanical one at metasystem/plans/two-bars-for-changes-design.md:410-416. The carried accidental model expressly includes stopping honest MISCLASSIFICATION at metasystem/records/two-bars/two-bars-design.md:78-89.

DISPOSITION: ACCEPTED. Folds into revision round 3.

## TB-LP-R2B-05 [critical]

Sections 2, 4, and 7 do not actually restore the carried direct-fix evidence bar. The trailer contains no proof identifier; the local proof record contains no whole-validator identity, final zero outcome, command/version, or evidence hashes; and the history report has no way to join a direct-fix commit to durable proof. This silently weakens the predecessor again, leaving TB-LP-R1-15 and TB-LP-R1-17 open and violating R-25b-m1. Remedy: restore one content-bound reachable proof/witness reference carrying the retained full-validator and red-green evidence fields.

Evidence: The complete trailer grammar lacks a proof reference at metasystem/plans/two-bars-for-changes-design.md:129-147. The proposed local record has only defectId, trees, assertion fields, outcomes, nonce, and consumedBy at metasystem/plans/two-bars-for-changes-design.md:354-374. The report emits only commit, class, defect fields, and subject at metasystem/plans/two-bars-for-changes-design.md:497-520. The binding predecessor retains candidate-tree witness binding, whole-validator zero-outcome identity, evidence hashes, and content-bound reachable Defect-Proof references at metasystem/records/two-bars/two-bars-design.md:38-46 and :163-185; R-25b-m1 forbids weakening at metasystem/memory/rulings.md:52.

DISPOSITION: ACCEPTED. Folds into revision round 3.

## TB-LP-R2B-06 [critical]

Section 8's installed push engine can silently pass stale rules after the first release. Verb ignorance detects only a binary predating landing-check; a later gitignored installed binary can know the verb while implementing an older floor, manifest, proof schema, or governance rule. This is exactly the unresolved half of round-one finding TB-LP-R1-10. Remedy: bind push evaluation to a checked engine or policy-surface version and fail closed on any version mismatch.

Evidence: The design fixes push identity to installed bin/metasystem and claims it can never silently pass old rules at metasystem/plans/two-bars-for-changes-design.md:544-560. The installed binary is gitignored at metasystem/.gitignore:2. By contrast, the live commit boundary deliberately builds prospective policy into a temporary engine at metasystem/scripts/agents/commit.sh:111-128. No version or behavior-surface handshake is specified for the installed push engine.

DISPOSITION: ACCEPTED. Folds into revision round 3.

## TB-LP-R2B-07 [critical]

Sections 2 and 8 do not close the hook lifecycle or fleet-cutover hole. A rebase updates checked-out hook source but does not install it in Git's hooks directory, so a machine that has not run metasystem up can push through the claimed next-landing cutoff with no pre-push hook. Composition order is also unset, allowing an existing commit-msg hook to modify the message after provenance validation. TB-LP-R1-12 and TB-LP-R1-16 remain materially open. Remedy: make successfully probed hook arming a promotion prerequisite and specify core.hooksPath, post-git-init, and final-validator ordering.

Evidence: The design promises composed installation at metasystem/plans/two-bars-for-changes-design.md:149-162, but migration says a rebase alone delivers the flip before push while separately admitting that installation occurs at metasystem up at metasystem/plans/two-bars-for-changes-design.md:601-621. Live enrollment resolves a separate Git hooks directory through `git rev-parse --git-path hooks` at metasystem/cmd/metasystem/goalsync_verbs.go:67-80. The carried lifecycle expressly includes core.hooksPath and post-adoption git init at metasystem/records/two-bars/two-bars-design.md:24-31. The obligation table tests only a generic pre-existing-hook case at metasystem/plans/two-bars-for-changes-design.md:794 and :811.

DISPOSITION: ACCEPTED. Folds into revision round 3.

## TB-LP-R2B-08 [critical]

Section 8 cannot make every observe-mode push verdict durable in the commit trailer. The verdict is minted when the commit is created, but the post-rebase push check happens later; a pre-push hook that may not refuse in OBSERVE cannot rewrite an already-created commit, so a new push-time failure can reach origin carrying the earlier `pass` verdict. Final-message validation also defines no exact cardinality or token equality for the verdict trailer. TB-LP-R1-25 remains open. Remedy: define a durable push-time observation record and exact verdict cardinality instead of treating the pre-push result as mutable commit text.

Evidence: The commit boundary mints the message before the push boundary at metasystem/plans/two-bars-for-changes-design.md:532-552. Observe claims that every result is stamped into history at metasystem/plans/two-bars-for-changes-design.md:579-588, while DRAFT and OBSERVE cannot refuse at :568-577. The live sequence creates the commit at metasystem/scripts/agents/commit.sh:263 and invokes push only at :278-285. The final-message rule validates exactly one Landing-Provenance trailer but does not impose the corresponding verdict rule at metasystem/plans/two-bars-for-changes-design.md:149-162.

DISPOSITION: ACCEPTED. Folds into revision round 3.

## TB-LP-R2B-09 [critical]

Section 0 says the push boundary reruns every check, but section 8's detailed evaluator reruns only trailer shape, Bar (a) blobs, and Bar (b) class/floor checks. It omits the direct-fix proof, growth fuse, manifest authorization, and rulings add-only check. Rechecking the original proof would also fail after ordinary rebases because it is bound to the old absolute candidate tree, yet no rebased-proof rule is supplied. Implementers can therefore build materially different push gates, leaving TB-LP-R1-02, TB-LP-R1-03, and TB-LP-R1-15 incomplete. Remedy: specify one exhaustive push evaluator and an explicit proof rebinding or refusal rule.

Evidence: The one-page claim that every check is rerun is at metasystem/plans/two-bars-for-changes-design.md:38-46. The detailed push checklist at metasystem/plans/two-bars-for-changes-design.md:544-552 omits mechanisms defined at :354-408. The proof requires candidateTree to equal the proved pre-rebase index tree at metasystem/plans/two-bars-for-changes-design.md:360-374, while the design acknowledges rebasing can change the outgoing tree at :260-282.

DISPOSITION: ACCEPTED. Folds into revision round 3.

## TB-LP-R2B-10 [critical]

Sections 3.7 and 4 evaluate history-shaped invariants one outgoing commit at a time against origin, not against the outgoing set as a whole. Two outgoing commits can therefore cite one chain or split one defect identity while neither is yet origin-reachable; each individual check can pass and one push can violate one-landing-per-chain or undercount the fuse. TB-LP-R1-13 and TB-LP-R1-24 are not sufficient. Remedy: join chain identities and defect aggregates across origin plus the complete outgoing range before accepting any ref update.

Evidence: One-landing scans only commits reachable from fetched origin at metasystem/plans/two-bars-for-changes-design.md:243-253. The fuse likewise names origin history plus the staged change at metasystem/plans/two-bars-for-changes-design.md:381-395. The pre-push contract loops over each outgoing commit independently at metasystem/plans/two-bars-for-changes-design.md:544-552. No whole-range uniqueness or aggregation step exists, despite the split-commit fixture promised at :799-804.

DISPOSITION: ACCEPTED. Folds into revision round 3.

## TB-LP-R2B-11 [critical]

Section 4's authorizedBy check proves only that some ruling identifier exists, not that the ruling authorizes the affected class or effect. It even reads the candidate tree while appended rulings rows are carriage, so the same landing can supply its own unrelated or newly appended row and collapse the claimed human key into the examination key. TB-LP-R1-11 and the authorization part of TB-LP-R1-21 remain open. Remedy: join to a typed human-authority record that names the exact class mutation and governing effect.

Evidence: The check requires only that authorizedBy names an existing candidate-tree row at metasystem/plans/two-bars-for-changes-design.md:397-408. Rulings rows may be appended through carriage at metasystem/plans/two-bars-for-changes-design.md:426-437. The governing Law 2 requires complete recorded human authorization at the base action boundary at metasystem/plans/seat-governance-record.md:191-195; existence of an arbitrary row does not prove that authorization.

DISPOSITION: ACCEPTED. Folds into revision round 3.

## TB-LP-R2B-12 [high]

Section 8 assumes a freshly fetched remote-tracking tip at every pre-push invocation, but two expressly covered paths do not establish that fact. Direct commit.sh --push and commit-then-push-later can invoke the hook with stale origin tracking, producing the wrong outgoing range and origin trailer scan. An implementer must guess whether to mutate network state in the hook or use Git's supplied remote object identifier. Remedy: define the hook's authoritative remote tip for every ref update and fail closed when it cannot be established.

Evidence: The push design computes `origin/<branch>..<local>` from a freshly fetched tip at metasystem/plans/two-bars-for-changes-design.md:544-552 and claims direct push and later push coverage at :561-566. The live direct-push path performs `git push` without a fetch at metasystem/scripts/agents/commit.sh:278-285. Only land.sh establishes a fetch before its first push at metasystem/scripts/agents/land.sh:244-253 and :267-277.

DISPOSITION: ACCEPTED. Folds into revision round 3.

## TB-LP-R2B-13 [high]

Section 4's revert-exact class can erase legitimate changes made after the named commit. It permits any reachable one-parent commit and forces every affected path back to that commit's parent's exact blob, without requiring the current parent to still equal the named commit's postimage. A mechanically declared revert can therefore clobber later work. Remedy: refuse direct revert unless every affected current-parent entry still matches the named commit's postimage.

Evidence: The grammar permits any non-merge commit reachable from origin at metasystem/plans/two-bars-for-changes-design.md:138-147. The class then requires exact entries from `revert-of^` at metasystem/plans/two-bars-for-changes-design.md:346-352, with no precondition on the reverting commit's parent tree.

DISPOSITION: ACCEPTED. Folds into revision round 3.

## TB-LP-R2B-14 [high]

Sections 3 and 7 require landedCommit to be written after a successful push, but the expressly supported commit-then-push-later path has no post-success owner. A pre-push hook cannot know whether the subsequent push succeeds, while an ordinary later git push returns to no driver that can patch the record. The report will therefore flag a lawful supported landing as an integrity failure. Remedy: derive the landing commit from origin history or restrict bookkeeping to a mechanism with a real post-success owner.

Evidence: The design assigns the write after successful push at metasystem/plans/two-bars-for-changes-design.md:254-258 and requires the report to match it at :501-508. It nevertheless claims commit-then-push-later is covered at :561-566. The live commit wrapper explicitly leaves a failed direct-push commit standing for a later manual push at metasystem/scripts/agents/commit.sh:278-285; the proposed lifecycle contains only a pre-push hook.

DISPOSITION: ACCEPTED. Folds into revision round 3.

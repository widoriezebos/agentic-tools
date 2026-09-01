# Two-bars landing-provenance — Sol critique round 3, with seat dispositions

Critic: codex gpt-5.6-sol (job two-bars-critique-r3) against design r3
(sha256 1fd87d2a, landed 2ac5c546). 12 findings, all critical, all
material. Trajectory across the loop: 25 -> 14 -> 12 — convergence has
stalled. All findings ACCEPTED; the seat raises the loop-shape decision
to Wido rather than dispatching fold round 4 on the same pattern (the
goal-scope-bounds precedent: the design lane cannot see implementer-
private seams from the design; each ping-pong costs an attempt).

## TB-LP-R3-01 [critical]

Section 4 still permits an honestly misclassified design change to use the mechanical-defect lane. Its state proof can assert only that chosen old bytes existed and the new bytes do not, which every small non-floor change can satisfy after calling the old bytes defective. The undefined assertion payload and red/green outcome grammar also force the implementer to invent a command or predicate language. This leaves the accepted wrong-declaration finding standing.

Evidence: The design withdraws prose-docs but expressly routes a documentation defect through a state assertion that checks the presence and absence of selected bytes at metasystem/plans/two-bars-for-changes-design.md:374-389. The proof schema names assertionKind and an untyped assertion without defining either executable grammar or outcome values at metasystem/plans/two-bars-for-changes-design.md:417-436, then admits that mechanical-defect cannot distinguish a design change from a mechanical one at metasystem/plans/two-bars-for-changes-design.md:513-520. The carried accidental model required stopping honest misclassification at metasystem/records/two-bars/two-bars-design.md:78-89, and the original accepted finding identifies this exact bypass at metasystem/records/two-bars/two-bars-lp-critique-r1.md:57-63. Remedy: settle a finite, mechanically decisive assertion contract that mere byte replacement cannot satisfy, or remove this class.

DISPOSITION: ACCEPTED; folding deferred pending Wido's loop-shape decision.

## TB-LP-R3-02 [critical]

Section 8's rebased-proof rule does not prove that the outgoing tree is green. It compares only entries on the old proof's changed paths; an upstream change on any other path can invalidate the test or repository-state assertion while the push still accepts the old green result.

Evidence: The proof's red outcome, green outcome, and gate result are produced against the old baseTree and candidateTree at metasystem/plans/two-bars-for-changes-design.md:417-436. After rebase, the push rule merely compares entries for the old proof path set and never reruns the assertion or gate against the actual outgoing commit at metasystem/plans/two-bars-for-changes-design.md:709-722. The live landing path rebases before push and can repeat that rebase after a race at metasystem/scripts/agents/land.sh:267-290. Remedy: rerun the assertion and gate on the actual post-rebase parent and commit, or require a fresh proof after every tree-changing rebase.

DISPOSITION: ACCEPTED; folding deferred pending Wido's loop-shape decision.

## TB-LP-R3-03 [critical]

The policy-version handshake still fails open on the precise stale-rule case it was accepted to fix. The checker compares compiled and declared integers, but nothing mechanically requires a policy-bearing code change to increment the declared version; forgetting the bump leaves a stale installed engine numerically current and behaviorally obsolete.

Evidence: Section 8 states only the comparison and a prose convention that every rule change bumps enginePolicyVersion at metasystem/plans/two-bars-for-changes-design.md:687-698. Obligation TB-O09 checks manifest schema and malformed handling but has no policy-surface-to-version-bump fixture at metasystem/plans/two-bars-for-changes-design.md:1222. The installed engine remains outside Git at metasystem/.gitignore:2. Remedy: define the complete policy-bearing path set and mechanically refuse any such change whose manifest version does not advance.

DISPOSITION: ACCEPTED; folding deferred pending Wido's loop-shape decision.

## TB-LP-R3-04 [critical]

Section 0's rule R-11 summary is false on the revision's own migration path: an agent can make and raw-push a commit from an enforce-mode checkout whose hooks were never armed, reaching origin with neither bar. This is the honest raw-Git habit in the accidental threat model, not deliberate hook tampering.

Evidence: Section 0 promises that every agent landing presents exactly one bar and that both boundaries fail closed at metasystem/plans/two-bars-for-changes-design.md:17-57. Section 8 then accepts a machine that raw-pushes without running metasystem up after the flip and concedes that it misses post-rebase verification and whole-range joins at metasystem/plans/two-bars-for-changes-design.md:857-866. The current absent-engine guard demonstrates the unarmed behavior by doing nothing when the binary cannot run at metasystem/scripts/agents/pre-commit-guard.sh:28-50. Rule R-11 rejects a design whose simple explanation does not describe its real behavior at metasystem/memory/rulings.md:36. Remedy: make successfully armed commit and push boundaries a mechanical prerequisite for enforce-mode use by every checkout, including post-git-init checkouts.

DISPOSITION: ACCEPTED; folding deferred pending Wido's loop-shape decision.

## TB-LP-R3-05 [critical]

The direct-fix evidence blobs are not actually pinned by the proposed single Git reference. A reference targeting the proof-record blob keeps that one blob alive, but object identifiers written inside arbitrary JSON are not Git reachability edges to the two transcript blobs; deleting the reference at archival also makes the record itself collectible.

Evidence: The design stores redEvidence and greenEvidence as separate blob object identifiers at metasystem/plans/two-bars-for-changes-design.md:428-430, then claims one reference pointing at the record blob pins the record together with both evidence blobs at metasystem/plans/two-bars-for-changes-design.md:438-443. It promises later cat-file reconstruction at metasystem/plans/two-bars-for-changes-design.md:621-637 while also prescribing deletion of that reference during archival. Git traverses reachability through commit, tree, and tag object links, not hexadecimal text inside a blob. Remedy: place the record and transcripts in one reachable Git object graph and define retention that preserves that graph for the promised audit horizon.

DISPOSITION: ACCEPTED; folding deferred pending Wido's loop-shape decision.

## TB-LP-R3-06 [critical]

The class-authorization fold proves neither authorization of the three genesis classes nor authorization of an exact later mutation. The current rulings register contains no required token, the initial-class table supplies no authorizedBy values, the bootstrap bypasses the new checker, and a permanent class-name token would authorize every future path-rule or fuse change to that class.

Evidence: The manifest contract requires authorizedBy at metasystem/plans/two-bars-for-changes-design.md:347-356, but the three initial rows omit its concrete values at metasystem/plans/two-bars-for-changes-design.md:366-372. Future checking requires only a pre-existing row containing landing-class=<class-id> at metasystem/plans/two-bars-for-changes-design.md:484-505, while the bootstrap landing is explicitly ungoverned at metasystem/plans/two-bars-for-changes-design.md:829-836. A repository search found no landing-class token in any actual ruling row. The accepted finding required authorization of the exact class mutation and governing effect at metasystem/records/two-bars/two-bars-lp-critique-r2.md:91-97. Remedy: pre-record human authorization bound to the canonical digest and effect of each initial or modified class row before its landing.

DISPOSITION: ACCEPTED; folding deferred pending Wido's loop-shape decision.

## TB-LP-R3-07 [critical]

The never-direct-fix floor still omits live code that owns the human-authorization boundary. It protects internal/governance, which only defines vocabulary, but not internal/goal or internal/humanauthority, where obligation promotion and proof of enrolled-human authority are actually enforced; a mechanical-defect commit can therefore change the authority gate itself.

Evidence: The floor enumeration includes internal/governance but omits internal/goal and internal/humanauthority at metasystem/plans/two-bars-for-changes-design.md:323-337. The actual base action boundary checks human proof and writes an ENFORCED obligation in metasystem/internal/goal/verbs.go:550-610, using proof code from metasystem/internal/humanauthority/authority.go:107-121 and :416-514. The generic instruction-bearing list does not cover either internal package at metasystem/scripts/agents/instruction-bearing-paths.txt:1-13. Remedy: generate the enforcement floor from every package that reads, authorizes, mutates, or applies provenance state, including these authority owners.

DISPOSITION: ACCEPTED; folding deferred pending Wido's loop-shape decision.

## TB-LP-R3-08 [critical]

The design never names an implementable owner or durable location for the global landing.provenance mode, and it does not join four-machine arming evidence to promotion. An implementer must guess whether this is a goal obligation, configuration, or a new registry; using the cited existing governance type allows human authorization alone to activate enforcement without the promised observe evidence.

Evidence: The push evaluator is told to read committed governance state without a path or schema at metasystem/plans/two-bars-for-changes-design.md:736-739. Promotion cites internal/governance at metasystem/plans/two-bars-for-changes-design.md:788-797, but that package explicitly owns no policy engine at metasystem/internal/governance/types.go:1-3. The existing persistence operation attaches obligations to claimed, budgeted goal records at metasystem/internal/goal/verbs.go:550-608 and checks no hook reports. Section 8 nevertheless calls four per-machine reports a prerequisite at metasystem/plans/two-bars-for-changes-design.md:848-856, while TB-O21 tests only authorization at metasystem/plans/two-bars-for-changes-design.md:1234. Remedy: name one durable global state owner and a transition operation that mechanically consumes the four arming and clean-window records before ENFORCED.

DISPOSITION: ACCEPTED; folding deferred pending Wido's loop-shape decision.

## TB-LP-R3-09 [critical]

The universal bar-(a) critique requirement cannot be attached to the design changes that are the goal's primary loop case. The live reference owner accepts code-critic or warden evidence reviewing an implementer, but rejects design-critic evidence and any reviewed designer; conformance likewise produces reviewedTree only for implementer records. The revision's obligations modify only landing-check, so an implementer following them leaves design-lane output unlandable.

Evidence: Section 3 requires a completed critique and sealed reviewed tree for every bar-(a) landing at metasystem/plans/two-bars-for-changes-design.md:204-252. Live reference binding recognizes only code-critic and warden at metasystem/internal/dispatch/review_reference.go:80-100 and requires the reviewed role to be implementer at metasystem/internal/dispatch/review_reference.go:117-124, even though hazard closure recognizes design-critic at metasystem/internal/dispatch/hazard.go:261-301. Conformance rejects non-implementers and creates reviewedTree only for implementer work at metasystem/internal/validate/conformance.go:119-122 and :299-318. TB-O08 names only landing-check as owner at metasystem/plans/two-bars-for-changes-design.md:1221. Remedy: settle and test the designer-to-design-critic reviewed-tree and reference lifecycle before making bar (a) universal.

DISPOSITION: ACCEPTED; folding deferred pending Wido's loop-shape decision.

## TB-LP-R3-10 [critical]

A clean critique round does not automatically make the cited canonical register clean. The live register resolves a prior finding only when a later return explicitly repeats the same identifier with material=false; a normal zero-finding clean return leaves every accepted prior finding open forever. The design neither requires those resolution rows nor names another disposition-to-register operation.

Evidence: Section 3 relies on job critique-open-finding-ids returning an empty set at metasystem/plans/two-bars-for-changes-design.md:206-224. The live fold logic changes an existing entry to resolved only when the same identifier is present with material=false at metasystem/internal/dispatch/finding_register.go:342-370; absent findings are untouched. The critique skill's stop rule is a round with zero material findings, not mandatory repetition of every historical identifier, at metasystem/skills/design-critique/SKILL.md:26-32. Remedy: define a mechanical resolution join from dispositions and the exact clean follow-up round into the canonical register.

DISPOSITION: ACCEPTED; folding deferred pending Wido's loop-shape decision.

## TB-LP-R3-11 [critical]

The accepted implementation-readiness finding is not folded: the design explicitly proceeds to implementation while its named gate fails. Calling the enumerated refusal an implementation-readiness gate reverses the accepted requirement that the actual gate pass before implementation.

Evidence: The design says both that the command exits 1 for every current critical or high row and that this failure is the readiness gate at metasystem/plans/two-bars-for-changes-design.md:1192-1210, then immediately supplies the implementation build order at metasystem/plans/two-bars-for-changes-design.md:1239-1248. Running both named validator forms returned exit 1 with all 24 rows MISSING. The binding accepted finding requires the actual command to pass before implementation at metasystem/records/two-bars/two-bars-lp-critique-r2.md:11-17. Remedy: reach a truthful passing preimplementation state or record a lawful designer rebuttal to that accepted requirement before starting the build.

DISPOSITION: ACCEPTED; folding deferred pending Wido's loop-shape decision.

## TB-LP-R3-12 [critical]

The observe record cannot establish the clean window claimed for promotion. Its append-only line grammar has no window identity or resolution event, so 'zero unresolved' is undefined after the first would-refuse; and observe-mode missing-engine or classification failures remain fail-open before an evaluator can write any line, making some failures invisible.

Evidence: Observe-mode guards retain fail-open behavior at metasystem/plans/two-bars-for-changes-design.md:582-597. The push log contains only date, commit, boundary, and verdict at metasystem/plans/two-bars-for-changes-design.md:799-816, while promotion requires zero unresolved would-refuse lines without defining resolution or a fresh-window boundary at metasystem/plans/two-bars-for-changes-design.md:818-824. The current guard confirms that an absent engine produces neither refusal nor evidence at metasystem/scripts/agents/pre-commit-guard.sh:28-50. Remedy: define append-only window and resolution records and make every unevaluable observe check emit a durable would-refuse outcome.

DISPOSITION: ACCEPTED; folding deferred pending Wido's loop-shape decision.

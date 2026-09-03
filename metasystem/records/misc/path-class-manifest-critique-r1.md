# Path-class manifest design critique — round 1 (Sol)

Chain: design revision 1 -> critic path-class-crit1 (codex {'effective': 'gpt-5.6-sol', 'requested': 'gpt-5.6-sol'}, reviewed commit 6a247a1aff5408e79254bfe97e56a0f46c377759), 2026-09-03. 13 material findings under the stop criterion: material only if it changes what gets built. Full return: artifacts/agents/path-class-crit1/rounds/1/return.json.

## PCM-R1-001 — high, material=True

CLAIM: Add the manifest row `plans/goals.md ledger` and a compatibility fixture. The design says absent paths are retained when code still names them, but carries only `plans/goals-accepted.json`. Fresh adoption still creates the omitted half of that ledger pair. As written, longest-prefix matching classifies it through `plans/ record`, allowing wrapper carriage and prose-waiver treatment of a file that may be changed only by goal verbs. The build artifacts that change are metasystem/scripts/agents/path-classes.txt and the tracked-path compatibility case in metasystem/internal/pathclass/pathclass_test.go.

EVIDENCE: metasystem/plans/path-class-manifest-design.md:83-97 says `plans/goals-accepted.json` is listed because code names it, then omits `plans/goals.md` from the ledger rows. metasystem/internal/goal/goal.go:122-129 names them as LedgerPath and BaselinePath; metasystem/scripts/adopt.sh:298 and :327-342 creates and preserves the pair; metasystem/internal/goal/migrate.go:283-288 deletes both in the same goal transaction.

## PCM-R1-002 — high, material=True

CLAIM: Give repository-outer keys and installation-relative keys distinct namespaces or another unambiguous scope rule. The single raw key space cannot represent the tracked repository-root metasystem directory and the installation-local built binary: both are `metasystem`, but the proposed exact row calls that key runtime. In adopted mode, template-only outer rows such as `development/ record` also become classifications for an application's own installation-relative paths. The build artifacts that change are the grammar and resolver in metasystem/internal/pathclass/pathclass.go, the affected rows in metasystem/scripts/agents/path-classes.txt, and template-versus-adopted resolution fixtures in metasystem/internal/pathclass/pathclass_test.go.

EVIDENCE: `git ls-tree --name-only HEAD` reports a tracked top-level directory named metasystem. metasystem/plans/path-class-manifest-design.md:66-79 feeds installation-relative and repository-relative keys into the same table, while :98 assigns the exact key `metasystem` to the runtime binary. metasystem/internal/stateroot/owner.go:68-94 shows that template and adopted layouts have different installation geometry.

## PCM-R1-003 — high, material=True

CLAIM: Override the behavior-bearing files inside the repository-root development directory instead of classifying all of development as record. At minimum, development/README.md describes the maintained tree, development/project-rules-local.md contains this repository's operating invariants, and development/devin-selftest.md is an executable operator runbook. Register carriage must not be allowed to revise those instructions without a reviewed chain. The build artifact that changes is the corresponding exact behavior rows in metasystem/scripts/agents/path-classes.txt, with assertions in metasystem/internal/pathclass/pathclass_test.go.

EVIDENCE: metasystem/plans/path-class-manifest-design.md:87-95 defines instructions, documentation, and harness configuration as behavior but assigns the whole root-level development directory to record. The root-level development/project-rules-local.md:1-9 calls itself this project's filled-in rules and states commit invariants. The root-level development/devin-selftest.md:3-6 calls itself the reproduction procedure, and :17-84 supplies operational commands and refusal recovery.

## PCM-R1-004 — high, material=True

CLAIM: Do not allow exact-revert to bypass record ownership and append-only rules. The proposed `record` exact-revert outcome can delete an appended ruling, receipt, or newly created history file, contradicting both the goal's register-carriage-only contract and the repository's append-only record contract. The build artifacts that change are metasystem/internal/landing/observe.go's exactRevertClassError outcome for record paths and TestObserveExactRevertRefusesByClass in metasystem/internal/landing/observe_test.go, which must cover all five class outcomes rather than asserting that a record inverse passes.

EVIDENCE: metasystem/plans/path-class-manifest-design.md:133-139 says an exact inverse of a record is allowed. The goal at metasystem/plans/goals/path-class-manifest.md:4 says records land under register carriage. metasystem/plans/path-class-manifest-design.md:204-211 makes rulings, receipts, and the digest append-only and existing history immutable; metasystem/records/README.md:3-6 says records are append-only for agents and humans.

## PCM-R1-005 — high, material=True

CLAIM: The record rule must verify actual stream ownership, not merely a caller-supplied goal name. A seat can pass another open, parked, or queued goal identifier and modify that goal's prefixed design; any existing plan that does not match an open goal is explicitly made a shared register and can be revised under any goal. That violates the stated requirement that the seat owning the stream lands its records. The build artifacts that change are the ownership branch in metasystem/internal/landing/observe.go's registerCarriage function and TestObserveRecordSemantics: they must bind the base goal's claimed machine and lineage to the wrapper actor, define the lawful treatment of unmatched existing plans, and include a cross-seat refusal fixture.

EVIDENCE: metasystem/plans/path-class-manifest-design.md:209-216 checks handoffs against Machine but checks designs only against `Goal-Item`, and explicitly makes legacy designs shared. metasystem/internal/goal/file.go:75-84 defines the claimed goal owner, and metasystem/plans/goals/path-class-manifest.md:11-13 demonstrates that a claimed goal records machine, lineage, revision, and epoch. The design's self-grade at :291-294 concedes that any open goal can revise the remaining plans.

## PCM-R1-006 — medium, material=True

CLAIM: Define a deterministic goal-identifier tie-break for design filenames. Goal identifiers are not prefix-free, so `plans/<goal-id>-*.md` can match more than one base goal; “a path matching several rows takes the first” resolves table rows, not two identifiers within the same row. The build artifacts that change are the goal-name matcher used by registerCarriage and a TestObserveRecordSemantics case asserting the selected owner, preferably the longest complete goal identifier.

EVIDENCE: metasystem/plans/path-class-manifest-design.md:210 and :213 do not define an identifier tie-break. The tracked base contains both metasystem/plans/goals/codex-handshake-budget.md and metasystem/plans/goals/codex-handshake-budget-load-fragile.md, so a future `plans/codex-handshake-budget-load-fragile-design.md` matches both prefixes.

## PCM-R1-007 — high, material=True

CLAIM: Carry the new ownership inputs through the real landing entrypoint and move machine resolution before evaluation. metasystem/scripts/agents/land.sh is the normal wrapper caller, rejects unknown options, has no `--goal`, and is absent from the claimed exhaustive diff. In metasystem/scripts/agents/commit.sh, machine_nickname is currently assigned after the evaluator call, so forwarding it from its present location would fail under `set -u`. The build artifacts that change are metasystem/scripts/agents/land.sh, metasystem/scripts/agents/land-fixtures.sh, the ordering in metasystem/scripts/agents/commit.sh, and an end-to-end land-to-evaluator fixture.

EVIDENCE: metasystem/plans/path-class-manifest-design.md:163-172 requires Goal and Machine inputs but :270-282 omits land.sh and land-fixtures.sh. metasystem/scripts/agents/land.sh:25-80 rejects unknown options and :244-253 forwards no goal. metasystem/scripts/agents/commit.sh:297-306 evaluates the landing, while :360 reads machine_nickname.

## PCM-R1-008 — high, material=True

CLAIM: Make Goal-Item a validated, single-valued wrapper-owned field. The design promises that the stamped line and evaluator input cannot disagree, but it neither validates the goal identifier nor rejects a Goal-Item already present in `-m`, `-F`, or caller-supplied `--trailer` arguments. Git retains both values, leaving later readers free to choose a different owner than the evaluator did. The build artifacts that change are metasystem/scripts/agents/commit.sh and TestCommitWrapperStampsGoalItemTrailer in metasystem/scripts/agents/path-class-fixtures.sh, which must cover malformed and conflicting inputs as well as the happy path.

EVIDENCE: metasystem/plans/path-class-manifest-design.md:168-172 makes the “cannot disagree” claim. metasystem/scripts/agents/commit.sh:43-90 passes arbitrary remaining arguments to git and :100-110 scans only the obsolete claude.ac spelling. Running git interpret-trailers with existing `Goal-Item: victim` and requested `Goal-Item: fx` produced both trailer lines.

## PCM-R1-009 — high, material=True

CLAIM: Return unclassified-path details from the base-tree evaluator instead of recomputing them with the checked-out verb. The evaluator deliberately judges the landing base manifest, while the proposed refusal branch invokes a verb that reads the candidate checkout. A candidate that changes the manifest and adds an extra path can therefore be refused as unclassified but print a classified answer or no required refusal text. The build artifacts that change are a structured base-derived refusal detail in metasystem/internal/landing/observe.go's Observation, its consumption in metasystem/scripts/agents/commit.sh, and a base-versus-candidate fixture in metasystem/internal/landing/observe_test.go or metasystem/scripts/agents/path-class-fixtures.sh.

EVIDENCE: metasystem/plans/path-class-manifest-design.md:69-73 assigns different manifest snapshots to the evaluator and verb, while :168-172 tells commit.sh to print the verb's text after a path-unclassified decision. The current Observation at metasystem/internal/landing/observe.go:47-57 carries no offending path or refusal detail.

## PCM-R1-010 — high, material=True

CLAIM: Define and promote the complete fail-closed code set. The design says an unreadable manifest refuses every landing and record violations refuse, but its promotion section adds only path-unclassified, ledger, and runtime, explicitly leaves record-not-owned observed, and does not promote register-carriage-policy-unreadable or register-carriage-not-append-only. Adding direct-fix-floor-refused later therefore does not enforce the claimed second bar. The build artifacts that change are metasystem/scripts/agents/landing-promotion.json and evaluator fixtures proving that malformed policy, non-append changes, and unowned records have mode `refuse` at the intended rollout point.

EVIDENCE: metasystem/plans/path-class-manifest-design.md:59-64 says malformed policy refuses every landing; :206-211 describes record refusals; :226-230 promotes three codes but leaves record-not-owned observed. metasystem/internal/landing/promotion.go:36-38 enforces only listed codes, and metasystem/scripts/agents/landing-promotion.json:3 currently lists only missing-declaration and conflicting-declarations.

## PCM-R1-011 — high, material=True

CLAIM: Preserve the invariant that every runtime-declared instruction file is behavior. Dropping the runtime registry union is safe for a new unclassified root filename, but not for a declaration under an already classified record prefix such as plans/. That file would immediately become waiver-eligible despite being declared instruction-bearing. The manifest can remain the classification source if conformance rejects any runtime InstructionFile whose manifest class is not behavior. The build artifacts that change are metasystem/internal/validate/conformance.go and the existing TestConformanceProtectsDeclaredInstructionFile in metasystem/internal/validate/conformance_test.go, extended with a record-classified declaration.

EVIDENCE: metasystem/plans/path-class-manifest-design.md:174-179 drops the registry union based only on the root-unclassified case. metasystem/internal/validate/conformance.go:541-550 currently protects every runtime declaration unconditionally. metasystem/internal/runtimes/runtimes.go:62-64 and :437-440 permit any clean relative instruction path, and metasystem/internal/validate/conformance_test.go:369-382 currently pins immediate protection for a newly declared runtime instruction file.

## PCM-R1-012 — medium, material=True

CLAIM: Specify the refusal text when no classified ancestor exists. The required example `product.txt` is a repository-root child and none of the proposed rows is its ancestor, so `<entry> (<class>)` has no defined value. The build artifacts that change are the refusal formatter in metasystem/internal/pathclass/pathclass.go and the exact expected output in TestPathClassVerbOneWordAndRefusalText and TestPathClassVerbAnswersFromManifest.

EVIDENCE: metasystem/plans/path-class-manifest-design.md:115-119 requires a nearest classified ancestor without defining a none value; :235-239 uses root-level product.txt as unclassified; :261-262 requires the shell fixture to assert that ancestor text.

## PCM-R1-013 — medium, material=True

CLAIM: Make the deleted-reader fixture search every behavior source, not a hand-selected subset. The proposed command proves today's nine readers are gone only from cmd, internal, scripts, skills, AGENTS.md, wow.md, and most docs; it omits behavior-classified CLAUDE.md, README.md, metasystem.conf, optional-skills, and harness registration roots. A future retained reader there would not fail the fixture. The build artifact that changes is TestDeletedListsHaveNoReader in metasystem/scripts/agents/path-class-fixtures.sh, ideally deriving its search set from manifest behavior paths while excluding declared historical record trees.

EVIDENCE: metasystem/plans/path-class-manifest-design.md:87-93 classifies the omitted surfaces as behavior, while :263-264 searches only a subset. Running the literal search today found exactly nine live files, so this is a recurrence-proof defect rather than an undiscovered current reader.

## PCM-R1-N01 — low, material=False

CLAIM: Size note: the original boundary already spans more than twenty files and several integration seams; the required land-wrapper, ownership, promotion, and namespace corrections leave no credible correction-round reserve inside 240 minutes. Splitting the work changes scheduling rather than a function, file, fixture, or manifest row, so this note is not material under the binding stop criterion.

EVIDENCE: metasystem/plans/path-class-manifest-design.md:268-282 declares one 240-minute slice while listing four new artifacts, two deletions, and roughly twenty modified files; PCM-R1-001 through PCM-R1-013 add omitted or underspecified work on the same critical path.

## Critic-declared gaps

- The design gives no sentinel for “nearest classified ancestor” when an unclassified path has no classified ancestor; I did not invent one.

- The design gives no tie-break when one design filename matches multiple hyphenated goal identifiers; I did not choose one silently.

- Once the unsafe shared-register treatment of unmatched existing plans is rejected, the design does not say whether those files are frozen, explicitly mapped, or migrated; that ownership policy needs an explicit fold decision.

- The design does not reconcile its fail-closed wording with the smaller promotion set, so I did not infer which refusal codes Wido intended to enforce in this slice versus the later promotion.

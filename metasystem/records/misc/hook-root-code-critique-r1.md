# Hook-root code critique — round 1 (chain hrd-build1-20260906, round 8)

Chain: build rounds 1-8 of hrd-build1-20260906 (reviewed tree 9b626d12681e91457955f20475d318e1a42f45ae, twelve files) -> critic hrd-review1-20260906 (Fable, code-critic, read-only runtime; the coordinator carried its return here verbatim). verdictMaterialCount=2.

## F-1 — high, material=True

CLAIM: The new linked-worktree mapping in the hook's resolver sends a second-session checkout (an isolated writer created by scripts/agents/second-session.sh as a linked worktree, armed as its own world) to the primary checkout's installation and state world. The hook's own OWNED-ELSEWHERE message directs users to that script, and the known-issues register records live use of such a checkout for dispatch. On main the shipped hook resolved that session to its own world: the payload cwd's Git toplevel is the worktree, the template marker is tracked there, the harness equals the marker's metasystem directory and carries metasystem.conf, so the state root became the worktree's own installation. On the reviewed tree the resolver sees a git-dir that differs from the common dir and maps to the primary, so every consumer of the turn runs against the primary: up announces the second session's process there (and claims the primary's lease if its holder is not live), the turn verdict reads the primary's ledger, the evidence trail and steward component record land in the primary, the second session's own armed supervision never sees hook evidence (hook-freshness dead there), and the Stop answers OWNED-ELSEWHERE telling the user to do what they already did. A delegate worktree dispatched from a second session shares the primary's common directory and also maps to the primary rather than to the session that dispatched it. This is the design's stated reject condition: a linked worktree whose primary counterpart at the same relative installation path is not the governing installation. The change ships a regression in a supported layout. Engine presence alone cannot separate a delegate sandbox from a second session, because the host builds engines inside delegate worktrees too (this chain's own worktree carries metasystem/bin/metasystem); a second session is distinguished by its own armed supervision state beneath its installation. Which criterion governs is the design authority's decision, so this finding changes what gets built.

EVIDENCE: Reviewed hook metasystem/scripts/agents/supervision-hook.sh lines 33-73: the resolver maps whenever the git-dir and common-dir differ and the common dir's basename is .git, with no test for the worktree being a governed world. metasystem/scripts/agents/second-session.sh lines 7-9 and 22 create the destination as a linked worktree of the current checkout (git worktree add beside it); line 40 arms supervision with --repo set to that destination through the new harness printed by validate session-isolation; metasystem/internal/validate/sessionisolation.go lines 74-88 compute that harness as the destination's copy of the source harness path. metasystem/memory/known-issues.md row KI-34 records a critique round dispatched from the second-session worktree agentic-tools-slc-r4 on 2026-08-09, so the layout is used in practice, and dispatch there requires a built engine in that worktree. Main's hook (metasystem/scripts/agents/supervision-hook.sh on main, lines 374-382) sets the state root to the harness when the marker sits at the cwd toplevel and the harness is its metasystem directory with metasystem.conf, which holds in a second-session worktree because development/metasystem-design.md and metasystem/metasystem.conf are tracked (both present in the delegate worktree metasystem/artifacts/agents/worktrees/hrd-build1-20260906, which ships tracked files). The design metasystem/plans/supervision-hook-root-design.md lines 1736-1738 names exactly this as a reject condition, and its worktree rule at lines 324-383 reasons only about delegate worktrees that ship no engine. The design-critique registers hook-root-critique-r4.md and hook-root-critique-r5.md contain no mention of second sessions, so no prior disposition covers this. Concrete input: run second-session.sh from a fleet checkout, build the engine in the new worktree, start a Claude session there, and fire Stop; the hook's up call carries --metasystem-root and --repo naming the primary's metasystem directory with the second session's pid.

## F-2 — medium, material=True

CLAIM: When the parent's state-root query does not answer before the deadline, the refusal record and the trail line are written beneath the physical script grandparent (deadline_harness_root). In a mapped delegate worktree that directory is the sandbox, so a slow-root timeout writes sandbox state, which the design forbids for a mapped worktree, and the record vanishes with the sandbox. The parent already holds the mapped installation (deadline_installation) without any engine, and the round-6 disposition's phrase 'the installation's own supervision directory' names that directory in the design's vocabulary; the disposition's parenthetical naming deadline_harness_root coincides with the installation in every layout except the mapped worktree, the one layout where the choice matters. The two slow-root fixture cases pin the sandbox location and drop the sandbox-negative assertion for those cases, so the certification now certifies the sandbox write. The fix is one assignment (fallback root = deadline_installation when identification succeeded, else deadline_harness_root) plus the fixture's expected root for the slow-root cases and the restored sandbox negative.

EVIDENCE: metasystem/scripts/agents/supervision-hook.sh line 145 sets deadline_record_root to deadline_harness_root; line 156 builds the record path from it; line 163 builds the trail directory from it; line 119 shows deadline_installation available from the engine-free resolver before either is used; lines 349-369 write the record and trail line on timeout in both the resolved and unresolved cases. metasystem/scripts/agents/supervision-fixtures.sh: run_nested_timeout is called for nested-wt-parent-timeout-slow-root and nested-wt-parent-steered-timeout-slow-root with expected_root set to the worktree installation, and the assertion after them checks only the worktree root and the scope, not the worktree installation's artifacts directory, while the two fast cases before them still assert that directory is absent. Design lines 1414-1425 and 1742-1744 (nothing in a mapped worktree reads or writes sandbox state). Round-6 disposition in metasystem/records/misc/hook-root-critique-r6.md, coordinator disposition paragraph. metasystem/.gitignore ignores artifacts/, so the write does not poison conformance, but the record is lost with the sandbox and a later resolved-root timeout for the same session counts occurrences from a different file.

## F-3 — low, material=False

CLAIM: A resolver answer that arrives during the parent's published-output validation window is never harvested. The last harvest runs after the wait loop, then up to six engine calls validate the worker's published output, then the resolver is stopped with no harvest afterwards. A state root that arrived in time is then recorded as unresolved and the record goes to the fallback root with the unresolved cause. The Stop is still blocked, so this is a degradation of record placement only. A harvest call after the resolver stop and before the hook-expire call closes it.

EVIDENCE: metasystem/scripts/agents/supervision-hook.sh: harvest at line 216, published validation at lines 288-317, resolver stop at line 319, no further call to deadline_capture_engine_coordinates before the timeout record at line 357.

## F-4 — low, material=False

CLAIM: The inline collector the hook now runs for a vendored installation (state world different from the installation) returns 1 when the lease gate reports a null claim epoch, which is the HUMAN holder answer the original collector script serves by running the collection ungated. In that layout a Stop whose caller classifies HUMAN (no identified runtime process and a controlling terminal, or an identified process that is not announced) records 'the hook evidence state could not be maintained' and composes a refusal where the original collected. The rewrite was orchestrator-directed in build round 2, so this is not a conformance finding, and with a runtime that is identified and announced the caller classifies HOLDER, so reachability is narrow. Separately, lease require-holder and run-held take no metasystem-root, so under the state-world root they read adapters at the scope, where the vendored layout has none; that is a pre-existing limitation of those verbs the rewrite inherits.

EVIDENCE: metasystem/scripts/agents/supervision-hook.sh lines 454-475 (collect_hook_evidence, the claim-epoch test at line 467); metasystem/scripts/agents/evidence-gc.sh lines 20-30 (the human branch); metasystem/internal/lease/verbs.go lines 365-376 (HUMAN returns holder true with no claim epoch) and 464-472 (run-held runs a HUMAN ungated); metasystem/internal/lease/classify.go lines 368-370 (HUMAN classification); metasystem/cmd/metasystem/lease.go lines 88-103 and 121-139 (no metasystem-root flag). Round-2 brief metasystem/artifacts/agents/hrd-build1-20260906/rounds/2/prompt.md item 1 directed the move to the resolved world.

## F-5 — low, material=False

CLAIM: Two fixture negatives are weaker than the design's text. The copied-hook and symlink cases assert no artifacts directory directly under the scope's development directory, but those hooks' physical grandparent is development/sub, so the location a misdirected write would use is unchecked; the exact-literal match on the output corroborates that no write happened, so the case still discriminates. The sibling, inside, freshness, linked-worktree and Git-steered cases all fire under METASYSTEM_BIN set to the up auditor, so the no-override engine resolution is proven only by the copied-hook firings and the two completion cases that unset the override.

EVIDENCE: metasystem/scripts/agents/supervision-fixtures.sh: copied_dir is development/sub/scripts/agents and the negative checks development/artifacts; fire_nested always adds METASYSTEM_BIN=$fixture_up_auditor to the command words; the completion cases and the copied-hook loop pass env -u METASYSTEM_BIN.

## F-6 — low, material=False

CLAIM: The up census-scope query moved from reading git's standard output only to the state-root package's shared query, which parses combined standard output and error. A git warning on standard error with exit 0 would corrupt the scope path. This shape is pre-existing in the compiled authority and the design asked for the single implementation, so this is a note for the authority's owner, not a defect of this change.

EVIDENCE: metasystem/internal/stateroot/stateroot.go lines 42-50 (CombinedOutput then TrimSpace then Abs); metasystem/cmd/metasystem/up.go upRepositoryScope now calls stateroot.RepositoryTop; the removed code used command.Output().

## F-7 — low, material=False

CLAIM: The override rule added to up in build round 4 (an explicit METASYSTEM_BIN turn skips the invoking-engine drift check for ordinary arming and re-arms from the enrolled path) launches supervision from the enrolled command, so it creates no engine split; but the branch that re-arms a rebuilt enrolled engine under an override has no test, only the unchanged-enrollment branch is covered.

EVIDENCE: metasystem/internal/up/up.go lines 442-467 (the override branches) and line 367 (armingOptions.Command = enrolledCommand(enrolled), which takes precedence over Binary at metasystem/internal/supervise/arming.go lines 603-611); metasystem/internal/up/up_test.go TestInvokingEnrollmentAcceptsAnExplicitOverrideWithoutChangingEnrollment asserts rearmed.Status is empty, so the rebuilt branch is never entered.

## Gaps the critic named

- Nothing was executed in this review: the runtime is read-only, so every claim is from reading the reviewed tree, the diff, the design, the registers and the round records, with the host receipt cited as given.
- Whether any fleet seat checkout is itself a linked worktree of another seat's repository could not be verified from this host; the second-session script, its fixture and the KI-34 record establish that the layout is a supported and used one regardless.
- F-1 changes the design's worktree rule, which the code critic cannot amend; the criterion that separates a delegate sandbox from a governed worktree (armed supervision state at the candidate, a branch-name convention, or another mark) is the design authority's choice, and the fixture set for it does not exist yet.
- The behavior of the bed under a machine load that stretches the parent past sixty seconds (the elapsed < 60 assertions in the timeout cases) was not assessed; it is main's deadline fixture shape and the host receipt reports it green.

## Coordinator dispositions (m1, 2026-09-06)

Both material findings are built behind fixtures in the next round; the
design's worktree rule is amended by this record (the design authority's
choice the critic could not make), not by a seventh revision.

- F-1: the linked-worktree mapping to the primary applies only when the
  candidate worktree is NOT a governed world of its own. The mark is the
  candidate's own armed supervision state: a steward identity under the
  candidate installation (its artifacts, agents, steward, identity.json)
  means the worktree is its own world - the second-session layout - and
  resolves to itself; a worktree without one (every delegate sandbox)
  maps to the primary. Fixture: a linked worktree armed as its own world
  resolves to itself and runs its Stop there; an unarmed one maps.
- F-2: on a slow-root timeout the refusal record and the trail line go
  under the mapped installation the parent already holds without any
  engine (deadline_installation), never under the physical script
  grandparent; the two slow-root cases assert the record under the mapped
  installation and keep their sandbox-negative assertion.
- F-3: one more harvest of the resolver's answer after the resolver is
  stopped and before the hook-expire call.
- F-5: the copied-hook and symlink negatives check the hooks' physical
  grandparent (development/sub) as well; at least one ordinary nested
  case fires without METASYSTEM_BIN.
- F-7: a test for the override branch that re-arms a rebuilt enrolled
  engine.
- F-4 (vendored-layout HUMAN caller through the inline collector) and
  F-6 (the shared repository-top query parses combined output; a note
  for the stateroot authority's owner) are recorded, not built here.

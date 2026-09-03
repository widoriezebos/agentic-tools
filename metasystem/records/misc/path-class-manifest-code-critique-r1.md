# Path-class manifest slice 1 code critique — round 1 (Fable)

Chain: build path-class-build1 (terminal round path-class-build1-r2, reviewed tree 71d5f0ea) -> code-critic path-class-cc1 (claude {'effective': 'claude-fable-5-1', 'requested': 'claude-fable-5-1'}), 2026-09-03. 5 material findings. Full return: artifacts/agents/path-class-cc1/rounds/1/return.json.

## PCM-CC1-001-TRACKED-PATH-TEST-NEEDS-GIT — high, material=True

CLAIM: TestRepositoryManifestClassifiesEveryTrackedPath in metasystem/internal/pathclass/pathclass_test.go runs git ls-files against the parent directory of the installation and fails the test on any git error. The Go gate that certifies a landing runs the internal package tests inside a frozen export or an extracted archive snapshot, and both deliberately omit the .git directory, so that parent is a temporary directory git does not recognise and the gate goes red on its first run. In an adopted installation the same walk is wrong too: the parent is either the application repository, whose every file then reports as unclassified, or a directory outside any repository. The design says the repository walk belongs to template mode only. The test must walk the installation with git run from the installation, add the repository walk only in template mode, and skip when git reports no repository.

EVIDENCE: Diff: the test computes installation as the absolute path of ../.., takes its parent as the repository, runs git -C repository ls-files -z through Output(), and calls t.Fatal on error. metasystem/internal/proofrun/manifest.go lines 135 to 141 prune .git from the export; freeze.go lines 23 and 38 place the export under a fresh temporary directory. metasystem/scripts/agents/go-gate.sh lines 282 to 331 run the same gate from that export; lines 111 to 113 and 584 to 587 describe the controller running it inside an extracted git-archive tree; line 528 runs go test over ./internal/... there. Design section 7 line 252 to 253: the test walks git ls-files of the installation 'and, in template mode, of the repository'. Builder rounds 1 and 2 ran the package tests only from the live checkout. A grep of every internal test for ls-files or show-toplevel finds only the mission-bed helper, which targets repositories the tests build themselves.

## PCM-CC1-002-COVERAGE-RATCHET-FLOOR-MISSING — high, material=True

CLAIM: The diff adds the new package metasystem/internal/pathclass with tests but registers no coverage floor. The full Go gate's coverage ratchet refuses any measured package that has neither a floor nor an exemption, so the landing gate fails on its first full run. Both baselines, metasystem/scripts/agents/coverage-ratchet.json and coverage-ratchet-linux.json, need an internal/pathclass floor, and neither file is inside the declared slice-1 boundary, so the boundary must be amended in the same correction round.

EVIDENCE: metasystem/internal/audit/coverage.go lines 94 to 104: a measured package absent from floors and exemptions yields the violation 'has no ratchet floor; register it'. go-gate.sh line 528 measures every package under internal with -cover and lines 579 to 580 exit 1 when the ratchet refuses. A grep of both baselines finds internal/stateroot at line 56 and 57 respectively but no internal/pathclass. The diff touches neither baseline; design section 8 lists neither in either slice. Builder evidence in both rounds shows no full-gate run.

## PCM-CC1-003-WAIVER-RULE-NEVER-ANSWERS-OUTSIDE — medium, material=True

CLAIM: The critique-waiver rule in metasystem/internal/validate/conformance.go classifies every changed path in the installation namespace only. A path outside the installation, such as the record file development/evidence-index.md in the template or any application file in an adopted repository, now reads as unclassified and the prose-under-30 waiver refuses it, where before this diff it was waivable. The design's resolution rule says the waiver rule resolves by location, with repository rows in template mode and 'outside' in adopted mode, and that record and outside stay waivable; its section 3(b) formula, however, is the installation-only Class call the implementer followed. The orchestrator must rule which reading stands. If the location rule stands, the merge stage needs a resolver that knows the installation root, repository root and mode, plus a test with an out-of-installation record path.

EVIDENCE: Diff lines 1247 to 1248 hand classes.Class to mergeWaiver and lines 1273 to 1275 refuse behavior, ledger, runtime and unclassified. The diff's Manifest.Class resolves only the install namespace and can never return outside. Base conformance.go lines 72 to 79: installationPath returns an out-of-prefix repository path unchanged; lines 721 to 740: boundaryViolations refuses only plans/ and artifacts/agents, so such paths reach the classifier. The base isInstructionBearing matched only listed entries, so development paths were waivable. Design lines 75 to 78 (location rule) and 163 to 167 ('record and outside stay waivable').

## PCM-CC1-004-RESOLVER-MASKS-DISCOVERY-FAILURE — medium, material=True

CLAIM: ResolvePath in metasystem/internal/pathclass/pathclass.go accepts the ownership oracle's 'outside' answer before looking at the error that comes with it. The oracle returns 'outside' plus an error not only when the path lies outside the repository but also when it cannot locate the installation (an engine binary not sitting at <installation>/bin/metasystem) or cannot read the repository top. The verb then prints the word outside with exit 1 and no diagnostic, so a broken or misplaced installation is reported as a path outside the repository, and the resolver's own discovery errors are unreachable for these cases. The sibling path owner verb prints the error in the same situation. Run discovery before the oracle, or surface the oracle's error unless the resolver's own containment check confirms the path is outside, and add a misinstalled-engine test.

EVIDENCE: Diff lines 763 to 770: the OwnerOutside check returns Resolution{Class: Outside} with a nil error before ownerErr is examined. metasystem/internal/stateroot/owner.go lines 33 to 57 return OwnerOutside with an error for installation lookup failure (mode empty) and repository-top failure; stateroot.go lines 137 to 155 fail the lookup when the executable is not under bin/. Base metasystem/cmd/metasystem/path_verbs.go lines 20 to 27: path owner prints the error. Design section 2 defines outside as outside the repository or the application's own path in adopted mode.

## PCM-CC1-005-EXACT-REVERT-UNCLASSIFIED-PASSES-FLOOR — medium, material=True

CLAIM: Under exact revert, exactRevertFloorError in metasystem/internal/landing/observe.go refuses every classified path but lets an unclassified path through to the exact-inverse checks. The hard-coded floor it replaces also caught nested instruction paths by suffix and by directory name (product/AGENTS.md, product/scripts/x), so an exact revert of a commit that added such a nested file now passes where it was refused before, although slice 1 promised that no verdict widens and that the floor reads behavior rows. The nested leg of TestObserveDeclaredDirectFixEvaluatesPerClassRule used to pin that unconfigured case; the diff added an explicit manifest row for product/AGENTS.md to its fixture, so the test no longer proves what it proved. The same function also treats record, ledger and runtime rows as floor, which is fail-closed but not what the brief states. Decision needed: refuse unclassified paths under exact revert until slice 2 introduces path-unclassified and restore the nested leg's original premise, or record the transitional pass in the design.

EVIDENCE: Diff lines 367 to 380: the new floor test is classes.Class(changedPath) != pathclass.Unclassified, with the comment that an unclassified path 'still has to satisfy the exact-inverse checks'. Diff lines 385 to 406 show the deleted floor matching HasSuffix(clean, '/'+exact) and Contains(clean, '/'+prefix). Base observe_test.go lines 329 to 343: the nested leg commits product/AGENTS.md with no policy row and expects direct-fix-floor-refused. Diff lines 438 to 445 append install:product/AGENTS.md behavior to that fixture's manifest. Design lines 281 to 284: 'the floor reads behavior rows ... no verdict widens'; section 3 table routes unclassified to path-unclassified in slice 2.

## PCM-CC1-006-SYMLINKED-WORKING-DIRECTORY — low, material=False

CLAIM: The resolver makes the input absolute against the caller's working directory as the shell reports it, which keeps symlinks, while both the installation root (symlinks resolved on the executable) and the repository root (git prints the physical path) are physical. On a repository reached through a symlinked directory, such as /tmp or /var on macOS, the containment check fails and the verb answers outside. The three-form test avoids the case by resolving symlinks on its temporary directory. The ownership oracle has the same convention today, so this is an inherited seam rather than a new defect.

EVIDENCE: Diff lines 811 to 817 (absoluteInput uses filepath.Abs only), 827 to 843 (installation root resolved with EvalSymlinks), 845 to 853 (repository root from git rev-parse --show-toplevel), and line 1034 where the test resolves its temp dir first. owner.go line 30 to 31 states symlinks are judged by entry path.

## PCM-CC1-007-SKIP-IDIOM-NO-LONGER-FIRES — low, material=False

CLAIM: TestInstructionOwnersAreBehavior checks os.IsNotExist on the error from pathclass.Load, but Load wraps the file error with fmt.Errorf and %w, and os.IsNotExist does not unwrap that form, so the frozen-export skip can never trigger. Harmless today because the export copies every non-runtime byte including the manifest, but the skip is dead code.

EVIDENCE: Diff lines 1429 to 1432 and 594 to 601; proofrun/manifest.go lines 95 to 101 say every non-runtime byte is exported so tests find the content they read.

## PCM-CC1-008-FIXTURE-EMPTY-SEARCH-SET — low, material=False

CLAIM: In TestDeletedListsHaveNoReader the repository-side search hands rg the list of existing repository behavior paths; if none exists (an adopted installation whose parent directory is not the template repository) rg receives no paths and reads standard input or walks the parent directory. Only reachable outside the template layout, which the fixture assumes.

EVIDENCE: Diff lines 1657 to 1659 and 1674 to 1678; the existence guard can leave repo_paths empty and the rg call passes the array unconditionally.

## PCM-CC1-009-DECLARED-FILE-TEST-REPLACED-NOT-EXTENDED — low, material=False

CLAIM: The design says TestConformanceProtectsDeclaredInstructionFile 'gains' a plans/NEWRT.md declaration; the diff replaces the root-level NEWRT.md case with it, so a root-level declared file with no row is no longer tested directly. The new invariant covers it (unclassified is not behavior), so no behavior is lost.

EVIDENCE: Diff lines 1375 to 1390 versus design line 255 to 256.

## Critic-declared gaps

- My tools were read-only (file read, glob, grep); I could not run go test, the shell fixture, the conformance verb, or git. Every execution claim in the builder's evidence is taken as read, and my own evidence levels are read or ran-as-search only.

- I could not enumerate the top-level tracked entries of the installation or the repository: directory-wide globs timed out on this large checkout. The check 'a tracked top-level entry with no row' therefore rests on the builder's recorded pass of TestRepositoryManifestClassifiesEveryTrackedPath in a live git checkout, which is the exhaustive form of that check.

- The harness reported the tool catalog as unobserved and this job as advisory; I did not run metasystem validate conformance --stage review myself and read reviewedTree from the round-2 review artifact.

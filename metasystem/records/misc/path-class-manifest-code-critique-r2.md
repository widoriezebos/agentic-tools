# Path-class manifest, first part: closing code critique (Fable)

Chain: build path-class-build1 (terminal round path-class-build1-r3, reviewed tree 34e56d849455691c6a5852ea0885a3cd71447d5b) -> code-critic path-class-cc2, 2026-09-03. 2 material findings. Full return: artifacts/agents/path-class-cc2/rounds/1/return.json.

## PCM-CC2-001-WAIVER-DROPS-LEDGER-AND-RUNTIME — medium, material=True

CLAIM: The critique-waiver rule in the conformance merge stage now refuses only behavior paths and unclassified paths inside the installation. The certified design's section 3(b) says the prose-under-30 waiver also refuses ledger and runtime paths, and the round-2 tree refused them; the correction round removed both classes. A waiver that touches a goal-ledger file such as records/goals/<id>.md is therefore accepted by the merge stage (the plans/ ledger rows are still caught by the separate plans/ boundary check, and runtime rows are mostly non-Markdown or ignored, so the exposed surface is records/goals/). The fix brief's sentence 'refuses only behavior or unclassified within the installation' can be read as ordering this, so the orchestrator must either restore the two classes in metasystem/internal/validate/conformance.go with a test leg or record a design amendment; either way the build changes.

EVIDENCE: Diff lines 1463 to 1470: the switch in mergeWaiver has arms only for Behavior and for Unclassified with namespace Install. Design lines 163 to 167: 'the waiver refuses when any path is behavior, ledger, runtime or unclassified; record and outside stay waivable'. My round-1 register (finding 003 evidence) records the round-2 tree refusing behavior, ledger, runtime and unclassified. Base conformance.go lines 721 to 740: boundaryViolations refuses only plans/ and artifacts/agents, so a records/goals/ Markdown file reaches the class check and passes it. The manifest (diff lines 2035 to 2038) classifies records/goals/ as ledger. Fix brief lines 34 to 36 carry the sentence that drops the two classes while also saying 'as the design's section 3 says'.

## PCM-CC2-002-ROOT-LAYOUT-WAIVER-STILL-REFUSES-APP-FILES — medium, material=True

CLAIM: The location fix for the waiver rule covers the template and a vendored adopted layout but not the layout scripts/adopt.sh produces, where the installation is the repository root and the git prefix is empty. There, waiverPathMode reports adopted mode, yet ResolveRepositoryPath with an empty prefix resolves every changed path in the install namespace before looking at the mode. An application file with no install row (for example src/notes.md) is unclassified-in-install and refused; an application file under docs/ matches the install docs/ behavior row and is refused. Both were waivable before this slice, contrary to the design's rule that an adopted installation's own additions keep today's rules, and the path class verb answers outside for the same file in the same layout, so the two consumers of one manifest disagree. This is the 'any adopted application file' case of PCM-CC1-003, still open for the real adopted layout. The empty-prefix adopted case in metasystem/internal/validate/conformance.go must consult application ownership (the state-root ownership oracle or its shipped-inventory rule) before manifest matching, with a test whose prefix is empty.

EVIDENCE: Diff lines 1315 to 1318: waiverPathMode returns Adopted when installPrefix is empty. Diff lines 771 to 778: ResolveRepositoryPath returns m.Resolve(Install, key) when the prefix is empty, before any mode check. Diff lines 1463 to 1469: unclassified in the install namespace is never waivable; install:docs/ is a behavior row (diff line 2015). adopt.sh usage lines 10 to 18 and target paths at lines 355 to 462 install metasystem.conf, scripts/ and .github/ directly under the target; stateroot/owner.go lines 86 to 95 name this the unvendored adopted layout and owner_test.go lines 98 to 128 prove docs/application.md is application-owned there. conformance.go lines 222 to 232: the prefix is git rev-parse --show-prefix, empty at a repository root. Shell fixture diff lines 1943 to 1948: the verb answers outside for docs/application.md with the engine at the fixture root. The deleted list (diff lines 1834 to 1846) named only specific files and directories, so src/notes.md and docs/guide.md were waivable at base. Design lines 73 to 78 and 163 to 167; fix brief lines 29 to 36 name the adopted application file case. The only new mode test (diff lines 1597 to 1625) uses a nested installation named metasystem, never an empty prefix.

## PCM-CC2-003-LANDING-PACKAGE-HAS-NO-RATCHET-FLOOR — high, material=False

CLAIM: Independent of this slice, the package internal/landing has neither a floor nor an exemption in either coverage baseline, so the full Go gate's ratchet refuses with 'package internal/landing has no ratchet floor; register it' on this tree whatever this diff does. The correction brief's condition that the gate be green cannot be met without a landing floor, and adding one is a behavior-path change to the same two baseline files this chain already touches. Not a defect of the diff; recorded so the gate replay's refusal is not misread as a slice failure.

EVIDENCE: coverage-ratchet.json read in full: fifty-nine floors from internal/acp to internal/wiredoc with no internal/landing, and exempt holds only cmd/metasystem. Grep for internal/landing in coverage-ratchet-linux.json: no match. audit/coverage.go lines 94 to 104: a measured package absent from floors and exempt is a violation. go-gate.sh line 528 measures every package under internal with -cover and lines 579 to 580 exit 1 on refusal. internal/landing holds observe_test.go with no build constraint, so it is measured. memory/rulings.md R-40-m0 dates the two-bars landing to 2026-09-02; the newest gate-failure log (2026-08-30) predates it and records missionrunner test failures; the ratchet step keeps no failure log.

## PCM-CC2-004-TRACKED-PATH-WALK-IGNORES-MODE — low, material=False

CLAIM: The tracked-path test walks the installation's parent repository whenever that parent is a git work tree, without checking template mode; design section 7 says the repository walk belongs to template mode only. In the adopted layout adopt.sh produces the installation's parent is outside the repository, so the probe skips in practice; the walk misfires only when an installation's parent lies inside some other work tree, where sibling files land in the repo namespace as unclassified. Section 7 is outside this review's regression scope and the correction brief allowed a plain skip, so this is recorded, not actioned.

EVIDENCE: Diff lines 1194 to 1220: the parent is filepath.Dir of the installation, the probe is git rev-parse --is-inside-work-tree, and every non-metasystem/ tracked path is resolved in the repo namespace regardless of mode. Design lines 252 to 253. Fix brief lines 17 to 22.

## PCM-CC2-005-EXACT-INVERSE-LOGIC-UNREACHABLE-IN-SLICE-ONE — low, material=False

CLAIM: Because Manifest.Class can only answer one of the five install-namespace classes and the exact-revert floor now refuses all five, the exact-inverse comparison in exactRevert is unreachable in slice 1, and the two test legs that proved a passing exact inverse and a not-exact-revert refusal now assert the floor instead. In the template the design says exact revert has no lawful target, and the observed-only code direct-fix-floor-refused is not promoted, so no landing outcome changes; an adopted installation's revert of its own file is observed as floor-refused until slice 2 brings the mode-aware class table and its passing outside leg. Slice 2 must restore a leg that exercises the comparison.

EVIDENCE: Diff lines 367 to 378 and 744 to 747 (base observe.go lines 744 to 777); diff lines 433 to 449 change both legs' expectations; landing-promotion.json lists only missing-declaration and conflicting-declarations; design lines 126 to 129 and 238.

## PCM-CC2-006-MISINSTALLED-DIAGNOSTIC-WHEN-MANIFEST-ABSENT — low, material=False

CLAIM: discoverInstallationRoot reports 'executable is not installed at <installation>/bin/metasystem' whenever scripts/agents/path-classes.txt is missing beside the binary, including an engine that is correctly installed in an adoption predating the manifest. The message names the wrong cause; the exit path is otherwise right.

EVIDENCE: Diff lines 905 to 912: the only check after locating bin/.. is a stat of the manifest path, and its failure produces the installation message.

## Critic-declared gaps

- My tools were read-only (file read, glob, grep). I could not run go test, the shell fixtures, the conformance verb, the Go gate or git; every execution claim is the builder's, taken as read.

- I could not enumerate the tracked paths of the installation or the repository because directory-wide globs time out on this checkout. Manifest completeness rests on the builder's live-checkout run of the tracked-path test.

- I did not run metasystem validate conformance --stage review myself; the reviewed tree is read from the round-3 review artifact.

- The path-class coverage floor of 72.7 percent is a single measurement from the builder's sandbox on one platform, copied into both baselines; neither platform's gate has measured it, and no round reached the gate's coverage stage.

- The full gate cannot be green on this tree for a reason outside the slice: internal/landing has no coverage floor or exemption in either baseline (finding PCM-CC2-003). The brief's condition that the gate be green in the export therefore cannot be met by this diff alone; the orchestrator must decide whether to fold a landing floor into this chain (the same two baseline files are already in its boundary and are behavior paths) or land it separately. Coverage of internal/validate on the final tree (floor 79.9) is also unmeasured.

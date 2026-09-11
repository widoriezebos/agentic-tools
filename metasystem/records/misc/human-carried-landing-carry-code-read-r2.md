# Second read of the carried landing (Opus, hcl-cc2-20260911)

Chain hcl-build3-20260911, round 1, reviewed tree faccdb3b25ea9fc1d9ca6fa10a87864fe7a2fbfa. 16 findings, 12 material.

## Gaps the critic named

- The role is read-only, so metasystem/records/misc/human-carried-landing-carry-code-read-r2.md was not written. This return is the register for the coordinator to project: the findings, then the mandate-1 fold lines and the four named checks as evidence entries, then the verdict entry.

- As the brief asked, no Go test, bed, build or live proof was run. Every finding comes from reading the reviewed tree extracted with git archive, plus three shell probes: git show path resolution under the metasystem/ prefix, bash's EXIT trap on SIGTERM, and empty-array expansion under set -u on bash 3.2.

- Not traced to a conclusion: whether a group word can reach a structured red result when the group never ran (missing rather than failing). That depends on whether any carried-eligible attempt exists from which to recover the candidate engine digest, and it sits inside HCL-C-57.

- The new two-seat, crash and ledger-path legs, and the three goal-cli scenarios, were read but not run. Whether they pass is the coordinator's host rerun.

- Not re-checked against the reviewed tree: the ShellRows line citations for the new exit-3 asks in land.sh and commit.sh, and whether land.sh's parse of goal fetch output (the tip= field) matches the verb.

- The provider tool catalog and context isolation are unobserved, so this critique is advisory about runtime independence.


## Findings

### HCL-C-50 (critical, material)

Claim: No carried landing can reach its push in an installation that lives under a prefix directory, which is how this repository installs metasystem (under metasystem/). Each attempt also leaves a reservation open that blocks every other seat. print_carried_advisory, the function that prints the pre-push lines, reads the goal file with `git show "<ledger tip>:plans/goals/<goal>.md"` (scripts/agents/land.sh:805). Git resolves <commit>:<path> from the repository's top directory, not the working directory, so under a metasystem/ prefix the path does not exist, git exits 128, and `|| exit $?` ends land.sh. This happens after the carried commit exists and the local intent entry is created with land.sh as its owner (land.sh:753-775), and before the push (land.sh:819-825). The exit trap then asks `goal carrying --abandon`, which refuses because the intent's owner is alive (HCL-C-52), and the trap discards the refusal. What remains: a local carried commit at HEAD, and an open reservation row on origin's ledger that every other seat counts as in-flight carry debt until the word expires, up to four hours. A rerun takes the local: branch (land.sh:879-894), reaches the same line and exits the same way. Design 08 ('What the machine says, then does') and fold decision C14 put these lines before the push, and 05 and 06 require the push. The land bed cannot catch it: every carried leg installs at the repository top. If it stands: read the goal file prefix-aware (for example the ./plans/goals/ form from the root, or through the engine), and run one carried leg under a prefixed installation.

Evidence: land.sh:805 `goal_text=$(git show "$carried_ledger_tip:plans/goals/$landing_goal.md") || exit $?`; land.sh:13 `cd "$root"`, where root is the installation directory. Ran from metasystem/: `git show HEAD:plans/goals/human-carried-landing-carry.md` gives 'fatal: path 'metasystem/plans/goals/human-carried-landing-carry.md' exists, but not 'plans/goals/human-carried-landing-carry.md''; the HEAD:./plans/... form works. The goal ledger here is on main (trunk commits such as 01124a60c 'goal claim ...'). Order: finish_carried_publication (land.sh:819-839) runs create_carried_intent, then print_carried_advisory, then the push; abandon armed at 692 and 890, disarmed only after the push at 832. AbandonCarrying refuses a live owner (internal/goal/verbs.go:3541). The land bed seeds at the repository top (scripts/agents/land-fixtures.sh:467 adds scripts, payload.txt and plans/existing.md at the seed root).

### HCL-C-51 (high, material)

Claim: A seat's second carried landing is refused by the counselor line its first carried landing wrote. After a carried landing, goal carried's confirmed hook appends a line to records/counselor/carried-landings.jsonl in the checkout. This chain made that file tracked (created empty), so the checkout now shows it modified. land.sh's staging refuses unstaged changes, so the next landing must stage the line. In carried mode, candidatePathPolicy clears the goal for every records/counselor/ path (internal/landing/observe.go:177-178). recordCarriageError then refuses any change to an existing records/ file without a held goal: record-not-owned, 'existing record records/counselor/carried-landings.jsonl requires a held goal'. This happens even though step 2 of the classification has already proven the goal is held. Fold decision A4 said a records/counselor line refuses 'unless the wrapper wrote it', and A10 said the file is 'carried to origin by the next landing'. An ordinary landing with a chain and a held goal would pass the append-only check. But the carried landing exists for a seat whose ordinary landings are blocked, so such a seat can carry exactly once. This refuses a verified human on the machine's own judgement. Side note, not what makes this material: the same rule admits a brand-new file under records/counselor/ with no goal, and the workspace projection does not bind it. If it stands: admit an append-only change to carried-landings.jsonl whose added lines are cl-<opid> lines of carried rows on the tree (or treat it as the goal-free append-only register the fold names), keep refusing other counselor changes, and add a leg that carries twice on one seat.

Evidence: internal/landing/observe.go:151-185 (candidatePathPolicy; goal cleared at 177-178). observe.go:903-917: the records/ branch returns record-not-owned when the file existed and the goal is not held (910-911), and nil for a new file (904-909). scripts/agents/path-classes.txt:33 `install:records/ record`. scripts/agents/land.sh:366-386, stage_changes 'unstaged changes remain after staging'. records/counselor/carried-landings.jsonl is added empty by this diff; internal/landing/registers.go:11 lists it only for the workspace projection. No land leg runs two carried landings on one seat.

### HCL-C-52 (medium, material)

Claim: The wrapper's exit trap cannot close its own reservation once the local intent entry exists. land.sh creates that entry with itself as owner (`goal carrying --commit ... --owner-pid $$`). `goal carrying --abandon` refuses while a created carried entry for the word has a live owner ('carried intent ... is in flight'). The trap runs inside that owner, so from the intent entry until the push succeeds every exit leaves the row open: a failure while printing the pre-push lines (always, under HCL-C-50), a signal during those lines or the before-push pause, a failed push, and the moving-origin ask. The trap discards the refusal (`>/dev/null 2>&1 || true`). Fold decision A2 says every exit between the reservation and the push that is not the push closes the row, 'a signal' included. Design 08 'Who closes it' says the wrapper abandons on every ask after the row and before the push. Other seats have no release and wait out the word's expiry. Exits before the intent entry do close the row (commit.sh asks, second carry-forward asks, record failures of the local commit), and carried-asks checks that. If it stands: in the trap, terminalize the wrapper's own created intent entry (it is the owner) before abandoning, and add a leg that fails between the intent entry and the push and finds the closer row.

Evidence: scripts/agents/land.sh:242-251 (cleanup, trap cleanup EXIT); 753-775 (create_carried_intent with --owner-pid "$$"); 819-839 (finish_carried_publication: intent, lines, pause, push, disarm at 832). internal/goal/verbs.go:3512-3544, AbandonCarrying; the live-owner refusal is at 3541. internal/goal/journal.go:156-174, OwnerAlive. Ran /bin/bash -c 'trap "echo exit-trap-ran" EXIT; kill -TERM $$': it printed exit-trap-ran with status 143, so the trap fires on a signal and is then refused.

### HCL-C-53 (low, material)

Claim: One of the pre-push lines names the wrong review obligation for a red battery. land.sh always prints `carried obligation finding: carried:<commit>`, but the goal verb writes `carried:<commit>:battery-red` when the battery is red. Design 08 says the line is 'the obligation finding it will write', and 07 gives both forms. Only a green landing is exercised, so no leg sees it. If it stands: append :battery-red when Carried-Battery is red, and check the line in the red-battery leg.

Evidence: scripts/agents/land.sh:815; internal/goal/verbs.go:3797-3800; design lines 1355-1358 (07) and 1521-1530 (08).

### HCL-C-54 (low, material)

Claim: The goal carry --supersede line that land.sh prints when origin moved the workspace passes --by human:<name>, and goal carry adds human: again. Run as printed, it writes a carry word whose actor is human:human:<name>, and the Carried-By trailer, the carried row and the counselor line inherit that name. land.sh takes carried_by from landing carry-status's by field, which is the word row's actor (human:Wido, or human:wido for a channel word). runGoalCarry passes --by straight into Actor.Human, and historyActor renders "human:" + Human. Fold decision A7(a) required the printed line to be complete and correct as printed, and design 04 names the flag --by <name>. If it stands: print the name without the human: prefix (or have the command layer refuse a human:-prefixed --by), and assert the printed line in the second-word leg.

Evidence: scripts/agents/land.sh:673 and 713 (the printed lines use --by $carried_by); internal/landing/carried.go:50 (status.By = word.History.Actor); cmd/metasystem/goalsync_mutations.go:452 (Actor{... Human: by}); internal/goal/verbs.go:186-188 (historyActor).

### HCL-C-55 (low, material)

Claim: landing observe --judge base without --live-failure is accepted. Design fixture HCL-03-LIVE-FAILURE-STAMPED makes it a usage error, exit 2, from landing_verbs.go. The flags are parsed and passed through unchecked; the observer then records an empty live-failure, and the evaluator-unavailable case cannot match. commit.sh always passes the value, so the wrapper path is correct, but a direct call is not refused as the page says, and the fixture was deleted rather than built. If it stands: exit 2 in runLandingObserve when --carried is set and --judge is base with an empty --live-failure, or --judge is neither live nor base.

Evidence: cmd/metasystem/landing_verbs.go:27-66: flags at 42-43, no check before ObserveParams at 52. internal/landing/carried.go:264-267 and 287. Design lines 906-909.

### HCL-C-56 (low, material)

Claim: land.sh now admits --carried with neither --staged-only nor a pathspec (land.sh:172). On this host, where #!/usr/bin/env bash resolves to bash 3.2, a rerun in that form dies in the verify step. With nothing staged, verify_checks reaches `git diff --check -- "${pathspecs[@]}"`, and on bash 3.2 an empty array under set -u is 'unbound variable', which ends the script before carry-status is read. The cleanup trap removes the step output, so the human sees no reason. The reruns of 06 (local:, origin:, ledger:) are exactly the runs with nothing staged, and the asks print 'rerun land.sh --carried <opid>'. The beds always pass --staged-only, so none sees it. If it stands: guard the expansion, or require --staged-only in carried mode and say so in the usage line.

Evidence: scripts/agents/land.sh:6 `set -uo pipefail`, 172, 363. Ran `bash --version`: GNU bash 3.2.57. Ran `/bin/bash -c 'set -uo pipefail; a=(); printf "[%s]\n" "${a[@]}"'`: 'a[@]: unbound variable', status 127. Every carried land.sh call in scripts/agents/land-fixtures.sh passes --staged-only (for example the crash leg at 718-752 and the asks legs at 836-841).

### HCL-C-57 (high, material)

Claim: Nothing in the tree proves the match of step 14 or the red battery, which is the case the carried landing exists for. No Go test or bed leg drives any of these:
- a group word landing on a red battery (HCL-05-RED-BATTERY-LANDS). This is the only exercise of commit.sh's red path (test verify --carried --json, the group-list parse, `Carried-Battery: red missing=... failing=...`) and of the carried engine-digest recovery end to end;
- a closed chain whose only red group is named (HCL-05-CHAIN-PLUS-GROUP);
- a code word under an insufficient result (HCL-21-CODE-WORD-NEEDS-SUFFICIENT, HCL-21-TWO-FAILURES-ONE-NAME);
- an uncovered obligation or a discrepancy beside the named group (HCL-21-UNCOVERED-OBLIGATION-ASKS, HCL-21-DISCREPANCY-ASKS);
- a verify error asking carry-battery-unverified (HCL-21-VERIFY-ERROR-ASKS).
The implementer's return names these as gaps. Proven elsewhere, so outside this finding: a code word naming a different refusal asks and closes its row (carried-asks); a green code word lands with seven trailers (carried-fresh); the evaluator-unavailable ordering and the unneeded decision at function level (TestHCL37); the carried digest rule at unit level. This is design 05 step 14 and its fixtures, and mandate 7. If it stands: a table test over decideCarriedMatch or observeCarried for these rows, and a leg that lands a group word on a red battery.

Evidence: The 20 TestHCL functions in the reviewed tree include no match table beyond TestHCL37 (internal/landing/hcl_carried_test.go:11-32); carry-landing-standard lists 3 names (testing.json:38). Carried legs name only missing-declaration or conflicting-declarations (scripts/agents/land-fixtures.sh:673-676). The red path is in scripts/agents/commit.sh:720-760 and 785-792 and has no leg. Proven: carried-asks and carried-ledger-path at land-fixtures.sh:836-853; carried-fresh through 886-905; cmd/metasystem/test_test.go:386-389. Implementer return, gaps 4-6.

### HCL-C-58 (high, material)

Claim: The judge fallback and the proof fences are unproven. Nothing drives:
- the base-judge fence table (HCL-03-BASE-JUDGE-BLIND-ASKS). carried.go:355-395 matches the page's ten directories and five files by reading, but no candidate under any owner is shown to ask;
- a dead live engine, where commit.sh builds the base judge in a scratch worktree and lands with `Carried-Judge: base tree=... live-failure=exit=9` (HCL-05-LIVE-JUDGE-DEAD-BASE-JUDGES);
- no judge deciding (HCL-05-NO-JUDGE-ASKS);
- a generation-zero terminal word refused outside a fixture root (HCL-02-GENERATION-ZERO-REFUSED-IN-PRODUCTION). That is carried.go:184-186, plus CarryWordProven, which the debt, cap, status and done readers use.
Every land leg runs a fake-runtime root, so the production branch of the generation-zero rule is never taken. The return names HCL-02 and HCL-03 as gaps; the two judge legs are not named at all. This is design 02, and 05 'The evaluator' and step 11; mandate 7; and mandate 8 for HCL-C-03. If it stands: the fence table and generation-zero test in internal/landing, and the dead-live-judge and no-judge legs.

Evidence: internal/landing/carried.go:184-186, 232-239, 355-395; internal/goal/verbs.go:2840-2842; scripts/agents/commit.sh:321-348 (build_base_carry_judge), 600-640 (--judge live at 602, --judge base at 623). The carried legs write metasystem.runtimes=fake into metasystem.conf (land-fixtures.sh carried seed). No TestHCL03 or TestHCL02 function exists.

### HCL-C-59 (high, material)

Claim: Most recovery paths are unproven. Proven: the origin: rerun through goal carried --entry (carried-crash and the two-seat reruns), and the rebuild from commit trailers (goal-cli carried-record). Not driven:
- the local:<sha> rerun branch with ensure_recovery_reservation and verify_local_carried_commit (HCL-06-CRASH-BEFORE-PUSH-RESUMES, HCL-19-RECOVERY-KEEPS-REBASED-COMMIT, HCL-32-RECOVERY-CONFLICT-KEEPS-COMMIT);
- the ledger: branch and goal carried --repair-counselor (HCL-26-COUNSELOR-REPAIRED, HCL-26-REPAIR-FROM-ROW-ALONE, HCL-06-REPLAY-REFUSED);
- goal recover's new carrying and carried cases: the closer leg, the NothingToDo leg, and the confirmed leg's counselor replay (HCL-18-RECOVER-COMPLETES-CARRIED, HCL-18-RECOVER-ABANDONS-UNPUSHED-INTENT, HCL-18-LIVE-OWNER-UNTOUCHED, HCL-26-CRASH-BEFORE-HOOK-RECOVERS);
- the superseded landing state (HCL-23-SUPERSEDED-LANDS-NOTHING);
- consumption without a clock (HCL-24-CONSUMPTION-WITHOUT-CLOCK).
Recovery runs on every machine; the goal's exposure answer is 3. This is design 05 recovery, 06, and mandate 7. If it stands: carried-crash legs for the local, ledger and superseded branches, and the internal/goal recovery tests.

Evidence: scripts/agents/land.sh:735-751 (ensure_recovery_reservation), 848-863 (superseded and ledger branches), 879-894 (local branch). internal/goal/recover.go, reviewed tree: the carrying and carried cases of requestForEntry at 245-248 and recoverConfirmedEffect at 420-432. internal/goal/verbs.go:3746-3771 (recovering closer and NothingToDo). Proven: scripts/agents/land-fixtures.sh:718-752 and 755-834; scripts/agents/goal-cli-fixtures.sh:416-433.

### HCL-C-60 (high, material)

Claim: The refusals inside the goal verbs are unproven. Not driven:
- the replay refusal for a different goal and for each of the fourteen fields (HCL-25-REPLAY-MISMATCH-REFUSED). The code checks the goal target first and all fourteen fields, confirmed by reading only;
- supersede of an expired, consumed, foreign-seat or non-carry target, a target moved under the transaction, the push-before-row refusal, the captured-tip scan and an in-flight target (HCL-30-SUPERSEDE-PRECONDITIONS, -TARGET-MOVED, -PUSH-BEFORE-ROW, -SCANS-CAPTURED-TIP, -IN-FLIGHT), and the seat transfer (HCL-29-SUPERSEDE-TRANSFERS-SEAT);
- goal done refusing an open word, and archiving past an expired one (HCL-06-DONE-REFUSES-OPEN-CARRY, HCL-06-EXPIRED-WORD-IS-CLOSED);
- the obligation and in-flight debt asks at the word (HCL-08-DEBT-ASKS-AT-WORD, HCL-08-DEBT-CARRYING-ROW-AT-WORD);
- the reservation's word rechecks and its compare-and-swap (HCL-33-RESERVATION-RECHECKS-WORD, HCL-33-RESERVATION-CAS);
- abandon from another seat and with a live owner (HCL-33-ABANDON-CLOSES);
- the critic discharge of a carried obligation, and the accept-risk replay and changed-why refusal (HCL-07).
Driven: the format ask and raise, the word lines, the cap at the word, one supersede and the remote ask (goal-cli carry-word); one rebuilt record, its obligation, BudgetExceptions and an identical replay (carried-record); accept-risk discharge with one register line (carried-discharge); debt at the landing in all three forms (the two-seat legs); an own-seat abandon (carried-debt-abandoned). Mandates 7 and 8 (HCL-C-25). If it stands: the internal/goal tests and goal-cli scenarios named above.

Evidence: internal/goal/verbs.go:3735-3738 (goal target), 3857-3868 (compareCarriedRow), 1211-1222 (done), Carry's supersede preconditions inside carryRequest (verbs.go:3236-3290), 3512-3544 (AbandonCarrying). scripts/agents/goal-cli-fixtures.sh:367-414 (carry-word), 416-433 (carried-record), 435-453 (carried-discharge). No internal/goal TestHCL25, TestHCL30, TestHCL06 or TestHCL33 function exists; carry-goal-standard lists 5 names (testing.json:37).

### HCL-C-61 (medium, material)

Claim: The forward path and several landing-side asks are unproven. Not driven:
- the rebase-conflict posture before the reservation (HCL-05-REBASE-ASKS);
- a second word after origin moves code, including the channel --kind carry question of fold A7(a) (HCL-04-SECOND-WORD-LANDS);
- a peer ledger move keeping the word (HCL-04-LEDGER-MOVE-KEEPS-WORD);
- the release on the second carry-forward's ask (HCL-32-LEDGER-MOVE-RELEASES-ON-ASK);
- the lease held throughout (HCL-04-LEASE-HELD-THROUGHOUT);
- the cap at the landing (HCL-08-CAP-ASKS-AT-LANDING), and a paid debt landing (HCL-08-DEBT-PAID-LANDS);
- a goal not held (HCL-04-NOT-HELD-ASKS), and main only (HCL-05-MAIN-ONLY);
- the stop list under a valid word (HCL-05-RECORD-FAILURES-STAY), and a typed carried trailer (HCL-05-TRAILER-OWNED);
- the unneeded ask at the landing and its uncounted exception (HCL-05-UNNEEDED-ASKS, HCL-08-UNNEEDED-NOT-COUNTED);
- the seven-step order with its crash points (HCL-06-ORDER), and two commands only (HCL-08-TWO-COMMANDS);
- fold decision A7(b), the non-ancestor owner exiting 2, and A7(d), the failed code fetch asking with exit 3.
Driven: code mismatch with row abandonment (carried-asks), the ledger-path refusal (carried-ledger-path), and the order of the pre-push lines (carried-fresh). The legs carried-forward, carried-chain-group and carried-record-failures were removed rather than built. Mandate 7. If it stands: those legs.

Evidence: scripts/agents/land.sh:648-675 (carry_forward_staged: posture, second-ask release, channel question at 668-672), 340-349 (main only), 841-846 (fetch ask). cmd/metasystem/goalsync_mutations.go:224-227 (owner exit 2). The land bed's scenario list names seven carried legs, none of carried-forward, carried-chain-group or carried-record-failures (scripts/agents/land-fixtures.sh:30-34). Implementer return, gaps 4-7.

### HCL-C-62 (low)

Claim: TestHCL39FormatOneReservationAndRecordAsk drives the reservation and record asks with empty arguments. It does not use a bound channel word or show the reservation publishing after --raise-format, as the fold's fixture line describes. The behaviour is still covered: the format check comes before any word validation, so a channel word is refused the same way, and the raise itself is proven by goal-cli carry-word. Recorded, not actioned.

Evidence: internal/goal/hcl_carry_test.go:32-63; internal/goal/verbs.go:3340-3343; scripts/agents/goal-cli-fixtures.sh:379-392.

### HCL-C-63 (low)

Claim: In single-machine mode land.sh never reaches the classification's carry-remote-required. carry-status reports consumption 'missing-anchor:', and land.sh asks 'unsupported consumption state'. Only a channel-bound word can get there, since goal carry refuses in that mode. In remote mode a missing anchor is still called carry-word-consumed (carried.go:210-212), where page 06 says carry-word-missing; that is reachable only through a fetch race. The fold scoped this to the word, the reservation and the classification, so it is recorded, not actioned.

Evidence: internal/goal/verbs.go:2944-2946 (Kind missing-anchor, no error); internal/landing/carried.go:59-70; scripts/agents/land.sh:903.

### HCL-C-64 (low)

Claim: Some bed assertions are weaker than the page. carried-crash counts at least two approvedRef lines, not exactly one carried row and one counselor line. carried-fresh does not check the Carried-Past or Carried-Battery values. The interleave leg crashes seat A and reruns it instead of pausing and resuming it. Each assertion still fails on a tree without the behaviour its leg's name states.

Evidence: scripts/agents/land-fixtures.sh:745-750, 860-866, 755-771.

### HCL-C-65 (low)

Claim: commit.sh's carried-judge EXIT trap replaces the legacy branch's proof-engine cleanup trap. With the testing contract off, that leaks one temporary engine file per carried landing; under the contract nothing is lost.

Evidence: scripts/agents/commit.sh:406 (legacy trap) and 616 (carried judge trap).


## Fold lines and named checks

- {"command": "cat metasystem/artifacts/agents/hcl-build3-20260911/rounds/1/review.json; git archive faccdb3b25ea9fc1d9ca6fa10a87864fe7a2fbfa | tar -x -C $TMPDIR/rt; git archive 01124a60c:metasystem | tar -x -C $TMPDIR/base; diff -rq base rt", "level": "ran", "observed": "review.json names reviewedTree faccdb3b25ea9fc1d9ca6fa10a87864fe7a2fbfa. The reviewed installation tree differs from trunk 01124a60c in 54 paths, three of them new (internal/landing/carried.go, internal/landing/hcl_carried_test.go, internal/goal/hcl_carry_test.go) plus the new empty records/counselor/carried-landings.jsonl. This matches the 54 paths in diff.patch and the implementer's diffBoundary. scripts/agents/sync-transport.sh is not among them."}

- {"command": "cd metasystem && git rev-parse --show-prefix; git show HEAD:plans/goals/human-carried-landing-carry.md; git show HEAD:./plans/goals/human-carried-landing-carry.md", "level": "ran", "observed": "The prefix is metasystem/. From inside metasystem/, 'git show HEAD:plans/goals/...' fails with: fatal: path 'metasystem/plans/goals/human-carried-landing-carry.md' exists, but not 'plans/goals/human-carried-landing-carry.md'. The './plans/goals/...' form works. This is the form land.sh:805 uses, from its installation root (land.sh:13 cd \"$root\")."}

- {"command": "/bin/bash -c 'trap \"echo exit-trap-ran\" EXIT; kill -TERM $$; sleep 2; echo not-killed'", "level": "ran", "observed": "Printed exit-trap-ran, exit status 143. Bash runs its EXIT trap when it is killed by SIGTERM, so land.sh's cleanup trap does fire on a signal."}

- {"command": "head -1 scripts/agents/land.sh; bash --version; /bin/bash -c 'set -uo pipefail; a=(); printf \"[%s]\\n\" \"${a[@]}\"; echo expanded-ok'", "level": "ran", "observed": "land.sh starts with #!/usr/bin/env bash. The host bash is GNU bash 3.2.57, and command -v bash resolves to /bin/bash. Expanding an empty array under set -u fails with 'a[@]: unbound variable', exit status 127."}

- {"command": "cat -n internal/landing/carried.go (reviewed tree)", "level": "read", "observed": "observeCarried order: remote mode required (158-160), ledger tip, goal live, goal held, word row, Carried-By equals the word's actor (181-183), generation zero outside a fixture root (184-186), seat, workspace, carryable name, expiry, consumption, debt, cap, judge fence (232-239), candidatePathPolicy (240-245), then the ordinary observation, verify, and decideCarriedMatch (285-309). evaluator-unavailable is decided before carry-unneeded (287-296). The base-judge owner list (383-395) equals the page's ten directories and five files."}

- {"command": "diff base/internal/landing/observe.go rt/internal/landing/observe.go; sed -n 783,917p rt/internal/landing/observe.go; grep records scripts/agents/path-classes.txt", "level": "read", "observed": "candidatePathPolicy (observe.go:151-185) runs the ledger, runtime and unclassified class checks, then record ownership for Record-class paths. For any records/counselor/ path in carried mode it clears the goal (177-178). recordCarriageError's records/ branch (903-917) refuses an existing record without a held goal as record-not-owned and admits a new file. path-classes.txt:33 classes install:records/ as record."}

- {"command": "diff base/scripts/agents/land.sh rt/scripts/agents/land.sh; sed -n 340,420p and 900-1039p rt/scripts/agents/land.sh", "level": "read", "observed": "run_carried_landing (841-903) fetches, runs goal fetch and carry-status, and branches for superseded, ledger, origin and local before 'stage caller paths' at 905. reserve_carry arms the abandon at 692. create_carried_intent passes --owner-pid $$ (775). print_carried_advisory (789-817) runs before the push, with git show at 805 and the finding line at 815. The abandon is disarmed after the push (832). cleanup (242-247) runs goal carrying --abandon and discards its output. stage_changes refuses unstaged changes (366-386). verify_checks expands \"${pathspecs[@]}\" at 363. The argument check at 172 admits --carried with no pathspecs."}

- {"command": "sed -n 3405-3560,3653-3700,3718-3868 rt/internal/goal/verbs.go; grep -n OwnerAlive -A 18 journal.go", "level": "read", "observed": "AbandonCarrying (3512) refuses with carry-debt-unpaid 'carried intent ... is in flight' when a created carried entry for the word has a live owner (3541). OwnerAlive probes the recorded pid and start identity (journal.go:156-174). carriedRequestMode compares the goal target (3735-3738), then the fourteen fields (compareCarriedRow 3857-3868). A red battery writes finding carried:<sha>:battery-red (3797-3800). done refuses an open or origin-consumed word (1211-1222). CarryWordProven (2840-2842) admits generation zero only for channel rows or a fixture root."}

- {"command": "sed -n 394-460 rt/cmd/metasystem/goalsync_mutations.go; grep historyActor rt/internal/goal/verbs.go", "level": "read", "observed": "syncReqClassified builds Actor{Human: by} from --by (452), and historyActor renders \"human:\" + Human (verbs.go:186-188). land.sh's carried_by comes from carry-status's by field, which is the word's actor, human:<name> (carried.go:50), and land.sh prints it as --by in the supersede lines at 673 and 713."}

- {"command": "diff base/cmd/metasystem/landing_verbs.go rt/cmd/metasystem/landing_verbs.go", "level": "read", "observed": "runLandingObserve parses --judge and --live-failure (42-43) and passes them into ObserveParams (52-57). Nothing checks that --judge base comes with --live-failure."}

- {"command": "diff base/cmd/metasystem/test.go rt/...; diff base/cmd/metasystem/test_test.go rt/...; sed -n 1018-1045 rt/cmd/metasystem/test.go", "level": "read", "observed": "retainedCandidateEngineDigest needs a sufficient successful attempt unless carried is true (test.go:1214, 1232-1233). The ordinary call passes false. The base test case, a later failed attempt with a different digest, is restored (test_test.go:365-385), and a separate carried case follows (386-389). test verify --json prints the result even when it is insufficient, then exits 1 (1027-1043). commit.sh passes --carried only in carried mode (728)."}

- {"command": "grep -rn 'func TestHCL' rt --include='*_test.go'; grep carry-*-standard rt/testing.json; read every TestHCL body", "level": "ran", "observed": "There are 20 TestHCL functions: goal 5, landing 2, cmd 1, refusal 5, config 2, dispatch 3, counselor 2, channel 1, testpolicy 2, steward 1. carry-goal-standard, carry-landing-standard and carry-plumbing-standard list exactly the 5, 3 and 7 names defined (testing.json:37-39). Every function calls the production function its name states, so none is a shell."}

- {"command": "diff base/scripts/agents/land-fixtures.sh rt/...; diff base/scripts/agents/goal-cli-fixtures.sh rt/...; diff base/scripts/agents/dispatch-fixtures.sh rt/...", "level": "read", "observed": "The land bed has seven carried legs: carried-fresh, carried-asks, carried-ledger-path, carried-crash (after-push rerun), carried-two-seat, carried-debt-abandoned, carried-debt-expired. All run land.sh's own reservation and seed the installation at the repository top (467). No leg uses a group word, a base judge, the local: or ledger: branch, a second carry on one seat, or a prefixed installation. goal-cli adds carry-word, carried-record and carried-discharge. dispatch adds the commit-subject scenario."}

- {"command": "grep -c episodeAt base/internal/goal/*.go rt/internal/goal/*.go; diff base rt for verbs.go file.go txn.go validate.go health.go | grep '^<' | grep -i 'elapsed|episode|budget'", "level": "ran", "observed": "episodeAt counts are equal in base and reviewed tree (file.go 14, file_test.go 9, verbs.go 4, verbs_test.go 1). The only removed budget-bearing line is steward/health.go:1027; its replacement adds carried=<k> to the appetite line, as the page says. The elapsed-budget code is untouched."}

- {"command": "diff base/internal/landing/registers.go rt/...; diff base/testing.json rt/...", "level": "read", "observed": "registers.go adds carried-landings.jsonl to appendOnlyRegisters and compacts entries already covered by records/counselor, so the workspace projection's path list is unchanged. testing.json only adds surfaces paths, groups and obligations, and widens gate-plumbing to internal/refusal/**. Nothing is removed or weakened."}

- {"command": "mandate 1 fold line: HCL-C-35", "level": "read", "observed": "Folded as decided. land.sh:841-903 reads carry-status and takes the superseded (848-853), ledger (854-863), origin (864-878) or local (879-894) branch before staging at 905; only a fresh word stages. carried-crash reruns with nothing staged (land-fixtures.sh:718-752). Exception: the no-flag rerun form crashes on bash 3.2 (HCL-C-56)."}

- {"command": "mandate 1 fold line: HCL-C-36", "level": "read", "observed": "Partly folded. The EXIT trap is armed at reserve_carry (land.sh:692), disarmed after the push (832), and abandons in cleanup (242-247). carried-asks checks the closer row after carry-refusal-mismatch (land-fixtures.sh:849-853). Not carried out from the intent entry to the push (HCL-C-52). The carry-unneeded half is not driven, but it uses the same trap path, so it is a duplicate oracle."}

- {"command": "mandate 1 fold line: HCL-C-37", "level": "read", "observed": "Folded as decided. carried.go:285-296 decides evaluator-unavailable before carry-unneeded. TestHCL37EvaluatorUnavailablePrecedesUnneeded (hcl_carried_test.go:11-32) drives the three rows the fold names."}

- {"command": "mandate 1 fold line: HCL-C-38", "level": "read", "observed": "Ledger, runtime, unclassified and record checks are folded: they run before the match in every declaration mode (carried.go:240-245, observe.go:151-185), and the ledger-path leg is real (land-fixtures.sh:663-666, 836-853). The counselor half is not as decided: the wrapper's own line refuses (HCL-C-51), and the counselor leg was not built."}

- {"command": "mandate 1 fold line: HCL-C-39", "level": "read", "observed": "Folded. The reservation asks carry-format-required (verbs.go:3341-3343), the record asks at Carried (3659-3662) and at the rebuilt record (3732-3734), and ValidateTree flags carrying and carried rows under format 1 (validate.go:157). The fixture is partial (HCL-C-62, not material)."}

- {"command": "mandate 1 fold line: HCL-C-40", "level": "read", "observed": "Folded. The classification asks carry-remote-required at carried.go:158-160; Carry, Carrying, Carried and CarriedFromCommit ask it first (verbs.go:3179 and each verb's opening lines); the Question row is at register.go:130. The goal-cli carry-word scenario drives it (goal-cli-fixtures.sh:367-378), and TestHCL40 drives the three goal verbs. Wrapper note in HCL-C-63 (not material)."}

- {"command": "mandate 1 fold line: HCL-C-41", "level": "read", "observed": "Folded by deletion. The three shared shell helpers are gone, and each remaining TestHCL function drives its production path. No TestHCL function is still a shell. As a result most of the page's Go fixtures do not exist (HCL-C-57 to HCL-C-61)."}

- {"command": "mandate 1 fold line: HCL-C-42", "level": "read", "observed": "Partly folded. The carried legs now drive land.sh's own reservation, and five real legs were added. The shell legs carried-chain-group, carried-forward and carried-record-failures were deleted, not built. No remaining leg would pass on a tree without the behaviour its name states; weaker-than-page assertions are in HCL-C-64. Most of the page's legs are absent (HCL-C-57 to HCL-C-61)."}

- {"command": "mandate 1 fold line: HCL-C-43", "level": "read", "observed": "Folded as decided. register_test.go:302-369 reads carry-status from main.go's landing table (main.go:134) and anchors \"\\n    --carried)\\n\" to commit.sh's argument loop. Its negative table removes each needle from a copy and fails if the check still passes."}

- {"command": "mandate 1 fold line: HCL-C-44", "level": "read", "observed": "Folded in shape but defective. The eight lines print before the push (land.sh:789-817, called at 821), and carried-fresh checks their order before the push step (land-fixtures.sh:886-898). They fail under a prefixed installation (HCL-C-50) and name the wrong finding for a red battery (HCL-C-53)."}

- {"command": "mandate 1 fold line: HCL-C-45", "level": "read", "observed": "(a) partly: the channel question is posted (land.sh:668-672) but the printed --by value is wrong (HCL-C-54). (b) folded: goalsync_mutations.go:224-227 exits 2. (c) folded: verbs.go:3669-3671, TestHCL45. (d) folded: run_carried_landing turns a failed fetch into an exit-3 ask. (a), (b) and (d) are not driven (HCL-C-61)."}

- {"command": "mandate 1 fold line: HCL-C-46, HCL-C-48, HCL-C-49, counselor-file gap", "level": "read", "observed": "HCL-C-46 folded as decided (see the test.go evidence). HCL-C-48 folded: ParseCarryWord reads keyed fields outside the quoted why, and TestHCL48 (hcl_carry_test.go:81-92) proves it. HCL-C-49 folded in code (carried.go:181-183) with no test; the check is fold-only, so that is not material. Counselor-file gap partly folded: the file is created, tracked and in the projection list (registers.go:11), but the carried classification refuses the wrapper's own line (HCL-C-51)."}

- {"command": "mandate 2 traces", "level": "read", "observed": "(1) A chainless candidate with plans/goals/illicit.md staged under a missing-declaration word passes steps 1 to 11, then candidatePathPolicy returns ledger-path-not-goal-verb in refuse mode; commit.sh exits 3 and the trap closes the row. (2) A records/counselor line: an existing file always refuses record-not-owned because the goal is cleared, including the wrapper's own carried-landings.jsonl line (HCL-C-51); a new file under records/counselor/ is admitted. (3) missing-declaration plus a red group G: a word naming missing-declaration mismatches because R is insufficient, and a word naming group:G mismatches because O is missing-declaration; the ask names O, the group, U, D and the word (carried.go:261-263)."}

- {"command": "mandate 6 identity gate and record failures", "level": "read", "observed": "goal carry proves with humanauthority.Prove only (goalsync_mutations.go:101, proveEnrolledGoalHumanAuthority) and exits 2 on --temporary-human-word or --review-by ('carry takes no relayed word'). set-budget uses ProveOrTemporaryGoalAuthority (1336-1338), which refuses the relay once the fleet has enrolled; that is the page's stated equivalence (design 02). The fixture proof goes only through fixtureauth (963-977). A typed carried trailer exits 1 before any commit, the postcondition requires each of the seven carried trailers exactly once, and commit.sh's record-failure checks run before observe, unchanged."}

- {"command": "mandate 8 four named checks", "level": "read", "observed": "HCL-C-33: the interleave and trailer-without-row legs (abandoned, expired) exist and drive land.sh's own reservation (land-fixtures.sh:755-834). They assert the in-flight ask names A's row and seat, the third form names A's word, B pushes no commit and writes no reservation, A's rerun completes, and B's rerun names A's obligation. They were not run here, and seat A crashes and reruns instead of pausing and resuming. HCL-C-25: the code checks the goal target, then the fourteen fields, but nothing drives it (HCL-C-60). HCL-C-03: the fence list equals the page, but nothing drives it (HCL-C-58). HCL-C-34: the named lists equal the defined functions, and TestHCL34 selects each owner, checks set inclusion, drives a negative probe and the three-file rule (contract_test.go:177-235). Execution is real for what exists."}

- {"command": "verdict", "level": "inferred", "observed": "Not closable as is. One fold: HCL-C-50, HCL-C-51, HCL-C-52 (defects; the first blocks every carried landing in this repository); HCL-C-53, HCL-C-54, HCL-C-55, HCL-C-56 (small defects); HCL-C-57 to HCL-C-61 (page behaviours with no proof: build the named fixtures, or record each gap as a review obligation or accepted risk)."}

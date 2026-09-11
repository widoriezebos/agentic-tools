# Closing read of the carried landing, round 1 (Opus, hcl-cc1-20260911)

Chain hcl-build2-20260911, round 1, reviewed tree b38cf7977ec3ce21a5f04be0fa686dec9b27730a (installation subtree). Design: plans/human-carried-landing-carry-design.md revision 6. 15 findings, 11 material.

## Gaps the critic named

- The role is read-only, so metasystem/records/misc/human-carried-landing-carry-code-read-r1.md was not written. This return is the register for the coordinator to project.

- No bed, test, build or live proof was run, as the brief asked. Every finding comes from reading the reviewed tree extracted with git archive, plus text searches and one name-comparison script.

- dispatch.sh close on a commit subject was traced by reading only. The dispatch bed dispatches two commit-subject critics but closes neither, so closing such a chain is unproven.

- How the counselor line reaches origin, a gap the third design read left open: goal carried appends records/counselor/carried-landings.jsonl in the checkout. The file is not tracked, git does not ignore it, and neither landing-classes.json nor landing-promotion.json names it. While it is untracked, land.sh's staging refuses that seat's next landing of any kind (land.sh:373-377) unless it is staged. The accepted-risk register already takes the same unnamed route today.

- Shell refusal register: the ShellRows line numbers for the new asks are a few lines off at the reviewed tree. Several new exit-3 asks have no row, for example the unsupported-consumption ask at land.sh:845, the workspace ask at land.sh:656, the incomplete status and reservation asks, and the binding ask at commit.sh:720. The page accepts that this list is kept by hand.

- The single-machine finding (HCL-C-40) rests on reading txn.go's LocalLedgerBranch and the page's own fact that the ledger is a separate branch in that mode. It was not reproduced.

- Three findings need page amendments: the stop-list exception (HCL-C-35), the order of the evaluator-unavailable and unneeded cases (HCL-C-37), and the local-mode anchor (HCL-C-40). The goal's three design reads are spent, so the coordinator decides how the fold amends the page.

- The provider tool catalog and context isolation are unobserved, so this critique is advisory about runtime independence.


## Findings

### HCL-C-35 (critical, material)

Claim: Every recovery rerun of land.sh --carried is refused before it can branch. The wrapper stages the caller's paths before the carried sequence, and staging refuses an empty staged set. After a crash nothing is staged. So the local:<sha> rerun (carried commit at HEAD), the origin:<sha> rerun (commit pushed, no carried row), the ledger: replay and the counselor repair all stop with 'the caller-selected staging set is empty' (exit 2) and never read landing carry-status. The origin: case is the interval between the push and the carried row. In that interval every other seat's carry refuses carry-debt-unpaid and names land.sh --carried <w> as the closer, and that closer cannot run; only a hand-typed goal carried --rebuild-from-commit completes the record. Design 05 (the push and recovery) and 06 (the carry-status branches) require these reruns through the wrapper, as do fixtures HCL-06-CRASH-BEFORE-PUSH-RESUMES, HCL-06-CRASH-AFTER-PUSH-COMPLETES-RECORD, HCL-06-REPLAY-REFUSED, HCL-19-RECOVERY-KEEPS-REBASED-COMMIT and HCL-26-COUNSELOR-REPAIRED. The page's stop list in 05 also names an empty staging set as a stop in every mode, which contradicts its own reruns. If this stands: land.sh reads carry-status and branches before staging (or exempts reruns from the empty-set rule), the page's stop list gains that exception, and carried-crash drives each rerun.


Evidence: scripts/agents/land.sh:877 runs the required step 'stage caller paths' (stage_changes); the carried branch comes after it at land.sh:893-896. stage_changes returns 2 when nothing is staged (land.sh:365-367) and refuses unstaged or untracked paths (369-377). The unrecorded-debt ask names land.sh --carried as its closer at internal/landing/carried.go:273. No carried land leg reruns the wrapper (land-fixtures.sh:584-676).

### HCL-C-36 (high, material)

Claim: A classification ask after the reservation leaves the reservation open. land.sh publishes the carrying row, then runs commit.sh as a required step. When commit.sh asks (carry-unneeded, carry-refusal-mismatch, carry-battery-unverified, carry-base-judge-blind, no judge decided, test verify failed, or the binding ask), the step fails and the script exits without goal carrying --abandon. Only the two asks inside the second carry-forward release the row. Design 08, 'Who closes it', says the wrapper runs --abandon itself on every ask after the row and before the push. Consequence: one mistaken word, such as one naming a refusal the landing does not hit, leaves an open row. Every other seat counts that row as in-flight debt (carry-debt-unpaid) until the word expires, up to four hours, and no other seat can release it. If this stands: land.sh abandons its own row on every exit between the reservation and the push, and carried-asks checks the closing row after carry-unneeded and carry-refusal-mismatch.


Evidence: land.sh:848 reserve_carry, 851 the second carry-forward (which releases on its own asks), 853 run_required_step 'commit' commit_changes. run_required_step calls fail_step, which exits (land.sh:263-279); the EXIT trap cleanup only deletes temp files (land.sh:242-246). The only releases are at land.sh:641-644 and 652-655 inside carry_forward_staged. commit.sh exits 3 on every carried refusal at commit.sh:640-647, and on the judge, verify and binding asks at 618, 627, 736 and 720. The in-flight debt check is internal/goal/verbs.go:3014-3036 (openCarryingDebt).

### HCL-C-37 (high, material)

Claim: A word naming evaluator-unavailable can never land. The classification returns carry-unneeded whenever the ordinary verdict passes and the testing result is sufficient, before it looks at the word. The evaluator-unavailable branch requires exactly that state (base judge, a recorded live failure, ordinary pass, sufficient result), so it is unreachable. Design 05 step 14 and 'The evaluator' say evaluator-unavailable is carried in exactly that case, and the register keeps its override land.sh --carried. Consequence: when the live engine is dead and the base judge passes the tree, the human names the one refusal that occurred and is told 'the refusal you named did not occur'. Ordinary mode refuses evaluator-unavailable, so no lawful path remains. The page's two bullets (the evaluator-unavailable case and the unneeded case) overlap, so the fold should state that evaluator-unavailable is decided first. If this stands: the match decides the evaluator-unavailable case before carry-unneeded, and a table test drives it.


Evidence: internal/landing/carried.go:240-241 returns carry-unneeded when the ordinary verdict is pass and result.Delivery.Sufficient is true. carried.go:248-249 sets matched only when the judge is base, a live failure is recorded, the ordinary verdict is pass and the result is sufficient. internal/refusal/register.go keeps evaluator-unavailable with the override land.sh --carried. TestHCL03LiveFailureStamped is a one-line call to the shared helper (internal/landing/hcl_carried_test.go:10-59).

### HCL-C-38 (critical, material)

Claim: A chainless carry lands ledger and record changes that neither the word nor any check sees. With no --chain and no --direct-fix, the ordinary observer returns missing-declaration before any path class or record ownership is checked, and the carried classification adds no such check. The word binds only the workspace projection, which removes plans/goals, plans/goals.md, plans/goals-accepted.json, records/counselor and records/goals. The pre-commit guard stops only plans/goals/ and plans/channel/. So a staged change to records/goals/**, plans/goals.md or plans/goals-accepted.json (all ledger class), or to records/counselor/** (for example a written accepted-risk register line), lands under a word naming missing-declaration. The word's workspace id is the same with or without those bytes. The same early return hides a second carryable refusal (path-unclassified or runtime-path-refused) behind the named one, so a landing with two defects lands. Design: the opening paragraph says a ledger-meaning check (a record not owned, a ledger path changed outside a goal verb) is never carried; 05 step 6 says 'a candidate that touches a ledger path is ledger-path-not-goal-verb, never carried'; 05 step 14 says one word carries one defect. If this stands: the carried classification runs the path-class and record-ownership checks over the candidate's changed paths whatever the declaration (or refuses any change under a path the projection removes), and a land leg stages a ledger path beside a chainless payload and expects the refusal.


Evidence: internal/landing/observe.go:119-121 returns missing-declaration when Chain and DirectFix are empty. ledger-path-not-goal-verb, runtime-path-refused and path-unclassified are raised only at observe.go:744-760, and record-not-owned only at 818-878, all inside the chain and direct-fix paths. internal/landing/carried.go:146-264 has no path or record check and compares only the projection (carried.go:180-183). internal/landing/registers.go:10-18 lists the removed paths. scripts/agents/pre-commit-guard.sh:80 matches only (^|/)plans/(goals|channel)/. scripts/agents/path-classes.txt:35-39 marks plans/goals/, plans/channel/, plans/goals.md, plans/goals-accepted.json and records/goals/ as ledger. commit.sh's agent refusal block (commit.sh:648) runs only in refuse mode, which a passing carried observation is not.

### HCL-C-39 (high, material)

Claim: The format fence can be bypassed through a channel word, and every seat that has not rebuilt then stops advancing its ledger. goal carry refuses a format-1 ledger without --raise-format, but a channel word is an answer row that format 1 already accepts, and nothing on the channel path checks the format. The reservation (goal carrying) and the record (goal carried) publish rows carrying approvedRef= on a format-1 ledger, and the new format-1 rule in ValidateTree flags only HUMAN_AUTHORITY_PROVEN. An engine at ab513438b rejects approvedRef= on every verb but resume and set-obligation. So the first carrying row makes the whole tree a parse problem for that engine: its goal fetch refuses to advance, its writes are rejected, and its steward keeps the stale tree. Design 02's fence (HCL-C-27) exists to prevent exactly this: 'the first such row would break every seat that has not rebuilt'. Mandate 5 asks what an ab513438b engine does with the new rows; this is what it does. If this stands: goal carrying and goal carried refuse format 1 with the raise-format ask (or ValidateTree under format 1 refuses carrying and carried rows), and a fixture drives a channel word on a format-1 ledger through the reservation.


Evidence: internal/goal/verbs.go:3144 asks carry-format-required only inside goal carry's mutation. validateCarryReservation (verbs.go:3254-3312), carryingRequest (verbs.go:3390-3416) and carriedRequestMode (verbs.go:3599-3696) never read FormatVersion. internal/goal/validate.go:150-165 adds a format-1 problem only for HUMAN_AUTHORITY_PROVEN. At the base, internal/goal/file.go:1466 returns 'approvedRef= is only valid on resume and set-obligation history'; the reviewed tree widens that rule at file.go:1486. channel poll binds a carry answer with no format check (the internal/channel/poll.go change).

### HCL-C-40 (medium, material)

Claim: In single-machine mode no carry can land, and goal done archives a goal that still has an open word. To decide consumption, the code looks on the code tip for the word's anchor: the ledger commit whose Goal-Transaction trailer equals the word's opid. In single-machine mode the code tip is refs/heads/main, but ledger commits live only on refs/heads/metasystem/goals, so the anchor is never found and every word without a carried row reads missing-anchor. Consequences: 
  - The classification refuses carry-word-consumed ('consumed at missing-anchor:'), where design 06 says a missing anchor is carry-word-missing with 'fetch the ledger'.
  - The reservation refuses the same way.
  - land.sh asks about an 'unsupported consumption state'.
  - goal done treats the word as closed, because it refuses only origin, or none while unexpired.
  - The cap counts the word as open. The land beds switch the carried legs to remote mode, so nothing exercises this. The page's own anchor rule (06, Consumption) names refs/heads/main for local mode, which cannot hold the anchor, so this is also a specification gap. The fold must either anchor on the ledger branch in local mode or declare local mode out of scope with its own ask. If this stands, either the code-tip choice, the classification code and the done predicate change, or local mode gets a named refusal and a fixture.


Evidence: internal/goal/txn.go:35 LocalLedgerBranch = refs/heads/metasystem/goals. internal/landing/carried.go:190-193 and verbs.go:2925-2930 (carryCodeTip) choose refs/heads/main in local mode. verbs.go:2833-2840 (carryAnchorAndCommit) walks that tip for Goal-Transaction, and verbs.go:2867 returns missing-anchor when it finds none. carried.go:198-200 turns any consumption other than none into carry-word-consumed. land.sh:845 asks about an unsupported consumption state. verbs.go:1171 refuses done only for origin, or for none while unexpired. openCarryWords counts missing-anchor as open. scripts/agents/land-fixtures.sh:571-576 sets goal.sync-remote to origin for the carried legs.

### HCL-C-41 (critical, material)

Claim: The named Go fixtures do not test what they are named for. 58 TestHCL functions are one-line calls into three shared helpers:
  - The goal helper round-trips a hand-built word, reservation and carried row through the parser and the field comparison.
  - The landing helper checks the refusal struct's shape, a sorted set union and the fence prefix list. It never calls observeCarried, CarryDebtAt, CarryConsumptionAt or ReadCarryStatus.
  - The command helper checks the temporary-word and expiry flags, two channel-ask flags and a finding parse.As a result:
  - TestHCL21TwoFailuresOneName, the other four TestHCL21 tests, TestHCL05WrongRefusalAsks and TestHCL05UnneededAsks never run the match.
  - TestHCL02GenerationZeroRefusedInProduction builds no generation-zero word.
  - TestHCL33ExpiredReservationNotDebt, the unit oracle behind HCL-C-33, never runs the debt scan.
  - TestHCL25ReplayMismatchRefused never drives the goal-target case of HCL-C-25, and changes only one of the fourteen fields.
  - The TestHCL30 supersede tests, the TestHCL18 recovery tests, TestHCL26CrashBeforeHookRecovers, TestHCL06DoneRefusesOpenCarry, TestHCL07DischargeByCritic, TestHCL27FormatRaiseAsksThenWrites and TestHCL18OwnerMustBeAncestor never call the verb they name.
  - TestHCL03BaseJudgeBlindAsks never produces carry-base-judge-blind.The three carry groups list these names, and TestHCL34 only checks that they are listed. So the plan 'executes every fixture' while no fixture proves its oracle, and the implementer's green run of every named test is not evidence for the behaviours. HCL-C-35 to HCL-C-40 all went unnoticed through this gap. Design: 'Fixtures of this revision' (each fixture with its oracle) and the contract section's naming rule; mandate 11. If this stands, each named function drives the production path and asserts the oracle its design line states.


Evidence: internal/goal/hcl_carry_test.go:21-112: assertHCLGoalContract plus 30 one-line tests; only TestHCL27OldReaderRefusesFormatTwo has its own body. internal/landing/hcl_carried_test.go:10-59: assertHCLLandingContract plus 12 one-line tests. cmd/metasystem/goalsync_mutations_test.go:1253-1305: assertHCLCommandContract plus 16 one-line tests. testing.json:35-37 lists the names; internal/testpolicy/contract_test.go:65-128 checks only that they are listed.

### HCL-C-42 (critical, material)

Claim: The land bed's six carried legs test two asks and one landing, and the two-seat fixture HCL-C-33 requires does not exist.
  - carried-asks runs only the main-branch ask and exits.
  - carried-record-failures runs only a typed Carry: trailer.
  - carried-fresh, carried-chain-group, carried-forward and carried-crash all run the same chainless missing-declaration landing with a green battery, under a reservation the bed publishes itself before land.sh runs. land.sh's own reservation transaction therefore never runs.Absent from the land bed:
  - HCL-33-TWO-SEATS-INTERLEAVE, and both the abandonment and expiry legs of HCL-33-TRAILER-WITHOUT-ROW-IS-DEBT.
  - HCL-28-PEER-DEBT-SEEN.
  - Every group word, including HCL-05-RED-BATTERY-LANDS and HCL-05-CHAIN-PLUS-GROUP.
  - The HCL-21 asks, HCL-05-UNNEEDED-ASKS, the dead-live-judge and no-judge legs, HCL-23, and the cap and debt asks at the landing.
  - The forward cases: a second word, a ledger move, the rebase posture, the release on a second ask, and the lease held throughout.
  - HCL-06-ORDER's crash points and every carried-crash rerun.
  - All but one item of the record-failure list.The goal-cli scenarios also omit the transfer, the in-flight and obligation debt asks at the word, the push-before-row refusal, the fourteen-field and goal-target replay refusals, abandon, done, recover, the confirmed leg, the critic discharge, and the accept-risk replay and changed-why. The implementer's '19 isolated legs green' therefore certifies one landing. Mandate 1 (HCL-C-33's two-seat fixture) and mandate 11. If this stands, each leg and scenario drives what its design lines name, the two HCL-33 two-seat legs included.


Evidence: scripts/agents/land-fixtures.sh:584-676 is the only carried body: carried-asks at 620-632 (the topic-branch ask), carried-record-failures at 634-646 (the typed Carry trailer), and one shared landing for the other four at 648-676. The bed runs goal carrying itself at 605-609. The carry-word, carried-record and carried-discharge scenarios in scripts/agents/goal-cli-fixtures.sh cover only the format ask and raise, the word lines, the cap ask, one supersede, one rebuilt record and its replay, and one accept-risk.

### HCL-C-43 (medium, material)

Claim: TestHCL03NoPendingAfterSlice2 does not check the entry points the page names, and its negative check cannot fail. The 'negative entry-point oracle' removes the needle once, then errors only if the copy still holds it while the source held exactly one. That can never happen, and the entry-point check is never run on the copy. So the page's negative table (remove each entry point from a copy of the source and expect the named failure) is missing. The test looks for carry-status in landing_verbs.go, where it appears only in a usage string, rather than in main.go's landing verb table; deleting carry-status from that table leaves the test green. The --carried) needle also matches commit.sh's landing-detection loop, so deleting the argument-loop case the page names leaves it green too. Design: the register-flip section and its fixture line ('driven negatively by a table that removes each entry point from a copy of the source'); mandate 9. If this stands: read carry-status from main.go's landing table, anchor --carried) to the argument loop, and drive the negative table through the same check.


Evidence: internal/refusal/register_test.go:343 checks for carry-status in cmd/metasystem/landing_verbs.go. register_test.go:354-357 builds the copy with strings.Replace(source, needle, '', 1) and errors only if the copy still contains the needle while strings.Count(source, needle) == 1. scripts/agents/commit.sh:14 has '--carried) landing_requested=1' in the detection loop, separate from the argument-loop case.

### HCL-C-44 (low, material)

Claim: The landing never tells the human what it is about to stamp. Design 08 ('What the machine says, then does') and HCL-08-LANDING-LINES-THEN-ACT require these lines before the push: the reservation opid and ledger tip L; the judge, with the live failure for the base judge; the ordinary verdict; the testing result with its four lists; the obligation finding; and the exception count after this one. land.sh prints only step names and captures each step's output, and commit.sh prints no such line. goal carry also prints its word lines after the row is published, where 08 has the machine speak first and then act. Consequence: a red battery or a base-judge landing is pushed without the lines the page promises. If this stands, land.sh or commit.sh prints these lines before the push step, and carried-fresh checks their order.


Evidence: scripts/agents/land.sh:249-261: run_step prints '== STEP: <name>' and sends the command's output to a temp file. A search of commit.sh's echo and printf lines for battery, judge, finding, exception, obligation or reservation matches only error lines at commit.sh:618, 627 and 736. cmd/metasystem/goalsync_mutations.go:130-162 prints the word lines after goal.Carry returns confirmed.

### HCL-C-45 (low, material)

Claim: Four asks or exit codes differ from the page:
  - (a) When origin moved code, the workspace ask always prints a goal carry --supersede line and never posts the --kind carry question for a channel word (design 05 step 4). The printed command also leaves out the required --by and --why, so running it as printed exits 2.
  - (b) goal carrying --owner-pid with a live process that is not an ancestor exits 1, where HCL-18-OWNER-MUST-BE-ANCESTOR says exit 2.
  - (c) A second goal carried --entry on a confirmed entry exits 1 ('journal entry ... is terminal'), where HCL-06-CARRIED-IDEMPOTENT says it returns confirmed with detail idempotent.
  - (d) A failed code fetch at the start of the carried sequence is a step failure, where 05 step 1 makes it an exit-3 ask.If this stands, each follows the page, or the page is amended.


Evidence: (a) scripts/agents/land.sh:652-656 holds the workspace ask text; cmd/metasystem/goalsync_mutations.go:62-65 makes --by and --why required for goal carry. (b) internal/goal/journal.go:288-289 returns a plain error, which printCarryMutation turns into exit 1 (goalsync_mutations.go:33-52). (c) journal.go:322-323 refuses a terminal entry before any idempotent path. (d) land.sh:788 runs the fetch as a required step.

### HCL-C-46 (low)

Claim: Outside the page's file list, cmd/metasystem/test.go now recovers the candidate engine digest from the newest completed attempt, red ones included; before, it needed a sufficient successful attempt. The old test checked that a later failed attempt with a different digest does not hide the sufficient digest; it was rewritten with fixture data that avoids that case. For ordinary landings this seems to fail closed: a later red attempt with a different digest now points verify at the red result. The change is also what lets test verify return a structured red result for a group word. Recorded so the coordinator can confirm the ordinary delivery gate is unchanged.


Evidence: cmd/metasystem/test.go:1206-1232: the Delivery.Sufficient condition is removed and TerminalFailed attempts are accepted. cmd/metasystem/test_test.go:335-384: the test is renamed and the failed attempt's digest is set equal to the candidate digest.

### HCL-C-47 (low)

Claim: scripts/agents/path-classes.txt now classifies testing.json as behaviour; at the base it had no class. This changes landing policy outside the page's file list, and the file is one of the inputs the base judge fences. It should be recorded as intended.


Evidence: scripts/agents/path-classes.txt:7 adds 'install:testing.json behavior'; a grep of the base path-classes.txt finds no testing.json entry.

### HCL-C-48 (low)

Claim: ParseCarryWord searches the whole reason, including the quoted why, for supersedes=. The real field comes last, so a why containing ' supersedes=<opid>' is read first, and every reader then treats the new word as superseding that opid, with no superseded row written. The fields should be parsed outside the quoted why.


Evidence: internal/goal/verbs.go:2693 runs a regular expression for (^|\s)supersedes= over the whole history reason. The Carry mutation writes why=<quoted> before supersedes= in the row's reason.

### HCL-C-49 (low)

Claim: commit.sh stamps the Carried-By trailer from the value its caller passes in --carried-by, and the observer never checks that value against the word's actor; only the opid, the refusal name and the ledger tip are checked. land.sh passes the word's actor, so the wrapper path is correct. A direct commit.sh --carried call, though, can put another name on the commit, and from there on the carried row and the counselor line.


Evidence: scripts/agents/commit.sh:716-720 checks the provenance for opid, past and ledger only. commit.sh:780-795 stamps Carried-By from landing_carried_by. internal/landing/carried.go:256-260 builds the provenance without the actor.


## The four named checks

- {"command": "git archive b38cf7977ec3ce21a5f04be0fa686dec9b27730a | tar -x; git archive ab513438be2b0aef00d189b25ed9db9a17d64f01:metasystem | tar -x; diff -rq base rt", "level": "ran", "observed": "The reviewed tree differs from the base in 50 files and adds 3 (internal/landing/carried.go, internal/landing/hcl_carried_test.go, internal/goal/hcl_carry_test.go), matching the 53-path diff.patch and the reviewedTree in review.json."}

- {"command": "rg for func TestHCL names in every _test.go of the reviewed tree, then a python comparison against the carry groups in testing.json", "level": "ran", "observed": "79 TestHCL functions exist. None of the 31, 29 and 7 names listed in carry-goal-standard, carry-landing-standard and carry-plumbing-standard is missing."}

- {"command": "cat internal/goal/hcl_carry_test.go internal/landing/hcl_carried_test.go; read the cmd/metasystem/goalsync_mutations_test.go diff", "level": "read", "observed": "30 of 31 goal tests, all 12 landing tests and all 16 new command tests are one-line calls to three shared helpers. None of the helpers calls observeCarried, CarryDebtAt, Carry, Carrying, Carried, recovery or done."}

- {"command": "sed -n 856,935p scripts/agents/land.sh; read stage_changes at land.sh:360-378", "level": "read", "observed": "The staging step runs at land.sh:877, before the carried branch at 893-896. Staging refuses an empty staged set at 365-367 and unstaged or untracked paths at 369-377."}

- {"command": "read land.sh cleanup, run_step, fail_step and run_required_step (242-279) and run_carried_landing (786-857)", "level": "read", "observed": "commit.sh runs as a required step after reserve_carry. fail_step exits, and the EXIT trap only deletes temp files. goal carrying --abandon is called only inside carry_forward_staged (641-644, 652-655)."}

- {"command": "read internal/landing/carried.go:146-264", "level": "read", "observed": "carry-unneeded at 240-241 comes before the evaluator-unavailable branch at 248-249, and that branch requires the same ordinary pass and sufficient result."}

- {"command": "sed -n 96,140p internal/landing/observe.go; rg the ledger-path, record-not-owned and path-unclassified sites; read registers.go:1-55, pre-commit-guard.sh:60-100 and the ledger lines of path-classes.txt", "level": "read", "observed": "A chainless candidate returns missing-declaration at observe.go:119-121, before the path-class and record checks at 744-878. The workspace projection drops plans/goals, plans/goals.md, plans/goals-accepted.json, records/counselor and records/goals. The pre-commit guard matches only plans/goals/ and plans/channel/. path-classes.txt:35-39 marks records/goals/, plans/goals.md and plans/goals-accepted.json as ledger."}

- {"command": "grep the approvedRef rule in the base internal/goal/file.go; read validateCarryReservation, carryingRequest, carriedRequestMode and the new ValidateTree format rule", "level": "read", "observed": "Base file.go:1466 rejects approvedRef= on every verb but resume and set-obligation. The reservation and carried transactions never read FormatVersion. validate.go:150-165 flags only HUMAN_AUTHORITY_PROVEN under format 1."}

- {"command": "read internal/goal/txn.go:20-60 and verbs.go carryAnchorAndCommit, CarryConsumptionAt, carryCodeTip and doneRequest", "level": "read", "observed": "LocalLedgerBranch is refs/heads/metasystem/goals (txn.go:35). In local mode the Goal-Transaction anchor is searched on refs/heads/main, which yields missing-anchor (verbs.go:2867). goal done refuses only origin, or none while unexpired (verbs.go:1171)."}

- {"command": "read the scripts/agents/land-fixtures.sh, goal-cli-fixtures.sh and dispatch-fixtures.sh diffs", "level": "read", "observed": "The six carried land legs share one body. carried-asks checks only the main-branch ask, and carried-record-failures only a typed Carry trailer. The other four run one chainless green landing under a reservation the bed makes itself. No two-seat, crash, rerun, group-word, judge or debt leg exists."}

- {"command": "read internal/refusal/register_test.go:298-390 and the register.go diff", "level": "read", "observed": "The negative-oracle condition is impossible: the copy must still hold the needle while the source held exactly one. carry-status is looked for in landing_verbs.go, not in main.go's landing table. 45 carried overrides, 3 changed overrides and 15 Question rows are present."}

- {"command": "rg echo and printf lines in land.sh and commit.sh for reservation, judge, battery, finding, exception and obligation", "level": "ran", "observed": "No pre-push report lines exist; commit.sh matches only error lines at 618, 627 and 736."}

- {"command": "git ls-files metasystem/records/counselor; git check-ignore -v metasystem/records/counselor/carried-landings.jsonl", "level": "ran", "observed": "accepted-risk-register.jsonl and misclassification-register.jsonl are tracked; carried-landings.jsonl is not ignored."}

- {"command": "read the surfaces at testing.json:5-8 and the groups at 35-37; read internal/testpolicy/contract_test.go:39-128", "level": "read", "observed": "carry-landing-standard is a standard group, and carried-landing-landing a critical obligation, of proof-and-landing, dispatch-goal-mission and human-authority-and-channel. TestHCL34 checks for each owner path that the names are listed."}

- {"command": "read internal/goal/verbs.go carriedRequestMode, compareCarriedRow and CarryDebtAt; read internal/landing/carried.go baseJudgeOwns", "level": "read", "observed": "The row's goal is compared with the intent's before AlreadyApplied at verbs.go:3613-3620. The third debt arm at 3059-3073 reads no expiry or reservation state. The fence at carried.go:338-350 includes governance, humanauthority and fixtureauth."}

- {"command": "read scripts/agents/dispatch.sh:2656-2715, internal/dispatch/close.go:15-90 and the finding_register.go diff", "level": "read", "observed": "close runs job critique-register-close and job close-check, and neither looks up an implementer for a critic root. ReconcileReviewReference refuses commit subjects but runs only with --reconcile-evidence."}

- {"command": "read the cmd/metasystem/test.go:1206-1232 and test_test.go:335-384 changes", "level": "read", "observed": "The candidate engine digest is now recovered from completed red attempts. The old test's different-digest case was replaced by equal-digest fixture data."}

# conclusion-vs-compaction build — the closing read (chain bolboc-build1, round 1)

Chain: build round bolboc-build1 (implementer, codex gpt-5.6-sol, reviewedTree 55e0fe87e2c8ac3cdc071ab60ef61e2764ab8171, base 40924132, diff sha256 31da3c0c5b77d8caf51eff6ac00faa2b53ce33b4869630eb06754402940014da) -> critic bolboc-crit1 (code-critic, fresh context, claude-opus-5, xhigh; the harness observed claude-opus-5 and the return claimed nothing different; reviewedTree in the return equals the round record). 3 findings, 0 material. The coordinator carried the return here verbatim.

## BCE-01 — low, material=False

CLAIM: The new seat test proves the refusal line only where it is written, not where a human would read it. It calls the idle-enforcement function directly and checks the text there; the three whole-turn calls that follow assert only that nothing blocks and no continuation or incident hook fires, so nothing pins that the line survives into the verdict a seat actually sees.

EVIDENCE: I traced the path myself: after idle enforcement the turn verdict reassigns its display through the composer at turnverdict.go:721, which keeps the existing display text as the middle part of the joined result. The line therefore does survive, and no defect ships. This is a note about the proof, not about the behaviour, and the design's fixture list did not ask for the end-to-end assertion.

## BCE-02 — low, material=False

CLAIM: The two human surfaces treat the gate's text differently. The channel cuts the cause at its first semicolon, but the orientation line of the goal command prints the whole cause, which for a budget over the tier box is about two hundred and fifty characters carrying both the numbers and the full remedy sentence.

EVIDENCE: The refusal text in norm.go builds one long sentence containing a semicolon before the remedy clause. The channel cuts there deliberately; the command does not, and the design says so in as many words: the cause is the first refusal's text verbatim, and orientation stays one line. It is one line, since none of the four gate refusals contains a newline. So this is the accepted design, recorded only because the asymmetry will surprise a reader who sees both surfaces.

## BCE-03 — low, material=False

CLAIM: A backlog whose only work is refused still reads as an empty backlog to the steward. Its shared-backlog classifier counts queued goals and claimed goals and otherwise answers that there is no claim held and no declaration; it never mentions the refusal, so the machine-to-machine surface stays silent while the two human surfaces now explain the cause.

EVIDENCE: Read of internal/steward/openwork.go: the classifier reads only in-flight work, claimable goals, claimed goals, the goal-free declaration and the queued count. Section 2.3 of the design page decided this explicitly, leaving the steward unchanged on the grounds that a refusal is a reading of configuration against a record rather than ledger movement, and routing the human's view to the channel and the goal command. True, but outside this page's declared change list.

## Gaps the critic named

- I could not run the two premise pins in the writer's own test against the untouched production code, because the same test file also carries the new refused-only frontier subtest, which needs the new field and so will not compile there. Rather than edit a test to make it run, I ran all three pins on the reviewed tree, where every writer source file is byte-identical to the base commit; that proves the same writer facts. The builder's own evidence covers the untouched-tree run.
- The builder's fail-before and premise-pin evidence is narrated prose that quotes the exact value lines rather than pasted test output. For the two named canaries I closed this myself by reproducing both failures on the untouched tree; for the three premise pins the narration stands, mitigated by the reviewed-tree run above.
- When I reproduced the goal package's two-minute timeout on the untouched tree I kept only the tail of the output, so I did not capture which test was running when it expired. The causal question the brief asked is nonetheless settled: the timeout occurs without any of the build's changes.
- I did not run the two process-owning fixture beds; the brief assigns them to the orchestrator and reports them still running at dispatch. Nothing in this read depends on them, and a failed bed folds the chain regardless of what I found.
- I did not run the pinned static analyser that the fast gate runs, because this sandbox has no network access to fetch it. I ran formatting and vet instead, both clean, and the builder reports the fast gate green.

## What the critic verified

- (ran) git diff --numstat 4092413287dab57f4c5b947730729d48464c6ccb:metasystem 55e0fe87e2c8ac3cdc071ab60ef61e2764ab8171 => Twelve files changed, 417 lines added and 27 removed. The twelve are exactly the change list in section 3 of the design page: the two metrics source files plus the new metrics test file, the frontier and turn-verdict source files plus their two test files in the goal package, the goal command and its test, the channel report and its test, and the backlog mechanism document. Nothing on the page's untouched list appears: the goal record format, validation, verb, split, reconcile, mapping, migration, ordering, approval and norm sources are all absent, as are AGENTS.md, every goal record, and all steward code. Of the 27 removed lines, exactly one is in a test file.
- (ran) git diff 4092413287dab57f4c5b947730729d48464c6ccb:metasystem 55e0fe87e2c8ac3cdc071ab60ef61e2764ab8171 -- '*_test.go' | grep '^-' | grep -v ' => One removed test line: the old condition of the over-norm frontier subtest. Its replacement keeps every original clause and adds two more (the Ready list must be exactly the claimable goal, and the refusal must be retained with the norm cause). No assertion anywhere in the change was loosened, and no test was deleted or skipped.
- (ran) ripgrep for the strings "done", "split", the done state constant, and any read of a history line's verb across every Go file in internal/met => In non-test code the only occurrences of the words done and split as history verbs are the archiveVerbs table on line 148 of compute.go. The only backward walks over a record's history are the new helper (line 160) and the pre-existing claim-or-steal epoch loop inside concludingEpoch (line 534), which reads neither verb. Two remaining checks of the done state (lines 624 and 838) test the record's state field and scan no history. So after this change the helper is the only function in the package that decides a conclusion from history.
- (read) Read internal/metrics/compute.go lines 140 to 235 and compare against the code printed in section 1.4 of the design page => The helper is character-for-character the page's version. It returns no conclusion for a nil record or any record whose state is not done; it walks history backwards; on the first line whose verb is done or split it parses the stamp and, if the stamp is malformed, returns no conclusion immediately rather than continuing to an earlier line of the same verb. Its three callers (the goal-bounds reader, the whole-period selector, and the exported concluded-in-window predicate) all discard the returned index; the epoch reader uses the index to stop counting claims at the archive act.
- (ran) Extract the base tree to a scratch directory, copy in only the build's new metrics test file, then run: go test ./internal/metrics -run '^Te => Both canaries fail against the untouched production code, for the reasons the page requires. The waiting-row subtest reports Value = "c building_hours=12.000 proving_hours=12.000 waiting_share=0.500 epochs=1" where the test wants unavailable, and the split-parent subtest reports Value = "e building_hours=12.000 proving_hours=12.000 waiting_share=0.500 epochs=1" where it wants proving_hours=36.000 and waiting_share=0.750. Both quoted lines name proving_hours=12.000 and waiting_share=0.500 exactly as the page's table predicts. This reproduces the builder's fail-before claim from my own run rather than accepting the narration.
- (ran) cd <reviewed tree> && go test ./internal/metrics -count=1 -timeout=3m => The whole metrics package passes in 11.3 seconds on the reviewed tree, including all eight subtests of the new conclusion test and the pre-existing lifecycle-edge, zero-duration and attribution tests the page said must not move.
- (ran) cd <reviewed tree> && go test ./internal/goal -run '^(TestPriorityLifecycle|TestNextPriority|TestRefusedBacklogIsReportedWithoutBlocking)$'  => Passes in 8.4 seconds, matching the builder's reported 8.03 seconds. Because the diff changes no writer source at all (the verb, split and ordering files are byte-identical to the base), a green run of the three writer premise pins on the reviewed tree proves the same writer facts the page asked to be proven on the untouched tree.
- (ran) cd <base tree, none of the build's changes present> && go test ./internal/goal -count=1 -timeout=2m => FAIL after 120.258 seconds: the untouched goal package already exhausts the two-minute ceiling that the design's own commands use. This settles the one soft spot the brief named. The widened-run timeout the builder reported is not caused by the change, since it reproduces with none of the change present. My run shared the machine with the orchestrator's fixture beds, which is the same condition the builder ran under.
- (read) Read cmd/metasystem/identity_probes_test.go around the two-second ownership helper => The command package's failing probe polls the operating system for up to two seconds, waiting for a freshly spawned sleep process group to reach an ownership outcome. It spawns and scans real processes and is therefore sensitive to machine load; it shares no state, no file and no code path with the single added string case in the goal command. The judgement that it is unrelated to this change holds.
- (read) Read internal/goal/project.go (the frontier and the claimable-work reader), internal/goal/turnverdict.go (the idle enforcement and its diges => The frontier keeps the existing single gate call and simply retains its answer in a new branch, adding no second traversal and no second configuration load; the selector that reduces the frontier to one action never reads the new list. The digest that decides whether an idle backlog is unchanged hashes only the claimable goals, the claimed goals and the non-terminal jobs, so a refusal cannot move it. The idle enforcement writes its one line before the digest and touches none of the blocking fields. The three human-facing strings match the page word for word, including the parenthesised count of further refusals, and the channel cuts the cause at its first semicolon as specified. The goal listing command is not touched.
- (read) Read the three compaction call sites: the done verb, the split writer and the reconcile publisher => Each moves the departing record into the archive map and deletes it from the live map before calling the compaction walker, and each hands the walker the live map. So a compaction line can only ever land on a live record. This is the property the whole conclusion rule rests on, and it holds in the reviewed tree rather than only in the page's prose.
- (read) Read the claim admission gate and the approval horizon helper, and compare the horizon the frontier uses with the one the gate uses => The gate has four goal-level refusals: a missing approval or budget, an approval record that fails validation, a budget above the tier box without norm coverage, and an expired relayed approval. Every one is a fact about the record, so the new type's comment claiming it is never about the machine is accurate. Configuration failures stay unwrapped and still fail the whole frontier. The frontier filters expired approvals into the awaiting category before calling the gate, using a horizon built by the very same helper from the same tree and instant, so an expired approval cannot leak into the new category.
- (read) ripgrep for every consumer of the frontier type and the claimable-work type across the repository => The consumers are the goal command, the channel report, the claimable-work reader and the steward's attention snapshot. Neither type is serialised to JSON or written to disk anywhere, so adding a field changes no stored or transmitted contract. The attention snapshot is unchanged, as the page decided.
- (read) Read internal/goal/turnverdict.go where the turn verdict composes its display after idle enforcement => The display composer takes the existing display text as its middle part and joins prefix, body and green lines, so the refusal line written during idle enforcement survives into the verdict a human actually reads. There is no later assignment that would discard it.
- (read) Read scripts/agents/goal-cli-fixtures.sh around the two steps that pin exact wording for the no-work line => Both steps run under a label filter over goals that are either parked or unlabelled; neither scenario has an approved goal whose budget exceeds its tier box, so no refusal can appear and the pinned wording is unaffected. The page's claim that the shell bed keeps its exact text is sound.
- (ran) gofmt -l on the eleven changed Go files, and go vet over the four changed packages, in the reviewed tree => Both silent: nothing is unformatted and vet reports no problem, including in the epoch reader where the new call assigns to two named results alongside one new variable.
- (ran) grep every added line of the persisted diff for round, slice, finding or critique identifiers and for plan or design document paths => No matches beyond the header of the documentation file itself. No source comment refers to a round, a slice, a finding, or a design document.
- (ran) shasum -a 256 of the persisted diff, and git cat-file on the recorded tree => The diff hashes to 31da3c0c5b77d8caf51eff6ac00faa2b53ce33b4869630eb06754402940014da, matching the round record, and the recorded tree object 55e0fe87e2c8ac3cdc071ab60ef61e2764ab8171 exists locally, so everything above was read from the tree the orchestrator persisted rather than from prose.

## Coordinator disposition (m1b, 2026-09-09)

No material findings; the chain closes on this read. The critic did not take
the builder's word: it reproduced both canary failures on the untouched tree
itself, ran the writer-premise pins on the reviewed tree where every writer
source is byte-identical to the base, confirmed the helper is
character-for-character the page's, confirmed by search that nothing in
internal/metrics reads done or split except the helper, traced the turn
verdict's display composer to prove the refusal line survives to the human,
and settled the one soft spot the brief named: the goal package's two-minute
timeout occurs on the untouched tree without any of the build's changes.

The three notes are accepted as recorded, not folded:

- BCE-01: the seat test asserts the refusal line where it is written, not in
  the composed verdict. The critic traced the composer and the line survives;
  no defect ships. Carried on the goal as a test-hardening candidate.
- BCE-02: goal next prints the gate's full cause while the channel cuts at
  the first semicolon. The page specifies exactly this ("verbatim",
  "orientation stays one line"). Accepted design; recorded so the asymmetry
  does not surprise a reader.
- BCE-03: the steward's shared-backlog classifier stays silent on a backlog
  whose only work is refused. Page section 2.3 decided this on purpose and it
  is outside the change list. Recorded for whoever revisits the steward's
  machine-to-machine surface.

The process-owning beds were running on the reviewed tree when the read was
dispatched (dispatch scenario passed at 10:27Z, mission-runner in progress);
their result and the receipt-bound battery on the candidate tree are the
landing's proof and are recorded in the landing commit.

# brain seat build code review — round 2 (chain brain-build1b-20260907, round-6 tree)

Chain: build rounds 5-6 (Sol) folded the round-1 review -> reviewed tree e4089dd9fcfa977be72c676bb0fe676910fd3678 -> critic brain-review4-20260907 (Fable, code-critic, read-only). 3 material findings, 4 notes. The coordinator carried the return here verbatim because the critic wrote no register file.

## F-1 — medium, material=True

CLAIM: When the bounded child that reads the optional boot inputs is stopped at the deadline, the parent marks all four sections skipped and drops every section the child had already finished, instead of reading whatever section files are complete as design revision 3 section 2 requires. The deadline line then names sections that were in fact read. A brain whose fleet or digest input stalls therefore boots without the asks awaiting Wido and the claims held here that were available on disk, and the stalled fixture row cannot see this because its stall sits in the first section.

EVIDENCE: In cmd/metasystem/brain_boot.go composeBrainBoot sets optionalInputsFailed to true when the deadline fired or the child's wait returned an error, and then marks every section skipped without opening its file; the SIGTERM the parent sends makes the child's exit an error, so the branch that reads complete section files is unreachable after any deadline. Ran on a scratch declared bed holding one open channel question and a named pipe at artifacts/agents/supervision/last-census.json: brain boot with a 1500 millisecond deadline returned in two seconds with sections asks, held, fleet and digest all skipped, a deadline line naming all four, and no ask line in the payload. Running the boot-inputs child alone for one second on the same bed wrote a complete asks section file carrying the question. The design page says the parent reads whatever section files are complete and only a section whose file is absent is skipped.

## F-2 — medium, material=True

CLAIM: brain declare succeeds on a checkout that holds a job record in status pending-setup, which design revision 3 section 1 step 5 lists as a quiescence obstacle that must be refused with the cancel command. The pending-setup extension of the in-flight set is keyed on the declaration, so the undeclared checkout being converted still uses trunk's pending-and-running set at declare time and the obstacle is invisible exactly when it must be refused. After declaration the same record becomes in flight, so every turn end on the new brain prints STILL WORKING for a husk the brain is fenced from reaping, and the seat stays that way until a human withdraws, reaps and declares again. The brain-declare-quiescence row seeds only a pending job and cannot see this.

EVIDENCE: Ran on the scratch bed with artifacts/agents/jobs/husk-job.json in status pending-setup: brain declare with the fixture human authority exited 0 and printed the declared record; report turn-verdict on the now-declared seat displayed STILL WORKING: implementer husk-job [pending-setup, fake] under the BRAIN SEAT line; brain fence with act reap on that seat was fenced with exit 2. In internal/report/openwork.go inFlightStatuses returns the set containing pending-setup only when the record reads declared or corrupt, and brainDeclarationObstacles in cmd/metasystem/brain.go takes its job obstacles from report.Scan, which uses that set.

## F-3 — medium, material=True

CLAIM: The dispatch half of the never-a-bottleneck row brain-absent-node-proceeds does not pass on the host tree: the fake-runtime delegate it starts names the design-critic role with a design path that the node clone's reviewed commit does not contain, so the delegate refuses before it starts and the row proves nothing about a node dispatching while another checkout is the brain. A named fixture that fails is material by the review rule, and design revision 3 section 6 names this row as the proof of that section.

EVIDENCE: scripts/agents/dispatch-fixtures.sh copies the design-critic role file into the node clone's working tree under a metasystem/scripts/agents/roles directory without committing it, then dispatches with --design pointing at that path; the brief reports the host failure text saying the design path is not a blob at the reviewed commit for this leg. Not rerun here because the fixture beds cannot run in this sandbox.

## N-1 — low, material=False

CLAIM: The scanner now reads channel question files on every checkout and appends an unreadable file to the unreadable list, so on an undeclared node a malformed question file turns the goal clause into an inputs-unreadable line and suppresses that node's steer. Design revision 3 section 4 says an unreadable question file joins the unreadable list and also promises that no other seat's verdict changes; the round-1 fold removed the draft-scan error from undeclared seats but left the question-file error in. Not material because the design names the behaviour and one visible line about a corrupt local artifact is defensible; recorded so the coordinator can decide.

EVIDENCE: Ran on the scratch bed after withdraw with a broken file at artifacts/agents/channel/questions/broken.json: report turn-verdict displayed one input unreadable naming that file, and with the file removed it displayed the goal clause naming the queued goal. internal/report/scan.go appends the question walk's unreadable entries to the result, and withoutBrainOnlyScanEffects in internal/goal/turnverdict.go filters only entries prefixed draft scan.

## N-2 — low, material=False

CLAIM: Weak fixture rows. brain-boot-stalled stalls the first section, so it cannot distinguish complete sections kept from all sections dropped. brain-declare-quiescence seeds a pending job and never a pending-setup one. brain-fence-helper-fails exercises the router fences under the internal marker only and never the legacy guard's helper failure. brain-delegate-refuses never runs the follow-up form. brain-boot-caps measures the header with a character count rather than bytes, which is equal for the ASCII fields it uses; the Go side has no explicit 320-byte assertion, the maximal header computing to 275 bytes by construction.

EVIDENCE: Read of scripts/agents/brain-fixtures.sh and scripts/agents/dispatch-fixtures.sh at the reviewed tree.

## N-3 — low, material=False

CLAIM: classifyGoalAuthorityFirst treats the presence of the fixture-human-authority flag as a fixture proof before the prover has verified the fake root, so on a declared brain outside the fake root an agent passing that flag skips the brain's human-word refusal and is refused later by the prover with its own text. No act is admitted; only the refusal text differs from the design's.

EVIDENCE: Read of cmd/metasystem/goalsync_mutations.go: classifyGoalAuthorityFirst builds a fixture-only proof from the flag alone, and proveGoalHumanAuthority refuses when fixtureauth.New or FixtureGoalProof rejects the root.

## N-4 — low, material=False

CLAIM: On a checkout with no ledger identity, brain withdraw removes the corrupt record and then fails reading the host pointer, because the pointer path built from an empty identity is the pointer directory itself; the verb exits 2 with an is-a-directory error although the withdrawal happened.

EVIDENCE: Ran on a scratch directory with a fake-runtime metasystem.conf and a broken record at artifacts/agents/brain.json: brain withdraw printed brain withdraw: read .../registry/.metasystem/brain: is a directory and exited 2; brain show then read undeclared with exit 3.

## Gaps the critic named

- The bash fixture beds could not run in this sandbox: bash 3.2 cannot create heredoc temp files under any allowed temp directory, so the brain, dispatch, land, goal-cli and hook rows rest on the host's receipts and on reading. I substituted direct engine probes on a scratch migrated bed built with the same steps the fixtures use.
- The reviewed tree hash could not be recomputed because the sandbox denies writes to the shared git object database; equality with the dispatcher's artifact was proven by reverse-applying the diff onto the worktree instead.
- The runtime's live consumption of the session-start context field and the compact source was not observed; the design itself lists this as unprovable in a sandbox.
- The process-inspection test and the full command-package suite were not rerun here; I ran the brain-related subset of the goal, report and command packages and the six touched leaf packages in full.
- Finding F-3 was not reproduced here; it rests on the host failure the brief reports and on the fixture's own code.
- Tool names were unobserved by the launcher, so this return is advisory per the runtime notice.

## Coordinator disposition (m1, 2026-09-07)

All three material findings and the four notes fold in build round 7,
the last round of this chain: the review rounds are spent, so round 7
closes the chain on this review's evidence after the host gate.

- F-1 (deadline drops finished sections): after the deadline the parent
  reads every section file the child completed and marks only the
  unfinished ones skipped; the deadline line names only those. The
  stalled row stalls a later section so the kept sections are observed.
- F-2 (declare misses pending-setup): declare's quiescence check uses
  the brain's in-flight set, pending-setup included, regardless of the
  declaration, and prints the cancel command; the quiescence row seeds
  a pending-setup record.
- F-3 (absent-node dispatch leg): the fake delegate uses the
  implementer role with a plain brief on the fake runtime, nothing that
  needs a design path; the row proves the node's delegate starts.
- N-1 (question-file errors on undeclared seats): brain-only; an
  undeclared seat's verdict is unchanged by a malformed question file.
- N-2 (weak rows): the stalled row stalls a later section; quiescence
  seeds pending-setup; the fence-helper row also covers the legacy
  guard; the delegate-refuses row runs the follow-up form; the caps row
  counts bytes and the Go test asserts the 320-byte header bound.
- N-3 (fixture flag before root proof): the fixture proof is built only
  after the fake root is verified, so a declared brain outside it gets
  the brain's own refusal text.
- N-4 (withdraw with no ledger identity): withdraw with an empty
  identity removes the record and reports it without touching the
  pointer directory.

Next: round 7, host gate, close the chain with this review's evidence,
full battery receipt, land, conclude.

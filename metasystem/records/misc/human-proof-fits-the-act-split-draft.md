# split human-proof-fits-the-act

## member hp-terminal-grade-for-stopping-acts
- Intent: A human at any live terminal of the host, with no agent in the ancestry, can stop, park, release or unpark-without-approval, and session-stop, without an enrolled terminal: a second proof grade (the enrollment walk without the write), placed on those acts only, so no seat is ever wedged by a dead enrolled terminal. Design plans/human-proof-fits-the-act-design.md revision 2, sections "The two proofs" and the stopping rows of section 1.
- Next step: Sol builds ProveTerminal beside Prove in internal/humanauthority, the grade field on the proof and the request, and the terminal-grade rows for session stop, park, release, unpark onto a queued state and the stopping process verbs; fixtures drive an agent shell refused for a stop and a human shell at an unenrolled terminal allowed to stop and refused for an approve. One Sol round, one Opus review, land.
- Labels: comfort

## member hp-refusals-print-the-one-command
- Intent: Every refusal in the authority layer prints exactly one command that would have succeeded from where the human sits, or says in words that none would: TERMINAL_NOT_REACHED names the enrolled terminal by its tmux session or tty, says the current terminal is not it, that enrolling here needs no prior authority and retires the other terminal, and that the refused command is then retried; a structured no-enrollment outcome shared by both proofs maps to the enroll command; an unreadable or incomplete enrollment says no command can safely complete until the record is repaired, and how. Design revision 2 section 3 with findings HPA-07, HPA-08, HPA-14 and HPA-15 folded.
- Next step: Sol builds the outcome code, the refusal renderer with the ephemeral carrier for the failed enrolled attempt (the CLI builder returns the request plus the structured attempt), and the texts for every code in internal/refusal/register.go the authority layer owns; fixtures assert each text against its code. One Sol round, one Opus review, land. Coordinate with goal human-goal-verbs-forgiving, which owns the same rule for the goal verbs' other refusals.
- BlockedBy: hp-terminal-grade-for-stopping-acts
- Labels: comfort

## member hp-resume-takes-its-budget-from-the-ledger
- Intent: goal resume reads the standing approved budget from the ledger instead of demanding the five-member tuple retyped on the command line; a human who wants a different budget uses set-budget, and a mismatch refusal prints the standing values. Design revision 2 section 4.
- Next step: Sol changes runGoalResume to take the goal's Budget when no tuple is given, keeps the explicit tuple as an override that must equal the standing one, and prints the standing values on mismatch; a fixture resumes a breach-stopped goal with no budget flags. One Sol round, one Opus review, land.
- Labels: comfort

## member hp-every-by-proves-a-human
- Intent: An exported lineage plus --by no longer reaches any human-only act without a proof: the CLI builder proves on every --by regardless of how the lineage arrived, and migrate, repair, fleet enrollment, reopen's all-parked-arc branch and set-pin get explicit proof plumbing, so Actor.Human alone is never sufficient anywhere. Findings HPA-01 and HPA-16 of the human-proof critiques; the primary canary is an agent shell with the lineage exported, --by on steal, foreign release, classify-sweep, set-pin, reconcile, migrate and repair, every one refused, in a scratch ledger and never the live one.
- Next step: Sol builds the builder change in cmd/metasystem/goalsync_mutations.go and the proof plumbing on the routes that bypass syncReq (goalsync_verbs.go for migrate and repair, internal/goal/approval.go for fleet enrollment, verbs.go for reopen's parked-arc branch and set-pin); the scratch-ledger canary and an engine fixture proving Actor.Human without Authority is refused everywhere. One Sol round, one Opus review at xhigh, land. Sequence with fixture-human-authority-mintable-from-conf, which closes the other mint.
- BlockedBy: hp-terminal-grade-for-stopping-acts
- Labels: security

## member hp-recovery-grants-no-grade-to-unstamped-intents
- Intent: goal recover never completes a pre-change journal entry that carries --by but no authority grade: a missing stamp grants no grade and terminalizes toward a fresh human invocation, and only an explicitly human-ratified migration record grandfathers an old entry. Finding HPA-11.
- Next step: Sol changes internal/goal/recover.go to refuse the stored intent when its authorityGrade is absent and to record the terminalization; a fixture seeds a dead owner's journal with an unstamped foreign release and proves recover does not run it. One Sol round, one Opus review, land.
- BlockedBy: hp-every-by-proves-a-human
- Labels: security

## member hp-grading-matrix-for-widening-acts
- Intent: Every remaining human-only act carries its grade from the matrix in design revision 2 section 1, with the second critique folded: set-pin (set and clear), unpark onto a standing approval, set-arc that creates a claim, discharge-review-obligation, arm, steward arm and restart, brain withdraw, mission start and resume under a closed fence, and both resolve-taint variants take the enrolled grade (findings HPA-02, HPA-03, HPA-04, HPA-12); the channel-authority consumers keep their existing classes and are never coerced into another grade (HPA-13).
- Next step: Fable revises the matrix section only, folding HPA-12 and HPA-13 (one page, one critique); then Sol builds the rows in one round with a fixture per row; Opus review; land.
- BlockedBy: hp-every-by-proves-a-human
- Labels: security

## member hp-relayed-word-retired
- Intent: The temporary relayed-word class is removed for real: the flags, the temporary outcome and validators in internal/humanauthority, the temporary legs of the goal verbs, the RELAY_AFTER_ENROLLMENT register row, ArmTemporary and its steward caller, the help and recovery text that advertise the flags, and the runner's carry-forward, which keeps historical fields as inert provenance and never mints authority from them. Design revision 2 section 6 with HPA-06 folded.
- Next step: Sol removes every site named in the design and HPA-06, with the fixtures that relied on the relay moved to the injected prover that goal fixture-human-authority-mintable-from-conf introduces. One Sol round, one Opus review, land.
- BlockedBy: fixture-human-authority-mintable-from-conf
- Labels: cleanup

## member hp-enrollment-before-migration
- Intent: A checkout can enroll a terminal before its backlog is migrated, and the fleet-cutoff publication waits until the backlog exists, so migrate can take the enrolled grade and the bootstrap needs no exception. Finding HPA-05.
- Next step: Sol splits enroll-terminal into the local write and the fleet publication, defers the publication until the backlog is converted, and grades migrate enrolled; a fixture enrolls on an unconverted checkout, migrates, and sees the cutoff published after. One Sol round, one Opus review, land.
- BlockedBy: hp-every-by-proves-a-human
- Labels: security

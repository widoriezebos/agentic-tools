# busy-seat-shells-are-ended

- State: approved
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: one core for hours per incident, no data loss; novelty 2: a CPU predicate and child evidence the census lacks today; exposure 3: every seat runs shell commands through the harness; accumulation 1: incidents are rare but unbounded when they occur"
- Tier: 2
- Intent: On 2026-09-14 a seat's Bash command, a descendants-walk loop that never ends under zsh, burned 65 to 70 percent of a core for twelve hours after the tool's timeout moved it to the background (records/misc/leaked-processes-diagnosis-2026-09-15.md, class C). Wido decided (R-112-m1e) that the engine ends such a shell. Critique round 2 of seat-machines-shed-leaked-processes showed that samples ten minutes apart cannot prove a shell had no child for 30 minutes. DONE: a seat shell that burns CPU with no child process for 30 minutes is ended by the engine, logged, and reported to its session, by a predicate provable from the evidence the engine reads (for example the shell's own accumulated CPU time over the interval) and rechecked beside each signal, never ending a shell whose work is in its children; proven by a planted busy loop ended within the bound while a long go test and a long checksum are left alone. Seed material: rules W1 to W5 and S3 of the revision-2 design page of seat-machines-shed-leaked-processes, and its critique rounds 1 and 2.
- Origin: human
- Next step: Design first by a Claude Fable delegate from the seed material named in the intent, answering critique round 2's SMLP-204 to -206; then a Codex critique; then the build.
- OpenedAt: 2026-09-15T09:05:29Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-15T09:05:33Z revision=2 opid=3TJKYFEN5XAYSMMKNF4A5JXDK7-m1e-c6925449 authority=proven digest=e8e4f27b54999ff256bbf98afe06982dce699fd3a80156a72ff299dbf8ef9752

History:
- 2026-09-15T09:05:29Z Y72CF7BYS3RMM691NRZA2Z9YY7-m1e-c6925449 open actor=human:Wido targets=busy-seat-shells-are-ended
- 2026-09-15T09:05:33Z 3TJKYFEN5XAYSMMKNF4A5JXDK7-m1e-c6925449 approve actor=human:Wido targets=busy-seat-shells-are-ended
Integrity: sha256=acd522dd87817220aed0a9479e52cec7fc5c45c5ee8fa12d120764d1db209e5a

# codex-jobs-run-through-a-metasystem-verb

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="severity 3: the largest measured leak, about 21 GB and 574 processes in a day; novelty 2: owning an external plugin's job lifecycle through a verb and a terminal observer; exposure 3: every seat delegates builds and critiques to Codex daily; accumulation 3: one broker tree per worktree, growing until killed"
- Tier: 3
- Intent: Codex companion brokers leak: the openai-codex plugin 1.0.6 keeps one detached broker per working directory, shuts it only through its SessionEnd hook for the session's own cwd, and runs each job in a detached worker with no completion callback, so every worktree ever used for a Codex job kept a five-process tree (574 processes, about 21 GB on 2026-09-15). Design critique of seat-machines-shed-leaked-processes showed that a janitor cannot shut a broker safely from outside while callers use the plugin directly. Wido decided (R-112-m1e, R-113-m1e) that every Codex job on a seat machine goes through a metasystem verb, and that the codex-rescue agent is not used on seat machines until it goes through the verb too. DONE: a metasystem verb launches a Codex job, observes its terminal state without relying on a later caller action, cancels a job by its id from any directory, and shuts the job's broker at the job's terminal state and on cancel, never while another job uses it; seat briefs and launch scripts use only the verb; a census lists any broker without a live job; proven by a day of three-seat Codex use that ends with no broker older than its job. Seed material: section 3.2 and rules K1 to K10 of plans/seat-machines-shed-leaked-processes-design.md revision 2, and its critique rounds 1 and 2.
- Origin: human
- Next step: Design first by a Claude Fable delegate from the seed material named in the intent, answering critique round 2's SMLP-201 to -203, -207 to -210 and -213; then a Codex critique; then Wido approves the unit budget. Until the verb exists, seats keep shutting the broker for their job's cwd when the job ends (R-113-m1e).
- OpenedAt: 2026-09-15T09:05:17Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-15T09:05:17Z VH18NT8H5Z1JXZ6SVWGC6ZCTZH-m1e-c6925449 open actor=human:Wido targets=codex-jobs-run-through-a-metasystem-verb
Integrity: sha256=75a293369e41b3ee174a8e8dc7a45afd2748bc3480a50793ed104c0bf8f239da

No shape defect remains. S1 closes cleanly; the five residual issues below are mechanical and fixture-expressible. Under the declared failsafe rule, the critique loop should close and implementation may begin.

Evidence level: checked by reading the full brief and repository HEAD `209d796`; no files changed and no tests run.

1. **FIXTURE-EXPRESSIBLE — S4 — Every effective skip path must consult the policy.**  
   A build must inventory actual omitted validation families, not only calls to `delivery_contract_skip`. Today `--delivery-contract` exports `METASYSTEM_SKIP_AGENT_FIXTURES=1` before validation starts, and that variable bypasses supervision, census, fingerprint, and mission fixtures outside the named helper ([validate-metasystem.sh](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:25), [validate-metasystem.sh](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:39), [validate-metasystem.sh](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:597)). S4’s exact-set and policy-version rules provide the expected behavior; a differential fixture can catch any unlisted omission.

2. **FIXTURE-EXPRESSIBLE — S2 — “Recorded runner pid” must mean the complete process identity.**  
   Supersession must require `identity.Dead` for the recorded PID plus start identity; PID reuse is dead, while an unreadable probe is `Unknown` and authorizes nothing. The repository explicitly defines identity as PID plus start time and makes liveness three-way ([identity.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/identity/identity.go:1), [identity.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/identity/identity.go:28), [identity.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/identity/identity.go:110)). I interpret S2’s “identity liveness rules” accordingly; live, dead, reused-PID, and unknown fixtures must pin it.

3. **FIXTURE-EXPRESSIBLE — S3 — Reset reporting must stop treating `LastCommit` as the battery subject.**  
   The current reset command prints `state.LastCommit` as the reset commit ([gate_weight.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/gate_weight.go:90)). Under S3, that field may name a newer concurrent landing; the battery subject instead comes from its checkpoint or envelope ([brief](/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools/af965b26-11a5-4d56-b6c2-8f1df989f9f3/scratchpad/gate-run-freeze-brief.md:207)). A concurrent-add fixture must verify both state and report provenance.

4. **FIXTURE-EXPRESSIBLE — S3 — `PostCheckpointSinceUTC` must be scoped to one checkpoint lifetime.**  
   After abandonment or supersession, a later checkpoint’s first add must replace—not inherit—the earlier checkpoint’s timestamp. This follows from S3’s “first add while a checkpoint is open” and S2 allowing the next battery to open ([brief](/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools/af965b26-11a5-4d56-b6c2-8f1df989f9f3/scratchpad/gate-run-freeze-brief.md:197), [brief](/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools/af965b26-11a5-4d56-b6c2-8f1df989f9f3/scratchpad/gate-run-freeze-brief.md:207)). A two-checkpoint fixture can enforce clearing or equivalent per-checkpoint ownership.

5. **FIXTURE-EXPRESSIBLE — S5 — Pending reset-repair data must survive until publication is confirmed.**  
   Because repair republishes `reset.json` “from the state,” the consumed checkpoint must retain the destination envelope identity and complete reset result until the appendix exists. A subsequent add or battery opening must not overwrite that replay material first. Failure injection followed by a weight read can validate this without further design work ([brief](/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools/af965b26-11a5-4d56-b6c2-8f1df989f9f3/scratchpad/gate-run-freeze-brief.md:222)).

VERDICT: CONDITIONALLY ENDORSED

## OBLIGATION ROWS

| ID | Testable obligation |
| --- | --- |
| RUN-01 | Invoking the battery’s single entry point produces a final report naming the exact subject commit. |
| RUN-02 | Starting a battery creates an independent local clone detached at the recorded subject commit. |
| RUN-03 | Uncommitted or ignored live-checkout bytes present at startup do not enter the battery subject. |
| RUN-04 | The isolated clone never appears in Git’s linked-worktree inventory. |
| RUN-05 | The run directory is fresh, outside every checkout, and has no symlink component. |
| RUN-06 | Creating the clone while mission-wall worktree enumeration runs leaves that enumeration byte-identical. |
| RUN-07 | Removing the clone while mission-wall worktree enumeration runs leaves that enumeration byte-identical. |
| RUN-08 | The clone builds and executes its own engine binary. |
| RUN-09 | A complete battery leaves the live checkout’s `bin/metasystem` bytes unchanged. |
| RUN-10 | The isolated build uses the declared shared `GOCACHE`. |
| RUN-11 | Goal ledgers, receipts, artifacts, and other live coordination state are not copied into the clone. |
| RUN-12 | The clone’s `conf.local` is materialized only from the committed generic-safe battery template. |
| RUN-13 | The live checkout’s `conf.local` is never copied, and changing it does not change the subject digest. |
| RUN-14 | Without the battery override, supervision still resolves the normal user-home registry path. |
| RUN-15 | With the battery override, supervision resolves only the run-scoped registry path at the existing `UserHomeDir` seam. |
| RUN-16 | Starting the isolated run does not change process `HOME`. |
| RUN-17 | Starting the isolated run does not redirect Go or runtime-adapter homes. |
| RUN-18 | The isolated run arms supervision in its own registry. |
| RUN-19 | The live census never attributes an isolated-run PID to the live checkout. |
| RUN-20 | The isolated census never attributes a live-checkout PID to the isolated clone. |
| RUN-21 | Teardown signals only exact process identities recorded by the isolated run. |
| RUN-22 | Successful teardown removes the isolated clone. |
| RUN-23 | Evidence-copy failure retains the clone and reports its path. |
| WGT-01 | Every accumulator operation locks the stable sibling `battery-weight.flock`, never `battery-weight.json`. |
| WGT-02 | A writer holding the sibling lock remains mutually exclusive after the state file is atomically renamed. |
| WGT-03 | Add, checkpoint, reset, abandonment, supersession, and read-side repair use the same lock. |
| WGT-04 | Every accumulator mutation increments `generation` exactly once. |
| WGT-05 | A malformed or torn state file refuses loudly and remains unchanged. |
| WGT-06 | Opening a checkpoint records generation, accumulated weight, landing count, subject, full runner identity, run identity, and repair destination. |
| WGT-07 | A second battery encountering a live open checkpoint refuses without mutating state. |
| WGT-08 | An `Alive` identity verdict never authorizes checkpoint supersession. |
| WGT-09 | An `Unknown` identity verdict never authorizes checkpoint supersession. |
| WGT-10 | A reused PID whose identity differs from the record is treated as dead. |
| WGT-11 | A new battery superseding a dead runner records that transition in its envelope. |
| WGT-12 | Superseding a checkpoint preserves all accumulated weight. |
| WGT-13 | Red validation publishes `abandoned.json` and leaves the weight unchanged. |
| WGT-14 | Evidence-copy failure publishes abandonment when possible and leaves the weight unchanged. |
| WGT-15 | Operator abort publishes `abandoned.json` and leaves the weight unchanged. |
| WGT-16 | After abandonment, the next battery may open a checkpoint. |
| WGT-17 | After provable runner death, the next battery may supersede and open a checkpoint. |
| WGT-18 | Weight added during a battery remains after that battery consumes its checkpoint. |
| WGT-19 | Landing counts added during a battery remain after checkpoint consumption. |
| WGT-20 | The first weighted add during a checkpoint sets `PostCheckpointSinceUTC`. |
| WGT-21 | The first zero-weight add during a checkpoint sets `PostCheckpointSinceUTC`. |
| WGT-22 | Later adds during the same checkpoint do not replace `PostCheckpointSinceUTC`. |
| WGT-23 | Partial reset subtracts exactly the checkpointed accumulated weight. |
| WGT-24 | Partial reset subtracts exactly the checkpointed landing count. |
| WGT-25 | Partial reset adopts the checkpoint’s first post-checkpoint timestamp as `SinceUTC`. |
| WGT-26 | A partial reset lacking a post-checkpoint timestamp falls back to reset time. |
| WGT-27 | A reset leaving no remainder restarts `SinceUTC` at reset time. |
| WGT-28 | Every add updates `LastCommit` to that landing’s commit. |
| WGT-29 | Reset never changes `LastCommit`. |
| WGT-30 | A concurrent-add reset report names the battery subject rather than the newer `LastCommit`. |
| WGT-31 | Abandonment or supersession clears or otherwise retires the prior checkpoint’s post-checkpoint timestamp. |
| WGT-32 | Reset of an already consumed checkpoint refuses as stale without another subtraction. |
| WGT-33 | Injected reset-write failure leaves the checkpoint open and the accumulator unchanged. |
| SUR-01 | The engine loads one versioned behavior-surface policy and rejects an unsupported policy version. |
| SUR-02 | The `ENGINE` projection reproduces the declared D33 engine-input closure. |
| SUR-03 | The `LANDING` projection includes repository content and excludes the coordination class. |
| SUR-04 | The `PAYLOAD` projection equals adoption’s allowlist minus the named tailored class. |
| SUR-05 | Configuration, registrations, ignore files, and fresh ledgers are classified as `TAILORED` by name. |
| SUR-06 | Every digest comparison reports its projection and both compared endpoints. |
| SUR-07 | Path classification is identical at repository root and under a nested metasystem prefix. |
| SUR-08 | Filenames containing whitespace, newlines, or shell metacharacters are processed NUL-safely. |
| SUR-09 | A rename across policy classes is classified as one removal and one addition. |
| SUR-10 | A symlink or symlink ancestor is judged through the declared component walk. |
| SUR-11 | Changing only a file mode does not change a surface digest. |
| SUR-12 | The witness digest consumes the `ENGINE` projection from the policy owner. |
| SUR-13 | The exact-final-bytes landing law consumes the `LANDING` projection. |
| SUR-14 | Weight classification weighs exactly the `LANDING` projection’s members. |
| SUR-15 | Adoption payload equivalence consumes the `PAYLOAD` projection. |
| SUR-16 | Prospective landing classification uses the fast gate’s proof-built engine. |
| SUR-17 | Battery classification uses the isolated clone’s engine build. |
| SUR-18 | Toolchain identity is stored separately from every byte digest. |
| SUR-19 | Digest equality is never accepted when independent toolchain identity differs. |
| SUR-20 | A landing that edits the policy is judged by its own prospective policy. |
| SUR-21 | A stale live binary cannot affect a prospective landing’s classification. |
| SUR-22 | The policy’s initial skip set equals every validation family effectively omitted by the current delivery-contract behavior. |
| SUR-23 | Environment-driven suppression cannot omit a family absent from the policy’s skip set. |
| SUR-24 | A concrete project extra-suite omission must resolve through one declared policy family, not its arbitrary basename. |
| SUR-25 | Payload or toolchain mismatch runs the skipped proof instead of accepting reuse. |
| SUR-26 | An unlisted skip request does not skip its validation family. |
| SUR-27 | Adding a skip family without increasing the policy version fails the policy fixture. |
| SUR-28 | Tailored-path differences remain subject to adoption’s existing dedicated assertions. |
| EVD-01 | Every run creates a run-ID-scoped durable envelope directory before any appendix can publish. |
| EVD-02 | Stage 1 publishes for both green and red validation outcomes. |
| EVD-03 | The stage-1 schema contains logs, exit codes, failure artifacts, subject SHA, surface digest, timings, validation exit, copy result, and per-file copy digests. |
| EVD-04 | Stage 1 becomes visible only through an atomic rename. |
| EVD-05 | Per-file copy digests are verified before reset is attempted. |
| EVD-06 | Reset is forbidden until stage-1 verification succeeds. |
| EVD-07 | Forced-red failure artifacts exist in the durable envelope before clone cleanup. |
| EVD-08 | Copy failure classifies the run as evidence-incomplete and never green. |
| EVD-09 | Copy failure performs no reset. |
| EVD-10 | Red validation with successful copy abandons the checkpoint without reset. |
| EVD-11 | Green validation with verified copy consumes only the checkpointed weight. |
| EVD-12 | Reset failure after green evidence reports `green/reset-unrecorded` and leaves the checkpoint open. |
| EVD-13 | Successful reset publishes `reset.json` atomically as a separate appendix. |
| EVD-14 | Failure to publish `reset.json` leaves the checkpoint consumed with repair data intact. |
| EVD-15 | The next weight read repairs a missing `reset.json` while holding `battery-weight.flock`. |
| EVD-16 | Read-side repair completes before a new checkpoint can overwrite its replay data. |
| EVD-17 | Repeating read-side repair does not subtract weight twice or publish conflicting reset facts. |
| EVD-18 | A failed read-side repair leaves the consumed repair record available for a later retry. |
| EVD-19 | Successful teardown publishes `teardown.json` into the existing stage-1 envelope directory. |
| EVD-20 | Failed teardown publishes a retained-worktree result without changing the otherwise-final verdict. |
| EVD-21 | `teardown.json` is written by the durable controller after the teardown attempt, not from deleted clone state. |
| EVD-22 | Runner death before terminalization leaves the checkpoint recoverable by S2 supersession. |
| EVD-23 | Runner death after stage 1 but before reset preserves the published stage-1 envelope. |
| EVD-24 | Runner death after reset but before `reset.json` is recovered by the next weight read. |
| EVD-25 | Runner death during teardown leaves every already-published appendix intact. |
| INT-01 | A concurrent goal-verb write does not alter or terminate the isolated battery result. |
| INT-02 | A concurrent live-checkout commit does not alter the battery’s subject or result. |
| INT-03 | A concurrent live-checkout rebase does not alter the battery’s subject or result. |
| INT-04 | A concurrent live-checkout checkout does not alter the battery’s subject or result. |
| INT-05 | A concurrent push does not alter the battery’s subject or result. |
| INT-06 | A concurrent live `conf.local` edit does not alter the battery’s subject or result. |
| INT-07 | Concurrent live supervision arming does not claim or disrupt isolated-run processes. |
| INT-08 | Concurrent live supervision disarming does not kill isolated-run processes. |
| INT-09 | A concurrent second battery attempt refuses while the first runner is live without disturbing it. |
| INT-10 | Every named live-checkout interference operation remains executable while the isolated battery runs. |
| INT-11 | If a residual verb freeze is implemented, it blocks only the demonstrated remaining interference window and releases afterward. |
| BLD-01 | Unit tests cover every policy class and each projection’s boundary cases. |
| BLD-02 | A static check proves all digest, landing, weight, and skip consumers route through the policy owner. |
| BLD-03 | Go formatting, vet/static analysis, and shell syntax checks pass for every touched implementation surface. |
| BLD-04 | All touched existing fixture legs pass without weakening their assertions. |
| BLD-05 | `docs/collaboration.md` describes weight-triggered milestone cadence instead of mandatory per-landing full batteries. |

Per C7, performance comparison, the first post-landing full battery, the timebox stop line, and the fresh-Codex delivery sequence remain goal-tracked process evidence rather than fixture rows.

Codex session ID: 01a03a46-4996-7ec3-a684-05c0650f9920
Resume in Codex: codex resume 01a03a46-4996-7ec3-a684-05c0650f9920

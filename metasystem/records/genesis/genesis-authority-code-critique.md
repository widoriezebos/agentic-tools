# Genesis authority — code-critique round 1 dispositions

Critic: code-critic, Claude claude-opus-5 (native subagent; the dispatchable
code-critic lane requires an implementer job and this was a main-session
change — the issue-9 precedent), reviewing the uncommitted working tree at
7606339+dirty against `plans/genesis-authority-design.md`. Verdict line: 4
material of 9 findings. Body read in full; all nine dispositioned. The
critic's own replay evidence is quoted in its return (kept verbatim in the
turn record); its conformance pass verified the territory constraint clean
(one goalverbs.go hunk, Reconcile's genesis arm only; adopt.sh seeding and
git-init handling untouched; no missionrunner/goal-verb/wall files).

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| GC-R1-01 | accepted | The fixture probe called `lease classify` without `--caller-pid`, classifying pid 0 → always HUMAN; under agent ancestry the probe then died calling the refusal a human's, and its success-arm assertion was a tautology. Proven twice: the critic's verbatim replay, and the harness itself (adopt-fixtures run at 17:15Z failed with exactly that message; evidence dir since removed after diagnosis). | The probe passes `--caller-pid $$` and runs against a scratch COPY of the adopted target. adopt-fixtures.sh rerun green after the fix. |
| GC-R1-02 | accepted | `goal prune` refuses at ≤10 done goals (`DoneKept`, `goal.go:37`), so the undo path aborted under `set -euo pipefail`, and the "pristine pair" claim was false regardless — the Done block survives. | The probe mutates only the scratch copy; the real target is never written, so there is nothing to undo. The done/prune sequence is gone. |
| GC-R1-03 | accepted | `goalCaller` returned an error on any shape-probe failure BEFORE Authorize ran, refusing the human — contradicting the design's R2/R3 (a probe that cannot read refuses the SHAPE, never the human). Proven live: PATH without git refused a HUMAN genesis. | goalCaller sets `adoptionShaped=false` on a read or probe error and lets the matrix decide; a machinery refusal appends the probe error. Two new command-layer tests (human passes with no git on PATH; delegate refusal names the probe failure). |
| GC-R2-01 | accepted | CRITICAL. `gitIn` inherited the process environment, so `GIT_DIR`/`GIT_WORK_TREE` redirected the HEAD guard to a caller-chosen repository — proven live (a DELEGATE bypassed the tracked-ledger refusal with one env var), and accidental exposure is real (hooks and rebase/bisect subprocesses export GIT_DIR). Same shape as review hole 1: a caller-named root deciding authorization. | `gitIn` strips the repository-steering git environment (GIT_DIR, GIT_WORK_TREE, GIT_COMMON_DIR, GIT_INDEX_FILE, GIT_CEILING_DIRECTORIES, GIT_OBJECT_DIRECTORY, GIT_ALTERNATE_OBJECT_DIRECTORIES). New test pins both directions (a steering env can neither hide a tracked ledger nor invent one). |
| GC-R1-04 | noted | True: the healthy-pair skip in adopt.sh is not exercised by the fixtures (re-adoption exits at the same-SHA gate first). The branch fails safe (a broken probe → reconcile runs, the old behaviour) and its condition was replayed in isolation by the critic. Exercising it needs a fixture that re-adopts at a DIFFERENT SHA, which is the adoption-upgrade path — out of this arc's scope; named in the report. | none |
| GC-R1-05 | noted (fixed) | benchmark/README.md:229 still said "genesis is human-reserved". | Line updated: any shell; genesis admits the provisioner for the goal-free first baseline. |
| GC-R1-06 | noted | The declared codex.sh bash-3.2 hunk is judged correct and minimal by the critic's own bash-3.2 replay; it is the same idiom adopt.sh already uses. | none |
| GC-R2-02 | noted (fixed) | Latent predicate mismatch: the store skipped the re-check for ANY holder, the matrix only for MAIN+holder; unreachable today (`verbs.go:220` sets Holder only for MAIN) but a hazard if holdership widens. | The store's guard now reads `!(Class==MAIN && Holder)`, matching the matrix. |
| GC-R2-03 | noted | Up to three git subprocesses run under the store lock; no-prompt commands, latency-only, no timeout convention exists in internal/goal to align with. Accepted as is. | none |

Round closed by join: 9 findings, 9 dispositions. After the amendments:
`go test ./internal/goal/ ./internal/authority/ ./cmd/metasystem/ -count=1`
green; adopt-fixtures.sh and validate-kit.sh reran green (word-budget
override 1500 for the pre-existing 86bd66a blocker, see the report).

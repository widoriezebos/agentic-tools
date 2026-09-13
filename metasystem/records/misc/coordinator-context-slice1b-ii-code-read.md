# Opus reads of coordinator-context slice 1b-ii part 1 (2026-09-13, code-critique)

## Round 1

# CCB slice 1b-ii part 1: Opus read of the carried tip build

Worktree `.claude/worktrees/ccb-1bii-tip` at 3dbac92e. Every change is staged, so `git diff` at the root shows nothing, and the seat's copy `ccb-1bii.diff` is 0 bytes. I read `git diff --cached` instead (18 files, 963 lines). `ccb-1bii-raw.diff` differs from it at line 137.

## Verdict

**4 material findings. Not fit to land by human commit.** F-1 means the dispatch bed fails as written. F-2 is a regression on any dispatch refused after its body was staged.

## Material findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | high | yes | The `large-brief-is-referenced-not-refused` leg fails at its `cmp`. The bed stops there, so `reference-mismatch-refuses-launch` never runs. That leaves rows 07 and 10 without fixture proof. | `dispatch-fixtures.sh:2262` compares the staged file with `$large_brief`. But `dispatch.sh:1714-1724` appends `## Declared Outputs` and the manifest digest to every design-critic brief, and 1763 composes that version. So `staged/task-direction.md` is the large brief plus that section, `cmp -s` fails, and the leg hits `exit 1`. The brief dictated this line, but the named fixture still fails. The right comparison is `artifacts/agents/large-brief/brief.md`, the copy made by `cp "$brief" "$payload/brief.md"` (about line 1851). |
| F-2 | high | yes | Staging writes into the final round directory before claim-launch wins. If the dispatch is then refused, `artifacts/agents/<job>/` is left behind, and retrying the same dispatch dies with `job payload collision`. | `composition.go:281` stages the task direction before recipe and continuation refusals can happen. After compose, `dispatch.sh` can still refuse: packet bound (1772), preflight, goal and slice admission (1799-1801; SLICE_CAP_REFUSED tells the caller to approve and retry), and LOCK_BUSY. None of these creates `jobs/<job>.json`. `fail_setup_husk` (312-320) removes no payload. On retry, `dispatch.sh:1636-1637` refuses because `$agents/$job` exists without a record. Default ids are derived only from goal, revision, role and brief digest (`operation.go:35`), so the same retry always gets the same id. Before this change, refused paths wrote only under `$record_locks`; `mkdir -p "$round_dir"` runs after WON. |
| F-3 | medium | yes | For the same reason, a refused wrapper can replace a live job's verified referenced body. | Scenario: `follow-up --job X --message other.md` (over 32 KiB) runs while round N is pending or running. The repeated path (2323-2348) has no message-hash check before compose. Compose at 2643 atomically rewrites `rounds/N/staged/task-direction.md`, and only then does preflight refuse on the fingerprint. The chain lock is released right after `__launch` (1937-1940), before the adapter verifies. If the rewrite lands before verification, the job fails with `reference_mismatch`. If it lands after, the running delegate reads bytes nobody verified. That breaks row 08 ("the path that was verified"). |
| F-4 | medium | yes | The exhaustion-discipline gate reads the successor's `prompt.md` for the open finding ids. A referenced task direction moves those ids out of the prompt, so merge conformance refuses. | `validate/conformance.go:1041-1056` and `authorization.go:317-333` read `rounds/<n>/prompt.md`, and `conformance.go:985-995` fails with `prompt does not enumerate open findings`. An implementer follow-up message over 32 KiB is now staged, and the prompt carries only the stanza. No test covers this path, and neither the brief nor the design names this reader. |

## Not material (recorded, not blocking)

- **N-1:** `fake.go` `workingMode` falls back only to `staged/task-direction.md`. A fresh-context follow-up whose prior brief is staged and whose message has no header gets `implement`. This is what the brief asked for, and no leg exercises it.
- **N-2:** `fake.sh` `behavior_present` greps `prompt.md`, so `FAKE:` markers inside a staged brief are silently ignored. This is a trap for future fixtures.
- **N-3:** No test covers `escapes-root` or verb exit 2. There is a TOCTOU gap between `Lstat` and `ReadFile` in `references.go:54-60`, outside the threat model.
- **N-4:** The Go tests write under the real `artifacts/agents/test-<Name>`, as the brief requires. A crashed test leaves those directories behind.
- **N-5:** The `open=` field of the mismatch line is ambiguous when the root path has spaces. The shell parses only `path=`, so nothing breaks.

## Checked and clean

- **Build and static checks:** `go build ./...`, `go vet` and `gofmt -l` on the four packages, and `staticcheck@v0.8.0` on dispatch, adapter, refusal and cmd/metasystem produced no output.
- **Race tests (`-race -count=1`), all ok:**
  - `internal/dispatch -run 'ComposeRolePacket|VerifyReferences|JobRecord|Grammar|Freeze|Corpus|Composition'`
  - all of `internal/refusal` (including HCL03 with `references.go:19`)
  - `internal/adapter -run 'WorkingMode|WriteFakeReturn'`
  - `cmd/metasystem -run 'VerifyReferences|ComposeRolePacket'`
- **Test diff:** test files have additions only (0 deleted lines), so no existing test was changed.
- **Part 2:** none of its files are touched.
- **Byte identity at or under the limit:** the new `appendRange(slot, source, raw, raw)` produces the same bytes as the old `appendSource` body. The record gains `references: []`. No strict schema or other reader of the composition shape exists outside `build.go`.
- **Carry onto the tip:** 57ba0586 added only `wait-delivery` to each adapter and `runtime-common.sh`. It prints `blocking` and starts no process, so there is no new launch path.
  - The check runs immediately before `mark_cli_prefork` at `claude.sh:143`, `codex.sh:152` and `devin.sh:572`.
  - At `devin.sh:338` it runs before the ACP server fork, and a failure removes both FIFOs before anything is started.
  - In `fake.sh:211-225` it runs before `__handshake`.
  - The only other exec is `devin.sh:205`, the same-session repair turn that F-1 left out.
  - On failure there is no prefork marker and no child; the record fails from pending with phase `launch`. The patience check treats it as never started (no effectiveModel).
- **Shell options:** every adapter runs `set -euo pipefail`. Callers use the function in `||` context, so its `set +e`/`set -e` pair changes nothing outside it. The `sed | head` output is a few lines written in one go, so SIGPIPE is not a realistic risk.
- **Fixture knobs:**
  - `supervise launch-detached` passes the parent environment through (`cmd.Env = append(os.Environ(), …)`, `supervise_arming.go:90`).
  - Bash exports prefix assignments on a function call to its children (tested).
  - `config.Get` checks `METASYSTEM_DISPATCH_MAX_INLINE_INPUT_KB` first (`resolve.go:67`).
  - The design-critic recipe sources total 18,783 bytes, so the 8 KiB packet-bound leg refuses as intended.
  - The tamper leg has the same shape as the working `handshake-failure` leg.
- **OpenPath:** it is built from the resolved stage directory, so it is absolute under the control root, and the stanza names it. The read root `"."` expands to the repository top level, which contains the control root.
- **Staging refusals:** a stage dir outside the root, a symlink escape and a missing stage dir are all tested. A body of exactly 32 KiB stays inline.
- **Admission:** `build.go` binds each reference's slot, digest and byte count to a source, and the dishonest case is refused.
- **Verify-references:** decoding is strict (unknown fields and trailing values refused), every reference is re-read, and exit codes 0/9/1 are tested.

Material finding count: 4

## Round 2 (closing read)

# CCB slice 1b-ii part 1: Opus closing read, round 2

Worktree `.claude/worktrees/ccb-1bii-tip/metasystem` at 3dbac92e. I read the unstaged `git diff` (21 files, including the two intent-to-add files) against the r1 read, the fold brief, both result rounds, the original brief, and design section 8b.

## Verdict

**1 material finding. Not fit to land by human commit as it stands.** F-1, F-2 and F-4 are closed in code, and each has a proof that fails without the fold. F-3 is closed in code, but its fixture proof is vacuous: the fold's edit to that leg removed the hold it depends on.

## Material findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| R2-1 | high | yes | The fold broke the `repeat-follow` leg. Round 2 no longer holds under custody, so the F-3 assertion runs against a completed round and would pass without the fold. The leg's existing contention assertions also become load-fragile. | **Round 2 cannot see the marker.** `dispatch-fixtures.sh:2970` moves `FAKE:custodial-critique` into the parent brief, and 2977 replaces it in the message with 40 KiB of filler. The fake adapter looks for markers only in the current round's prompt (`fake.sh:62`, `prompt="$round_dir/prompt.md"` at 168, read at 237). The fake `current` profile has `resume=true` (`fake.go:271`; `select.go:165`), so this follow-up is resumed. Only fresh-context rounds get `prior-brief` (`dispatch.sh:2622-2626`). The design-critic recipe carries only the role file, the skill and the schema (`role-packets.json`). So round 2's prompt holds the reference stanza and no marker, and round 2 completes as soon as it launches.<br><br>**Consequence 1, contention.** `wait_for_chain_lock` plus the 50 ms chain-lock poll (`dispatch.sh:555`) now decide whether the second wrapper sees `pending`/`running` (BOUND or IN-PROGRESS) or `completed`. If it sees `completed`, it dispatches r3, and the assertions at 2996-3003 fail. Before the fold, this ordering was deterministic.<br><br>**Consequence 2, F-3.** At 3011, after both wrappers have exited and several engine calls have run, r2 is complete. The third wrapper takes the new-round path (round 3) and runs the design-critic register and exhaustion advances. Its preflight on opid r3 with r2's operation id then returns REFUSED-OPID-MISMATCH `operation-id-bound-to-another-job` (`claim.go:103-112`). Compose never touches `rounds/2/staged`, with or without the fold, so the `cmp` at 3013 passes vacuously. If an advance refuses first instead, the leg fails.<br><br>**Artifact to change:** the `repeat-follow` leg in `scripts/agents/dispatch-fixtures.sh`, or `fake.sh` `behavior_present` (r1's N-2), so that round 2 holds while the changed follow-up runs.<br><br>Confirmed by reading; no bed was run. |

## F-1 to F-4, closure

- **F-1: closed.**
  - `dispatch-fixtures.sh` now compares the staged copy with `artifacts/agents/large-brief/brief.md`.
  - That file is `cp "$brief"` (`dispatch.sh:1873`), where `$brief` is the same return-path-form file passed to compose.
  - Compose stages the raw file bytes (`composition.go` `os.ReadFile(p.Brief)`), and `atomicfile.WriteText` does not transform them. The old comparison against `$large_brief` fails.
- **F-2: closed.**
  - Compose writes only to `mktemp -d $record_locks/composed-staged.*` (`dispatch.sh:1771`, 2673). `OpenPath` comes from `ReferenceDir`.
  - Every refusal between compose and WON exits through the EXIT trap installed at 1645 and 2595, which calls `cleanup_composition_temporaries`. That covers the compose refusal, the packet bound (`die`), preflight (`return`), goal and slice admission, LOCK_BUSY and a non-WON claim.
  - The cleanup variables are globals initialised at 89-91, so `set -u` is safe.
  - WON only means a new reservation was created (`claim.go:574`), so `$round_dir/staged` cannot already exist on either path.
  - `mv` is a same-filesystem rename under `$agents`, so it cannot move part of the directory.
  - The packet-bound leg plus its retry would fail before the fold. The Go test pins `OpenPath` under `ReferenceDir` while the stage sits in record-locks.
- **F-3: code closed; proof vacuous (R2-1).**
  - A refused follow-up writes only its temporary directory.
  - Promotion sits beside the prompt and composition `mv` on both paths (1874-1883, 2781-2790), after WON.
- **F-4: closed.**
  - `successorTaskDirection` (`conformance.go:913`) returns the prompt when there is no `composition.json` or no `task-direction` reference, which covers legacy rounds and records with `references` null or `[]`.
  - Otherwise it reads bytes through `ReadVerifiedReference`: root-bound, `Lstat` regular file, digest and size checked.
  - A mismatch, an unreadable prompt or composition, or a duplicate reference fails closed and names the `REFERENCE_MISMATCH` line.
  - Both new tests fail without the fold: a stanza-only prompt does not enumerate F-9, and the refusal text differs.
  - The shell assertion `conformance-fixtures.sh:203` still gets its text, because the prompt is present and there is no composition.

## Round 1 clean checks re-run

- **Build and static checks:** `go build ./...`, `go vet` on the five packages, `gofmt -l` and `staticcheck@v0.8.0` (from the local module cache) all produced no output.
- **Race tests (`-race -count=1`), all ok:**
  - dispatch `ComposeRolePacket|VerifyReferences|JobRecordRejects|Composition|Grammar|Freeze`
  - validate `Exhaustion|Conformance|Authorization|Warden`
  - adapter `WorkingMode|WriteFakeReturn`
  - cmd/metasystem `VerifyReferences|ComposeRolePacket`
  - all of refusal
- **Byte identity:** round 2 did not change the inline path of `appendBody`. The exact-limit test stays byte-identical with and without a stage.
- **Adapters:** unchanged since r1.
  - Verification still runs immediately before `mark_cli_prefork` (`claude.sh:143`, `codex.sh:152`, `devin.sh:338` and 572) and before `__handshake` in `fake.sh`.
  - `wait_delivery` (`runtime-common.sh:15-29`, `fake.sh:366-379`) only prints `blocking`.
- **OpenPath before promotion:** nothing reads it. `enforce_inline_input_limit` and the input hash read only the prompt temp. `build-record` and `readCompositionForJob` run after the `mv` and check shape only.
- **ReferenceDir escape:** the resolved final directory must lie inside the root, and there is a test for a final directory outside it. If the directory is swapped for a symlink after compose, `mv` lands outside and adapter verification then refuses with `escapes-root`.

## Not material

- **N-1:** `internal/evidence/gc.go:609` skips non-regular entries in record-locks. A `composed-staged.*` directory left behind by a SIGKILL between compose and promotion is never swept, although stale temp files are.
- **N-2:** at `dispatch.sh:1769-1774` and 2671-2676, if `mktemp -d` fails after the two file `mktemp`s, those two files leak, because the cleanup variables are set afterwards.
- **N-3:** the F-4 tests call `exhaustionDiscipline` through a closure built in the test. The one-line production closures in `mergeCritique` and `wardenReviewFailures` are never run with a referenced successor.
- **N-4:** when a staged task direction exists, only that body counts as enumeration. Ids that appear only in other prompt slots no longer count. This is stricter and fails closed.
- **N-5:** fixtures prove only the packet-bound refusal path. The preflight, admission and LOCK_BUSY paths share the same trap and are verified by reading only.

Material finding count: 1

Disposition (seat): round 1's four findings folded by Codex round 2; round 2's R2-1 (the repeat-follow leg's marker) folded by the seat: the marker rides in the staged follow-up message and the fake adapter reads markers from the staged task direction too. The whole dispatch bed passed standalone on the landed tree.

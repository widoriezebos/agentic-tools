VERDICT: land (0 material findings)

# Read: fixture-children unit 5f, the third fix (supervisor tests count a helper's custodian by exact identity)

Commit read: a5923a8ddbfd29390354333cc1b9ffdd58543a6a (rebound from b9286339e by the coordinator; same tree 24a36b8f,
same parent a1f29152e, now carries `Goal-Unit: fixture-children-cannot-outlive-their-test/5f`). The fix diff read is
`2a7630029..2f7c3ad55`, which touches only `internal/proofrun/supervisor_test.go` (+140 -26).

## Findings

No material findings.

1. Not material. `internal/proofrun/supervisor_test.go:586-592` (checklist item 6). The new witness proves two
   things: a custodian that is not excluded makes it fail, and so does exclusion by pid alone. No witness separates
   the `err == nil` and `state == identity.Alive` conditions. Dropping either one would still pass, but the change
   would be harmless: a failed or dead probe returns an `Exact` that does not match the announced ref, so
   `SameIdentity` still keeps the process. The brief did not ask for these conditions to be witnessed.
2. Not material. The commit message of a5923a8dd, the record rather than the code. It says the unit was "Built by
   Codex over three rounds". Its seat check names only testenv, testutil, identity and the census witness. It says
   nothing about the `internal/proofrun` hang, the helper readiness protocol, or the custodian accounting in
   `supervisor_test.go`, all of which are in the tree. The trailer and the content are right; the message tells only
   part of the story.
3. Outside this read, not material to the fix. `internal/proofrun/supervisor.go:259,265,281,330,335,342` together
   with `signalProcessTree` at :402. On a verdict, a cancel or a reader failure, the supervisor sends SIGKILL to each
   pid in `lastMembers`. While its owner lives, a real run's custodian is one of those members, so it dies with its
   owner and cannot run its cleanup. Any fixture child that has already left the tree survives that kill. Supervision
   still never waits on the custodian (see item 7 below). The seat may want to check whether the goal's design covers
   a supervisor kill.

## Checklist

1. Commit binding: holds. The +/- lines and file headers of `git diff a5923a8dd^ a5923a8dd` (tree identical to
   b9286339e) match `git diff 4e3517338 2f7c3ad55` exactly: 11 files, +415 -97. Every file's post-image blob is the
   same, except `testing.json`, whose base differs between a1f29152e and d66e323c2. Its change lines are still
   identical. Nothing else is in the commit.
2. Exact identity: holds.
   - `excludeSupervisorHelperCustodians` (:584) excludes a pid only when `KernelProber.Probe` succeeds, reports
     `Alive`, and `identity.SameIdentity` matches the announced ref. A dead pid, a reused pid, a mismatched start and
     a probe error all keep the process.
   - Only pids in the excluded set are removed from `Members` and `MemberCPU`, so every other process still counts.
     `RetainVanishedMembers` and the other sample fields are kept.
   - The probe runs after the tree sample, and that is sound. The ref was announced while the custodian was alive,
     before any real sample (the readiness gate at :415 never calls the inner reader before readiness). A later match
     on pid and start means the pid in the sample was that same custodian.
   - Readiness parsing (:810) rejects the old bare `ready`, `ready ` with nothing after it, and any bad ref. Refs are
     `pid=..;micro=..` or `pid=..;ticks=..;boot=..`, and `EncodeRef` refuses a boot ID containing `;` or `|`, so the
     `|` separator is safe.
   - Nested helpers propagate refs recursively. Each announce puts its own custodian first and then the refs its
     nested helper announced, so reaped-child announces all three custodians of its chain. Every custodian starts in
     `testenv.Main` before `m.Run`, so all of them exist at readiness.
   - There is no data race: `fixture.custodians` is written before the send on `ready`, and read only after that
     receive.
3. Witness strength: holds.
   - setsid descendant: after exclusion, `observedMembers >= 2` and `len(Members) < 2` need the busy-forever process.
   - Section result growth: the same, for stage-child.
   - Darwin `retained-child`: any non-root member left after exclusion is the nested busy-forever process.
   - Linux `reaped-child`, read against `supervisor_linux.go`: 3, 2 and 1 now mean root, reaping-member and
     busy-forever, then root and reaping-member, then root. After its owner exits, a custodian is reparented out of
     the ppid tree; no subreaper is set anywhere in `internal`. Excluding custodian CPU cannot make the counter drop:
     a custodian is never waited on by its owner, so its CPU never enters the owner's reaped-children ticks.
   - The seat's VM run shows `reaped_child` passing on Linux (`check-5f-r3.out`).
4. Coverage: holds. Real process readers are used only at `supervisor_test.go:232` and :264,
   `supervisor_darwin_test.go:45` and `supervisor_linux_test.go:63`, and all four go through `helper.gate`, which
   always wraps the reader. Every other member or CPU counting site uses scripted samples.
5. Prohibitions: hold. The fix adds no comment, sleep, retry, time-bounded wait or duration assertion (grep of the
   added lines: 0). It adds no way to skip a custodian: a helper without one exits 35. The fix does not change
   `testenv.Main` or the production reader, sampler or supervisor.
6. Witness mutation, my own: holds. I changed the condition at :589 to `exact.Pid == ref.Pid`, which excludes by pid
   alone. `go test -count=1 -timeout=60s -run '^TestSupervisorHelperCustodianAccountingUsesExactIdentity$'` failed
   at once, in 0.00s:
   `supervisor_test.go:647: identity-mismatched sample = {... Members:[98799] ...}; want both processes retained`.
   The seat's mutation (`mut-5f-r3.out`) shows that removing the exclusion makes the new witness fail at :637, while
   the three process witnesses time out under a 20s bound.
7. The production question: the builder's "no" is correct.
   - `superviseCommand` returns on `command.Wait()` (:212, :262), and every kill path then waits only on that root
     Wait.
   - The callers give pipe-backed writers as Stdout and Stderr (`test_build.go:489,894`, `test_go.go:142`), so Wait
     does block until every writer closes. But `startFixtureCustodian` (`testenv.go:225`) starts the custodian with
     stdin and stdout on /dev/null, stderr on its own log, and ExtraFiles holding only its own two pipes. Go marks its
     other descriptors close-on-exec, so the custodian holds none of the supervisor's pipes.
   - The custodian is `Setsid`, so it is not in the suite's process group either; `reconcile.go:194` reads live
     members by pgid.
   - Finding 3 is the other direction: the supervisor kills the custodian rather than waiting on it.
8. Comments: hold. The fix adds no comments. The old `//lint:ignore` lines are unchanged, and the commit message is
   not a code comment.

## Runs in my copy (git archive of b9286339e, the same tree as a5923a8dd)

- `go test -count=1 ./internal/proofrun/`: ok in 37.2s.
- `go vet ./internal/proofrun/` and `GOOS=linux go vet ./internal/proofrun/`: both clean.
- The mutation above: red as expected.
- My runs left no test processes and no custodian logs. The 167-byte `metasystem-test-registry-*.custodian-*.log`
  files in `/tmp`, dated 12:41-12:43 local, came before my runs and hold the sandbox's `sysctl kern.proc.all:
  operation not permitted`. I left them alone. The copy is deleted.

Every checklist item was checked. The Linux helper was read, not run, as the brief says.

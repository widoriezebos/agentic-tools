# g1-s1 Server lifecycle: design critique

Critic: Opus 5, fresh context, not the author. Designer and adjudicator: Claude. Materiality test: would an implementer working from this design build something different, or wrong, because of this finding?

## Round 1, 2026-09-21

Verdict: 11 material findings. All accepted; none refuted. The critic read source only and executed nothing. It verified every code reference in the design; two were off by one line and one named the wrong file.

| Finding | Problem | Disposition |
| --- | --- | --- |
| F1 | The design claimed that an UNTRACKED interface server costs only a misleading line in the engine's `stop` output. False. The steward counts UNTRACKED processes (`internal/steward/census.go:74`); with no live worker it returns unknown and notifies instead of reviving (`verdict.go:130`); handoffs are held (`handoff.go:101`); the end-of-turn watchdog lists the process until it is acknowledged (`internal/supervise/watchdog.go:135`). A running interface suppresses automatic revival for the checkout | **Accepted in round 1; reversed in round 2, see N1.** The designer verified only the steward's handling of an UNTRACKED process, not whether the server is ever classed as one. It is not. The round 1 text: the design now states the real consequence. `g1-s17` is rescoped to the census, the steward, the handoff hold, the watchdog, and the `stop` wording, unblocked, and moved to directly after this slice. This slice's build is unchanged; its record already carries the identity the census will match |
| F2 | `Read` and `Stop` removed a dead record without holding the lock, so a status check racing a new server could delete the new server's record and leave it running but invisible | **Accept.** New rule: the record is created, replaced, or removed only by a process holding the lock. Readers probe the lock without blocking and clean up only when they win it, after re-reading. A lock held with no live record is a new state, `busy` |
| F3 | `ui stop` had no output or exit code for the stale and uninspectable outcomes, and `Restart` had to branch on a timeout that `Status.State` could not express | **Accept.** `Stop` returns its own outcome type with every case, each with a line and an exit code |
| F4 | Check 3 refused a top-level navigation from a link on another page, which sends `Sec-Fetch-Site: cross-site` | **Accept.** A GET with `Sec-Fetch-Mode: navigate` and `Sec-Fetch-Dest: document` is exempt. The response is not readable by the originating page and framing is denied separately |
| F5 | The parent rotated `server.log` before launching, so a refused second `start` rotated a live server's log away | **Accept.** The parent never rotates. The child trims the log only after it holds the lock |
| F6 | The end-of-file branch of the readiness read could not fire unless the parent closed its own copy of the pipe's write end, which the design did not say | **Accept.** Stated. A partial line is treated as no line |
| F7 | `ui status --json` had no schema | **Accept**, by removal. The simple case comes first (D11); the exit code and the health route cover machine use. It returns when a consumer needs it |
| F8 | The launch and handshake sat in `cmd/`, against the plan's convention, and were declared untestable | **Accept.** They move to `lifecycle.Launch` behind an injected spawn seam and are tested in process. Only the thin real spawn is left to the walkthrough |
| F9 | The Host allow-set broke on port 80; `localhost` as a listen host resolves ambiguously; port 0 was not stated as valid | **Accept.** The listen host must be a loopback IP literal, and names including `localhost` are refused. Port 0 is valid. The allowed Host set adds the bare forms when the port is 80 |
| F10 | `go-build.sh` stamps every dirty tree `dev-<HEAD>-dirty` (`scripts/agents/go-build.sh:75`), so the build-difference line stayed silent during exactly the server development D10 is for | **Accept.** The record carries a SHA-256 of the serving executable; `status` compares it with the executable that was invoked. The stamp is shown for information only |
| F11 | An unparseable or future-schema record had no defined outcome, and produced the same wedge as F2 | **Accept.** New state `unreadable`, resolved by the same lock probe: removed under the lock when nobody serves, reported and left alone when somebody does |

Non-material findings, folded in because they were free: the `KernelProber` citation; "kill the child" against "no SIGKILL" (the launch may kill its own never-ready child; `stop` never force-kills); `restart --listen` against "the same address" (restart resolves the address exactly as `start` does); the unreadable-record message of `AlreadyRunningError`; `<n>` used for both pid and seconds; a new server taking the lock during a stopper's wait (on timeout the stopper re-proves the original identity and reports stopped when it is dead); the `attention.go` pattern resolving through a different function than the design specifies. Not actioned: the blocked `flock` goroutine on the timeout path is never released, which is harmless because the command exits.

## Round 2, 2026-09-21

A fresh critic, scoped to the round 1 corrections and what they introduced. Verdict: 5 material findings, all accepted. It confirmed F2 to F9 and F11 as corrected, F10 as corrected on the writing side only (see N4), and F1 as **not** corrected, because F1 itself was wrong.

| Finding | Problem | Disposition |
| --- | --- | --- |
| N1 | Round 1's F1, and revision 2's "known limitation", are false. The census never sees the interface server. It filters the process table through the configured runtime signatures before it classifies anything (`internal/census/run.go:177`, `signature.go:84`), and those signatures match only a command named `claude`, `codex`, `devin`, or `devin-delegate-acp` (`scripts/agents/adapters/claude.sh:224`, `codex.sh:215`, `devin.sh:806`). `bin/metasystem ui serve` is never enumerated, so it is never UNTRACKED and nothing downstream applies | **Accept; F1's disposition is reversed.** The designer verified this in source, including the three signature patterns. In round 1 the designer had checked only the downstream half of F1's chain, the steward's handling of an UNTRACKED process, and not whether the server would ever be classed as one. The design now states that the engine does not see the process, the warning is withdrawn, and `g1-s17` is dropped in the plan |
| N2 | `Stop` on `busy` could evaluate to `running`, which had no line, no exit code, and no stated control flow | **Accept.** `Stop` dispatches once more after the wait; a `running` found then is signalled; a second `busy` is returned. `StopOutcome` is enumerated and never `running` |
| N3 | `StopOptions.WaitExit` appeared in no behaviour and, injected, bypassed the lock that record removal depends on | **Accept.** Removed. The only seam is `After`, the timer. Tests hold a real lock, which works because `flock` belongs to the open file description, so removal is always under a lock that was really taken |
| N4 | Nothing exported a way to compute the current executable's digest, so `cmd` would have duplicated the hashing, and neither side said what a hashing failure does | **Accept.** One exported `lifecycle.ExecutableDigest`. `Serve` fails if it cannot hash. `status` prints a line saying it cannot compare, and still exits 0 |
| N5 | The child's arguments were never specified, so `--listen` and `--metasystem-root` given to `start` could be silently dropped | **Accept.** `lifecycle.ServeArgs` builds the exact list from values the launcher has already resolved and validated, and it is tested |

Non-material findings folded in: the bounded lock wait is now a named technique with a rule for a late acquisition; a reader that wins the lock removes whatever record is there without probing again; both `start` refusal lines are stated to come from the child; the walkthrough no longer claims the log is byte-identical after a refused start. Not actioned: a second `start` against an uninspectable server costs the full lock wait before refusing; `Wait` can be spent twice on the busy-then-signal path; the parameter order of `NewHandler` and `httpd.New` differs.

Lesson recorded for later slices: a finding that reverses a premise is verified along its whole chain, from the first cause to the claimed effect, before it is accepted.

## Round 3, 2026-09-21

A fresh critic, scoped to N1 to N5. Verdict: 2 material findings, both accepted. N1 to N5 confirmed as corrected.

On N1 the critic was asked to break the claim along its whole chain, because it had flipped once. It confirmed the filter at `internal/census/run.go:177` and the three argv0-anchored signatures, and then checked every other enumeration in the engine: the mission, job, proof-run, run, and steward stop families read records; the untracked family re-runs the same filtered census; the supervision family requires an owner tag and a `--component` argument in argv (`internal/supervise/arming.go:701`); the janitor selects from registry claims; run owners and announcements are driven by their own records; the mission runner sums a tree rooted at its own pid, and the detached server is reparented away from it; the lease's ancestry walk recognises steward plumbing by its second argument and would walk past the server. Result: nothing in the engine sees the interface server.

| Finding | Problem | Disposition |
| --- | --- | --- |
| R1 | The launcher opened the log in a directory only the child created, so the first `ui start` in any checkout would fail at spawn, with no outcome row; and `Read`'s lock probe had no rule for a missing directory, which also left open whether a read-only `status` writes into a checkout | **Accept.** `Launch` and `Serve` create the directory; `Read` and `Stop` create nothing and report `stopped` when the directory or lock file is missing; a cannot-launch outcome row is added |
| R2 | The listed test "a lock acquired after the bound fired is released" could not be written with the stated seams without a sleep or a poll | **Accept.** The unexported bounded-wait helper returns a completion channel that the in-package test waits on |

Non-material, folded in: the options field is renamed `DigestFunc` so that `ExecutableDigest` names one thing; `LaunchSpec.Executable` is `os.Executable()` resolved by `cmd`; port 0 and `restart` are stated.

**Loop closed at round 3.** Material findings fell from 11 to 5 to 2, and both remaining findings are expressible as tests. They become obligations O1 and O2 in the design's Verification section, which the code critique checks by name. A fourth round would be polishing.

## Astra design review, 2026-09-21

Verdict on this slice design: implement as written. No finding of the [Astra review](../user-interface-design-critique-astra.md) touches it. Building still waits for the human's explicit go.

## Astra's second design review, 2026-09-21

Finding B8: the slice resolved its checkout with `upRepositoryScope` and kept its state under it, while D20 said every home of state is located through the state-root owner, which in the self-hosted layout is the nested installation. The two readings give different lock identities. Astra's verdict was that the slice could no longer be implemented as written.

Disposition: partly agreed. The ambiguity was real and is removed in revision 5 of the design and in the master. The direction of the fix is not the one Astra suggested. The engine keeps two conventions in the self-hosted layout, checked in a template clone in active use: process families, the steward and the supervision, keep lifecycle state under the Git checkout (`cmd/metasystem/up.go:45`, `:169`, `process_verbs.go:52`), while the goal journal, the channel, and jobs keep domain state beneath the installation. The interface server is a process family and was modelled on the steward runner, so its state stays where it was designed, and D20's sentence was narrowed to the subject's state. No behaviour changed, so the design stands as implementable and the existing build still conforms. The human has the last word.


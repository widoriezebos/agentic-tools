# Build: remove the retired relayed-word path

Goal: `no-code-carries-the-relayed-word-path`
Base: `origin/main` 01bc1e82c (the code surface below was measured at
30f41710a; 01bc1e82c only adds this goal's approval to the ledger).
One chain. Kind: build.

## What Wido asked for

On 2026-09-20: "Remove the temporary word functionality. I need clean
code."

## Why this is a deletion and not a design question

R-82-m1b (`metasystem/memory/rulings.md`, 2026-09-07) already retired the
relayed-word path: `TemporaryGoalAuthorityHorizon` stays `2026-09-06` and
is not renewed, so `--temporary-human-word` with `--review-by` refuses
every date from 2026-09-07 on. The row itself says the two rules in
`metasystem/internal/humanauthority/authority.go` are mutually
unsatisfiable once the horizon is past. The flag has been unreachable for
thirteen days. That row ordered an enforcement sweep the same hour; the
sweep stopped at the fixture legs. You are finishing it.

Nothing here changes behaviour that any caller can reach today. If you
find a reachable caller, stop and report it instead of deleting it.

## THE ONE HARD CONSTRAINT: stop writing, keep reading

Landed goal records already carry the relayed-word fields. The record
grammar is closed, so a reader that forgets a field which landed records
contain refuses the **whole ledger**, not just that line. This cost two
refusals on 2026-09-20 alone: `goal migrate` refused a legacy ledger over
a `- Risk:` line, and `goal open` refused with `Approved: unknown key
"episode"`.

So: remove every code path that **writes** or **accepts** a relayed word,
and keep every code path that **parses** one out of an existing record.
`metasystem/internal/goal/file.go:1267` and `:1278` show the shape to
preserve — a tolerant `parseKVRecord` key lookup. Do not narrow it.

## Do not remove

The authenticated-channel authority is the live replacement, not part of
this cut. Keep `AuthorityOutcomeAuthenticatedChannelWord` and
`AuthorityOutcomeVerifiedChannelAnswer`
(`metasystem/internal/goal/attention.go:943`,
`metasystem/internal/humanauthority/authority.go`) and both channel paths
in `metasystem/internal/channel/poll.go`. Keep the power-of-attorney
verbs and their `--expires`/`--tiers`/`--verbs` flags; they are a
different mechanism.

Treat `--review-by` per call site: delete it where it exists only to
accompany the relayed word, keep it where another act uses it. Report
which way you went.

## The surface, measured at 30f41710a

Production:

- `metasystem/cmd/metasystem/goalsync_mutations.go:209` and `:1057` — the
  two flag registrations
- `metasystem/cmd/metasystem/goal_refusal.go:30` and `:101` — the remedy
  that re-runs the command without the flag, and the site that appends it
- `metasystem/internal/humanauthority/authority.go` —
  `validateTemporaryGoalAuthority` and the `temporaryGoalProofAt` seam
- `metasystem/internal/governance/types.go:103` —
  `TemporaryGoalAuthorityHorizon` and `TemporaryGoalAuthorityRuling`
- `metasystem/internal/goal/file.go:302` — the "horizon has passed"
  message
- `metasystem/internal/goal/stop.go:472` — `recordTemporaryRelay`, the
  writer
- `metasystem/internal/steward/identity.go:66` — the persisted
  `temporaryHumanWord` field
- `metasystem/internal/steward/runner.go:344`, `:711`, `:858` — the
  rebuild carry and the status suffix

Sixteen production files and fourteen `_test.go` files match
`temporary-human-word`, `TemporaryHumanWord`, `temporaryGoalProof` or
`TemporaryGoalAuthority`. Find them with `git grep` rather than trusting
this list to be complete. Delete tests that only cover the removed path;
do not weaken a test that covers something else.

Fixtures: `metasystem/scripts/agents/goal-cli-fixtures.sh` still passes
the flag.

Governance: mark R-32-m1 swept in `metasystem/memory/rulings.md`, under
R-82-m1b, so no row points at deleted code. One row edit, no rewriting of
history.

On-disk note: `artifacts/agents/steward/identity.json` on this machine
carries no `temporaryHumanWord` key, so nothing has to be migrated. An
identity file that does carry one must still load.

## Units

| unit | lines |
| --- | --- |
| rwr | 1500 |

Almost all of it is deletion. If the cut runs past the budget, do the
file list in the order given above, stop cleanly and say what is left.

## Deliverable

NAME is `rwr`. Return the three files the rules below require, under
`metasystem/artifacts/reports/`. In `rwr-build-return.md`, besides what
the rules ask for, say what you kept and why, give the `--review-by`
decision per call site, and name any reachable caller you found.

## Checks, on top of the rules below

This is a macOS host: run no Linux tests and report no Linux result.
`internal/goal` tests need `-timeout 40m`. Run
`bash scripts/agents/goal-cli-fixtures.sh` from `metasystem/` as well,
because you are editing it.

## Rules

RULES:
- You are the implementer. Work only inside this worktree. metasystem/ is the Go module root; read metasystem/AGENTS.md first, briefly. Bash and Go only.
- Do not run git commit, git push, git stash, git checkout of other refs, or any git command outside this worktree. Never rm a variable path.
- Do NOT edit metasystem/testing.json. Instead list every new or renamed Go test and fixture scenario in your return, each with the test group you think it belongs to. Never rename or drop an existing test function: a landing cannot drop a listed test.
- Artificial clocks only: no sleeps or wall-time waits in tests, no retry-until-green, no t.Skip, never raise a bound or a targetMs, never lower a floor. A test that needs time gets an injected clock.
- Every rule you add has a named test that FAILS when that rule alone is removed. Prove it for each rule (remove, see red, restore) and say so in the return.
- A new refusal code or token needs its row in the refusal register (TestHCL03EveryCodeRowed). Name any register rows you added in the return.
- Size: at most 1,500 changed lines. If the work is larger, do the items in the order given and stop cleanly; list what is left.
- Machine rule: while /tmp/metasystem-testrun-lock exists, run no tests; wait and retry.
- Two repository guards your sandbox CAN run, so run them by name before you return: `go test -count=1 -run '^TestEveryPackageUsesSharedMain$' ./internal/testenv` (every package with _test.go files needs a TestMain that calls testenv.Main, copy a sibling package's main_test.go) and `go test -count=1 -run '^TestStandardStreamPipesUseSharedCapture$' ./cmd/metasystem` (a test in cmd/metasystem captures stdout or stderr only through the shared helper captureCommandOutput, never its own os.Pipe).
- Callers rule: when you change what an exported function, type or record REQUIRES of its callers (a new mandatory field, a stricter precondition, a changed signature or schema), grep the whole module for every caller, fix the callers and their test fixtures even when they lie outside your file set (tests and fixtures only there; say so in the return), and run the callers' tests that exercise the changed symbol BY NAME. List the callers you found in the return.
- IMPACTED TESTS, BEFORE you return, from metasystem/. Tests target what your change impacts, never whole packages: (a) every test you added or changed; (b) in each package you changed, every test whose file references a function, type, constant, verb, flag or file you changed (grep that package's *_test.go); (c) in each package that imports a changed package (`go list -f '{{.ImportPath}} {{join .Imports " "}}' ./... | grep -E '<changed pkg>( |$)'`), every test whose file references a changed exported symbol; (d) the two guards above. Run each by -run name, plain and with every build tag its package's tests use (e.g. -tags batchtest); -count=3 only for new tests that start processes, goroutines or fixtures. Also run `gofmt -l` on the files you touched, `go build ./...`, `go vet ./...` (compiles every test file in the module) and `bash scripts/agents/go-gate.sh --fast` (static only; a stop at the staticcheck download is expected in the sandbox, every red before it is yours). Fix EVERY red and rerun. A test that fails only because the sandbox blocks loopback listeners, sysctl or the network is listed by name as SANDBOX-ONLY. Paste the impact list (a to d, with the greps you used) and the final output tails in the return. A return with an unfixed non-sandbox red or without these is not done.
- DO NOT RUN: whole-package test runs, fixture beds, the full scripts/agents/go-gate.sh, or anything that needs the network. The integrator runs the whole touched packages once, on the host, when the slices combine. A red found there comes back to you as a fix round.
- RETURN FILES, all under metasystem/artifacts/reports/: <NAME>-build-return.md (what you did per item, the rule-to-test table with the mutation result per rule, the gate outputs, new test names with groups, register rows, anything left open, any file you touched outside the file set and why), <NAME>.diff (git diff of the whole worktree against HEAD, including new files: use `git add -N .` first, never commit), <NAME>-commit-msg.txt (subject line under 72 characters, a body that says what and why).

- Every new test's first statement is t.Parallel() and the test is parallel-safe (no t.Setenv, no os.Chdir, no shared mutable globals or fixture roots; t.TempDir per test). A test that truly needs process-global state gets a reasoned exemption in testing-parallel-ratchet.json instead. The verb 'audit parallel-ratchet' refuses raises at the gate.
- Any human-gated skill (retro, steward, approvals) is OUT OF SCOPE for a build job: never run it, never wait for a ruling on it; if a receipt or cadence says one is due, note it in the return and continue.

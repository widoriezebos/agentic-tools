You are critiquing one design page in a Go repository. Change NO file in the repository. Your working directory is `metasystem/` (the Go module) inside the repository; the tree is checked out detached at commit 4e4e46de7. Your only deliverable is the return file named at the end, which lies outside the repository.

DIRECT MODE, by the repository owner's word (Wido, 2026-09-17): the delivery machinery is OFF for this job. Do NOT run the retro, `receipt.sh`, a task receipt, a critique or conformance ceremony, or any other skill step that `AGENTS.md` or `wow.md` would normally require at the end of a task. Never pause to ask a person for approval of anything: nobody is there to answer. Do not write into `metasystem/memory/`, `metasystem/records/` or the instruction log.

# START STATE CHECK

Before reading anything else, confirm and put the two lines in the return: `git rev-parse HEAD` prints 4e4e46de7e0b1a6a2f7c6b3fdfc6c2f0d7ccd6b2 or a commit starting with 4e4e46de7, and `shasum -a 256 plans/stop-hook-never-forces-an-empty-turn-members-design.md` starts with cd68981cab79448b. If either differs, write only those two lines and the word MISMATCH as the first line of the return, and stop.

# What to critique

The page `plans/stop-hook-never-forces-an-empty-turn-members-design.md` (459 lines, 7,321 words) designs members 3 to 7 of the umbrella goal stop-hook-never-forces-an-empty-turn, one section per member, each section meant to be the whole brief of one build job. Read it whole. Context, read as needed: the umbrella page `plans/stop-hook-never-forces-an-empty-turn-design.md` and the parked member-2 page `plans/stop-decisions-record-deadline-evidence-design.md`. Member 1 is landed (commit c8074077) and is not under critique.

The page must hold against the code as it stands at this commit. The stop-related sources it builds on: `internal/goal/turnverdict.go`, `internal/goal/turnfacts.go`, `internal/goal/project.go`, `internal/goal/stopfence*.go`, `internal/report/stopblock.go`, `internal/steward/tick.go`, `internal/steward/notify.go`, `internal/steward/intervene.go`, `internal/dispatch/` (job records, cap continuation), `internal/run/` (run records), `internal/runtimes/runtimes.go`, `internal/testutil/`, `cmd/metasystem/goal.go`, `cmd/metasystem/goalsync_mutations.go`, `cmd/metasystem/report.go`, `cmd/metasystem/host_verbs.go`, `cmd/metasystem/main.go`, `cmd/metasystem/runtime_conformance_test.go`, `scripts/agents/supervision-hook.sh`, `scripts/agents/supervision-hook-fixtures.sh`, `scripts/agents/adapters/codex.sh`, `scripts/agents/adapters/runtime-common.sh`, `scripts/agents/adapters/fake.sh`, `scripts/agents/dispatch.sh`, `scripts/enforcement/claude-code-hooks.json`, `docs/design/turn-verdict-delivery-contract.md`, `testing.json`.

Rules the page is bound by (from its brief): a test never depends on wall-clock time (injected clocks and fakes; never t.Skip, never a raised bound, never a retry); one mechanism per member, each with its own DONE, landing alone in the page's order (3, 4, 5, 6, 7a, 7b); the engine never names a runtime (Claude and Codex specifics stay in scripts/agents/); no read rule on an append-only record is tightened; no test name that `testing.json` lists is renamed or dropped; no member depends on member 2 (parked); every DONE rule names the fixture that fails without it, and every fixture has one mutation that turns it red.

Observed 08:37 CEST: a headless claude -p delegate ran only the plugin Stop hook (stop-review-gate-hook.mjs); the repository supervision-hook.sh Stop entry did not run, no hooks.log line; member 7 must account for headless hosts.

# REQUIRED CHECKS

Do every check below and report the result of each, even when it passes.

1. Every symbol, verb, flag, record field, file and script function the page names exists at this commit with the shape the page assumes. Grep for each one. The page's own list under `## Not checked` names the places it could not confirm: resolve every item of that list against the tree and say what is there.
2. For every DONE rule in every section: the named fixture can be made red-before-green with the seams the page names, and its mutation in part 5 turns exactly that fixture red. Say where a mutation would leave the fixture green or turn a listed test red instead.
3. Every fixture's setup can be built without wall-clock waits: name any fixture whose described observation needs a sleep, a retry or a real timer, and any process fixture that has no readiness line to wait on.
4. Order and coupling: each member builds only on members landed before it in the page's order and on member 1. Name any dependency on member 2, on a later member, or on two members landing together.
5. Engine boundary: any place where code under `internal/` or `cmd/` would branch on a runtime name (claude, codex) as designed.
6. Protected names: any listed test name in `testing.json` that a section renames, drops or reassigns, and any append-only record (hook log lines, refusal records, job records, run records, verdict state, goal files) whose older form the design would refuse to read.
7. Estimates: for each section, whether the design as written plausibly fits its estimate, and where a section is two mechanisms that cannot land alone.

# What counts as material

A finding is MATERIAL when the page must change before a builder can start the section: the mechanism cannot work against the code as it stands and the fix is more than a rename or a path correction; a DONE rule has no fixture that fails without it; a mutation does not turn its fixture red; a fixture needs wall-clock time; a member depends on member 2 or on a later member; the engine names a runtime; a listed test name is dropped or renamed; an older record would stop reading; a section is two mechanisms. Everything else (a renamed symbol, a wrong line reference, a better name, a missing sentence) is a remark.

For each material finding give: the section and the page line, the tree file and line it conflicts with, a concrete failing scenario (what the builder would do, what would then be wrong), and the smallest change to the page. CITATIONS by symbol: name the function, type or script function, then the file and line at this commit.

# Bounds

Run no `go test`, `go build`, `go vet` or gate: other test runs share this machine. Reading, grep, sed, gofmt -l and `go doc` are fine. No single command may wait longer than 240 seconds. Request the smallest section of a file you need; open a file through a bounded view (sed -n ranges, grep -n), never whole.

# Return

Write the result to the absolute path /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct/stophook-crit-build-return-r1.md (create it; it lies outside the repository). Its first line is exactly `CRITIQUE: material N` with N the count of material findings (0 when none). Then these sections, in this order: `## Start state` (the two lines of the start state check); `## Material` (numbered findings, the fields above; the word none when there are none); `## Remarks` (one line each, at most eight); `## Required checks` (one line per check 1 to 7 with its result); `## Not checked resolved` (one line per item of the page's `## Not checked` list, saying what the tree holds and where). The return contains no angle brackets and no placeholder; it says only what you verified in this tree. Print nothing else.

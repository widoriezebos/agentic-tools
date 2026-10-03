# Brief: rosters-are-configuration-items

Goal state: claimed, tier 2, approved box 2d/12/1800m/1/20; the read has at most 20 rounds.

# Goal

What: A roster becomes a configuration item of its own: a named set that says which runtime and model does each kind of work (design, design critique, build, code critique, and so on). There can be many rosters, each with a type that says where it applies: one per risk tier, one for the landing lane, one for the Project Partner. People see and change them in the Settings page of the interface and with a few plain verbs, instead of editing many separate lines in the configuration file. Why: Wido, 2026-10-01: "I want a goal and a design for rosters as a configuration item, but separate (not a bunch of line level setting in the config). Reason is I want many rosters, one per risk tier, one for the 'lane', one for the project partner. So this needs a roster-type (identifier) and a UI and verb(s) for maintaining them easily. UI in the settings of the metasystem." Today the roster is spread over many role and launch keys in the configuration files, one setting per line, with no way to say "this set of models for high-risk work, that set for the lane". Pros: one place to see and change who does what, a different roster per kind of work, and easy edits from the interface. Cons: the engine has to read rosters instead of today's keys, and existing seats' settings need a clear path over.

Next step on the ledger: Next: Fable writes the design (the roster item and its types, where rosters live, how the engine picks the roster for a piece of work, the verbs, the Settings page, and the path from today's keys), one Astra critique, then Wido decides the build. Done when: rosters of each type can be listed, created and changed from the Settings page and from the verbs, and work of each kind runs on the roster its type selects. Added 2026-10-02 (Wido: 'the running agent; is that defined in the roster which agent and which model? That should be the case'): today runtime/model/effort are scattered over two overlapping key families (launch.KIND.* and role.ROLE.*) and copied per checkout (five seat checkouts on this host). The roster must: cover EVERY agent kind incl. the seat agent and the landing lane agent; be ONE key family; be shared by every checkout on the host (one place to change a model); refuse an unknown key instead of silently falling back; have a 'roster show' verb plus the Settings view; include effort (Wido 2026-10-01).; ASKED 201G4SZ4JZTR470W4ZA13ZFWBY (other): The new rosters design is written, but I can't commit it, and Astra's critique reads only committed files. How should it get committed? Details: Fable's revision 5 of the rosters design (3,990 words) is staged in the m1i checkout. A seat's commit here needs a fully green test run (about 45 minutes per run). Four runs since 06:50 were red, each for a reason outside this change: two problems on main that other seats have since fixed (stale layout goldens, a count of serial tests), deep test mode that can't pass on this Mac, and a test that fails when the computer is busy (load about 30 on 18 cores) because it reads the live process list. The goal's 4-hour claim ends at 10:14; after that no new work starts.; ASKED A9B7CHS894W4ZY7RAE4G7Y1QD5 (other): Astra's critique of the rosters design found one gap, and Fable has fixed it, but I can't commit the fix for Astra's second look. Will you commit it as you did the first draft? Details: the gap was a missing table of which existing behaviour moves to the new rosters (who writes a model setting, who picks the model, who records a refused launch); the new page adds it with 20 rows, and the engine's own check of that table now passes. A seat's commit needs a fully green test run; two more runs since 10:20 were red for reasons outside this change: the test result could not be packaged because the steward appends to a tracked log file during the run, and four mission-runner tests failed under load (43 on 18 cores) and passed on rerun. Each run takes 45 to 70 minutes on this busy computer, and the goal's 7 hours end at 13:14.; ASKED 6AEX293VAZJ9XT3NYD0FBN8C1P (other): The rosters design has passed Astra's critique. Do you want it built, and do you agree with its seven recommendations? What it builds: one rosters file per computer (in ~/.metasystem/host), shared by every checkout, holding six rosters: Tier 1, Tier 2 and Tier 3 work, Seat, Landing lane and Project Partner. Each row names the agent, the model and the effort for one kind of work (design, design critique, build, code critique, and the rest). Nothing falls back to today's settings: a missing row is refused with the one command that sets it. Only a person changes a roster (metasystem roster set); roster show and roster list read them; roster import copies each checkout's current settings across. 15 steps of at most 300 changed lines each; steps 1 to 4 (the store, show and list, set, and the seat and landing agents reading their rows) already answer 'which agent and model does the seat run' and change it once for all checkouts, in about 6.5 hours; all 15 take about 29 hours. Astra's one finding, a missing table of which existing behaviour moves where, is fixed and confirmed. The seven decisions, with my recommendation: (1) a fourth roster type, Seat, for the seat agent and the steward's follow-up session, which serve no single goal: yes. (2) A signed-in browser session may change a roster from the Settings page: yes, for setting a row only. (3) Each checkout keeps an on/off switch for steward-started seats that names no agent: yes, default off. (4) Fixed roster names (tier-1 to partner) rather than names you choose: fixed. (5) Rosters stay out of the proof's fingerprint, since who does the work does not change what a proof proves: yes. (6) A brand-new computer gets no starter roster; its first start is refused with the command to set one: yes. (7) Settings that today carry no effort are imported at xhigh: yes. The page: metasystem/plans/designs/rosters-are-configuration-items.md in the m1i checkout (the fixed version is staged there, not yet committed).; ANSWERED 6AEX293VAZJ9XT3NYD0FBN8C1P: Build as recommended

# Workspace

Branch goal/rosters-are-configuration-items. Its workspace does not exist yet; metasystem work build prepares it. Leave the change there, uncommitted.

# Inputs

- Design: /Users/wido/LocalStorage/GitHub/agentic-tools-m1i/metasystem/plans/designs/rosters-are-configuration-items.md (accepted; the specification this brief builds)

# Units

This brief builds unit 1 of the accepted design (section 8, row 1): the rosters store. Units 2 to 15 are later briefs; build none of them here.

| Unit | Lines |
| --- | ---: |
| store | 290 |

## What unit 1 builds

A new file `internal/config/roster.go` and its tests. Nothing reads the store yet: no existing reader, verb, launch or dispatch changes in this unit.

1. **The kind table** (design section 2). One table in `internal/config` holds the six fixed rosters, their type and their rows:
   - `tier-1`, `tier-2`, `tier-3` (type `tier`): `design`, `design-critique`, `build`, `code-critique`, `verify`, `investigate`, `warden`, `behavior-judge`;
   - `seat` (type `seat`): `seat`, `steward`;
   - `landing` (type `landing`): `landing`;
   - `partner` (type `partner`): `partner`.
   The same table answers which row a dispatch role and a launch kind use: role `implementer` in mode `design` is `design`, `implementer` in any other mode is `build`, `design-critic` is `design-critique`, `code-critic` is `code-critique`, `verifier` is `verify`, `investigator` is `investigate`, `warden`, `behavior-judge`, and `steward-continuation` is `steward` in roster `seat`; launch kind `design` is `design`, `critique` is `design-critique`, `build` is `build`, `read` is `code-critique`, `seat` is `seat` in roster `seat`, `landing` is `landing` in roster `landing`. Launch kind `proof` runs no agent and has no row. Export one small function per mapping (role and mode to row; launch kind to row; a goal's risk tier to its roster, with tier 0 or an unknown tier answering `tier-3`, design section 3) so units 4, 10 and 11 call them instead of keeping lists of their own.
2. **The file** (design section 4). `HOME/host/rosters.json`, where HOME is the registry home the caller passes (the directory `board.Home()` answers, `~/.metasystem` normally); the lock is `HOME/host/rosters.lock`. Every function takes HOME as a parameter and reads no environment or global state. Form: JSON with `"version": 1` and `"rosters"`, a map from roster identifier to `{"type": TYPE, "rows": {ROW: {"runtime": R, "model": M, "effort": E}}}`. A `main` row is `{"runtime": "main"}` with no model and no effort; a `partner` row has no effort.
3. **Reading a row**: `RosterRow(home, roster, row string, runtimes []string)`. The design names `config.RosterRow(home, roster, row)`; the extra `runtimes` argument (the installation's `metasystem.runtimes`, resolved by the caller) is how the read checks "a runtime this installation does not list" at every read, as the Moved effects table's first row requires. It returns the row's runtime, model and effort, or a refusal. It never tries another roster, an old key or a built-in value. Refusals (design section 3, these exact sentences, with ROSTER, ROW, RUNTIME, the runtime list and REASON filled in):
   - no file, no such roster in it, or no such row in it: "No agent is set for ROW in roster ROSTER on this computer. A person sets it with: metasystem roster set ROSTER ROW RUNTIME:MODEL:EFFORT. Nothing was started." (for `partner` the command ends `RUNTIME:MODEL`);
   - the row's runtime is not in `runtimes` (and is not `main`): "Roster ROSTER runs ROW on RUNTIME, and this installation runs LIST", then the same command and "Nothing was started.";
   - the file cannot be read or parsed, its version is not 1, it holds a roster or a row the kind table does not know, a roster's type differs from the table, or a row breaks a row rule below (rule 4's author-and-reviewer check excepted): "The rosters file PATH can't be used: REASON. metasystem roster list shows what is wrong. Nothing was started.", PATH written with `~` for the user's home directory when it lies under it.
   Each refusal is a typed error carrying one of three codes, `LAUNCH_ROSTER_UNSET`, `LAUNCH_ROSTER_RUNTIME` and `LAUNCH_ROSTER_UNREADABLE`, plus the `roster set` argument vector, so unit 4 can record it in `Admit` without parsing text.
4. **Row rules** (design section 2), checked at write, and at read where noted:
   - all three fields are required (`partner`: runtime and model only, and a third field is refused);
   - effort is one of `low`, `medium`, `high`, `xhigh`, `max`;
   - `auto` is not a runtime;
   - `main` in place of the three fields is allowed only in `verify`, `investigate`, `warden` and `behavior-judge`;
   - a model that is a template placeholder (the pattern `^<[^<>]+>$` that `internal/dispatch/roster.go:60` uses) is refused at write and at read (R-106-m1e);
   - the runtime is one the caller's `runtimes` lists, at write and at read;
   - at write only: within a tier roster, `build` and `code-critique` may not name the same model, nor `design` and `design-critique` (R-133-ui).
5. **Writing a row**: `SetRosterRow(home, roster, row, value string, runtimes []string)`, `value` being `RUNTIME:MODEL:EFFORT`, `RUNTIME:MODEL` for `partner`, or `main`. Under an exclusive `lock.File` on the lock path (as `internal/landing/lane/lane.go:113-127` does), it reads the file, changes the one row, creating the roster on its first row, and replaces the file with `atomicfile.WriteFile` (`internal/landing/lane/lane.go:182-189`). The read happens inside the lock, so two writers changing different rows both keep their change. It answers whether anything changed: writing the value a row already holds changes nothing and is not an error. A file the read would refuse as unusable is refused for writing too, naming the same reason; a write never repairs or drops another row. Refusals, each ending "nothing was changed" (design section 5's words):
   - "tier-9 is not a roster; the rosters are tier-1, tier-2, tier-3, seat, landing and partner"
   - "tester is not a row of tier-1; its rows are design, design-critique, build, code-critique, verify, investigate, warden and behavior-judge"
   - "a row is RUNTIME:MODEL:EFFORT, and effort is low, medium, high, xhigh or max"
   - "gemini is not one of this installation's runtimes (claude, codex)"
   - "the Partner takes no effort; write RUNTIME:MODEL"
   - "tier-3 build and code-critique would both run claude-opus-5-5; the author and the reviewer of a piece of work are different models"
   - "main is only for verify, investigate, warden and behavior-judge"
   and a placeholder model's refusal in the same form. Who may call the write (a person's proof) is unit 3's, not this unit's.
6. **Reading every roster**, for `roster list` in unit 2: one function returning every roster of the table with each row's value or "not set", and the unusable-file refusal when the file cannot be used. Keep it to the parser `RosterRow` already needs.

# Constraints

The accepted design above is the specification; build within its scope and limits. Read its sections 2 to 5 and the Moved effects table's first row before starting.

- Go only. Source comments speak plain English about the system (what a rule protects, what fails without it); no review rounds, finding numbers, unit numbers or design revision history in code.
- Follow `internal/landing/lane/lane.go` for the host directory, the lock and the atomic write; `internal/config` may import `internal/lock` and `internal/atomicfile` (neither imports `internal/config`).
- No fallback of any kind: no compiled default, no `auto`, no environment variable, no other roster.
- Refusal sentences are user-facing text and are tested word for word; keep them exactly as written here.

The independent read's tool-call budget (the configured allowance; change it here if this work needs another):
Maximum reader tool calls: 60

# Expected Return

The change, uncommitted in the goal worktree. The unit runner records the diff, runs the proof and the independent read,
and keeps the run awaiting judgement.

# Acceptance Criteria

- `go test -count=1 ./internal/config/ ./internal/launch/` passes from `metasystem/`, and `go vet ./internal/config/` is clean.
- The three witness tests of design section 8, row 1, exist and pass:
  - `TestRosterRowRefusesAMissingRowNamingTheCommand`: no file, a file without the roster, and a roster without the row each refuse with the code `LAUNCH_ROSTER_UNSET`, the exact sentence above and the `roster set` argument vector; a set row is answered with its three fields. Mutation that turns it red: answer an empty row instead of refusing.
  - `TestSetRosterRowKeepsARowAnotherWriterAdded`: concurrent writers (separate goroutines, each taking the lock through its own `lock.File` call, which `flock` makes exclusive between them) each set a different row of the same roster; every row is present afterwards. Mutation that turns it red: read the file before taking the lock.
  - `TestEveryRoleAndLaunchKindHasARow`: walks the role files embedded in `internal/protocol` (`protocol.Files()`, every role `protocol.Dispatchable` admits; `orchestrator.md` is not dispatchable) and every launch kind except `proof`, and fails when one maps to no row of the kind table. The launch kinds come from one exported list in `internal/launch` that `adapterForLane` (`internal/launch/launch.go:737-755`) also switches on, so a new kind cannot be added without a row. The test may live in an external test package (`config_test`) to import `internal/launch` without a cycle. Mutation that turns it red: drop `warden` from the table.
- Further tests, one per rule, in the store's own test file: each write refusal above with its exact words and "nothing was changed", the file unchanged byte for byte afterwards; a repeat write answers "nothing changed"; a placeholder model refused at write and, written into the file by hand, at read; a runtime not in `runtimes` refused at write and at read; `main` accepted for `verify` and refused for `build`; `partner` takes `RUNTIME:MODEL` and refuses a third field; an unknown roster or row found in the file, a wrong version and malformed JSON each refuse every read with `LAUNCH_ROSTER_UNREADABLE`; a tier of 0 or 9 answers `tier-3`.
- Each test uses its own temporary home (`t.TempDir()`); no test touches the real `~/.metasystem`, sets process environment, or uses Git.
- No existing reader, verb, key or compiled default changes: `git diff --stat` names only `internal/config/roster.go`, its test file or files, and the small export in `internal/launch` (`launch.go` and, if needed, one test).
- The diff is at most 300 changed lines; if it would grow past that, stop and report which part to split off.

# Gap Rule

stop and report a gap; never fill it silently.

When blocked, stop and return `BLOCKED:` with the refusal's first line, what you tried and the decision needed; your seat asks.

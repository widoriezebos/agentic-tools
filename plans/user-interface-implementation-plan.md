# User interface implementation plan

Status: draft for human correction, 2026-09-21. Gate 1 is sliced against a source reading of the engine at commit `94181f05a`. Gate 2 is provisional and gates 3 to 7 are deliberately coarse.

[user-interface-design.md](user-interface-design.md) (the master) owns what is built and why. This file owns how the work is divided, in what order, and what state each slice is in. It never restates or overrides the master. A slice design that finds the master wrong stops and raises an amendment under [Master amendments](#master-amendments); it does not settle the matter locally.

## Working agreement while the machinery does not bind us

Human instruction, 2026-09-21: invoking `metasystem` is not off limits, but this stream is not constrained by the machinery in any way. Whatever needs bypassing may be bypassed. This holds until the human confirms that another agent's work on the engine is finished and the machinery can be relied on again.

- The engine is a tool here, not a gate. Use it wherever it helps: audits, reads of goals and fleet state, test wrappers. When it refuses, fences, caps a budget or a round count, or demands a goal or a receipt, go around it and record the bypass in one line in the slice's evidence note so it can be reconciled later.
- Slices are not goals yet, and nothing here waits on a goal, an approval, a delegate chain, engine-run proof, or engine landing.
- Verbs that publish to the shared goal ledger or act on other machines are outward-facing regardless of the machinery question; they run only on the human's instruction.
- This file is the only source of slice state. Update the slice's row in the same commit as the work that changes its state.
- Work happens on `ui-development`. Each slice is implemented on its own branch `ui/<slice-id>` cut from `ui-development`, and merged back after its code critique and verification.
- The roster is D12: Fable designs, Codex on Sol reviews designs and code, Claude on Opus builds. Codex is dispatched through the Codex plugin, from a git worktree created by hand that outlives the run. Opus builds as a fresh-context subagent in such a worktree. Both critique steps use the materiality test and written dispositions from `metasystem/skills/design-critique` and `code-critique` as a method; the engine's round limits and registers do not bind them.
- Verification is recorded in the slice's evidence note. The baseline is plain Go tooling: `go build ./...`, then `go vet` and `go test` on the packages the slice changes and their direct consumers. Engine checks such as `metasystem audit parallel-ratchet` (without `--update`) are run as well where they work, and skipped with a note where they do not.
- When the machinery returns, every slice not yet `landed` becomes a goal and its row records the goal id; from then on the goal ledger owns that slice's state. Landed slices are not retrofitted unless the human asks.

## Pipeline for one slice

| Step | Who | Input | Output | State after |
| --- | --- | --- | --- | --- |
| 1. Define | Claude with the human | Master sections, this plan | A row in the gate's table | `defined` |
| 2. Design | Claude with the human | Row, master sections, verified code references | `user-interface/<slice-id>-design.md` from the template below | `designed` |
| 3. Design critique | Codex on Sol, fresh context, not the author | The slice design and the master sections it cites | `user-interface/<slice-id>-design-critique.md` with findings and a disposition for each; the loop stops at the first round with no material finding, or earlier when findings are falling and every one that remains can be expressed as a test, in which case each becomes a named obligation that the code critique checks | `design-critiqued` |
| 4. Implement, **only on the human's explicit go** (D13) | Claude on Opus | The slice design at a pinned commit; nothing else is authority | Branch `ui/<slice-id>` | `implementing` |
| 5. Code critique | Codex on Sol, fresh context | `git diff ui-development...ui/<slice-id>` against the slice design: conformance first, then defects | `user-interface/<slice-id>-code-critique.md`; corrections return to the builder | `code-critiqued` |
| 6. Verify and land | Claude, then the human | The slice design's verification section | Evidence note in the code-critique file; merge to `ui-development` | `landed` |

Other states: `blocked` (with the reason and what unblocks it) and `dropped` (with the reason). A slice design is written when the slice is next, never in advance; until then its row is the placeholder.

## Decisions recorded

| ID | Human decision | Date |
| --- | --- | --- |
| D1 | The server and all application logic are Go, shipped as the one `metasystem` executable. That executable contains everything it needs at run time: images, fonts, HTML, JavaScript, and other resources are bundled into it. The browser interface is built with React. An npm toolchain may be used to build it; none is part of the deployed runtime. The server is started and stopped with a `metasystem ui start` and `metasystem ui stop` style subcommand pair. | 2026-09-21 |
| D8 | Everything that makes up the interface lives inside the `metasystem/` source folder, in its own place there if that makes sense. Nothing is added at the host repository root, which belongs to the sources and artefacts of the application the MetaSystem is building. | 2026-09-21 |
| D9 | The interface server is its own process. Restarting the MetaSystem must not restart it, or require restarting it. | 2026-09-21 |
| D2 | Gate 1 needs no access token for reads. Loopback binding with Host, Origin, and fetch-site checks is enough until sign-in arrives at gate 3. The simple case is built first. | 2026-09-21 |
| D7 | The built frontend bundle is committed, inside the Go package that embeds it, with a Go test that fails when the bundle is older than the frontend source. File names are stable, the bundle is rebuilt as the last commit of a frontend slice after rebasing, and frontend slices land one at a time. | 2026-09-21 |
| D10 | Restarting the interface server onto a new build must be easy, so that developing the server itself is easy. | 2026-09-21 |
| D11 | Standing instruction: follow Claude's recommendations and build the simple case first. | 2026-09-21 |
| D12 | Roster, until further notice: design by Claude on Fable; design review by Codex on Sol; building by Claude on Opus; code review by Codex on Sol. A build costs far more tokens than a review of it, so Codex reads and Claude writes. Set in this clone's `metasystem/metasystem.conf.local` and confirmed with the engine's roster resolver. | 2026-09-21 |
| D13 | No implementation starts, and no builder is dispatched, without the human's explicit go for that step. A review the human names comes first. D11 covers design decisions, never starting work. | 2026-09-21 |
| D14 | The interface's look and interaction take their inspiration from two modern web applications the human named: the ChatGPT Codex web app and claude.ai. | 2026-09-21 |
| D15 | Fonts and every other embedded asset are open source. The frontend adds the build-time tools that make sense for the chosen look and for keyboard operation; the proposed set is Tailwind, Radix primitives, a resizable-panels library, and an open icon set. | 2026-09-21 |

## Implementation constraints that follow from D1

These bind every slice design. Items marked proposed are Claude's defaults and open to correction.

- One executable and nothing else at run time. The built frontend bundle is embedded with `go:embed`. The server needs no Node, and the browser fetches nothing from outside the server: fonts, icons, and script all come from the bundle, so the interface works offline on loopback.
- Division of labour. The Go server owns the read models, validation, and every rule about state, authority, and allowed transitions, and exposes them as a JSON API with a server-sent event stream. React owns presentation, client-side navigation, resolving a reference into a route, and view state such as the dock, unsaved buffers, and the reveal policy. React never decides what is allowed; it renders the allowed actions and refusals the server returns. This is the master's split: the frontend resolves references, and the browser holds no authority or transition policy of its own.
- The same structured results feed the browser and, from gate 3, the brain's tools. Nothing is shaped for React alone.
- Server code is standard library first (`net/http`, `embed`, `encoding/json`), in keeping with a `go.mod` that has two dependencies. Any new Go dependency is named and justified in the slice design that needs it.
- Proposed: TypeScript, Vite, and as few npm dependencies as the work allows, each named and justified in its slice design like a Go dependency. The lockfile is committed, installs use `npm ci`, and the Node version is pinned.
- Go tests are the primary proof: read models directly, handlers with `httptest`. Proposed for the frontend: type-checking plus a small Vitest suite, development-time only, for the client logic that carries risk (reference resolution, the unsaved-input guard, the reveal policy). What a human sees is checked in a real browser during each slice's verification walkthrough.
- The bundle is committed, per D7, so every existing `go build` path on every machine yields the complete executable and Node is needed only where the frontend is changed.
- React and an npm toolchain are an exception to the Go-only rule in `development/project-rules-local.md`. The human has made it; recording it there is part of M4.
- Look and interaction, from D14. Patterns are borrowed; structure is not, because both inspirations are chat-first with a history sidebar and this interface is workspace-first with stable sections and a permanent dock. From claude.ai: the conversation with a side panel for its subject, which is the master's focused Brain view, mirrored in docked mode with the application view in the centre and the dock on the right; the calm reading layout for Project documents; composer chips for attached context. From the Codex web app: task lists with status chips, change counts, and times for Backlog and Fleet; the detail split of the agent's log beside the diff and files for goal Execution and Evidence; a prominent composer on Overview. From both: a collapsible sidebar, little chrome, one accent on neutrals, light and dark themes, a command palette as the master's action finder, streamed text, skeleton loading, keyboard-first operation. No brand assets, logos, or licensed fonts: fonts are committed and embedded, so they must be openly licensed, and the look is our own.
- Decided by D15, replacing "as few npm dependencies as the work allows": a small curated set, all build-time only, because the borrowed look and the master's keyboard-operation requirement are not worth hand-rolling: Tailwind, Radix primitives for menus, dialogs, and tooltips, a resizable-panels library for the dock, and an open icon set. Each is still named and justified in `g1-s8`.
- Placement, from D8. Go packages under `metasystem/internal/ui/`, the React source in its own directory under `metasystem/` and fenced off from Go tooling, and the built bundle inside the Go package that embeds it. Runtime state follows the engine's existing convention for its own processes, `artifacts/agents/ui/` in the checkout, which is ignored by Git; it holds no source.
- Process independence, from D9. The server is a detached process in its own session. It is not an engine run, job, or supervised component, it is in none of the families the top-level `stop` ends, and it does not consult the engine's stop fence. Replacing `bin/metasystem` by a rebuild leaves a running server on the build it started with, so `ui status` reports when the running server is older than the executable on disk.
- `metasystem ui start` is independent of `metasystem up`. It starts no engine, declares no brain, and claims no goal. `metasystem ui stop` stops only the interface server; the top-level `stop` remains the engine's. A `ui status` is expected alongside them.

## Staying clear of the other agent

Goal `tests-parallel-and-deterministic` was claimed on 2026-09-21 by another machine. No code for it exists in this clone yet, so the list below is predicted from its predecessor arc and its stated next step. Revisit it when that work lands.

- Do not edit: `metasystem/testing-parallel-ratchet.json`, `metasystem/testing.json`, `internal/testenv`, `internal/testutil`, `internal/parallelratchet`, `internal/testselect`, `internal/testpolicy`, `internal/testexec`, `internal/hostload`, `internal/proofrun`, `cmd/metasystem/test*.go`, `cmd/metasystem/audit.go`, or any existing `*_test.go` or `testmain_test.go`.
- Slices add new files. The permitted touches to existing files are one `ui` family entry in `cmd/metasystem/main.go`, the package-map and family rows in `docs/architecture.md`, commented keys in `metasystem.conf`, and whatever a slice design names explicitly with its reason.
- Work that must edit existing CLI handlers or their tests is split into its own slice and held `blocked` until the other agent is done. `g1-s16` is the first of these.

## Conventions every new package must meet

From `internal/testenv`, `internal/parallelratchet`, and `docs/architecture.md`. Each slice design repeats the ones that apply, because Codex sees only the slice design.

- Logic lives in an `internal/` package. `cmd/metasystem/ui.go` parses flags and routes only; the architecture document allows nothing worth testing in `cmd/`. A domain package importing `internal/goal` is allowed; `goal` must never import the interface packages.
- Every package has `testmain_test.go` calling `testenv.Main`. Deliberate environment inputs are declared with `testenv.Declare`; tests never call `os.Setenv`.
- Every top-level test and independent subtest calls `t.Parallel()`. A package absent from the ratchet file has a serial-test ceiling of zero, and we may not edit that file, so one serial test is a violation. We hold to this by choice, so the packages pass the ratchet the day the machinery binds again. Check it with `metasystem audit parallel-ratchet` where that runs, and by inspection in the code critique either way.
- No wall-clock sleeps. Anything periodic, the freshness loop above all, takes an injected clock and is tested by advancing it.
- No fixed ports: `127.0.0.1:0` or `httptest`. Temporary state under `t.TempDir()`. Child processes through `testutil.Fixture`. Assertions with `testutil.Expect` and `Require`.
- A new configuration key is dotted and namespaced (`ui.*`), read by a typed function in a new `internal/config/ui.go` with its default as a constant beside it.

## Open decisions for the human

| ID | Decision | Blocks | Recommendation |
| --- | --- | --- | --- |
| D3 | The second independent review of the master's round-one amendments ran on 2026-09-21: [ten material findings](user-interface-design-critique-r2.md), each with a proposed disposition. The rulings are the human's. | Gate 2 and gate 3 slice designs. S1, S2, S3, S5, and S10 must be ruled first | S3 to S10 ruled by the human on 2026-09-21: accepted as proposed. S1 and S2 under discussion. None blocks gate 1. S1 touches `g1-s2`: until it is ruled, the server serves the checkout it is started in, and the dedicated clone waits. |
| D4 | Sign-in system. | Gate 3 | Not yet investigated. |
| D5 | First ACP provider and its demonstrated isolation envelope. | Gate 3 | Not yet investigated. |
| D6 | Rendering Markdown documents (goal intent, designs, rulings) needs either a renderer dependency or a small renderer of our own. | `g1-s11` | Show source text in gate 1 and decide when the Project editors are designed at gate 5. |

## Master amendments

Found while mapping the master onto slices. Under D11, M1, M2, M5, M6, and M7 proceed on the proposed resolution without an individual ruling, and any of them can be reopened. M3 and M4 are ruled by D2 and D1. With the human's go-ahead, all seven were written into the master on 2026-09-21: a dated decisions note at its head, the gate 1 access posture in the command-edge section, and the gate table. The React exception was added to the Go-only rule in `development/project-rules-local.md` the same day. New gaps found later are added below as open rows.

| ID | Gap | Proposed resolution |
| --- | --- | --- |
| M1 | Overview and Application appear in the agreed navigation but in no delivery gate. | Overview read-only in gate 1, since it is a projection of the same reads. Application with gate 6, since it depends on evidence joins. |
| M2 | Board operations (intake, approve, withdraw approval, reorder) are required scope but no gate names them. Gate 4 covers only a queued-goal edit; gate 5 names split, slices, and approved-definition revision. | Place them in gate 5 ahead of the split editor, because guided revision already needs withdraw and approve. |
| M3 | Gate 1 precedes sign-in, yet the master requires authenticated reads. Ruled by D2. | State the gate 1 access posture explicitly in the command-edge section. |
| M4 | No implementation technology is named. Ruled by D1. | Record D1 in the master as a dated human decision, following the precedent of its sign-in note, and add the frontend exception to the Go-only rule in `development/project-rules-local.md`. Done. |
| M5 | The board's In Progress and Review and Verification lanes rest on a "recorded phase". No such field exists: goal states are `queued`, `approved`, `claimed`, `parked`, `done`, `abandoned` (`internal/goal/file.go:512`), `goal.Phase` is a transaction phase, and batch states live outside the goal record. | Gate 1 places claimed work in In Progress, places a landing claim in Review and Verification as "landing", and labels every other phase "not recorded". Recording an execution phase is owner work; assign it a gate (5 or 6). |
| M6 | The Draft lane's source, `plans/goals-drafts/`, has no Go reader, schema, or test anywhere in the engine. | Gate 1 shows the lane with a named gap. A drafts owner belongs with intake in gate 5. |
| M7 | The Fleet seat row asks for facts nothing records: a session's role and model (announcements carry the runtime only; role and model sit on job records), an idle state (only an alert episode exists), an unknown state per seat (health is checkout-wide), a job's stop reason (only an error text and separate breach-stop routes), and spend per seat (spend is per machine-day). The master already places the seat registry in gate 6 but names none of these. | Gate 1 shows each as "not recorded". Add them to gate 6 as owner work beside the seat inventory. |

## Gate 1: readable workspace

Master deliverable: dedicated clone, accepted-tip reads and freshness; stable navigation, Backlog and goal detail, local Fleet, and Decisions over existing records; reference-based context and result views. No model, no mutation. Unknown lanes, missing links, and remote coverage are marked honestly. Human task walkthroughs validate the navigation before forms are built.

What the code already offers, from source reading at commit `94181f05a` (nothing was executed):

- Reading the accepted tip is exported and free of side effects: `goal.ResolveEndpoint` (`internal/goal/txn.go:61`), `goal.Project` (`project.go:72`), `goal.ProjectAt` and `goal.AcceptedLedgerTip` (`attention.go:559`, `:543`). The accepted tip is the ref `refs/metasystem/goals/accepted` (`txn.go:51`). `plans/goals-accepted.json` is the legacy baseline and is not it.
- Fetch and validate is exported and moves refs only: `goal.FetchAdvance` (`fetchadvance.go:30`). `goal reconcile` is the legacy worktree path and is not used.
- The journal is readable without mutation: `goal.Entries`, `goal.PushedBlocking`, `goal.ClassifyRecovery` (`journal.go:482`, `:508`, `:556`). "Pushed but unknown" is `Phase == PhasePushed` with no terminal outcome, not an outcome value. No aggregate journal status exists yet.
- Lane grouping, card badges, and the live-or-archived lookup exist only inside CLI handlers: `listSynced` (`cmd/metasystem/goal.go:394`), `goalDisplayRecord`, `goalListSummary`, `goalListMarkers` (`goal_list.go:20` to `:169`), and `runGoalShow` (`goal.go:475`). Arc membership is the unexported `arcMembers` (`internal/goal/verbs.go:3301`).
- History has no accessor; callers index `Projection.Tree.Live[id].History` or `Tree.Archived(id)`.
- Command routing is a literal table, `families()` in `cmd/metasystem/main.go:29`; a family entry brings its help text with it. Daemon precedents are the steward runner (`internal/steward/runner.go`: exclusive `flock`, pid with start time, log) and the detached-process facility in `internal/run`. Reusable foundations: `internal/lock`, `identity`, `atomicfile`, `stateroot`, `output`, `events`.

- Decisions reads are nearly all exported. Open questions: `channel.WalkOpenQuestions` and `channel.ReadQuestion` (`internal/channel/question.go:142`), with the answer and its lifecycle on the question record. Approvals: `goal.ApprovalRecord` with actor, authority grade, and digest (`internal/goal/file.go:199`), and authority provenance on every history line (`file.go:438`). Grants: `PowerOfAttorneyEntry` on the ledger root (`internal/goal/root.go:46`), reachable through the projection; there is no filtered lister. `memory/rulings.md` is a Markdown table of prose whose only parser is unexported and reads three columns (`internal/steward/ruling_sweep.go:81`).
- Fleet reads are thin. There is no seat type and no seat registry: a seat is a machine nickname (`goal.ResolveMachine`, `internal/goal/actor.go:22`), and a seat that is off is invisible. Session announcements are typed (`internal/lease/classify.go:21`) but listing them is unexported. Job records have a typed lens (`dispatch.JobRecordOf`, `internal/dispatch/jobrecord.go:22`) but no lister, and the chain-root walk is unexported (`chain.go:28`). Health is exported (`steward.ObserveHealth`, `internal/steward/health.go:285`), as is the last census (`internal/census/run.go:70`). The nearest existing aggregate is `watch.Read` (`internal/watch/watch.go:131`), which is verdict-shaped, not seat-shaped.
- No recorded source exists for a session's role or model, for an idle seat, for an unknown state per seat, for a job's stop reason, or for spend per seat. See M7.

Slices `g1-s2` to `g1-s6` are plain Go packages with no dependency on the server, and `g1-s8` needs only `g1-s1`, so the read models and the frontend toolchain can proceed side by side. The shortest path to a board in a browser is `s1`, `s2`, `s4`, `s7`, `s8`, `s9`, `s10`. Design `g1-s1` first: it fixes package layout, naming, configuration, and lifecycle for everything after it.

The engine does not see the interface server. The process census filters the process table through the configured runtime signatures before it classifies anything (`internal/census/run.go:177`, `signature.go:84`), and those signatures match only a command named `claude`, `codex`, `devin`, or `devin-delegate-acp`. `bin/metasystem ui serve` is never enumerated, so it is never classed UNTRACKED, never counted by the steward, never listed by the watchdog or by the engine's `stop`, and never signalled. An earlier revision of this plan said the opposite twice over: first that the only cost was a misleading line in `stop`, then, after critique round 1 of `g1-s1`, that a running interface suppresses the steward's revival. Round 2 refuted both by reading the filter. `g1-s17` is dropped.

| Slice | Outcome a human can observe | Depends on | Design | State |
| --- | --- | --- | --- | --- |
| `g1-s1` Server lifecycle | `metasystem ui start` launches a background loopback server for one configured project and brain checkout and reports its address; `ui status` reports it; `ui stop` ends it cleanly after confirming the process is the one it started (pid and start time); `ui restart` replaces it with the build on disk at the same address. A second start is refused. Foreign Host and Origin are refused, and only GET and HEAD are served. No engine, brain declaration, or claim is started, and the engine's stop fence is deliberately not consulted. New: a `ui` family entry, `cmd/metasystem/ui.go`, packages under `internal/ui/`, `internal/config/ui.go` | none | [design](user-interface/g1-s1-server-lifecycle-design.md), [critique](user-interface/g1-s1-server-lifecycle-design-critique.md) | `design-critiqued`, on hold. The design closed three Opus critique rounds on obligations O1 and O2 and now awaits the design review the human asked for. A Codex build of it exists on branch `ui/g1-s1` (`3f1aadbd1`, worktree `.claude/worktrees/g1-s1`). It was dispatched without the human's go, before D12 and D13, and is unreviewed; its build, vet, and tests pass. The human decides whether it is kept, rebuilt under D12, or discarded |
| `g1-s2` Accepted-tip snapshot | For the checkout the server is started in (the dedicated clone waits for the ruling on S1 of the second review), a package holds a parsed snapshot read through `goal.Project` from the accepted ref, never from worktree bytes, with tip identity, observation time, and the owner's staleness banners. Composition of exported functions; no extraction | none | none yet | `defined` |
| `g1-s3` Freshness loop and journal status | One non-overlapping `goal.FetchAdvance` loop on an injected clock (five seconds connected, thirty idle, bounded backoff, manual refresh); another clone's publication becomes visible; a new read-only journal status reports each outcome, and a pushed-unknown as a workspace-level condition | `g1-s2` | none yet | `defined` |
| `g1-s4` Backlog read model | A package, no HTTP, projects every goal into exactly one lane with Waiting precedence, badges, the closed-items set, dependencies, and history. It states the gaps in M5 and M6 instead of guessing. It is written as the future shared owner of what `listSynced` and `goal_list.go` do today, without touching them, so it lives outside the interface's own package tree | M5, M6 | none yet | `defined` |
| `g1-s5` Decisions read model | A package projects open and answered questions, approvals with their authority grade, grants with scope, expiry, and revocation, and the rulings table read row by row from its canonical file. Adds an exported rulings reader in a new file; rulings stay prose and are shown as such | none | none yet | `defined` |
| `g1-s6` Local fleet read model | A package projects this machine's sessions, lease holder, jobs with their goal and chain, census, and health, each with its observation time. Adds exported listers in new files in `internal/lease` and `internal/dispatch` so the on-disk formats keep one owner. Every fact in M7 is reported as "not recorded", and the absence of a seat registry as a coverage gap | M7 | none yet | `defined` |
| `g1-s7` Read API, references, and events | JSON routes over the read models. Results identify objects by kind, stable identifier, and revision, never by route. A server-sent event stream carries invalidation only. The payload shapes are written to serve the brain's tools later without change | `g1-s1` to `g1-s6` | none yet | `defined` |
| `g1-s8` Frontend toolchain and embedded bundle | A React application skeleton builds into a bundle that the server embeds and serves, with a fallback so client-side routes reload correctly, a content security policy that fits it, and a plain statement in the browser when an executable was built without a bundle. Go tooling is fenced off from the frontend source tree. Carries the committed-bundle policy of D7 and its staleness test, and a development mode with hot reload that proxies to the running Go server without loosening the server's request checks | `g1-s1` | none yet | `defined` |
| `g1-s9` Application shell | Built to `g1-s18`. Navigation rail with the agreed sections, empty states, stable links, the resolver from reference to route, connection and freshness indicators, refetch on invalidation, a reload when the server's build changes after a reconnect, and a Brain entry that states it is unavailable until gate 3 | `g1-s7`, `g1-s8`, `g1-s18` | none yet | `defined` |
| `g1-s10` Board and list | Read-only board and list with filters and the closed-items filter; no drag | `g1-s9` | none yet | `defined` |
| `g1-s11` Goal detail | One goal workspace with Summary, Definition, Plan, Execution, Evidence, and History, each showing what is recorded and naming what is not | `g1-s9`, D6 | none yet | `defined` |
| `g1-s12` Decisions view | Inbox and record of decisions; a question opened from a goal or from the inbox is the same page | `g1-s9` | none yet | `defined` |
| `g1-s13` Fleet view | Local sessions, jobs, and health, linked to the same goal detail, with coverage gaps visible and no controls | `g1-s9` | none yet | `defined` |
| `g1-s14` Overview | What needs me and what changed since the last visit, with a server-local last-visit marker | M1; `g1-s10` to `g1-s13` | none yet | `defined` |
| `g1-s15` Walkthrough validation | The human walks representative tasks through the read-only workspace; findings amend the master before any form is built | `g1-s1` to `g1-s14` | none, human activity | `defined` |
| `g1-s18` Interface design language and shell mockups | Design only, no product code: colour, type, spacing, and radius tokens in light and dark; the shell layout with the rail, the workspace, and the dock in docked and focused modes; a component inventory; the empty, loading, stale, and error states; and static mockups of the shell, the board, a goal detail, and the focused Brain view for the human to react to. Every view slice cites it | D14 | none yet | `defined` |
| `g1-s16` CLI adopts the shared projection | `goal list` and `goal show` call the `g1-s4` package, so the terminal and the browser cannot disagree about a goal's lane or badges. Edits `cmd/metasystem/goal.go`, `goal_list.go`, and their tests | `g1-s4`; the other agent's work finished | none yet | `blocked` |
| `g1-s17` The engine accounts for the interface server | Dropped. It existed to correct a classification that never happens; see the paragraph above. Whether the engine's `status` should mention a running interface can be raised later as a convenience | none | none | `dropped` |

## Gate 2: shared command foundations

Provisional. All engine-owner work with no interface, the highest risk of colliding with the other agent's changes, and resting on the amendments D3 covers. Slice in detail only after D3.

| Slice | Outcome | State |
| --- | --- | --- |
| `g2-s1` Trusted server entry | Authority and lease owners accept per-request principals from a server path; nothing is inherited from the server's OS parent | `defined` |
| `g2-s2` Shared fences | Brain fences and caller validation move from CLI handlers into entry points that the CLI, HTTP, and MCP share | `defined` |
| `g2-s3` Serialized publication and recovery | Writes through the clone are serialized; every journal outcome is preserved; a pushed-unknown blocks all writes until the recovery owner resolves it | `defined` |
| `g2-s4` Preparation path for goal edit | Preview and publication share one computation; preview performs no journal write, push, hook, or execution effect | `defined` |
| `g2-s5` Review basis | Owner-computed digest of decision-relevant fields; basis and effects rechecked inside the transaction, including on replay | `defined` |
| `g2-s6` Refusals, allowed actions, changed references | Typed refusals with unmet conditions and recovery operations; allowed actions per subject and caller; changed references derived from the change set | `defined` |
| `g2-s7` Operation identity | Operation id assigned before submission and durably correlated; a retry reuses it; the outcome survives a server crash | `defined` |

## Gates 3 to 7

One row each until the gate before it is under way. The investigations named here come from the master's remaining implementation investigations and must finish before the gate is sliced.

| Gate | Coarse content | Must precede slicing |
| --- | --- | --- |
| 3. Protected human and brain access | Sign-in and session lifecycle; human and agent request separation; host and provider isolation; interactive ACP host; seven MCP tools; exclusive brain occupancy; private sitting store; budget enforcement; Brain dock and context capture | D4, D5 |
| 4. First complete edit | Queued-goal edit through the brain and by hand; result cards; `present`; interrupted outcomes; basis conflicts | Gates 2 and 3 evidenced |
| 5. Project and planning | Working-material metadata; `DefinitionRefs` and migration; board operations (M2); guided approved-definition revision; split editor; slice-plan owner and revision | Slice-plan owner investigation; document-location investigation |
| 6. Fleet and evidence | Durable seat inventory; remote observations; `records/fleet-control/` requests and acknowledgments; retained artifact retrieval; goal-to-attempt joins; Application (M1) | Seat inventory, fleet request owner, and evidence retention investigations |
| 7. Complete human coverage | Operation coverage inventory; remaining operation families; recovery, setup, and maintenance flows; Settings pages | Audit of the routed command surface |

## Coverage

Every proof obligation and acceptance scenario has a gate. It gains a slice when its gate is sliced. An obligation with no slice in a sliced gate is a defect in this plan.

### Critique-derived proof obligations

| Obligation | Gate and slice |
| --- | --- |
| UID-R1-01 | `g2-s1`; observed end to end at gate 4 |
| UID-R1-02 | Gate 3 |
| UID-R1-03 | Visibility after fetch: `g1-s3`. Write block and recovery: `g2-s3` |
| UID-R1-04 | Gate 6 |
| UID-R1-05 | `g2-s4` |
| UID-R1-06 | Proposal and queued edit: gate 4. Interrupted guided revision: gate 5 |
| UID-R1-07 | Gate 5 |
| UID-R1-08 | `g2-s5` |
| UID-R1-09 | Gate 3 |
| UID-R1-10 | Gate 3 |
| UID-R1-11 | Gate 3; resume from shared working material: gate 5 |
| UID-R1-12 | Gate 4 |
| UID-R1-13 | `g1-s15`, and the ordering rule that gates 2 and 3 are evidenced before gate 4 starts |
| UID-R1-14 | Gate 3 |
| UID-R1-15 | Established at gate 3; held at gate 7 as coverage grows |

### Acceptance scenarios

Numbered by their order under "Acceptance scenarios" in the master at commit `94181f05a`.

| Gate | Scenarios |
| --- | --- |
| 1 | 15 result references survive a route change (resolver only), 19 no model invocation on navigation, 21 blocker directly inspectable, 34 authoritative status with named gaps, 36 seat to goal to delegate and back (local) |
| 2 | 17 same rules and real actors for tool and form, 18 lost notification and no duplicate, 23 stale basis refused and irrelevant change preserved, 24 disconnect during a decision |
| 3 | 1 sign in and act without enrollment, 3 expired session, 6 same conversation across sections, 7 message bound to the selection at send time, 8 filtered or paginated set, 25 examiner cannot read the sitting, 26 brain unavailable and work continues |
| 4 | 2 reserved action stays a proposal, 9 permitted edit updates everything consistently, 10 failed check opens in Evidence, 11 no overwrite, no cross-tab navigation, no approval, 12 edit by conversation then by hand, 13 reveal without a second message, 14 completion while disconnected, 16 session ends after success |
| 5 | 4 engine stopped and a draft saved, 5 fresh session resumes from records, 22 intent recorded without approval, 28 new project with no goals, 29 canonical documents without copies, 30 drag to Ready authorizes exact work and budget, 31 concurrent claim, 32 Done cannot be fabricated, 37 to 41 split, slices, revision, and the refused split |
| 6 | 27 intended versus evidenced behavior, 33 completed item to retained evidence, 35 inactive and unreachable seats |
| 7 | 20 every object type discussable, 42 discoverable exceptional actions, 43 every inventoried operation has a browser path, 44 the full walkthrough |

## Slice design template

```markdown
# <slice-id> <name>

- Gate, state, author, date
- Master commit and the sections this slice refines (links)
- Depends on (slices, decisions)
- Discharges (UID-R1-xx, scenario numbers)

## Outcome
One paragraph: what a human can observe once this lands.

## Scope and non-goals

## Existing code this builds on
File and line references, each verified at the named commit.

## Contracts
Packages, exported types and functions, routes and payloads, configuration keys, on-disk state.

## Behaviour
Main flow, then a table of failure and edge cases with the required result.

## Change boundary
Files expected to change. Files that must not be touched.

## Verification
Go tests to add. The observable walkthrough with exact commands. Which obligations and scenarios it demonstrates.

## Open questions
Must be empty before implementation starts.
```

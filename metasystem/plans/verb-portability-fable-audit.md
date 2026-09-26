# Verb portability audit (Fable 5.1, read-only)

Date 2026-09-26. Source worktree `agentic-tools-agent-help-20260926` at `0a38b2e01` (uncommitted agent-help slice). Actual help read from `E/metasystem-corrected`. Evidence kinds: **reproduction** (ran the binary), **read** (read the source or the descriptions file), **inference** (reasoned from source; not executed end to end).

Question answered: is the public CLI, all 44 verbs with their forms and options, valid for a completely different application that adopts the metasystem? Short answer: 32 of 44 verbs are generic workflow or governance as written, and every one of the 44 stays valid for another application. Five verbs mix generic forms with tool administration forms and need per-form labels, not per-verb labels. Two verbs carry a real portability defect (abandon, settings compatibility) that comes from the engine-floor check, and it is a message and wording defect, not a design defect.

## 1. Per-verb verdicts (44)

Codes: **G** generic application workflow or governance. **G+A** generic verb with administration forms or options named in the next column. **A** tool administration of the installation (legitimate for every app, label it). **L** carries a leak, see section 2.

| # | verb | group | audience | verdict | administration forms / metasystem-internal options | leak |
|---|---|---|---|---|---|---|
| 1 | goals | goals | both | G | `--machine` (fleet read, advanced) | |
| 2 | show | goals | both | G | | |
| 3 | approve | goals | human | G | `--under`, `--temporary-human-word`, `--review-by` (seat authority; governance, keep) | |
| 4 | budget | goals | human | G | same as approve | |
| 5 | pause | goals | both | G | | |
| 6 | resume | goals | human | G | `--under`, `--verified` (seat act) | |
| 7 | done | goals | both | G | | |
| 8 | start | operations | both | G+A | admin: `start [checkout]`, `start ui`, `start machine NAME` (+`--destination`, `--resume`, `--installation`). workflow: `start session`, `start mission M` | L8 |
| 9 | stop | operations | both | G+A | admin: `stop [checkout]`, `stop ui`. workflow: `stop job J`, `stop session --by`, `stop design G` | |
| 10 | restart | operations | human | A | both forms | |
| 11 | status | work | both | G+A | admin: `status [checkout]`, `status ui`, `status --machines`. workflow: `status G`, `job`, `work`, `mission` | |
| 12 | enroll | goals | human | A | whole verb is authority setup; it sits under "plan and steer goals" today | |
| 13 | ask | questions | agent | G | `--kind stop|budget-above-norm|carry` are governance questions (keep, advanced) | |
| 14 | answer | questions | human | G | | |
| 15 | check | operations | both | A | `check` and `check settings` are diagnostics; `check goals` is a ledger-edit preview | L5 |
| 16 | open | goals | both | G | `--origin human|main` is provenance internals | L9 |
| 17 | edit | goals | both | G+L | the 17 `--obligation` options (surface digest, toolchain identity, timing envelope, typed review assumptions) | L3 |
| 18 | claim | goals | agent | G | `--take-over` is a person's act, fine | |
| 19 | release | goals | agent | G | | |
| 20 | accept-risk | goals | human | G | | |
| 21 | pin | goals | human | G | fleet-aware but generic | |
| 22 | prioritize | goals | human | G | | |
| 23 | reopen | goals | both | G | | |
| 24 | abandon | goals | human | G+L | engine-floor precondition (not in help; in the refusal) | L1 |
| 25 | block | goals | both | G | | |
| 26 | unblock | goals | both | G | | |
| 27 | unapprove | goals | human | G | | |
| 28 | grant | goals | human | G | governance of seats; generic for any app with agent seats | |
| 29 | revoke | goals | human | G | | |
| 30 | split | goals | both | G | | |
| 31 | group | goals | both | G | | |
| 32 | ungroup | goals | both | G | | |
| 33 | notes | goals | both | G | | |
| 34 | repair | operations | both | G+A | admin: `repair goals` (all four forms; `--upgrade` is a one-time migration), `repair waits`. workflow: `repair review G`, `repair mission M` | L7 |
| 35 | incidents | operations | both | G | `--to MACHINE` fleet assignment, fine | |
| 36 | design | work | agent | G | | |
| 37 | brief | work | agent | G | | |
| 38 | build | work | agent | G | `--model`, `--effort` advanced, fine | |
| 39 | wait | work | agent | G+A | admin: `wait resume ID` (machine launch) | |
| 40 | test | work | both | G | details point at `test plan`, `test verify`, `test report`, which public help does not describe | L5 |
| 41 | settings | operations | both | A | all forms; `compatibility --minimum-engine` is the engine floor | L2 |
| 42 | review | work | both | G+L | `--finding` fixture-obligation options `--implementation-chain`, `--artifact`, `--result`, `--critic` | L4 |
| 43 | revise | work | agent | G | | |
| 44 | land | work | both | G | `--upgrade-goals` (ledger migration inside land), `--exception CODE` (codes listed nowhere public), `--transfer` (seat) | L7 |

Totals: G 32, G+A 5 (start, stop, status, repair, wait), G+L 3 (edit, abandon, review), A 4 (restart, enroll, check, settings). No verb is invalid for another application. Every "metasystem-specific" item is a form or an option, never a whole verb, except `settings compatibility` and `start machine`, which are justified administration (fleet engine compatibility, fleet provisioning) and belong in a labelled administration block.

## 2. Ranked leaks

### L1. abandon is gated on an engine-floor ancestry check that runs in the application's repository against a commit from the template repository
- Evidence (read): `internal/goal/abandon.go:255-290` `requireAbandonFleetFloor`; `:263` refusal text names `metasystem settings compatibility --minimum-engine <stamp> --by NAME`; `:267` and `:278` name `scripts/agents/go-build.sh`. `cmd/metasystem/goalsync_mutations.go:2374-2383` binds `supervise.BuildStamp` and `goal.IsAncestor`. `internal/goal/attention.go:664-681`: `IsAncestor(root, …)` runs `git merge-base --is-ancestor` in `root`, which is `Endpoint.Root`, the ledger repository worktree (`internal/goal/txn.go:30`); exit 1 is "no", any other failure is returned as an error. `scripts/adopt.sh:216` builds the shipped engine with the **template's** `go-build.sh`; `scripts/agents/go-build.sh:10,65` stamps `git -C "$root" rev-parse HEAD` where `root` is the template checkout. `internal/supervise/engine_floor.go:55-73` repeats the same comparison for every registered seat and names `metasystem up`, which is not a public verb.
- Effect in an unrelated application (inference, not reproduced): the first `abandon` refuses and asks the person to record the template SHA as the "minimum engine". That works once, because equal commits short-circuit (`attention.go:669`). After the application rebuilds its own engine, the stamp becomes the application's HEAD. `merge-base --is-ancestor <templateSHA> <appHEAD>` inside the application repo then fails with an unknown-revision error, which is returned as an error, so `abandon` fails with "place this engine's build against fleet floor …" and no recovery text. The recovery that exists (re-record the floor with the application SHA) is not named. A mixed fleet (one seat still template-stamped) hits the same error through the registry pass. Single-machine installs (`goal.sync-remote=local`) are affected the same way.
- Smallest remedy: in `requireAbandonFleetFloor` and `EngineFloorProblems`, treat an ancestry error that is not exit 1 as "not comparable in this repository" and refuse with the public route "record the floor for this build: metasystem settings compatibility --minimum-engine <stamp> --by NAME"; replace `metasystem up` with `metasystem restart checkout` and drop the script path. Message-level only, no design change. Not part of the help slice; file it as a follow-up goal.
- Uncertainty: I did not run adoption. `go-build.sh:67` has a vendored-prefix path that may change which repository is stamped; a fresh adoption should be checked once.

### L2. `settings compatibility --minimum-engine` and the legacy `goal engine-floor` line say "engine commit"
- Evidence (reproduction): `metasystem-corrected help goal` prints `engine-floor human-only: record the oldest engine commit every enrolled seat runs; the first abandon refuses without it`. `help settings` details say "records a person's assertion that every seat runs at least that engine". `cmd/metasystem/intent_operations.go:380-386` forwards to `goal engine-floor`; validation is 40 lowercase hex only (`:374`), no ancestry check on record.
- Effect: in another application the value is the template SHA or the application's own HEAD depending on who built the binary; "engine commit" tells the reader to look in a repository they do not have. It is valid administration, wrongly worded.
- Smallest remedy: word it as "the engine build stamp every seat runs (as `status --machines` prints it)" and place the form under the administration label. No mechanism change.

### L3. `edit --obligation` exposes the engine's governed-obligation grammar as goal editing
- Evidence (read): `public-verb-descriptions.json` edit options: `--obligation DRAFT|OBSERVE|LIMITED|ENFORCED`, `--platform`, `--toolchain-identity`, `--surface-digest`, `--max-active-jobs`, `--timing-envelope-sec`, `--effect`, seven typed review assumptions, `--destructive-reach`. All advanced.
- Effect: an agent building another application reads a behaviour-surface governance record as an ordinary goal field. Whether these render in `help edit` text was not checked (uncertainty).
- Smallest remedy: keep the options; add one details line, "obligation fields record the engine's own governed obligations; ordinary goals never need them", and if help forms are cheap, a form `edit obligation` under administration.

### L4. `review G --finding F --test NAME` fixture-obligation options
- Evidence (read): `--implementation-chain`, `--artifact`, `--result`, `--critic`, each described as "a fixture obligation". Advanced.
- Effect: fixture obligations exist only where governed fixture beds exist; elsewhere the options are noise. Not harmful.
- Smallest remedy: one details line saying when a fixture obligation arises. Nothing else.

### L5. Public help and refusals reference surfaces the public help does not describe
- Evidence (reproduction and read): `help test` details: "test plan, test verify and test report keep their own grammar". `engine_floor.go` refusals: `metasystem up`. `abandon.go`: `scripts/agents/go-build.sh`. `help check` claims "each problem names the command that fixes it", which I could not verify for a foreign checkout.
- Remedy: name public verbs (`restart checkout`, `test`) or say "see metasystem help internal test".

### L6. The compatibility catalogue is reachable but unannounced
- Evidence (reproduction): `help`, `help all`, `help agent`, `help human` contain zero mentions of `internal`; `help internal` exists and says "no public task needs it"; `help internal --json` is refused by design (`intent_help.go:183`). The JSON index carries 44 commands and a protocol, no pointer.
- Effect: an agent that inherits an existing hook or script calling `metasystem goal …` cannot find its documentation from public help. Capability is kept, discoverability is not.
- Smallest remedy: one line in the bare help "More:" block: `metasystem help internal   compatibility catalogue: engine families for existing scripts, not a public task`, and one protocol instruction or field in JSON with the same sentence. This is the "separate section" the human asked for, and it already exists.

### L7. Ledger migration rides inside land and repair
- Evidence (read): `land --upgrade-goals` "raise the goal ledger's format in the same act"; `repair goals --upgrade --source-digest --amendments --identity --sync-mode`; `land --exception CODE|group:NAME` where refusal codes and testing groups are not listed publicly.
- Effect: legitimate one-time administration shown beside everyday landing. Remedy: label as administration in text and JSON (section 3); optionally point `--exception` at where codes come from (the refusal itself prints its code, I believe; unverified).

### L8. `start machine NAME` (clone, build, configure, enroll, supervise a fleet machine on this host) shares a verb with `start session`
- Reproduction: `help operations`. Valid fleet administration; a per-form label solves it.

### L9. `open --origin human|main`
- Read. Provenance internals; advanced. Leave it, or hide it from text help.

## 3. Critique of root's classification and the smallest separation

Root's proposal: keep the four existing groups, label actual tool administration, label the old internal family. Agreed in shape. Two corrections:

1. **Administration is a property of forms, not of the operations group.** Of the 24 usage lines under "Operate, diagnose and repair this checkout", these are application workflow and must not be labelled administration: `stop job J`, `stop session --by`, `stop design G`, `repair review G`, `repair mission M …` (both forms), `incidents` (all three), `check goals`. These are administration: `start [checkout]`, `start ui`, `start machine`, `stop [checkout]`, `stop ui`, `restart checkout`, `restart ui`, `check`, `check settings`, `repair goals` (four forms), `repair waits`, `settings` (all four forms), plus `enroll` from the goals group and, from the work group, `status [checkout]`, `status ui`, `status --machines`, `wait resume ID`. `start session` and `start mission M` are workflow.

2. **"MetaSystem-specific with a good reason" applies to forms, and each has its reason:** `settings compatibility` (fleet engine compatibility; reason stands, wording wrong, L2), `settings coordinator` (ledger coordinator election), `start machine` (fleet provisioning), `repair goals --upgrade` and `land --upgrade-goals` (one-time ledger migration), `edit --obligation` (engine governance; reason not stated anywhere I read, ask the owner before keeping it in public text), `review --finding` fixture options (fixture beds). None needs its own top-level verb, so no new group.

Recommended separation, smallest that works across the three surfaces without hiding anything:

- **Human text** (`help`, `help human`, `help operations`, `help all`): keep the four headings. Inside the operations block print two sub-blocks: "Stop or recover work" (workflow forms) and "This installation (a person's act at the enrolled terminal)" (administration forms, and `enroll` moves here from goals). The three administration `status` forms and `wait resume` get a trailing "(installation)" tag on their usage line rather than a move.
- **Agent text** (`help agent`): same two sub-blocks, plus one protocol sentence: "Installation forms change this checkout or fleet, not the application; they are a person's act." No verb removed from the agent index (the design already says audience is a hint, not permission).
- **JSON** (`help --json`, `help COMMAND --json`): add one field, `role`, per command entry with values `workflow`, `administration`, `mixed`; for the five mixed verbs add `administrationForms` listing the usage lines (or first target word) that are administration. Add to `protocol` one entry `compatibilityCatalogue: {"helpArgv": ["metasystem","help","internal"], "note": "engine families for existing scripts; not a public task"}`. Nothing else changes shape, so `TestIntentAgentResultProtocol` and the forms tests stay valid with one added assertion each.
- **Legacy family**: leave `help internal` and `help <family>` exactly as they are; add only the pointer line (L6). Do not add family verbs to the JSON command index.

This is a labelling change on top of the implemented slice, not a redesign. L1 and L2's wording are separate follow-ups and should not block shipping the help slice.

## 4. Uncertainty
- L1 is a source-traced inference; no adoption was run and no abandon was executed. The vendored-prefix stamping path (`go-build.sh:67`) was not traced.
- Whether advanced options render in `help edit` and `help review` text was not checked; the JSON descriptions include them.
- `help check`'s promise that each problem names its fix was not verified against a foreign checkout.
- Where landing refusal codes are listed for `land --exception` was not traced.
- Tool budget used: 14 of 25 calls, no subagents, no mutations outside the two named reports.

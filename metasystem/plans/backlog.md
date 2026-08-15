# Backlog

- Goal and current status: the standing queue of ratified-but-not-scheduled work. Items leave by becoming a design stream, a retro proposal, or a deliberate rejection recorded here.
- Next step: none
- In flight right now: nothing
- Waiting on: nothing — items here are deliberately queued

## Queued

1. **Make "a turn may not end with unwatched work" a mechanism, not a rule.**
   Raised by the human 2026-08-08 after a delegate finished at 06:50 and was
   noticed at 07:38: "can that be a metasystem thing so that we ensure this
   happens? ... And if it is in memory, I'm not so sure if this is a reliable
   way of doing it." They are right that memory and ledger prose are the wrong
   home. Shape, to be designed properly: (a) `dispatch.sh dispatch` without
   `--wait` prints the exact waiter command for the job it just created, so the
   agent never invents a polling loop; (b) `dispatch.sh watch --job <id>` is
   that waiter — it blocks until the job is terminal and exits, which is what
   every runtime's background mechanism turns into a wake-up; (c) the stop hook
   REFUSES a turn that ends with jobs this main dispatched still in flight and
   no waiter running for them, the same once-only shape as the open-work
   refusal. Detection reads job records and process state, so it is
   runtime-neutral; only the wake-up itself is per-adapter plumbing. The honest
   limit to state in the design: nothing outside a session can wake it, so the
   mechanism enforces "do not end the turn blind" rather than "you will be
   woken". Carries IL-24, which is the prose version of this.
2. **Two bars for changes: the design loop for design changes, a direct fix
   for mechanical defects.** Raised by the human 2026-08-08 after they set this
   as the rule for one unattended run and asked whether it should be standard,
   "maybe with a few conditions added". It should, and the conditions are the
   whole point: without them "it's just a bug" becomes the escape hatch that
   swallows every design change. Shape, to be designed properly:
   (a) the classification is DECLARED, not assumed -- a change states which bar
   it took and why, so the choice can be audited later;
   (b) a named list that is NEVER a direct fix, whatever it looks like: an
   invariant, a contract or schema other tools parse, an authority boundary, a
   human ruling, or a safety mechanism. This session is the case for the list.
   The bm-2 delegates failing looked exactly like a bug ("Devin jobs are
   failing"), and the actual defect was in the ownership model -- design, not
   mechanics. The same session also had genuine direct fixes (a tar pipe, a
   working-directory slip) where a design chain would have been pure ceremony;
   (c) when in doubt, the loop wins -- the default on an unclear call is to
   escalate, not to proceed;
   (d) a direct fix that grows beyond the defect stops being one and escalates;
   (e) the EVIDENCE bar does not move. A direct fix still needs a
   failing-then-passing proof and the gates. Skipping the loop skips critique,
   never proof.
   Mechanism, not memory (the human's standing objection): a rule that lives in
   prose gets forgotten under time pressure, which is exactly when it matters.
   Candidates to evaluate in the design -- the commit wrapper requiring a
   declared classification, and paths that carry invariants or schemas requiring
   a design-chain reference before they can be committed to. Whether that is
   enforceable without becoming bureaucratic is the real design question.

3. **Streaming visibility inside the per-turn lifecycle (human-ratified
   direction, 2026-08-09).** Delegates are silent until exit: a 319-second
   SWE-1.7 inference is indistinguishable from a hang, an empty reply costs
   the whole session to discover, and a dead runner went unnoticed for 4.5
   hours. The fix is NOT a lifecycle change: one process per turn stays (the
   lease, census, and reaper are built on process exit as the turn boundary).
   Each runtime's native streaming lands inside that pattern: claude
   `--output-format stream-json`; codex `exec --json` already emits JSONL we
   merely fail to tail; devin runs `devin acp` PER TURN -- the human's
   synthesis: ACP as a transport within the turn, not a persistent server --
   which also moves permission answering to our side of the wire (devin is
   the one runtime with notEnforced everywhere). Supervision gains one
   additive signal: a per-turn last-event-age heartbeat the reaper and any
   watch can read. Honest gaps to resolve in the design: ACP has no standard
   usage reporting (today ACU/tokens come from ATIF final_metrics at exit),
   the turn-end evidence boundary moves to the JSON-RPC response for devin,
   and devin's ACP loadSession support needs a probe. Sequenced AFTER the
   bm-2 cohort completes -- changing the instrument mid-cohort makes
   repetitions incomparable. Design loop: it touches the turn-end and usage
   contracts. None of this makes inference faster; it makes slowness and
   death visible while they happen. CONSOLIDATED 2026-08-09 into
   plans/adapter-streaming.md at the human's request (since folded into the flight-recorder stream, plans/flight-recorder.md, which owns sequencing) -- the full design draft
   (the abstract adapter interface, per-runtime transports, the liveness
   sidecar, ACP-per-turn with its probe questions) lives there; this entry is
   now just the queue marker.

4. **Fixtures-as-arbiter: a principled exit from fine-grained critique
   (human-ratified in one instance 2026-08-09; generalize).** During the
   flight-recorder chain the human set a close rule after six falling rounds
   (21, 15, 10, 4, 3, 2): remaining detail-grain findings are folded, then
   implementation STARTS, with the folded findings expressed as NAMED
   FIXTURES and code-critique as the next reviewer — prose review stops
   being the arbiter once the grain drops to timestamp formats and field
   caps, because at that grain the implementation's own tests judge better
   and cheaper than another prose round. The human: "I really like the
   approach... you fold the remaining issues into fixtures that then
   together with code critique become the arbiter." Not yet a standing
   rule; it lived in one plan header. Conditions that keep it from becoming
   an escape hatch, to be designed into the critique skills alongside the
   stop-rule sharpening below:
   (a) the trajectory must be FALLING and the chain past its round budget;
   (b) every remaining finding must be mechanical-grain — it names a value,
   format, or bounded choice, not an invariant or contract shape; one
   invariant-grade finding and the exit is unavailable;
   (c) the fold is 1:1 — each finding becomes a NAMED fixture obligation in
   the plan's Proof section, so nothing dissolves into prose;
   (d) code-critique of the implementation becomes MANDATORY, not the
   usual per-batch judgement call;
   (e) the switch is RECORDED in the plan header with the trajectory, as
   the flight-recorder instance was.

5. **Qualified-name sweep of agent-facing text (human-raised 2026-08-09).**
   The glossary's namespacing rule — projects own bare words in their own
   workspace; metasystem prose speaks "delegate job", "recorder event",
   "mission gate", "mission runner", "host turn" — is stated but not yet
   enforced where it matters: the dispatch prompt templates, role briefs
   (`scripts/agents/roles/*`), the skills, and docs/. Sweep them, and add
   the rule to whatever lints prose (the audit's placeholder check is the
   natural home for a collision-prone-bare-word check in agent-facing
   templates). Raised because bm-2 literally builds a task runner: a
   delegate reading "check the job record" inside that workspace cannot
   know which system is meant.

6. **Per-delegate caps in the benchmark roster (human-directed 2026-08-09),
   then the two comparison cohorts.** The bm-2 post-mortem was decisive: a
   timed-out Devin implementer left 11 Java files, 1,322 COMPILING lines in
   its worktree — killed by the uniform 15-minute job cap, work discarded,
   mission shipped a skeleton (acceptance 1/53 in both repetitions). Not
   the model, not the CLI, not a harness bug: the cap fits claude/codex
   pacing and truncates Devin's slower-but-productive pace (91-98% of its
   wall clock is inference). Work, in order:
   (a) DESIGN LOOP (fence/roster contract change): the spec's roster gains
   per-delegate caps — at minimum wall-clock minutes per job, considered
   alongside the other fence dimensions (jobs, concurrency) — flowing from
   manifest through mission fences to dispatch --cap-min. The human asked
   for exactly this: "where we specify the agent we should also specify
   the budget for that agent".
   (b) THE CAP-DISCOVERY EXPERIMENT (human-specified 2026-08-09): the
   cap keys on the (RUNTIME x MODEL) PAIR — the roster entry — not the
   runtime alone (human's refinement: Devin on a fast model and Devin on
   a slow model are different animals; runtime-only would be too crude),
   with role as an optional further dimension. WHERE (human, 2026-08-09):
   the cap is declared exactly where the binding is declared — in
   metasystem.conf beside the role.<role>.runtime and
   role.<role>.model.<tier> keys for ordinary operation, and in the
   benchmark manifest's roster for cohorts; dispatch resolves the cap from
   the most specific binding that matches the job, and the design decides
   the precedence order once. For Devin on
   swe-1-7, do not guess: set the cap ONCE deliberately very high
   (effectively unbounded within the mission), run, and CLOSELY MONITOR
   actual demand while verifying it is still progressing — progress proxy
   until adapter streaming lands: worktree churn (files appearing and
   changing) plus the session state, sampled by the watch. The observed
   natural completion time, with a safety margin, IS the sensible cap.
   The experiment mission must account for fence interactions: one long
   implementer job consumes most of a 3-hour wall clock, so the
   experiment's spec raises the wall-clock fence (or runs one repetition)
   rather than letting the outer fence masquerade as the answer.
   (c) Comparison cohorts on the same spec: Opus delegates and codex sol
   delegates (new specs per the bm-2 precedent, each with its roster
   ruling and cost note — both are PAID delegates, unlike swe-1-7).
   Together the three runs separate model, product, and structure.

7. **Critique stop-rule sharpening (human-ratified 2026-08-07, rides the
   next retro's change gate).** A chain closes on a round with zero
   UNREFUTED material findings: each refutation carries evidence, survives
   exactly one rebuttal round, and persistent disagreement escalates to
   the human. Today the stop rule counts what the critic returns, giving
   the critic an implicit veto over closure; with IL-21 forcing refutation
   attempts, that veto will start binding. Owner doc when adopted:
   `skills/design-critique/SKILL.md` (and code-critique's mirror section);
   the join script gains the unrefuted-count check. Wido: "I like the
   sharpening and I think we should implement it."
8. **KI-23 acknowledged-process mechanism** — after the one-writer
   implementation lands (same files).
9. **Mission-completion protocol design** (plans/mission-completion-protocol.md,
   seven carried findings) — after the coexistence stream closes.
10. **Benchmark kit halves** — schema-v2 alignment DONE 2026-08-09: every
   role return schema accepts schemaVersion/claimed/reviewedCommit/
   chainClosed, the census schema accepts v2 (generation, stateDigest),
   turn result is optional (failed turns have none), chainClosed is
   root-only in job records. Proof: re-extracting the finished bm-2 cohort
   yields ZERO schema-shaped gaps (was four kinds). REMAINING in this
   item: the censusScanMilliseconds watch samples only the FINAL census,
   which always completes after the run window, so the watch never has a
   value — it needs a during-run duration source (pre-existing limitation,
   visible in every scorecard to date).
11. **Confirming benchmark re-run** (Opus critics vs luna builders, the
   0.019→0.981 effect) — with the human present, after the above.
12. **Devin integration** — DONE 2026-08-09: adapter and host proven, bm-2
   cohort complete with two graded repetitions; follow-on work lives in
   item 6 (caps and comparison cohorts).
13. **Incremental Go migration, ruled 2026-08-09 (the human).** The standing
   rule: NEW process-critical components are built in Go behind the
   EXISTING file/CLI contracts; proven bash stays until a component comes
   up for rewrite anyway; there is never a big-bang port. Rationale: the
   components talk through files and CLI verbs, so a Go binary is invisible
   behind the same interface, and the fixture suite — subprocess in, files
   out — remains the arbiter unchanged. Go's wins are implementation-level
   (no set -e/quoting/torn-string bug classes; exact kernel process start
   times shrink the REG-6 whole-second residual); protocol design stays
   language-independent and keeps the design loop. First component:
   supervision (the ruling is recorded in plans/supervision-lifecycle.md,
   Implementation order). Candidates after that, only as they come up for
   rewrite: dispatch's record/CAS layer, the census, the registry tooling.
   THE CUT, refined 2026-08-09 (the human): the end state is TWO languages
   — Go and shell — with NO Python or other scripting languages. The
   principle: A DECISION LIVES IN GO; AN INVOCATION LIVES IN SHELL. Go
   owns everything that writes a record, takes a lock, proves a death, or
   validates a contract; shell owns composition — adapters invoking
   runtime CLIs, the suite sequencing fixtures, hooks, and thin
   contract-stable wrappers (`arm-supervision.sh` ends as
   `exec metasystem supervise "$@"`). A wrapper that grows an `if` about
   system state has strayed. Shape: ONE multi-verb binary (`metasystem
   supervise|lease|census|event|record ...`), git-style, shared internal
   packages; binaries are never committed — the suite builds first and
   wrappers fail loudly when the binary is missing. Every `.py` file dies
   by the come-up-for-rewrite rule, never by a sweep; inline python
   heredocs in bash (json_field and kin) become binary verbs as their
   host scripts thin, which also removes dispatch's dozens of
   python3 spawns per verb (the KI-4 lesson). Fixtures stay shell on
   purpose: black-box and language-neutral is what makes them the
   migration's arbiter.


14. **A generic goal system for every agent in the metasystem
   (human-raised 2026-08-14).** The problem, in the human's words: "we lose
   track of the goal that we are chasing." Motivating incident, same day:
   the stop hook told the human "NOTHING LEFT TO WORK ON in this checkout"
   while roughly a quarter of a 101-finding program remained — the backlog
   lived in docs/reviews/, invisible to the open-work scanner, and even
   after the narrow fix (a plans/ note) the turn-end verdict only says THAT
   open work exists, not WHAT to do next. The ask: a goal facility like the
   session-level task tracking Claude has natively, but generic — one
   mechanism, working identically for every agent operating inside the
   metasystem, any runtime. Shape to be designed properly: (a) a declared
   goal with an explicit, named next step, owned like a plan note but
   consumed by machinery; (b) the stop-hook verdict names that next step
   VERBATIM — "the task may be done, but proceed with <next step>" — so an
   orchestrating agent continues instead of idling, with no ambiguity;
   (c) runtime-neutral: the same surface for claude, codex, and devin
   agents, host and delegate alike; (d) relates to backlog item 1 (turn
   may not end with unwatched work) — that item guards in-flight work,
   this one guards the thread of intent between turns; the design should
   say whether they are one mechanism or two. Path per the human: design,
   critique, implement — queued deliberately, not scheduled now.

15. **Tracked long-running work with terminal-state watching — the
    monitor facility as METASYSTEM behavior (human-raised 2026-08-14).**
    The human's observation, verbatim intent: the launch-detached +
    monitor + resume pattern proven during the D33 work "is generic
    behavior that should end up in the meta system." Today it exists
    only in the orchestrating agent's own harness (a Claude Code
    Monitor watching a log condition) plus hand-written continuation
    prompts — the metasystem cannot see a detached suite run at all
    (the census would flag it UNTRACKED), and no other runtime gets
    the pattern for free. Motivating evidence, same day: the harness's
    10-minute cap killed tracked suite runs, forcing detached runs; a
    monitor then caught a 205-second suite failure immediately, where
    the fallback timer would have slept 15 minutes on it. The shape:
    the metasystem registers a long-running RUN (suite, benchmark
    cohort, any detached process) as a record with pid/start/log/
    expected-verdict; a watcher pass (the existing
    watch-background-jobs machinery is the natural host) emits its
    terminal state as an event; the continuation — WHAT to do on
    green, on red, on hang — rides the record, which is where this
    composes with backlog item 14's goal system: 14 carries "what
    next", this carries "when". Runtime-neutral like 14: the same
    facility for every agent operating inside the metasystem, not a
    Claude-specific convenience. Design → critique → implement; queue
    behind item 14 or fold the two designs into one pass if 14's
    critique reaches for run-watching anyway.

16. **Runtime-agnosticism audit: the core must never name an agent
    (human-raised 2026-08-15).** The human's observation, verbatim
    intent: seeing `if !strings.Contains(command,
    "$CLAUDE_PROJECT_DIR/metasystem")` — "we are building something
    that is tied to Claude. HOWEVER the meta system must be agent
    agnostic (it should work with Codex and Devin and any other
    future agent too)." The evidence sites as of be3f195:
    internal/hooks/hooks.go hardcodes Claude Code's hook config
    grammar and the $CLAUDE_PROJECT_DIR env var in a CORE package —
    the `hooks` self-check ("does this repo run under its own
    metasystem") is therefore DEFINED as a Claude-only concept, and a
    codex or devin session gets no equivalent check at all;
    internal/audit/metasystem.go requires the Claude-specific
    CLAUDE.md by name in the instruction-asset audit (softer, but the
    same pattern: core code naming one runtime's file). The sanctioned
    home for runtime knowledge is the adapter seam
    (internal/adapter/{claude,codex,devin}.go, the per-runtime verbs,
    scripts/agents/adapters/*.sh) — the design goal is that every
    runtime-integration surface (hook shape, config pointer file,
    session env names) is DECLARED by its adapter and the core
    consumes declarations, so adding a future runtime touches only
    its adapter. Shape: one audit pass enumerating every runtime
    name outside the adapter seam (rg claude|codex|devin over
    cmd/ internal/ minus the seam), a ruling per site (move behind
    the seam, generalize, or document why it stays), and the
    agnosticism rule stated in docs/architecture.md as standing
    doctrine. Design → critique → implement; queue with items 14-15
    (the monitor facility already carries the same runtime-neutral
    requirement).

17. **Runtime code lives in its runtime's file — find and fix the
    strays (human-raised 2026-08-15).** The human's observation:
    `DevinPermissionMode` is declared in internal/adapter/codex.go —
    "I would not expect anything dev inside a file called Codex —
    this makes me think there's a bit of a mess up here. The question
    arises: is that happening in other places too?" Confirmed at
    d029177: codex.go:132 exports DevinPermissionMode; a first scan
    finds no second obvious stray in internal/adapter (remaining
    non-matching names are file-local helpers). Mandate: dig in and
    figure out what's going on — why the function landed there
    (shared origin? copy-drift?), whether subtler cross-runtime
    leakage exists in internal/adapter, the per-runtime cmd verb
    files (main.go's header claims one file per verb family), and
    scripts/agents/adapters/*.sh; move each stray to its runtime's
    home; and decide whether the one-runtime-per-file convention
    deserves a mechanical check so it cannot drift again. Overlaps
    backlog item 16 (agent agnosticism): 16 rules on runtime names
    outside the adapter seam, this one rules on placement WITHIN the
    seam — one audit pass can serve both. Not scheduled now, per the
    human.

18. **ACP as the delegate transport (human-raised 2026-08-15).** The
    human's question, on the Devin delivery failures: "Is there no way
    to use ACP to make this more robust?" There is: the installed CLI
    ships `devin acp` (Agent Client Protocol server over stdio,
    verified at devin 3000.4.25). ACP replaces both failure roots the
    bm-2 arm exposed: permission requests arrive as JSON-RPC calls the
    adapter answers by policy — the envelope enforced per tool call
    with no dead-stop, which is the path to RETIRING the D61
    dangerous-mode waiver — and delivery becomes a typed end-of-turn
    event with streamed tool-call updates instead of stdout scraping
    (the D62 ladder becomes a live tap). Costs: the adapter seam gains
    a long-lived Go protocol client (session lifecycle, capability
    negotiation, envelope-driven permission answering, cancellation,
    usage parity); this is an architecture change deserving its own
    design + critique loop. Composition: feeds item 15 (streamed
    events are the monitor facility's raw material) and item 16 (ACP
    is runtime-neutral — Claude Code and Codex-class CLIs speak it,
    so adapters shrink toward launch configs behind one client).
    Prototype against `devin acp` first; the graded-permissions
    restoration is the acceptance story. Sequenced by the human's
    ruling when the current series closes.

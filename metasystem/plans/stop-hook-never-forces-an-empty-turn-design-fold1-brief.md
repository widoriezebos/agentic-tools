Finding Id: stop-hook-design-fold-1
Disposition: accepted

# Finding Being Corrected

The independent design read (Codex, gpt-5.6-sol, 2026-09-12 17:3xZ) of
stop-hook-never-forces-an-empty-turn-design.md returned six material
findings. Each changes what gets built; fold every one into the page. Then
add one section the page lacks: the work is too large for one landing, and
the program's rule is one mechanism per member goal.

# Disposition Reasoning and Evidence

Keep the page you wrote in round 1 (stop-hook-never-forces-an-empty-turn-design.md
in the metasystem plans directory) and revise it in place; touch nothing
else. Do not commit. Every path you cite must exist in the worktree under
the metasystem/ prefix. Stay under 260 lines, plain English, short
sentences.

Fold these six findings, each with the evidence the critic gave:

1. The refusal census is incomplete. Invalid arguments are not "before
   Stop starts": the Claude launcher turns any nonzero hook exit, including
   metasystem/scripts/agents/supervision-hook.sh lines 15 and 16, into an
   unpersisted raw refusal (metasystem/scripts/enforcement/claude-code-hooks.json,
   line 25). And "turn verdict unavailable" collapses distinct engine
   producers: state-root, fence, session-stop, verdict-state, status-write,
   state-write, consume and lock failures all call failClosedTurnVerdict
   (metasystem/internal/goal/turnverdict.go lines 229 to 250, 260 to 308,
   337, 412 to 418; metasystem/internal/goal/sessionstop.go lines 428 to
   445 and 484 to 501). List each producer in section 1 with its class,
   and say in section 2 how the launcher's raw block is removed or
   replaced and how every fail-closed producer migrates to the typed
   classification.
2. Runtime independence and its proof do not hold as written.
   metasystem/scripts/agents/adapters/runtime-common.sh owns delegate
   lifecycle plumbing only (lines 3 to 13) and the fake adapter does not
   source it (metasystem/scripts/agents/adapters/fake.sh lines 134 to 136);
   the no-hook fallback in
   metasystem/docs/design/turn-verdict-delivery-contract.md (lines 26 to
   32) prints `goal next` and enforces nothing; the existing hook fixtures
   invoke the shell directly (metasystem/scripts/agents/supervision-hook-fixtures.sh
   lines 672 to 694, 909 to 945, 1162 to 1171). Define the real common
   adapter boundary every runtime's stop gate calls, and make the
   two-runtime proof two production runtimes (the roster hosts claude,
   codex and devin); the fake stays an auxiliary bed.
3. DONE clause 1 is not met for the idle refusal: the mandatory idle
   refusal offers an attended human's `session stop`
   (metasystem/internal/goal/turnverdict.go lines 490 to 496), not a
   command the seat can run to clear the condition or start the work. Name
   the lawful seat-executable action for the idle refusal (the claim and
   dispatch of the ready goal is the obvious candidate; say which verb).
   The cited command sources are wrong: metasystem/cmd/metasystem/run.go
   lines 240 and 433 are verb parsers; run-watch's renderer is at lines 80
   to 82 and no job-watch renderer exists. Cite the real renderers or name
   the one to add.
4. "Reported to the steward" fails when incident storage fails. The normal
   path is concrete (the version-2 record, the steward tick, QueueNotification
   in metasystem/internal/steward/intervene.go lines 315 to 331); the
   fallback is not: the tick consumes only ledger-attention events
   (metasystem/internal/steward/tick.go lines 237 to 245) and the hook log
   records only decision and elapsed time (supervision-hook.sh lines 878
   to 889). Either specify an independent durable record with its steward
   reader, or weaken the guarantee to "delivery unconfirmed" and say so in
   DONE's terms.
5. The idle-with-backlog path is not untouched: today's unchanged digest
   holds every nonterminal job (turnverdict.go lines 509 to 521), gathered
   without goal, revision or owner joins (metasystem/internal/goal/project.go
   lines 434 to 470); the page narrows it to relevant activity, so another
   seat's job churn stops resetting the counter. State this as the contract
   amendment it is, name the test that pins the new behavior, and keep
   DONE clause 4's three-refusals handoff intact.
6. The seen-state is not scoped to one stop deadline: the episode is a
   rolling 60 seconds from first observation and excludes attempt and
   generation, so it can coalesce distinct deadlines and suppress the
   steward report for a new one. Key the episode to the actual deadline
   coordinates (the hook's turn generation and its deadline end), not an
   inferred window.

Then add section 7, "Slices": split the mechanism into member goals of one
mechanism each, in landing order, each with its own DONE sentence and the
fixtures from section 5 that belong to it. The rule (Wido, 2026-09-10):
no large goals; an umbrella record for context and one-mechanism members
with their own DONE. Name each member as a goal id in the program's
spelling (lower-case words joined by hyphens) and say which DONE clauses
of the umbrella it carries.

Wall clock: 25 minutes. Stop and report if the page is not finished by
then.

evidence carries exactly two `{command, observed, level}` items,
replayable from the worktree's repository root: (1)
`git -C metasystem status --short` observing the one changed file; (2)
`( cd metasystem/plans && wc -l stop-hook-never-forces-an-empty-turn-design.md )`
observing the line count. whatWasDone names the six folds in one line each
and the member goals of section 7.

# Unchanged Return Contract

The original role and return schema remain binding without additions, removals, or relaxations.

Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.

Schema: the same scripts/agents/schemas/implementer.schema.json used for the original dispatch

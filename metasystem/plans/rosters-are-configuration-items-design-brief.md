# Design brief: rosters-are-configuration-items, revision 5 (one roster family for every agent, shared by the host)

## Revision

Revision: revision 5 of the design `plans/designs/rosters-are-configuration-items.md`. Revisions 1
to 4 were written on 2026-10-01 on the ui seat and live only on branch `ui-development` (commit
9faaf905a); none was accepted and none is on main, so on main this is the goal's first design
record.

Reason: on 2026-10-02 Wido added five requirements to the goal that revision 4 contradicts. Wido's
question, verbatim: "the running agent; is that defined in the roster which agent and which model?
That should be the case". The goal's next step now says the roster must "cover EVERY agent kind
incl. the seat agent and the landing lane agent; be ONE key family; be shared by every checkout on
the host (one place to change a model); refuse an unknown key instead of silently falling back;
have a 'roster show' verb plus the Settings view; include effort (Wido 2026-10-01)". Revision 4
kept rosters per checkout in a git-ignored folder, answered a missing roster or kind from today's
keys, and left the seat agent and the steward's continuation outside the roster. Revision 5
redesigns those parts and keeps what still holds.

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check.

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

### What the goal asks (the goal record, verbatim parts)

Intent: "A roster becomes a configuration item of its own: a named set that says which runtime and
model does each kind of work (design, design critique, build, code critique, and so on). There can
be many rosters, each with a type that says where it applies: one per risk tier, one for the
landing lane, one for the Project Partner. People see and change them in the Settings page of the
interface and with a few plain verbs, instead of editing many separate lines in the configuration
file."

Wido, 2026-10-01, in the same record: "I want a goal and a design for rosters as a configuration
item, but separate (not a bunch of line level setting in the config). Reason is I want many
rosters, one per risk tier, one for the 'lane', one for the project partner. So this needs a
roster-type (identifier) and a UI and verb(s) for maintaining them easily. UI in the settings of
the metasystem."

Next step: "Fable writes the design (the roster item and its types, where rosters live, how the
engine picks the roster for a piece of work, the verbs, the Settings page, and the path from
today's keys), one Astra critique, then Wido decides the build. Done when: rosters of each type can
be listed, created and changed from the Settings page and from the verbs, and work of each kind
runs on the roster its type selects."

Added 2026-10-02, verbatim: "today runtime/model/effort are scattered over two overlapping key
families (launch.<kind>.* and role.<role>.*) and copied per checkout (five seat checkouts on this
host). The roster must: cover EVERY agent kind incl. the seat agent and the landing lane agent; be
ONE key family; be shared by every checkout on the host (one place to change a model); refuse an
unknown key instead of silently falling back; have a 'roster show' verb plus the Settings view;
include effort (Wido 2026-10-01)."

The record's pros and cons: one place to see and change who does what, a different roster per kind
of work, easy edits from the interface; against that, the engine has to read rosters instead of
today's keys, and existing seats' settings need a clear path over.

### Decisions already made (do not reopen; the page states them as given)

- G1, the goal's five requirements of 2026-10-02 above, and Wido's standing instruction for this
  goal's designs: "build the smallest thing possible that works".
- G2, effort lives in the roster. Wido, 2026-10-01, quoted in revision 4's head: "model and effort
  live in the roster; changing them means changing the roster" (R-28-m1 archived), and "builds run
  at xhigh; effort becomes part of each roster" (R-90-m1 kept). R-90-m1 is row 147 of
  `memory/rulings.md`; it names the hazard table as "the enforcer to sweep".
- G3, the content of the rosters is not this page's decision. R-133-ui (`memory/rulings.md` row
  192): design on Claude Fable `claude-fable-5-1`, design critique on Codex Astra `gpt-6-astra`,
  build on Claude Opus `claude-opus-5-5`, code critique on Codex Sol `gpt-6-sol`; the author and
  the reviewer of a piece of work are different models. R-123-m1u (row 182): a model never routes
  alone, every row names its runtime. R-106-m1e (row 165): a launch whose model is a template
  placeholder is refused before it spawns, and the shipped `metasystem.conf` carries no model. The
  glossary's Roster entry: "Changing it is a human ruling".
- G4, the types Wido named stay: one roster per risk tier, one for the landing lane, one for the
  Project Partner.

### The prior revision and its critique

Revision 4, complete, saved from commit 9faaf905a:
`/Users/wido/LocalStorage/agentic-tools-evidence/rosters-20261001/rosters-are-configuration-items-rev4.md:1-99`.
Read it whole. Its sections 1 and 3 are a careful trace of today's readers; reuse what is still
true after checking it against main.

Astra's two critique rounds on revisions 2 and 3, for the findings and their evidence:
`/Users/wido/LocalStorage/agentic-tools-evidence/rosters-20261001/astra-r1.md` and
`/Users/wido/LocalStorage/agentic-tools-evidence/rosters-20261001/astra-r2.md`.

What revision 5 must change, because a 2026-10-02 requirement contradicts revision 4:

1. Where rosters live (revision 4 section 2): per checkout in a git-ignored `rosters/` folder.
   Now one copy shared by every checkout on the host.
2. Fallback (revision 4 sections 2 and 3): a missing roster or kind was answered by today's
   `role.*`, `launch.*` and `ui.partner.*` keys and the compiled defaults. Now the roster is the one
   key family, and an unknown or retired key is refused, never silently read.
3. Coverage (revision 4 section 3): the seat lane and the steward's continuation were "not kinds",
   and the verifier and investigator stayed on `main`. Now every agent kind the engine starts is
   a roster row.
4. The two-row display (revision 4 section 4, Astra RO-05 and RO-06) existed only because two
   legacy readers could disagree. With one key family it should disappear; say so if it does.

What revision 4 settled that revision 5 keeps unless a requirement above forces a change (each
was a material Astra finding, so dropping one needs a stated reason):

- The tier is the goal's risk tier; work whose tier the reader cannot learn uses the tier-3 roster,
  the engine's own precedent for an unknown tier.
- A roster's effort reaches execution on every path, including Claude's dispatch command through
  `claude --effort` (Astra RULING-RO-01).
- The saved model selections (`unitReadModel`, `plan.Read.Model`, the standalone read's frozen
  model) resolve through the roster, or a goal's review silently runs on the old model (RO-02).
- The landing agent resolves its settings from the registered lane checkout, not from the seat
  (RO-03). A host-wide roster may make this moot; say whether it does.
- The Partner reads its runtime once, when the interface starts; a change takes effect on
  `metasystem ui restart`, and the page shows configured and active values until then (RO-04).

### Facts from the code (claims to check)

A read-only trace on 2026-10-03 (by seat m1g at main 5f1af9a61; the cited files are unchanged on
main since, except the three marked "rechecked"). They are claims: check each one the page relies
on at its cited lines, and say on the page when one is wrong. Paths are relative to the
`metasystem/` module.

Keys and checks

1. `internal/config/defaults.go:15-40`: the `Setting` type and `compiledSettings`, the one table
   of compiled defaults. It is not an allowlist. `:259-309`: the `role.*` and
   `mode.design.role.implementer.*` rows. `:323-397`: the `launch.*` runtime, model, effort and
   window rows.
2. Every compiled key that picks a runtime, model or effort today: `role.default`,
   `role.implementer`, `role.code-critic`, `role.design-critic` (each `.runtime` and
   `.model.claude|codex|devin`), `role.investigator.runtime`, `role.verifier.runtime`,
   `mode.design.role.implementer.runtime` and `.model.claude|codex|devin`, `launch.build`,
   `launch.critique`, `launch.design`, `launch.read` (each `.runtime`, `.model` and
   `.model.claude|codex|devin`), `launch.build.effort`, `launch.seat` and `launch.landing` (each
   `.runtime`, `.model`, `.effort`), and `ui.partner.runtime` and `ui.partner.model`. Not about
   who runs what, and out of scope: `metasystem.runtimes`, `runtime.R.maximal-models`,
   `runtime.R.model-alias.M`, `model.tier.N` (the escalation ladder), `ui.partner.command.R`.
3. `internal/config/validate.go:57-75`: only key shape and duplicates are refused. `:437-451`:
   roles and modes are enumerated from whatever keys exist. `:511-577`: the launch-lane check
   covers build, critique, design and read only. There is no unknown-key check: a misspelt
   `role.NAME.runtime` is enumerated as a role and never read by dispatch, and
   `launch.seat.*` and `launch.landing.*` are not checked. `settings set`
   (`cmd/metasystem/intent_work.go:2089-2136`) writes any key to the local file.

The dispatch reader (roles)

4. `internal/dispatch/roster.go:145-296`: `ResolveRoster`, the one resolver for a dispatch role:
   runtime `role.R.runtime` then `role.default.runtime`; model `role.R.model.RUNTIME` then
   `role.default.model.RUNTIME`; a placeholder is refused; `main` cannot be dispatched.
   `internal/config/resolve.go:303-416`: `Get`; mode scoping applies only to role runtime and
   model keys.
5. Callers: `internal/delegation/dispatch_phase.go:245-253` (main dispatch; an unknown role is
   refused at `:152`), `cmd/metasystem/goal_branch.go:622-650` (the code critic),
   `cmd/metasystem/goal.go:733-741` and `cmd/metasystem/steward_verbs.go:207-214` (the steward's
   continuation), `internal/steward/handoff_capture.go:1096-1099` (role `steward-continuation`).
   Other readers: `internal/adapter/supervisor/codex.go:193-200`,
   `internal/adapter/supervisor/external.go:311-315`, and adoption tailoring, which rewrites
   `role.*` and `launch.*` (`internal/validate/conftailor.go:126-268`).
6. Dispatch roles are the embedded files under `internal/protocol/roles/`, with no Go list
   (`internal/protocol/protocol.go:89-97`). Dispatchable today: behavior-judge, code-critic,
   design-critic, implementer, investigator, steward-continuation, verifier, warden. Roles with no
   compiled row (warden, steward-continuation, behavior-judge) run silently on `role.default.*`.
   No `role.R.effort` key exists; a dispatched job's effort comes from the hazard table
   (`internal/dispatch/hazard.go:37-74`).

The launch reader (kinds)

7. `internal/launch/settings.go:13-42`: the key constants. `:142-155`: the seat and landing
   agents borrow the build lane's per-runtime model. `:157-229`: `resolveSettings`. `:314-352`:
   critique, design and read all take `launch.build.effort`; only seat and landing have an
   effort of their own.
8. `internal/launch/launch.go:105-131`: `Manager.Start` applies the settings. `:728-746`:
   `adapterForLane`, the launch kinds: build, critique, design, read, seat, landing, and proof
   (which has no model).
9. Callers: `cmd/metasystem/steward_seat.go:57-72` (the seat a steward starts) and
   `cmd/metasystem/landing_agent.go:111-141` (the landing agent, on the lane checkout's settings).
   There is no `launch.steward.*`; the steward's delegate is the dispatch role
   `steward-continuation`.

The Partner

10. `internal/config/ui.go:223-249` (`UIPartner`) and `:279-350`; read once when the interface
    starts (`cmd/metasystem/ui.go:263-284`). `config.Validate` does not check these keys.

Layering and shared state

11. `internal/config/resolve.go:295-302`: precedence, highest first: flag, environment, local
    mode-scoped role key, local base key, committed mode-scoped key, committed base key, compiled
    default, the caller's default. No host-wide or cross-checkout settings file exists.
12. State shared by every checkout on a host already lives under the registry's home, normally
    `~/.metasystem`, redirected for tests by `METASYSTEM_SUPERVISION_REGISTRY_HOME`
    (`internal/board/card.go:158-176`); the host records sit in its `host/` directory beside a
    lock file (`internal/landing/lane/lane.go:75-82`). On this host that directory holds the
    landing lane's record, the board and the claim locks. Nothing there is a setting today.
13. Test reach of a key change: 64 test files under `internal/` and `cmd/` (235 lines) set or
    assert `role.*`, `launch.*` or `ui.partner.*` runtime, model or effort keys. The project's
    benchmark configurations pin their own cohorts with today's keys
    (`development/project-rules-local.md` at the repository top, the second rule).

Verbs and the Settings page

14. Rechecked: verbs are rows of `intentCommands` (`cmd/metasystem/intent.go:120-148`); the
    object groups, summaries and administration objects are at
    `cmd/metasystem/intent.go:1314-1350`. `settings show|keys|set|check` are at
    `cmd/metasystem/intent_work.go:318-366`.
15. Rechecked: the Settings page (`internal/ui/web/_app/src/panes/Settings.tsx:22-43`) shows
    About, Private store, Landing gate and Appearance; the landing gate card shows each value with
    the source it came from (`internal/ui/web/_app/src/panes/Settings.tsx:99-160`). No HTTP route
    writes a setting; the only human writes are the goal acts (`internal/ui/httpd/acts.go:16-57`).
16. Rechecked: the Settings empty state still promises "Pages for runtimes and models ... will be
    read and changed here" (`internal/ui/web/_app/src/panes/empties.ts:47-55`).

Cited code excerpts:

1. `internal/launch/settings.go:142-155`, the seat and landing agents borrowing the build lane's
   model (read it there; no excerpt is quoted here, so nothing can drift).

Example page:

`plans/designs/critique-findings-need-proof.md` is the example for structure and level of detail:
short sections, a verified "what happens today", "step 1: what exists after the slice", "proof
for step 1" as named tests, "later, when it hurts", and "open questions for Wido" with the
recommendation first.

### What the page must decide

1. **The roster item.** What one roster holds: an identifier, a type, and one row per agent kind
   with runtime, model and effort. Name every agent kind the engine starts today from facts 6, 8
   and 9 and the Partner, the seat agent, the landing agent and the steward's continuation
   included, and say which row each reads. One vocabulary for the kinds: say which of today's two
   (dispatch roles, launch kinds) survives or what replaces both, and how a dispatch role and a
   launch kind that do the same work (implementer and build, code-critic and read, design-critic
   and critique, design mode and design) map to one row. Say what a row means for the kinds that
   run in the calling session today (`main`). Effort has one owner: say how a row's effort and the
   hazard table's required effort relate so they cannot disagree silently (G2).
2. **Types and how the engine picks.** For each launch and dispatch path, the rule that selects
   the roster (goal work: the goal's tier), the rule for work with no goal or an unknown tier, and
   which roster the seat agent, the steward's continuation and the warden read. When the selected
   roster is missing, lacks the row, or names a runtime this installation does not list: a
   refusal that names the roster, the row and the one command that sets it, never a fallback to
   another roster, an old key or a built-in default. Say where the rule that the author and the
   reviewer of a piece of work are different models is checked, if anywhere.
3. **Where rosters live.** A separate item, not lines in `metasystem.conf`, one copy shared by
   every checkout on the host (fact 12). Decide the place and the file form; how tests point it
   at a temporary home; what a second computer of the fleet has (its own copy); what a host with
   several different projects adopted shares or does not; what a brand-new host or installation
   has before anyone writes a roster, given R-106-m1e and "refuse ... instead of silently falling
   back"; who writes the file (only the engine, through the verbs and the Settings page); and how
   two checkouts changing it at once keep it whole.
4. **The verbs.** `roster show` by name, and the fewest further plain verbs that list, create and
   change rosters in the engine's OBJECT ACTION grammar, with the lines a person reads on success
   and on each refusal. Changing a roster is a human ruling: say who may run the changing verbs
   using the engine's existing proof of a person's act, and put any change to that rule under open
   questions instead of deciding it.
5. **The Settings page.** What it shows for rosters and how a person changes one there, on a
   phone 390 pixels wide, in plain words (no configuration keys or internal codes by default). The
   page and the verbs go through one engine path. Say what the page shows for a missing or broken
   roster and for the Partner between a change and the restart.
6. **The path from today's keys.** The exact list of keys the rosters replace (fact 2). How the
   five seat checkouts and the lane checkout on this host move over: one command that reads the
   values a checkout resolves today and writes the rosters, or a hand step, and how a person sees
   the result before anything is refused. What the engine does with a retired key afterwards
   (`settings check` and every launch refuse it, naming the roster command that replaces it). How
   the 64 test files move (fact 13), with behaviour tests on per-test instances and no global
   state. How the benchmark configurations that pin their own cohorts keep running. No period in
   which one launch can read both a retired key and a roster and pick differently.
7. **Units.** At most 300 changed lines each, in landing order, each with its witness test and
   the mutation that turns it red. Say which is first and what step 1 is: the smallest thing that
   already serves Wido (for example: the host roster file exists, `roster show` prints it, and one
   launch path reads it with no fallback), with the rest listed after it. Implementation is Go;
   the Settings page is the existing React interface.

Open questions for Wido go in one list, recommendation first; order the units so that as little as
possible waits on an answer.

Out of scope, and named so on the page: which models the rosters hold (G3); prices and spend keys;
the runtime adapters' own flags and `ui.partner.command.R`; aliases and `model.tier.N`; the hazard
table's rules other than the effort sentence of item 1; sharing rosters between computers of the
fleet beyond what item 3 states.

## Recurring findings

From Astra's two rounds on revisions 2 and 3 of this design (the critique files above), the
classes that came back:

- A path that chooses a model before the roster reader runs and passes it on as an override (the
  unit runner, saved read models, the standalone read, the escalation ladder), so the roster is
  shown but not used. Trace every caller to the adapter.
- A value the page shows that execution never reads (Claude's dispatch command ignoring effort;
  the Partner captured at interface start). For every row, name the argument or field that
  carries it into the running agent.
- A file read from the seat's checkout when the reader runs in another checkout (the landing
  agent reads the registered lane's settings).
- One displayed value standing for two effective ones that can differ.

## Tool-call budget

Maximum delegate tool calls: 60

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: 4000 words

Cut a draft that exceeds this ceiling. If cutting would make the page
incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique
only through the context pack above. Never resume a delegate from an earlier
revision.

## Page artifact and return shape

Write the page to: the one file named under "Your output" above this request, starting with the
head given there. It is published as
`/Users/wido/LocalStorage/GitHub/agentic-tools-m1i/metasystem/plans/designs/rosters-are-configuration-items.md`.
End the page with a section "Threat model and rabbit-hole risks": our own agents and operators make
mistakes and crash, nobody attacks; name what could make this goal grow without end (the test
migration, adopted projects, the fleet) and where the page cuts it.

Return only these two lines:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1i/metasystem/plans/designs/rosters-are-configuration-items.md
DESIGN: ready (N words, with N the page's word count)
```

or:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1i/metasystem/plans/designs/rosters-are-configuration-items.md
DESIGN: blocked (the reason in one line)
```

When blocked, stop and return `BLOCKED:` with the refusal's first line, what you tried and the decision needed; your seat asks.

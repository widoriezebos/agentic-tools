# Help an agent choose and complete the right task

- Kind: design
- Id: 01M3FDYDMV0G5V9YC1M9XAKGZR
- Status: done
- Goals: verbs-match-intent

Author: root Codex. Wido requested design and implementation on 26 September
2026, explicitly preserving the human angle. The existing machinery bypass and
Opus/Fable-only delegate roster continue. This extends the completed intent
workflows; it changes discovery, never execution or authority.

Design critique closed at round 1 on two fixture-expressible obligations (AH-D1,
AH-D2); root adjudication is in `plans/agent-help-dispositions.md`. Independent
implementation critique closed at round 1 with zero material findings; the final
root correction and verification are recorded in `plans/agent-help-verification.md`.

## Step 1 and success

Ship one useful help improvement: a compact agent entry point, public structured
discovery, and focused contracts for the overloaded review command. An agent can
distinguish feedback from committing work, select the right public syntax, and
interpret progress and recovery without reading source or internal commands.
The existing bare/help/-h/--help, human/topics/all and command help output remains
byte-for-byte unchanged, except the explicitly agent-facing `help agent`.
The new focused help is also useful to humans. There is no new top-level verb.

Optimize relevant information per lookup, not terseness alone. A shorter catalogue
that omits human decisions or manual work is worse. JSON is for reliable discovery
and selection; it is not presumed cheaper than text. Preserve plain text defaults.

## Grounded facts

Base main: 0a38b2e01 (the incoming CSS correction does not touch CLI help).
Actual installed baseline: root help 24 lines/168 words; help agent 161 lines/
1,073 words/31 verbs; help review 67 lines/898 words; help all 44 verbs.
`help --json` refuses. Captured outputs, including 53 human help calls, are in
`/Users/wido/LocalStorage/agentic-tools-evidence/agent-help-20260926/`.

- `cmd/metasystem/main.go:781-819` handles help before repository resolution or
  handlers. Keep that read-only boundary and existing family-help compatibility.
- `cmd/metasystem/intent.go:30-66` owns public command and flag descriptors;
  `intent.go:903-1055` renders human help and filters agent help by audience.
- `cmd/metasystem/intent.go:571-629` owns the six public outcomes, JSON envelope
  and optional next.argv/reason. Nonzero can represent unfinished work. A confirmed
  invocation need not mean the entire goal is complete.
- `cmd/metasystem/intent_delivery.go:45-102` owns review syntax and its different
  effects. Diagnostic changes/diff never grant landing authority; review G with
  --changes/--patch commits and publishes the submitted work.
- `cmd/metasystem/ui_describe.go:87-95` projects the same descriptors into the
  Partner catalogue. No second command registry is needed.

## Public interface

`metasystem help [TOPIC|COMMAND [FORM]] [--json]`. The JSON switch may appear
before or after selectors; repeated --json is harmless. No repository is needed.
Unknown options, excess words or unsupported forms refuse with exit 2 before
any handler, with help guidance; JSON requests receive the public refusal envelope.
Existing valid help paths and help aliases retain their current output and exits.
New JSON discovery is public-only: explicit internal/family/compatibility requests
with --json refuse rather than silently widening the public catalogue.

`help agent` becomes a compact guide, followed by all public commands grouped by
intent with one line per verb. Include human-audience commands as discoverable
choices, label audience as a hint, never as permission. Explain:

- Start from the assigned goal; goals --ready/claim serve unassigned work only.
- Use --json for command results; place it before --check, which consumes the rest.
- Distinguish requested invocation success from goal completion. Inspect outcome,
  summary and data even on a nonzero exit: in-progress is not a failed operation.
- Inspect decision as a free-text prerequisite requiring judgment, not executable
  argv. It can ask the agent for missing input or require a human act; resolve only
  what existing authority permits. Never invent human approval or treat every
  decision as human-only.
- Follow next.argv as an argument vector without shell evaluation, preserving
  references, only within existing authority. It is a suggested continuation,
  not permission, a finding disposition or evidence of human approval.
- Wait using the public wait command and retained reference. Do not blindly retry
  mutations after partial/failed/refused results or invent IDs/approvals.
- Use show/status/check to inspect state, focused help for review choices, and
  human help when the next action needs a person. No internal instructions.

Keep this guide materially smaller than the current agent page while retaining
all 44 commands. The acceptance target is at most 75 lines and fewer words than
the observed 1,073-word baseline; these are this change's measurement, not a new
arbitrary permanent parser limit.

## Structured discovery and focused contracts

JSON reuses the existing public result envelope: schemaVersion=1, verb=help,
outcome=confirmed or refused, summary, targets, optional decision/next and data. Do not create an alternate
success protocol. Successful data identifies its topic and contains either a
compact commands index, one command description, or one focused form. Index
entries contain name, group, summary, audience and a help argument vector. Root, agent and all JSON requests return the complete public index. The
agent/root index includes the execution protocol above, with outcome names taken
from the existing constants. Command descriptions project existing usage, visible
options (value label, repeat, rest, advanced, aliases, description), details and
examples. Hidden options and compatibility commands never enter public JSON.
`help goals --json` describes the command, following existing command precedence;
`help all --json` gives the complete compact index. Options and syntax are
descriptive, not a second input validator or an invented complete JSON Schema.

Add eight explicit review help forms: goal, submit, design, job, commit, changes,
diff and finding. `help review` stays unchanged; `help review FORM` gives only:
purpose, exact usage, required inputs, effects, authority conditions, repetition/
retry semantics, relevant visible options, and an executable-shaped example.
`help review --json` adds a forms array (name, purpose, help argv) to the ordinary
command description so a caller
can discover narrow help. The agent guide points to the focused choices.

The mapping is explicit: goal uses current full-help usages 1 and 9; submit 2;
finding 3; design 4; job 5; commit 6; changes 7; diff 8. Diagnostic changes/diff
exclude --changes, --patch, --work, --dispositions, --finding and --test; their
contracts state that a goal before --changes or --patch submits instead. Tests
assert diagnostic argv starts with changes/diff without submission flags, while
submit names a goal and one submission flag. They check actual argument parsing
and shape; existing real-owner review fixtures separately prove effect boundaries.
No classifier or execution router is added merely to test the documentation.

The review descriptor owns the forms. Shared named usage strings feed both its
unchanged full human usage list and the focused forms; neither parses prose to
guess semantics nor copies a second catalogue. Option grammar comes from
the command's existing flags, selected by exact names for each form. A form may
supply a contextual description or value label where the shared flag describes another mode
(for example, the full review page's brief flag says commit review, while the
diagnostic form needs a feedback brief; submission needs --after COMMIT). These
notes never change parsing. The focused
contract includes common --repo/--json and all relevant optional behavior. Scope
and authority are prose conditions, not a false allowed=true field. Form names
are help selectors, never new execution syntax (submit shows review G --changes
or --patch). Examples use placeholders, not live goal IDs or shell substitution.

Keep the shared result protocol in one help owner consumed by text and JSON.
Other commands retain their complete existing command description; do not invent
universal idempotency or mutation metadata. Their future focused forms can use
the same descriptor field when a real confusing choice warrants them.

## Boundaries, failure and non-goals

Help renders compiled descriptors only. No Git, private configuration, current
seat/approval lookup, provider call, file mutation or command execution. Test a
poisoned repository resolver and sentinel handlers. No automatic authorization,
command recommendation engine, natural-language router or dynamic permission
cache. A missing/invalid form cannot fall through into the product handler.
Success and refusal JSON must each be one parseable object with matching exit.
No dependency, execution parser, result contract, old help alias or human workflow
changes. The incoming CSS update is not part of this implementation.

## Proof and review

Root authors and implements; independent Fable critiques design (failsafe round 2)
and code (at most 3 rounds, stop at no material findings). Materiality and the
WORKS/SAFE test follow the two critique skills. Adjudicate every finding. Scope
is this first usable slice; no new generic schema/authorization machinery under
critique pressure. Changed-line estimate: 900-1,400 including focused tests.

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| AH-1 | HIGH | Public interface | Human help stays byte-identical; agent index keeps every public capability with less reading | cmd/metasystem help renderer | `cmd/metasystem/intent_help.go` | `TestIntentAgentHelpCatalogue` and captured original human outputs | Before/after built CLI comparison | DONE | Verified; see plans/agent-help-verification.md |
| AH-2 | HIGH | Structured discovery | Same public descriptors, visible flags and result envelope; no hidden internals or execution | cmd/metasystem help router | `cmd/metasystem/main.go`; `cmd/metasystem/intent_help.go` | `TestIntentHelpJSON`; `TestIntentHelpNeverExecutes` | Built CLI from a directory outside every repository; invalid/unknown input | DONE | Verified; see plans/agent-help-verification.md |
| AH-3 | HIGH | Focused contracts | Review forms distinguish diagnostic feedback from submission and preserve authority/replay conditions | cmd/metasystem review descriptor | `cmd/metasystem/intent_review_help.go` | `TestIntentReviewHelpForms` with actual parser and declared effect boundaries | Focused help and independently selected task commands; plans/agent-help-verification.md | DONE | Verified; see plans/agent-help-verification.md |
| AH-4 | HIGH | Public interface | Agent understands outcomes, argument vectors, waits and conditional authority | cmd/metasystem help protocol | `cmd/metasystem/intent_help.go` | `TestIntentAgentResultProtocol` | Fresh Opus help-only task selection, judged by root, with actual output-size measurements; plans/agent-help-verification.md | DONE | Verified; see plans/agent-help-verification.md |

No claim of statistical model-performance improvement: one bounded fresh-agent
task trial is a smoke test, alongside measured output size and deterministic
contract checks. Select regression checks for the changed help/router surface;
preserve all prior runtime and coverage floors. No full-project rerun for prose.

## Deferred until a demonstrated need

More focused forms build on the review form descriptor, when another command's
choices cause errors. Tool-calling JSON Schema builds on projected flag metadata
only with a real consumer and shared enforcement. Dynamic action availability
belongs to execution owners, never a help-only policy copy. A statistically useful
agent benchmark builds on the first help-only task corpus; it is not this slice's
claim. Context-specific recommendations likewise wait for a measured failure.

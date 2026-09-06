Working Mode: design
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal human-goal-verbs-forgiving)
Date: 2026-09-06

# Goal

Author the one-page design for goal human-goal-verbs-forgiving, slice 1
of the goal record metasystem/plans/goals/human-goal-verbs-forgiving.md;
read that record first, it is the contract. Wido's word of 2026-09-06,
after three refused attempts to give one goal a bigger budget at his
terminal: "this command is too complex, not intuitive and maybe not
forgiving enough". The seat's audit of the whole verb surface is
metasystem/records/misc/verb-surface-audit-2026-09-06.md; this goal is
one member of the arc verbs-match-intent
(metasystem/plans/goals/verbs-match-intent.md) and takes only the human
budget act, the --by default and the refusal rule. DONE for the goal,
from the record: at the enrolled terminal one verb gives a live goal
(queued, parked or claimed) a box, taking the compact form everyone
already writes in prose (1d/10/720m/1/3) or a preset (norm for the
tier's ceiling, keep for the standing box), and does the approval or
revision rebind itself; the five explicit limits stay as the long form;
--by defaults to the enrolled human's recorded name; every refusal of a
human verb prints one complete command that would have succeeded with
the values it saw; a fixture drives each refusal and asserts the printed
command runs. Authority does not widen: every act stays human-only at
the enrolled terminal, every history line stays as it is.

# Workspace

The delegate worktree the dispatcher created for this job. Read
anything; write exactly one NEW file, human-goal-verbs-forgiving-design.md,
in the metasystem plans directory.

# What already exists, and binds

- The flag parsing and the routing by goal state:
  metasystem/cmd/metasystem/goalsync_mutations.go. parseSyncFlags
  declares --by, --budget, the five limit flags and --approved-ref for
  every sync verb; hasAnyBudgetFlag, budgetTuple (all-or-nothing, the
  "complete budget tuple is required" and "budget flags are
  all-or-nothing" refusals) and approvalBudget (the "mutually exclusive"
  and "--budget is box" refusals); trySyncMutation's switch routes open,
  approve, set-budget, resume and the rest, and its need closure prints
  "goal <verb> needs --<flag>"; runGoalSetBudget and
  proveGoalHumanAuthority prove the enrolled terminal (or fixture
  authority, or a temporary human word with --review-by).
- The verb table and its one-line descriptions:
  metasystem/cmd/metasystem/main.go (approve, unapprove, set-budget,
  accept-risk, resume, park, done, set-pin). set-budget is described as
  "replace a claimed goal's approved tuple"; approve carries the budget
  for an unclaimed goal. That split by internal state is the defect.
- The budget grammar: metasystem/internal/goal/budget.go
  (ParseWorkingDuration and FormatWorkingDuration: a day is eight
  working hours; NewBudget; budgetIntentArgs and budgetFromIntentArgs
  are the history line's shape). The dishonesty of that grammar (a
  human's 24h shown as 3d) is goal breach-clock-and-budget-honesty
  (metasystem/plans/goals/breach-clock-and-budget-honesty.md), not this
  one: the compact form must say which grammar it inherits and must not
  redefine it here.
- The norm: metasystem/internal/goal/norm.go (config.TierBox reads the
  tier's ceiling from metasystem.conf; requireWithinGoalNorm and
  goalNormApproval refuse an over-norm tuple unless --approved-ref
  carries the strict quadruple). The preset "norm" is that box.
- The transactions: metasystem/internal/goal/verbs.go (the set-budget
  intent and its claim rebind; approve; resume) and
  metasystem/internal/goal/file.go (which history verbs count as an
  approval, lines near "unapprove" and "set-budget"). The history line
  and the Approved line formats are fixed; the design changes what the
  human types, never what the ledger records.
- The enrolled terminal: metasystem/internal/humanauthority/authority.go.
  The Enrollment record (schema, enrolledAt, generation, terminalId,
  terminalRef, sessionLeaderRef) carries NO human name today, and the
  Proof carries none either. The --by default therefore needs a source
  that the design must settle: a name recorded at enrollment by the
  human act that enrolls the terminal, or an existing recorded fact of
  that terminal; never git config of the checkout (an agent can set it)
  and never a guess.
- The fixtures: metasystem/scripts/agents/goal-cli-fixtures.sh drives
  human verbs under fixture human authority (--by Wido with
  --fixture-human-authority, and the approve with the five limits near
  its middle); the receipt for slice 2 is the go gate plus this suite.
- The flag-name rule of the arc: names follow intent. The --repo flag
  that meant the metasystem directory is goal repo-flag-resolves-one-root
  (metasystem/plans/goals/repo-flag-resolves-one-root.md); do not touch
  it, do not add a flag whose name is an internal noun.

# What the design must settle

1. The one verb and its grammar. Name it (the record says "one verb
   gives a live goal a box"; the audit's human page lists `budget`).
   The compact form: five tokens separated by slashes in the order
   everyone writes (elapsed/attempts/reserved job minutes/active
   jobs/review rounds, e.g. 1d/10/720m/1/3), the unit suffix rules for
   each token, what a missing token means (refuse, or keep that limit
   from the standing box), and the two presets: norm (the tier's box
   from the config) and keep (the standing box unchanged, which is what
   `--budget box` means today). The long form (the five explicit flags)
   stays and means the same thing. Say what happens to `goal approve
   --budget box`, `goal approve` with the five limits, and `goal
   set-budget`: kept as aliases that route to the same path, or
   removed, and what their help lines say.
2. The routing table by goal state: queued (approve with this box),
   parked (approve or resume with this box, per what the ledger
   requires), claimed (set-budget's rebind of the claim to a new
   revision), done or abandoned (refuse, printing the command that
   opens a successor or nothing). One table, every state, every
   outcome; the human never picks the verb by state.
3. Over-norm: when the box exceeds the tier's ceiling the verb refuses
   today unless --approved-ref carries the strict quadruple. Say
   whether the one verb at the enrolled terminal records that word
   itself (the human typing the box IS the word) or still needs the
   flag; keep authority exactly as it is (an enrolled human act is
   what --approved-ref proves today, so folding it must not let a
   temporary word or a fixture widen it).
4. The --by default: its source (see the enrollment fact above), what
   the verb prints when no name is recorded (a refusal that shows the
   enrollment command with a name), and that an explicit --by still
   wins.
5. The refusal rule: one function every human verb's refusal passes
   through, which prints (a) the reason in one plain sentence and (b)
   one complete command line that would have succeeded with the values
   the verb saw, filled in from the goal's state and the standing box;
   where it lives, how a verb hands it the values, and the rule for a
   refusal that has no succeeding command (say so in words). Apply it
   to every refusal in goalsync_mutations.go and name each one.
6. The fixture list for slice 2: one scenario per refusal in the table,
   each asserting that the printed command, run verbatim under fixture
   human authority, succeeds and produces the same history line the
   long form produces; one scenario per goal state for the compact
   form; one for each preset; one for the --by default and one for an
   explicit --by; one for over-norm.
7. Scope for slice 2, bounded: the files that change, in words; the
   things that do not change: the ledger record formats, the history
   verbs, the fixture authority path, the seat's machine verbs.

Write plainly, as the landed designs in metasystem/plans do; no
finding ids, no round references in the design itself.

# Constraints

Wall-clock budget: 40 minutes. One page, about the size of the shorter
landed designs in metasystem/plans, not a treatise. Do not edit
anything but the design file. Gap rule: a fact you cannot find in the
tree is reported as a gap with the question written out, never
guessed.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file
named under Workspace.

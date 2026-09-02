Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal fleet-join-bootstrap)
Date: 2026-09-02

# Goal

Author the design for goal fleet-join-bootstrap: read
metasystem/plans/goals/fleet-join-bootstrap.md first, its Intent is the
evidence base. Wido's open question, verbatim from the dispatching seat's
record: "whether a fresh session can boot itself into the fleet unaided".
Three machines answered no on 2026-09-02 (m1b's fresh host clone, and the
m0 and m0b guest clones before hand-fixing). Write exactly one NEW file,
metasystem/plans/fleet-join-bootstrap-design.md. Every claim about the tree
cites file and line; read the seams before you write about them.

# Workspace

The delegate worktree the dispatcher created for this job. Read anything;
write only the design file.

# What the design must settle

1. THE JOIN SEQUENCE AS ONE OWNER. Today a fresh clone has: no engine
   (bin/ is gitignored; scripts/agents/go-build.sh is the one fenced
   build); no roster (metasystem.conf.local is gitignored: find every key
   a seat needs by reading metasystem/internal/config/resolve.go and
   validate.go, the machine id resolution in metasystem/internal/goal
   (ResolveMachine), evidence.root, the role and mode model keys, the
   cap.min rows in metasystem/internal/dispatch/cap.go); no ledger refs
   (the fetch refspec is +refs/heads/* only while the ledger lives under
   refs/metasystem/*: read metasystem/internal/goal/fetchadvance.go,
   accepted.go, txn.go and how goal verbs publish
   refs/metasystem/machines/<machine>/accepted to origin); no enrollment
   (metasystem up refuses on ENROLLMENT_DRIFT or a missing enrollment,
   metasystem/internal/up/up.go; the relayed-word path is steward arm
   --temporary-human-word in metasystem/cmd/metasystem/steward_verbs.go).
   Decide the owner of the join sequence against the
   existing-owner-before-new-surface rule (metasystem/docs/architecture.md;
   the components of up in internal/up/up.go): an engine verb, an up
   mode, or a script. Specify the sequence step by step with the exact
   refusal each step gives when its precondition is missing and the exact
   next command that refusal names.
2. THE ROSTER TEMPLATE. Specify the committed template a newcomer fills
   (name, location, what is placeholder and what is derived), checked
   against what metasystem config tailor and config validate already do
   (metasystem/cmd/metasystem, metasystem/internal/config). Name the
   minimum a seat must set by hand: machine id, the three model lanes, the
   evidence root, the design-mode lane keys R-25 requires.
3. THE LEDGER REFSPEC. Where the refs/metasystem/* fetch refspec is added
   (clone time, first goal verb, or the join step), and what
   "no accepted tree; the first fetch or the migration bootstraps it"
   (metasystem/internal/goal/project.go line 39) should say and do on a
   machine that has never fetched.
4. THE REMEDY TEXTS. List every message on the join path that names a
   nonexistent or incomplete command, each with file and line and its
   corrected text: at least metasystem/internal/goal/project.go line 73
   ("goal list --fetch validates and advances it": no such flag; find
   what actually advances the accepted tree), and the ENROLLMENT_DRIFT
   remedy in metasystem/internal/up/up.go near line 394, which names only
   the agent-free terminal while R-37-m3 (metasystem/memory/rulings.md)
   authorizes the relayed-word re-arm on every machine. Search for
   others: grep the refusal strings in internal/goal, internal/up,
   internal/steward, cmd/metasystem.
5. THE FIXTURE. A fresh clone of the template against a bare remote joins
   and reaches a green metasystem up, or the honest documented stop where a
   human word is required. Specify the fixture in the style of
   scripts/agents/second-session.sh and the adopt fixtures
   (scripts/adopt-fixtures.sh or its nearest sibling); name what it asserts
   and what it cannot assert in a sandbox (KI-15: the suite needs real
   process visibility).
6. SLICES. Two slices of at most 240 reserved minutes: slice 1 is this
   design's closure; slice 2 builds the owner, the template, the refspec,
   the corrected messages and the fixture. Give estimates grounded in
   recorded precedent from job records under artifacts/agents, or say the
   estimate is unsupported.

Out of scope, name it as such: the harness hook's wrong-root defect
(goal supervision-hook-wrong-root), the steward identity drift
(vm-epoch-identity-drift), and account provenance.

Self-grade per the house rule: confidence, weakest claim, reject condition.

# Constraints

Wall-clock budget: 45 minutes. Design only; no builds, no benchmarks
(R-31). Edit nothing but the design file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file named
under Workspace.

# Gap Rule

stop and report a gap; never fill it silently.

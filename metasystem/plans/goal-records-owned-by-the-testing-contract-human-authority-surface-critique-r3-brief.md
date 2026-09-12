Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal goal-records-owned-by-the-testing-contract)
Date: 2026-09-10

# Review brief: the receipt-repair round of the human-authority surface chain (chain ha-surface1-20260910, reviewing round ha-surface1-20260910-r3)

FINDING IDS: chain-unique, continue at HAS-08, never F-n. You are the
third critic on this chain. Report `round` as 1 in your return: it is
this job's own round. One focused round.

Why: round two carried the reviewed hp-terminal change (its
terminal-grade walk goes to the root of the process tree) with the
human-authority and report-scan surfaces of metasystem/testing.json;
your predecessor ha-surfacecrit2-20260910 found the join exact (HAS-07)
and one Linux-only skip, carried to goal
skipped-test-fails-its-group-on-the-other-os (HAS-05). The landing
receipt on this Mac then refused three groups, and round three folds
exactly those (the fold brief is in the plans directory under this
goal's name, the file ending in surface-fold-r3-brief):

1. governed-standard: nineteen internal/steward tests refused their
   fixture terminal with ANCESTRY_CYCLE because the fake reader in
   internal/steward/ledgerattention_test.go gave pid 1 parent 1. Round
   three gives pid 1 no parent.
2. shell-and-dependency-audits: the proof-grades scenario in
   scripts/agents/goal-cli-fixtures.sh used perl to start its headless
   case in a new session (macOS has no setsid; python3 is confined to
   declared sites). Round three adds one engine verb, proc detach, in
   cmd/metasystem/process_verbs.go (registered in cmd/metasystem/main.go,
   one unit test): run one command as a new session leader with stdin
   from the null device and return its status; the scenario calls it
   in place of perl.
3. section/goal-cli-fixtures: the receipt runs its shell sections with
   the checkout's enrolled engine, whose proc probe predates the
   terminalKnown and sessionLeaderPid fields this change adds, so the
   proof-grades scenario cannot pass inside a receipt until goal
   receipt-beds-run-the-candidate-engine lands. Round three demotes the
   group from delivery the way 7db34a27 did for gate-fence (obligations
   emptied, off the standard list of surface goal-cli-fixture and the
   deep list of surface dispatch-goal-mission, still in cadence);
   approved in Wido's name under his 2026-09-10 delegation; restored by
   goal goal-cli-fixtures-return-to-per-landing-proof.

Scope: the computed diff of round three against round two. Attack, in
order: (a) proc detach: a real new session (Setsid), no controlling
terminal inherited, the child's exit status propagated including
signals, stdin from /dev/null, stdout and stderr passed through, no run
record or reservation, argument parsing that cannot swallow the
command's own flags, and the unit test proving the session and the
status; (b) the fixture now proves TERMINAL_NOT_REACHED through that
verb exactly as it did through perl (compare the two launches); (c) the
steward fixture models pid 1 the way the humanauthority tests' own
fixtures do; (d) the testing.json change is exactly the demotion
described and nothing else (the diff shows ten changed lines: name each);
(e) nothing outside those files changed relative to round two. Do not
re-review rounds one and two: two critics did.

# Mandate

1. The three repairs are exact and minimal; proc detach is sound.
2. Nothing else changed.

If nothing material remains, say so; that closes the chain and it
lands.

# Constraints

Wall-clock budget: 20 minutes. Return per the code-critic schema with
the reviewedTree from the review record beside the computed diff (the
conformance validator refuses from a sandbox; say so).

# Gap Rule

Stop adding at a gap that needs a decision no page has made; keep what
is written; report the gap with the resolution you propose. Never
delete written work.

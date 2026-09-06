Working Mode: design
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal human-goal-verbs-forgiving)
Date: 2026-09-06

# Design critique, round 1: one verb gives a goal a box

FINDING IDS: chain-unique, HGV-C-01, HGV-C-02, ... never F-n.

The design under review is
metasystem/plans/human-goal-verbs-forgiving-design.md, revision 1,
landed on main as commit f989beaf; its SHA-256 is
0746a6b84af842520492d8ff75dcd342ea70360f722dca3956f01ab7aa920a2a.
The "Declared Outputs" digest line the dispatcher stamps into your
prompt is the digest of the outputs manifest, not of the design; do not
stop on that difference. The goal record is
metasystem/plans/goals/human-goal-verbs-forgiving.md and the design
brief the designer worked from is
metasystem/plans/human-goal-verbs-forgiving-design-brief.md. Wido's
word: "this command is too complex, not intuitive and maybe not
forgiving enough"; the DONE line is in the record. The seat's audit of
the verb surface is metasystem/records/misc/verb-surface-audit-2026-09-06.md.

Round budget: the goal has two review rounds and this is the one design
critique the record plans; slice 2 builds from the design as it stands
after one fold. Material only if an implementer working from the
design would build something different or wrong, or if the design
widens authority. Taste is not material.

The designer flagged four choices for the orchestrator; the
orchestrator's rulings, which bind this round:

- approve admits a parked goal (the park stands): accepted as the
  smallest change that makes the DONE line true; test that nothing
  else in the approve transaction moves.
- `--budget box` today means the tier's box, not the standing one; the
  design maps it to the `norm` preset and `keep` is new: accepted.
- the refusal rule covers the human-only verbs in the CLI file and the
  shared refusals they pass through; the mixed and seat verbs are the
  arc's later slices: accepted.
- the page is 272 lines: accepted, no appendix.

# Mandate

1. Authority. Section 3 makes a box typed at the enrolled terminal the
   over-norm word, recorded with the act's operation id as the
   reference. Read metasystem/internal/goal/norm.go and
   metasystem/internal/humanauthority/authority.go: is the branch the
   design adds reachable only by a proof that is a real enrolled
   terminal (not fixture-only, not a temporary word, not a channel
   proof), and does the recorded NormApproval line keep everything a
   later reader needs? Is there any path by which a seat, a fixture or
   a relayed word records an over-norm box without a human at the
   terminal? This is the design's riskiest addition; test it hardest.
2. The routing table (section 2). One outcome per state, every state
   the ledger has (metasystem/internal/goal/approval.go,
   metasystem/internal/goal/verbs.go, metasystem/internal/goal/stop.go):
   is any state missing (abandoned, interrupted, a claim whose machine
   is not this one, a goal whose approval is withdrawn), and is the
   breach-stopped two-act rule the only honest one?
3. The grammar (section 1). The positional box token "found by shape
   wherever it sits": can it be mistaken for a flag value or an id, and
   what does the flag package do with a token after the flags? The
   empty-member rule and the fewer-than-five refusal: is the printed
   command always complete and always runnable?
4. The `--by` default (section 4). The enrollment record gains a
   `human` field; an older engine refuses the new field. Read how the
   record is read and validated: does the fleet cutoff or any mixed
   engine host break on the new field before every seat rebuilds, and
   does the design say what a terminal does in that window?
5. The refusal rule (section 5). Read the refusal sites in
   metasystem/cmd/metasystem/goalsync_mutations.go: does the table
   miss a human-only refusal, and is every printed command one that
   would in fact succeed with the values seen (name the rows where it
   cannot)?
6. The fixture list (section 6) against
   metasystem/scripts/agents/goal-cli-fixtures.sh: can a headless bed
   drive each listed scenario under fixture human authority, and does
   the list prove the printed command runs for every refusal row a bed
   can drive?
7. Scope (section 7) against the goal's budget (4h, 6 attempts, 720
   job-minutes, 2 review rounds): is slice 2 one Sol round with one
   Fable code critique, or does the design need splitting?

Ground every finding in file-and-line evidence read in the worktree.
Plain English throughout.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema; the
declared outputs manifest names the one record you write.

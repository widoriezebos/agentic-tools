Working Mode: design
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal seat-mutual-awareness)
Date: 2026-09-06

# Design critique, round 1: seats see each other and ask each other

FINDING IDS: chain-unique, SMA-C-01, SMA-C-02, ... never F-n.

The design under review is
metasystem/plans/seat-mutual-awareness-design.md, revision 1, landed on
main as commit 6948392b; its SHA-256 is
a18e447192f6cd4506a9e72a7d1a874369a1c7564f2ed0ce7261d23d762aed1d.
The "Declared Outputs" digest line the dispatcher stamps into your
prompt is the digest of the outputs manifest, not of the design; do not
stop on that difference. The goal record is
metasystem/plans/goals/seat-mutual-awareness.md and the brief the
designer worked from is
metasystem/plans/seat-mutual-awareness-design-brief.md. Wido's binding
word (a single-use code on anything inbound from outside; a seat's
words carry no human authority) is not up for redesign; findings that
the design violates it are the most material kind.

Round budget: this goal's box allows three review rounds; treat this as
the first of at most two. Material only if an implementer working from
the design would build something different or wrong.

# Mandate

1. THE TWO DEVIATIONS. The designer deviated from the brief twice and
   asks for a ruling: (a) the records live in a sibling directory of
   the channel directory, not inside it, because the landed channel
   validator refuses unknown paths and unknown keys and an older engine
   would refuse the shared tip; (b) the health role has two ages (30
   minutes seat-facing, 180 minutes before the role goes dead) because
   a dead role opens an alert episode that reaches the human. Verify
   both claims against the landed code with file and line (the channel
   validator in metasystem/internal/goal/channel.go, the alert path in
   metasystem/internal/steward/alert_episode.go), and rule: right,
   wrong, or right for a different reason. If (a) is right, check that
   the new directory is truly invisible to every older engine: the goal
   ledger reader, the pre-commit guard's path regexp
   (metasystem/scripts/agents/pre-commit-guard.sh), the path-class
   manifest (metasystem/scripts/agents/path-classes.txt: a directory
   with no row lands as what class, and does the design add the row it
   needs?), the transport sync, and `goal fetch`'s tip validation.
2. THE AUTHORITY BOUNDARY. Find any path by which a seat-written record
   or a seat's answer reaches a surface that grants authority: goal
   history lines and the approval reader, the channel inbound matcher
   (metasystem/internal/goal/channel_inbox.go), the budget re-approval,
   the alert channel that reaches the human, the narrator digest that
   the human reads. The design claims four structural facts and an
   eight-row refusal table close it; test each fact, and name any path
   it cannot close that the design failed to list.
3. THE WRITER ON THE LEDGER. The steward tick becomes a ledger writer
   (a presence commit per machine per 30 minutes or on change). Check
   the consequences the design must have settled: the push race with
   goal verbs and other machines' ticks (lost-to-winner handling, the
   pushed-blocking refusal, recovery), ledger-attention and narrator
   noise on every other machine, transport sync volume, and what
   happens on a machine whose steward runs an engine without this
   change while others write. The design makes the quiet claim a
   fixture (SMA-F-QUIET); say whether a fixture can prove it or whether
   the design must state the mechanism.
4. ASK AND ANSWER SEMANTICS. Late answers, double answers, not-mine,
   closing by the asker, a target machine that never runs a steward,
   an ask to a machine that does not exist: each must have exactly one
   named outcome. Check the transition table for holes and for a state
   an implementer would have to invent.
5. SURFACING. The health role, the digest lines and `seat wait`: can
   the target seat miss a question (a tick that never runs, a Stop hook
   that only prints the health line when unhealthy), and can the asker
   miss the answer? Name the path.
6. FIXTURES AND BOX. Are the eight scenarios and twelve unit tests
   sufficient to certify the design, and is the three-slice build with
   its estimate honest against a box of one day, six attempts, 240
   job-minutes and three review rounds? Name what is missing.
7. THE TRACED GAP. The designer could not trace which lineage the
   resident steward can read for the presence record. If you can trace
   it (the lease and announcement code under metasystem/internal), say
   where; if not, say the gap stands.

Ground every finding in file-and-line evidence read in the worktree.
Plain English throughout; a person who has not seen this repository
must understand each finding from its words.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema; the
declared outputs manifest names the one record you write.

# Gap Rule

stop and report a gap; never fill it silently.

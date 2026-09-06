Working Mode: design
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal seat-mutual-awareness)
Date: 2026-09-06

# Goal

Author the design for goal seat-mutual-awareness. Read
metasystem/plans/goals/seat-mutual-awareness.md first; it is the
contract. Wido's order of 2026-08-31, from the record: seats must be
aware of each other and ask each other questions directly, without the
human as relay; DONE means a seat can discover what other seats have in
flight and put a question to them as the normal, mechanized path. His
binding design word for the inbound loop, verbatim in the record's Next
step: anything inbound on the external channel carries a single-use TOTP
code verified against a secret provisioned once at his agent-free
terminal; a code authorizes only the message it accompanies; tiering
survives. That word is not up for redesign. This design has to fit
beside it and must never let a seat's message be mistaken for the
human's.

The fleet today: machines m0, m0b, m1, m1b, m1c, m1d, m2, m3 and paper,
each a checkout of one repository, sharing one ledger branch that goal
verbs commit to and push. A seat learns about the others only by
reading goal claims (the `Claimed:` line on a goal record names the
machine and its lineage) and by asking Wido. On 2026-08-31 a seam check
between two seats went through Wido because nothing else existed.

# Workspace

The delegate worktree the dispatcher created for this job. Read
anything; write exactly one NEW file, seat-mutual-awareness-design.md,
in the metasystem plans directory.

# What already exists, and binds

- The channel ledger, and how far it has landed. The design
  metasystem/plans/fleet-channel-gateway-design.md (revision 4, approved)
  defines the directory on the ledger branch that holds question
  records, inbound records and listener heartbeats, with exact schemas
  (section FCG-INBOX-02), a validator with a refusal table, one opid per
  commit carrying the committing machine and lineage, and the rule that
  the git push race is the arbiter. Landed so far: the record types, the
  validator, the transition matrix and the opid helper
  (metasystem/internal/goal/channel.go); the ledger read and the inbound
  match (metasystem/internal/goal/channel_inbox.go); the inbox publish
  and the atomic answer (metasystem/internal/channel/inbox.go). NOT
  landed: the posting protocol and open-work pass as a library (the
  design's build step 3b), the cut-over that moves the verbs onto the
  ledger (step 3c) and the resident listener with its heartbeat
  (step 4). Today `channel ask` still writes a question under the
  machine's own artifacts directory (metasystem/internal/channel/question.go,
  the channelRoot function), invisible to every other machine. The
  design must say exactly which landed pieces it builds on, which
  unlanded gateway steps it depends on (if any), and what it lands
  itself so that seat awareness does not wait for the whole cut-over;
  cite file and line for each.
- Human authority. A question's answer from the human is verified by
  the code and the provider user id on the committing machine
  (FCG-COMMIT-05); the code is checked against the message's own send
  time (metasystem/plans/goals/channel-totp-verified-at-poll-time.md
  records that rule as landed). Human-only acts (approve, set-budget,
  resume, enrollment) happen at the enrolled terminal.
- Health and surfacing. The steward tick evaluates health roles
  (metasystem/internal/steward/health.go) and the Stop hook prints the
  health line and the narrator digest to the seat at every turn end
  (metasystem/internal/steward/narrate.go composes what a seat is doing
  from the goal ledger). The seat's status to the human is composed in
  metasystem/internal/channel/report.go from open questions and
  landings.
- Conduct rules for anything a human reads:
  metasystem/docs/seat-communication.md. The core never names a runtime
  (metasystem/AGENTS.md); decisions live in Go, scripts only relay.

# What the design must settle

1. DISCOVERY. Define the one durable record that says what a machine
   has in flight, who writes it, and when. Decide between (a) a per-
   machine record beside the listener heartbeat in the channel ledger
   (the heartbeat schema in FCG-INBOX-02 is the model: one file per
   machine, overwritten only by its owner, refused when a newer version
   already landed) carrying the claimed goals, the current chain (root
   job, role, round, started), the last landing, the engine digest and
   the update time, written by the steward tick without any seat
   conduct; and (b) deriving all of it from goal claims alone, which is
   free but cannot see a chain. Say which and why; if (a), give the exact
   schema in the table form FCG-INBOX-02 uses, the write cadence, the
   staleness rule, and the validator rows it adds. Define the read verb
   (one verb, name it) that prints, per machine: alive or silent since
   when, claimed goals with their next step's first line, the chain in
   flight, and the questions it has open or owes. This verb is what a
   seat runs before it asks.

2. ASKING A SEAT. A seat-to-seat question is a question record whose
   destination is a machine, not the human's channel. Decide the exact
   shape: a new kind (name it) or a destination field value, and how the
   validator tells it from a human-destined question so that no
   provider ever posts it and no inbound rule ever matches it. The ask
   carries: the asking machine and lineage (from the opid), the target
   machine, the goal it concerns, the facts, the question text, and what
   the asker will do if nobody answers (the no-answer consequence,
   seat-communication rule 2 applies between seats too). Name the verb
   and its flags. It is a ledger transaction under the asker's opid; no
   provider post.

3. ANSWERING. The target seat answers with a ledger transaction under
   its own machine and lineage (name the verb). The answer binds only as
   information: it carries no human authority, so it can never approve,
   budget, resume or enroll anything, and the design says so in one
   sentence the validator enforces (a seat answer on a question whose
   kind is budget-above-norm or stop, or whose destination is the human
   channel, is refused by name). No TOTP is involved: the authentication
   of a seat answer is the committing machine's own identity on the
   ledger, exactly as goal claims are authenticated today. State the
   residual plainly: any process that can push to the ledger as that
   machine can answer as it; the ledger already trusts that for claims.
   Also settle: an answer to a question no longer open is `late` and
   binds nothing (FCG-MATCH-06 has the pattern); a second answer from the
   same machine; a question the target closes as not-mine.

4. SURFACING, MECHANIZED. Both directions arrive without anyone
   polling by hand: the target seat learns of a question owed to it,
   and the asker learns of the answer, through the steward tick and the
   Stop hook's health and digest lines. Specify the health role (name
   it): alive when this machine owes no seat question; unhealthy after
   a bounded age (a config key with a default; propose 30 minutes) while
   a question addressed to it is unanswered, the line naming the asker,
   the goal and the age; and the narrator digest line for a new question
   and for a new answer. Say what `channel wait` does for a machine-
   destined question. The human sees none of this unless a seat's status
   post chooses to mention it; say whether it does (recommend: the
   status post counts owed seat questions in one clause, no more).

5. THE HUMAN CHANNEL STAYS WHAT IT IS. State, with the validator rows
   that enforce it, that a seat record can never be an inbound record,
   that the provider poll never confirms or matches anything a seat
   wrote, and that Wido's inbound rule (TOTP, single use, code bound to
   its message) is untouched by this design. The design critic will try
   to find a path where a seat's words reach the human's authority
   surface; close every one you can see and name the ones you cannot.

6. FIXTURES. Name the fixture scenarios by name, on the existing fake
   provider and a two-clone ledger bed: machine A publishes its
   in-flight record and machine B's discovery verb shows it; A asks B, B's
   health goes unhealthy after the age, B answers, A's wait returns the
   answer and A's digest names it; an answer to a closed question is
   late; a seat answer on a human-destined question is refused; a record
   written by the wrong machine is refused; the validator refuses each
   malformed shape by name. Name the Go unit tests for the new engine
   surface and the coverage floor they respect
   (metasystem/docs/project-rules.md, Local Invariants).

7. BUILD ORDER AND BOX. Slices that each land alone, in order, each
   with its own gate; estimate attempts. The goal's box is one day, six
   attempts, 240 job-minutes, three review rounds; say whether the box
   holds the build or name the raise the seat must ask Wido for.

8. NON-GOALS. No new provider, no second bot, no change to the TOTP
   rule, no authority for seats, no answer archive (goal answer-archive
   owns harvest and rotation), no central brain.

Ground every claim in file-and-line evidence from the worktree per
metasystem/docs/design/design-principles.md; where the design cannot see
a seam, say so rather than guess. Self-grade per the house rule:
confidence, weakest claim, reject condition. Plain English throughout:
a person who has not seen this repository must understand it.

# Constraints

Wall-clock budget: 45 minutes. A design of the size of
metasystem/plans/fleet-slack-channel-design.md, not of the gateway
design. Do not edit anything but the design file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file
named under Workspace.

# Gap Rule

stop and report a gap; never fill it silently.

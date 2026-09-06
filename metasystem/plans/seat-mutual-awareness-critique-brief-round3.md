Working Mode: design
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal seat-mutual-awareness)
Date: 2026-09-06

# Design critique, round 3: seats see each other and ask each other

FINDING IDS: chain-unique, continue the sequence: SMA-C-21, SMA-C-22, ... never F-n.

The design under review is
metasystem/plans/seat-mutual-awareness-design.md, revision 3, landed on
main as commit 278d8b9d; its SHA-256 is 193114134672f292e651ef1694a296b1966d62ba036ba90872ec3c02f89f0c4f.
The "Declared Outputs" digest line the dispatcher stamps into your
prompt is the digest of the outputs manifest, not of the design; do not
stop on that difference. The two earlier registers are
metasystem/records/misc/seat-mutual-awareness-critique-r1.md and
metasystem/records/misc/seat-mutual-awareness-critique-r2.md (ten
material findings each, all accepted); the designer answered round 2
from metasystem/plans/seat-mutual-awareness-fold3-brief.md and the
design ends with two dispositions tables. Wido's binding word (a
single-use code on anything inbound from outside; a seat's words carry
no human authority) stands.

Round budget: this is the ladder's last review. The question is
convergence: would an implementer working from revision 3 build the
right thing? Material only if the answer is no and you name the
artifact; findings that would merely polish are not material.

# Mandate

1. Each round-2 disposition: sufficient, or does it move the problem?
   Rule per finding, briefly; reopen with a new id only where the
   answer fails.
2. The repair verb's exemption (a correcting transaction on top of a
   tip the validator refuses): is the one-path exemption the design
   describes implementable without disturbing goal validation, and is
   the named fallback (a dedicated repair transaction path) sound? This
   is the riskiest new mechanism; test it hardest against
   metasystem/internal/goal/txn.go and metasystem/internal/goal/recover.go.
3. The unforgeable enable marker: what exactly does the validator check
   (authority proven, opid, terminal identity), and can an old landing
   or a seat forge any of it?
4. Expiration ownership and the post-deadline outcomes: exactly one
   outcome per case now?
5. The box: seventeen reservations, 2400 job-minutes, three days,
   twenty attempts, three review rounds; is that count right under
   metasystem/docs/backlog-mechanism.md, and is anything in the
   four-slice order still out of place?

If revision 3 converges, say so with zero material findings; the seat
then asks Wido for the build box on the design's number.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema; the
declared outputs manifest names the one record you write.

# Gap Rule

stop and report a gap; never fill it silently.

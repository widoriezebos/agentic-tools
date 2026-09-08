Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal chain-landing-after-base-move-recertifies)
Date: 2026-09-08

# Design fold: revision 2 of the recertification page, and the last one

Deliverable: revision 2 of
metasystem/plans/chain-landing-after-base-move-recertifies-design.md,
edited in place. Revision 1 was written on this lane an hour ago and is
landed on trunk; read it first. This round folds its one independent
critique and closes the page. No code, no fixtures, no other file.

Your authority to author this design is unchanged and is recorded in
metasystem/memory/rulings.md as R-87-m1c and R-86-m1b: design authoring
uses codex on gpt-6-astra at xhigh, and Wido restated it today. What
stays the orchestrator's: dispositions, certification, the ledger, the
receipt.

The critique is job clbm-crit1 and its disposition is
metasystem/records/misc/chain-landing-after-base-move-recertifies-critique-r1.md.
Five material findings, all accepted. Two notes are recorded and not
actioned and you should leave them alone: a textually disjoint but
semantically coupled pair passing is the residual risk the goal
knowingly accepts, and the canaries being unreachable today is what
"specified, not written" means.

The standard the critique applied, and the one to hold yourself to in
this fold: every statement must leave two implementers building the same
thing.

## CLB-01, high - the unattended node needs one named, durable move

The page says an unattended node uses an existing stream-park mechanism.
No such mechanism exists: a literal search finds the term only in your
own page. An implementer must guess between `goal park`, mission
parking, a scheduler record and a diagnostic exit, and those differ
materially. `goal park` is a ledger transition that records a reason and
clears claim binding (metasystem/internal/goal/verbs.go:1204-1270).
Mission parking is unavailable to this mission-free chain.
metasystem/scripts/agents/land.sh has no parking transition at all: it
reports a failed step and exits, which a reader cannot distinguish from
a stall.

Name exactly one owner and specify it: the invocation, the record it
persists, the authority precondition, what a reader sees that
distinguishes a park from a stall, and what happens when recording the
park itself fails. If clearing the claim is wrong for this case, say so
and name what is used instead.

## CLB-02, high - the content rule must exclude what the page says it excludes

The page says binary blobs are unsupported, then defines the admission
rule as regular modes plus the absence of a NUL byte, expressly allows
bytes that are not valid UTF-8, and forces Git through `--text`, which
treats every file as text. A both-sided blob of high-bit bytes with
newline separators and no zero byte satisfies every stated check and
would be admitted.

Close the gap so the rule matches the claim, and make the non-text
canary use the same mechanically defined test the production path uses,
so the two cannot drift.

## CLB-03, high - area-width recertification needs a defined command

The page says execution evidence remains necessary and defers to "the
same required checks for that chain's gate width".
metasystem/docs/orchestration.md:76-78 fixes only the full-width
battery, and metasystem/internal/landing/receipt.go:55-57 accepts any
non-empty command, with lines 290-310 validating success and tree
posture without identifying an area check. So one implementation can
accept `true` and another can try to recover the original chain's
focused checks, and those are different safety contracts.

Define how production selects and binds the command for an area-width
chain. If the honest answer is that an area-width chain must supply its
command explicitly, or that recertification requires full width, say
that plainly rather than leaving it to the implementer.

## CLB-04, medium - the diff grammar must match the tool's real output

With the exact option set your page specifies, Git emits hunk headers
that carry a trailing section suffix. Your grammar shows only the
unsuffixed shape and your page makes malformed output a refusal, so a
strict parser would reject ordinary middle-of-file insertions, which
the DONE case includes. The page also relies on Git's no-final-newline
marker without defining how the parser treats it.

State that the optional section suffix is permitted and excluded from
the coordinates, and define the parser's treatment of the
no-final-newline marker. Write this against what the tool actually
prints; the critic proved both by running Git rather than by reading
about it, and so should you.

## CLB-05, high - the refusal canaries must fail for the right reason

Your candidate-tamper subtest is properly isolated with an otherwise
valid receipt. The changed-target and changed-proof checks require only
"not bar a", which many wrong implementations also satisfy: one that
falls back to the old chain path, one that rejects a stale receipt, one
that checks only the record's self-hash. A byte-flipped payload tests
corruption, not a forged record whose author recomputed the input and
record digests.

Rewrite both observations so each names the refusal reason it requires
and keeps every unrelated prerequisite valid, so the only thing that can
produce the refusal is the defence under test.

This is the third design in this program whose refusal canary was too
weak, so treat the rule as general and apply it to every refusal
observation on the page, not only to these two.

## Scope

Fold these five and nothing else. Do not revisit the decision itself:
the mechanical no-overlap proof stands, and there is still no
merge-only critic lane. Do not restructure the page, do not add a
mechanism the goal does not ask for, and keep every canary's ceiling.

Mark the page revision 2 and say at the top which findings it folded.

This is the last design round for this page. After it the build proceeds
behind the fixtures the page names, with no second critique. If you
believe something cannot be settled without a further revision, say so
plainly in the return and stop; that decision goes to Wido, not to a
third revision.

Gap rule: stop and report a gap; never fill it silently.

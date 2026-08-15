# Delegate delivery: every channel a return can arrive by, and the guards on each

Working Mode: design

Owner: main session (delegate), under Wido's 2026-08-15 morning ruling
("prioritize this design, get it designed, critiqued in a loop and
then implemented before we do anything else"). Status: r1, critique
in flight; r2 will ALSO fold Wido's boundary ruling (same morning, in
writing): everything Go is better suited for goes in Go — which moves
the ladder walk itself out of the shell. r2's shape: ONE collect verb
(`adapter devin-collect`) owns rung ordering, fall-through, every
guard, the reply-source decision, and writes the provenance itself,
emitting a structured verdict (delivered | needs-repair | empty);
devin.sh keeps process lifecycle only — spawn, wait, custody, and on
needs-repair one repair CLI invocation followed by a second
devin-collect over the repair's outputs. The r1 text below still
shows the walk in shell; r2 supersedes that section.

## The problem, with two cohorts of evidence

A Devin delegate's reply is whatever `devin -p` prints — stdout is
the only delivery channel the adapter reads. Two cohorts proved
swe-1-7 does not reliably use it: the model finishes work by WRITING
FILES, not by emitting a final message. In cohort
bm-2-20260814t213312z-37844 (graded permissions) the final write was
confirmation-blocked and the session died undelivered (D57). In
cohort bm-2-20260815t062523z-18265 (dangerous mode, D61) the write
SUCCEEDED — a schema-perfect, self-identifying return landed in
/tmp/design-critique-return.json — and stdout was still empty (D62).
One model bias, two failure shapes, five delegate jobs' work lost
while sitting intact in evidence we already held.

D62 shipped the first correction: the augmented prompt names ONE
exact return path inside the round evidence and the collect step
reads it when stdout is empty. This design owns the full mechanism —
because a named path is a request, and a request the model can
ignore is not robustness (the human's exact objection).

## The delivery ladder

When the CLI process exits (the trigger the adapter already owns:
custody-registered pid, F4 deadline custodian for hangs — nothing
here changes process supervision), the collect step walks channels
in order and takes the FIRST that yields qualifying bytes:

1. **stdout** (`raw.out`) — the CLI's native reply channel,
   unchanged.
2. **The named return file** (`<round>/devin-return.json`) — shipped
   in D62. The prompt's Delivery section names it; the model is asked
   to write it AND print.
3. **Transcript mining** — the exported ATIF transcript records
   every tool call with its full arguments, including complete file
   contents for `write` calls. A new engine verb, `adapter
   devin-recover --transcript F --job ID`, returns the LAST write
   whose content parses as a JSON object carrying `"jobId": <ID>`.
   Exit 0 with the bytes; exit 3 when nothing qualifies; exit 1 on
   unreadable/unparseable transcript. This rung costs nothing at
   runtime and recovers the D62 incident's exact shape (the /tmp
   write is in the transcript verbatim).
4. **The same-session delivery repair** (the human's rung) — the
   adapter already owns a bounded one-shot repair turn that resumes
   the session (`-r <session>`); today it fires only for MALFORMED
   returns. It extends to the EMPTY case: when rungs 1-3 yield
   nothing and the session correlated, the repair resumes it with a
   delivery-specific prompt — "your session completed work but no
   return was delivered; write the ONE JSON return object to <named
   path> now, and also print it" — then re-walks rungs 1-3 on the
   repair's output. The existing bounds hold verbatim: ONE repair
   per round, recorded in returnRepairs, never inventing content
   (the session's own context produces the return or nothing does).
   A turn with no correlated session cannot repair and adjudicates
   empty exactly as today.

Rungs are per-round and independent per attempt: the repair's output
gets rungs 1-3, never a second repair.

## The guards, uniform across rungs

- **Parseable JSON object** — a torn or partial write is never
  promoted into a reply (rung 2 checks via `util json-validate`,
  rung 3 inside the verb).
- **Self-identification** — rung 3 requires `jobId` equality with
  the record; rungs 1-2 inherit the same property one step later
  (below). Recovery accepts only a return that names this exact job;
  it never guesses which file the model "meant".
- **Full schema validation stays where it lives** — the existing
  return pipeline (`normalize_return` → `validate return-complete`,
  which already checks job identity) validates whatever the ladder
  hands it. The ladder's job is candidate selection, not validation
  authority; there is exactly one validator and it is unchanged.
- **Provenance is stamped, never inferred**: the collect step writes
  `<round>/reply-source` containing one token — `stdout`,
  `named-file`, `transcript`, or `repair` — and logs the same line.
  Graders and scorecards can always see which channel delivered;
  nothing is silently promoted.

## What stays shell and what becomes engine

Per the core-vs-plumbing ruling: the LADDER WALK (try, fall through,
log) is choreography and stays in devin.sh, ~15 lines. Every DECISION
is an engine verb: `adapter devin-recover` (rung 3, new),
`util json-validate` (rung 2's guard, existing),
`validate return-complete` (the one validator, existing), and
`adapter devin-prompt --return-file` (the named channel, shipped).
The repair prompt text for the empty case ships as a heredoc in
devin.sh beside the existing malformed-repair prompt.

## Runtime scope, stated plainly

This mechanism is Devin-only today, by need: Claude and Codex have
CLI-enforced structured output (`--output-schema` and its codex
equivalent), so their runtime pins the reply into the channel the
adapter reads — this failure class cannot occur there, and adding
ladders they cannot exercise would be dead machinery. The GENERIC
form — delivery channels declared per adapter, one engine-owned
walker — is real but belongs to the ACP transport design (backlog
item 18), which replaces the transport outright and adds per-call
permission answering; the ladder survives that migration as the
validation layer over typed events. Items 16 and 18 own the
generalization; building the abstraction now for one consumer would
be speculation.

## What this deliberately does not do

- No ACP client (item 18's design loop owns it; the CLI's `devin
  acp` mode is verified present and waiting).
- No workspace file sweeps — racy, guessy, dominated by the
  transcript rung.
- No streaming — a live-events concern (item 15/18), not a delivery
  contract.
- No change to claude/codex adapters, to process supervision, or to
  the return validator.
- No second repair, no widened repair authority — the existing
  bound is the bound.

## Blast radius

- internal/adapter: `DevinRecoverReturn` (new, ~40 lines) beside the
  other devin decisions; unit tests.
- cmd/metasystem: the `adapter devin-recover` verb row and flag
  parser.
- scripts/agents/adapters/devin.sh: the ladder walk in the collect
  step, the empty-case repair prompt, the reply-source stamp.
- Fixtures: the devin selftest gains ladder legs (below); no other
  suite surface changes.

## Proof obligations

- Unit (Go): last-matching-write wins; wrong-jobId writes never
  recover; non-JSON content never recovers; exec calls are not
  writes; absent/unparseable transcript errors distinctly (exit 3 vs
  1 at the verb).
- Ladder (fixture, fake-runtime staging of the devin collect path):
  stdout wins when present; named file recovers when stdout empty;
  transcript recovers when both empty; the repair fires ONLY when
  all three miss and a session correlated; a torn named file falls
  through to the transcript rather than failing the turn;
  reply-source carries the correct token on every path.
- Repair bounds: exactly one delivery repair; its output re-walks
  rungs 1-3; no session → adjudicate empty as today (the D24 stage
  decision unchanged).
- Regression: the D62 incident replayed from its frozen transcript
  recovers the /tmp return via rung 3 with jobId verified.
- Provenance: the kit's extractor tolerates the reply-source file's
  presence (additive evidence; no schema change).

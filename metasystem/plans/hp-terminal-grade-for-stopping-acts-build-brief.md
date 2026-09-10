Working Mode: build
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-09

# Goal

Goal hp-terminal-grade-for-stopping-acts, the first member of the split
of human-proof-fits-the-act. Its record is
metasystem/plans/goals/hp-terminal-grade-for-stopping-acts.md; the shared
design is metasystem/plans/human-proof-fits-the-act-design.md, revision
2, and this member builds its section "The two proofs" and the stopping
rows of section 1, nothing else. In short: today every human act
demands the enrolled-terminal walk, so a human away from the one
enrolled tab cannot even stop, park or release work, and a dead
enrolled terminal wedges the seat. DONE means: a human at any live
terminal of the host, with no agent in the ancestry, can session-stop,
park, release and unpark-to-queued without an enrollment; acts that
grant or widen authority keep the enrolled walk unchanged; an agent
passes neither grade anywhere.

# Where it lives (traced)

- metasystem/internal/humanauthority/authority.go: Prove (line 611),
  Enroll (line 567), the Proof type and its Valid and ValidFor (lines
  99 to 145), the outcome constants (lines 27 to 40).
- metasystem/cmd/metasystem/goalsync_mutations.go: syncReqWithProof
  (line 60 onward) builds every goal verb's request and proves human
  authority only when the lineage is absent; the sync verbs park,
  release, unpark and session stop reach the engine through it.
- metasystem/internal/goal/verbs.go: VerbRequest (line 190) and the
  Actor.Human tests inside park, release and unpark; the design's
  inventory names each.
- metasystem/internal/goal/sessionstop.go: the session-stop
  authorization path.
- metasystem/internal/refusal/register.go lines 33 to 45: the
  authority layer's codes. This member adds one.
- metasystem/scripts/agents/goal-cli-fixtures.sh: the bed that drives
  the goal verbs; metasystem/internal/humanauthority/authority_test.go:
  the tree reader that models ancestry (treeReader) for fixtures.

# The change

1. ProveTerminal(root, invokerPID, reader, now) in the humanauthority
   package: exactly what Enroll performs before it writes (stable read
   of the invoker, a controlling terminal required, the session leader
   read, the walk to it reading every node twice, any adapter signature
   refused, a node whose terminal differs refused), reading no
   enrollment, returning a Proof with the walked nodes, TerminalRef the
   session leader, TerminalGeneration zero, Outcome
   HUMAN_AUTHORITY_PROVEN and a new field Grade set to "terminal". Enroll
   becomes ProveTerminal plus the write, so the two never drift. Prove's
   successful proof carries Grade "enrolled". Grade is empty on every
   refused proof.
2. Prove's no-enrollment branch: when ReadEnrollment fails, run the
   terminal walk first; a refusal of that walk is returned as is; a walk
   that reaches the session leader returns Outcome TERMINAL_NOT_ENROLLED
   (a new constant and one new row in the refusal register), the walked
   nodes recorded, Valid false. The same outcome carries an unreadable
   or incomplete record, with the read error in the text.
3. Two predicates: ValidFor keeps its meaning (enrolled grade only);
   TerminalValidFor accepts either grade. FixtureGoalProof carries the
   enrolled grade. Proof records written before this change carry no
   grade and read as enrolled.
4. The proof travels on the request: VerbRequest gains Authority
   *humanauthority.Proof, set by the builder. One helper,
   (VerbRequest).requireHuman(row, grade), refuses when --by is absent
   (the row keeps its sentence), when --by arrived with no proof (a
   plain sentence, no code token), and when the proof's grade is below
   the row's, with a typed GradeRefused{Verb, Row, Needed, Got}. In THIS
   member the helper replaces the Actor.Human tests of park, release,
   unpark-to-queued and session stop only, all at the terminal grade;
   every other row is untouched and keeps its full walk (the sibling
   member hp-every-by-proves-a-human sweeps them).
5. The builder proves high first, then low, for the four rows above:
   when --by is present, call Prove; on HUMAN_AUTHORITY_PROVEN carry the
   enrolled proof; on TERMINAL_NOT_REACHED or TERMINAL_NOT_ENROLLED call
   ProveTerminal and carry the terminal proof when it passes, keeping
   the full walk's refusal on the request for the renderer; any other
   Prove outcome is rendered at once. The lineage derives from the
   proof when none was supplied: terminal-<id>-0 for a terminal-grade
   proof. For every other verb the builder's behaviour is unchanged in
   this member.
6. Tests and fixtures: in authority_test.go, through the tree reader, a
   human shell at an unenrolled terminal passes ProveTerminal and Prove
   returns TERMINAL_NOT_ENROLLED; an agent signature anywhere refuses
   both walks; a headless caller refuses both. In the goal package, a
   request with Actor.Human set and no Authority is refused by
   requireHuman on each of the four rows. In goal-cli-fixtures.sh: a
   human shell at an unenrolled terminal parks, releases and unparks a
   fixture goal and session-stops, and is refused for approve with the
   enrolled row named; an agent shell is refused for all of them. Name
   the smallest run that proves each.

# Not in scope

Every row graded enrolled; the refusal texts (member
hp-refusals-print-the-one-command); the builder for verbs other than
the four (member hp-every-by-proves-a-human); the relayed word; the
matrix. Do not touch the fixture authority grant.

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l . (empty)`;
`go test ./internal/humanauthority/ ./internal/goal/ ./cmd/metasystem/ -count=1`
green; `bash scripts/agents/goal-cli-fixtures.sh` green (say so if the
sandbox cannot run it; the orchestrator replays it outside). Coverage on
internal/humanauthority and internal/goal must not drop below the
tree's figures (measure before and after; internal/goal was 82.4
percent on 2026-09-09).

# Constraints

Wall-clock budget: 90 minutes; return before it ends even if something
is red, naming it. DESIGN-BEARING reach (the authority boundary moves):
xhigh. Declare the boundary as every file that differs from main. Stop
adding at a gap that needs a decision no page has made; keep what is
built and green; report the gap with the resolution you propose. Never
delete built work. Plain English in comments and messages; no finding
or round references in source comments.

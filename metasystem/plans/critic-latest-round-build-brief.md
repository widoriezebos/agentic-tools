Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal critic-reviews-root-hides-latest-round)
Date: 2026-09-06

# Goal

Goal critic-reviews-root-hides-latest-round (tier 2, MECHANICAL, approved
by Wido on 2026-09-06). Its record,
metasystem/plans/goals/critic-reviews-root-hides-latest-round.md, is the
contract. In short: a code-critic dispatched with `--reviews` naming a
chain ROOT whose latest implementer round is a follow-up reviews the
latest tree, but the critique register reads the root's own round's
diff and refuses "reviewed implementer round has no diff.patch" although
conformance ran for the latest round. The record prefers the door
refusal (fail before a critic is spent), a refusal that names the round
and the file, and a follow-up door message that names the verb.

# Facts (read on 2026-09-06)

- metasystem/scripts/agents/dispatch.sh lines 1311 to 1315: a
  code-critic or warden dispatch requires `--reviews`, a valid id, an
  existing record, and role implementer. Nothing checks that the named
  job is the chain's latest implementer round. The follow-up path (line
  1892) copies `reviews` from the latest critic record into the child
  and passes it to claim-launch (lines 2080 and 2112).
- metasystem/internal/dispatch/claim.go: validateClaimReviews (line 738)
  checks only role and id shape; ClaimLaunch has the repository root and
  can read job records.
- metasystem/internal/dispatch/finding_register.go,
  critiqueSubjectForRound (from line 727): reads root["reviews"], takes
  THAT job's own `round`, and reads
  artifacts/agents/<chain root>/rounds/<round>/diff.patch; on a miss it
  returns "reviewed implementer round has no diff.patch; run conformance
  --stage review first" without naming the job, the round or the path.
- metasystem/internal/dispatch/critique.go line 159: the follow-up door
  refuses "advance the canonical register before reading exhaustion"
  without naming the verb.
- Today's specimen: chain enroll-login-build1 gained round
  enroll-login-build1-r2; a fresh critic named the round job and worked;
  the earlier critic chain's follow-up inherited `reviews` = the root and
  resolved round 1's diff, which is the follow-up shape of the same
  lookup. The closure law (metasystem/internal/dispatch/hazard.go)
  requires a FRESH critic over the final round anyway, so a follow-up
  critic round exists to resolve its own register, not to certify the
  final tree.

# Decisions (the orchestrator's; decided, not open)

D1. The door, in Go at claim: for a FRESH dispatch of a code-critic or
warden, `reviews` must be the latest implementer round of its chain.
Add the check beside validateClaimReviews in ClaimLaunch's fresh path,
reading the reviewed job's chain from the jobs directory (chainMembers
exists in this package): if any implementer member of that chain has a
higher round than the named job, refuse with the message "code-critic
dispatch --reviews names <named job> (round N), but that chain's latest
implementer round is <latest job> (round M); name the round job". A
verifier dispatch keeps today's rules. The dispatcher script needs no
new check of its own; its die on the claim refusal must surface that
message verbatim (confirm it does; fix only if it swallows it).

D2. Follow-up dispatches are exempt: a critic follow-up inherits its
root's `reviews` by design and the register resolves it against that
same binding. Do not refuse or rewrite the inherited value. Say so in a
comment stating the invariant (a follow-up resolves the register of the
round it continues; certification of a later work round is a fresh
critic's job).

D3. The register's refusal names what it looked for: in
critiqueSubjectForRound, the missing-diff error becomes "reviewed
implementer round <job> (round N) has no diff.patch at <repo-relative
path>; run validate conformance --stage review --job <job> first", and
the empty-paths error names the same job and path.

D4. The follow-up door's message names the verb: critique.go's refusal
becomes "critic chain <root> has folded through round F but its latest
record is round L; run job critique-register-advance --repo <repo>
--root-job <root> --round-job <latest round job> first".

D5. Pins. In metasystem/internal/dispatch, using the package's existing
claim and record helpers (claimParamsForTest, writeJSONFile, ClaimLaunch
as in review_reference_test.go): a fresh code-critic claim naming a root
whose chain has an r2 refuses with the D1 text naming both jobs and
rounds; naming the r2 job succeeds; a follow-up claim inheriting the root
succeeds; a warden behaves like the code-critic; a verifier naming the
root still succeeds. For D3 and D4, tests that assert the new texts
(the register test file has helpers; critique.go's refusal has a test
neighbour to extend). If metasystem/scripts/agents/dispatch-fixtures.sh
has a chain with a follow-up round that a code-critic scenario can name,
add the two-line assertion (root refused naming the round job; round job
accepted); if it does not, say so rather than building one.

D6. Non-goals: no change to what the register resolves for an inherited
binding; no change to hazard.go; no change to the verifier or
design-critic doors; nothing under plans.

# Gate

From the metasystem directory: `gofmt -l .` prints nothing; `go vet
./...`; `go build ./...`; `go test ./internal/dispatch/ -count=1`;
`bash -n scripts/agents/dispatch.sh`. Report each with its evidence
level.

# Constraints

Wall-clock budget: 40 minutes. MECHANICAL reach (tier 2): the
orchestrator's gate is the examination. Declare the boundary as every
file that differs from main. Gap rule: stop and report a gap with your
proposed contract written out; never fill it silently.

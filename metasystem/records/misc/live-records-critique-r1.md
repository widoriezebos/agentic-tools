# Live-records design critique — round 1 (Sol)

Chain: design implementer-050218716497f94c7b2fdb47 (Fable lane) ->
critic design-critic-3997bb5e8447a36d45cba182 (codex gpt-5.6-sol,
xhigh, fresh context), 2026-09-02. Nine material findings, four
critical: the append-only carriage premise is advisory not enforced;
the fifteen post-driver manual conflicts are unexplained by the
design's solved claim; the carry commit breaks the caller's staging
contract in pathspec mode; the carry-to-guard race stays open; a
concurrent rebase can destroy live bytes; adoption does not confer
enforcement; land.sh has NO landing-wide mutex; a crash after
re-staging strands staged bytes; and the proof plan stubs out the
mechanisms most likely to fail. Revision 2 folds each by id.

## LR-001 — critical, material=True

CLAIM: LR-001, the advisory-carriage premise: the design relies on an append-only carriage refusal that is not enforced. The union attribute is shipped, but the append-only result is only written into a commit trailer; neither the commit nor push path rejects a would-refuse result. A rewritten digest can therefore cross the boundary the design calls safe, so implementing only the proposed local carry would leave the cross-machine safety premise false.

EVIDENCE: metasystem/internal/landing/observe.go lines 467–469 classify digest rewrites, but metasystem/scripts/agents/commit.sh lines 286–311 explicitly keep policy mismatches non-blocking in observation mode. metasystem/scripts/agents/land.sh lines 273–313 pushes without checking that verdict, and the tracked scripts contain no pre-push guard. The reviewed commit itself reached origin with Landing-Provenance-Verdict set to would-refuse, demonstrating that this is observation rather than refusal.

## LR-002 — high, material=True

CLAIM: LR-002, the unexplained post-landing conflict trail: the design declares the cross-machine append race solved while also citing fifteen manual digest resolutions after the union attribute landed. The repository proves that the attribute is present and that twenty-seven later commits carried digest changes, but it contains no event-level evidence reconciling those resolutions as an adoption gap, an attribute gap, a different conflict class, or a mistaken count. Because the design uses this claim to exclude cross-machine work, an implementer would leave an observed failure class untouched.

EVIDENCE: metasystem/.gitattributes line 2 has applied merge=union to the digest since commit 95e0a644. metasystem/plans/live-records-design-brief.md lines 19–20 and metasystem/plans/live-records-landing-design.md lines 35–48 assert the later manual collisions. Searches of subsequent commit bodies found only aggregate assertions, not individual rebase commands, conflict outputs, or resolutions.

## LR-003 — critical, material=True

CLAIM: LR-003, the carry commit cannot preserve the caller's staging contract: in pathspec mode the proposed pre-staging carry invokes the real commit gate while the caller's ordinary changes are still unstaged, so the gate refuses them as unbound. In staged-only mode the caller's changes are already in the shared index, so staging the live record and running commit.sh commits the caller's changes inside the carry commit, leaving the intended caller commit empty. The central mechanism therefore works in neither supported landing mode without a different index-isolation design.

EVIDENCE: metasystem/scripts/agents/land.sh lines 201–242 leaves pathspec changes unstaged until stage_changes and permits a populated index in staged-only mode. metasystem/scripts/agents/commit.sh lines 173–193 refuses projected unstaged bytes, while lines 312–341 prove and commit the entire index. The design's call site at metasystem/plans/live-records-landing-design.md lines 185–189 runs carry before stage_changes and specifies no alternate index, temporary worktree, or preservation protocol.

## LR-004 — critical, material=True

CLAIM: LR-004, the carry-to-guard race remains open: finishing a bounded carry loop does not prevent the writer from appending immediately after the loop returns and before stage_changes, require_clean_after_commit, or git rebase performs its clean-tree check. One append in that gap produces the same refusal as today and bypasses the five-attempt diagnostic entirely. The design narrows the race window but does not solve the local race it claims to settle.

EVIDENCE: metasystem/plans/live-records-landing-design.md lines 166–173 bounds retries only inside the carry verb, while lines 182–194 place separate calls before existing guards. metasystem/scripts/agents/land.sh executes each guard or rebase as a later command. The writer's private lock in metasystem/internal/narratordigest/digest.go is not held across either transition.

## LR-005 — critical, material=True

CLAIM: LR-005, concurrent rebase can destroy bytes: when two machines changed the live path, git rebase writes a union result to the digest while the local writer may independently read, append, and atomically replace that same pathname. Atomic rename prevents torn writer output but provides no ordering against Git. Either operation can overwrite the other's later bytes, violating the no-dropped-bytes law before a subsequent carry can observe the loss.

EVIDENCE: metasystem/internal/narratordigest/digest.go lines 67–97 and 110–194 show that only digest writers share the flock. metasystem/scripts/agents/land.sh lines 269–311 runs Git rebase without that lock. The design's interleaving table covers a writer during git add and separate cross-machine landings, but never their composition with the writer active while rebase mutates the live path.

## LR-006 — high, material=True

CLAIM: LR-006, three-line adoption does not confer generic append-only enforcement: the existing carriage evaluator applies appendOnly only to hard-coded paths, including the digest. A newly registered live path merely passes the allowlist check. If its writer replaces the file between the carry's prefix comparison and git add, the staged rewrite can pass carriage because the new path is not in the hard-coded switch. A new role therefore cannot obtain the promised byte law through three declarative lines alone.

EVIDENCE: metasystem/internal/landing/observe.go lines 458–471 invokes appendOnly only for memory/receipts.log and records/narrator-digest.log, with a distinct rule for memory/rulings.md; all other allowed paths receive no shape check. The proposed validation at metasystem/plans/live-records-landing-design.md lines 208–213 checks only union attribution and allowlist coverage, not that generic append-only enforcement applies.

## LR-007 — high, material=True

CLAIM: LR-007, the assumed one-landing-per-checkout serialization is not present: land.sh does not acquire a landing-wide mutex. commit.sh's lease wrapper covers the commit subprocess, not the preceding and repeated index mutations, and human-classified calls do not take that lock. Two carry-enabled landings in one checkout can therefore stage into or commit from each other's shared index. Calling this only an inherited hazard makes an implementer omit required serialization for the new index operations.

EVIDENCE: metasystem/scripts/agents/land.sh lines 1–120 contain no lease acquisition and lines 223–252 mutate the index before invoking commit.sh. metasystem/scripts/agents/commit.sh acquires its lease only inside that subprocess. The design nevertheless states at metasystem/plans/live-records-landing-design.md lines 175–178 that one checkout runs one landing at a time and uses that assertion to add no concurrency control.

## LR-008 — high, material=True

CLAIM: LR-008, a crash after re-staging is not self-recovering ordinary commit state: termination after git add but before the carry commit leaves live-record bytes staged. The next pathspec landing refuses the non-empty index before carry runs, while staged-only mode risks combining those bytes with the caller's selection. The design's claim that the next landing simply rebases and pushes the carry applies only after a carry commit completed and omits the requested re-stage-to-push crash window.

EVIDENCE: metasystem/scripts/agents/land.sh lines 212–219 refuses a non-empty index in pathspec mode before the proposed first carry call. The design stages before committing at metasystem/plans/live-records-landing-design.md lines 159–169, but its crash table at lines 255–257 discusses only completed carry commits and defines no restart marker, index rollback, or idempotent recovery rule.

## LR-009 — high, material=True

CLAIM: LR-009, the proof plan removes the mechanisms most likely to fail: the migration extends a fixture that replaces the real commit wrapper with plain git commit, so it cannot detect the index incompatibility, append-only observation behavior, or static re-proof race. Its byte property covers only two local legs and omits the same-path two-machine rebase, a writer during rebase, both staging modes, and a crash after staging. An implementation could satisfy every named fixture while violating the required interleavings.

EVIDENCE: metasystem/scripts/agents/land-fixtures.sh lines 4–7 explicitly reduce the commit wrapper, and lines 46–61 implement the stub as git commit. The planned fixtures in metasystem/plans/live-records-landing-design.md lines 269–291 assert byte preservation only for the dirty-before and mid-commit local legs and name no cross-machine or crash-restart specimen.

## Critic-declared gaps (verbatim)

- The repository does not retain individual transcripts for the asserted fifteen post-95e0a644 manual digest conflicts. The aggregate claim is insufficient to classify them as an adoption gap, an attribute gap, or a wrong claim, so that classification remains open rather than inferred.
- The runtime was read-only, so no temporary repositories could be created to execute the missing two-machine and crash interleavings. The concurrency findings are grounded in the shipped control flow and filesystem ownership, not a live mutation test.

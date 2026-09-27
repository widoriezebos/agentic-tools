# Sol's read of the g1-s64 build

Produced 2026-09-27 by Codex on `gpt-6-sol`, read-only, against `git diff ui-development...ui/g1-s64` at `4c7aa8563` (six commits, rebased onto `86bbe90b5`) and the design's revision 2, under R-124: one fix round, then landing. Verbatim; the fix round's disposition is in the design's Built section.

---

## Finding

**1. S64-C-01 — High — The Abandon card omits the goal's full intent.** In `argumentsOf` (metasystem/internal/ui/web/_app/src/partner/proposing.ts:264), the `abandon-goal` case displays the reason, successor, and dependents but never `line.read.intent`. `ProposalSubstance` (metasystem/internal/ui/web/_app/src/partner/Proposal.tsx:227) displays only the short title, which `titleFrom` (metasystem/internal/ui/partner/proposals.go:379) cuts at the first sentence. The frontend test (metasystem/internal/ui/web/_app/src/partner/proposing.test.ts:275) expects this omission, and the waiting-card screenshot (`g1-s64-evidence/1280-light-01-abandon-waiting.png`) shows it.

D4 requires the line to show the intent whole. A human can therefore press an irreversible Abandon after seeing only a partial account of the goal, even when the freshness guard passes. **Smallest fix:** add an `Intent` argument from `line.read.intent` for Abandon and test it with a multi-sentence intent. **Changes what is built:** yes, the card and its test. **Works and stays safe without it:** no; the required informed press is missing.

## Conformance and proof read

| Decision | Result |
| --- | --- |
| D1 | `Abandon` (metasystem/internal/goal/abandon.go:36) admits `ValidFor` or `SessionValidFor` for the request root, rejects nil proof, and records the session on the abandoned goal's history line. The dependent refusal names `--successor`. |
| D2 | `Authority.Abandon` (metasystem/internal/ui/act/act.go:351) sends reason and successor through `request()` and `settle()`, without `Waive` or `Also`. `admitProposal` and `liveDependentsOf` (metasystem/internal/ui/partner/proposals.go:192) use live observation rows; the card describes refusal or repointing. |
| D3 | `abandonGoal` (metasystem/internal/ui/httpd/acts.go:368) uses the shared act policy and bounded decoder; its route and act-table row (metasystem/internal/ui/httpd/describe.go:70) are present. A blank reason reaches the act blank and is refused. |
| D4 | The catalogue maps public `abandon`/`reason` to route `abandon-goal`/`because`; the client, word map, dependents sentence, and freshness guard (metasystem/internal/ui/web/_app/src/partner/proposing.ts:697) are present. **Partial:** the full intent is stored and compared but not shown, as above. |
| D5 | The diff adds no board or Decisions Abandon button, no done or reopen act, and no browser waive or also field. |

The named engine tests establish session admission and history, terminal admission, nil-proof refusal, dependent refusal wording, and successor repointing. The act tests establish reason handling, repointing, blank-input refusal, and delegation of successor validity to the engine. The route tests establish body mapping, policy, signed-in hand, missing-engine handling, and passage of refusal words; their canned refusal test checks transport, while the engine tests check the words themselves. The catalogue and HTTP joins cover all ten rows and the `reason`→`because` mapping. Admission tests establish the stored read and live dependents. Frontend tests establish the word, dispatch, dependents sentence, and changed-goal/dependent guards; their Abandon display expectation misses D4's full-intent requirement.

The builder reports green Go, TypeScript, and bundle checks. My focused Go test **did not run**: the read-only sandbox refused Go's temporary build directory. The worktree remained clean.

## Departures adjudicated

1. **Accepted:** carrying the admitted proof into `abandonRequest` makes the recorded session the proof that authorized the act.
2. **Accepted:** the three existing test changes follow D1's changed proof-refusal wording and signature.
3. **Accepted:** the abandoned goal's own appended line records the session; dependent history still names the actor, and `settle()` records the proof.
4. **Accepted:** transitional `Verb`, `Body`, `ProposedActNamed`, and `ProposeVerbs` preserve the nine existing rows and correctly map Abandon's public fields to its route body. The later g1-s62 catalogue change is separate work.
5. **Accepted:** the engine validates successor liveness; admission does not need to duplicate that rule.
6. **Accepted:** neither skill copy contains a numbered act list; the tool description and schema now enumerate ten acts.
7. **Accepted:** "Holds up" is the existing label, followed by the design's dependents sentence.
8. **Accepted:** the intermediate red catalogue join is closed by the later catalogue commit; the final diff contains both sides.
9. **Accepted:** the successor sentence has a frontend assertion. The requested screenshots cover an applied abandon and a dependent refusal.

## Deferred and non-material

The new session test does not separately try a proof minted for another root; `SessionValidFor` checks the root, and the existing authority test covers another project's proof. An already closed goal with unchanged descriptive fields can pass the page comparison, but the engine refuses its Abandon. Neither changes this slice's first-use safety.

VERDICT: 1 material findings (fail the works-and-safe test): S64-C-01

# Verification of the intent-based command redesign

The coverage follow-up is complete. The unchanged full race/coverage gate
passes all 118 registered floors. Nine adoption scenarios passed in that run;
the last passed separately after an audit expectation correction. Current
frontend and HTTP/bundle checks also pass. All six
original coverage-policy failures and the subsequently discovered untested
failure mapper are resolved without lowering a floor.

Detailed sources and actual results are retained under
`/Users/wido/LocalStorage/agentic-tools-evidence/verbs-match-intent-20260925`.
The accepted [design](designs/verbs-match-intent.md) has all critical and high
obligations done; the [inventory](verbs-match-intent-goal-index.md) lists the
23 related goals.

## Coverage repair and main integration — September 26, 2026

Wido subsequently requested that the coverage failures be repaired and the
redesign integrated with remote main and pushed. The integration preserves
remote main's UI changes and all of its existing coverage floors. The original
local redesign is commit `25f0e2e04267fe626b5622e95e0c71b756d48aae`.

The final tested candidate is `29086561a4f74dc6fdd7a71fac88bb4316e705eb`,
tree `ccf74df15f01491d8173f6af8236f24167e6774f`, incorporating remote main
`dbafa75fae548ae7ff6010d51d47303a43a49e53`. The published integration has
the same product bytes; completion records and its receipt are added afterward.

The four missing package floors came from the intervening remote main changes.
The merged HTTP owner clears its unchanged floor. A new session regression
covers token-generation failure, spent pairing codes, absence of a session
on failure, and successful recovery. The full native run also exposed an
untested failure-to-ledger mapper; its new tests cover filtering, evidence,
identity and detached ownership, with four mutations caught. No floor was
lowered and no exemption was added.

The delegate integration fixture also leaked its background steward after
removing the runner identity file. Its existing cleanup now disarms through
the steward owner before removing fixture state and checks that the captured
kernel identity is gone. Both real delegate cases passed, and both owned
processes were absent afterward. Older unrelated processes were left alone.

The first full native coverage run passed all tests but failed solely on the
mapper floor; that red result remains recorded. A queued second attempt was
interrupted before native admission to repair the process leak. The third
completed native run failed the shared test-initialization audit: the new
mapper package needed its standard TestMain wrapper. That ten-line correction
passed both failing checks and independent review before the final run.

The fourth unchanged full native gate passed: 12,435 passing test terminals,
ten explicit skips, zero failures and no started test missing a terminal.
All 118 floors passed; the existing ratchet owner independently judged the
retained native output with exit zero (`coverage-final-r4-owner-judge/status.json`).
The enclosing adoption command still exited one: nine scenarios passed,
while filled-delivery exposed only an expected actor-site line whose spacing
was stale after gofmt aligned a new upstream field. That historical exit
remains one in `coverage-final-r4-gate/adoption-status.json`.

The final one-line correction changes only that expected spacing. The full
brain fixture entrypoint passes, independent Sol review has no material
findings, and the unchanged filled-delivery scenario body passes at
`29086561a4` (`coverage-filled-final/result.json`). The external wrapper
omits only outer proof admission and witness production under the explicit
bypass, with no authority or witness asserted and no scenario body altered.
Nine original passes plus the corrected scenario provide complete scenario
evidence with exact revision labels. No parent result was rewritten or
synthetic governed certificate created. Earlier red and interrupted runs remain
evidence of those attempts.

Independent Sol reviews of the merge/session/frontend repairs, mapper tests
fixture teardown and the shared TestMain correction are retained with root dispositions in the evidence
directory. The final frontend's 1,058 tests, typecheck and bundle passed on the
current integrated UI source (`coverage-final-r4-frontend`). Separate tagged tests, stop-cost checks and affected
contract sections retain their actual source revisions; the final evidence
reuse assessment records the boundaries. No governed testing certificate,
production human authority, or live installation is claimed.

The final integration includes main's subsequent edit-sheet simplification
and design records. No Go source changed after the full-gate candidate
`d374f08c9daef03cbe20ff131e34215fc242b47b`. All 1,058 frontend tests, typecheck
and bundle were rerun on `58815db14`, followed by passing HTTP and
embedded-bundle race/coverage tests (`coverage-final-main-delta/`). The final
candidate `29086561a4f74dc6fdd7a71fac88bb4316e705eb` changes only one literal
expected actor-site line in the brain audit: gofmt-aligned spaces match the
existing Go caller. Its full brain-fixture entrypoint and the affected
filled-delivery adoption body passed separately. No full Go run was repeated
for that audit text; each result retains its actual source identity.

Actual full-run coverage (all 118 registered floors pass):

| Package | Measured | Required floor |
| --- | ---: | ---: |
| `internal/rulings` | 96.1% | 96.1% |
| `internal/seat/launch` | 74.6% | 74.6% |
| `internal/ui/decisions` | 94.7% | 94.7% |
| `internal/ui/fleet` | 87.4% | 87.4% |
| `internal/ui/httpd` | 88.9% | 88.5% |
| `internal/ui/session` | 91.0% | 89.5% |
| `internal/landing/trunkredmap` | 100.0% | 42.1% |

The measurements and original producer output are retained under
`coverage-final-r4-gate/native-retained/`. Ten explicit optional or platform
skips remain recorded; skipped bodies are not counted as executed proof.

## Original redesign verification record

The remainder records the original local release at
`3e59d41077fe4cbfba6ad00f5b09f20e5c9da1a8`, tree
`88069098c0bdcc4e6de6fb703887dee35fe50be6`, integrated locally as `25f0e2e04`.
Its original failures and source-specific diagnostics are historical evidence;
the later coverage and integration outcome is recorded above.

## Scope and authority

Wido authorized the complete public command redesign, root-authored design,
independent Fable or Opus design critique, subsequent implementation, and
machinery bypass. The stop hook is ignored as instructed. The goal ledger
remains truthful: no formal approval, claim, testing attempt, certificate or
production human authority has been invented for this verification.

Product implementation was authored by Opus 5.5. Independent Sol reviews and
root's source reads cover the component changes and integration resolutions.
Fable's design critique and the connection amendment are closed; each material
finding has a disposition and proof. Source provenance and exact launches
remain in the handoff and external evidence directory.

## Observed runtime behavior

| Behavior | What root ran and observed | Retained evidence |
| --- | --- | --- |
| Public CLI and compatibility | Final rebuilt engine passed 72 exact commands: all 48 verb help pages, human/agent/internal help, equivalent directory/file/symlink paths, conflicts before mutation, forged authority refusal, literal typo remedy, exact budget and parked resume, nested process readers, legacy and internal readers | `root-release-public-runtime-3e59.json` |
| Goal authority and lifecycle | Physical Git fixtures exercised explicit fake human authority, lawful agent conclusion, parked and stopped resume, unchanged budget and claim epoch, and partial results after publication | `root-runtime-lifecycle-r2.json`, `root-runtime-lifecycle-r2-records.json`, `root-runtime-parked-final.json` |
| Named work and continuation | Changed retained brief refuses build resume and wait; restored brief returns the same run without another launch | `root-runtime-work-final.json` |
| Unit lock release | An inherited file descriptor retains a close-only lock; explicit unlock releases it. The promoted deterministic regression passes | `root-unit-lock-descriptor-probe.json`; `internal/launch/unit_lock_test.go` |
| Generated worktree and builder | Actual OS child observes worktree cwd, brief, model and declared runtime settings; repeated invocation starts no second child; interrupted settings copy finishes on retry | `root-connection-final-runtime.log` |
| Committed critic | Actual branch-read owner, built delegate, dispatch script and Claude adapter launch a real fixture executable. Plain prose gains the default mode; selected critic and existing maximal-model authorization reach the worktree. Earlier candidates fail the same probes | `root-connection-brief-red.log`, `root-connection-brief-green.log`, `root-connection-final-delegate.log` |
| Connected delivery | The c78 command group passed the physical same-goal A/B build, fold A, real register/close, collection/publication and public batch admission journey; repeated admission adds no duplicate member | `verification-final-c78-repeat/native-result.json` and its command-interface-smoke logs |

Model executables, goal authority and expensive proof operations are declared
fixtures where used. Actual process, filesystem, Git, closure and publication
owners run at the boundaries named above. This does not certify production
terminal ancestry, the semantic quality of a live model's review, or a live
production landing. Later batch sealing and push are covered separately by
the existing batch lifecycle fixture, not by the same-subject admission claim.

## Selected release checks

The existing protected testing contract was computed from product base
`4ebc7c15726ef5f8d44f1d72b2231aefae11da05` and the exact candidate, using the
live goal's risk tuple `(3, 2, 3, 2)`. It selects **198 groups: 160 native and
38 existing shell sections**, deep mode, nine workers subject to the inherited
worker ceiling. Selection and all actual results are retained separately.

The external verification driver calls the real `proofrun.RunTestPlan` and
supplies the final built engine through `WithResourceCustodyExecutable`.
The first run omitted that argument and failed custody startup before tests;
`verification-final-70d13aa39/driver-diagnosis.txt` records why that run is
invalid. It contributes no passing or failing product evidence. The corrected
run is `verification-final-70d13aa39-r2`.

The section driver extracts the selected existing bodies without changing
their checks, removes the governed entry admission wrapper under the user's
bypass, retains checkout and collection guards, and records both source hashes.
It creates no governed receipt. Each selected body must exit zero and emit its
own matching passing row; completeness is checked separately.

The evidence is deliberately recorded by revision, rather than presented as
one green result on the final source:

| Execution | Observed result | Evidence directory or file |
| --- | --- | --- |
| Complete 160 native groups at `70d13aa39` | 152 passed; five fixture failures; three groups could not start without an external driver input | `verification-final-70d13aa39-r2/native-result.json` |
| Ten affected or connected groups at `c78a9c7c0` | All passed after seven test-file corrections and the driver correction; includes ordinary and batch-tagged command suites | `verification-final-c78-repeat/native-result.json` |
| All 38 original sections at `c78a9c7c0` | 32 passed and six failed; every selected section completed | `verification-final-sections-c78/section-results.json`, `driver-status.json` |
| Final fast/static prerequisites | Both passed | `verification-final-3e59-fast/native-result.json` |
| Final documentation and protocol sections | Both passed | `verification-final-sections-3e59/section-results.json` |
| Final public CLI walkthrough | All 72 commands passed | `root-release-public-runtime-3e59.json` |
| Final adoption bodies | All ten scenarios passed; driver exited zero | `verification-final-adoption-3e59/result.json` |
| Five final affected native groups | All passed; driver exited zero; complete collection | `verification-final-3e59-native/native-result.json`, `native-status.json` |
| Thirteen previously failed scenarios | All passed: ten landing, two goal completion and one supervision scenario; each driver exited zero | `verification-final-scenario-drivers-3e59.json` |

The source at `c78a9c7c0` changed only seven test files relative to the first
complete native run. Root read the patch and independently reproduced the
baseline child-wait failure before its fixture repair. Assertions and strict
Git and model/session checks remain intact. The earlier stop-cost performance
observation retains its original revision; its measured path and private
helpers did not change. The batch-tagged pass is likewise explicitly c78
coverage, not a final-revision tagged execution.

The final production corrections after c78 are confined to two public-command
wording checks in the existing doctrine audit and registration of the existing
fixture-authority flag for legacy `done`. The other changes are meaningful
owner tests, four explicit fixture callers and audit-facing prose. Root read
them all. The five final native groups cover the common parser, audit,
launch and branch owners and test-environment policy. The read-only source
analysis is `release-evidence-composition-final.md`.

Of the six original section failures, two were interrupted by stale audit
wording and the new ruling's reference. That same audit failure interrupted
ten landing scenarios and one supervision scenario. Two goal-completion
scenarios lacked explicit fixture proof and exposed the missing parser flag.
The adoption parent stopped at its full coverage prerequisite before any
scenario started; the complete race tests passed before that refusal.
Original passing scenarios remain evidence at their actual source revision.
Filtered reruns are not described as new complete section passes.

The adoption diagnostic preserved all ten scenario bodies, capabilities,
checks and cleanup. Its external wrapper omitted only parent proof-launch
and witness startup under Wido's bypass. It asserted no worker authority or
witness and issued no governed receipt. Both the unchanged-body extraction
and actual terminal passes are retained. The original failed adoption section
remains failed evidence.

Both affected owner packages passed the builder's race/coverage execution
above unchanged floors: launch 83.4839% against 82.8%; branch 78.3509% against
77.9%. These packages, including their tests, are byte-identical between the
coverage producer and final candidate. The final completion regression proves
that an exact configured fake root succeeds and ordinary or unconfigured
roots refuse without ledger effects; its mutation check fails without the
one-line parser correction.

The original local release recorded six coverage-ratchet problems that predated the redesign:

- Missing floor registrations for unchanged `internal/rulings`,
  `internal/seat/launch`, `internal/ui/decisions` and `internal/ui/fleet`.
- HTTP UI coverage of 87.7%, below its 88.5% floor.
- UI session coverage of 88.8%, below its 89.5% floor.

Root ran both UI packages at baseline `4ebc7c157` with race and coverage.
All tests passed and the measured percentages exactly matched the candidate.
`release-baseline-ui-coverage-status.json` records the actual command and
successful exit. Those six ratchet failures were unresolved at that local release. The follow-up
above records their repair and the later successful full gate. No floor was
lowered. The original release evidence remains diagnostic; no governed
testing certificate is asserted.

No dependencies were added. Necessary process-wide fixtures remain serial;
eligible new tests were made parallel first. The command-package serial count
moved from 536 to 541 under the authorized bypass. No assertion, coverage
floor or other package baseline was weakened.

The final correction checks are complete. The local integration commit
contains a receipt and these completion records. The descriptive account
`final-diagnostic-release-account.json` links the actual terminal results and
their hashes; it is not a substitute for a governed certificate. The final
source manifest is `final-source-manifest-3e59.json`; correction decisions are
in `release-correction-root-adjudication.json`. Every original failed or
invalid run remains available beside its explicitly labeled replacement.

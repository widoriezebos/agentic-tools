# brain seat build code review — round 4 and last (chain brain-build1b-20260907, round-9 tree)

Chain: round 9 added report-package tests after the landing gate refused on a coverage floor -> reviewed tree ed8f831ea957d76058946d78577356353d5c5e13 -> critic brain-review7-20260907 (Opus 5, code-critic, read-only). ZERO material findings; 5 notes. The critic judged the new tests by deleting production behaviour in a scratch copy and watching which tests noticed.

## F-1 — medium, material=False

CLAIM: The third stanza of the new test TestQuestionAndDraftScansKeepUndeclaredVerdictUnchanged does not check the behaviour its name promises. Its purpose is to show that a brain-only scanner failure leaves an ordinary checkout's end-of-turn verdict unchanged, but the test passes unchanged when the production filter that produces that behaviour is deleted outright.

EVIDENCE: Deleting the whole filter in internal/goal/turnverdict.go that drops scanner failures prefixed 'draft scan: ' and 'question scan: ' leaves this test passing. A throwaway probe shows the verdict text really does change under that deletion, from 'no current goal; the queue holds draft-here' to '2 inputs unreadable: ...', but the test never looks at that text; it only checks that the turn is not blocked, and neither the original nor the broken code blocks. The one text check the test does make looks for the literal word UNCERTAIN, which a search of the whole tree shows appears nowhere in any Go source, only in this test and in a planning document, so that check can never fail. This is not material because the behaviour is genuinely proven elsewhere in the same chain: TestUndeclaredCheckoutDropsOnlyBrainDraftScannerFailure in internal/goal/turnverdict_brain_test.go fails under exactly that deletion, as I confirmed by running it. The stanza is therefore redundant rather than a hole in what certifies the change, and the coverage it buys is bought by its other two stanzas, which do discriminate.

## F-2 — medium, material=False

CLAIM: The checkout this review ran in does not hold the round-9 content of the one file that changed. Landing from this checkout's current staged state would ship the round-8 version of internal/report/scan_test.go and re-trip the coverage gate that caused this round to exist.

EVIDENCE: Comparing every one of the 41 changed paths in this checkout against the reviewed tree, 40 match exactly and internal/report/scan_test.go does not: this checkout holds 57052e5c4eac6adc96a8a5085a954d0d0f52f6e1, which is the round-8 content, while the reviewed tree holds d2f353fd7023205a7045c54516adb7c036b646da. This says nothing against the change under review, which is why it is not material; it is an instruction for the landing step, which must take its content from the round-9 computed patch or the implementer's branch rather than from what is already staged here.

## F-3 — low, material=False

CLAIM: The 320-byte header bound named in the design's second addendum is declared as a constant but never enforced at run time. Nothing in production code reads it.

EVIDENCE: The constant HeaderBytes in internal/brain/brain.go is referenced only from internal/brain/brain_test.go, which builds the largest header the byte caps allow and asserts that it fits. The bound is therefore proven by arithmetic over the caps rather than checked when a header is actually built. That is a defensible reading of the addendum, since the caps are what the code enforces and they imply the bound, and it is unchanged from the tree passed at the previous round.

## F-4 — low, material=False

CLAIM: The catch-all branch of the fence refuses an unrecognised act with a message that does not name a command a node should run instead, unlike every named act.

EVIDENCE: In internal/brain/brain.go, the fence's final branch returns the text 'this checkout is declared the brain; act <name> is fenced'. All seven named acts (dispatch, follow-up, cancel, close, reap, land, claim) end with the exact command for a node. The catch-all is unreachable from every call site in the tree, since each passes one of the named acts, and it fails closed, so this is a wording gap on an unreachable path rather than an act a declared brain can still perform.

## F-5 — low, material=False

CLAIM: The boot command captures the optional-input child's error output into a buffer that is never read, so a child that fails immediately is reported to the operator as a deadline overrun rather than as a failure.

EVIDENCE: In cmd/metasystem/brain_boot.go the buffer is created and attached to the child's error stream at lines 107 and 108 and appears nowhere else in the file. When a section file is missing for any reason the notice emitted reads 'BOOT DEADLINE: <names> not read within <n> ms', which misdescribes a fast crash. The operator is still told which sections are missing and is still given the command to run by hand, so no input is silently dropped; only the stated cause can be wrong. Unchanged from the tree passed at the previous round.

## Gaps the critic named

- I did not re-run the four shell fixture beds (brain, supervision-hook, goal-cli and land) or the full Go test suite. The round-9 change is a single Go test file that no shell fixture reads, and I proved cryptographically that every other file is byte-identical to the tree those beds were run against, so their evidence carries forward. I rely on the host's round-8 results for them.
- The sandbox refused to create a git index lock, so I could not compute a tree identifier for this checkout's staged state directly. I compared per-path stored-file identifiers for all 41 changed paths instead, which gives the same answer at finer grain.
- The runtime notice classifies this review as advisory: the tool catalogue was not observed, so this run cannot prove it examined the change in isolation from other context.
- I judged the new tests by deleting production behaviour in a scratch copy of the reviewed tree and watching which tests noticed. That method finds assertions that check nothing; it does not find behaviour that no test in the tree exercises at all. I did not attempt a full audit of what the report package still leaves unexercised at 87.3 percent.

## Coordinator disposition (m1, 2026-09-07)

The tree lands. Two notes were acted on immediately and three go to the
backlog.

Acted on now:

- F-2 was right and would have wasted the landing: the main checkout's
  staged state still held round 8's copy of the report scanner test,
  because the earlier landing attempt applied round 8's diff. The staged
  set was rebuilt from round 9's certified diff and the test file's
  stored identifier now matches the reviewed tree exactly.
- F-1 is the one weakness in the new tests: the stanza named for an
  undeclared checkout's verdict passes even when the production filter
  it exists to prove is deleted. The other stanzas do assert real
  behaviour and the coverage they buy is honest, so this does not hold
  the landing, but the stanza is worth strengthening and is recorded as
  such rather than left silent.

To the backlog, none of them defects in what lands:

- F-3, the 320-byte header bound is a constant proven by arithmetic in a
  test rather than checked when a header is built.
- F-4, the fence's unreachable catch-all branch is the one refusal that
  does not end with a node's command.
- F-5, the boot child's error output is captured into a buffer nothing
  reads, so an immediate child failure is reported as a deadline
  overrun.

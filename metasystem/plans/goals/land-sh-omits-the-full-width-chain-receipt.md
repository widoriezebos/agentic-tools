# land-sh-omits-the-full-width-chain-receipt

- State: approved
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="Every tier-3 goal with accumulation 2 or higher lands through a full-width chain; today each such landing needs a seat to bypass land.sh by hand, which is the class of workaround the landing scripts exist to prevent."
- Tier: 3
- Intent: A chain root whose goal has accumulation 2 or higher carries gateWidth full and its landing evaluator (internal/landing/observe.go, the width==full branch) refuses with chain-full-gate-refused unless a test receipt for the full battery command is supplied with --test-receipt. scripts/agents/land.sh only creates and passes a receipt in its tier-1 lane (create_test_receipt runs only when --tests is set, and --tests is refused outside --direct-fix tier-1), so land.sh --chain cannot land a full-width chain at all; the human-carried-landing design says the receipt step covers 'the full battery under width full (land.sh's receipt step)'. Seen 2026-09-06 on m1c landing chain rgr-build1 (goal race-gate-red-on-main, accumulation 3), which had to go through scripts/agents/commit.sh --chain --test-receipt directly. DONE means land.sh --chain on a full-width root creates the full-battery receipt itself (or accepts a receipt path), passes it to commit.sh, a fixture pins a full-width chain landing through land.sh as green, and the direct commit.sh path is no longer needed.
- Origin: main
- Next step: Read land.sh create_test_receipt and commit_changes and observe.go's width==full branch; decide whether land.sh runs the full battery itself for width full or takes --test-receipt <path>; brief one round with a fixture in the landing fixtures.
- OpenedAt: 2026-09-06T08:58:27Z
- Revision: 2
- Labels: robustness
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T11:02:38Z revision=2 opid=SB01VF9Q2A59X40GM66RJJP2JV-m1-a4f8999f authority=proven digest=848f98dd9a904f8df0dd52580828f105f1d13235fee3b981d1a3fec0db84b31b

History:
- 2026-09-06T08:58:27Z 4BGGDASRBEYZ8M01PSZ3GK2137-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=land-sh-omits-the-full-width-chain-receipt
- 2026-09-06T11:02:38Z SB01VF9Q2A59X40GM66RJJP2JV-m1-a4f8999f approve actor=human:Wido targets=code-critic-runtime-has-no-shell,human-acts-derive-their-lineage,land-sh-omits-the-full-width-chain-receipt,landing-receipt-races-the-narrator-digest
Integrity: sha256=bfc01e975c28cd0fdace70018e6f358d007994e79e67b545e1cd1a97d2d1eba9

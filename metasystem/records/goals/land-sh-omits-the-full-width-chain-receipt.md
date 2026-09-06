# land-sh-omits-the-full-width-chain-receipt

- State: done
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="Every tier-3 goal with accumulation 2 or higher lands through a full-width chain; today each such landing needs a seat to bypass land.sh by hand, which is the class of workaround the landing scripts exist to prevent."
- Tier: 3
- Intent: A chain root whose goal has accumulation 2 or higher carries gateWidth full and its landing evaluator (internal/landing/observe.go, the width==full branch) refuses with chain-full-gate-refused unless a test receipt for the full battery command is supplied with --test-receipt. scripts/agents/land.sh only creates and passes a receipt in its tier-1 lane (create_test_receipt runs only when --tests is set, and --tests is refused outside --direct-fix tier-1), so land.sh --chain cannot land a full-width chain at all; the human-carried-landing design says the receipt step covers 'the full battery under width full (land.sh's receipt step)'. Seen 2026-09-06 on m1c landing chain rgr-build1 (goal race-gate-red-on-main, accumulation 3), which had to go through scripts/agents/commit.sh --chain --test-receipt directly. DONE means land.sh --chain on a full-width root creates the full-battery receipt itself (or accepts a receipt path), passes it to commit.sh, a fixture pins a full-width chain landing through land.sh as green, and the direct commit.sh path is no longer needed.
- Origin: main
- Next step: Round two folded critique round one's operator-facing notes (reviewed tree 13963bf2): every refusal the lane adds says why before the usage line; the fixture scenario has four legs. Seat-side proof on the worktree (14:20Z, stock bash): the land bed passes all seven isolated legs. Critique round two (lsr-critic2) is running. On a clean verdict: close, land the dispositions records, pull, receipt at the frozen base, then land the chain THROUGH THE NEW LANE (the staged land.sh with --chain lsr-build1 --test-receipt <path> --staged-only), which is the goal's own proof; conclude (agent-opened).
- Concluded: Landed c9a21e96 (chain lsr-build1, two rounds, critiques lsr-critic1 and lsr-critic2 with zero material findings; dispositions 81b3356c; provenance pass bar a). land.sh accepts --test-receipt with --chain, refuses a full-width root early without one, binds a supplied receipt to the staged candidate's metasystem subtree, forwards it to the commit wrapper, and every refusal it adds says why; the landing fixture bed gained the four-leg full-width-chain scenario. Proven by the bed (seven legs green, seat-side under stock bash 3.2) and by this very landing, which went through the new lane with a receipt made in a detached worktree.
- OpenedAt: 2026-09-06T08:58:27Z
- Revision: 8
- Labels: robustness
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T11:02:38Z revision=2 opid=SB01VF9Q2A59X40GM66RJJP2JV-m1-a4f8999f authority=proven digest=848f98dd9a904f8df0dd52580828f105f1d13235fee3b981d1a3fec0db84b31b
- Sliced: machine=m1c lineage=main-1788680061-17829-64951c revision=3 at=2026-09-06T13:36:57Z

History:
- 2026-09-06T08:58:27Z 4BGGDASRBEYZ8M01PSZ3GK2137-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=land-sh-omits-the-full-width-chain-receipt
- 2026-09-06T11:02:38Z SB01VF9Q2A59X40GM66RJJP2JV-m1-a4f8999f approve actor=human:Wido targets=code-critic-runtime-has-no-shell,human-acts-derive-their-lineage,land-sh-omits-the-full-width-chain-receipt,landing-receipt-races-the-narrator-digest
- 2026-09-06T13:36:17Z 1MWTRTMRG2X6H1FTJ70YX1ESX6-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=land-sh-omits-the-full-width-chain-receipt
- 2026-09-06T13:36:57Z 9QBCC5KQKK8RK71TK5TFWA6AFY-m1c-7cd0bd60 slice-start actor=m1c+main-1788680061-17829-64951c targets=land-sh-omits-the-full-width-chain-receipt
- 2026-09-06T13:37:09Z E25T0YYXQZ74CYW9FKV1E5HN30-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=land-sh-omits-the-full-width-chain-receipt
- 2026-09-06T14:00:14Z 0N86AK7V6YTX5FZR1KGV5QWSZJ-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=land-sh-omits-the-full-width-chain-receipt
- 2026-09-06T14:20:58Z X38YW8F7SDZMSQR031W9N6WWX5-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=land-sh-omits-the-full-width-chain-receipt
- 2026-09-06T14:52:47Z P9B3BJHAZ8F55W9Y7GF7DNCXYF-m1c-7cd0bd60 done actor=m1c+main-1788680061-17829-64951c targets=land-sh-omits-the-full-width-chain-receipt
Integrity: sha256=a6845b398793a59e9abac2d130ae8d2ef4dba8e32b87ac7ff8dcf48b6500f54f

# temp-trees-never-outlive-their-owner

- State: done
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="severity 3: leaked temp trees are the disk-pressure cascade that already turned a Lima guest read-only, and 9.9 GB of them were measured on this Mac today; novelty 2: the owner key and the process reap already exist, so the design question is only where the sweep runs and what carries the stamp; exposure 3: every temp creator in the tree is in scope, including two production ones (a landing receipt clone and one payload file per supervision-hook invocation); accumulation 3: cleanup is exit-path-only, so every missed exit leaks permanently and the total only ever grows"
- Tier: 3
- Intent: Wido on 2026-09-20: "We should not leak anything in the meta system. We should have proper cleanup the moment the process ends and no longer any other process needs it." Today every temp tree is removed only on its own creator exit path (defer, trap, Close), and nothing sweeps at startup, so a SIGKILL, kill-timeout, panic or set -e death before the trap installs leaks the tree permanently; measured 9,969 MB across leaked fixture entries.
- Origin: human
- Next step: DESIGN FIRST, one chain: stamp every temp tree with the existing owner key (metasystem/scripts/agents/fixture-budget.sh:46 mints it via proc fixture-key, and it already embeds a pid plus a start-time microstamp, which is exactly "is the owner alive and still the same process"), sweep dead-owner entries at startup, and keep the trap for the happy path; then convert the byte-heavy creators, metasystem/internal/gittree/detached.go:41 and metasystem/scripts/agents/supervision-hook.sh:1685 among them; land a guard test that fails when a new MkdirTemp or mktemp call site lands unstamped.
- Concluded: Delivered by engine-owns-disk-lifetimes: temporary folders are now owned by the process that made them and swept when that process dies (b4f45f564, f54fe5b7c, 92fe59894). A guard test keeps the number of unowned temporary-folder calls at zero.
- OpenedAt: 2026-09-20T07:19:46Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-20T07:19:46Z 0MQ88A537V245RH81FASANJ7AV-m1c-c6925449 open actor=human:Wido targets=temp-trees-never-outlive-their-owner
- 2026-09-30T18:54:34Z MKET74MC92CAP5BQ3ECFYRQ5BD-ui-bc2fda53 done actor=human:Wido targets=temp-trees-never-outlive-their-owner
Integrity: sha256=c4bb0a26abbb1268a91e2bdf7238430923464fed10d3e9440ca39dcfb6e51105

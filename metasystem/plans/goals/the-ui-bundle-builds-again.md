# the-ui-bundle-builds-again

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="third-party front-end dependency versions and the generated bundle; a wrong bump breaks the browser interface and is caught by its own tests and the bundle check before main"
- Tier: 2
- Intent: The browser interface bundle can be rebuilt on this host again: the front-end dependencies that npm audit reports vulnerable (dompurify critical, vite high, vitest moderate, as reported by the ui seat on 2026-10-26) move to audit-clean versions, the pinned node toolchain installs by a documented route that works, the bundle is regenerated and TestBundleIsCurrent is green, so a change under internal/ui/web/_app can land again. Today scripts/bundle.mjs refuses and every _app change leaves the bundle check red.
- Origin: main
- Next step: Steward cuts one brief (one unit; dependency bumps, the regenerated bundle and the documented node route, Go production lines near zero): bump the three packages to the nearest audit-clean versions, fix what the bumps break in the app's own tests, record the node install route that works (npx -p node@24.21.0 no longer yields a binary; the official tarball with its checksum did), rebundle. Then lane-steward U6a2's browser line (patch builds/lane-steward-U6a-20261025/u6a2-d8b-browser-draft.patch) can land. One Astra round, machine route on ui after a-round-survives-a-restart, lane landing.
- OpenedAt: 2026-10-26T07:13:49Z
- Revision: 2
- Budget: elapsedLimit=5d attemptLimit=12 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=SBX1SDWT81JEWAVEMWM2973Q53-m1e-acb921fd minutes=2400 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-10-26T07:14:07Z revision=2 opid=SBX1SDWT81JEWAVEMWM2973Q53-m1e-acb921fd authority=proven digest=0abdcdbe832beead500e2e434025ec334787e86e2eea44b271864d0140d56056 episode=2

History:
- 2026-10-26T07:13:49Z JYJWDDDNYV4W186WP97FF6ZE6Z-m1e-acb921fd open actor=human:Wido targets=the-ui-bundle-builds-again
- 2026-10-26T07:14:07Z SBX1SDWT81JEWAVEMWM2973Q53-m1e-acb921fd approve actor=human:Wido targets=a-capacity-refusal-is-an-environment-stop,the-impact-check-selects-only-tests-this-host-can-run,the-ui-bundle-builds-again
Integrity: sha256=0d140a39a310193ff40711c8d5e75143709d398616db2b9e7c327ac7df0482a5

# the-ui-bundle-builds-again

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="third-party front-end dependency versions and the generated bundle; a wrong bump breaks the browser interface and is caught by its own tests and the bundle check before main"
- Tier: 2
- Intent: The browser interface bundle can be rebuilt on this host again: the front-end dependencies that npm audit reports vulnerable (dompurify critical, vite high, vitest moderate, as reported by the ui seat on 2026-10-26) move to audit-clean versions, the pinned node toolchain installs by a documented route that works, the bundle is regenerated and TestBundleIsCurrent is green, so a change under internal/ui/web/_app can land again. Today scripts/bundle.mjs refuses and every _app change leaves the bundle check red.
- Origin: main
- Next step: Steward cuts one brief (one unit; dependency bumps, the regenerated bundle and the documented node route, Go production lines near zero): bump the three packages to the nearest audit-clean versions, fix what the bumps break in the app's own tests, record the node install route that works (npx -p node@24.21.0 no longer yields a binary; the official tarball with its checksum did), rebundle. Then lane-steward U6a2's browser line (patch builds/lane-steward-U6a-20261025/u6a2-d8b-browser-draft.patch) can land. One Astra round, machine route on ui after a-round-survives-a-restart, lane landing.
- OpenedAt: 2026-10-26T07:13:49Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-26T07:13:49Z JYJWDDDNYV4W186WP97FF6ZE6Z-m1e-acb921fd open actor=human:Wido targets=the-ui-bundle-builds-again
Integrity: sha256=e1ae649b0f8ecf70979a3758fafced98182ff450416369b6f05ef6dbecb09d7d

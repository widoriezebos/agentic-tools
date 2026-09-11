# narrator-writes-its-digest-relative-to-cwd

- State: done
- Risk: severity=1 novelty=1 exposure=3 accumulation=1 basis="severity 1: a stray untracked records/ directory at the repository root, which also voids a landing receipt candidate; novelty 1: one path join; exposure 3: every armed seat; accumulation 1: one stray file per storyline event"
- Tier: 3
- Intent: The narrator appends its digest through a path relative to the process working directory. When the steward runner's cwd is the repository root, the append creates records/narrator-digest.log at the repository root instead of metasystem/records/narrator-digest.log: seen on m1d on 2026-09-10 (a 158-byte file holding one HIGHLIGHT line, 'A landing moved the repository storyline to commit e06236a1'), where it also voided a landing receipt (candidate moved after the committed delivery receipt) together with the tracked digest's own append. DONE means the narrator resolves its digest path from the metasystem root it serves, never from cwd, with a fixture that runs the narrator with a foreign cwd and finds the digest only under the metasystem root.
- Origin: main
- Next step: Appetite: 1h. Find the narrator's digest writer (the steward runner spawns it; grep narrator-digest.log under internal), anchor the path on the resolved installation root, add the foreign-cwd fixture.
- Concluded: Duplicate of goal:steward-narrates-beside-the-repo-root in the 2026-09-11 backlog consolidation on Wido's word; Same defect, same day: steward-narrates-beside-the-repo-root (open 0:0; m1 2026-09-10 08:17Z and 08:41Z) names the site (internal/steward NarrateDigest with the root from up --repo <repository root>) and the stash-restore breakage; this record (Wido, m1d 12:14Z) attributes it to cwd, but narratordig. Its specific requirement is appended to that goal's next step.
- OpenedAt: 2026-09-10T12:14:58Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T12:14:58Z Z0HXF84GX571J8AV7DNEMDCMSH-m1-c6925449 open actor=human:Wido targets=narrator-writes-its-digest-relative-to-cwd
- 2026-09-11T22:04:02Z ZF16FMY29KNF15ZYQFBQJ3QGYZ-m1-c6925449 done actor=human:Wido targets=narrator-writes-its-digest-relative-to-cwd
Integrity: sha256=6593514fe64a05b397c6755ca63dae01f8fd8b0bc263c26e59a99619e5d2ae32

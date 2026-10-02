# hp-enrollment-before-migration

- State: abandoned
- Risk: severity=2 novelty=2 exposure=1 accumulation=1 basis="severity 2: bootstrap ordering of enrollment and migration; a wrong order leaves migrate on the weaker grade; novelty 2: splitting enroll into local write and fleet publication; exposure 1: one act per checkout lifetime; accumulation 1: nothing compounds"
- Tier: 2
- Intent: What: A person can enroll their terminal in a new checkout before its backlog has been migrated, and the fleet-wide announcement of that enrollment waits until the backlog exists. Why: Today enrolling is refused on a checkout whose backlog is not migrated yet, and it announces to the fleet in the same act, so migrating a new checkout cannot use the person's enrolled terminal as proof and the first-run setup needs a special exception. Pros: A clean first-run order (enroll, migrate, then announce) with no exception. Cons: Enrollment becomes two steps inside the code, and it happens once per checkout, so the gain is modest.
- Origin: main
- Next step: Next: Split terminal enrollment into the local write and the fleet announcement, delay the announcement until the backlog exists, and let migrate accept the enrolled terminal as proof. Done when: a test enrolls on an unmigrated checkout, migrates it, and sees the fleet announcement published only after the migration.
- OpenedAt: 2026-09-09T14:35:30Z
- Revision: 7
- Labels: security
- BudgetExceptions: 0
- Abandoned: by=human:Wido at=2026-10-02T21:01:28Z revision=7 opid=WWQ3ESJTS8HKSA38RDZ03CPSYZ-m1e-9c612d71 because=Obsolete (audit 2026-10-02): goal migrate no longer exists

History:
- 2026-09-09T14:35:30Z S5HBNF0PRTRKM216MAZA9BB25C-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=hp-enrollment-before-migration
- 2026-09-09T14:36:28Z 76XV3QEKM1Y759NSDZ4GYEH9R3-m1-c6925449 approve actor=human:Wido targets=fixture-human-authority-mintable-from-conf,hp-enrollment-before-migration,hp-every-by-proves-a-human,hp-grading-matrix-for-widening-acts,hp-recovery-grants-no-grade-to-unstamped-intents,hp-refusals-print-the-one-command,hp-relayed-word-retired,hp-resume-takes-its-budget-from-the-ledger,hp-terminal-grade-for-stopping-acts
- 2026-09-11T07:59:46Z 0MXKAV6JW5FSD6MPZMNRJ19DVB-m1-c6925449 unapprove actor=human:Wido targets=hp-enrollment-before-migration reason=Wido 2026-09-11: standing approval withdrawn to regain control of what is built next; re-approve deliberately
- 2026-09-13T08:18:00Z 06S5RYSNWMPCBWMCTBAF0NFG7T-m1-c6925449 edit actor=human:Wido targets=hp-enrollment-before-migration
- 2026-09-30T18:50:09Z CQD3Y3NWYR48BW3R6ZGWRTVJ83-ui-bc2fda53 edit actor=human:Wido targets=hp-enrollment-before-migration
- 2026-09-30T19:07:12Z BARDC4RYZEDVKXR51ZTZAMYDTK-ui-bc2fda53 edit actor=human:Wido targets=hp-enrollment-before-migration
- 2026-10-02T21:01:28Z WWQ3ESJTS8HKSA38RDZ03CPSYZ-m1e-9c612d71 abandon actor=human:Wido targets=hp-enrollment-before-migration reason=Obsolete (audit 2026-10-02): goal migrate no longer exists
Integrity: sha256=a2d1f6ff9e8c78d27022c7a4d3c9e68c6d212cdba7515f0c6d924a0fa9db1a11

# critic-workspace-custody

- State: done
- Intent: Ad-hoc review-role dispatches default to the LIVE checkout as workspace with write grades: a certifying code-critic left modified product bytes in the coordinator's tree while testing scanner evasions (2026-08-25, jobs c342/72be/6342 — caught by an unexplained-modification check before landing)
- Origin: main
- Next step: Appetite: 2h. Review roles (code-critic, design-critic, warden) get a safe default: dispatch them --worktree or with a read-only permission envelope unless the caller explicitly overrides; the dispatcher refuses a write-granted review role on the live checkout without an explicit flag. Evidence preserved in the session scratchpad (critic-residue/); the restored tree diffed clean against HEAD. Related: suite-dispatch-exclusion (same single-writer checkout family), the wall (missions have this protection; ad-hoc dispatch does not).
- Concluded: Closed at the root: review roles (code-critic, design-critic, warden) default to a read-only envelope on the live checkout; write grants there refuse pre-spend with the role, incident class, and override flag named; worktrees keep quarantined grants; follow-ups inherit and cannot silently escalate. Certified 1/0 after the follow-up-inheritance and pre-spend fixture gaps folded; full dispatch suite green. The incident evidence (critic-residue/) stays preserved in the session record; the KI-27 --reviews-tree direction from the sweep drafts (D-7) remains a natural extension for whoever takes that draft.
- OpenedAt: 2026-08-25T20:17:12Z
- Revision: 4
- Labels: custody

History:
- 2026-08-25T20:17:12Z 2CK11CPFW4GA4KRKT0N4JAFQ9R-m2-bc1be9cb open actor=m2+mac-coordinator targets=critic-workspace-custody
- 2026-08-26T05:41:22Z 9G3F1N5EYDWPNZX15Y8A2D0EEP-m2-bc1be9cb edit actor=m2+mac-coordinator targets=critic-workspace-custody
- 2026-08-26T11:38:51Z H354P9YFJA4GS4TQMZ0A8JEBGR-m2-bc1be9cb claim actor=m2+mac-coordinator targets=critic-workspace-custody
- 2026-08-26T12:22:27Z 2G70XSM3TNTHJD3TVF0R3G0KPF-m2-bc1be9cb done actor=m2+mac-coordinator targets=critic-workspace-custody
Integrity: sha256=a7adc506e86508753338e2ae0d1633f337a651a0aa2c5402025ced678bf75307

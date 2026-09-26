# Agent help verification and scope update

Root authored and drove the help change on 26 September 2026. Evidence directory:
`/Users/wido/LocalStorage/agentic-tools-evidence/agent-help-20260926`.
This records the initial help slice; the later human instruction to remove legacy
choices and separate administration is designed in `designs/verb-cleanup.md`.
Its final acceptance must assess the combined change, not inherit a claim that
all final human help bytes are unchanged.

## Actual results before the cleanup

- Built the CLI and drove 68 cases outside every repository. All 53 pre-existing
  public human help outputs were byte-identical. No help call wrote files there.
- Agent entry point: 161 lines / 1,073 words / 31 commands became 69 lines /
  748 words / all 44 public commands, grouped by intent. Structured success and
  refusal use the existing envelope and correct exit; no handler or repository
  resolver is entered. Eight focused review forms distinguish feedback/submission.
- Focused new help tests and existing help/catalogue/Partner tests passed. A
  separate race run with the real-owner review fixtures passed in 42.756 seconds.
  The final submit --after COMMIT label correction passed focused race/help/Partner
  tests in 6.750 seconds; profile `label-correction.cover`. These subset profiles
  are not whole-package coverage measurements.
- The actual fast static gate passed: format, vet, staticcheck, dependency and
  parallel ratchets, refusal/SessionStart/Stop audits, build and project records.
  `fast-static.exit` is 0. No coverage baseline changed.
- The direct whole command-package race run hit its fixed 20-minute deadline:
  334 top-level tests passed, 8 skipped, 229 were still paused. No assertion
  failure was reported before timeout. The package outcome is failure, not
  successful evidence; `command-package.exit` is 1. Do not consume its incomplete
  coverage or raise the timeout and repeat blindly. The repository's partitioned
  native owner is the final regression runner after the cleanup stabilizes.

## Independent reviews and bounded agent trial

Fable 5.1 design launch `20260926t180359-91cd2b1378` and code launch
`20260926t182056-0b1e5a2a17` are terminal. Both reports and complete dispositions
are tracked beside this record. Code review on candidate 556676ff6 had zero
material findings. The later value-label correction was root-reviewed separately.

Opus 5.5 launch `20260926t182306-1ad931f42b` used only help from outside Git,
with no source access or task execution. It made 14 successful help calls and
selected valid forms for the ten supplied tasks, preserved a path containing a
space as one argv word, declined unauthorized accept-risk, and did not equate
build success with goal completion. It marked six tasks unambiguous and four
with contextual uncertainty (design location, original retry inputs, unavailable
build approval/size, exact wording of a human question). Root agrees those
uncertainties cannot be counted as ten end-to-end successes. This is a bounded
smoke trial, not a statistical performance improvement or workflow proof.

The trial exposed the submit --after N label, corrected to COMMIT, and incomplete
wait review discovery. The latter is included in the subsequent cleanup audit.

## Testing applicability boundary

The public policy plan refused because the enrolled live engine predates incoming
main's CSS update. Root did not re-arm the live installation or fabricate a
policy identity. An external diagnostic called the actual selection owner on
base 0a38b2e01 and candidate 556676ff6: 203 groups (125 ordinary Go, 40 other
native, 38 sections). It is root evidence under the explicit machinery bypass,
not a governed certificate. No final selection or coverage claim is made here;
the subsequent cleanup changes execution routes and needs its own final proof.

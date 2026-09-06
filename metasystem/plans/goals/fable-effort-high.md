# fable-effort-high

- State: approved
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a lower effort tier costs review depth, nothing breaks; novelty 1: the hazard table and the adapter's effort flag exist; exposure 2: every Fable dispatch (code critic, design rounds); accumulation 1: nothing compounds"
- Tier: 2
- Intent: Wido's word 2026-09-06 21:2x: 'I want to change model config for Fable; from xhigh to high.' Today the reasoning effort is not configuration: internal/dispatch/hazard.go fixes it per hazard class (MECHANICAL medium; DESIGN-BEARING and DESTRUCTIVE-REACH xhigh for builder and independent critic alike), for every runtime and model, and the admission refuses a weaker row. DONE means: a conf key per runtime and model (for example effort.claude.claude-fable-5-1=high in metasystem.conf.local) that the dispatcher applies to that pair's jobs, with the hazard table's value as the default when no key is set; the job record shows the effective effort and its origin; a fixture proves a Fable code-critic dispatch under DESIGN-BEARING runs at high when the key says so and at xhigh when it does not; the human page documents the key. The human's key is the authority: the hazard minimum is the default, not a fence against the human's own roster.
- Origin: main
- Next step: Read the hazard table and ResolveHazardConfiguration in internal/dispatch/hazard.go, the reasoningEffort composition in internal/dispatch/build.go, and the claude adapter's --reasoning-effort flag; small build (config key, resolution with origin, record field, fixture, conf.local line effort.claude.claude-fable-5-1=high); Fable code review; land.
- OpenedAt: 2026-09-06T19:51:43Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T21:08:25Z revision=2 opid=ADWN9PAKED3DRADSQDN2W0EBFR-m1-7cd0bd60 authority=proven digest=9ebdfc2860d1c042fe7f1a551df51b41171246cff4ede4da96748634085f3218

History:
- 2026-09-06T19:51:43Z K7ZED54C48XTE6PCVCTHAWM3GE-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=fable-effort-high
- 2026-09-06T21:08:25Z ADWN9PAKED3DRADSQDN2W0EBFR-m1-7cd0bd60 approve actor=human:Wido targets=fable-effort-high
Integrity: sha256=374cf083d1f02af196d33795b3cbfbdadc57e45dcf3f151b91598430275bedec

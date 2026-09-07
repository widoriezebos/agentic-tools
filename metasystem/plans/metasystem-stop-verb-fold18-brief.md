Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 20: fold the third critique (chain stopverb-build1)

The third independent read found five material items and three notes,
two accepted. Dispositions are in
records/misc/metasystem-stop-critique-r6.md and the design's new section
16 decides each; read section 16 first, it wins over earlier sections.
The acceptance the beds prove is unaffected: every one of these lives in
the three verbs' own scope handling and refusal rendering, except one
latent path in the transaction.

# Decisions (the orchestrator's; decided, not open)

D75. Per 16.1: `arm` takes `--installation` and resolves its scope
through the same parser `stop` and `status` use, so a checkout whose
installation is elsewhere can be armed by the person who stopped it. No
refusal may name a flag its own verb rejects. Package test: a scope with
a separate installation directory arms.

D76. Per 16.2: exhausting the transition lock's bounded wait against a
LIVE holder is the stop-in-progress refusal, with the holder named and
`metasystem status` as the second line. Only an unreadable or
dead-holder lock takes another branch. Test both.

D77. Per 16.3: arm's crashed-stop re-inventory records an unreadable
family as a survivor entry rather than aborting, and its refusal points
at `metasystem stop`. Test with one family unreadable.

D78. Per 16.4: `arm --all` refuses in section 9's two-line shape with
section 13.3's sentence and exit 1, through the same parse path the
other two verbs use. Test the exit code and both lines.

D79. Per 16.5: your return lists EVERY unbuilt slice-1 scenario as a
gap. Six are unbuilt and deferred (proof-run-stop, remote-job,
slow-owner, crash-recovery, ignored-signal to slice 1b; fleet to slice
2). Name them even though this brief defers them.

D80. Per 16.6: an unknown family name prints a `NOT STOPPED` line
instead of returning an error that discards the report. Test it with an
injected family.

D81. Per 16.7: the stopped health remedy prints its path unquoted like
every other remedy.

D82. Nothing else. The census-per-pass cost is recorded for slice 2; do
not change the enumeration here.

# Verification

Reported at evidence level ran, with the private caches earlier rounds
used: `scripts/agents/go-gate.sh --fast`, and `go test -count=1 -timeout
40m` over the boundary. Name what you expect from the five beds; the
orchestrator runs them outside your sandbox and reports.

# Constraints

Wall-clock budget: 120 minutes. The goal has budget for this round and a
further read. Return per the implementer schema with the cumulative diff
boundary listed, and obey D79 in it. Gap rule: stop and report a gap;
never fill it silently.

Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal fable-5-1-model-rollover)
Date: 2026-09-02 (round 2; round 1 declined to certify for want of a go run)

# Goal

Independent critique of metasystem/plans/fable-5-1-rollover-design.md (landed,
in your worktree): the design for goal fable-5-1-model-rollover under Wido's
order "Fable 5.1 is released, make sure we use that model for Fable models
going forwards". The design keeps Wido's own landed tracked value
(metasystem/metasystem.conf, runtime.claude.maximal-models=claude-fable-5-1),
changes two fixture literals in
metasystem/internal/dispatch/composition_test.go (the test composes against
the real root and is red on main today — verify by running it), appends a
ruling row R-46-m0b to metasystem/memory/rulings.md, and argues live-round
safety from the local-overlay rule.

# Your mandate

1. Is the test verdict per file correct? Your sandbox is read-only, so do not
   run go; verify by READING. Dispatcher's run on main at 2026-09-02 07:38Z,
   supplied as evidence: `go test ./internal/dispatch -run
   TestHazardConfigurationAcceptsConfiguredMaximalModel` FAILS with
   "composition_test.go:260: runtime claude has no executable maximal-effort
   mapping for destructiveReach DESIGN-BEARING". Confirm from the test source
   why (which root it composes against, which conf it reads) and grep the
   other four test files the design clears; confirm none reads the committed
   conf.
2. Is the live-round safety argument sound: the maximal gate fires at
   dispatch admission and retroactively at chain closure
   (metasystem/internal/dispatch/hazard.go) — can a seat with an unclosed
   chain critiqued on claude-fable-5 be refused at closure by this tracked
   value, and does the local-override remedy the design offers actually
   win over the tracked line (metasystem/internal/config/resolve.go)?
3. Is the ruling row accurate and does it leave R-25-m1 untouched?
4. Anything the two-literal build could break (byte pins, other tests
   asserting the old id against the real root)?

Findings material and grounded, quoting the disagreeing text or code. A
clean return closes the design phase and the build dispatches.

# Constraints

Wall-clock budget: 20 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.

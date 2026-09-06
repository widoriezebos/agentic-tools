Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal enroll-terminal-refuses-macos-terminal)
Date: 2026-09-06

# Review brief: the system-login node in the enrollment walk (chain enroll-login-build1)

FINDING IDS: chain-unique, ELN-01, ELN-02, ... never F-n.

Round budget: 1 focused round, then at most one correction and its
re-review (the goal record carries reviewRoundLimit 2). R-60-m1's rule:
a finding is material only if it changes what gets built and names the
artifact it would change.

Threat model: the enrollment walk in
metasystem/internal/humanauthority/authority.go admitting, through the
new rule, a process that an agent controls or that is not the operating
system's own login program (a user-owned process with withheld
arguments, a root-owned process with any other executable, a process
whose facts differ between its two reads); the widened darwin parent
reader and the executable-path reader in metasystem/internal/identity
changing what an ancestry walk sees for their other callers
(metasystem/internal/gaterun, metasystem/internal/validate,
metasystem/internal/lease, metasystem/cmd/metasystem/identity_probes.go)
in a way that changes a classification or a proof; the proof record
(Node, Proof.Valid) accepting a node it should not or rejecting one it
should accept; the linux build or vet breaking (the repository
cross-builds linux in its gate); a refusal that no longer carries its
outcome code or that names the wrong facts; a weakened or deleted test;
the fake reader's owner default silently turning an existing refusal
case into an admission. Out of scope: an agent that starts its own
session (tmux, script, a pseudo-terminal) and enrolls from it, which the
brief records as a boundary this goal neither opens nor closes; taste;
the census's own use of the widened parent reader beyond its tests
staying green.

Scope: the computed diff of implementer job enroll-login-build1.
Contract: metasystem/plans/enroll-terminal-login-node-build-brief.md
(decisions D1 through D7 and the measured facts) and the goal record
metasystem/plans/goals/enroll-terminal-refuses-macos-terminal.md.

# Mandate

1. The rule admits exactly the setuid-root system login program with
   withheld arguments, on both reads, and nothing else: trace every
   path through the changed stableRead and name the outcome each fact
   combination produces; compare with D1 and D2.
2. The darwin readers answer for a root-owned process (the brief's
   measured facts) without changing what they answer for user-owned and
   dead processes: read the new sysctl and proc_info code against the
   struct layouts the existing readers already document, and run the
   identity package tests.
3. The pins of D6 exist, exercise the shapes they claim (the
   Terminal.app chain with the login as session leader; the tmux
   chain; each refusal case with its named facts in the error text),
   and would fail against the old code.
4. The refusal text names the process, the reason and the tmux shape,
   and still carries the outcome code.
5. The linux cross-build and vet pass: run `GOOS=linux go build ./...`
   and `GOOS=linux go vet ./...` from the metasystem directory.
6. Nothing outside the declared boundary changed; no test weakened.

If nothing material remains, say so; that closes the chain and the fix
lands.

# Constraints

Wall-clock budget: 40 minutes. Return per the code-critic schema with
the reviewedTree from `validate conformance --stage review --job
enroll-login-build1`. Gap rule: stop and report a gap; never fill it
silently.

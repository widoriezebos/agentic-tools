# Account-provenance design critique — round 1 (Sol)

Chain: implementer-b9aa0a156d35a3030e5a716e -> design-critic-49797feaac830e69ffcc0d1f, 2026-09-02. Eight material findings; revision 2 folds by id at resume.

## account-provenance-r1-identity-verb-collision — high

CLAIM: The proposed account identity adapter verb collides with an existing interface. An implementer following the design would replace a verb that already returns runtime version and configuration hash, breaking its caller or forcing an undocumented compatibility decision.

EVIDENCE: metasystem/plans/account-provenance-design.md lines 145–167 call the account interface identity. However, metasystem/scripts/agents/adapters/claude.sh, codex.sh, devin.sh, and fake.sh already dispatch identity to their configuration-identity functions, and metasystem/internal/adapter/selftestrun.go

## account-provenance-r1-codex-proof-bound — high

CLAIM: The Codex credential-claims grade overstates its proof. As designed, it can honestly record only that a mutable local credential file contained particular claims while the Codex command-line interface separately reported some login; it cannot say the credential was issued to that identity or that this identity was logged in unless token authenticity, validity, and binding to the successful login are verified.

EVIDENCE: metasystem/plans/account-provenance-design.md lines 22–27 and 113–125 elevate decoded identity claims from the local Codex authentication file to an issued-to and logged-in assertion. The design specifies no signature, issuer, audience, expiry, or equality validation between the file's account ident

## account-provenance-r1-authority-and-disagreement — high

CLAIM: The authority and disagreement rule turns point observations into interval and cause claims that the mechanism does not prove. An announcement cannot be authoritative for every host turn, and a job-composition capture cannot be authoritative for every later paid call, when re-login is explicitly allowed. Likewise, differing records show differing observations, not necessarily that an operator switched accounts.

EVIDENCE: metasystem/plans/account-provenance-design.md lines 17–29 and 68–75 call each record authoritative for spend and say disagreement means an account switch. Lines 117–123 concede there is no per-call receipt and no refusal of re-login. Those facts support only "surface X reported identity Y at time Z.

## account-provenance-r1-retirement-condition — high

CLAIM: The landing-message retirement condition can retire the only human account statement before the records contain usable provenance. It checks only an attestation enum on one announcement, although the stated schema permits both account identifiers to be absent, the announcement may predate a re-login, and job captures may independently be unattested.

EVIDENCE: metasystem/plans/account-provenance-design.md lines 87–102 make accountId and accountLabel optional, while lines 131–143 allow the hand-written clause to disappear solely when the announcement grade is cli-surface or credential-claims. The condition does not require a usable identifier or label, a c

## account-provenance-r1-non-gating-time-bound — high

CLAIM: The promise that identity capture never gates dispatch lacks the deadline and process-containment contract needed to make it true. A synchronous identity command can hang rather than fail, especially for the network-dependent Devin surface, leaving implementers to invent a timeout and termination outcome.

EVIDENCE: metasystem/plans/account-provenance-design.md lines 77–81 handles failed or empty capture but not a capture that never returns. Lines 114 and 242–245 say Devin could not be exercised because of network egress, while lines 224–225 merely assume a sub-second call. No maximum duration, kill behavior, o

## account-provenance-r1-runtime-registry-coverage — medium

CLAIM: The statement that a new runtime needs only an adapter script and no engine edit is false for the shipped registry. The design must distinguish generic account capture for an already registered runtime from registration of a genuinely new runtime; as written, its reject condition and extensibility claim contradict the code.

EVIDENCE: metasystem/plans/account-provenance-design.md lines 169–172 promises script-only runtime addition. metasystem/internal/runtimes/runtimes.go lines 1–12 and 161–242 declare the runtime universe in Go, and metasystem/internal/config/validate.go lines 121–139 rejects any name outside that universe befor

## account-provenance-r1-semantic-validation-fixtures — high

CLAIM: The account-object validator and fixture plan stop at syntactic JSON and do not pin the semantic or secrecy contract. An implementation can accept an attested object with no identity, an unattested object that carries identity, unknown fields containing credential material, an invalid timestamp, or a raw sensitive error while still satisfying the only named malformed-JSON test.

EVIDENCE: metasystem/plans/account-provenance-design.md lines 85–102 describes field combinations informally and introduces an error member outside the displayed object. Lines 153–158 assign shape validation to the engine, but lines 204–208 name only bad JSON degrading to unattested. No fixture pins closed ke

## account-provenance-r1-devin-unresolved-mapping — high

CLAIM: The Devin account mapping is an unresolved design decision deferred to implementation. "Map whatever it proves" leaves the implementer to choose fields, grade, and failure behavior after an unrecorded live experiment, contrary to the gap rule.

EVIDENCE: metasystem/plans/account-provenance-design.md line 114 says the Devin output is unverified and directs implementation to inspect it later on a network-capable terminal. Lines 242–245 repeat that limitation without selecting an unconditional unattested first version or providing a sanitized observed-

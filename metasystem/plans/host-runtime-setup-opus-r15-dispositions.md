# Host runtime setup: sixth independent code review

Opus5 reviewed installation tree 0b14ad2c66f770299927cdf946e0650f80cf2ea2
and the separate outer registration/instruction patch whose SHA256 is
b10363419cefc5b6db91612f77d6b155ef6ddf419bd8147a5a1b0331dece30d7.
The immutable return reports one material finding and five nonmaterial
observations. The coordinator refutes the proposed blocker below because it
opposes Wido's explicit shared-default requirement. The return is preserved;
this is not described as a zero-material reviewer verdict or a risk waiver.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| HOST-C6-001 | refuted | The observation that adopters selecting Codex inherit Astra is true and is the requested contract, not a defect. Wido explicitly requested Codex/Astra/xhigh design authoring and then asked for a global default; R-87-m1c and design-default-brief-r13.md bind that to the shared tracked MetaSystem configuration. The older anonymous-template convention was deliberately superseded for this one role. Exact check: `bin/metasystem job resolve-roster --conf <isolated copy of the reviewed metasystem.conf> --mode design --role implementer` exited 0 and returned codex:gpt-6-astra without an overlay, then exited 0 and returned claude:claude-opus-5 with an explicit local override; commands and observed JSON are in artifacts/host-runtime-setup/shared-design-main-roster-proof.json. Its shared configuration SHA256 b063a55168dd0c2d95700d688fe922aa2909c7fa88d4d3892ec59823b46470e0 matches the final reviewed file. The critic independently ran tailoring and resolution and confirmed both default inheritance and override precedence, and confirmed Claude-only adoption removes the Codex binding. docs/orchestration.md documents the choice. Restoring a placeholder would remove the authorized global default; no concrete wrong behavior, unapproved model dispatch, or broken certification is demonstrated. | none; retain the explicitly requested default |
| HOST-C6-002 | noted | The unused legacy hook checker is optional cleanup. It changes no current runtime or certification result and is outside the remaining delivery work. | none |
| HOST-C6-003 | noted | The receipt hook now uses the shared runtime/custody guard to preserve delegate isolation. The reviewer measured no exceeded deadline. Complete native Claude and mixed-runtime fixtures passed in the R14 proof whose runtime source is identical in R15. | none |
| HOST-C6-004 | noted | Adoption ships all adapter scripts. The reviewer found no supported installation missing one, and the fallback retains strict recorded identity checks. No observed defect is established. | none |
| HOST-C6-005 | noted | The recovery test still requires initial failure and then exact live runner identity, current enrollment generation and engine digest. Accepting an already verified recovered runner is authorized and passed all eleven native supervision scenarios in R10; the relevant fixture is unchanged in R15. | none |
| HOST-C6-006 | noted | The critic-only branch excludes work selection, while register, terminal-round, mirror and effort checks remain. The reviewer reports no valid critic chain that can exploit the missing builder-round check. Optional strengthening is not a delivery requirement. | none |

The critic read the full source diff and outer patch, and independently checked
the five-line final delta, copied profile hashes, relative skill links, preserved
coverage floors and hook guard ordering. It ran configuration and structural
probes but no native suite or provider lifecycle session. Main owns those proofs
and must read the complete final suite log and finish actual installation before
certifying delivery. Generated configuration readiness is distinct from a
provider application's trust and actual lifecycle execution.

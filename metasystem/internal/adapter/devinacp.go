package adapter

// Devin's ACP dialect, registered per the seam pattern. Both
// mappings are BEHAVIORAL EVIDENCE from the P1 wire probe
// (records/acp/acp-wire-probe.md):
//
//   - tools=read-only → mode "ask": step C showed ask mode strips
//     the toolset to read-only tools and the turn still ends
//     usefully — the deny leg, observed.
//   - tools=runtime-default → mode "accept-edits": step D showed
//     the default mode executing write and shell tools locally,
//     auto-approved — the allow leg, observed.
//
// "bypass" is deliberately unreachable from any envelope grade:
// it is the dangerous mode this transport exists to retire, and
// "smart" auto-approved an out-of-workspace write in step E, so
// neither is a lawful target of the mapping.
func init() {
	RegisterACPDialect("devin", ACPDialect{
		ModeForTools: map[string]string{
			"read-only":       "ask",
			"runtime-default": "accept-edits",
		},
	})
}

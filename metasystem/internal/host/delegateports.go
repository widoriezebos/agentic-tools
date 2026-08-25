package host

// The host adapters' read-side registration with the delegate seam —
// the host-side halves of the same usage and result beats the
// dispatch adapters register. Signatures match the owner functions;
// the collect port copies fields one-for-one and returns the verdict
// document bytes.

import (
	"encoding/json"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
)

func init() {
	delegate.RegisterPorts("claude", delegate.Ports{
		HostResult:            ClaudeResult,
		HostBoundaryAskEvents: HostBoundaryAskEvents("claude"),
	})
	delegate.RegisterPorts("codex", delegate.Ports{
		HostBoundaryAskEvents: HostBoundaryAskEvents("codex"),
	})
	delegate.RegisterPorts("devin", delegate.Ports{
		HostReturn:            DevinReturn,
		HostTurnUsage:         HostDevinUsage,
		HostCollect:           hostCollectPort,
		HostBoundaryAskEvents: HostBoundaryAskEvents("devin"),
	})
	delegate.RegisterPorts("fake", delegate.Ports{
		HostFakeReturn:        FakeReturn,
		HostFakeResult:        FakeResult,
		HostBoundaryAskEvents: HostBoundaryAskEvents("fake"),
	})
}

func hostCollectPort(in delegate.HostCollectInputs) ([]byte, bool, error) {
	verdict, err := HostDevinCollect(HostCollectParams{
		Root:           in.Root,
		TurnRecordPath: in.TurnRecordPath,
		TurnDir:        in.TurnDir,
		Workspace:      in.Workspace,
		StdoutPath:     in.StdoutPath,
		NamedPath:      in.NamedPath,
		TranscriptPath: in.TranscriptPath,
		RejectDigests:  in.RejectDigests,
	})
	if err != nil {
		return nil, false, err
	}
	encoded, err := json.Marshal(verdict)
	if err != nil {
		return nil, false, err
	}
	return encoded, verdict.Delivered, nil
}

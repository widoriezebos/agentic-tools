package adapter

// The dispatch adapters' read-side registration with the delegate
// seam: usage extraction and result interpretation, exactly the
// operations this package already owns, behind the seam's port
// vocabulary. The signatures match the owner functions one-for-one,
// so the wiring carries no argument mapping beyond the collect
// input's field-for-field copy — and the differential fixtures hold
// the wiring to byte identity against the direct calls.

import (
	"encoding/json"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
)

func init() {
	delegate.RegisterPorts("claude", delegate.Ports{
		Usage:       ClaudeUsage,
		ResultField: ClaudeResultField,
		Events:      RuntimeEvents("claude"),
	})
	delegate.RegisterPorts("codex", delegate.Ports{
		Usage:  CodexUsage,
		Events: RuntimeEvents("codex"),
	})
	delegate.RegisterPorts("devin", delegate.Ports{
		TurnUsage: DevinTurnUsage,
		Settle:    DevinSettle,
		Collect:   collectPort,
		Events:    RuntimeEvents("devin"),
	})
	delegate.RegisterPorts("fake", delegate.Ports{
		Usage:  func(_, outputPath string) error { return WriteFakeUsage(outputPath) },
		Return: WriteFakeReturn,
		Events: RuntimeEvents("fake"),
	})
}

// collectPort maps the seam's collect inputs onto the owner's params
// field-for-field and returns the verdict document bytes with the
// delivered flag.
func collectPort(in delegate.CollectInputs) ([]byte, bool, error) {
	verdict, err := DevinCollect(CollectParams{
		Root:           in.Root,
		Job:            in.Job,
		RoundDir:       in.RoundDir,
		Workspace:      in.Workspace,
		StdoutPath:     in.StdoutPath,
		NamedPath:      in.NamedPath,
		TranscriptPath: in.TranscriptPath,
		ACPOutcomePath: in.ACPOutcomePath,
		RecordPath:     in.RecordPath,
		Attempt:        in.Attempt,
		Session:        in.Session,
		PresenceOnly:   in.PresenceOnly,
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

package host

// The host-side boundary-ask projection (acp-adapter-seam slice
// three): the accepted host return's askCandidates surfaced as seam
// events. CLI runtimes cannot ask mid-turn — their approval
// surfaces are structurally suppressed — so the turn boundary is
// where an emulated ask truthfully lives. Projection only: the
// candidate object rides verbatim, ids stay adjudication's to
// allocate, and answering stays with the mission runner.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
)

// HostBoundaryAskEvents is the port body for one host runtime. The
// accepted-return document is runtime-neutral (the orchestrator
// schema owns askCandidates), so one projection serves every host
// runtime; the runtime name only prefixes the kind. Missing file or
// no candidates yields empty and nil error; a malformed document is
// an ERROR, never invention.
func HostBoundaryAskEvents(runtime string) func(acceptedReturnPath string) ([]delegate.Event, error) {
	return func(acceptedReturnPath string) ([]delegate.Event, error) {
		data, err := os.ReadFile(acceptedReturnPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		var body struct {
			AskCandidates []json.RawMessage `json:"askCandidates"`
		}
		if err := json.Unmarshal(data, &body); err != nil {
			return nil, fmt.Errorf("accepted return %s unreadable: %v", filepath.Base(acceptedReturnPath), err)
		}
		var events []delegate.Event
		for index, candidate := range body.AskCandidates {
			events = append(events, delegate.Event{
				Seq:    uint64(index + 1),
				Kind:   runtime + "/boundary-ask-candidate",
				Params: append([]byte(nil), candidate...),
			})
		}
		return events, nil
	}
}

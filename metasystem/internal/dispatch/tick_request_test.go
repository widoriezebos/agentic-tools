package dispatch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/tickrequest"
)

func TestJobTerminalTransitionsRequestOnlyTheirCheckout(t *testing.T) {
	for _, terminal := range []string{"completed", "failed", "cancelled", "timeout", "protocol"} {
		t.Run(terminal, func(t *testing.T) {
			root, other := sandbox(t), sandbox(t)
			createPending(t, root, "job-a")
			setupPending(t, root, "job-a")
			patch := writeJSON(t, filepath.Join(t.TempDir(), "patch.json"), map[string]any{})
			if _, err := RecordCAS(root, "job-a", "pending", "running", patch); err != nil {
				t.Fatal(err)
			}
			if _, err := tickrequest.Read(root); !os.IsNotExist(err) {
				t.Fatal("nonterminal transition requested a tick")
			}
			if terminal == "protocol" {
				if err := RecordProtocolError(root, "job-a", "running", "bad output", ""); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := RecordCAS(root, "job-a", "running", terminal, patch); err != nil {
					t.Fatal(err)
				}
			}
			r, err := tickrequest.Read(root)
			if err != nil || r.Reason != "job-ended:job-a" {
				t.Fatalf("request=%+v err=%v", r, err)
			}
			if _, err := tickrequest.Read(other); !os.IsNotExist(err) {
				t.Fatal("other checkout received request")
			}
			if err := tickrequest.Acknowledge(root, r); err != nil {
				t.Fatal(err)
			}
			if terminal == "protocol" {
				if err := RecordProtocolError(root, "job-a", "running", "bad output", ""); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := RecordCAS(root, "job-a", terminal, terminal, patch); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := tickrequest.Read(root); !os.IsNotExist(err) {
				t.Fatal("idempotent publication requested another tick")
			}
		})
	}
}

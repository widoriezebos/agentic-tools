package channel

import (
	"context"
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/tickrequest"
)

func TestQuestionAnswerAndWithdrawalRequestLocalSteward(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"answer", "withdraw"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			bed, p, q, now := pollLedgerBed(t)
			if _, err := tickrequest.Read(bed.root); !os.IsNotExist(err) {
				t.Fatal("asking a question requested a cycle")
			}
			if action == "withdraw" {
				if _, err := Withdraw(bed.root, q.ID, "superseded", nil, DestinationConfig{}, now); err != nil {
					t.Fatal(err)
				}
			} else {
				answerCodeOff(t, bed.root)
				cfg := pollBedConfig(bed, p, now)
				cfg.TOTPSecret = ""
				p.inbound = []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "approved", SentAt: now}}
				if _, err := bed.poll(context.Background(), cfg); err != nil {
					t.Fatal(err)
				}
			}
			r, err := tickrequest.Read(bed.root)
			if err != nil {
				t.Fatal(err)
			}
			if r.Reason != "question-closed:"+q.ID {
				t.Fatalf("request: %+v", r)
			}
			if _, err := tickrequest.Read(t.TempDir()); !os.IsNotExist(err) {
				t.Fatal("question reached another checkout")
			}
			if err := tickrequest.Acknowledge(bed.root, r); err != nil {
				t.Fatal(err)
			}
			if _, err := Withdraw(bed.root, q.ID, "superseded", nil, DestinationConfig{}, now); err != nil {
				t.Fatal(err)
			}
			if _, err := tickrequest.Read(bed.root); !os.IsNotExist(err) {
				t.Fatal("closed question requested another cycle")
			}
		})
	}
}

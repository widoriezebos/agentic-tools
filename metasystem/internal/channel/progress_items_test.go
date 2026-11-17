package channel

import (
	"context"
	"testing"
)

func TestPollCountsEachDispositionAfterItsWork(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"post", "answer", "contract reply"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed, p, q, now := pollLedgerBed(t)
			count := 0
			if scenario == "post" {
				q.Thread = nil
				p.beforePost = func() {
					if count != 0 {
						t.Fatalf("post counted before returning: %d", count)
					}
				}
			} else {
				if scenario == "contract reply" {
					q.Contract = true
				}
				code, err := TOTPCode("JBSWY3DPEHPK3PXP", now)
				if err != nil {
					t.Fatal(err)
				}
				p.inbound = []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "approved " + code, SentAt: now}}
			}
			if err := writeJSON(questionPath(bed.root, q.ID), q); err != nil {
				t.Fatal(err)
			}
			cfg := pollBedConfig(bed, p, now)
			cfg.Item = func() { count++ }
			result, err := bed.poll(context.Background(), cfg)
			want := 2
			if scenario == "post" {
				want = 1
			}
			if err != nil || result.Dispositions != want || count != want {
				t.Fatalf("count=%d dispositions=%d want=%d err=%v", count, result.Dispositions, want, err)
			}
		})
	}
}

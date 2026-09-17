package main

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

func TestBatchWithdrawCommandUsesRecordedJoinerIdentity(t *testing.T) {
	if reflect.ValueOf(landingBatchVerbs["withdraw"]).Pointer() != reflect.ValueOf(runBatchWithdraw).Pointer() {
		t.Fatal("landing batch withdraw is not registered to its implementation")
	}
	if reflect.ValueOf(productionBatchWithdrawDependencies().withdraw).Pointer() != reflect.ValueOf(batch.RequestWithdrawal).Pointer() {
		t.Fatal("production withdraw does not use the batch withdrawal transaction")
	}
	request := batchWithdrawRequest{SeatRoot: "/seat", LandingRoot: "/landing", GoalID: "goal-a", At: time.Unix(2, 0)}
	ensured := false
	dependencies := batchWithdrawDependencies{
		machine: func(string) (string, error) { return "seat-machine", nil },
		lineage: func() string { return "seat-lineage" },
		withdraw: func(_ batch.Store, goalID, machine, lineage, seatRoot, actor string, at time.Time) (batch.Record, error) {
			if goalID != "goal-a" || machine != "seat-machine" || lineage != "seat-lineage" || seatRoot != "/seat" || actor != "seat-machine+seat-lineage" || !at.Equal(request.At) {
				t.Fatalf("withdraw identity goal=%s machine=%s lineage=%s root=%s actor=%s at=%s", goalID, machine, lineage, seatRoot, actor, at)
			}
			return batch.Record{BatchID: "01j5x00000000000000000ba01"}, nil
		},
		ensure: func(root string) error { ensured = root == "/landing"; return nil },
	}
	if _, err := executeBatchWithdraw(request, dependencies); err != nil || !ensured {
		t.Fatalf("withdraw command err=%v ensured=%t", err, ensured)
	}
	dependencies.lineage = func() string { return "" }
	if _, err := executeBatchWithdraw(request, dependencies); err == nil || !strings.Contains(err.Error(), "BATCH_WITHDRAW_REFUSED") {
		t.Fatalf("missing lineage refusal=%v", err)
	}
}

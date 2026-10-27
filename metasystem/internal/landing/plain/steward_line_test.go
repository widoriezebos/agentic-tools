package plain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestLandingStewardLineIdentityIgnoresAges(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	b.lines("queue.jsonl", Line{Goal: "g", SHA: "g", At: bedNow.Add(-75 * time.Minute).Format(time.RFC3339Nano)})
	b.step(bedNow)
	first, err := ReadStewardLine(b.install)
	if err != nil || first.Identity == "" || len(first.Rows) == 0 || first.Report.Summary == "" || !first.At.Equal(bedNow) {
		t.Fatalf("line=%+v err=%v", first, err)
	}
	b.step(bedNow.Add(30 * time.Minute))
	second, err := ReadStewardLine(b.install)
	if err != nil || second.Identity != first.Identity || second.Rows[len(second.Rows)-1].Minutes == first.Rows[len(first.Rows)-1].Minutes {
		t.Fatalf("age changed identity or did not advance: first=%+v second=%+v err=%v", first, second, err)
	}
	b.lines("queue.jsonl", Line{Goal: "g", SHA: "g", At: bedNow.Add(-75 * time.Minute).Format(time.RFC3339Nano)}, Line{Goal: "another", SHA: "another", At: bedNow.Add(-90 * time.Minute).Format(time.RFC3339Nano)})
	b.step(bedNow.Add(30 * time.Minute))
	third, err := ReadStewardLine(b.install)
	if err != nil || third.Identity == second.Identity {
		t.Fatalf("new row kept identity: %+v %v", third, err)
	}
	status := ObserveStatus(b.home, b.record, b.seams, nil)
	data, err := json.Marshal(status)
	if err != nil || status.Health == nil || status.Health.LineText == "" || !strings.Contains(string(data), `"line":`) {
		t.Fatalf("status lost health.line: %s %v", data, err)
	}
}

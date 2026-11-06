package channel

import (
	"context"
	"testing"
	"time"
)

func TestPollPlainCloseUsesItsClock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 8, 28, 15, 4, 0, 0, time.FixedZone("fixture", 2*3600))
	q := Question{ID: "plain", Goal: "bounded", State: "answered", OpenedAt: now.Add(-time.Hour), Answer: &Answer{At: now.Add(-time.Minute), Phase: "recorded"}}
	if err := writeJSON(questionPath(root, q.ID), q); err != nil {
		t.Fatal(err)
	}
	if err := advanceAnswerWithEndpoint(context.Background(), PollConfig{RepoRoot: root, Now: now}, &q, nil); err != nil {
		t.Fatal(err)
	}
	stored, err := ReadQuestion(root, q.ID)
	if err != nil || stored.State != "closed" || !stored.ClosedAt.Equal(now) || stored.ClosedAt.Location() != time.UTC {
		t.Fatalf("poll close lost injected UTC time: %+v %v", stored, err)
	}
}

package goal

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type recordReadWithoutCapture struct {
	Repository
	captures int
}

func (r *recordReadWithoutCapture) Capture(string) (string, error) {
	r.captures++
	return "", errors.New("network is offline")
}

func TestChannelRecordsReadOnlyTheAcceptedLedger(t *testing.T) {
	t.Parallel()
	ep, _ := fakeGoalEndpoint(t)
	repository := &recordReadWithoutCapture{Repository: ep.repository()}
	ep.Repository = repository
	files, err := ReadRecordFiles(ep, goalsPrefix)
	if err != nil || len(files) == 0 || repository.captures != 0 {
		t.Fatalf("local records: files=%d captures=%d err=%v", len(files), repository.captures, err)
	}
}

func TestChannelAnswerDeadlineStopsFetchAndPush(t *testing.T) {
	t.Parallel()
	ep := Endpoint{Root: t.TempDir(), Remote: "unreachable", Branch: "refs/heads/goals"}.WithNetworkDeadline(time.Unix(1, 0))
	if _, err := CaptureTip(ep, "expired"); err == nil || !strings.Contains(err.Error(), "deadline expired") {
		t.Fatalf("expired fetch: %v", err)
	}
	if outcome, err := PublishCAS(ep, strings.Repeat("1", 40), strings.Repeat("2", 40)); outcome != CASUnknown || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expired push: %s %v", outcome, err)
	}
}

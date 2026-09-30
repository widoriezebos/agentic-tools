package batchowner

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// The judgement itself: a failed up is the owner's re-arm done only when the
// engine is current and the supervision is armed afterwards; up's exit and
// words decide nothing else.
func TestLandingOwnerRearmJudgesUpByTheStateItLeaves(t *testing.T) {
	t.Parallel()
	failed := func(context.Context, string, string) (testrun.UpOutcome, error) {
		return testrun.UpOutcome{Failed: true}, errors.New("bin/metasystem up --repo /lane: exit status 1")
	}
	succeeded := func(context.Context, string, string) (testrun.UpOutcome, error) { return testrun.UpOutcome{}, nil }
	current := func(string) error { return nil }
	stale := func(string) error { return errors.New("enrolled engine digest changed") }
	armed := func(string) (bool, error) { return true, nil }
	down := func(string) (bool, error) { return false, nil }
	unknown := func(string) (bool, error) { return false, errors.New("unreadable owner record") }
	for _, test := range []struct {
		name    string
		up      func(context.Context, string, string) (testrun.UpOutcome, error)
		current func(string) error
		armed   func(string) (bool, error)
		want    string
	}{
		{"session step stop, engine current, supervision armed", failed, current, armed, ""},
		{"session step stop, supervision down", failed, current, down, "supervision is not armed"},
		{"session step stop, supervision unknown", failed, current, unknown, "unreadable owner record"},
		{"engine not current", failed, stale, armed, "enrolled engine digest changed"},
		{"up armed everything", succeeded, stale, down, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := ownerUpLandedEngineWith(context.Background(), "/lane", "/lane", test.up, test.current, test.armed)
			if test.want == "" && err != nil || test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("re-arm = %v; want %q", err, test.want)
			}
		})
	}
}

package supervise

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAwaitArmedWaitsForTheCensusOfItsGeneration(t *testing.T) {
	t.Parallel()
	pending := ArmedInspection{Component: "repo-watcher", Reason: "the latest census did not succeed"}
	failure := func(published, generation, seq int64, message string) armingLook {
		return armingLook{inspection: pending, published: published,
			census: armingCensus{present: true, generation: generation, scanSeq: seq, firstError: message}}
	}
	armed := armingLook{published: 7, census: armingCensus{present: true, generation: 7, scanSeq: 4, succeeded: true}}
	type step struct {
		after time.Duration
		look  armingLook
	}
	for _, test := range []struct {
		name    string
		steps   []step
		elapsed time.Duration
		want    ArmedInspection
	}{
		{
			name: "late-retry-arms",
			steps: []step{{0, failure(7, 0, 1, "supervision-state: state.json: no such file")},
				{100 * time.Second, armingLook{published: 7, census: armingCensus{present: true, generation: 7, scanSeq: 2, succeeded: true}}}},
			elapsed: 100 * time.Second,
		},
		{
			name:    "one-failure-then-success",
			steps:   []step{{0, failure(7, 7, 3, "scope-unresolved:1234")}, {time.Second, armed}},
			elapsed: time.Second,
		},
		{
			name:    "two-failures-end-early",
			steps:   []step{{0, failure(7, 7, 3, "scope-unresolved:1234")}, {time.Second, failure(7, 7, 4, "scope-unresolved:4242")}},
			elapsed: time.Second,
			want:    ArmedInspection{Component: "repo-watcher", Reason: "two censuses of supervision generation 7 failed; the latest: scope-unresolved:4242"},
		},
		{
			name:    "one-failure-seen-many-times",
			steps:   []step{{0, failure(7, 7, 3, "scope-unresolved:4242")}},
			elapsed: 190 * time.Second,
			want:    ArmedInspection{Component: "repo-watcher", Reason: pending.Reason + "; latest census: scanSeq 3, generation 7, verdict CENSUS-FAILED, scope-unresolved:4242"},
		},
		{
			name:    "other-generation-never-counts",
			steps:   []step{{0, failure(7, 6, 5, "scope-unresolved:4242")}, {time.Second, failure(7, 6, 6, "scope-unresolved:4242")}},
			elapsed: 190 * time.Second,
			want:    ArmedInspection{Component: "repo-watcher", Reason: pending.Reason + "; latest census: scanSeq 6, generation 6, verdict CENSUS-FAILED, scope-unresolved:4242"},
		},
		{
			name: "before-publication-never-counts",
			steps: []step{{0, failure(0, 0, 1, "supervision-state: absent")}, {time.Second, failure(0, 0, 2, "supervision-state: absent")},
				{2 * time.Second, failure(7, 7, 3, "scope-unresolved:4242")}, {3 * time.Second, armed}},
			elapsed: 3 * time.Second,
		},
		{
			name:    "no-census",
			steps:   []step{{0, armingLook{inspection: pending, published: 7}}},
			elapsed: 190 * time.Second,
			want:    ArmedInspection{Component: "repo-watcher", Reason: pending.Reason + "; no census published"},
		},
		{
			name:    "successful-census-with-unverified-inspection",
			steps:   []step{{0, armingLook{inspection: pending, published: 7, census: armed.census}}},
			elapsed: 190 * time.Second,
			want:    ArmedInspection{Component: "repo-watcher", Reason: pending.Reason + "; latest census: scanSeq 4, generation 7, verdict SUCCESS, "},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			start := time.Unix(1_759_000_000, 0)
			at := start
			look := func(now time.Time) armingLook {
				current := test.steps[0].look
				for _, step := range test.steps {
					if now.Sub(start) >= step.after {
						current = step.look
					}
				}
				return current
			}
			got := awaitArmed(look, func() time.Time { return at }, func(duration time.Duration) { at = at.Add(duration) },
				armingWaitCeiling(EnsureOptions{IntervalSec: 60, WaitScaleMilli: 1000}))
			if got != test.want || at.Sub(start) != test.elapsed {
				t.Fatalf("inspection = %+v after %s; want %+v after %s", got, at.Sub(start), test.want, test.elapsed)
			}
		})
	}
}

func TestArmingLookReadsTheOwnersGenerationAndTheLatestCensus(t *testing.T) {
	t.Parallel()
	failed := armingCensus{present: true, generation: 7, scanSeq: 4, firstError: "scope-unresolved:4242"}
	for _, test := range []struct {
		name      string
		other     bool
		badState  bool
		census    string
		published int64
		want      armingCensus
	}{
		{name: "this-owner", census: `{"verdict":"CENSUS-FAILED","generation":7,"scanSeq":4,"errors":["scope-unresolved:4242","later error"]}`, published: 7, want: failed},
		{name: "another-owner", other: true, census: `{"verdict":"CENSUS-FAILED","generation":7,"scanSeq":4,"errors":["scope-unresolved:4242"]}`, want: failed},
		{name: "null-generation", census: `{"verdict":"CENSUS-FAILED","generation":null,"scanSeq":4,"errors":["supervision-state: absent"]}`, published: 7,
			want: armingCensus{present: true, scanSeq: 4, firstError: "supervision-state: absent"}},
		{name: "no-census-file", published: 7},
		{name: "success", census: `{"verdict":"SUCCESS","generation":7,"scanSeq":5,"errors":[]}`, published: 7,
			want: armingCensus{present: true, generation: 7, scanSeq: 5, succeeded: true}},
		{name: "unreadable-state", badState: true, census: `{"verdict":"CENSUS-FAILED","generation":7,"scanSeq":4,"errors":["scope-unresolved:4242"]}`, want: failed},
		{name: "malformed-census", census: `{`, published: 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			owner := ArmingOwner{Pid: 41, PidStartedAt: 100, InstanceTag: "owner-a"}
			document := stateDocument{Generation: 7, Owner: stateIdentity{Pid: owner.Pid, PidStartedAt: owner.PidStartedAt, InstanceTag: owner.InstanceTag}}
			if test.other {
				document.Owner.InstanceTag = "owner-b"
			}
			data, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			if test.badState {
				data = []byte(`{`)
			}
			if err := os.MkdirAll(SupervisionDir(root), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(SupervisionDir(root), "state.json"), data, 0o600); err != nil {
				t.Fatal(err)
			}
			if test.census != "" {
				if err := os.WriteFile(filepath.Join(SupervisionDir(root), "last-census.json"), []byte(test.census), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			at := time.Unix(1_759_000_000, 0)
			var inspectedAt time.Time
			inspection := ArmedInspection{Component: "reaper", Reason: "the component heartbeat is stale"}
			got := armingLookAt(root, owner, func(now time.Time) ArmedInspection {
				inspectedAt = now
				return inspection
			})(at)
			want := armingLook{inspection: inspection, published: test.published, census: test.want}
			if got != want || inspectedAt != at {
				t.Fatalf("look = %+v, inspection time = %s; want %+v at %s", got, inspectedAt, want, at)
			}
		})
	}
}

package plain

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestLandingHealthGoalsUseTheirOwnReasons(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	at := bedNow.Add(-95 * time.Minute).Format(time.RFC3339Nano)
	status := Status{Queue: []Entry{
		{Goal: "old", State: StateLanded, At: at},
		{Goal: "landed", State: StateLanded, At: at, LandedAt: bedNow.Add(-10 * time.Minute).Format(time.RFC3339Nano)},
		{Goal: "outside-window", State: StateLanded, At: bedNow.Add(-26 * time.Hour).Format(time.RFC3339Nano), LandedAt: bedNow.Add(-25 * time.Hour).Format(time.RFC3339Nano)},
		{Goal: "waiting", State: StateWaiting, At: at, Reason: "main needs proof"},
		{Goal: "selected", State: StateWaiting, At: at, SHA: "selected-tip"},
		{Goal: "next", State: StateWaiting, At: at, SHA: "next-tip"},
		{Goal: "returned", State: StateReturned, At: at, ReturnedAt: bedNow.Add(-5 * time.Minute).Format(time.RFC3339Nano), Reason: "own test failed"},
		{Goal: "superseded", State: StateSuperseded, At: at},
	}, View: lane.View{Summary: "whole status summary"}}
	batch := &Batch{ID: "batch", State: BatchPrepared, Members: []GoalSHA{{Goal: "selected", SHA: "selected-tip"}}}
	h, err := buildHealth(status, batch, lane.Pause{}, "", nil, nil, b.install, b.checkout, healthSeams(bedNow), b.git(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Goals) != 5 {
		t.Fatalf("goals: %+v", h.Goals)
	}
	reasons := map[string]string{}
	for _, g := range h.Goals {
		reasons[g.Goal] = g.Reason
		if g.State == StateLanded && g.Overdue {
			t.Fatal(g)
		}
	}
	if reasons["waiting"] != "main needs proof" || reasons["returned"] != "own test failed" || !strings.Contains(reasons["selected"], "selected") || !strings.Contains(reasons["next"], "next batch") || strings.Contains(h.Text(), "whole status summary") {
		t.Fatal(h.Text())
	}
	for _, r := range h.Rows() {
		if r.Row == "overdue-goal" && r.Subject == "landed" {
			t.Fatal(r)
		}
	}
	h.Goals = []GoalHealth{{Goal: "landed", State: StateLanded, Overdue: true}}
	if healthRow(h, "overdue-goal") != nil {
		t.Fatal("landed goals fired overdue")
	}
}

func TestLandingHealthStateAndRecoveryRows(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/steward-health-recovery.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, Owner, State, Summary, Move, Row, Act string
		Barren                                      int
		Paused, Proof, Fix, Merge, Open, Stop       bool
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			at := bedNow.Add(-10 * time.Minute).Format(time.RFC3339Nano)
			batch := &Batch{ID: "batch", Base: "old-main", State: BatchPrepared, Open: tc.Open, SealedAt: at, Selector: PolicyValue{Value: "auto"}}
			status := Status{View: lane.View{Owner: lane.OwnerView{State: tc.Owner, Barren: tc.Barren}, Summary: tc.Summary}, Paused: tc.Paused, MainMove: MainMove{Kind: tc.Move}, Queue: []Entry{
				{Goal: "newer", SHA: "new-tip", State: StateWaiting, At: at},
				{Goal: "held", SHA: "held-tip", State: StateWaiting, At: at, Held: true},
				{Goal: "older", SHA: "old-tip", State: StateWaiting, At: bedNow.Add(-20 * time.Minute).Format(time.RFC3339Nano)},
				{Goal: "landed", SHA: "landed-tip", State: StateLanded, At: at},
			}}
			if tc.Stop {
				status.Stop = &Stop{Evidence: "held"}
			}
			if tc.Proof {
				status.RunningProof = &RunningProof{State: "running"}
			}
			if tc.Fix {
				status.RunningFix = &Fix{State: "resolving"}
			}
			s := healthSeams(bedNow)
			s.Git = func(_ string, args ...string) (string, error) {
				if slices.Contains(args, "MERGE_HEAD^{commit}") {
					if tc.Merge {
						return "pending", nil
					}
					return "", exec.CommandContext(t.Context(), "false").Run()
				}
				return "main", nil
			}
			before := filepath.Join(Dir(b.install), "fixes")
			h, err := buildHealth(status, batch, lane.Pause{}, "", nil, nil, b.install, b.checkout, s, b.git(nil, nil))
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(h)
			if err != nil {
				t.Fatal(err)
			}
			var saved Health
			if err = json.Unmarshal(encoded, &saved); err != nil {
				t.Fatal(err)
			}
			if h.Line() != saved.Line() || !slices.Equal(h.Rows(), saved.Rows()) {
				t.Fatalf("serialized health lost facts: %s", encoded)
			}
			if h.State != tc.State || !strings.HasPrefix(h.Line(), "lane "+tc.State) {
				t.Fatalf("state: %s", h.Line())
			}
			if tc.Owner == lane.OwnerHeld && !strings.Contains(h.Line(), "after 2 runs that left the lane unchanged") {
				t.Fatal(h.Line())
			}
			var rows []string
			for _, r := range h.Rows() {
				rows = append(rows, r.Row)
				if r.Row == tc.Row && r.Act != tc.Act {
					t.Fatal(r)
				}
			}
			if tc.Row != "" && !slices.Contains(rows, tc.Row) {
				t.Fatalf("missing %s: %+v", tc.Row, h.Rows())
			}
			if tc.Row == "" && len(rows) > 0 {
				t.Fatal(h.Rows())
			}
			if _, err := os.Stat(before); !os.IsNotExist(err) {
				t.Fatal("health changed merge state", err)
			}
			if len(h.Batch.Waiting) != 3 {
				t.Fatal(h.Batch.Waiting)
			}
		})
	}
}

func TestLandingHealthBarrenHoldRequiresAdmissibleWork(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	h, err := buildHealth(Status{View: lane.View{Owner: lane.OwnerView{State: lane.OwnerHeld, Barren: 2}}, Queue: []Entry{{Goal: "held", State: StateWaiting, Held: true, SHA: "tip"}}}, nil, lane.Pause{}, "", nil, nil, b.install, b.checkout, healthSeams(bedNow), b.git(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	if healthRow(h, "barren-hold") != nil {
		t.Fatal(h.Rows())
	}
	if h.Batch != nil {
		t.Fatal(h.Batch)
	}
}

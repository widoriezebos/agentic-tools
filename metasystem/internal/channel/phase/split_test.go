package phase

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

type unavailableSplitRepository struct{ goal.Repository }

func (unavailableSplitRepository) Capture(string) (string, error) {
	return "", errors.New("remote connection closed")
}

func TestSplitApprovalTickReconcilesHeldChildren(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"unblocked", "refresh error", "missing child"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			ctx, root := context.Background(), t.TempDir()
			now := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
			write := func(path, text string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(root, "metasystem.conf"), "metasystem.runtimes=fake\n")
			record := &goal.RootRecord{Identity: "01J5X000000000000000000000", FormatVersion: "1", SyncMode: goal.SyncRemote, Revision: 1}
			repository := testgoal.New(map[string][]byte{"plans/goals/backlog.md": goal.RenderRoot(record)}, now, strings.Repeat("1", 40))
			endpoint := goal.Endpoint{Root: root, Remote: "origin", Branch: "refs/heads/main", Repository: repository}
			deadlines := 0
			endpoint.ProjectionDeadline = func(wait time.Duration) <-chan time.Time {
				deadlines++
				if wait != 4*time.Second {
					t.Errorf("split retry deadline=%s; want 4s", wait)
				}
				return make(chan time.Time)
			}

			authorization, err := fixtureauth.New(root)
			if err != nil {
				t.Fatal(err)
			}
			proof, err := humanauthority.FixtureGoalProof(root, authorization.GoalHumanAuthority(), now)
			if err != nil {
				t.Fatal(err)
			}
			request := func() goal.VerbRequest {
				t.Helper()
				ulid, err := goal.NewOperationULID()
				if err != nil {
					t.Fatal(err)
				}
				return goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: "test-machine", Lineage: "browser-session", Human: "Wido"}, Authority: &proof, Ulid: ulid, Now: now, ClaimEpoch: 1}
			}
			if result, err := goal.Open(request(), "source", "Build the reader and writer.", "human", "Shape the work."); err != nil || result.Outcome != goal.OutcomeConfirmed {
				t.Fatalf("open: %+v %v", result, err)
			}
			members := []goal.MemberDraft{{ID: "child-one", Intent: "Build the reader.", NextStep: "Write its brief."}, {ID: "child-two", Intent: "Build the writer.", NextStep: "Write its brief.", Blocked: []string{"child-one"}}}
			splitRequest := request()
			result, err := goal.Split(splitRequest, "source", members, goal.SplitRatification{Tier: goal.RatifierHuman, By: "Wido", DraftSHA256: goal.SplitDraftSHA256("source", members)}, &proof)
			if err != nil || result.Outcome != goal.OutcomeConfirmed {
				t.Fatalf("split: %+v %v", result, err)
			}
			transaction := goal.Opid(splitRequest.Ulid, splitRequest.Actor.Machine, splitRequest.Actor.Lineage)
			if err := NotifySplitApproval(ctx, endpoint, result.Tip, "source", transaction, []string{"child-one", "child-two"}, now); err == nil || len(channel.LoadLandedState(root).Pending) != 1 {
				t.Fatalf("initial unconfigured send: %v %+v", err, channel.LoadLandedState(root))
			}
			if scenario == "unblocked" {
				if result, err := goal.Unblock(request(), "child-two", "source", &proof); err != nil || result.Outcome != goal.OutcomeConfirmed {
					t.Fatalf("unblock: %+v %v", result, err)
				}
			} else if scenario == "refresh error" {
				endpoint.Repository = unavailableSplitRepository{repository}
			} else {
				state := channel.LoadLandedState(root)
				state.Pending[0].SplitChildren = append(state.Pending[0].SplitChildren, "missing-child")
				data, err := json.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
				write(filepath.Join(root, "artifacts", "agents", "channel", "landed.json"), string(data))
			}
			var mu sync.Mutex
			var delivered []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/auth.test" {
					_, _ = w.Write([]byte(`{"ok":true,"user_id":"test-bot"}`))
					return
				}
				if r.URL.Path != "/chat.postMessage" {
					t.Errorf("unexpected channel request: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				mu.Lock()
				delivered = append(delivered, r.FormValue("text"))
				mu.Unlock()
				_, _ = w.Write([]byte(`{"ok":true,"channel":"CFAKE","ts":"123.000001"}`))
			}))
			t.Cleanup(server.Close)
			dir := filepath.Join(root, "fake-channel")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			write(filepath.Join(dir, "base-url"), server.URL)
			write(filepath.Join(root, "metasystem.conf"), "metasystem.runtimes=fake\nchannel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.dir="+dir+"\nchannel.destination.fleet.fake.face=slack\nchannel.human.slack.user-id=test-human\nchannel.human.answer-code=off\n")
			tick := func() (int, error) {
				return run(ctx, root, now, func(string) (string, error) { return "test-machine", nil }, func(string) (goal.Endpoint, error) { return endpoint, nil })
			}
			undelivered, err := tick()
			if deadlines != 1 {
				t.Fatalf("split retry used %d fixture deadlines; want 1", deadlines)
			}
			if (err != nil) != (scenario != "unblocked") || (scenario == "unblocked" && undelivered != 0) {
				t.Fatalf("first tick: undelivered=%d err=%v", undelivered, err)
			}
			state := channel.LoadLandedState(root)
			wantError := "remote connection closed"
			if scenario == "missing child" {
				wantError = "cannot read child missing-child"
			}
			if len(state.Pending) != 0 || (scenario != "unblocked" && (len(state.Failed) != 1 || !strings.Contains(state.Failed[0].Error, wantError))) {
				t.Fatalf("retry did not settle: %+v", state)
			}
			if undelivered, err := tick(); err != nil || undelivered != 0 {
				t.Fatalf("second tick: undelivered=%d err=%v", undelivered, err)
			}
			mu.Lock()
			defer mu.Unlock()
			if scenario != "unblocked" {
				if len(delivered) != 0 {
					t.Fatalf("unreadable ledger delivered: %q", delivered)
				}
			} else if len(delivered) != 1 || !strings.Contains(delivered[0], "metasystem goal approve child-one") || strings.Contains(delivered[0], "child-two") || !strings.Contains(delivered[0], "metasystem goal unblock child-one --on source") {
				t.Fatalf("held child's request was not delivered exactly once: %q", delivered)
			}
		})
	}
}

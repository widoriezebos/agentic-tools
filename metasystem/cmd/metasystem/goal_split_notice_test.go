package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestGoalSplitRequestsChildApproval(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"confirmed", "lost confirmation", "delivered recovery", "unlanded split", "no channel", "configure then rerun", "failed send", "failed retry", "one approved", "all approved", "reversed", "unknown retry"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := newGoalCLIBed(t, goalCLISeed{allowTerminalProof: true})
			bed.announceHolder()
			gcliBudgetOpen(t, bed, "source", gcliBudgetTierTwo, "Split the responsibility.", "--blocked-by", "fix-docs")
			plan := filepath.Join(bed.root, "members.md")
			if err := os.WriteFile(plan, []byte("# split source\n\n## member child-one\n- Intent: Build the reader.\n- Next step: Write its brief.\n\n## member child-two\n- Intent: Build the writer.\n- Next step: Write its brief.\n- BlockedBy: child-one\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			var mu sync.Mutex
			attempts := 0
			var delivered []string
			failures := 0
			if strings.Contains(scenario, "failed") || scenario == "one approved" || scenario == "all approved" || scenario == "reversed" || scenario == "unknown retry" {
				failures = 1
			}
			if scenario == "failed retry" {
				failures = 2
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				if r.URL.Path != "/chat.postMessage" {
					t.Errorf("unexpected channel request: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				attempts++
				w.Header().Set("Content-Type", "application/json")
				if attempts <= failures {
					_, _ = w.Write([]byte(`{"ok":false,"error":"rate_limited"}`))
					return
				}
				delivered = append(delivered, r.FormValue("text"))
				_, _ = w.Write([]byte(`{"ok":true,"channel":"CFAKE","ts":"123.000001"}`))
			}))
			t.Cleanup(server.Close)
			configure := func() {
				dir := filepath.Join(bed.root, "fake-channel")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "base-url"), []byte(server.URL), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte("metasystem.runtimes=fake\nchannel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.dir="+dir+"\nchannel.destination.fleet.fake.face=slack\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if scenario != "no channel" && scenario != "configure then rerun" {
				configure()
			}
			transport := &splitConfirmRepository{Repository: bed.repo, loseConfirm: scenario == "lost confirmation", losePublish: scenario == "unlanded split"}
			public := func(argv ...string) (int, string) {
				command, rest, ok := resolveIntentArgv(argv)
				if !ok {
					t.Fatal(argv)
				}
				var out, errOut bytes.Buffer
				owners := bed.owners(&out, &errOut)
				owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
					e, err := bed.endpoint(root)
					e.Repository = transport
					return e, err
				}
				code := runIntentIn(command, rest, &out, &errOut, bed.root, owners)
				return code, out.String() + errOut.String()
			}
			args := []string{"goal", "split", "source", "--plan", plan, "--by", "Wido"}
			code, output := public(args...)
			if scenario == "lost confirmation" || scenario == "unlanded split" {
				mu.Lock()
				calls := attempts
				mu.Unlock()
				if code == 0 || calls != 0 {
					t.Fatalf("unconfirmed split notified: code=%d attempts=%d %s", code, calls, output)
				}
				if scenario == "unlanded split" {
					entries, err := goal.Entries(bed.root)
					if err != nil {
						t.Fatal(err)
					}
					for _, entry := range entries {
						if entry.Intent.Verb == "split" {
							entry.Owner.Pid, entry.Owner.PidStartedAt, entry.Owner.StartTicks, entry.Owner.BootID = 99999999, 1, 0, ""
							writeSplitNoticeEntry(t, bed, entry)
						}
					}
				}
				code, output = public("goal", "sync", "--recover")
				if code != 0 {
					t.Fatalf("recover: %d %s", code, output)
				}
				if scenario == "unlanded split" {
					if bed.accepted("plans/goals/child-one.md") != "" {
						t.Fatal("recovery replayed a person's unlanded split")
					}
					code, output = public(args...)
				}
			}
			if code != 0 {
				t.Fatalf("split: %d %s", code, output)
			}
			mu.Lock()
			immediateAttempts, immediateDelivered := attempts, len(delivered)
			mu.Unlock()
			if failures == 0 && scenario != "no channel" && scenario != "configure then rerun" && (immediateAttempts != 1 || immediateDelivered != 1) {
				t.Fatalf("confirmation did not immediately deliver one notice: attempts=%d delivered=%d", immediateAttempts, immediateDelivered)
			}
			parent, problems := goal.ParseFile([]byte(bed.goalRecord("source")))
			if len(problems) != 0 || parent.State != goal.StateSplit || parent.Split == nil {
				t.Fatalf("split wasn't applied: %+v %v", parent, problems)
			}
			for _, id := range parent.Split.Children {
				child, problems := goal.ParseFile([]byte(bed.goalRecord(id)))
				if len(problems) != 0 || child.Approved != nil || child.State != goal.StateQueued || !containsTestString(child.Blocked, "source") {
					t.Fatalf("notification approved or released child: %+v %v", child, problems)
				}
			}
			if scenario == "no channel" || scenario == "configure then rerun" || failures != 0 {
				if !strings.Contains(output, "not sent") || !strings.Contains(output, "metasystem goal approve child-one") || len(channel.LoadLandedState(bed.root).Pending) != 1 {
					t.Fatalf("failed or absent channel hidden: %s state=%+v", output, channel.LoadLandedState(bed.root))
				}
			}
			if scenario == "configure then rerun" {
				configure()
			}
			if scenario == "delivered recovery" {
				entry, err := goal.ReadEntry(bed.root, parent.Split.Transaction)
				if err != nil {
					t.Fatal(err)
				}
				entry.Phase, entry.Outcome = goal.PhasePushed, ""
				writeSplitNoticeEntry(t, bed, entry)
				if code, output := public("goal", "sync", "--recover"); code != 0 {
					t.Fatalf("delivered recovery: %d %s", code, output)
				}
			}
			if scenario == "one approved" || scenario == "all approved" {
				ids := []string{"child-one"}
				if scenario == "all approved" {
					ids = parent.Split.Children
				}
				for _, id := range ids {
					gcliBudgetHumanMust(t, bed, "goal", "edit", id, "--risk", gcliBudgetTierOne, "--basis", "child risk")
					gcliBudgetHumanMust(t, bed, "goal", "approve", id, "--budget", "norm")
				}
			}
			if scenario == "reversed" {
				gcliLedgerMust(t, bed, "goal", "split", "source", "--reverse", "--reason", "Keep the responsibility together.", "--by", "Wido")
			}
			if scenario == "all approved" || scenario == "reversed" || scenario == "unknown retry" {
				loaded, err := phase.Load(bed.root, false)
				if err != nil {
					t.Fatal(err)
				}
				e, err := bed.endpoint(bed.root)
				if err != nil {
					t.Fatal(err)
				}
				if scenario == "unknown retry" {
					e.Repository = &splitConfirmRepository{Repository: bed.repo, published: true, loseConfirm: true}
				}
				err = channel.RetrySplitApproval(context.Background(), e, loaded.Provider, loaded.Destination, bed.clock())
				if (err != nil) != (scenario == "unknown retry") {
					t.Fatalf("retry: %v", err)
				}
			} else {
				for range 2 {
					if code, output := public(args...); code != 0 {
						t.Fatalf("repeat blocked split: %d %s", code, output)
					}
				}
			}
			mu.Lock()
			defer mu.Unlock()
			wantDelivered, wantAttempts := 1, 1
			if failures > 0 {
				wantAttempts = 2
			}
			if scenario == "no channel" {
				wantDelivered, wantAttempts = 0, 0
			}
			if scenario == "all approved" || scenario == "reversed" || scenario == "unknown retry" {
				wantDelivered, wantAttempts = 0, 1
			}
			if scenario == "failed retry" {
				wantDelivered = 0
			}
			if len(delivered) != wantDelivered || attempts != wantAttempts {
				t.Fatalf("delivery: attempts=%d messages=%q; want %d/%d", attempts, delivered, wantAttempts, wantDelivered)
			}
			state := channel.LoadLandedState(bed.root)
			if scenario == "no channel" {
				if len(state.Pending) != 1 {
					t.Fatal("unconfigured delivery lost its pending request")
				}
			} else if len(state.Pending) != 0 || ((scenario == "failed retry" || scenario == "unknown retry") && len(state.Failed) != 1) {
				t.Fatalf("retry state: %+v", state)
			}
			if len(delivered) != 0 {
				lines := strings.Split(delivered[0], "\n")
				if !strings.Contains(lines[0], "goal source") || !strings.Contains(lines[0], "unapproved and held by source") || !strings.Contains(lines[1], "metasystem goal approve child-two") || !strings.Contains(delivered[0], "metasystem goal unblock child-two --on source") || !strings.Contains(delivered[0], "Remaining prerequisites: child-one, fix-docs.") || !strings.Contains(delivered[0], "Approval alone keeps the source hold.") {
					t.Fatalf("approval message lacks the reason, command or hold explanation: %s", delivered[0])
				}
				if (scenario == "one approved") == strings.Contains(lines[1], "metasystem goal approve child-one") {
					t.Fatalf("approval request is stale or omitted: %s", lines[1])
				}
				t.Logf("delivered once: %s", delivered[0])
			}
		})
	}
}

func writeSplitNoticeEntry(t *testing.T, bed *goalCLIBed, entry goal.Entry) {
	t.Helper()
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bed.root, "artifacts", "agents", "goal-transactions", entry.Opid+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

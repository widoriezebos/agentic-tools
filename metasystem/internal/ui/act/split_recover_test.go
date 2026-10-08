package act

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestUIActRecoversSplitApprovalNotice(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"send", "queue"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := ledger(t)
			openGoal(t, bed, "source")
			openGoal(t, bed, "ui-after")
			authority := sessionFor(t, bed)
			splitAuthority := provenFor(t, bed)
			seam := scripting(bed)
			landed := false
			seam.publish = func(plain goal.Repository, parent, commit string) (goal.CASOutcome, error) {
				outcome, err := plain.Publish(parent, commit)
				landed = err == nil
				return outcome, err
			}
			seam.capture = func(plain goal.Repository, opid string) (string, error) {
				if landed {
					return "", errors.New("remote connection closed during confirmation")
				}
				return plain.Capture(opid)
			}
			members := []goal.MemberDraft{{ID: "child-one", Intent: "Build the reader.", NextStep: "Write its brief."}, {ID: "child-two", Intent: "Build the writer.", NextStep: "Write its brief."}}
			asked := request(t, bed)
			asked.Actor.Human, asked.Authority = "Wido", &splitAuthority.proof
			_, err := goal.Split(asked, "source", members, goal.SplitRatification{Tier: goal.RatifierHuman, By: "Wido", DraftSHA256: goal.SplitDraftSHA256("source", members)}, &splitAuthority.proof)
			if err == nil {
				t.Fatal("split confirmation should have failed")
			}
			operation := goal.Opid(asked.Ulid, asked.Actor.Machine, asked.Actor.Lineage)
			entry, err := goal.ReadEntry(bed.root, operation)
			if err != nil || entry.Phase != goal.PhasePushed || entry.Outcome != "" {
				t.Fatalf("split did not leave an unresolved pushed entry: %+v %v", entry, err)
			}
			seam.capture, seam.publish = nil, nil
			var mu sync.Mutex
			var delivered []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			configure := func() {
				t.Helper()
				dir := filepath.Join(bed.root, "fake-channel")
				write(t, filepath.Join(dir, "base-url"), server.URL, 0o644)
				write(t, filepath.Join(bed.root, "metasystem.conf"), "metasystem.runtimes=fake\nchannel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.dir="+dir+"\nchannel.destination.fleet.fake.face=slack\n", 0o644)
			}
			if scenario == "send" {
				configure()
			}
			if err := authority.Approve("ui-after", box()); err != nil {
				t.Fatalf("UI act did not recover the split: %v", err)
			}
			entry, err = goal.ReadEntry(bed.root, operation)
			if err != nil || entry.Phase != goal.PhaseTerminal || entry.Outcome != goal.OutcomeConfirmed {
				t.Fatalf("split was not confirmed by UI recovery: %+v %v", entry, err)
			}
			state := channel.LoadLandedState(bed.root)
			mu.Lock()
			immediate := len(delivered)
			mu.Unlock()
			if scenario == "send" && (immediate != 1 || len(state.Posted) != 1 || len(state.Pending) != 0) {
				t.Fatalf("UI recovery did not immediately send the owed notice: delivered=%d state=%+v", immediate, state)
			}
			if scenario == "queue" {
				if immediate != 0 || len(state.Pending) != 1 || !strings.Contains(state.Pending[0].Error, "no approval channel configured") {
					t.Fatalf("UI recovery did not queue the owed notice: %+v", state)
				}
				configure()
			}
			loaded, err := phase.Load(bed.root, false)
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := authority.Approve("ui-after", box()); err != nil {
					t.Fatalf("repeat UI act: %v", err)
				}
				if err := channel.RetrySplitApproval(context.Background(), bed.endpoint(), loaded.Provider, loaded.Destination, fixtureNow); err != nil {
					t.Fatalf("channel retry: %v", err)
				}
			}
			state = channel.LoadLandedState(bed.root)
			if len(state.Pending) != 0 || len(state.Failed) != 0 || len(state.Posted) != 1 {
				t.Fatalf("delivery did not settle once: %+v", state)
			}
			for _, id := range []string{"child-one", "child-two"} {
				child := readGoal(t, bed, id)
				if child.Approved != nil || child.State != goal.StateQueued || len(child.Blocked) != 1 || child.Blocked[0] != "source" {
					t.Fatalf("recovery granted approval or removed the hold: %+v", child)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if len(delivered) != 1 || !strings.Contains(delivered[0], "metasystem goal approve child-one; metasystem goal approve child-two") {
				t.Fatalf("UI recovery did not send the owed notice once: %q", delivered)
			}
		})
	}
}

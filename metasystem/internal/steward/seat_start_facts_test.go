package steward

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func TestTheSeatPromptIsWrittenFromTheGoalsRecord(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"facts", "returned", "reader failures"} {
		t.Run(scenario, func(t *testing.T) {
			bed := newSeatBed(t, seatReadyGoal("alpha", "Continue the saved next step."))
			bed.goals["alpha"].NextStep += strings.Repeat("wide界", 1000)
			bed.tips["alpha"] = "goal-tip"
			d := *bed.seatDependencies()
			d.Units = func(string, string) ([]UnitStage, error) {
				return []UnitStage{{Unit: "build", Stage: "built"}, {Unit: "read", Stage: "review refused: BRIEF_REFUSED"}}, nil
			}
			entry := plain.Entry{SHA: "goal-tip", State: plain.StateWaiting}
			d.Lane = func(string, string) (plain.Entry, bool, error) { return entry, true, nil }
			d.Main = func(string) (string, error) { return "main-tip", nil }
			d.Contains = func(_, sha, main string) (bool, error) {
				if sha != "goal-tip" || main != "main-tip" {
					t.Fatal("wrong containment inputs")
				}
				return true, nil
			}
			d.Jobs = func(string) ([]map[string]any, error) {
				return []map[string]any{{"goalId": "alpha", "jobId": "job-open", "role": "critic", "reviews": "commit:goal-tip", "createdAt": "2026-01-01T00:00:00Z", "status": "failed", "error": "dispatch-refused", "refusalClass": "budget", "summary": "No room for a critic."}}, nil
			}
			d.Refusals = func() ([]launch.Refusal, error) {
				return []launch.Refusal{{Goal: "alpha", Kind: "build", Time: "2026-01-01T00:00:00Z", Code: "CLOSED_LAUNCH"}, {Goal: "alpha", Kind: "read", Time: "2026-01-01T00:00:00Z", Code: "OPEN_LAUNCH", Tag: "tag"}}, nil
			}
			d.Launches = func() ([]launch.Record, error) {
				return []launch.Record{{Goal: "alpha", Kind: "build", StartedAt: "2026-01-02T00:00:00Z"}}, nil
			}
			var messages []board.Message
			for i := 0; i < 8; i++ {
				sender, recipient := "counterpart-a", board.Address{Goal: "alpha"}
				if i == 2 {
					recipient = board.Address{Machine: seatBedMachine}
				}
				if i >= 4 {
					sender = "counterpart-b"
				}
				body, _ := json.Marshal(map[string]any{"id": fmt.Sprintf("message-%d", i), "thread": "thread", "kind": "reply", "from": board.Sender{Machine: sender}, "to": recipient, "at": bed.now.Add(time.Duration(i) * time.Minute), "text": fmt.Sprintf("saved-message-%d", i)})
				var message board.Message
				if err := json.Unmarshal(body, &message); err != nil {
					t.Fatal(err)
				}
				messages = append(messages, message)
			}
			d.Threads = func() ([]board.Thread, error) { return []board.Thread{{Messages: messages}}, nil }
			switch scenario {
			case "returned":
				entry.State, entry.Reason = plain.StateReturned, "Repair the recorded return."
			case "reader failures":
				d.Units = func(string, string) ([]UnitStage, error) { return nil, errors.New("units failed") }
			}
			record, err := startSeatWithDependencies(bed.root, SeatSelection{Goal: "alpha"}, d)
			if err != nil || record.LaunchID == "" || len(bed.launcher.starts) != 1 {
				t.Fatalf("seat did not start: %+v %v", record, err)
			}
			data, err := os.ReadFile(bed.launcher.starts[0].Brief)
			if err != nil {
				t.Fatal(err)
			}
			prompt := string(data)
			if len(data) > 12000 || !utf8.Valid(data) {
				t.Fatalf("prompt bound: %d", len(data))
			}
			require := func(text string) {
				t.Helper()
				if !strings.Contains(prompt, text) {
					t.Fatalf("missing %q in %s", text, prompt)
				}
			}
			require(seatBrief(SeatSelection{Goal: "alpha"}, true))
			for _, instruction := range []string{"stop advancing at that boundary", "--note <that note> --no-delegates", "repeat the same handoff command to confirm durable binding", "End your turn once the binding is durable", "remain alive, report the exact error"} {
				require(instruction)
			}
			if scenario == "reader failures" {
				for _, fact := range []string{"Units unavailable; metasystem work status alpha"} {
					require(fact)
				}
				require("job-open")
				return
			}
			require("bytes omitted; metasystem goal show alpha")
			for _, text := range []string{"build: built", "read: review refused: BRIEF_REFUSED", "metasystem work review alpha --work read", "job-open, role critic, refusal budget", "No room for a critic.", "OPEN_LAUNCH", "messages with counterpart-a: 1 omitted", "metasystem agent inbox"} {
				require(text)
			}
			for _, i := range []int{1, 2, 3, 5, 6, 7} {
				require(fmt.Sprintf("saved-message-%d", i))
			}
			if strings.Contains(prompt, "CLOSED_LAUNCH") || strings.Contains(prompt, "saved-message-0") {
				t.Fatal("closed refusal or old message kept")
			}
			switch scenario {
			case "facts":
				require("build: built; landed")
				require("read: review refused: BRIEF_REFUSED; landed")
				require("Continue the saved next step.")
			case "returned":
				require("build: built; returned: Repair the recorded return.")
			}
		})
	}
}

// pageSeatBrief starts selection through StartSeat on the claim-quota ledger
// with one ready goal and one this seat claims, its page read over the
// U1a1 sources when page is set, and returns the brief the seat was given.
// The page's unit reader words its stage apart from the seat's own.
// With blocked, a folder stands where the page is saved.
func pageSeatBrief(t *testing.T, selection SeatSelection, page, blocked bool) string {
	t.Helper()
	// The seat's other goal is claimed with its step due, or, for a ready
	// start beside it, waits at the landing gate for a person's word.
	held := claimQuotaHeld("held-h")
	held.Claimed.Lineage = SeatLineage
	held.History[len(held.History)-1].Opid, held.History[len(held.History)-1].Actor = goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC1", "m1e", SeatLineage), "m1e+"+SeatLineage
	if !selection.Held {
		held = seatLandingGoal("held-h", 3, situationNow.Add(-time.Hour))
		held.Claimed.Machine, held.StopCapability.Machine = "m1e", "m1e"
		for i := len(held.History) - 2; i < len(held.History); i++ {
			held.History[i].Opid, held.History[i].Actor = strings.ReplaceAll(held.History[i].Opid, seatBedMachine, "m1e"), strings.ReplaceAll(held.History[i].Actor, seatBedMachine, "m1e")
		}
		held.Landing.Opid = held.History[len(held.History)-1].Opid
	}
	root := canonicalPath(claimQuotaTickBed(t, nil, held, claimQuotaReady("ready-r", 1, "")))
	testprovider.Register(t, root)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("launch.seat.runtime=claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	launcher := &fakeSeatLauncher{}
	units := func(_, id string) ([]UnitStage, error) { return []UnitStage{{Unit: id + "-unit", Stage: "built"}}, nil }
	cfg := TickConfig{Now: situationNow, Seat: launcher, ProviderHome: testprovider.Home(root), AskHome: t.TempDir(), ClaimLaneState: claimQuotaLane(t, root, nil), Units: units}
	if page {
		cfg.Situation = func(root string, now time.Time) Situation {
			src := withEmptyHost(t, SituationSources{Now: func() time.Time { return now }, LaneHome: func() (string, error) { return t.TempDir(), nil },
				ClaimLaneState: claimQuotaLane(t, root, nil), BoardHome: func() (string, error) { return t.TempDir(), nil },
				Units: func(_, id string, _ goal.Projection, _ time.Time) ([]UnitStage, error) {
					return []UnitStage{{Unit: id + "-unit", Stage: "built, as the page read it"}}, nil
				}})
			return readSituation(t, root, src)
		}
	}
	if blocked {
		if err := os.MkdirAll(filepath.Join(situationPagePath(root), "blocked"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := StartSeat(root, cfg, deadCensus(), selection); err != nil || len(launcher.starts) != 1 {
		t.Fatalf("the seat for %s did not start: %v (%d starts)", selection.Goal, err, len(launcher.starts))
	}
	data, err := os.ReadFile(launcher.starts[0].Brief)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestTheSeatBriefCarriesThePage(t *testing.T) {
	t.Parallel()
	ready, held := SeatSelection{Goal: "ready-r"}, SeatSelection{Goal: "held-h", Held: true}
	t.Run("a ready goal", func(t *testing.T) {
		t.Parallel()
		brief := pageSeatBrief(t, ready, true, false)
		for _, want := range []string{"The steward started you for goal ready-r", "Situation (read 09:00; metasystem system situation --verbose prints all of it):",
			"claim ready-r: ready-r is ready to claim", "the lane is unread", "0 asks: none", "0 open alerts", "Next step: Build it.", "ready-r-unit: built", "Hand-in: none"} {
			if !strings.Contains(brief, want) {
				t.Fatalf("the brief lacks %q:\n%s", want, brief)
			}
		}
		if strings.Contains(brief, "held-h-unit") {
			t.Fatalf("the brief carries another goal's facts:\n%s", brief)
		}
	})
	t.Run("a held goal", func(t *testing.T) {
		t.Parallel()
		brief := pageSeatBrief(t, held, true, false)
		if !strings.Contains(brief, "Situation (read ") || !strings.Contains(brief, "held-h-unit: built, as the page read it") || strings.Contains(brief, "ready-r-unit") {
			t.Fatalf("the held goal's brief lacks its facts from the page:\n%s", brief)
		}
	})
	t.Run("no page", func(t *testing.T) {
		t.Parallel()
		without, unread := pageSeatBrief(t, ready, false, false), pageSeatBrief(t, ready, true, true)
		if strings.Contains(without, "Situation (") || without != unread || !strings.Contains(without, "ready-r-unit: built") {
			t.Fatalf("without a page the brief is not today's facts:\n%s\n---\n%s", without, unread)
		}
	})
}

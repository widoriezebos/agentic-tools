package steward

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
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

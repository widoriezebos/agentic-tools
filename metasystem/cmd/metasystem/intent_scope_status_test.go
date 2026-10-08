package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

func TestPersonScopeStatusAndBoardFollowRestore(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"complete", "red-proof", "question-repair"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			f, path := requiredScopeFixture(t, "")
			f.person = true
			home, err := board.Home()
			if err != nil {
				t.Fatal(err)
			}
			now := f.bed.manager.Now()
			seats := []board.Seat{f.bed.manager.Seat}
			f.owners.delivery.boardView = func(string, time.Time) board.View {
				picture, _ := board.Read(home, seats, nil, now, time.Hour)
				view := board.NewView(seats, picture)
				view.Readable = true
				return view
			}
			served := httpd.New(httpd.Info{Now: func() time.Time { return now },
				Observe: func() snapshot.Observation {
					return snapshot.Observation{State: snapshot.StateRead, Tree: &goal.TreeGoals{Live: map[string]*goal.GoalFile{f.bed.id: f.bed.goalFile(f.bed.id)}}}
				},
				Board: &httpd.BoardSource{Home: home, Seats: func() ([]board.Seat, error) { return seats, nil }, Stall: time.Hour},
			}, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}, nil)
			observe := func(excluded bool, expected string) {
				t.Helper()
				code, stdout, stderr := f.bed.run(f.owners, "work", "status", f.bed.id, "--work", "stopped", "--json")
				if code != 0 {
					t.Fatalf("status exit %d: %s / %s", code, stdout, stderr)
				}
				var result struct {
					Data struct {
						Board string                         `json:"board"`
						Work  []struct{ Work, Stage string } `json:"work"`
					} `json:"data"`
				}
				if err := json.Unmarshal([]byte(stdout), &result); err != nil || len(result.Data.Work) != 1 {
					t.Fatalf("status: %s / %v", stdout, err)
				}
				response := httptest.NewRecorder()
				served.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7878/api/board", nil))
				var payload board.View
				if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &payload) != nil || len(payload.Seats) != 1 || len(payload.Seats[0].Goals) != 1 {
					t.Fatalf("board exit %d: %s", response.Code, response.Body.String())
				}
				for surface, text := range map[string]string{"work status": result.Data.Work[0].Stage, "status board": result.Data.Board, "board API": payload.Seats[0].Goals[0].Text(now, time.Local)} {
					if strings.Contains(text, "person excluded required scope") != excluded || !strings.Contains(text, expected) {
						t.Errorf("%s excluded=%t: %q", surface, excluded, text)
					}
				}
			}
			var questionPath string
			var saved []byte
			if scenario == "red-proof" {
				f.bed.starter.fail["proof"] = true
			}
			if scenario == "question-repair" {
				questions, bad := channel.WalkOpenQuestions(f.bed.stateRoot())
				if len(bad) != 0 || len(questions) == 0 {
					t.Fatalf("questions: %v %v", questions, bad)
				}
				questionPath = filepath.Join(f.bed.stateRoot(), "artifacts", "agents", "channel", "questions", questions[0].ID+".json")
				saved, err = os.ReadFile(questionPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(questionPath, []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"--dispositions", path, "--by", "Wido", "--reason", "Remove required scope"}
			code, result := f.review(t, args...)
			if scenario == "red-proof" {
				if code == 0 {
					t.Fatalf("red proof succeeded: %+v", result)
				}
				observe(false, "drop pending (prepared)")
				return
			}
			if scenario == "question-repair" {
				if code == 0 || !strings.Contains(result.Summary, "repair remains pending") {
					t.Fatalf("question repair: %d %+v", code, result)
				}
				observe(true, "dropped; question repair pending")
				if err := os.WriteFile(questionPath, saved, 0600); err != nil {
					t.Fatal(err)
				}
				code, result = f.review(t, args...)
			}
			if code != 0 {
				t.Fatalf("exclude: %d %+v", code, result)
			}
			retained, err := json.Marshal(f.retained(t))
			if err != nil {
				t.Fatal(err)
			}
			cardPath := filepath.Join(board.Dir(home), seats[0].Machine, f.bed.id+".json")
			card, err := os.ReadFile(cardPath)
			if err != nil {
				t.Fatal(err)
			}
			observe(true, "dropped")
			if code, result := transferPublic(t, f.bed, f.owners, "goal", "scope", "restore", f.bed.id, "stopped", "--by", "Wido"); code != 0 {
				t.Fatalf("restore: %d %+v", code, result)
			}
			observe(false, "dropped")
			currentCard, err := os.ReadFile(cardPath)
			if err != nil || !bytes.Equal(card, currentCard) {
				t.Fatalf("scope restoration rewrote the board card: %v", err)
			}
			after, err := json.Marshal(f.retained(t))
			if err != nil || !bytes.Equal(retained, after) {
				t.Fatalf("scope restoration rewrote the retained run: %v", err)
			}
		})
	}
}

package missionrunner

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testTurn() Turn {
	return Turn{
		TurnID:      "demo-t3-ab12",
		MissionID:   "demo",
		Cycle:       json.Number("3"),
		Runtime:     "fake",
		Model:       "fixture",
		HostSession: nil,
	}
}

func testReturnDoc(turn Turn) map[string]any {
	return map[string]any{
		"turnId":                 turn.TurnID,
		"missionId":              turn.MissionID,
		"cycle":                  3,
		"dispatched":             []any{},
		"certified":              []any{},
		"streamUpdatesRequested": []any{},
		"askCandidates":          []any{},
		"factsForLedger":         []any{},
		"gaps":                   []any{},
		"identity":               map[string]any{"runtime": turn.Runtime, "model": turn.Model, "sessionId": turn.HostSession},
	}
}

func validResult(turnDir string) map[string]any {
	return map[string]any{
		"sessionId":  "sess-1",
		"outcome":    "completed",
		"usage":      map[string]any{},
		"rawPath":    filepath.Join(turnDir, "raw.out"),
		"returnPath": filepath.Join(turnDir, "return.json"),
	}
}

func passChecker(string) error { return nil }

func TestValidateReturnAccepts(t *testing.T) {
	turnDir := t.TempDir()
	turn := testTurn()
	writeJSONFile(t, filepath.Join(turnDir, "return.json"), testReturnDoc(turn))
	checked := ""
	validation, err := ValidateReturn(turn, validResult(turnDir), turnDir, func(path string) error {
		checked = path
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == "" || validation.ReturnPath != checked {
		t.Fatalf("completeness check must run on the contained return path, got %q", checked)
	}
	if validation.Returned["turnId"] != turn.TurnID {
		t.Fatal("validated return must be the parsed document")
	}
}

func TestValidateReturnRefusals(t *testing.T) {
	turnDir := t.TempDir()
	turn := testTurn()
	writeJSONFile(t, filepath.Join(turnDir, "return.json"), testReturnDoc(turn))

	extra := validResult(turnDir)
	extra["surprise"] = true
	if _, err := ValidateReturn(turn, extra, turnDir, passChecker); err == nil || err.Error() != "host result has missing or unexpected fields" {
		t.Fatalf("extra field: got %v", err)
	}

	running := validResult(turnDir)
	running["outcome"] = "running"
	if _, err := ValidateReturn(turn, running, turnDir, passChecker); err == nil || !strings.Contains(err.Error(), "host result outcome is not completed") {
		t.Fatalf("outcome: got %v", err)
	}

	escape := validResult(turnDir)
	escape["returnPath"] = filepath.Join(turnDir, "..", "outside.json")
	if _, err := ValidateReturn(turn, escape, turnDir, passChecker); err == nil || err.Error() != "host result returnPath escapes the turn directory" {
		t.Fatalf("escape: got %v", err)
	}

	if _, err := ValidateReturn(turn, validResult(turnDir), turnDir, func(string) error {
		return fmt.Errorf("orchestrator return is invalid: schema violation")
	}); err == nil || !strings.Contains(err.Error(), "schema violation") {
		t.Fatalf("checker refusal must surface verbatim, got %v", err)
	}
}

func TestValidateReturnIdentity(t *testing.T) {
	turn := testTurn()
	cases := []struct {
		name    string
		mutate  func(doc map[string]any)
		message string
	}{
		{"turn id", func(doc map[string]any) { doc["turnId"] = "other" }, "identity mismatch at turnId"},
		{"cycle", func(doc map[string]any) { doc["cycle"] = 4 }, "identity mismatch at cycle"},
		{"identity shape", func(doc map[string]any) { doc["identity"] = "orchestrator" }, "identity is missing"},
		{"model", func(doc map[string]any) {
			doc["identity"].(map[string]any)["model"] = "other"
		}, "runtime/model identity mismatch"},
		{
			// The orchestrator attests what the prompt told it — null on a
			// first turn — never the session id the adapter discovered,
			// which the model cannot know.
			"discovered session",
			func(doc map[string]any) { doc["identity"].(map[string]any)["sessionId"] = "sess-1" },
			"session identity mismatch",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			turnDir := t.TempDir()
			doc := testReturnDoc(turn)
			tc.mutate(doc)
			writeJSONFile(t, filepath.Join(turnDir, "return.json"), doc)
			_, err := ValidateReturn(turn, validResult(turnDir), turnDir, passChecker)
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("want %q, got %v", tc.message, err)
			}
		})
	}
}

func adjudicationState() map[string]any {
	return map[string]any{
		"streams": map[string]any{
			"s-app": map[string]any{"state": "active", "reason": nil},
			"s-db":  map[string]any{"state": "parked-reserved", "reason": "reserved decision"},
		},
	}
}

func TestAdjudicate(t *testing.T) {
	root := t.TempDir()
	mission := "demo"
	turn := testTurn()
	writeJSONFile(t, filepath.Join(jobsDirPath(root), "job-good.json"), map[string]any{"jobId": "job-good", "mission": mission, "turnId": turn.TurnID})
	writeJSONFile(t, filepath.Join(jobsDirPath(root), "job-foreign.json"), map[string]any{"jobId": "job-foreign", "mission": "other", "turnId": turn.TurnID})
	writeJSONFile(t, filepath.Join(jobsDirPath(root), "job-stale.json"), map[string]any{"jobId": "job-stale", "mission": mission, "turnId": "demo-t2-old"})
	writeJSONFile(t, filepath.Join(asksDirPath(root, mission), "prior.json"), map[string]any{"askId": "prior", "answeredAt": nil})
	writeJSONFile(t, filepath.Join(asksDirPath(root, mission), "answered.json"), map[string]any{"askId": "answered", "answeredAt": "2026-08-01T00:00:00Z"})

	returned := map[string]any{
		"dispatched": []any{
			map[string]any{"jobId": "job-good", "role": "implementer", "stream": "s-app"},
			map[string]any{"jobId": "job-foreign", "role": "implementer", "stream": "s-app"},
			map[string]any{"jobId": "job-stale", "role": "implementer", "stream": "gone"},
		},
		"streamUpdatesRequested": []any{
			map[string]any{"streamId": "s-app", "requestedState": "parked-reserved", "reason": "needs a decision"},
			map[string]any{"streamId": "s-db", "requestedState": "active", "reason": "wishful"},
			map[string]any{"streamId": "ghost", "requestedState": "done", "reason": ""},
		},
		"askCandidates": []any{
			map[string]any{"streamId": "s-app", "reasonClass": "red-test", "question": "line one\nline two"},
			map[string]any{"streamId": "s-app", "reasonClass": "bogus", "question": "?"},
		},
	}
	state := adjudicationState()
	verdict, err := Adjudicate(root, mission, turn, state, returned, "2026-08-10T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}

	if len(verdict.Accepted) != 3 {
		t.Fatalf("accepted: got %d items: %v", len(verdict.Accepted), verdict.Accepted)
	}
	if verdict.Accepted[0]["kind"] != "dispatched" || verdict.Accepted[1]["kind"] != "streamUpdate" {
		t.Fatalf("accepted kinds out of order: %v", verdict.Accepted)
	}
	if verdict.Accepted[2]["kind"] != "askCandidate" || verdict.Accepted[2]["askId"] != "ask-3-1" {
		t.Fatalf("accepted ask candidate: %v", verdict.Accepted[2])
	}

	wantReasons := []string{
		"job record is not stamped for this mission",
		"job record was not created during this host turn",
		"illegal stream transition parked-reserved to active",
		"stream does not exist",
		"reason class is unknown",
	}
	if len(verdict.Rejected) != len(wantReasons) {
		t.Fatalf("rejected: got %d items: %v", len(verdict.Rejected), verdict.Rejected)
	}
	for i, want := range wantReasons {
		if verdict.Rejected[i]["reason"] != want {
			t.Fatalf("rejected[%d]: got %v, want %q", i, verdict.Rejected[i]["reason"], want)
		}
		if verdict.Rejected[i]["askId"] != fmt.Sprintf("rejected-3-%d", i+1) {
			t.Fatalf("rejected[%d] askId: got %v", i, verdict.Rejected[i]["askId"])
		}
	}

	app := verdict.Streams["s-app"].(map[string]any)
	if app["state"] != "parked-reserved" || app["reason"] != "needs a decision" {
		t.Fatalf("accepted stream update not applied: %v", app)
	}
	db := verdict.Streams["s-db"].(map[string]any)
	if db["state"] != "parked-reserved" {
		t.Fatalf("rejected unpark must leave the stream parked: %v", db)
	}

	// Six asks: the accepted candidate plus one per rejection. With every
	// stream parked by now, rejections without a live stream land on the
	// first stream in id order.
	if len(verdict.Asks) != 6 {
		t.Fatalf("asks: got %d: %v", len(verdict.Asks), verdict.Asks)
	}
	if verdict.Asks[0]["question"] != "line one line two" {
		t.Fatalf("ask question must flatten newlines: %v", verdict.Asks[0]["question"])
	}
	byID := map[string]map[string]any{}
	for _, ask := range verdict.Asks {
		byID[ask["askId"].(string)] = ask
		if ask["answeredAt"] != nil || ask["answer"] != nil || ask["createdAt"] != "2026-08-10T00:00:00Z" {
			t.Fatalf("ask record shape: %v", ask)
		}
	}
	if byID["rejected-3-1"]["streamId"] != "s-app" || byID["rejected-3-2"]["streamId"] != "s-app" {
		t.Fatalf("dispatched rejections: %v %v", byID["rejected-3-1"], byID["rejected-3-2"])
	}
	if byID["rejected-3-3"]["streamId"] != "s-db" || byID["rejected-3-4"]["streamId"] != "s-app" {
		t.Fatalf("stream rejections: %v %v", byID["rejected-3-3"], byID["rejected-3-4"])
	}
	if byID["rejected-3-4"]["reasonClass"] != "host-failure" {
		t.Fatalf("rejection asks carry host-failure: %v", byID["rejected-3-4"])
	}
	question := byID["rejected-3-3"]["question"].(string)
	if !strings.HasPrefix(question, "Runner rejected host return streamUpdate: illegal stream transition") ||
		!strings.HasSuffix(question, "Review the return before proceeding.") {
		t.Fatalf("rejection question: %q", question)
	}

	wantWaiting := []string{"ask-3-1", "prior", "rejected-3-1", "rejected-3-2", "rejected-3-3", "rejected-3-4", "rejected-3-5"}
	if !reflect.DeepEqual(verdict.WaitingList, wantWaiting) {
		t.Fatalf("waiting list: got %v, want %v", verdict.WaitingList, wantWaiting)
	}
}

func TestAdjudicateAskIDCollision(t *testing.T) {
	root := t.TempDir()
	mission := "demo"
	turn := testTurn()
	// An ask with the natural next id already exists on disk, so allocation
	// must step past it instead of proposing an overwrite.
	writeJSONFile(t, filepath.Join(asksDirPath(root, mission), "ask-3-1.json"), map[string]any{"askId": "ask-3-1", "answeredAt": nil})
	returned := map[string]any{
		"dispatched":             []any{},
		"streamUpdatesRequested": []any{},
		"askCandidates": []any{
			map[string]any{"streamId": "s-app", "reasonClass": "red-test", "question": "?"},
		},
	}
	verdict, err := Adjudicate(root, mission, turn, adjudicationState(), returned, "2026-08-10T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(verdict.Asks) != 1 || verdict.Asks[0]["askId"] != "ask-3-1-2" {
		t.Fatalf("collision must allocate the next suffix: %v", verdict.Asks)
	}
}

func TestAdjudicateNoOpTransitionIsLegal(t *testing.T) {
	root := t.TempDir()
	turn := testTurn()
	returned := map[string]any{
		"dispatched": []any{},
		"streamUpdatesRequested": []any{
			map[string]any{"streamId": "s-db", "requestedState": "parked-reserved", "reason": "still waiting"},
		},
		"askCandidates": []any{},
	}
	verdict, err := Adjudicate(root, "demo", turn, adjudicationState(), returned, "2026-08-10T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(verdict.Accepted) != 1 || len(verdict.Rejected) != 0 {
		t.Fatalf("a same-state request refreshes the reason: %v / %v", verdict.Accepted, verdict.Rejected)
	}
	if verdict.Streams["s-db"].(map[string]any)["reason"] != "still waiting" {
		t.Fatalf("reason not refreshed: %v", verdict.Streams["s-db"])
	}
}

func TestAdjudicateParkedRequestNeedsReason(t *testing.T) {
	root := t.TempDir()
	turn := testTurn()
	returned := map[string]any{
		"dispatched": []any{},
		"streamUpdatesRequested": []any{
			map[string]any{"streamId": "s-app", "requestedState": "parked-stop-loss", "reason": ""},
		},
		"askCandidates": []any{},
	}
	verdict, err := Adjudicate(root, "demo", turn, adjudicationState(), returned, "2026-08-10T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(verdict.Rejected) != 1 || verdict.Rejected[0]["reason"] != "parked stream request has no reason" {
		t.Fatalf("got %v", verdict.Rejected)
	}
	if verdict.Streams["s-app"].(map[string]any)["state"] != "active" {
		t.Fatal("rejected update must not change the stream")
	}
}

package mission

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAuthorizeCapRecordsTheSettingThatBindsTheCap: the resolution a job
// carries names the signed key its cap came from and that key's minutes, so
// a later timeout can ask for the setting that actually ran out.
func TestAuthorizeCapRecordsTheSettingThatBindsTheCap(t *testing.T) {
	repo, mission := fenceEnv(t)
	pair, err := AuthorizeCap(repo, repo, mission, "job-pair", "codex", "gpt-5-6-sol", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	source, _ := pair["source"].(map[string]any)
	if source["key"] != "cap.min.codex.gpt-5-6-sol" || source["signedMin"] != int64(180) {
		t.Fatalf("pair resolution source = %v, want key cap.min.codex.gpt-5-6-sol signedMin 180", source)
	}
	lower := 20
	fallback, err := AuthorizeCap(repo, repo, mission, "job-default", "claude", "claude-fable-5", "", &lower)
	if err != nil {
		t.Fatal(err)
	}
	source, _ = fallback["source"].(map[string]any)
	if source["key"] != "fence.job-cap-min" || source["signedMin"] != int64(240) || source["origin"] != "argument" {
		t.Fatalf("default resolution source = %v, want key fence.job-cap-min signedMin 240 origin argument", source)
	}
}

// TestRefuseBudgetCapNamesTheSettingThatRanOut (H1): a timeout at a signed
// pair cap asks for that pair key, not the mission-wide job-cap-min; the ask
// says exactly how to raise it; a timeout at a job's own lower --cap-min asks
// nothing because the contract still allows more.
func TestRefuseBudgetCapNamesTheSettingThatRanOut(t *testing.T) {
	for _, test := range []struct {
		name       string
		resolution map[string]any
		wantToken  string
		wantKey    string
		absent     string
	}{
		{name: "signed pair cap", resolution: map[string]any{"rule": "contract-pair", "origin": "contract", "key": "cap.min.codex.gpt-5-6-sol", "signedMin": 180, "requestedMin": 180},
			wantToken: "cap.min.codex.gpt-5-6-sol", wantKey: "cap.min.codex.gpt-5-6-sol", absent: "`job-cap-min`"},
		{name: "mission default cap", resolution: map[string]any{"rule": "fence-default", "origin": "contract", "key": "fence.job-cap-min", "signedMin": 240, "requestedMin": 240},
			wantToken: "job-cap-min", wantKey: "fence.job-cap-min"},
		{name: "argument at the signed pair cap", resolution: map[string]any{"rule": "argument", "origin": "argument", "key": "cap.min.codex.gpt-5-6-sol", "signedMin": 180, "requestedMin": 180},
			wantToken: "cap.min.codex.gpt-5-6-sol", wantKey: "cap.min.codex.gpt-5-6-sol"},
		{name: "wall-clock truncation", resolution: map[string]any{"rule": "contract-pair", "origin": "contract", "key": "cap.min.codex.gpt-5-6-sol", "signedMin": 180, "requestedMin": 180, "truncatedBy": "wall-clock"},
			wantToken: "wall-clock-hours", wantKey: "fence.wall-clock-hours", absent: "cap.min.codex"},
		{name: "resolution from before keys were recorded", resolution: map[string]any{"truncatedBy": nil},
			wantToken: "job-cap-min", wantKey: "fence.job-cap-min"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, mission := fenceEnv(t)
			if err := RefuseBudgetCap(repo, mission, "job-1", test.resolution, nil); err != nil {
				t.Fatal(err)
			}
			ask, err := readJSONObjectFile(filepath.Join(missionDir(repo, mission), "asks", "fence-bound.json"))
			if err != nil {
				t.Fatalf("no fence ask: %v", err)
			}
			question, _ := ask["question"].(string)
			contractPath := "plans/mission-demo.contract.md"
			for _, want := range []string{"`" + test.wantToken + "`", test.wantKey, contractPath,
				"metasystem mission seal demo", "metasystem mission resume demo"} {
				if !strings.Contains(question, want) {
					t.Fatalf("ask question lacks %q:\n%s", want, question)
				}
			}
			if test.absent != "" && strings.Contains(question, test.absent) {
				t.Fatalf("ask question names %q, which did not run out:\n%s", test.absent, question)
			}
		})
	}

	t.Run("a job's own lower cap asks nothing", func(t *testing.T) {
		repo, mission := fenceEnv(t)
		resolution := map[string]any{"rule": "argument", "origin": "argument", "key": "cap.min.codex.gpt-5-6-sol", "signedMin": 180, "requestedMin": 20}
		if err := RefuseBudgetCap(repo, mission, "job-1", resolution, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(missionDir(repo, mission), "asks", "fence-bound.json")); !os.IsNotExist(err) {
			t.Fatalf("a timeout below the signed cap raised a fence ask: %v", err)
		}
	})

	t.Run("a pair key survives batching with an open ask", func(t *testing.T) {
		repo, mission := fenceEnv(t)
		if _, err := Refuse(repo, mission, "jobs"); err != nil {
			t.Fatal(err)
		}
		resolution := map[string]any{"rule": "contract-pair", "origin": "contract", "key": "cap.min.codex.gpt-5-6-sol", "signedMin": 180, "requestedMin": 180}
		if err := RefuseBudgetCap(repo, mission, "job-1", resolution, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := Refuse(repo, mission, "cycles"); err != nil {
			t.Fatal(err)
		}
		ask, err := readJSONObjectFile(filepath.Join(missionDir(repo, mission), "asks", "fence-bound.json"))
		if err != nil {
			t.Fatal(err)
		}
		question, _ := ask["question"].(string)
		for _, want := range []string{"`jobs`", "`cycles`", "`cap.min.codex.gpt-5-6-sol`", "fence.jobs", "fence.cycles"} {
			if !strings.Contains(question, want) {
				t.Fatalf("batched ask lost %q:\n%s", want, question)
			}
		}
	})
}

package delegation

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

// These tests carry the assertions the retired `job critique-close` and
// `job critique-read-admission` command tests made, against the delegate
// lifecycle's in-process entries that replaced those verbs.

// Authority-bearing flags parse strictly: a repeated flag, in either
// spelling, names itself; distinct flags pass.
func TestRepeatedFlagNamesTheFirstRepeatedAuthorityFlag(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--repo", "r", "--root-job", "authorized", "--root-job", "redirected"}, "root-job"},
		{[]string{"--repo", "r", "--root-job", "authorized", "--repo", "other"}, "repo"},
		{[]string{"--root-job=authorized", "--root-job", "redirected"}, "root-job"},
		{[]string{"-round", "1", "--round=2"}, "round"},
		{[]string{"--repo", "r", "--role", "code-critic", "--root-job", "candidate", "--round", "1"}, ""},
	} {
		if got := repeatedFlag(tc.args); got != tc.want {
			t.Fatalf("repeatedFlag(%q) = %q, want %q", tc.args, got, tc.want)
		}
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exit *Exit
	if errors.As(err, &exit) {
		return exit.Code
	}
	return -1
}

func writeJSONFixture(t *testing.T, dir, name string, value any) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The critic read admission writes its structured result in every outcome
// and relays the owner's decision as the exit code.
func TestCritiqueReadAdmissionWritesItsResultInEveryOutcome(t *testing.T) {
	t.Parallel()
	subject := dispatch.ReadSubject{
		Kind:                dispatch.SubjectLive,
		ImplementerRoot:     "implementer",
		ReviewedMember:      "implementer",
		ReviewedProjectTree: strings.Repeat("a", 40),
		DiffDigest:          strings.Repeat("b", 64),
	}
	readResult := func(t *testing.T, path string) dispatch.ReadAdmissionResult {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var result dispatch.ReadAdmissionResult
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	seedRead := func(t *testing.T, repo, root, status string, clean bool) {
		t.Helper()
		jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
		writeJSONFixture(t, jobs, "implementer.json", map[string]any{
			"jobId": "implementer", "role": "implementer", "round": 1,
			"parentJob": nil, "status": "completed",
		})
		record := map[string]any{
			"jobId": root, "role": "code-critic", "round": 1,
			"parentJob": nil, "status": status, "reviews": "implementer",
			"findingRegister": []any{}, "findingRegisterRound": 0,
		}
		if clean {
			record["findingRegisterRound"] = 1
			record["findingRegisterSubjectDigest"] = subject.Digest()
			record["cleanReadRounds"] = []any{map[string]any{"round": 1, "subject": subject}}
		}
		writeJSONFixture(t, jobs, root+".json", record)
		roundDir := filepath.Join(repo, "artifacts", "agents", root, "rounds", "1")
		writeJSONFixture(t, roundDir, "subject.json", subject)
		writeJSONFixture(t, roundDir, "return.json", map[string]any{
			"schemaVersion": 3, "jobId": root, "round": 1,
			"reviewedTree": subject.ReviewedProjectTree,
			"findings":     []any{}, "rigor": []any{},
		})
	}
	type admission struct {
		code   int
		stderr string
		result string
	}
	run := func(t *testing.T, seed func(repo string), role, rootJob string, round int64, subjectFile string) admission {
		t.Helper()
		s, stderr := internalSession(t, Ports{})
		if seed != nil {
			seed(s.root)
		}
		if subjectFile == "" {
			subjectFile = writeJSONFixture(t, t.TempDir(), "subject.json", subject)
		}
		resultFile := filepath.Join(t.TempDir(), "result.json")
		err := s.critiqueReadAdmission(role, rootJob, "", round, subjectFile, resultFile)
		return admission{code: exitCode(err), stderr: stderr.String(), result: resultFile}
	}

	t.Run("admitted", func(t *testing.T) {
		t.Parallel()
		got := run(t, nil, "code-critic", "candidate", 1, "")
		result := readResult(t, got.result)
		if got.code != 0 || got.stderr != "" || result.Decision != "ADMITTED" || result.SubjectDigest != subject.Digest() {
			t.Fatalf("admitted = %+v result %+v", got, result)
		}
	})

	t.Run("redundant", func(t *testing.T) {
		t.Parallel()
		var repo string
		got := run(t, func(root string) { repo = root; seedRead(t, root, "prior", "completed", true) }, "code-critic", "candidate", 1, "")
		result := readResult(t, got.result)
		if got.code != 11 || !strings.Contains(got.stderr, "REDUNDANT_READ") || result.Decision != "REDUNDANT_READ" ||
			result.CriticRoot != "prior" || result.Round != 1 || result.EventID == "" || !result.EventRecorded || !result.EventDurable {
			t.Fatalf("redundant = %+v result %+v", got, result)
		}
		logged, err := os.ReadFile(filepath.Join(repo, "artifacts", "agents", "prior", "reads-refused.jsonl"))
		lines := strings.Split(strings.TrimSpace(string(logged)), "\n")
		var event dispatch.ReadRefusal
		if err == nil && len(lines) == 1 {
			err = json.Unmarshal([]byte(lines[0]), &event)
		}
		if err != nil || len(lines) != 1 || event.ID != result.EventID {
			t.Fatalf("recorded events = %q, %v; result id %q", logged, err, result.EventID)
		}
	})

	t.Run("concurrent", func(t *testing.T) {
		t.Parallel()
		got := run(t, func(root string) { seedRead(t, root, "outstanding", "running", false) }, "code-critic", "candidate", 1, "")
		result := readResult(t, got.result)
		if got.code != 11 || !strings.Contains(got.stderr, "CONCURRENT_READ") || result.Decision != "CONCURRENT_READ" ||
			result.CriticRoot != "outstanding" || result.Round != 1 || result.EventRecorded || result.EventID != "" {
			t.Fatalf("concurrent = %+v result %+v", got, result)
		}
	})

	t.Run("event-write-failure", func(t *testing.T) {
		t.Parallel()
		got := run(t, func(root string) {
			seedRead(t, root, "prior", "completed", true)
			if err := os.Mkdir(filepath.Join(root, "artifacts", "agents", "prior", "reads-refused.jsonl"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, "code-critic", "candidate", 1, "")
		result := readResult(t, got.result)
		if got.code != 11 || !strings.Contains(got.stderr, "refusal event was not recorded") || result.Decision != "REDUNDANT_READ" ||
			result.EventID == "" || result.EventRecorded || result.EventDurable {
			t.Fatalf("event failure = %+v result %+v", got, result)
		}
	})

	t.Run("malformed-or-unreadable-subject", func(t *testing.T) {
		t.Parallel()
		inputs := t.TempDir()
		for name, subjectFile := range map[string]string{
			"malformed-json":    writeRaw(t, inputs, "json.json", "{"),
			"malformed-subject": writeRaw(t, inputs, "subject.json", `{"kind":"live"}`),
			"unreadable":        filepath.Join(inputs, "missing.json"),
		} {
			got := run(t, nil, "code-critic", "candidate", 1, subjectFile)
			_ = readResult(t, got.result)
			if got.code == 0 {
				t.Fatalf("%s subject was admitted", name)
			}
		}
	})

	t.Run("invalid-coordinates", func(t *testing.T) {
		t.Parallel()
		for name, values := range map[string]struct {
			role, root string
			round      int64
		}{
			"invalid-id":    {"code-critic", "Bad_ID", 1},
			"invalid-round": {"code-critic", "candidate", 0},
			"role-mismatch": {"design-critic", "candidate", 1},
		} {
			got := run(t, nil, values.role, values.root, values.round, "")
			_ = readResult(t, got.result)
			if got.code == 0 {
				t.Fatalf("%s request was admitted", name)
			}
		}
	})

	t.Run("result-write-failure", func(t *testing.T) {
		t.Parallel()
		s, _ := internalSession(t, Ports{})
		subjectFile := writeJSONFixture(t, t.TempDir(), "subject.json", subject)
		if err := s.critiqueReadAdmission("code-critic", "candidate", "", 1, subjectFile, t.TempDir()); exitCode(err) == 0 {
			t.Fatal("a result-file failure admitted the read")
		}
	})
}

func writeRaw(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

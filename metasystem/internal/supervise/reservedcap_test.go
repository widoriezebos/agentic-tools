package supervise

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newAgentsDir lays out root/artifacts/agents with jobs/ and returns the
// agents path, so the fixture-fence's Dir(Dir(agents)) lands on root.
func newAgentsDir(t *testing.T) string {
	t.Helper()
	agents := filepath.Join(t.TempDir(), "artifacts", "agents")
	if err := os.MkdirAll(filepath.Join(agents, "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	return agents
}

func writeAgentsFile(t *testing.T, agents, rel, content string) {
	t.Helper()
	path := filepath.Join(agents, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBlockingReservedCapDecisions(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		ceiling int64
		wantJob string
		wantCap int64
		blocked bool
		refusal string
	}{
		{name: "empty checkout clears", ceiling: 60},
		{
			name:    "live job at ceiling blocks",
			files:   map[string]string{"jobs/job-a.json": `{"jobId":"job-a","capMin":60,"status":"running"}`},
			ceiling: 60, wantJob: "job-a", wantCap: 60, blocked: true,
		},
		{
			name:    "terminal job does not block",
			files:   map[string]string{"jobs/job-a.json": `{"jobId":"job-a","capMin":90,"status":"completed"}`},
			ceiling: 60,
		},
		{
			name:    "cap below ceiling does not block",
			files:   map[string]string{"jobs/job-a.json": `{"jobId":"job-a","capMin":30,"status":"running"}`},
			ceiling: 60,
		},
		{
			name:    "null capMin is generationless and skipped",
			files:   map[string]string{"jobs/job-a.json": `{"jobId":"job-a","capMin":null,"status":"running"}`},
			ceiling: 60,
		},
		{
			name:    "malformed capMin refuses",
			files:   map[string]string{"jobs/job-a.json": `{"jobId":"job-a","capMin":"ninety","status":"running"}`},
			ceiling: 60, refusal: "capMin is malformed",
		},
		{
			name:    "unparsable job record refuses",
			files:   map[string]string{"jobs/job-a.json": `{broken`},
			ceiling: 60, refusal: "job record unreadable",
		},
		{
			name:    "record without jobId blocks under its file stem",
			files:   map[string]string{"jobs/stem-job.json": `{"capMin":75,"status":"running"}`},
			ceiling: 60, wantJob: "stem-job", wantCap: 75, blocked: true,
		},
		{
			name: "fence reservation with no job record blocks",
			files: map[string]string{
				"missions/m1/fences.json": `{"reservations":{"job-r":{"capMin":80}}}`,
			},
			ceiling: 60, wantJob: "job-r", wantCap: 80, blocked: true,
		},
		{
			name: "fence reservation for a terminal job does not block",
			files: map[string]string{
				"jobs/job-r.json":         `{"jobId":"job-r","status":"failed"}`,
				"missions/m1/fences.json": `{"reservations":{"job-r":{"capMin":80}}}`,
			},
			ceiling: 60,
		},
		{
			name: "highest cap wins across record and fence",
			files: map[string]string{
				"jobs/job-a.json":         `{"jobId":"job-a","capMin":70,"status":"running"}`,
				"missions/m1/fences.json": `{"reservations":{"job-a":{"capMin":95}}}`,
			},
			ceiling: 60, wantJob: "job-a", wantCap: 95, blocked: true,
		},
		{
			name: "equal caps tie to the lexically first job",
			files: map[string]string{
				"jobs/job-b.json": `{"jobId":"job-b","capMin":90,"status":"running"}`,
				"jobs/job-a.json": `{"jobId":"job-a","capMin":90,"status":"pending"}`,
			},
			ceiling: 60, wantJob: "job-a", wantCap: 90, blocked: true,
		},
		{
			name: "malformed reservation refuses",
			files: map[string]string{
				"missions/m1/fences.json": `{"reservations":{"job-r":"not-an-object"}}`,
			},
			ceiling: 60, refusal: "reservation job-r is malformed",
		},
		{
			name: "reservation without integral capMin refuses",
			files: map[string]string{
				"missions/m1/fences.json": `{"reservations":{"job-r":{"capMin":80.5}}}`,
			},
			ceiling: 60, refusal: "reservation job-r capMin is malformed",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			agents := newAgentsDir(t)
			for rel, content := range c.files {
				writeAgentsFile(t, agents, rel, content)
			}
			blocker, blocked, err := BlockingReservedCap(agents, c.ceiling)
			if c.refusal != "" {
				if err == nil || !strings.Contains(err.Error(), c.refusal) {
					t.Fatalf("want refusal containing %q, got blocker=%+v blocked=%v err=%v", c.refusal, blocker, blocked, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected refusal: %v", err)
			}
			if blocked != c.blocked || blocker.Job != c.wantJob || (c.blocked && blocker.Cap != c.wantCap) {
				t.Fatalf("want (%q,%d,%v), got (%q,%d,%v)", c.wantJob, c.wantCap, c.blocked, blocker.Job, blocker.Cap, blocked)
			}
		})
	}
}

// TestBlockingReservedCapUnreadableRecordRefuses proves the fail-closed rule
// for a record that exists but cannot be read: a directory named like a job
// record is neither absent nor parsable, so arming refuses.
func TestBlockingReservedCapUnreadableRecordRefuses(t *testing.T) {
	agents := newAgentsDir(t)
	if err := os.MkdirAll(filepath.Join(agents, "jobs", "job-dir.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := BlockingReservedCap(agents, 60); err == nil || !strings.Contains(err.Error(), "job record unreadable") {
		t.Fatalf("a directory posing as a job record must refuse arming, got %v", err)
	}
}

// TestBlockingReservedCapFixtureFence proves the identity-fixture fence: a
// leaked METASYSTEM_FAKE_PROCESS_IDENTITY_FILE refuses arming unless the
// checkout's configured runtime is fake.
func TestBlockingReservedCapFixtureFence(t *testing.T) {
	agents := newAgentsDir(t)
	root := filepath.Dir(filepath.Dir(agents))
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", filepath.Join(root, "id.json"))

	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := BlockingReservedCap(agents, 60); err == nil || !strings.Contains(err.Error(), "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE is set") {
		t.Fatalf("a leaked fixture on a non-fake checkout must refuse arming, got %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, blocked, err := BlockingReservedCap(agents, 60); err != nil || blocked {
		t.Fatalf("a fake-runtime checkout proceeds: blocked=%v err=%v", blocked, err)
	}
}

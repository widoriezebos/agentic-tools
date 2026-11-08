package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

func TestALaneRefusalNamesTheRemedyOfItsCause(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, policy, limit, cause, remedy, absent string
		load                                       float64
		unknown, otherBuild, ordinary              bool
	}{
		{name: "overload", load: 30, cause: "overload", remedy: "the lane waits until the host's one-minute load is below 24 and retries this same command", absent: "enrolls it once"},
		{name: "person policy", policy: "person", cause: "person-policy", remedy: "host.builds=person: " + humanauthority.PersonActRemedy("this same work build command"), absent: "one-minute load"},
		{name: "build cap", policy: "1", otherBuild: true, cause: "build-cap", remedy: "the lane waits for a build place and retries this same command"},
		{name: "unknown capacity", unknown: true, cause: "unknown-capacity", remedy: "the lane retries when host capacity can be read: host load: synthetic unreadable load", absent: "waits until the one-minute load is below"},
		{name: "invalid limit", limit: "0", cause: "invalid-limit", remedy: "a person declares a positive host.load-max: " + humanauthority.PersonActRemedy("metasystem settings set host.load-max N")},
		{name: "ordinary build", load: 30, ordinary: true, cause: "overload", remedy: humanauthority.PersonActRemedy("the original work build command"), absent: "the lane waits"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			fixture := newUnitFixture(t, "")
			policy, limit := row.policy, row.limit
			if policy == "" {
				policy = "auto"
			}
			if limit == "" {
				limit = "24"
			}
			checkHostPolicy(t, fixture, "host.builds="+policy+"\nhost.load-max="+limit+"\n")
			host := &checkHostLoad{load: row.load}
			fixture.manager.CapacitySources.Load = host.sample
			if row.unknown {
				fixture.manager.CapacitySources.Load = func(at time.Time) hostload.Sample {
					return hostload.Sample{At: at.Format(time.RFC3339Nano), Detail: "synthetic unreadable load"}
				}
			}
			if row.otherBuild {
				supervisor, child := ref(10), ref(20)
				other := Record{ID: "other-build", Kind: "build", Goal: "other-goal", Adapter: "codex-exec", WorkingDirectory: t.TempDir(), State: Running, StartedAt: fixture.manager.Now().Format(time.RFC3339Nano), AdapterData: map[string]json.RawMessage{}, Supervisor: &supervisor, Child: &child, ProcessGroup: &child}
				if err := fixture.manager.Store.Create(other); err != nil {
					t.Fatal(err)
				}
			}
			brief := filepath.Join(t.TempDir(), "merge.md")
			if err := os.WriteFile(brief, []byte("| Unit | Lines |\n| --- | --- |\n| lane-merge-1 | 5 |\n"), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := fixture.manager.Start(StartSpec{ID: "lane-build", Kind: "build", Goal: "goal", WorkingDirectory: fixture.worktree, Brief: brief, LaneMerge: !row.ordinary})
			code := "LAUNCH_BUILD_CAPACITY"
			if row.policy == "person" {
				code = "LAUNCH_BUILD_PERSON"
			}
			if !IsCode(err, code) {
				t.Fatalf("refusal=%v; want %s", err, code)
			}
			for _, text := range []string{row.remedy, "load=", "limit=" + limit, "cause=" + row.cause} {
				if !strings.Contains(err.Error(), text) || !strings.Contains(ErrorDetail(err), text) {
					t.Errorf("refusal=%v detail=%s; missing %q", err, ErrorDetail(err), text)
				}
			}
			if row.absent != "" && strings.Contains(err.Error(), row.absent) {
				t.Errorf("refusal names an unrelated remedy: %v", err)
			}
			if _, err := fixture.manager.Store.Read("lane-build"); !os.IsNotExist(err) {
				t.Fatalf("refused build left a job: %v", err)
			}
		})
	}
}

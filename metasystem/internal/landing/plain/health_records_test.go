package plain

import (
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestLandingHealthQuestionPremise(t *testing.T) {
	t.Parallel()
	old, next := strings.Repeat("a", 40), strings.Repeat("b", 40)
	for _, tc := range []struct{ name, fact, main, branch, want string }{
		{"moved", "Main is at commit " + old, next, "main", "gone"},
		{"same", "Main is at commit " + old, old, "main", "current"},
		{"prose", "May I repair this?", next, "main", "unknown"},
		{"checkout restored", "The checkout is not on main.", next, "main", "gone"},
		{"checkout still off main", "The checkout is not on main.", next, "goal/g", "current"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q := channel.Question{Lineage: lane.AgentLineage, Facts: []string{tc.fact}}
			got, err := LaneQuestionPremise(t.TempDir(), q, ProveSeams{Git: func(_ string, args ...string) (string, error) {
				switch args[0] {
				case "symbolic-ref":
					return tc.branch, nil
				case "rev-parse":
					return tc.main, nil
				default:
					return "", nil
				}
			}})
			if err != nil || got != tc.want {
				t.Fatalf("premise=%s error=%v; want %s", got, err, tc.want)
			}
		})
	}
}

func TestLandingHealthFinishedProofRecordsLoadGreenAndRed(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"exit 0", "exit 3"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			install, now := t.TempDir(), bedNow
			git := stubGit{commit: "commit", tree: "tree"}
			result, err := Run(install, install, command, "", io.Discard, ProveSeams{
				Now: func() time.Time { return now }, Git: git.run,
				Load: func(at time.Time) hostload.Sample {
					return hostload.Sample{At: at.Format(time.RFC3339), Load1m: at.Sub(bedNow).Minutes() + 7, Cores: 8, Available: true}
				},
				Command: func(cmd *exec.Cmd) error { now = now.Add(2 * time.Minute); return cmd.Run() },
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.LoadStart == nil || result.LoadEnd == nil {
				t.Fatalf("finished proof has no samples: %+v", result)
			}
			if result.LoadStart.Load1m != 7 || result.LoadEnd.Load1m != 9 || result.LoadStart.Saturated() || !result.LoadEnd.Saturated() {
				t.Fatalf("launch/completion samples changed: %+v / %+v", result.LoadStart, result.LoadEnd)
			}
			stored, _, err := LastResult(install)
			if err != nil || stored.LoadStart == nil || stored.LoadEnd == nil || stored.LoadEnd.Load1m != 9 {
				t.Fatalf("saved result lost samples: %+v %v", stored, err)
			}
			if command == "exit 3" {
				all, err := readLines[Result](resultsPath(install))
				if err != nil {
					t.Fatal(err)
				}
				for _, r := range all {
					if r.LoadStart == nil || r.LoadEnd == nil {
						t.Fatalf("a finished red record lost samples: %+v", r)
					}
				}
			}
		})
	}
}

func TestLandingHealthLoadEndIncludesProofCompletion(t *testing.T) {
	t.Parallel()
	install, now := t.TempDir(), bedNow
	git := stubGit{commit: "commit", tree: "tree"}
	result, err := Run(install, install, "exit 0", "", io.Discard, ProveSeams{
		Now: func() time.Time { return now },
		Git: func(dir string, args ...string) (string, error) {
			if len(args) > 1 && args[0] == "worktree" && args[1] == "remove" {
				now = now.Add(time.Minute)
			}
			return git.run(dir, args...)
		},
		Load: func(at time.Time) hostload.Sample {
			return hostload.Sample{At: at.Format(time.RFC3339), Load1m: at.Sub(bedNow).Minutes() + 7, Cores: 8, Available: true}
		},
		Command: func(cmd *exec.Cmd) error { now = now.Add(2 * time.Minute); return cmd.Run() },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.LoadStart == nil || result.LoadEnd == nil || result.LoadStart.Load1m != 7 || result.LoadEnd.Load1m != 10 {
		t.Fatalf("completion samples: %+v / %+v", result.LoadStart, result.LoadEnd)
	}
}

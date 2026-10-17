package up

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// Each observation is independent: a failed Read hides the start, whereas
// ArgvKnown=false with a successful Read hides only the command.
type terminalTreeReader struct {
	nodes map[int64]humanauthority.Snapshot
	reads map[int64]int
	last  func(int64, int, humanauthority.Snapshot) (humanauthority.Snapshot, error)
}

func (r *terminalTreeReader) Read(pid int64) (humanauthority.Snapshot, error) {
	node, ok := r.nodes[pid]
	if !ok {
		return humanauthority.Snapshot{}, os.ErrNotExist
	}
	count := r.reads[pid]
	r.reads[pid]++
	if r.last != nil {
		return r.last(pid, count, node)
	}
	return node, nil
}

func (*terminalTreeReader) SessionLeader(int64) (int64, error) { return 4200, nil }

type terminalSelfProber struct{}

func (terminalSelfProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{Pid: pid, StartedAt: time.Unix(1500, 0)}, identity.Alive, nil
}

func terminalOwnedChild(t *testing.T) bool {
	t.Helper()
	const marker = "METASYSTEM_TERMINAL_CASE_CHILD"
	if os.Getenv(marker) == t.Name() || strings.HasPrefix(os.Getenv(marker), t.Name()+"/") {
		return false
	}
	command := exec.Command(os.Args[0], "-test.run=^"+strings.ReplaceAll(regexp.QuoteMeta(t.Name()), "/", "$/^")+"$", "-test.count=1", "-test.timeout=30m")
	command.Env = append(os.Environ(), marker+"="+t.Name())
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("terminal case child: %v\n%s", err, output)
	}
	return true
}

func terminalFixtureWrite(t *testing.T, root, rel, contents string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func terminalFixtureOptions(t *testing.T) (Options, *terminalTreeReader) {
	t.Helper()
	root := canonicalRuntimePath(t.TempDir())
	binary := filepath.Join(root, "bin/metasystem")
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(binary, []byte("fixture engine\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("fixture engine\n"))
	if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := steward.MintIdentity(steward.RepoIdentityPath(root), steward.InstallIdentity{
		RepoIdentity: root, Generation: 1, InstallPath: binary, InstallDigest: fmt.Sprintf("sha256:%x", digest[:]), MintedAt: "2026-10-16T10:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	terminalFixtureWrite(t, root, "metasystem.conf", "metasystem.runtimes=fake\nwatch.interval-sec=60\n")
	terminalFixtureWrite(t, root, "processes.json", `{
		"4300":{"pidStartedAt":1700,"command":"/bin/zsh","terminal":true},
		"4301":{"pidStartedAt":1800,"command":"metasystem up","terminal":true},
		"4200":{"pidStartedAt":1600,"command":"/bin/zsh","terminal":true}
	}`)
	for name, value := range map[string]string{
		"METASYSTEM_FAKE_PROCESS_IDENTITY_FILE": filepath.Join(root, "processes.json"),
		census.FixtureSignatureRuntimesEnv:      "fake,codex",
		"METASYSTEM_AGENT_RUNTIME":              "", "METASYSTEM_OWNER_LINEAGE": "", "METASYSTEM_INSTANCE_TAG": "",
		"METASYSTEM_SESSION_ID": "", "METASYSTEM_DELEGATE_ROOT": "", "CLAUDE_CODE_SESSION_ID": "",
	} {
		if err := os.Setenv(name, value); err != nil {
			t.Fatal(err)
		}
	}
	reader := &terminalTreeReader{nodes: map[int64]humanauthority.Snapshot{}, reads: map[int64]int{}}
	for _, row := range []struct{ pid, start, parent int64 }{{4300, 1700, 4200}, {4200, 1600, 1}, {1, 1, 0}} {
		reader.nodes[row.pid] = humanauthority.Snapshot{
			Exact:      identity.Exact{Pid: row.pid, StartedAt: time.Unix(row.start, 0), Argv: []string{"/bin/zsh"}, ArgvKnown: true},
			Executable: "/bin/zsh", ExecutableKnown: true, OwnerUID: 1000, OwnerKnown: true,
			ParentPID: row.parent, ParentKnown: true, TerminalID: "tty-1", TerminalKnown: true,
		}
	}
	sessionParentPid = func(pid int64) (int64, bool) { return 4300, pid == 4301 }
	return Options{Root: root, MetasystemRoot: root, Scope: root, Binary: binary, Pid: 4300, StartTime: 1700,
		CallerPid: 4301, Runtime: "fake", Session: "terminal-fixture", NoRuntimeSession: true,
		TerminalReader: reader, Getpgid: func(int) (int, error) { return 4300, nil }, SelfProber: terminalSelfProber{}}, reader
}

func terminalNoRecord(t *testing.T, root string) {
	t.Helper()
	if entries, err := os.ReadDir(filepath.Join(root, "artifacts/agents/terminals")); !os.IsNotExist(err) && (err != nil || len(entries) != 0) {
		t.Fatalf("a refused terminal left lifecycle records: %v %v", entries, err)
	}
}

func terminalNoMain(t *testing.T, root string) {
	t.Helper()
	if entries, err := os.ReadDir(filepath.Join(root, "artifacts/agents/mains")); !os.IsNotExist(err) && (err != nil || len(entries) != 0) {
		t.Fatalf("person terminal announced or leased: %v %v", entries, err)
	}
}

func terminalAnnouncedAndLeased(t *testing.T, options Options) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(options.Root, "artifacts/agents/mains/terminal-fixture-*.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("target announcement: %v %v", paths, err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	var ann lease.Announcement
	if err := json.Unmarshal(data, &ann); err != nil {
		t.Fatal(err)
	}
	if ann.Pid != options.Pid || ann.PidStartedAt != options.StartTime || ann.MainId == "" {
		t.Fatalf("announcement does not bind the target: %+v", ann)
	}
	held := readLease(t, options.Root)
	if held.HolderMainId != ann.MainId || held.Pid != ann.Pid || held.PidStartedAt != ann.PidStartedAt {
		t.Fatalf("lease does not name announced main: %+v %+v", held, ann)
	}
}

func TestPersonTerminalStartup(t *testing.T) {
	t.Parallel()
	if terminalOwnedChild(t) {
		return
	}
	cases := []string{"person-no-lease", "shell-explicit-lineage", "mission-cli", "environment-lineage", "runtime-ancestor", "unconfigured-runtime", "target-custody", "ancestor-job-custody", "announced-ancestor", "headless", "unreadable-parent", "unreadable-custody", "unreadable-command", "attorney", "different-start", "final-read-failed", "final-read-changed", "final-command-changed", "final-parent-changed", "write-failed", "supervision-failed", "steward-failed", "mains-unreadable", "mains-file", "jobs-unreadable", "jobs-file", "nil-fixture-reader", "steward-plumbing"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if terminalOwnedChild(t) {
				return
			}
			options, reader := terminalFixtureOptions(t)
			want := lease.NotTerminal
			if name == "person-no-lease" || name == "shell-explicit-lineage" || name == "environment-lineage" || strings.HasSuffix(name, "-failed") && name != "final-read-failed" {
				want = lease.PersonTerminal
			}
			change := func(pid int64, edit func(*humanauthority.Snapshot)) {
				snapshot := reader.nodes[pid]
				edit(&snapshot)
				reader.nodes[pid] = snapshot
			}
			switch name {
			case "shell-explicit-lineage":
				options.OwnerLineage = "mission-m"
			case "mission-cli":
				options.OwnerLineage = "mission-m"
				change(4300, func(s *humanauthority.Snapshot) {
					s.Executable = options.Binary
					s.Exact.Argv = []string{options.Binary, "mission", "start", "m"}
				})
			case "environment-lineage":
				if err := os.Setenv("METASYSTEM_OWNER_LINEAGE", "mission-m"); err != nil {
					t.Fatal(err)
				}
			case "runtime-ancestor":
				change(4200, func(s *humanauthority.Snapshot) { s.Exact.Argv = []string{"codex", "exec"} })
			case "unconfigured-runtime":
				change(4300, func(s *humanauthority.Snapshot) { s.Exact.Argv = []string{"codex", "exec"} })
			case "target-custody":
				terminalFixtureWrite(t, options.Root, "artifacts/agents/supervision/state.json", `{"owner":{"pid":4300,"pidStartedAt":1700}}`)
			case "ancestor-job-custody":
				terminalFixtureWrite(t, options.Root, "artifacts/agents/jobs/job.json", `{"jobId":"job-1","custodyProcesses":[{"pid":4200,"pidStartedAt":1600}]}`)
			case "announced-ancestor":
				terminalFixtureWrite(t, options.Root, "artifacts/agents/mains/ancestor-4200.json", fmt.Sprintf(`{"sessionId":"ancestor","mainId":"main-1600-4200-abcdef","pid":4200,"pidStartedAt":1600,"pgid":4200,"runtime":"fake","instanceTag":"ancestor","announcedAt":"2026-10-16T10:00:00Z","commandHash":%q}`, lease.CommandHash("a different command")))
			case "headless", "attorney":
				change(4300, func(s *humanauthority.Snapshot) { s.TerminalID = "" })
				if name == "attorney" {
					terminalFixtureWrite(t, options.Root, "processes.json", `{"4300":{"pidStartedAt":1700,"command":"/bin/zsh","terminal":false},"4301":{"pidStartedAt":1800,"command":"metasystem up","terminal":false}}`)
					humanauthority.AtAttorney = func(string, int64, time.Time) (humanauthority.HelmGrant, bool) {
						return humanauthority.HelmGrant{By: "person", Grant: "live-grant", Until: "2099-01-01T00:00:00Z"}, true
					}
					classified, err := lease.ClassifyPersonAt(options.Root, options.MetasystemRoot, options.Pid, time.Unix(1900, 0))
					if err != nil || classified.Class != lease.ClassHuman || classified.Attorney == nil {
						t.Fatalf("attorney fixture does not admit a person-act: %+v %v", classified, err)
					}
				}
			case "unreadable-parent":
				want = lease.TerminalUnknown
				change(4200, func(s *humanauthority.Snapshot) { s.ParentKnown = false })
			case "unreadable-command":
				want = lease.TerminalUnknown
				change(4200, func(s *humanauthority.Snapshot) { s.Exact.ArgvKnown = false })
			case "unreadable-custody":
				want = lease.TerminalUnknown
				path := filepath.Join(options.Root, "artifacts/agents/supervision/state.json")
				if err := os.MkdirAll(path, 0o755); err != nil {
					t.Fatal(err)
				}
			case "different-start":
				want = lease.TerminalUnknown
				change(4300, func(s *humanauthority.Snapshot) { s.Exact.StartedAt = time.Unix(1701, 0) })
			case "final-read-failed", "final-read-changed", "final-command-changed", "final-parent-changed":
				want = lease.TerminalUnknown
				reader.last = func(pid int64, count int, s humanauthority.Snapshot) (humanauthority.Snapshot, error) {
					if pid == 4300 && count == 2 {
						if name == "final-read-failed" {
							return humanauthority.Snapshot{}, errors.New("start unreadable")
						}
						switch name {
						case "final-read-changed":
							s.Exact.StartedAt = time.Unix(1701, 0)
						case "final-command-changed":
							s.Exact.Argv = []string{"codex", "exec"}
						case "final-parent-changed":
							s.ParentPID = 4000
						}
					}
					return s, nil
				}
			case "write-failed":
				terminalFixtureWrite(t, options.Root, "artifacts/agents/terminals", "file")
			case "mains-unreadable", "jobs-unreadable", "mains-file", "jobs-file":
				want = lease.TerminalUnknown
				store, _, _ := strings.Cut(name, "-")
				path := filepath.Join(options.Root, "artifacts/agents", store)
				if strings.HasSuffix(name, "-file") {
					terminalFixtureWrite(t, options.Root, "artifacts/agents/"+store, "file")
				} else {
					if err := os.MkdirAll(path, 0o000); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = os.Chmod(path, 0o755) })
				}
			case "nil-fixture-reader":
				want = lease.TerminalUnknown
				options.TerminalReader = nil
			case "steward-plumbing":
				change(4200, func(s *humanauthority.Snapshot) { s.Exact.Argv = []string{options.Binary, "steward", "tick"} })
			}
			ref := humanauthority.ProcessRef{PID: options.Pid, PIDStartedAt: options.StartTime}
			if got := lease.PersonTerminalAt(options.Root, options.MetasystemRoot, ref, options.TerminalReader); got != want {
				t.Fatalf("predicate = %s, want %s", got, want)
			}
			reader.reads = map[int64]int{}
			armedCalls, stewardCalls := 0, 0
			options.RestampStopCapability = func(string, string, int64) (StopCapabilityRestampResult, error) {
				if want == lease.PersonTerminal {
					t.Fatal("terminal tried to adopt a seat's stop capability")
				}
				return StopCapabilityRestampResult{}, nil
			}
			options.EnsureArmed = func(supervise.EnsureOptions) (supervise.EnsureResult, error) {
				armedCalls++
				if want == lease.PersonTerminal {
					terminalNoMain(t, options.Root)
				} else {
					terminalAnnouncedAndLeased(t, options)
				}
				if name == "supervision-failed" {
					return supervise.EnsureResult{}, errors.New("supervision refused")
				}
				return supervise.EnsureResult{Action: "verified"}, nil
			}
			options.EnsureStewardRunner = func(string, *steward.EnrolledBinary) (steward.EnsureRunnerResult, error) {
				stewardCalls++
				if name == "steward-failed" {
					return steward.EnsureRunnerResult{}, errors.New("steward refused")
				}
				return steward.EnsureRunnerResult{Action: "verified"}, nil
			}
			result := Run(options)
			if component(result, "session-identity").Outcome != "verified" {
				t.Fatalf("fixture did not resolve the explicit pair: %+v", result)
			}
			if name == "person-no-lease" || name == "shell-explicit-lineage" || name == "environment-lineage" {
				wantLine := "your terminal 4300 is a person's; no agent main is announced; the steward's ladder may start a seat"
				if result.ExitCode() != 0 || result.Outcome != "person-terminal" || component(result, "person-terminal").Outcome != "recorded" || strings.Count(strings.Join(result.Lines(), "\n"), wantLine) != 1 || armedCalls != 1 || stewardCalls != 1 {
					t.Fatalf("terminal startup = %+v; callbacks=%d,%d", result, armedCalls, stewardCalls)
				}
				path := filepath.Join(options.Root, "artifacts/agents/terminals/4300-1700.json")
				before, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				wantBytes := `{"kind":"person-terminal","pid":4300,"pidStartedAt":1700,"pidStartTicks":0,"bootId":"","callerPid":4301,"callerPidStartedAt":1800}`
				info, err := os.Stat(path)
				if string(before) != wantBytes || err != nil || info.Mode().Perm() != 0o600 {
					t.Fatalf("record: %s %v %v", before, info, err)
				}
				reader.reads = map[int64]int{}
				repeated := Run(options)
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(before, after) || strings.Join(result.Lines(), "\n") != strings.Join(repeated.Lines(), "\n") || armedCalls != 2 || stewardCalls != 2 {
					t.Fatalf("repeat changed the answer or record: %+v %s %v", repeated, after, err)
				}
				classified, err := lease.ClassifyAt(options.Root, options.MetasystemRoot, options.Pid)
				if err != nil || classified.Class != lease.ClassHuman {
					t.Fatalf("terminal classification = %+v %v", classified, err)
				}
				terminalNoMain(t, options.Root)
				return
			}
			if result.Outcome == "person-terminal" || strings.Contains(strings.Join(result.Lines(), "\n"), "your terminal") {
				t.Fatalf("refused or failed path claimed terminal success: %+v", result)
			}
			if want == lease.PersonTerminal {
				failed := map[string]string{"write-failed": "person-terminal", "supervision-failed": "supervision-owner", "steward-failed": "steward-runner"}[name]
				if result.ExitCode() == 0 || result.Failed != failed || component(result, failed).Outcome != "failed" || result.Remedy == "" {
					t.Fatalf("failure lost component or remedy: %+v", result)
				}
				if name == "write-failed" && (!strings.Contains(result.Remedy, "artifacts/agents/terminals/4300-1700.json") || armedCalls != 0 || stewardCalls != 0) || name == "supervision-failed" && (armedCalls != 1 || stewardCalls != 0) || name == "steward-failed" && (armedCalls != 1 || stewardCalls != 1) {
					t.Fatalf("failure called a later owner: %+v callbacks=%d,%d", result, armedCalls, stewardCalls)
				}
				return
			}
			terminalNoRecord(t, options.Root)
			if strings.HasPrefix(name, "mains-") || strings.HasPrefix(name, "jobs-") {
				return
			}
			terminalAnnouncedAndLeased(t, options)
			if name == "mission-cli" {
				paths, err := filepath.Glob(filepath.Join(options.Root, "artifacts/agents/mains/terminal-fixture-*.json"))
				if err != nil || len(paths) != 1 {
					t.Fatalf("mission announcement: %v %v", paths, err)
				}
				data, err := os.ReadFile(paths[0])
				if err != nil {
					t.Fatal(err)
				}
				var ann lease.Announcement
				if err := json.Unmarshal(data, &ann); err != nil {
					t.Fatal(err)
				}
				held := readLease(t, options.Root)
				if ann.OwnerLineage != "mission-m" || held.OwnerLineage != "mission-m" {
					t.Fatalf("mission lineage lost: announcement=%q lease=%q", ann.OwnerLineage, held.OwnerLineage)
				}
			}
			if name != "unreadable-custody" && (result.Outcome != "armed" || armedCalls != 1 || stewardCalls != 1) {
				t.Fatalf("ordinary agent path changed: %+v callbacks=%d,%d", result, armedCalls, stewardCalls)
			}
		})
	}
}

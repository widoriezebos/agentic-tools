package supervisor

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The external runtime's self-test (VOA-30, R13): the shared SelftestRun,
// through externalOps.Selftest, runs its dispatch, follow-up, cancel and
// permission legs against the newagent fixture. Its delegate children are a
// fake-style engine stand-in (this test binary, entered from TestMain as
// runExternalEngineStandIn), which mints each job the way dispatch does and
// runs the real delegate-supervisor for newagent in process.

// recordDispatcher applies the lease-held callbacks to the job records the
// way the delegate lifecycle does for these legs: the handshake's session, a
// compare-and-swap's status and patch, and cancellation.
type recordDispatcher struct{ root string }

func (r recordDispatcher) Run(_, _ io.Writer, args ...string) int {
	job := flagValue(args, "--job")
	path := filepath.Join(r.root, "artifacts", "agents", "jobs", job+".json")
	switch args[0] {
	case "__handshake":
		editRecord(path, func(record map[string]any) { record["sessionId"] = flagValue(args, "--session") })
	case "__record-cas":
		var patch map[string]any
		data, _ := os.ReadFile(flagValue(args, "--patch"))
		_ = json.Unmarshal(data, &patch)
		editRecord(path, func(record map[string]any) {
			for key, value := range patch {
				record[key] = value
			}
			record["status"] = flagValue(args, "--status")
		})
	case "__cancel-owned":
		editRecord(path, func(record map[string]any) { record["status"] = "cancelled" })
	}
	return 0
}

// runExternalEngineStandIn answers `internal delegate --adapter-selftest |
// --follow-up | --cancel` for the external self-test.
func runExternalEngineStandIn() int {
	root := os.Getenv("EXTERNAL_ENGINE_ROOT")
	args := os.Args[1:]
	if len(args) < 4 || args[0] != "internal" || args[1] != "delegate" {
		fmt.Fprintf(os.Stderr, "engine stand-in: unexpected argv %q\n", args)
		return 64
	}
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	switch args[2] {
	case "--adapter-selftest":
		// runtime --brief B --workspace W --op JOB
		brief, workspace, job := args[5], args[7], args[9]
		// The cancel leg's turn runs until it is cancelled.
		return standInRound(root, job, "", "1", "dispatch", brief, workspace, "", !strings.HasSuffix(job, "-cancel"))
	case "--follow-up":
		parent := args[3]
		var record map[string]any
		data, _ := os.ReadFile(filepath.Join(jobs, parent+".json"))
		_ = json.Unmarshal(data, &record)
		session, _ := record["sessionId"].(string)
		workspace, _ := record["workspaceRoot"].(string)
		return standInRound(root, parent+"-r2", parent, "2", "follow-up", args[5], workspace, session, true)
	case "--cancel":
		d := standInDeps(root)
		return Main([]string{"newagent", "cancel", "--root", root, "--job", args[3]}, func(string) Deps { return d })
	}
	return 64
}

func standInDeps(root string) Deps {
	return Deps{Root: root, Engine: "/nonexistent/metasystem", Environ: os.Environ(), Getenv: os.Getenv,
		Pid: os.Getpid(), Stdout: os.Stdout, Stderr: os.Stderr, Clock: SystemClock(), Dispatch: recordDispatcher{root: root},
		GroupMembers: func(int, ...int) ([]int, error) { return nil, nil }, LookPath: exec.LookPath}
}

// standInRound mints one pending job as dispatch does and, when run, runs
// its round through the real supervisor.
func standInRound(root, job, parent, round, verb, briefPath, workspace, session string, run bool) int {
	agents := filepath.Join(root, "artifacts", "agents")
	rootJob := job
	if parent != "" {
		rootJob = parent
	}
	roundDir := filepath.Join(agents, rootJob, "rounds", round)
	brief, err := os.ReadFile(briefPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	raw := make([]byte, 16)
	_, _ = rand.Read(raw)
	capability, tag := hex.EncodeToString(raw), "newagent-tag-"+job
	digest := sha256.Sum256([]byte(capability))
	var parentJob, sessionID any
	if parent != "" {
		parentJob, sessionID = parent, session
	}
	status := "pending"
	if !run {
		status = "running"
	}
	record := map[string]any{
		"jobId": job, "operationId": job, "status": status, "round": map[string]int{"1": 1, "2": 2}[round],
		"parentJob": parentJob, "role": "implementer", "runtime": "newagent", "sessionId": sessionID,
		"requestedModel": "newagent-model", "effectiveModel": nil, "workspaceRoot": workspace,
		"sessionEstablishedSignal": "newagent-signal", "instanceTag": tag,
		"permissions": map[string]any{"requested": map[string]any{"readRoots": []string{}, "writeRoots": []string{workspace}, "network": "deny"}},
		"launchCapability": map[string]any{"digest": hex.EncodeToString(digest[:]), "jobId": job, "operationId": job,
			"instanceTag": tag, "adapterVerb": verb, "status": "minted", "mintedAt": time.Now().UTC().Format(time.RFC3339)},
	}
	gate := filepath.Join(root, "gates", job)
	for path, content := range map[string]string{
		filepath.Join(roundDir, "prompt.md"):        "Working Mode: implement\n\n" + string(brief),
		filepath.Join(roundDir, "composition.json"): `{"references":[]}` + "\n",
		gate: "",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	data, _ := json.Marshal(record)
	if err := os.WriteFile(filepath.Join(agents, "jobs", job+".json"), data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !run {
		return 0
	}
	d := standInDeps(root)
	return Main([]string{"newagent", verb, "--root", root, "--job", job, "--start-gate", gate,
		"--instance-tag", tag, "--launch-capability", capability}, func(string) Deps { return d })
}

// TestExternalRuntimeSelftestRunsTheSharedLegs: `newagent selftest` runs
// the shared self-test's dispatch, follow-up (same session), cancel and
// permission legs through the external adapter, with its custom probe's
// labels on the pass record. It runs in its own process: the scratch
// repository's git is a stub on that process's lookup path.
func TestExternalRuntimeSelftestRunsTheSharedLegs(t *testing.T) {
	t.Parallel()
	f, log := externalInstall(t, installOptions{role: "implementer"})
	conf := filepath.Join(f.root, "metasystem.conf")
	mustWrite(t, conf, readText(t, conf)+"role.default.model.newagent=newagent-model\n")
	stubs := filepath.Join(f.root, "stub-bin")
	for path, body := range map[string]string{
		filepath.Join(stubs, "git"): "#!/bin/sh\nexit 0\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	config, _ := json.Marshal(helperConfig{Args: []string{"newagent", "selftest", "--root", f.root}, Root: f.root, Engine: f.engine,
		Dispatcher: fakeRecordingDispatcher{Root: f.root, Log: filepath.Join(f.root, "dispatch.log")}})
	command := exec.Command(os.Args[0], "-test.run=^TestFakeSupervisorHelperProcess$")
	command.Env = append(f.environ(), "FAKE_SUPERVISOR_HELPER=1", "FAKE_SUPERVISOR_CONFIG="+string(config),
		"PATH="+stubs+":"+os.Getenv("PATH"), "TMPDIR="+t.TempDir(),
		"EXTERNAL_ENGINE_STANDIN=1", "EXTERNAL_ENGINE_ROOT="+f.root)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("selftest: %v\n%s\njob logs: %s", err, output, strings.Join(operationsLogged(t, log), " "))
	}
	if !strings.Contains(string(output), "newagent adapter selftest passed") {
		t.Fatalf("selftest output = %s", output)
	}
	jobs := filepath.Join(f.agents(), "jobs")
	var main, follow, cancel map[string]any
	for _, entry := range []struct {
		suffix string
		into   *map[string]any
	}{{"-main.json", &main}, {"-main-r2.json", &follow}, {"-cancel.json", &cancel}} {
		matches, _ := filepath.Glob(filepath.Join(jobs, "newagent-selftest-*"+entry.suffix))
		if len(matches) != 1 {
			t.Fatalf("records %s = %v", entry.suffix, matches)
		}
		*entry.into = readJSON(t, matches[0])
	}
	if main["status"] != "completed" || follow["status"] != "completed" || cancel["status"] != "cancelled" {
		t.Fatalf("legs = %v / %v / %v", main["status"], follow["status"], cancel["status"])
	}
	if follow["sessionId"] != main["sessionId"] || main["sessionId"] == nil {
		t.Fatalf("the follow-up resumed %v, want %v", follow["sessionId"], main["sessionId"])
	}
	logged := strings.Join(operationsLogged(t, log), " ")
	for _, want := range []string{"probe=ok", "prepare=ok", "cancel=ok"} {
		if !strings.Contains(logged, want) {
			t.Fatalf("adapter operations = %s, missing %s", logged, want)
		}
	}
	if records, _ := filepath.Glob(filepath.Join(f.agents(), "selftests", "*.json")); len(records) != 1 {
		t.Fatalf("pass records = %v", records)
	}
}

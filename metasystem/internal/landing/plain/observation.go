package plain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// Anchors are the registration and Git references belonging to one observation.
type Anchors struct {
	Lane                                      lane.Record
	HEAD, Branch, MergeHEAD, Main, OriginMain string
	Errors                                    []string
}
type Captured struct {
	Shadow      *state.Record
	Absent      bool
	Version     int
	Fingerprint string
}
type Observation struct {
	Captured Captured
	Anchors  Anchors
	Status   Status
	Coherent bool
}

var errNoMerge = errors.New("no merge is pending")

type Unstable struct{}

func (*Unstable) Error() string {
	return "the lane's records kept changing during this read (3 tries); compared again on the next call"
}

type Reregistered struct{ Before, After lane.Record }

func (r *Reregistered) Error() string {
	return fmt.Sprintf("the landing lane registration changed from checkout %s to checkout %s; read the registration again", r.Before.Root, r.After.Root)
}

// Capture reads the shadow before any view is assembled. An absent shadow is nil.
func Capture(home string) (*state.Record, error) {
	record, present, err := lane.Read(home)
	if err != nil || !present {
		return nil, err
	}
	shadow, present, err := state.Read(Dir(record.Install))
	if err != nil || !present {
		return nil, err
	}
	return &shadow, nil
}

// shadowInputs is the single inventory of raw files read by the projection.
func shadowInputs(home string) []string {
	paths := []string{lane.RecordPath(home)}
	for _, name := range []string{"landing-lane-paused.json", "landing-lane-resumed.json", "landing-agent-keeper.json"} {
		paths = append(paths, filepath.Join(lane.HostDir(home), name))
	}
	record, exists, _ := lane.Read(home)
	if !exists {
		return paths
	}
	for _, name := range []string{"state.json", "clock.json", "transition-finding.json", "resume-record-finding.json", "queue.jsonl", "batch.json", "running.json", "results.jsonl", "gates.jsonl", "pushes.jsonl", "stops.jsonl", "agent-step.json", "agent-idle.json", "drain.json", "regenerating.json", "stop-question", "hand-in-fetches.jsonl"} {
		paths = append(paths, filepath.Join(Dir(record.Install), name))
	}
	for _, pattern := range []string{filepath.Join(Dir(record.Install), "fixes", "*.json"), filepath.Join(Dir(record.Install), "incidents", "*.json")} {
		files, _ := filepath.Glob(pattern)
		paths = append(paths, files...)
	}
	var running Running
	data, _ := os.ReadFile(runningPath(record.Install))
	if json.Unmarshal(data, &running) == nil && running.Log != "" {
		paths = append(paths, running.Log)
	}
	files, _ := filepath.Glob(filepath.Join(home, "launch", "*", "record.json"))
	for _, path := range files {
		data, err := os.ReadFile(path)
		var record struct {
			Kind string `json:"kind"`
		}
		if err == nil && json.Unmarshal(data, &record) == nil && record.Kind == launch.LandingKind {
			paths = append(paths, path)
		}
	}
	return paths
}
func recordsFingerprint(home string) string {
	hash := sha256.New()
	for _, path := range shadowInputs(home) {
		var data []byte
		info, err := os.Stat(path)
		if err == nil {
			if info.Mode().IsRegular() {
				data, err = os.ReadFile(path)
			} else {
				err = fmt.Errorf("record has non-regular mode %s", info.Mode())
			}
		}
		// Lengths keep boundaries unambiguous; read failures are distinct from absence.
		fmt.Fprintf(hash, "%d:%s:%d:", len(path), path, len(data))
		hash.Write(data)
		fmt.Fprintf(hash, ":%v;", err)
	}
	if record, exists, err := lane.Read(home); err == nil && exists {
		stamp, err := shadowEngine(record.Install)
		fmt.Fprintf(hash, "engine-stamp:%d:%s:%v;", len(stamp), stamp, err)
	}
	return hex.EncodeToString(hash.Sum(nil))
}
func readAnchors(home string, seams ProveSeams) Anchors {
	a := Anchors{}
	record, present, err := lane.Read(home)
	if err != nil {
		a.Errors = append(a.Errors, err.Error())
	}
	a.Lane = record
	if !present {
		return a
	}
	read := func(args ...string) string {
		value, err := seams.git(record.Root, args...)
		if err != nil {
			var exit *exec.ExitError
			optional := strings.Contains(strings.Join(args, " "), "MERGE_HEAD") || args[0] == "symbolic-ref" || strings.Contains(strings.Join(args, " "), "refs/heads/main")
			if !optional || !errors.As(err, &exit) || exit.ExitCode() != 1 && exit.ExitCode() != 128 {
				a.Errors = append(a.Errors, fmt.Sprintf("%s: %v", strings.Join(args, " "), err))
			}
		}
		return value
	}
	a.HEAD = read("rev-parse", "--verify", "HEAD^{commit}")
	a.Branch = read("symbolic-ref", "--quiet", "--short", "HEAD")
	a.MergeHEAD = read("rev-parse", "--verify", "MERGE_HEAD^{commit}")
	a.Main = read("rev-parse", "--verify", "--quiet", "refs/heads/main^{commit}")
	a.OriginMain = read("rev-parse", "--verify", "--quiet", "refs/remotes/origin/main^{commit}")
	return a
}
func fingerprintWithAnchors(home string, anchors Anchors) string {
	data, _ := json.Marshal(anchors)
	sum := sha256.Sum256(append([]byte(recordsFingerprint(home)), data...))
	return hex.EncodeToString(sum[:])
}
func observationFingerprint(home string, seams ProveSeams) string {
	return fingerprintWithAnchors(home, readAnchors(home, seams))
}
func anchoredSeams(seams ProveSeams, a Anchors) ProveSeams {
	original := seams
	seams.ReadOnly = true
	seams.Lane = func() (lane.Record, error) { return a.Lane, nil }
	seams.Git = func(dir string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "symbolic-ref" {
			return a.Branch, nil
		}
		if len(args) > 0 && args[0] == "rev-parse" {
			for _, arg := range args[1:] {
				switch arg {
				case "HEAD^{commit}":
					return a.HEAD, nil
				case "refs/heads/main^{commit}":
					return a.Main, nil
				case "refs/remotes/origin/main^{commit}":
					return a.OriginMain, nil
				case "MERGE_HEAD^{commit}":
					if a.MergeHEAD == "" {
						return "", errNoMerge
					}
					return a.MergeHEAD, nil
				}
			}
		}
		return original.git(dir, args...)
	}
	return seams
}
func captureObservation(home string, expected *lane.Record, seams ProveSeams, build func(ProveSeams) lane.View) (Observation, error) {
	result := Observation{}
	a := readAnchors(home, seams)
	result.Anchors = a
	if expected != nil && !reflect.DeepEqual(*expected, a.Lane) {
		return result, &Reregistered{Before: *expected, After: a.Lane}
	}
	fingerprint := fingerprintWithAnchors(home, a)
	shadow, shadowErr := Capture(home)
	result.Captured = Captured{Shadow: shadow, Absent: shadow == nil, Fingerprint: fingerprint}
	if shadow != nil {
		result.Captured.Version = shadow.Version
	}
	readonly := anchoredSeams(seams, a)
	if readonly.Observed != nil {
		readonly.Observed()
	}
	readonly.Observed = nil
	owner := readonly.AgentProbe
	if owner == nil {
		owner = func(string) (lane.OwnerProbe, error) {
			if readonly.AgentRunning != nil {
				alive, err := readonly.AgentRunning()
				return lane.OwnerProbe{Alive: alive}, err
			}
			return lane.OwnerProbe{}, nil
		}
	}
	probe, probeErr := owner(a.Lane.Root)
	readonly.AgentProbe = func(string) (lane.OwnerProbe, error) { return probe, probeErr }
	var unready *bool
	if readonly.AgentReady != nil && !probe.Alive {
		readyErr := readonly.AgentReady(a.Lane.Root)
		readonly.AgentReady = func(string) error { return readyErr }
		value := readyErr != nil
		unready = &value
	}
	var view lane.View
	if build != nil {
		view = build(readonly)
	} else {
		view = lane.BuildView(lane.ViewSources{Home: home, Now: readonly.now(), Owner: readonly.AgentProbe, Ready: readonly.AgentReady, Wake: KeeperWake(home, readonly)})
	}
	status, raw := observe(home, a.Lane, view, readonly, checkoutGit(a.Lane.Root, readonly))
	raw.Anchors, raw.Previous = a, shadow
	raw.AgentLive, raw.AgentUnready = &probe.Alive, unready
	if probeErr != nil {
		status.Problems = append(status.Problems, "the landing agent cannot be observed: "+probeErr.Error())
	}
	status.Raw, status.Shadow = raw, shadow
	if shadowErr != nil {
		status.Findings = append(status.Findings, "the lane shadow cannot be read: "+shadowErr.Error()+"; the next refresh replaces it")
	}
	status.Problems = append(status.Problems, a.Errors...)
	if raw.Err != nil {
		status.Problems = append(status.Problems, raw.Err.Error())
	}
	result.Status = status
	result.Coherent = result.Captured.Fingerprint == observationFingerprint(home, seams)
	return result, nil
}

// ObserveStatus retries a changing read and assembles health only from a coherent observation.
func ObserveStatus(home string, expected lane.Record, seams ProveSeams, build func(ProveSeams) lane.View) Status {
	var last Observation
	for try := 0; try < 3; try++ {
		observation, err := captureObservation(home, &expected, seams, build)
		if err != nil {
			return Status{ObservationError: err, Problems: []string{err.Error()}}
		}
		last = observation
		if !observation.Coherent {
			continue
		}
		return assembleHealth(observation.Status, expected, anchoredSeams(seams, observation.Anchors), checkoutGit(expected.Root, anchoredSeams(seams, observation.Anchors)))
	}
	problem := &Unstable{}
	health := &Health{State: "unknown", ObservationProblem: problem.Error(), Shadow: last.Captured.Shadow}
	return Status{View: last.Status.View, Shadow: last.Captured.Shadow, ObservationError: problem, Findings: []string{problem.Error()}, Health: health}
}

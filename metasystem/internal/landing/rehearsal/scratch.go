// Package rehearsal owns isolated rehearsals of a candidate's landing lane.
package rehearsal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/candidateengine"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

type Step struct {
	Command string `json:"command"`
	WallMS  int64  `json:"wall_ms"`
	Exit    int    `json:"exit"`
	Output  string `json:"output"`
}

type Record struct {
	Candidate string    `json:"candidate_commit"`
	Tree      string    `json:"candidate_tree"`
	Engine    string    `json:"engine_path"`
	Scratch   string    `json:"scratch"`
	Scenarios []string  `json:"scenarios"`
	Steps     []Step    `json:"steps"`
	Outcome   string    `json:"outcome"`
	WallMS    int64     `json:"wall_ms"`
	Problem   string    `json:"problem,omitempty"`
	Breakdown Breakdown `json:"breakdown_ms"`
}

type Scratch struct {
	Record
	Checkout, Module, Registry, Harness string
	Env                                 []string
	ctx                                 context.Context
	base                                string
	stage                               string
	stageStart                          time.Time
	engineStamp, engineTree             string
	isolatedCaller                      bool
	processes                           identity.ProcessTable
	recordModule                        string
}

// Run bounds each child by the scenario deadline and retains its own exit.
func (s *Scratch) Run(dir, name string, args ...string) (string, error) {
	start := time.Now()
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Env = dir, s.Env
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := boundedexec.RunContext(s.ctx, cmd, boundedexec.FixedBound(10*time.Minute, "rehearsal scenario deadline"), name)
	step := Step{Command: strings.Join(append([]string{name}, args...), " "), WallMS: time.Since(start).Milliseconds(), Exit: -1, Output: strings.TrimSpace(output.String())}
	if cmd.ProcessState != nil {
		step.Exit = cmd.ProcessState.ExitCode()
	}
	s.Steps = append(s.Steps, step)
	if err != nil {
		return step.Output, fmt.Errorf("%s exited %d: %s: %w", step.Command, step.Exit, step.Output, err)
	}
	return step.Output, nil
}

func (s *Scratch) Git(dir string, args ...string) (string, error) { return s.Run(dir, "git", args...) }

func (s *Scratch) Verb(dir string, args ...string) (string, error) {
	start := time.Now()
	cmd := exec.Command(s.Engine, append(args, "--json")...)
	cmd.Dir, cmd.Env = dir, s.Env
	if len(args) >= 2 && args[0] == "landing" && (args[1] == "set" || args[1] == "run") {
		cmd.Env = append(append([]string(nil), s.Env...), "METASYSTEM_REHEARSAL_SCRATCH="+s.Scratch)
	}
	read := verbresult.Capture(cmd, strings.Join(args[:2], " "))
	result, err := read(boundedexec.RunContext(s.ctx, cmd, boundedexec.FixedBound(10*time.Minute, "rehearsal scenario deadline"), "green "+strings.Join(args, " ")))
	data, _ := json.Marshal(result)
	s.Steps = append(s.Steps, Step{Command: strings.Join(args, " "), WallMS: time.Since(start).Milliseconds(), Exit: result.Exit, Output: string(data)})
	if err != nil {
		return string(data), err
	}
	if result.Exit != 0 {
		return string(data), result.Err()
	}
	return string(data), nil
}

// Prepare archives the pinned commit before building; local files never enter it.
func Prepare(ctx context.Context, module string, environment []string) (*Scratch, error) {
	root, err := os.MkdirTemp(diskstore.ProcessTempRoot(), "metasystem-rehearsal-")
	if err != nil {
		return nil, err
	}
	path, err := diskstore.CreateTempStoreAt(root, diskstore.CheckoutRegistry(root), "green", "rehearsal", diskstore.Owner{Kind: diskstore.OwnerWorkspace, Ref: filepath.Base(root)})
	if err != nil {
		return nil, err
	}
	s := &Scratch{Record: Record{Scratch: path, Outcome: "failed", Scenarios: []string{"green"}}, ctx: ctx, base: realpath.Resolve(root), processes: identity.KernelProcessTable{}}
	s.Registry = filepath.Join(path, "registry", ".metasystem")
	for _, entry := range environment {
		key, value, _ := strings.Cut(entry, "=")
		if key == "METASYSTEM_SUPERVISION_REGISTRY_HOME" && value != "" {
			s.isolatedCaller = true
		}
		if strings.HasPrefix(key, "METASYSTEM_") && key != "METASYSTEM_TEST_WORKERS" && !strings.HasPrefix(key, "METASYSTEM_FIXTURE_") || strings.HasPrefix(key, "LANDING_") || strings.HasPrefix(key, "GIT_") {
			continue
		}
		s.Env = append(s.Env, entry)
	}
	s.Env = append(s.Env, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+filepath.Dir(s.Registry), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=rehearsal", "GIT_AUTHOR_EMAIL=rehearsal@example.invalid", "GIT_COMMITTER_NAME=rehearsal", "GIT_COMMITTER_EMAIL=rehearsal@example.invalid")
	refs, err := s.Git(module, "rev-parse", "--show-toplevel", "HEAD", "HEAD^{tree}")
	if err != nil {
		return s, err
	}
	parts := strings.Split(refs, "\n")
	top := strings.Join(parts[:len(parts)-2], "\n")
	s.Candidate, s.Tree = parts[len(parts)-2], parts[len(parts)-1]
	relative, err := filepath.Rel(top, realpath.Resolve(module))
	if err != nil {
		return s, err
	}
	archive, snapshot := filepath.Join(path, "candidate.tar"), filepath.Join(path, "snapshot")
	s.Engine, s.Checkout = filepath.Join(path, "bin", "metasystem"), filepath.Join(path, "lane")
	s.Module = filepath.Join(s.Checkout, relative)
	if err = errors.Join(os.MkdirAll(snapshot, 0700), os.MkdirAll(filepath.Dir(s.Engine), 0700)); err != nil {
		return s, err
	}
	s.Env = append(s.Env, "METASYSTEM_BUILD_STAMP="+s.Tree)
	reused, err := s.reuseEngine(module)
	if err != nil {
		return s, err
	}
	if _, err = s.Git(top, "archive", "--output="+archive, s.Candidate); err != nil {
		return s, err
	}
	if err = unpackArchive(archive, snapshot); err != nil {
		return s, err
	}
	if !reused {
		argv := candidateengine.DefaultBuildArgv(s.Engine)
		if _, err = s.Run(filepath.Join(snapshot, relative), argv[0], argv[1:]...); err != nil {
			return s, err
		}
	}
	origin := filepath.Join(path, "origin.git")
	for _, command := range []struct {
		dir  string
		args []string
	}{
		{path, []string{"init", "--bare", "--initial-branch=main", origin}},
		{path, []string{"clone", "--no-local", "--no-checkout", top, s.Checkout}},
		{s.Checkout, []string{"remote", "set-url", "origin", origin}},
		{s.Checkout, []string{"checkout", "-B", "main", s.Candidate}},
		{s.Checkout, []string{"config", "metasystem.goal.machine", "rehearsal"}},
	} {
		if _, err = s.Git(command.dir, command.args...); err != nil {
			return s, err
		}
	}
	s.Env = append(s.Env, "METASYSTEM_BIN="+s.Engine, "PATH="+filepath.Dir(s.Engine)+string(os.PathListSeparator)+os.Getenv("PATH"))
	return s, s.CheckIdentity()
}

func (s *Scratch) CheckIdentity() error {
	file, err := os.Open(s.Engine)
	if err != nil {
		return err
	}
	defer file.Close()
	stamp, err := enginebuild.ReadStamp(file)
	if err != nil || stamp != s.Tree && stamp != s.Candidate && (stamp != s.engineStamp || s.engineTree != s.Tree) {
		return fmt.Errorf("engine identity %q differs from candidate tree %s: %v", stamp, s.Tree, err)
	}
	return nil
}

// Finish keeps every failed scratch and writes the record before cleanup.
func (s *Scratch) Finish(module string, start time.Time, keep bool, problem error) error {
	problem = errors.Join(problem, s.waitChildren(start.Add(10*time.Minute)))
	s.WallMS = time.Since(start).Milliseconds()
	s.Breakdown.Total = s.WallMS
	if problem == nil {
		problem = s.CheckIdentity()
	}
	if problem == nil && s.WallMS >= (10*time.Minute).Milliseconds() {
		problem = fmt.Errorf("green rehearsal exceeded its ten-minute budget (%d ms)", s.WallMS)
	}
	if problem == nil {
		s.Outcome = "passed"
	} else {
		s.Outcome = "failed"
		s.Problem = problem.Error()
	}
	data, _ := json.MarshalIndent(s.Record, "", "  ")
	root := s.Scratch
	if s.recordModule != "" && s.recordModule == realpath.Resolve(module) && !s.isolatedCaller {
		root = module
	}
	path := filepath.Join(root, "records", "rehearsals", s.Tree+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if _, err := atomicfile.WriteFile(path, append(data, '\n'), 0644, root); err != nil {
		return err
	}
	if problem == nil && !keep {
		return diskstore.ReleaseTempStoreAt(context.Background(), diskstore.CheckoutRegistry(s.base), s.Scratch, "rehearsal", diskstore.Owner{Kind: diskstore.OwnerWorkspace, Ref: filepath.Base(s.base)})
	}
	return problem
}

// LaunchSettings copies only declared launch values, never a local secrets file.
func LaunchSettings(snapshot string) (string, error) {
	var text strings.Builder
	for _, setting := range config.CompiledSettings() {
		if !strings.HasPrefix(setting.Key, "launch.") || setting.Computed != "" {
			continue
		}
		value, _, err := config.Get(config.GetParams{Key: setting.Key, ConfPath: filepath.Join(snapshot, "metasystem.conf"), KeepAuto: true})
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&text, "%s=%s\n", setting.Key, value)
	}
	return text.String(), nil
}

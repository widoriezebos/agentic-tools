package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

const (
	ordinaryProjectTree             = "1111111111111111111111111111111111111111"
	ordinaryChangedTree             = "2222222222222222222222222222222222222222"
	ordinaryReuseTree               = "1212121212121212121212121212121212121212"
	ordinaryFailedTree              = "2323232323232323232323232323232323232323"
	ordinaryBrokenTree              = "3434343434343434343434343434343434343434"
	ordinaryInstallationTree        = "3333333333333333333333333333333333333333"
	ordinaryChangedInstallationTree = "4444444444444444444444444444444444444444"
	ordinaryReuseInstallationTree   = "3535353535353535353535353535353535353535"
	ordinaryFailedInstallationTree  = "3636363636363636363636363636363636363636"
	ordinaryBrokenInstallationTree  = "3737373737373737373737373737373737373737"
	ordinaryEngineTree              = "5555555555555555555555555555555555555555"
	ordinaryChangedEngineTree       = "6666666666666666666666666666666666666666"
	ordinaryReuseEngineTree         = "5656565656565656565656565656565656565656"
	ordinaryFailedEngineTree        = "5757575757575757575757575757575757575757"
	ordinaryBrokenEngineTree        = "5858585858585858585858585858585858585858"
	ordinaryBaseCommit              = "7777777777777777777777777777777777777777"
	ordinarySnapshotCommit          = "8888888888888888888888888888888888888888"
	ordinaryBuildOne                = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	ordinaryBuildTwo                = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	ordinaryBuildThree              = "cccccccccccccccccccccccccccccccccccccccc"
	ordinaryBuildFour               = "dddddddddddddddddddddddddddddddddddddddd"
	ordinaryBuildFive               = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	ordinaryFailedBuild             = "f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1"
	ordinaryBrokenBuild             = "f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2"
	ordinarySourceBlob              = "9999999999999999999999999999999999999999"
	ordinaryBaseScriptBlob          = "abababababababababababababababababababab"
	ordinaryReuseScriptBlob         = "acacacacacacacacacacacacacacacacacacacac"
	ordinaryFailedScriptBlob        = "adadadadadadadadadadadadadadadadadadadad"
	ordinaryBrokenScriptBlob        = "aeaeaeaeaeaeaeaeaeaeaeaeaeaeaeaeaeaeaeae"
	ordinaryChangedSumBlob          = "bcbcbcbcbcbcbcbcbcbcbcbcbcbcbcbcbcbcbcbc"
	ordinaryRecordsTree             = "1313131313131313131313131313131313131313"
	ordinaryRecordsInstallationTree = "3939393939393939393939393939393939393939"
	ordinaryRecordsBlob             = "9a9a9a9a9a9a9a9a9a9a9a9a9a9a9a9a9a9a9a9a"
	ordinaryMovedHead               = "8989898989898989898989898989898989898989"
	ordinaryJudgeCmdTree            = "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1"
	ordinaryJudgeInternalTree       = "b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1"
	ordinaryJudgeModuleBlob         = "c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1"
)

var ordinaryProjectionPaths = []string{"cmd/**", "internal/**", "scripts/agents/**", "go.mod", "go.sum", "records/misc/goals-migration-manifest.md"}
var ordinaryWorkspacePins = []string{"-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false", "-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}

// The declared OIDs name fixture facts. The fixture never constructs Git objects
// or supplies a candidate-engine identity decision to production.
type ordinaryCandidateFixture struct {
	t                                                         *testing.T
	root, installation, policyEngine, policyDigest, denialLog string
	steps                                                     []ordinaryCandidateStep
	requests                                                  map[string]int
	detachedRoot                                              string
	opened, closed                                            int
	script, source, sum, records                              []byte
	sourceIndex, targetIndex                                  string
	sourceEntries, targetEntries                              []byte
	snapshots                                                 map[string]ordinaryCandidateSnapshot
	oidFacts                                                  map[string]string
	commitRequests, commitOIDs                                map[string]string
	messageFacts                                              map[string]ordinaryMessageFacts
	expectedGoPath                                            string
	expectedGoBytes                                           []byte
	currentTree                                               string
	mutableTree, detachedTree, headPredecessor                string
}
type ordinaryCandidateSnapshot struct {
	tree, installation, engine, scriptBlob, sourceBlob, sumBlob string
	script, source, sum, records                                []byte
	entries                                                     []byte
}
type ordinaryMessageFacts struct{ environmentDigest, closure string }
type ordinaryCandidateStep struct {
	kind        string
	detached    bool
	args        []string
	output      []byte
	stdin       []byte
	environment string
}
type ordinaryDetached struct {
	workspace gittree.Workspace
	root      string
	owner     *ordinaryCandidateFixture
}

func (d *ordinaryDetached) Workspace() gittree.Workspace { return d.workspace }
func (d *ordinaryDetached) Close() error                 { d.owner.closed++; return os.RemoveAll(d.root) }

func newOrdinaryCandidateFixture(t *testing.T) *ordinaryCandidateFixture {
	t.Helper()
	originalPath := os.Getenv("PATH")
	shim := t.TempDir()
	log := filepath.Join(t.TempDir(), "denied-git.log")
	if err := os.WriteFile(log, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(shim, "git"), []byte("#!/bin/sh\nprintf 'denied git invocation\\n' >> '"+log+"'\nexit 97\n"), 0o755)
	t.Setenv("PATH", shim+string(os.PathListSeparator)+originalPath)
	root := t.TempDir()
	f := &ordinaryCandidateFixture{t: t, root: root, installation: filepath.Join(root, "metasystem"), denialLog: log,
		snapshots: make(map[string]ordinaryCandidateSnapshot), oidFacts: make(map[string]string),
		commitRequests: make(map[string]string), commitOIDs: make(map[string]string), messageFacts: make(map[string]ordinaryMessageFacts), requests: make(map[string]int),
		source: []byte("candidate engine source\n")}
	f.script = []byte(`#!/usr/bin/env bash
set -euo pipefail
[[ "$1" == --trimpath && "$2" == --out && -n "${3:-}" ]]
[[ "${CGO_ENABLED+x}:$CGO_ENABLED" == x:0 ]]
[[ "${GOENV+x}:$GOENV" == x:off ]]
[[ "${GOFLAGS+x}:$GOFLAGS" == x:-mod=readonly ]]
[[ "${GOTOOLCHAIN+x}:$GOTOOLCHAIN" == x:local ]]
[[ "${GOWORK+x}:$GOWORK" == x:off ]]
source=$(cat cmd/metasystem/engine.txt)
[[ "$source" == 'candidate engine source' ]]
[[ -n "$METASYSTEM_BUILD_STAMP" ]]
printf '#!/usr/bin/env bash\nstamp=%q\n# source=%s\nexit 0\n' "$METASYSTEM_BUILD_STAMP" "$source" > "$3"
chmod +x "$3"
`)
	f.writeFiles()
	f.declareSnapshot(ordinaryProjectTree, ordinaryInstallationTree, ordinaryEngineTree, ordinaryBaseScriptBlob, "")
	f.policyEngine = filepath.Join(t.TempDir(), "policy-engine")
	writeTestingFixtureFile(t, f.policyEngine, []byte("#!/usr/bin/env bash\n# enrolled policy engine\nexit 0\n"), 0o755)
	digest, err := fileSHA256(f.policyEngine)
	if err != nil {
		t.Fatal(err)
	}
	f.policyDigest = digest
	t.Cleanup(func() {
		if len(f.steps) != 0 {
			t.Errorf("ordinary repository replies left unused: %d", len(f.steps))
		}
		if f.opened != f.closed {
			t.Errorf("detached workspace cleanup: opened=%d closed=%d", f.opened, f.closed)
		}
		denied, err := os.ReadFile(f.denialLog)
		if err != nil || len(denied) != 0 {
			t.Errorf("real Git was requested: %q, %v", denied, err)
		}
	})
	return f
}
func (f *ordinaryCandidateFixture) declareOID(oid, fact string) {
	f.t.Helper()
	if prior, exists := f.oidFacts[oid]; exists && prior != fact {
		f.t.Fatalf("synthetic OID %s names different tracked bytes or edges", oid)
	}
	f.oidFacts[oid] = fact
}
func (f *ordinaryCandidateFixture) declareSnapshot(tree, installation, engine, scriptBlob, sumBlob string) {
	f.t.Helper()
	if _, exists := f.snapshots[tree]; exists {
		f.t.Fatalf("synthetic project tree %s was declared twice", tree)
	}
	s := ordinaryCandidateSnapshot{tree: tree, installation: installation, engine: engine,
		scriptBlob: scriptBlob, sourceBlob: ordinarySourceBlob, sumBlob: sumBlob,
		script: bytes.Clone(f.script), source: bytes.Clone(f.source), sum: bytes.Clone(f.sum), records: bytes.Clone(f.records)}
	if (sumBlob == "") != (s.sum == nil) {
		f.t.Fatal("go.sum blob declaration differs from tracked go.sum bytes")
	}
	f.declareOID(scriptBlob, fmt.Sprintf("100755:%x", s.script))
	f.declareOID(s.sourceBlob, fmt.Sprintf("100644:%x", s.source))
	if sumBlob != "" {
		f.declareOID(sumBlob, fmt.Sprintf("100644:%x", s.sum))
	}
	recordsBlob := ""
	if s.records != nil {
		recordsBlob = ordinaryRecordsBlob
		f.declareOID(recordsBlob, fmt.Sprintf("100644:%x", s.records))
	}
	s.entries = []byte(fmt.Sprintf("100644 %s 0\tcmd/metasystem/engine.txt\x00100755 %s 0\tscripts/agents/go-build.sh\x00", s.sourceBlob, s.scriptBlob))
	if sumBlob != "" {
		s.entries = append(s.entries, []byte(fmt.Sprintf("100644 %s 0\tgo.sum\x00", sumBlob))...)
	}
	f.declareOID(engine, fmt.Sprintf("projection:%x", s.entries))
	f.declareOID(installation, fmt.Sprintf("installation:%s:%s:%s:%s", s.scriptBlob, s.sourceBlob, s.sumBlob, recordsBlob))
	f.declareOID(tree, "project:metasystem="+installation)
	f.snapshots[tree] = s
	f.mutableTree = tree
}
func (f *ordinaryCandidateFixture) snapshot(tree string) ordinaryCandidateSnapshot {
	f.t.Helper()
	s, ok := f.snapshots[tree]
	if !ok {
		f.t.Fatalf("undeclared synthetic project tree %s", tree)
	}
	if tree != f.mutableTree {
		return s
	}
	if !bytes.Equal(f.script, s.script) || !bytes.Equal(f.source, s.source) || !bytes.Equal(f.sum, s.sum) || !bytes.Equal(f.records, s.records) {
		f.t.Fatalf("current tracked source differs from declared tree %s", tree)
	}
	for _, file := range []struct {
		path string
		data []byte
		mode os.FileMode
	}{
		{"scripts/agents/go-build.sh", s.script, 0o755},
		{"cmd/metasystem/engine.txt", s.source, 0o644},
		{"go.sum", s.sum, 0o644},
		{"records/counselor/peer.md", s.records, 0o644},
	} {
		path := filepath.Join(f.installation, file.path)
		data, err := os.ReadFile(path)
		if file.data == nil && os.IsNotExist(err) {
			continue
		}
		info, statErr := os.Stat(path)
		if err != nil || statErr != nil || !bytes.Equal(data, file.data) || info.Mode().Perm() != file.mode || !info.Mode().IsRegular() {
			f.t.Fatalf("tracked %s differs from declared bytes/mode: read=%v stat=%v", file.path, err, statErr)
		}
	}
	return s
}
func (f *ordinaryCandidateFixture) writeFiles() {
	f.t.Helper()
	writeTestingFixtureFile(f.t, filepath.Join(f.installation, "scripts/agents/go-build.sh"), f.script, 0o755)
	writeTestingFixtureFile(f.t, filepath.Join(f.installation, "cmd/metasystem/engine.txt"), f.source, 0o644)
	if f.sum != nil {
		writeTestingFixtureFile(f.t, filepath.Join(f.installation, "go.sum"), f.sum, 0o644)
	}
	if f.records != nil {
		writeTestingFixtureFile(f.t, filepath.Join(f.installation, "records/counselor/peer.md"), f.records, 0o644)
	}
}
func (f *ordinaryCandidateFixture) workspace() gittree.Workspace {
	return gittree.Workspace{Dir: f.root, RawSource: f.raw}
}
func (f *ordinaryCandidateFixture) dependency() candidateEngineIO {
	return candidateEngineIO{runGit: f.runGit, open: f.open}
}
func (f *ordinaryCandidateFixture) take(kind string) ordinaryCandidateStep {
	f.t.Helper()
	if len(f.steps) == 0 {
		f.t.Fatalf("unexpected %s request after all declared replies", kind)
	}
	step := f.steps[0]
	f.steps = f.steps[1:]
	f.requests[kind]++
	if step.kind != kind {
		f.t.Fatalf("request kind %s, want %s args=%q", kind, step.kind, step.args)
	}
	return step
}
func (f *ordinaryCandidateFixture) directory(detached bool) string {
	if detached {
		return f.detachedRoot
	}
	return f.root
}
func (f *ordinaryCandidateFixture) queueRaw(detached bool, output string, args ...string) {
	f.steps = append(f.steps, ordinaryCandidateStep{kind: "raw", detached: detached, args: args, output: []byte(output)})
}
func (f *ordinaryCandidateFixture) queueGit(detached bool, environment string, stdin, output []byte, args ...string) {
	f.steps = append(f.steps, ordinaryCandidateStep{kind: "git", detached: detached, args: args, stdin: stdin, output: output, environment: environment})
}
func (f *ordinaryCandidateFixture) raw(request gittree.RawRequest) gittree.RawResult {
	step := f.take("raw")
	dir := f.directory(step.detached)
	want := append(append([]string{"-C", dir}, ordinaryWorkspacePins...), step.args...)
	if request.Dir != dir || !reflect.DeepEqual(request.Args, want) || !reflect.DeepEqual(request.Env, gittree.ScrubbedEnviron()) || len(request.Stdin) != 0 || request.Operation != "git "+strings.Join(step.args, " ") {
		f.t.Fatalf("raw Git request: dir=%q args=%q env=%q stdin=%q operation=%q; want dir=%q args=%q", request.Dir, request.Args, request.Env, request.Stdin, request.Operation, dir, want)
	}
	return gittree.RawResult{Stdout: step.output}
}
func (f *ordinaryCandidateFixture) runGit(command *exec.Cmd) error {
	step := f.take("git")
	dir := f.directory(step.detached)
	if filepath.Base(command.Path) != "git" || command.Dir != "" || len(command.Args) < 3 || command.Args[0] != "git" || !reflect.DeepEqual(command.Args[1:3], []string{"-C", dir}) {
		f.t.Fatalf("direct Git command path=%q dir=%q args=%q", command.Path, command.Dir, command.Args)
	}
	args := command.Args[3:]
	if step.environment == "source" || step.environment == "target" {
		if !reflect.DeepEqual(args[:4], []string{"-c", "core.fileMode=true", "-c", "core.useReplaceRefs=false"}) {
			f.t.Fatalf("projection pins: %q", args)
		}
		args = args[4:]
	}
	if !reflect.DeepEqual(args, step.args) {
		f.t.Fatalf("Git args=%q, want %q", args, step.args)
	}
	var wantEnv []string
	switch step.environment {
	case "source", "target":
		if step.environment == "source" && len(args) > 0 && args[0] == "read-tree" {
			f.targetIndex = ""
		}
		index := f.sourceIndex
		if step.environment == "target" {
			index = f.targetIndex
		}
		actual := ""
		for _, entry := range command.Env {
			if strings.HasPrefix(entry, "GIT_INDEX_FILE=") {
				actual = strings.TrimPrefix(entry, "GIT_INDEX_FILE=")
			}
		}
		if actual == "" {
			f.t.Fatal("projection index was absent")
		}
		parent := filepath.Dir(actual)
		if !filepath.IsAbs(actual) || !strings.HasPrefix(filepath.Base(parent), "metasystem-engine-projection.") ||
			filepath.Dir(parent) != os.TempDir() || actual != filepath.Join(parent, step.environment+"-index") {
			f.t.Fatalf("projection index is not its exact private scratch filename: %q", actual)
		}
		other := f.targetIndex
		if step.environment == "target" {
			other = f.sourceIndex
			if other == "" {
				f.t.Fatal("target index opened before source index")
			}
		}
		if other != "" && (other == actual || filepath.Dir(other) != parent) {
			f.t.Fatalf("projection indexes are shared or in different scratch directories: %q %q", actual, other)
		}
		if len(args) > 0 && args[0] == "read-tree" {
			index = ""
		}
		if index == "" {
			if step.environment == "source" {
				f.sourceIndex = actual
			} else {
				f.targetIndex = actual
			}
		} else if index != actual {
			f.t.Fatalf("projection index changed: %q != %q", actual, index)
		}
		wantEnv = gittree.ScrubbedEnviron("GIT_INDEX_FILE=" + actual)
	case "commit":
		wantEnv = f.commitEnvironment()
	case "plain":
		wantEnv = gittree.ScrubbedEnviron()
	default:
		f.t.Fatalf("unknown environment %q", step.environment)
	}
	if !reflect.DeepEqual(command.Env, wantEnv) {
		f.t.Fatalf("Git environment does not match pinned %s environment", step.environment)
	}
	var actualStdin []byte
	var err error
	if command.Stdin != nil {
		actualStdin, err = io.ReadAll(command.Stdin)
	}
	if err != nil || !bytes.Equal(actualStdin, step.stdin) {
		f.t.Fatalf("Git stdin=%q want=%q err=%v", actualStdin, step.stdin, err)
	}
	if step.environment == "source" || step.environment == "target" {
		s := f.snapshot(f.currentTree)
		switch {
		case step.environment == "source" && len(args) > 0 && args[0] == "read-tree":
			f.sourceEntries, f.targetEntries = bytes.Clone(s.entries), nil
		case step.environment == "source" && len(args) > 0 && args[0] == "ls-files":
			if !bytes.Equal(step.output, f.sourceEntries) {
				f.t.Fatal("source index selection differs from declared entries")
			}
		case step.environment == "target" && len(args) > 0 && args[0] == "read-tree":
			f.targetEntries = nil
		case step.environment == "target" && len(args) > 0 && args[0] == "update-index":
			if !bytes.Equal(actualStdin, f.sourceEntries) {
				f.t.Fatal("target index did not receive selected source entries")
			}
			f.targetEntries = bytes.Clone(actualStdin)
		case step.environment == "target" && len(args) > 0 && args[0] == "write-tree":
			if !bytes.Equal(f.targetEntries, s.entries) || !bytes.Equal(step.output, []byte(s.engine+"\n")) {
				f.t.Fatal("target index contents or projection tree differ from declared snapshot")
			}
		}
	}
	if command.Stdout == nil || command.Stderr == nil {
		f.t.Fatal("Git streams not assigned")
	}
	if (step.environment == "commit" || step.environment == "plain") != (command.Stdout == command.Stderr) {
		f.t.Fatal("combined Git output did not retain one stream order")
	}
	if step.environment == "commit" {
		// The root and detached copies represent the same declared project tree.
		// Their private filesystem names cannot change a commit object.
		request := fmt.Sprintf("argv=%q env=%q stdin=%x", command.Args[3:], command.Env, actualStdin)
		oid := strings.TrimSpace(string(step.output))
		if prior, ok := f.commitRequests[request]; ok && prior != oid {
			f.t.Fatalf("identical commit-tree request returned %s and %s", prior, oid)
		}
		if prior, ok := f.commitOIDs[oid]; ok && prior != request {
			f.t.Fatalf("commit OID %s represents different complete requests", oid)
		}
		f.commitRequests[request], f.commitOIDs[oid] = oid, request
	}
	_, err = command.Stdout.Write(step.output)
	return err
}
func (f *ordinaryCandidateFixture) commitEnvironment() []string {
	env := gittree.ScrubbedEnviron()
	out := make([]string, 0, len(env)+2)
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		switch name {
		case "GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL", "GIT_AUTHOR_DATE", "GIT_COMMITTER_DATE":
			continue
		}
		out = append(out, entry)
	}
	return append(out, "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z")
}
func ordinaryExpectedBuildEnvironment(environment []string) []string {
	excluded := map[string]bool{
		"METASYSTEM_PROOF_CONTROL_ROOT": true, "METASYSTEM_PROOF_ATTEMPT": true,
		"METASYSTEM_PROOF_RECORD_KEY": true, "METASYSTEM_PROOF_CREATION_CLAIM": true,
		"METASYSTEM_PROOF_AUTH_BIN": true, "METASYSTEM_RUN_OWNER": true,
		"METASYSTEM_FIXTURE_ATTEMPT": true,
		"CGO_ENABLED":                true, "GOAMD64": true, "GOARM": true, "GOARM64": true,
		"GOENV": true, "GOEXPERIMENT": true, "GOFLAGS": true, "GOTOOLCHAIN": true,
		"GOWORK": true, "METASYSTEM_BUILD_STAMP": true,
	}
	result := make([]string, 0, len(environment)+10)
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if !excluded[name] {
			result = append(result, entry)
		}
	}
	return append(result, "CGO_ENABLED=0", "GOAMD64=v1", "GOARM64=v8.0", "GOARM=7", "GOENV=off",
		"GOEXPERIMENT=", "GOFLAGS=-mod=readonly", "GOTOOLCHAIN=local", "GOWORK=off", "METASYSTEM_BUILD_STAMP=")
}
func (f *ordinaryCandidateFixture) prepareDetached(tree string) {
	s := f.snapshot(tree)
	parent := f.t.TempDir()
	f.detachedRoot, f.detachedTree = parent, tree
	writeTestingFixtureFile(f.t, filepath.Join(parent, "metasystem/scripts/agents/go-build.sh"), s.script, 0o755)
	writeTestingFixtureFile(f.t, filepath.Join(parent, "metasystem/cmd/metasystem/engine.txt"), s.source, 0o644)
	if s.sum != nil {
		writeTestingFixtureFile(f.t, filepath.Join(parent, "metasystem/go.sum"), s.sum, 0o644)
	}
	if s.records != nil {
		writeTestingFixtureFile(f.t, filepath.Join(parent, "metasystem/records/counselor/peer.md"), s.records, 0o644)
	}
}
func (f *ordinaryCandidateFixture) open(workspace gittree.Workspace, tree string) (candidateDetachedWorkspace, error) {
	step := f.take("open")
	if tree != string(step.output) || workspace.Dir != f.root || workspace.RawSource == nil {
		f.t.Fatalf("open workspace=%q tree=%q want=%q", workspace.Dir, tree, step.output)
	}
	if _, err := os.Stat(f.detachedRoot); f.detachedTree != tree || os.IsNotExist(err) {
		f.prepareDetached(tree)
	}
	s := f.snapshot(tree)
	parent := f.detachedRoot
	f.opened++
	for _, file := range []struct {
		path string
		data []byte
		mode os.FileMode
	}{
		{"scripts/agents/go-build.sh", s.script, 0o755}, {"cmd/metasystem/engine.txt", s.source, 0o644},
		{"go.sum", s.sum, 0o644}, {"records/counselor/peer.md", s.records, 0o644},
	} {
		path := filepath.Join(parent, "metasystem", file.path)
		data, err := os.ReadFile(path)
		if file.data == nil && os.IsNotExist(err) {
			continue
		}
		info, statErr := os.Stat(path)
		if err != nil || statErr != nil || !bytes.Equal(data, file.data) || info.Mode().Perm() != file.mode || !info.Mode().IsRegular() {
			f.t.Fatalf("materialized %s differs from declared bytes/mode: read=%v stat=%v", file.path, err, statErr)
		}
	}
	return &ordinaryDetached{workspace: gittree.Workspace{Dir: parent, RawSource: f.raw}, root: parent, owner: f}, nil
}
func (f *ordinaryCandidateFixture) queueIdentity(tree, engineTree, commit string, environment []string, detached bool) {
	s := f.snapshot(tree)
	if detached {
		if _, err := os.Stat(f.detachedRoot); f.detachedTree != tree || os.IsNotExist(err) {
			f.prepareDetached(tree)
		}
	}
	if engineTree != s.engine {
		f.t.Fatalf("projection tree %s differs from declared %s", engineTree, s.engine)
	}
	f.currentTree = tree
	f.queueRaw(detached, s.installation+"\n", "rev-parse", "--verify", tree+":metasystem")
	f.queueRaw(detached, "tree\n", "cat-file", "-t", s.installation)
	f.queueGit(detached, "source", nil, nil, "read-tree", s.installation)
	f.queueGit(detached, "source", nil, bytes.Clone(s.entries), append([]string{"ls-files", "-s", "-z", "--"}, ordinaryProjectionPaths...)...)
	f.queueGit(detached, "target", nil, nil, "read-tree", "--empty")
	f.queueGit(detached, "target", bytes.Clone(s.entries), nil, "update-index", "-z", "--index-info")
	f.queueGit(detached, "target", nil, []byte(engineTree+"\n"), "write-tree")
	buildEnv := ordinaryExpectedBuildEnvironment(environment)
	installation := f.installation
	if detached {
		installation = filepath.Join(f.detachedRoot, "metasystem")
	}
	closure, err := proofrun.ToolchainClosureIdentity(installation, buildEnv)
	if err != nil {
		f.t.Fatal(err)
	}
	environmentDigest := bytesSHA256([]byte(strings.Join(buildEnv, "\x00")))
	facts := ordinaryMessageFacts{environmentDigest: environmentDigest, closure: closure}
	if prior, ok := f.messageFacts[commit]; ok && prior != facts {
		f.t.Fatalf("synthetic commit %s changed its message facts", commit)
	}
	if commit == ordinaryBuildTwo && (!bytes.Equal(s.sum, []byte("changed sum\n")) || tree != ordinaryChangedTree || s.engine == ordinaryReuseEngineTree) {
		f.t.Fatal("go.sum variation lacks its declared tracked bytes and tree edge")
	}
	if commit == ordinaryBuildThree {
		prior, ok := f.messageFacts[ordinaryBuildTwo]
		if !ok || !containsExactEntry(buildEnv, "METASYSTEM_FIXTURE_BUILD_MODE=changed") || prior.environmentDigest == environmentDigest {
			f.t.Fatal("declared build-mode input did not change the commit request's environment component")
		}
	}
	if commit == ordinaryBuildFour {
		prior, ok := f.messageFacts[ordinaryBuildThree]
		path := ""
		for _, entry := range buildEnv {
			if strings.HasPrefix(entry, "PATH=") {
				path = strings.TrimPrefix(entry, "PATH=")
			}
		}
		selected := filepath.Join(strings.Split(path, string(os.PathListSeparator))[0], "go")
		actual, readErr := os.ReadFile(selected)
		if !ok || selected != f.expectedGoPath || readErr != nil || !bytes.Equal(actual, f.expectedGoBytes) || prior.closure == closure {
			f.t.Fatalf("declared Go executable did not change the commit request's toolchain component: selected=%q expected=%q read=%v", selected, f.expectedGoPath, readErr)
		}
	}
	f.messageFacts[commit] = facts
	message := strings.Join([]string{"stable candidate proof snapshot", "engine-tree=" + engineTree, "toolchain-closure=" + closure,
		"build-environment=" + environmentDigest, "platform=" + runtime.GOOS + "/" + runtime.GOARCH,
		"build-context=CGO_ENABLED=0,GOAMD64=v1,GOARM64=v8.0,GOARM=7,GOENV=off,GOEXPERIMENT=,GOFLAGS=-mod=readonly,GOTOOLCHAIN=local,GOWORK=off,-buildvcs=false,-trimpath"}, "\n")
	f.queueGit(detached, "commit", nil, []byte(commit+"\n"), "-c", "user.name=MetaSystem", "-c", "user.email=metasystem@invalid",
		"-c", "author.name=MetaSystem", "-c", "author.email=metasystem@invalid", "-c", "committer.name=MetaSystem", "-c", "committer.email=metasystem@invalid",
		"-c", "i18n.commitEncoding=UTF-8", "commit-tree", engineTree, "-m", message)
}
func containsExactEntry(entries []string, want string) bool {
	for _, entry := range entries {
		if entry == want {
			return true
		}
	}
	return false
}
func (f *ordinaryCandidateFixture) queuePrepare(tree, engineTree, commit string, environment []string, build bool) {
	if build {
		f.prepareDetached(tree)
	}
	f.queueIdentity(tree, engineTree, commit, environment, false)
	if !build {
		return
	}
	f.steps = append(f.steps, ordinaryCandidateStep{kind: "open", output: []byte(tree)})
	f.queueRaw(true, tree+"\n", "rev-parse", "HEAD^{tree}")
	f.queueRaw(true, "", "rev-parse", "--show-prefix")
	f.queueIdentity(tree, engineTree, commit, environment, true)
	f.queueHeadUpdate(commit)
}
func (f *ordinaryCandidateFixture) queueHeadUpdate(commit string) {
	predecessor := f.headPredecessor
	if predecessor == "" {
		predecessor = ordinarySnapshotCommit
	}
	f.queueRaw(true, predecessor+"\n", "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	f.queueGit(true, "plain", nil, nil, "update-ref", "--no-deref", "HEAD", commit, predecessor)
}
func (f *ordinaryCandidateFixture) queueBuild(tree, engineTree, commit string, environment []string) {
	f.prepareDetached(tree)
	f.steps = append(f.steps, ordinaryCandidateStep{kind: "open", output: []byte(tree)})
	f.queueRaw(true, tree+"\n", "rev-parse", "HEAD^{tree}")
	f.queueRaw(true, "", "rev-parse", "--show-prefix")
	f.queueIdentity(tree, engineTree, commit, environment, true)
	f.queueHeadUpdate(commit)
}
func (f *ordinaryCandidateFixture) queueJudge() {
	f.declareOID(ordinaryBaseCommit, "base commit with declared judge sources")
	for _, item := range []struct{ id, fact string }{
		{ordinaryJudgeCmdTree, "base cmd tree"}, {ordinaryJudgeInternalTree, "base internal tree"},
		{ordinaryJudgeModuleBlob, "base go.mod blob"},
	} {
		f.declareOID(item.id, item.fact)
	}
	f.steps = append(f.steps, ordinaryCandidateStep{kind: "judge", args: []string{"rev-parse", "--verify", "--quiet", ordinaryBaseCommit + "^{commit}"}, output: []byte(ordinaryBaseCommit)})
	for _, source := range []struct{ path, mode, kind, id string }{
		{"cmd", "040000", "tree", ordinaryJudgeCmdTree},
		{"internal", "040000", "tree", ordinaryJudgeInternalTree},
		{"go.mod", "100644", "blob", ordinaryJudgeModuleBlob},
		{"go.sum", "", "", ""},
	} {
		path := "metasystem/" + source.path
		output := ""
		if source.id != "" {
			output = fmt.Sprintf("%s %s %s\t%s\x00", source.mode, source.kind, source.id, path)
		}
		f.steps = append(f.steps, ordinaryCandidateStep{kind: "judge", args: []string{"ls-tree", "-z", "--full-tree", ordinaryBaseCommit, "--", path}, output: []byte(output)})
	}
}
func (f *ordinaryCandidateFixture) judgeReader(_ context.Context, root string, args ...string) (string, error) {
	step := f.take("judge")
	if root != f.root || !reflect.DeepEqual(args, step.args) {
		f.t.Fatalf("judge source read: root=%q args=%q; want root=%q args=%q", root, args, f.root, step.args)
	}
	return string(step.output), nil
}
func (f *ordinaryCandidateFixture) queueBed(tree string) {
	f.steps = append(f.steps, ordinaryCandidateStep{kind: "open", output: []byte(tree)})
}
func (f *ordinaryCandidateFixture) openBed(root, tree string) (proofrun.CandidateWorkspace, error) {
	if root != f.root {
		f.t.Fatalf("candidate bed root=%q want=%q", root, f.root)
	}
	return f.open(f.workspace(), tree)
}
func (f *ordinaryCandidateFixture) assertExecutable(artifact *candidateEngineBuild, commit string) {
	f.t.Helper()
	if artifact == nil || artifact.Commit != commit {
		f.t.Fatalf("artifact commit=%+v want %s", artifact, commit)
	}
	data, err := os.ReadFile(artifact.Path)
	if err != nil {
		f.t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("stamp="+commit)) || !bytes.Contains(data, []byte("source=candidate engine source")) {
		f.t.Fatalf("built bytes did not carry bound stamp and source: %q", data)
	}
	digest, err := fileSHA256(artifact.Path)
	if err != nil || digest != artifact.Digest {
		f.t.Fatalf("artifact digest=%q actual=%q err=%v", artifact.Digest, digest, err)
	}
}
func (f *ordinaryCandidateFixture) assertDrained() {
	f.t.Helper()
	if len(f.steps) != 0 {
		f.t.Fatalf("unused repository replies: %d", len(f.steps))
	}
}

package missionrunner

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The armed-preflight fixture: a real
// git checkout with frozen instruments, a contract sealed and signed
// in-test, a bare origin carrying the signed bytes, live tagged holder
// processes behind fabricated-but-honest supervision facts, and a stub
// armer whose fingerprint agrees at seal and preflight by construction —
// the exact recipe scripts/agents/mission-fixtures.sh uses, ported.

const fixtureContract = `# Intent

Reach the declared score with frozen instruments.

# Non-goals

Do not publish or deploy.

# Initial streams

Keep one stream active when the other needs a reserved decision.

` + "```mission" + `
gate.command=scripts/gate.sh
gate.ref=instruments
gate.paths=scripts/gate.sh
truth.paths=truth/*.txt
truth.certification=certified
gate.direction=max
gate.threshold.score=>=1
gate.noise-floor.score=0
guard.audit.command=scripts/gate.sh
guard.audit.floor=1
guard.audit.noise=0
guard.cadence=1
ledger.cycle-budget=3
ledger.no-gain-budget=3
fence.wall-clock-hours=2
fence.cycles=3
fence.jobs=4
fence.concurrency=2
fence.job-cap-min=30
host.runtime=fake
host.model=fake-model
host.turn-cap-min=30
stream.primary=Reach the acceptance score.
envelope.dependencies=jq
exposure=EUR:10
` + "```" + `
`

func fixtureGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// spawnTaggedHold starts a live process whose argv carries the tag, and
// returns its pid and kernel start time. The caller's cleanup closes its hold
// pipe and waits for the process to be reaped.
func spawnTaggedHold(t *testing.T, tag string) (int, int64) {
	t.Helper()
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	holdRead, holdWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", `printf x >&3; read -r _ <&4`,
		"metasystem", "util", "hold", "--tag", tag)
	cmd.ExtraFiles = []*os.File{readyWrite, holdRead}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	readyWrite.Close()
	holdRead.Close()
	waited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(waited) }()
	t.Cleanup(func() {
		holdWrite.Close()
		<-waited
		readyRead.Close()
	})
	ready := []byte{0}
	if _, err := io.ReadFull(readyRead, ready); err != nil || ready[0] != 'x' {
		t.Fatalf("tagged holder pid %d did not publish readiness: byte=%q err=%v", cmd.Process.Pid, ready, err)
	}
	exact, state, err := identity.KernelProber{}.Probe(int64(cmd.Process.Pid))
	if err != nil || state != identity.Alive || !exact.ArgvKnown ||
		!strings.Contains(strings.Join(exact.Argv, " "), tag) {
		t.Fatalf("ready tagged holder pid %d was not visible: state=%s argvKnown=%v argv=%q err=%v",
			cmd.Process.Pid, state, exact.ArgvKnown, exact.Argv, err)
	}
	return cmd.Process.Pid, exact.StartedAt.Unix()
}

// buildPreflightRootWithStream appends a directive to the contract's
// primary stream text, which the prompt carries and the fake host reads.
// commitBedBaseline closes a fixture bed's tracked-space setup into a
// commit: the wall's start preflight demands a CLEAN initial
// baseline, exactly as a real mission repository starts.
func commitBedBaseline(t *testing.T, root string) {
	t.Helper()
	fixtureGit(t, root, "add", "-A", ".")
	status := exec.Command("git", "-C", root, "status", "--porcelain", "--", ".")
	out, err := status.CombinedOutput()
	if err != nil {
		t.Fatalf("bed baseline status: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		fixtureGit(t, root, "commit", "-qm", "bed setup baseline")
	}
}

func buildPreflightRootWithStream(t *testing.T, directive string) *Engine {
	t.Helper()
	return buildPreflightBed(t, directive, false)
}

// buildPreflightBed builds the launch-gate world. With nested true, the
// git repository roots at a PARENT directory and the whole bed lives in
// its metasystem/ subdirectory — the supported deployment layout, whose
// git trees carry a path prefix the toplevel layout's do not.
func buildPreflightBed(t *testing.T, directive string, nested bool) *Engine {
	t.Helper()
	root := t.TempDir()
	gitInitDir := root
	if nested {
		gitInitDir = t.TempDir()
		root = filepath.Join(gitInitDir, "metasystem")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	remote := filepath.Join(t.TempDir(), "origin.git")
	for _, dir := range []string{"scripts/agents", "truth", "plans", "docs"} {
		os.MkdirAll(filepath.Join(root, dir), 0o755)
	}
	testexec.WriteFile(filepath.Join(root, "scripts", "gate.sh"),
		[]byte("#!/usr/bin/env bash\nset -euo pipefail\nprintf 'metric=score=1\\n'\n"), 0o755)
	os.WriteFile(filepath.Join(root, "truth", "reference.txt"), []byte("certified truth\n"), 0o644)
	// The seal reads the pre-authorization table from project-rules; the
	// fixture carries the real repository's table verbatim.
	rules, err := os.ReadFile(filepath.Join("..", "..", "docs", "project-rules.md"))
	if err != nil {
		t.Skipf("project rules not readable from the test working directory: %v", err)
	}
	os.WriteFile(filepath.Join(root, "docs", "project-rules.md"), rules, 0o644)
	if nested {
		// The in-process seal and preflight resolve the project root
		// through the running binary's own location; from the test binary
		// that resolution lands on the repository toplevel, so the rules
		// must be readable there too.
		os.MkdirAll(filepath.Join(gitInitDir, "docs"), 0o755)
		os.WriteFile(filepath.Join(gitInitDir, "docs", "project-rules.md"), rules, 0o644)
	}
	os.WriteFile(filepath.Join(root, "metasystem.conf"),
		[]byte("metasystem.runtimes=fake\nrole.default.runtime=fake\n"), 0o644)
	// The stub armer: typed armed outcome for arming, a fixed fingerprint for both seal
	// and preflight — agreement by construction.
	testexec.WriteFile(filepath.Join(root, "scripts", "agents", "arm-supervision.sh"), []byte(
		"#!/usr/bin/env bash\nset -euo pipefail\n"+
			"if [[ ${1:-} == fingerprint ]]; then printf 'fixture-fingerprint\\n'; exit 0; fi\n"+
			"printf 'up outcome=armed authority=writer\\n'\n"), 0o755)

	fixtureGit(t, gitInitDir, "init", "-q", "-b", "main")
	fixtureGit(t, root, "config", "user.name", "fixture")
	fixtureGit(t, root, "config", "user.email", "fixture@example.invalid")
	// The fixture mirrors the deployment's projection boundary: runtime
	// state under artifacts/ (and the staged binary) is ignored, so the
	// wall's shippable snapshot excludes it exactly as in the real repo.
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("artifacts/\nbin/\nmetasystem.conf\n"), 0o644)
	fixtureGit(t, root, "add", ".gitignore", "scripts", "truth", "docs")
	fixtureGit(t, root, "commit", "-qm", "instruments")
	fixtureGit(t, root, "tag", "instruments")
	gitInitBare := exec.Command("git", "init", "-q", "-b", "main", "--bare", remote)
	if out, err := gitInitBare.CombinedOutput(); err != nil {
		t.Fatalf("bare init: %v\n%s", err, out)
	}
	fixtureGit(t, root, "remote", "add", "origin", remote)
	fixtureGit(t, root, "push", "-qu", "origin", "main")
	fixtureGit(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	// Older git does not mint origin/HEAD on push; the origin verification
	// resolves through it, and the shell fixture sets it explicitly too.
	fixtureGit(t, root, "remote", "set-head", "origin", "-a")

	// The contract: written, sealed, signed, committed, pushed.
	engine := &Engine{Root: root, Mission: "alpha"}
	contractPath := engine.contractPath()
	document := fixtureContract
	if directive != "" {
		document = strings.Replace(document,
			"stream.primary=Reach the acceptance score.",
			"stream.primary=Reach the acceptance score. "+directive, 1)
	}
	os.WriteFile(contractPath, []byte(document), 0o644)
	var sha string
	if nested {
		// Sealing resolves the project root through the sealing binary's
		// own location. The test binary lands on the repository toplevel,
		// which is wrong for a nested bed — the bed's own engine binary
		// self-locates at the bed root, exactly as deployed.
		binary, rerr := os.ReadFile(freshEngineBinary(t))
		if rerr != nil {
			t.Fatal(rerr)
		}
		os.MkdirAll(filepath.Join(root, "bin"), 0o755)
		if werr := testexec.WriteFile(filepath.Join(root, "bin", "metasystem"), binary, 0o755); werr != nil {
			t.Fatal(werr)
		}
		stdout, stderr, code := runCaptured(root, nil,
			filepath.Join(root, "bin", "metasystem"), "mission", "contract-seal", "--file", contractPath)
		if code != 0 {
			t.Fatalf("seal through the bed binary: exit %d\n%s%s", code, stdout, stderr)
		}
		sha = strings.TrimSpace(stdout)
	} else {
		sealed, err := contract.Seal(contractPath)
		if err != nil {
			t.Fatalf("seal: %v", err)
		}
		sha = sealed
	}
	approval := "\nApproval: name=Fixture Human; date=2026-08-12; contract-sha256=" + sha + "\n"
	handle, _ := os.OpenFile(contractPath, os.O_APPEND|os.O_WRONLY, 0o644)
	handle.WriteString(approval)
	handle.Close()
	fixtureGit(t, root, "add", "plans")
	fixtureGit(t, root, "commit", "-qm", "sign mission contract")
	fixtureGit(t, root, "push", "-q", "origin", "main")

	writeFreshSupervision(t, engine)
	return engine
}

// writeFreshSupervision gives each copied bed its own live process facts.
// A copied checkout may reuse immutable Git objects, but process identity and
// heartbeat paths belong to the test that owns the copy.
func writeFreshSupervision(t *testing.T, engine *Engine) {
	t.Helper()
	root := engine.Root
	stableNow := time.Now().UTC().Truncate(time.Second)
	engine.Now = func() time.Time { return stableNow }
	watcherPid, watcherStart := spawnTaggedHold(t, "fixture-watcher-tag")
	reaperPid, reaperStart := spawnTaggedHold(t, "fixture-reaper-tag")
	landingPid, landingStart := spawnTaggedHold(t, "fixture-landing-owner-tag")
	supervision := filepath.Join(root, "artifacts", "agents", "supervision")
	os.MkdirAll(supervision, 0o755)
	now := stableNow.Unix()
	watcherBeat := filepath.Join(supervision, "watcher.heartbeat.json")
	reaperBeat := filepath.Join(supervision, "reaper.heartbeat.json")
	landingBeat := filepath.Join(supervision, "landing-owner.heartbeat.json")
	writeDoc := func(path string, doc map[string]any) {
		if err := atomicWriteJSON(path, doc); err != nil {
			t.Fatal(err)
		}
	}
	writeDoc(watcherBeat, map[string]any{
		"function": "watcher", "pid": watcherPid, "pidStartedAt": watcherStart, "observedAtEpoch": now, "loadedCapMin": 330,
	})
	writeDoc(reaperBeat, map[string]any{
		"function": "reaper", "pid": reaperPid, "pidStartedAt": reaperStart, "observedAtEpoch": now,
	})
	writeDoc(landingBeat, map[string]any{
		"function": "landing-owner", "pid": landingPid, "pidStartedAt": landingStart, "observedAtEpoch": now,
	})
	writeDoc(filepath.Join(supervision, "state.json"), map[string]any{
		"intervalSec": 60, "fingerprint": "fixture-fingerprint", "generation": 1, "derivedWatcherCapMin": 330,
		"components": map[string]any{
			"watcher": map[string]any{"pid": watcherPid, "pidStartedAt": watcherStart,
				"instanceTag": "fixture-watcher-tag", "heartbeat": watcherBeat},
			"reaper": map[string]any{"pid": reaperPid, "pidStartedAt": reaperStart,
				"instanceTag": "fixture-reaper-tag", "heartbeat": reaperBeat},
			"landing-owner": map[string]any{"pid": landingPid, "pidStartedAt": landingStart,
				"instanceTag": "fixture-landing-owner-tag", "heartbeat": landingBeat},
		},
	})
	writeDoc(filepath.Join(supervision, "last-census.json"), map[string]any{
		"completedAtEpoch": now, "verdict": "SUCCESS", "fingerprint": "fixture-fingerprint", "generation": 1,
	})
	inspection := supervise.InspectArmedAt(filepath.Join(root, "artifacts", "agents"), root,
		int64(watcherPid), watcherStart, "fixture-watcher-tag", 60, stableNow)
	if !inspection.Armed() {
		t.Fatalf("fresh supervision did not pass at its semantic instant: %+v", inspection)
	}
}

// The full launch gate passes in-process: arming, seal verification,
// approval, origin, the gate command, supervision facts, the lease probe,
// and the pin landing the approved bytes into the fences.
func TestArmAndPreflightFullPass(t *testing.T) {
	engine, source := newGitFreePreflightBed(t, "")
	t.Cleanup(source.done)
	if err := engine.armAndPreflight("start"); err != nil {
		t.Fatalf("the full launch gate refused: %v", err)
	}
	pinned, err := os.ReadFile(engine.approvedContractPath())
	if err != nil {
		t.Fatalf("approved bytes not pinned: %v", err)
	}
	if !strings.Contains(string(pinned), "Approval: name=Fixture Human") {
		t.Fatal("the pinned bytes are not the signed contract")
	}
	signed, err := os.ReadFile(engine.contractPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(pinned) != string(signed) {
		t.Fatal("the pin differs from the approved signed bytes")
	}
	fences, err := readJSONDoc(engine.fencesPath())
	if err != nil {
		t.Fatalf("fences unreadable: %v", err)
	}
	sha, _ := fences["approvedContractSha256"].(string)
	if len(sha) != 64 {
		t.Fatalf("fences carry no contract sha: %v", fences["approvedContractSha256"])
	}
	if sha != fmt.Sprintf("%x", sha256.Sum256(pinned)) {
		t.Fatalf("fences sha does not match the pinned signed bytes: %q", sha)
	}
	// Until the mission is BORN, a second start may re-pin (the
	// stillborn rule); once state.json exists, it steers to resume.
	if err := engine.armAndPreflight("start"); err != nil {
		t.Fatalf("a stillborn pin must be re-pinnable: %v", err)
	}
	os.MkdirAll(engine.missionDir(), 0o755)
	writeText(t, filepath.Join(engine.missionDir(), "state.json"), "{}")
	if err := engine.armAndPreflight("start"); err == nil ||
		!strings.Contains(err.Error(), "already pinned; use resume") {
		t.Fatalf("second start after birth: %v", err)
	}
}

// preflightPolicyBed keeps the policy's repository answers tied to files in
// this bed. The committed entries are captured before each test's physical
// change; the snapshot entries are read from the changed files at admission.
type preflightPolicyBed struct {
	t         *testing.T
	engine    *Engine
	source    *hostCycleSource
	committed map[string]gittree.Entry
	reads     *strictWallReads
	workspace *strictBaselineWorkspace
}

func newPreflightPolicyBed(t *testing.T) *preflightPolicyBed {
	t.Helper()
	e, source := newGitFreePreflightBed(t, "")
	b := &preflightPolicyBed{t: t, engine: e, source: source,
		reads: &strictWallReads{t: t}, workspace: &strictBaselineWorkspace{t: t}}
	b.committed = b.physicalEntries()
	e.wallReadFacts = b.reads
	e.wallWorkspaceFactory = func(root string) wallWorkspace {
		if root != e.Root {
			t.Fatalf("workspace root %q, want %q", root, e.Root)
		}
		return b.workspace
	}
	t.Cleanup(source.done)
	return b
}

func (b *preflightPolicyBed) physicalEntries() map[string]gittree.Entry {
	b.t.Helper()
	paths := make([]string, 0, len(b.source.files)+2)
	for path := range b.source.files {
		paths = append(paths, path)
	}
	paths = append(paths, filepath.ToSlash(filepath.Join("plans", "mission-"+b.engine.Mission+".contract.md")))
	truthDir := filepath.Join(b.engine.Root, "truth")
	truth, err := os.ReadDir(truthDir)
	if err != nil {
		b.t.Fatal(err)
	}
	for _, file := range truth {
		path := "truth/" + file.Name()
		if _, known := b.source.files[path]; !known {
			paths = append(paths, path)
		}
	}
	entries := make(map[string]gittree.Entry, len(paths))
	for _, path := range paths {
		abs := filepath.Join(b.engine.Root, filepath.FromSlash(path))
		info, err := os.Lstat(abs)
		if err != nil {
			b.t.Fatal(err)
		}
		var data []byte
		mode := "100644"
		if info.Mode()&os.ModeSymlink != 0 {
			mode = "120000"
			target, err := os.Readlink(abs)
			if err != nil {
				b.t.Fatal(err)
			}
			data = []byte(target)
		} else if info.Mode().IsRegular() {
			if info.Mode()&0o111 != 0 {
				mode = "100755"
			}
			data, err = os.ReadFile(abs)
			if err != nil {
				b.t.Fatal(err)
			}
		} else {
			b.t.Fatalf("unexpected snapshot object %s: %s", path, info.Mode())
		}
		entries[path] = gittree.Entry{Mode: mode, OID: birthBlobOID(data)}
	}
	return entries
}

// The identifier describes the observed path, mode, and blob facts. It is a
// fixture tree identity, so distinct worktree, HEAD, and staged projections
// cannot collapse into a single canned answer.
func preflightFactTree(entries map[string]gittree.Entry, exclude ...string) string {
	paths := make([]string, 0, len(entries))
	for path := range entries {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	h := sha1.New()
	for _, path := range paths {
		if path == excludeAt(exclude, 0) || path == excludeAt(exclude, 1) {
			continue
		}
		entry := entries[path]
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00", path, entry.Mode, entry.OID)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func excludeAt(paths []string, index int) string {
	if index < len(paths) {
		return paths[index]
	}
	return ""
}

func (b *preflightPolicyBed) pin(stdout string, code int) {
	b.reads.git = append(b.reads.git, wallGitReply{root: b.engine.Root,
		args:   []string{"config", "--local", "--type=bool", "--get", "core.fileMode"},
		stdout: stdout, code: code})
}

func (b *preflightPolicyBed) admission(approved []byte, throughIdentity bool) string {
	b.t.Helper()
	live := b.physicalEntries()
	contractPath := filepath.ToSlash(filepath.Join("plans", "mission-"+b.engine.Mission+".contract.md"))
	ledger := missionLedgerRel(b.engine.Mission)
	exclude := []string{ledger, contractPath}
	raw := preflightFactTree(live)
	head := preflightFactTree(b.committed)
	observed := preflightFactTree(live, exclude...)
	committed := preflightFactTree(b.committed, exclude...)
	b.workspace.queries = append(b.workspace.queries,
		baselineQuery{method: "Snapshot", tree: "HEAD", result: raw},
		baselineQuery{method: "FilterTree", tree: raw, paths: exclude, result: observed},
		baselineQuery{method: "HeadTree", result: head},
		baselineQuery{method: "FilterTree", tree: head, paths: exclude, result: committed},
		baselineQuery{method: "Entries", tree: raw, paths: []string{contractPath}, entries: map[string]gittree.Entry{contractPath: live[contractPath]}},
		baselineQuery{method: "Entries", tree: head, paths: []string{contractPath}, entries: map[string]gittree.Entry{contractPath: b.committed[contractPath]}},
	)
	if throughIdentity {
		b.reads.bytes = append(b.reads.bytes, wallByteReply{root: b.engine.Root,
			content: approved, oid: birthBlobOID(approved)})
		b.workspace.queries = append(b.workspace.queries,
			baselineQuery{method: "RefMap"},
			baselineQuery{method: "FilterTree", tree: raw, paths: []string{ledger}, result: preflightFactTree(live, ledger)},
			baselineQuery{method: "StagedTree", result: head},
			baselineQuery{method: "FilterTree", tree: head, paths: []string{ledger}, result: preflightFactTree(b.committed, ledger)},
			baselineQuery{method: "FilterTree", tree: head, paths: []string{ledger}, result: preflightFactTree(b.committed, ledger)},
		)
	}
	return observed
}

func (b *preflightPolicyBed) done() {
	b.t.Helper()
	b.reads.done()
	if len(b.workspace.queries) != 0 {
		b.t.Fatalf("unconsumed workspace queries: %+v", b.workspace.queries)
	}
}

// The wall's repository preconditions: the local pin guards mode visibility,
// and only the exact tree in the human's signed seal admits existing dirt.
func TestWallPreflightPreconditions(t *testing.T) {
	t.Run("filemode-off", func(t *testing.T) {
		b := newPreflightPolicyBed(t)
		b.pin("false\n", 0)
		err := b.engine.armAndPreflight("start")
		if err == nil || !strings.Contains(err.Error(), "core.fileMode") {
			t.Fatalf("a fileMode-off repository must refuse by name: %v", err)
		}
		b.done()
	})
	t.Run("filemode-absent-local-pin", func(t *testing.T) {
		b := newPreflightPolicyBed(t)
		b.pin("", 1)
		err := b.engine.armAndPreflight("start")
		if err == nil || !strings.Contains(err.Error(), "core.fileMode") {
			t.Fatalf("an unpinned repository must refuse by name: %v", err)
		}
		b.done()
	})
	t.Run("dirty-baseline", func(t *testing.T) {
		b := newPreflightPolicyBed(t)
		writeText(t, filepath.Join(b.engine.Root, "truth", "uncommitted.txt"), "dirt\n")
		b.pin("true\n", 0)
		b.pin("true\n", 0)
		observed := b.admission(b.source.signed, true)
		err := b.engine.armAndPreflight("start")
		if err == nil || !strings.Contains(err.Error(), "initial baseline is dirty") ||
			!strings.Contains(err.Error(), "wall.sealed-baseline="+observed) {
			t.Fatalf("unsealed dirt must refuse and name its observed tree %s: %v", observed, err)
		}
		b.done()
	})
	t.Run("sealed-baseline", func(t *testing.T) {
		b := newPreflightPolicyBed(t)
		writeText(t, filepath.Join(b.engine.Root, "truth", "uncommitted.txt"), "dirt the human saw\n")
		b.pin("true\n", 0)
		b.pin("true\n", 0)
		observed := b.admission(b.source.signed, true)
		err := b.engine.armAndPreflight("start")
		if err == nil {
			t.Fatal("the dirty start must refuse before sealing")
		}
		parts := strings.Split(err.Error(), "wall.sealed-baseline=")
		if len(parts) != 2 {
			t.Fatalf("the refusal must name the sealable tree: %v", err)
		}
		if named := strings.Fields(parts[1])[0]; named != observed {
			t.Fatalf("named tree %s differs from the observed file facts %s", named, observed)
		}
		b.done()
		contractPath := b.engine.contractPath()
		raw, rerr := os.ReadFile(contractPath)
		if rerr != nil {
			t.Fatal(rerr)
		}
		document := string(raw)
		document = document[:strings.Index(document, "\nApproval:")]
		document = document[:strings.Index(document, "```mission-seal")]
		document = strings.Replace(document, "```mission\n",
			"```mission\nwall.sealed-baseline="+observed+"\n", 1)
		if err := os.WriteFile(contractPath, []byte(document), 0o644); err != nil {
			t.Fatal(err)
		}
		sha, serr := contract.SealWithSource(contractPath, b.engine.contractSource)
		if serr != nil {
			t.Fatalf("re-seal: %v", serr)
		}
		handle, err := os.OpenFile(contractPath, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := handle.WriteString("\nApproval: name=Fixture Human; date=2026-08-19; contract-sha256=" + sha + "\n"); err != nil {
			t.Fatal(err)
		}
		if err := handle.Close(); err != nil {
			t.Fatal(err)
		}
		b.source.signed, err = os.ReadFile(contractPath)
		if err != nil {
			t.Fatal(err)
		}
		b.committed[filepath.ToSlash(filepath.Join("plans", "mission-"+b.engine.Mission+".contract.md"))] =
			b.physicalEntries()[filepath.ToSlash(filepath.Join("plans", "mission-"+b.engine.Mission+".contract.md"))]
		b.pin("true\n", 0)
		b.pin("true\n", 0)
		if next := b.admission(b.source.signed, true); next != observed {
			t.Fatalf("the observed truth tree changed during sealing: %s -> %s", observed, next)
		}
		b.reads.git = append(b.reads.git, wallGitReply{root: b.engine.Root,
			args: []string{"for-each-ref", "--format=%(refname)", mission.MissionRefNamespace(b.engine.Mission)}})
		if err := b.engine.armAndPreflight("start"); err != nil {
			t.Fatalf("the sealed baseline must admit exactly its tree: %v", err)
		}
		b.done()
	})
}

// These contract-identity checks exercise the actual launch and admission
// decisions with separate committed and live entries from physical files.
func TestWallPreflightContractIdentity(t *testing.T) {
	t.Run("mode-flip", func(t *testing.T) {
		b := newPreflightPolicyBed(t)
		if err := os.Chmod(b.engine.contractPath(), 0o755); err != nil {
			t.Fatal(err)
		}
		b.pin("true\n", 0)
		b.pin("true\n", 0)
		b.admission(b.source.signed, false)
		err := b.engine.armAndPreflight("start")
		if err == nil || !strings.Contains(err.Error(), "differs from its committed form") {
			t.Fatalf("an executable contract must refuse by name: %v", err)
		}
		b.done()
	})
	t.Run("symlink", func(t *testing.T) {
		b := newPreflightPolicyBed(t)
		sealedCopy := filepath.Join(b.engine.Root, "artifacts", "sealed-copy.contract.md")
		if err := os.MkdirAll(filepath.Dir(sealedCopy), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sealedCopy, b.source.signed, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(b.engine.contractPath()); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(sealedCopy, b.engine.contractPath()); err != nil {
			t.Fatal(err)
		}
		b.pin("true\n", 0)
		b.admission(b.source.signed, false)
		if _, err := b.engine.admittedBaseline(map[string]string{}, b.source.signed); err == nil ||
			!strings.Contains(err.Error(), "symlink") {
			t.Fatalf("the admission gate must refuse a symlinked contract by name: %v", err)
		}
		b.done()
		if err := b.engine.armAndPreflight("start"); err == nil ||
			!strings.Contains(err.Error(), "symlink") {
			t.Fatalf("the full launch ladder must refuse the symlink shape by name: %v", err)
		}
		b.done()
	})
	t.Run("fifo", func(t *testing.T) {
		b := newPreflightPolicyBed(t)
		if err := os.Remove(b.engine.contractPath()); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(b.engine.contractPath(), 0o644); err != nil {
			t.Skipf("cannot create a FIFO on this filesystem: %v", err)
		}
		if err := b.engine.armAndPreflight("start"); err == nil ||
			!strings.Contains(err.Error(), "non-regular object") {
			t.Fatalf("a FIFO contract must refuse by name, not hang: %v", err)
		}
		b.done()
	})
	t.Run("byte-edit", func(t *testing.T) {
		b := newPreflightPolicyBed(t)
		handle, err := os.OpenFile(b.engine.contractPath(), os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := handle.WriteString("\n<!-- unsigned edit in the preflight gap -->\n"); err != nil {
			t.Fatal(err)
		}
		if err := handle.Close(); err != nil {
			t.Fatal(err)
		}
		if b.physicalEntries()["plans/mission-alpha.contract.md"].OID == b.committed["plans/mission-alpha.contract.md"].OID {
			t.Fatal("the edit did not change the physical contract blob")
		}
		b.pin("true\n", 0)
		b.admission(b.source.signed, false)
		if _, err := b.engine.admittedBaseline(map[string]string{}, b.source.signed); err == nil ||
			!strings.Contains(err.Error(), "differs from its committed form") {
			t.Fatalf("the admission gate must refuse edited contract bytes by name: %v", err)
		}
		b.done()
		err = b.engine.armAndPreflight("start")
		if err == nil {
			t.Fatal("an uncommitted contract edit must refuse")
		}
		b.done()
	})
}

// This small native adapter witnesses Git's boolean normalization and the
// index/worktree mode distinction used by the strict ordinary transcript.
func TestNativePreflightConfigAndModeAdapter(t *testing.T) {
	root := t.TempDir()
	fixtureGit(t, root, "init", "-q", "-b", "main")
	engine := &Engine{Root: root}
	for _, spelling := range []string{"yes", "on", "1", "TRUE"} {
		fixtureGit(t, root, "config", "core.fileMode", spelling)
		if err := engine.checkFileModePinned(); err != nil {
			t.Fatalf("Git true spelling %q must satisfy the local pin: %v", spelling, err)
		}
	}
	fixtureGit(t, root, "config", "--unset", "core.fileMode")
	if err := engine.checkFileModePinned(); err == nil || !strings.Contains(err.Error(), "core.fileMode") {
		t.Fatalf("an absent local pin must refuse: %v", err)
	}
	fixtureGit(t, root, "config", "core.fileMode", "true")
	path := filepath.Join(root, "contract.md")
	if err := os.WriteFile(path, []byte("signed bytes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, root, "add", "contract.md")
	fixtureGit(t, root, "commit", "-qm", "committed contract mode")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	workspace := gittree.Workspace{Dir: root}
	raw, err := workspace.Snapshot("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	head, err := workspace.HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	live, err := workspace.Entries(raw, []string{"contract.md"})
	if err != nil {
		t.Fatal(err)
	}
	committed, err := workspace.Entries(head, []string{"contract.md"})
	if err != nil {
		t.Fatal(err)
	}
	if live["contract.md"].Mode != "100755" || committed["contract.md"].Mode != "100644" ||
		live["contract.md"].OID != committed["contract.md"].OID {
		t.Fatalf("mode-only change: snapshot=%+v committed=%+v", live, committed)
	}
}

// The mission must run EXACTLY the pinned contract. A different contract
// committed over local HEAD between the pin and state birth satisfies the
// committed-form check (live equals HEAD) and stays outside the dirt
// decision (the contract is excluded from it); only the approved-bytes
// binding keeps E0 from recording a contract nobody approved.
func TestAdmissionBindsTheApprovedContract(t *testing.T) {
	b := newBirthBed(t)
	e := b.engine
	tampered := strings.Replace(string(b.snapshot), "exposure=EUR:10", "exposure=EUR:9999", 1)
	if err := os.WriteFile(e.contractPath(), []byte(tampered), 0644); err != nil {
		t.Fatal(err)
	}
	b.expectStart()
	b.admission([]byte(tampered), false)
	b.drop()
	if _, _, _, err := e.initializeState(b.lease); err == nil || !strings.Contains(err.Error(), "approved contract bytes") {
		t.Fatalf("admission must refuse a workspace contract that is not the pinned one: %v", err)
	}
	if pathExists(b.atState()) {
		t.Fatal("refused admission published state")
	}
}

// The pin file is MUTABLE; the fence digest is not. A pin replaced in
// the pin-to-birth gap dies at the child's authenticated read — the one
// read state birth is allowed — because admission and state construction
// take that read's bytes as given and never touch the file again.
func TestBirthRefusesAReplacedPin(t *testing.T) {
	b := newBirthBed(t)
	e := b.engine
	tampered := strings.Replace(string(b.snapshot), "exposure=EUR:10", "exposure=EUR:9999", 1)
	if err := os.WriteFile(e.approvedContractPath(), []byte(tampered), 0644); err != nil {
		t.Fatal(err)
	}
	b.expectStart()
	if _, _, _, err := e.initializeState(b.lease); err == nil || !strings.Contains(err.Error(), "approvedContractSha256") {
		t.Fatalf("a replaced pin must refuse at the authenticated read: %v", err)
	}
	if pathExists(b.atState()) {
		t.Fatal("a replaced pin must not birth a mission")
	}
}

// Birth uses the bytes it AUTHENTICATED: the pin file is replaced in the
// gap after the single authenticated read, and the mission must still be
// born on the authenticated contract — admission passes against the
// workspace (which matches those bytes, not the replacement), and the
// state binds their stream, not the replacement's. A birth that read
// the pin file again — for admission or for state construction — would
// bind the replacement instead and fail here.
func TestBirthUsesTheBytesItAuthenticated(t *testing.T) {
	b := newBirthBed(t)
	e := b.engine
	tampered := strings.Replace(string(b.snapshot), "stream.primary=Reach the acceptance score.", "stream.primary=Injected after authentication.", 1)
	e.afterApprovedParse = func() {
		if err := os.WriteFile(e.approvedContractPath(), []byte(tampered), 0644); err != nil {
			t.Errorf("cannot replace the pin in the gap: %v", err)
		}
	}
	b.birth()
	b.verifyBirth()
	if _, _, _, err := e.initializeState(b.lease); err != nil {
		t.Fatalf("birth on the authenticated bytes must succeed: %v", err)
	}
	state := readTestDoc(t, b.atState())
	streams, _ := state["streams"].(map[string]any)
	primary, _ := streams["primary"].(map[string]any)
	if goal, _ := primary["goal"].(string); !strings.HasPrefix(goal, "Reach the acceptance score.") {
		t.Fatalf("state bound %q — the replaced pin leaked into birth", goal)
	}
	if reason, _ := state["parkReason"].(string); reason == "wall-violation" {
		t.Fatal("admission judged the replacement, not the authenticated bytes")
	}
	pin, err := os.ReadFile(e.approvedContractPath())
	if err != nil || !strings.Contains(string(pin), "Injected after authentication.") {
		t.Fatalf("the seam did not replace the pin: %v", err)
	}
}

// A non-regular object at the state path freezes the mission id with
// every artifact intact: no sweep, no clock reset, no reconciliation
// through the object. What a symlink points at may be a living
// mission's state; only a human removes it.
func TestNonRegularStatePathFreezesTheMission(t *testing.T) {
	b := newBirthBed(t)
	e := b.engine
	target := filepath.Join(t.TempDir(), "elsewhere-state.json")
	writeText(t, target, "{}\n")
	statePath := b.atState()
	if err := os.Symlink(target, statePath); err != nil {
		t.Fatal(err)
	}
	fencesBefore := readTestDoc(t, e.fencesPath())
	if _, _, _, err := e.initializeState(b.lease); err == nil || !strings.Contains(err.Error(), "non-regular object") {
		t.Fatalf("start must refuse the shape by name: %v", err)
	}
	if _, _, _, err := e.resumeState(); err == nil || !strings.Contains(err.Error(), "non-regular object") {
		t.Fatalf("resume must refuse the shape by name: %v", err)
	}
	if !pathExists(e.approvedContractPath()) {
		t.Fatal("the refusal must not sweep the pin")
	}
	fencesAfter := readTestDoc(t, e.fencesPath())
	if fencesBefore["startedAt"] != fencesAfter["startedAt"] {
		t.Fatal("the refusal must not reset the fence clock")
	}
	if info, err := os.Lstat(statePath); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the object must survive untouched: %v", err)
	}
	if !pathExists(target) {
		t.Fatal("the symlink target must survive untouched")
	}
	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "never-exists.json"), statePath); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := e.resumeState(); err == nil || !strings.Contains(err.Error(), "non-regular object") {
		t.Fatalf("resume must name a dangling symlink, not report absence: %v", err)
	}
	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	e.afterApprovedParse = func() {
		if err := os.Symlink(filepath.Join(t.TempDir(), "never-exists.json"), statePath); err != nil {
			t.Errorf("cannot plant the mid-birth symlink: %v", err)
		}
	}
	b.birth()
	b.drop()
	if _, _, _, err := e.initializeState(b.lease); err == nil || !strings.Contains(err.Error(), "non-regular object") {
		t.Fatalf("publication must refuse the mid-birth symlink by name: %v", err)
	}
	e.afterApprovedParse = nil
	if info, err := os.Lstat(statePath); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("publication must not replace the symlink: %v", err)
	}
}

// A mission that provably LIVED is never mistaken for a stillborn
// remnant: with the state file lost, every start-side entry refuses by
// naming the birth evidence and nothing is swept — the ledger, the pin,
// and the anchors all survive. The ledger's own booked cycles are the
// belt for missions whose birth record is also gone.
func TestLostStateFreezesTheBornMission(t *testing.T) {
	engine := buildGitFreeHostCycle(t, "FAKEHOST:close-stream")
	leasePath := filepath.Join(engine.Root, "artifacts", "agents", "checkout.lease.json")
	signal := filepath.Join(t.TempDir(), "start.json")
	if code := engine.internalRun("start", "metasystem-mission-runner-alpha-fixture-ls", signal); code != 0 {
		t.Fatalf("the bed mission must be born, exit %d", code)
	}
	missionRefs := func() map[string]string {
		t.Helper()
		refs, err := engine.wallWorkspace(engine.Root).RefMap()
		if err != nil {
			t.Fatalf("read the mission's anchors: %v", err)
		}
		anchors := make(map[string]string)
		for ref, oid := range refs {
			if strings.HasPrefix(ref, mission.MissionRefNamespace(engine.Mission)) {
				anchors[ref] = oid
			}
		}
		return anchors
	}
	bornAnchors := missionRefs()
	if len(bornAnchors) == 0 {
		t.Fatal("the born mission has no anchors")
	}
	statePath := filepath.Join(engine.missionDir(), "state.json")
	ledgerPath := filepath.Join(engine.missionDir(), "ledger.md")
	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := engine.initializeState(leasePath); err == nil ||
		!strings.Contains(err.Error(), "birth evidence") {
		t.Fatalf("initialization must refuse the lost-state mission by name: %v", err)
	}
	if err := engine.armAndPreflight("start"); err == nil ||
		!strings.Contains(err.Error(), "birth evidence") {
		t.Fatalf("the pin ladder must refuse the lost-state mission by name: %v", err)
	}
	// The PUBLIC launch refuses at its head, before the stale-lease
	// cleanup could rewrite anything.
	if err := engine.launch("start", false); err == nil ||
		!strings.Contains(err.Error(), "birth evidence") {
		t.Fatalf("the public launch must refuse before any mutation: %v", err)
	}
	// With the fences file gone too, the pin ladder must STILL refuse
	// instead of minting a fresh pin over the lived mission.
	if err := os.Remove(engine.fencesPath()); err != nil {
		t.Fatal(err)
	}
	if err := engine.armAndPreflight("start"); err == nil ||
		!strings.Contains(err.Error(), "birth evidence") {
		t.Fatalf("a missing fences file must not reopen the pin ladder: %v", err)
	}
	if !pathExists(ledgerPath) || !pathExists(engine.approvedContractPath()) {
		t.Fatal("the refusals must not sweep the lived mission's evidence")
	}
	if anchors := missionRefs(); !maps.Equal(anchors, bornAnchors) {
		t.Fatalf("the lived mission's anchors changed during refusal: born=%v now=%v", bornAnchors, anchors)
	}
	// The BELT: even with the birth record gone too, booked cycles in
	// the ledger prove the mission lived.
	if err := os.Remove(engine.birthRecordPath()); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := engine.initializeState(leasePath); err == nil ||
		!strings.Contains(err.Error(), "birth evidence") ||
		!strings.Contains(err.Error(), "archive the mission directory") {
		t.Fatalf("booked cycles must refuse with a performable remedy: %v", err)
	}
	if !pathExists(ledgerPath) {
		t.Fatal("the belt refusal must not remove the lived ledger")
	}
	// The LAST belt: with the record gone and the ledger reduced to a
	// header, the mission's surviving anchors alone must still freeze
	// the id — a lived mission's refs are never a rebirth's to drop.
	writeText(t, ledgerPath, "# Mission Ledger\n")
	if _, _, _, err := engine.initializeState(leasePath); err == nil ||
		!strings.Contains(err.Error(), "anchor namespace") {
		t.Fatalf("surviving anchors alone must refuse rebirth: %v", err)
	}
	if anchors := missionRefs(); !maps.Equal(anchors, bornAnchors) {
		t.Fatalf("the anchor refusal changed the refs: born=%v now=%v", bornAnchors, anchors)
	}
}

// The launch lock makes start decisions and births mutually exclusive:
// a launcher blocked behind the lock re-checks AFTER acquiring it, so a
// mission born in the gap is seen and refused — its pin and clock
// survive untouched. Without the lock, the blocked launcher's cached
// no-birth decision would overwrite the newborn's fences.
func TestLaunchLockSerializesStartDecisions(t *testing.T) {
	engine, source := newGitFreePreflightBed(t, "")
	t.Cleanup(source.done)
	if err := engine.armAndPreflight("start"); err != nil {
		t.Fatalf("initial preflight: %v", err)
	}
	fencesBefore := readTestDoc(t, engine.fencesPath())
	hold, err := engine.acquireLaunchLock()
	if err != nil {
		t.Fatal(err)
	}
	// The competitor announces its arrival at the lock; the birth lands
	// while it is held there, a fact waited for rather than an instant.
	blocked := make(chan struct{})
	var once sync.Once
	previous := launchLockAttempted
	launchLockAttempted = func() { once.Do(func() { close(blocked) }) }
	t.Cleanup(func() { launchLockAttempted = previous })
	done := make(chan error, 1)
	go func() { done <- engine.armAndPreflight("start") }()
	select {
	case <-blocked:
	case err := <-done:
		t.Fatalf("the competing launcher ended before reaching the lock: %v", err)
	}
	writeJSONFile(t, filepath.Join(engine.missionDir(), "state.json"),
		map[string]any{"fabricated": "born in the gap"})
	writeJSONFile(t, engine.birthRecordPath(), map[string]any{"missionId": engine.Mission})
	hold.release()
	if err := <-done; err == nil || !strings.Contains(err.Error(), "use resume") {
		t.Fatalf("the unblocked launcher must see the birth and refuse: %v", err)
	}
	fencesAfter := readTestDoc(t, engine.fencesPath())
	if fencesBefore["startedAt"] != fencesAfter["startedAt"] {
		t.Fatal("the newborn's clock must survive the refused launcher")
	}
}

// A human-sealed dirty baseline carries a mission all the way through
// birth and a real cycle: the child re-admits the sealed tree, records
// E0, and the first turn runs with the sealed dirt still in place.
func TestSealedBaselineBirthsAndRuns(t *testing.T) {
	engine := buildFullCycleRoot(t, "FAKEHOST:close-stream")
	writeText(t, filepath.Join(engine.Root, "truth", "uncommitted.txt"), "dirt the human saw\n")
	err := engine.armAndPreflight("start")
	if err == nil {
		t.Fatal("the dirty start must refuse before sealing")
	}
	parts := strings.Split(err.Error(), "wall.sealed-baseline=")
	if len(parts) != 2 {
		t.Fatalf("the refusal must name the sealable tree: %v", err)
	}
	observed := strings.Fields(parts[1])[0]
	contractPath := engine.contractPath()
	raw, rerr := os.ReadFile(contractPath)
	if rerr != nil {
		t.Fatal(rerr)
	}
	document := string(raw)
	document = document[:strings.Index(document, "\nApproval:")]
	document = document[:strings.Index(document, "```mission-seal")]
	document = strings.Replace(document, "```mission\n",
		"```mission\nwall.sealed-baseline="+observed+"\n", 1)
	os.WriteFile(contractPath, []byte(document), 0o644)
	sha, serr := contract.Seal(contractPath)
	if serr != nil {
		t.Fatalf("re-seal: %v", serr)
	}
	handle, _ := os.OpenFile(contractPath, os.O_APPEND|os.O_WRONLY, 0o644)
	handle.WriteString("\nApproval: name=Fixture Human; date=2026-08-19; contract-sha256=" + sha + "\n")
	handle.Close()
	fixtureGit(t, engine.Root, "add", "plans")
	fixtureGit(t, engine.Root, "commit", "-qm", "seal the dirty baseline")
	fixtureGit(t, engine.Root, "push", "-q", "origin", "main")
	// The RE-SEALED contract must be re-pinned: the child runs against
	// the pin, and the pin still holds the pre-seal bytes.
	if err := engine.armAndPreflight("start"); err != nil {
		t.Fatalf("the sealed baseline must re-pin: %v", err)
	}
	signal := filepath.Join(t.TempDir(), "start.json")
	if code := engine.internalRun("start", "metasystem-mission-runner-alpha-fixture-sd", signal); code != 0 {
		t.Fatalf("the sealed baseline must birth and run, exit %d", code)
	}
	state := readTestDoc(t, filepath.Join(engine.missionDir(), "state.json"))
	if baseline, _ := state["initialBaseline"].(string); baseline == "" {
		t.Fatal("the sealed birth must record its E0")
	}
	// EXACTLY the worked terminals: completion, or the close-stream
	// bed's all-streams park. A host-failure or stop-loss park would
	// mean no accepted host turn ever ran on the sealed baseline.
	status, _ := state["status"].(string)
	reason, _ := state["parkReason"].(string)
	if !(status == "completed" || (status == "parked" && reason == "all-streams-parked")) {
		t.Fatalf("the sealed mission must reach a worked terminal, not %q (%q)", status, reason)
	}
	ledger, lerr := os.ReadFile(filepath.Join(engine.missionDir(), "ledger.md"))
	if lerr != nil || !strings.Contains(string(ledger), "Cycle 1") {
		t.Fatalf("the sealed mission booked no cycle: %v", lerr)
	}
	if strings.Contains(string(ledger), "Return: rejected") {
		t.Fatalf("the sealed mission's turn must be accepted, not rejected:\n%s", ledger)
	}
	if !pathExists(filepath.Join(engine.Root, "truth", "uncommitted.txt")) {
		t.Fatal("the sealed dirt must survive the run")
	}
}

// The birth record self-heals at resume: a born mission whose record is
// missing gets it re-stamped from the verified living state.
func TestBirthRecordSelfHealsAtResume(t *testing.T) {
	b := newBirthBed(t)
	e := b.engine
	b.birth()
	b.verifyBirth()
	if _, _, _, err := e.initializeState(b.lease); err != nil {
		t.Fatalf("birth: %v", err)
	}
	if err := os.Remove(e.birthRecordPath()); err != nil {
		t.Fatal(err)
	}
	b.resume()
	if _, _, _, err := e.resumeState(); err != nil {
		t.Fatalf("resume over a missing birth record: %v", err)
	}
	if !pathExists(e.birthRecordPath()) {
		t.Fatal("resume must re-stamp the birth record")
	}
}

// Absence must be PROVEN before anything destructive runs: a ledger
// path that exists but cannot be read as a ledger refuses the start
// instead of counting as no evidence.
func TestUnprovableEmptinessRefusesRebirth(t *testing.T) {
	b := newBirthBed(t)
	e := b.engine
	ledgerPath := b.atLedger()
	if err := os.MkdirAll(ledgerPath, 0755); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := e.initializeState(b.lease); err == nil || !strings.Contains(err.Error(), "cannot prove") {
		t.Fatalf("an unreadable ledger probe must refuse, not authorize: %v", err)
	}
	if info, err := os.Lstat(ledgerPath); err != nil || !info.IsDir() {
		t.Fatalf("the occupied ledger path must survive the refusal: %v", err)
	}
}

func TestStillbornInitCleansItsArtifacts(t *testing.T) {
	b := newBirthBed(t)
	e := b.engine
	// The child rechecks fileMode before any repository snapshot.
	b.expectStart()
	b.fileMode("false")
	b.drop()
	if _, _, _, err := e.initializeState(b.lease); err == nil || !strings.Contains(err.Error(), "core.fileMode") {
		t.Fatalf("the child must recheck the fileMode pin: %v", err)
	}
	if pathExists(b.atLedger()) || pathExists(e.approvedContractPath()) {
		t.Fatal("fileMode refusal must clean ledger and pin")
	}
	b.pin()
	// Dirt lands after the corrected ordinary repin.
	writeText(t, filepath.Join(e.Root, "truth", "gap-dirt.txt"), "dirt\n")
	b.expectStart()
	b.admission(b.snapshot, true)
	b.drop()
	if _, _, _, err := e.initializeState(b.lease); err == nil || !strings.Contains(err.Error(), "initial baseline is dirty") {
		t.Fatalf("the child re-admission must refuse the gap dirt by name: %v", err)
	}
	if pathExists(b.atLedger()) {
		t.Fatal("the stillborn ledger must be removed")
	}
	if pathExists(e.approvedContractPath()) {
		t.Fatal("the stillborn pin must be removed")
	}
	if err := os.Remove(filepath.Join(e.Root, "truth", "gap-dirt.txt")); err != nil {
		t.Fatal(err)
	}
	b.pin()
	statePath := b.atState()
	e.afterApprovedParse = func() {
		if err := os.MkdirAll(statePath, 0755); err != nil {
			t.Errorf("cannot squat the state path mid-birth: %v", err)
		}
	}
	b.birth()
	b.drop()
	if _, _, _, err := e.initializeState(b.lease); err == nil || !strings.Contains(err.Error(), "state initialization refused") {
		t.Fatalf("the mid-birth squatter must fail the state write: %v", err)
	}
	e.afterApprovedParse = nil
	if pathExists(e.birthRecordPath()) {
		t.Fatal("a proven same-pass publication failure must unstamp the birth record")
	}
	if info, err := os.Lstat(statePath); err != nil || !info.IsDir() {
		t.Fatalf("the squatting directory must survive: %v", err)
	}
	if pathExists(b.atLedger()) {
		t.Fatal("the state-birth failure must sweep the stillborn ledger")
	}
	if pathExists(e.approvedContractPath()) {
		t.Fatal("the state-birth failure must sweep the stillborn pin")
	}
}

// A NESTED checkout — the bed lives in the metasystem/ subdirectory of a
// parent git repository, the supported deployment layout — admits a clean
// start and books a real cycle. The whole flight runs through the real
// wrapper and the bed's own binary, whose self-located project root is
// the nested bed, exactly as deployed.
func TestNestedCheckoutMissionBirth(t *testing.T) {
	// The nested double-spawn chains real engine starts whose inner
	// windows compound; under the package'''s compressed scale the chain
	// cannot finish inside its verify window. Real scale until the
	// chain'''s windows are audited individually — tracked under
	// timing-tests-synthetic-clock.
	// Pin lifted 2026-08-30 (timing slice 4): the verification window no
	// longer compresses below its base — it bounds real git-heavy birth
	// work with an early exit, so generosity is free and the nested
	// start fits it at any scale.
	engine := equipFullCycleBed(t, buildPreflightBed(t, "FAKEHOST:close-stream", true))
	statePath := filepath.Join(engine.missionDir(), "state.json")
	teardownBound, err := ScaledWaitAtLeast(10, 10*killThroughFloor)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		recordPath, _, _ := engine.runnerPaths()
		record, err := readJSONDoc(recordPath)
		if err != nil {
			return
		}
		pgid, pgidOK := jsonInt(record["pgid"])
		// Killing needs IDENTITY, never a number: a signal goes out
		// only when the recorded pid still runs under the recorded
		// instance tag — the same proof the stale-lease cleanup
		// demands — so a reused pid or group id can never be hit.
		if record["endedAt"] == nil {
			pid, pidOK := jsonInt(record["pid"])
			tag, tagOK := record["instanceTag"].(string)
			if pidOK && tagOK && tag != "" &&
				pidExists(int(pid)) && strings.Contains(processCommand(int(pid), fixtureauth.CommandProbe{}), tag) {
				if pgidOK && pgid > 1 {
					_ = syscall.Kill(-int(pgid), syscall.SIGKILL)
				}
			}
		}
		// The runner's group is not the whole story: adapters launch
		// DETACHED in their own sessions, so their git children write
		// .git objects from groups the runner record never names —
		// the third sighting of this cleanup race proved the single
		// group wait insufficient. The bed's own job records name
		// every group there is: sweep each identity-proven group,
		// then wait them all out under the scaled ceiling, failing
		// LOUDLY instead of returning into the TempDir race.
		groups := []int64{}
		if pgidOK && pgid > 1 {
			groups = append(groups, pgid)
		}
		// Two record families name detached groups: delegate JOB
		// records, and the mission's TURN records — the detached
		// HOST's identity lives only in turns/*/turn.json, and this
		// bed (FAKEHOST) creates hosts without any delegate job.
		recordPaths := []string{}
		if entries, dirErr := os.ReadDir(filepath.Join(engine.Root, "artifacts", "agents", "jobs")); dirErr == nil {
			for _, entry := range entries {
				if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
					recordPaths = append(recordPaths, filepath.Join(engine.Root, "artifacts", "agents", "jobs", entry.Name()))
				}
			}
		}
		if turnPaths, globErr := filepath.Glob(filepath.Join(engine.missionDir(), "turns", "*", "turn.json")); globErr == nil {
			recordPaths = append(recordPaths, turnPaths...)
		}
		for _, recPath := range recordPaths {
			doc, docErr := readJSONDoc(recPath)
			if docErr != nil {
				continue
			}
			recPgid, ok := jsonInt(doc["pgid"])
			if !ok || recPgid <= 1 {
				continue
			}
			// EVERY valid recorded group joins the bounded wait —
			// the host leader is reaped early while descendants
			// linger, so a dead-leader group is exactly the one
			// that must still be waited out. Waiting is
			// identity-free; only the SIGNAL needs the live
			// identity proof.
			groups = append(groups, recPgid)
			tag, _ := doc["instanceTag"].(string)
			recPid, pidOK := jsonInt(doc["pid"])
			if tag == "" || !pidOK || !pidExists(int(recPid)) ||
				!strings.Contains(processCommand(int(recPid), fixtureauth.CommandProbe{}), tag) {
				continue
			}
			_ = syscall.Kill(-int(recPgid), syscall.SIGKILL)
		}
		deadline := time.Now().Add(teardownBound)
		for _, g := range groups {
			for {
				if err := syscall.Kill(-int(g), 0); err != nil {
					break
				}
				if time.Now().After(deadline) {
					t.Errorf("mission-birth teardown: process group %d still alive after %s; TempDir removal would race it", g, teardownBound)
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
		}
		// FOURTH sighting of this race: a writer in an UNRECORDED
		// group — a detached descendant's git child — outlived every
		// record-named sweep above. The checkout directory is the one
		// truth every writer shares, and nothing else lawfully works
		// inside this test's private TempDir: sweep by cwd, then wait
		// the directory quiet before TempDir removal runs.
		checkout := filepath.Dir(engine.Root)
		sweepDeadline := time.Now().Add(teardownBound)
		for {
			live := 0
			if pids, pidErr := identity.AllPids(); pidErr == nil {
				for _, pid := range pids {
					cwd, ok := identity.ProcessCwd(pid)
					if !ok || (cwd != checkout && !strings.HasPrefix(cwd, checkout+string(os.PathSeparator))) {
						continue
					}
					live++
					_ = syscall.Kill(int(pid), syscall.SIGKILL)
				}
			}
			if live == 0 {
				break
			}
			if time.Now().After(sweepDeadline) {
				t.Errorf("mission-birth teardown: %d process(es) still working under %s after %s; TempDir removal would race them", live, checkout, teardownBound)
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	})
	cmd := exec.Command(filepath.Join(engine.Root, "bin", "metasystem"),
		"mission", "start", "--root", engine.Root, "--mission", engine.Mission, "--foreground")
	cmd.Dir = engine.Root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the nested start must give birth: %v\n%s", err, out)
	}
	state, err := readJSONDoc(statePath)
	if err != nil {
		t.Fatalf("the foreground nested mission returned without terminal state: %v", err)
	}
	status, _ := state["status"].(string)
	reason, _ := state["parkReason"].(string)
	if !(status == "completed" || (status == "parked" && reason == "all-streams-parked")) {
		t.Fatalf("the nested mission must reach a worked terminal, not %q (%q)", status, reason)
	}
	ledger, lerr := os.ReadFile(filepath.Join(engine.missionDir(), "ledger.md"))
	if lerr != nil || !strings.Contains(string(ledger), "Cycle 1") {
		t.Fatalf("the nested mission booked no cycle: %v", lerr)
	}

	// The AUTHORIZATION join holds in this layout too: an implementer
	// worktree is cut at the repository toplevel, but the validator
	// derives its boundary base in the mission project's own path space,
	// and that tree must BE a named expected-tree point of the mission —
	// otherwise no nested implementer chain could ever be authorized.
	worktree := filepath.Join(t.TempDir(), "delegate")
	fixtureGit(t, engine.Root, "worktree", "add", "--detach", worktree, "HEAD")
	defer fixtureGit(t, engine.Root, "worktree", "remove", "--force", worktree)
	delegateProject := gittree.Workspace{Dir: filepath.Join(worktree, "metasystem")}
	base, berr := delegateProject.TreeOf("HEAD")
	if berr != nil {
		t.Fatalf("the delegate project tree must resolve: %v", berr)
	}
	points := mission.ExpectedTreePoints(state)
	named := false
	for _, point := range points {
		if point.Tree == base {
			named = true
		}
	}
	if !named {
		t.Fatalf("the delegate's project-space base %s must be a named expected-tree point (points: %v)", base, points)
	}
	// The toplevel tree is a DIFFERENT identity — the exact confusion the
	// project scoping exists to prevent.
	rawTop := exec.Command("git", "-C", worktree, "rev-parse", "HEAD^{tree}")
	topOut, terr := rawTop.CombinedOutput()
	if terr != nil {
		t.Fatal(terr)
	}
	if top := strings.TrimSpace(string(topOut)); top == base {
		t.Fatalf("the bed is degenerate: toplevel and project trees coincide (%s)", top)
	}
}

// The RESUME child re-checks the fileMode pin: the parent's preflight
// does not survive the parent-child gap on either launch mode, and
// resume's reconciliation trusts the tree equation.
func TestResumeChildRechecksFileMode(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "alpha"}
	writeText(t, filepath.Join(engine.missionDir(), "state.json"), "{}")
	args := []string{"config", "--local", "--type=bool", "--get", "core.fileMode"}
	facts := &strictWallReads{t: t, git: []wallGitReply{
		{root: engine.Root, args: args, stdout: "true\n"},
		{root: engine.Root, args: args, stdout: "false\n"},
	}}
	engine.wallReadFacts = facts
	continuity := &strictResumeContinuity{t: t}
	engine.continuityFacts = continuity
	if err := engine.wallPreflight("resume", nil, nil); err != nil {
		t.Fatalf("parent preflight must accept the true local pin: %v", err)
	}
	if _, _, _, err := engine.resumeState(); err == nil ||
		!strings.Contains(err.Error(), "core.fileMode is not pinned true") {
		t.Fatalf("the resume child must recheck the fileMode pin: %v", err)
	}
	if len(continuity.calls) != 0 {
		t.Fatalf("the child reached continuity after the fileMode refusal: %+v", continuity.calls)
	}
	facts.done()
}

// The full cycle in-process: after the armed preflight, internalRun — the
// run-loop body Launch spawns — drives initializeState, ReserveCycle,
// prompt assembly, the real fake-host adapter (through the real engine
// binary), adjudication, measurement, the ledger, conclusion, and the
// anchor. Launch's own subprocess spawn is untestable in-process
// (os.Executable is the test binary), so the loop body is driven directly,
// which is exactly what the spawned process runs.
// buildFullCycleRoot extends the preflight root with the pieces a running
// turn needs: the real engine binary, the real fake-host adapter, a
// permissive prompt checker, the armed pin, and the anchor seam. The
// behavior directive, when given, rides the contract's stream text into
// the prompt, which is how the fake host selects its behavior.
var (
	freshBinaryOnce sync.Once
	freshBinaryPath string
	freshBinaryErr  error
)

// freshEngineBinary compiles cmd/metasystem from the CURRENT source into
// a shared temporary location, once per test process.
func freshEngineBinary(t *testing.T) string {
	t.Helper()
	freshBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "metasystem-test-binary.")
		if err != nil {
			freshBinaryErr = err
			return
		}
		freshBinaryPath = filepath.Join(dir, "metasystem")
		build := exec.Command("go", "build", "-o", freshBinaryPath, "./cmd/metasystem")
		build.Dir = filepath.Join("..", "..")
		if out, err := build.CombinedOutput(); err != nil {
			freshBinaryErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if freshBinaryErr != nil {
		// A proof that silently vanishes is no proof:
		// the wrapper certification REQUIRES the reviewed binary.
		t.Fatalf("cannot build the engine binary from source: %v", freshBinaryErr)
	}
	return freshBinaryPath
}

func buildFullCycleRoot(t *testing.T, behavior string) *Engine {
	t.Helper()
	return pinAndSeamBed(t, equipFullCycleBed(t, buildPreflightRootWithStream(t, behavior)))
}

// equipFullCycleBed adds the running-turn pieces to a preflight bed: the
// reviewed binary, the real adapters and return checker, the prompt
// authority artifacts, and the committed baseline. pinAndSeamBed follows
// for in-process runs; a wrapper-driven run pins through its own launch
// ladder instead.
func equipFullCycleBed(t *testing.T, engine *Engine) *Engine {
	t.Helper()
	equipFullCycleFiles(t, engine)
	commitBedBaseline(t, engine.Root)
	return engine
}

func equipFullCycleFiles(t *testing.T, engine *Engine) {
	t.Helper()
	root := engine.Root
	// The bed binary is COMPILED FROM THE REVIEWED TREE, once per test
	// process: a prebuilt
	// bin/metasystem can be stale, and a wrapper fixture passing against
	// yesterday's implementation proves nothing about this one.
	binary, err := os.ReadFile(freshEngineBinary(t))
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(root, "bin"), 0o755)
	if err := testexec.WriteFile(filepath.Join(root, "bin", "metasystem"), binary, 0o755); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(root, "scripts", "agents", "hosts"), 0o755)
	// The host and its shared library travel together:
	// fake.sh sources host-common.sh from its own directory.
	for _, name := range []string{"fake.sh", "host-common.sh"} {
		adapter, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "hosts", name))
		if err != nil {
			t.Fatal(err)
		}
		testexec.WriteFile(filepath.Join(root, "scripts", "agents", "hosts", name), adapter, 0o755)
	}
	// The human entrypoint IS the engine verb now (the wrapper died in
	// the L15 delete tranche); the resolution fixtures drive the same
	// binary a human types. The prompt checker runs for real too — the
	// authority artifacts below make its pass honest instead of stubbed.
	// The prompt authority artifacts, verbatim from the repository: without
	// them AssemblePrompt refuses and the cycle parks before any host runs.
	// The return checker and its role schema travel the same way:
	// without them EVERY orchestrator return is rejected and each
	// mission ends in a host-failure park that loose terminal
	// assertions read as success.
	for _, artifact := range []string{
		filepath.Join("scripts", "agents", "roles", "orchestrator.md"),
		filepath.Join("scripts", "agents", "templates", "host-turn-instruction.md"),
		filepath.Join("scripts", "assert-return-complete.sh"),
		filepath.Join("scripts", "agents", "schemas", "orchestrator.schema.json"),
	} {
		data, err := os.ReadFile(filepath.Join("..", "..", artifact))
		if err != nil {
			t.Skipf("prompt authority artifact not readable: %v", err)
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(artifact, ".sh") {
			mode = 0o755
		}
		os.MkdirAll(filepath.Dir(filepath.Join(root, artifact)), 0o755)
		testexec.WriteFile(filepath.Join(root, artifact), data, mode)
	}
}

// equipFullCycleBed then pins the contract in-process and installs the
// anchor seam the in-process run loop needs.
func pinAndSeamBed(t *testing.T, engine *Engine) *Engine {
	t.Helper()
	if err := engine.armAndPreflight("start"); err != nil {
		t.Fatalf("preflight: %v", err)
	}
	engine.anchorFn = func(statePath, ledgerPath, identityName string) error {
		return mission.Anchor(statePath, engine.Root, ledgerPath)
	}
	return engine
}

func TestInternalRunFullCycle(t *testing.T) {
	engine := buildGitFreeHostCycle(t, "")
	root := engine.Root
	_ = root

	// A standing outage mark rides into the happy path: any provider
	// success must clear it (provider-outage-posture), witnessed here
	// rather than in a second full mission run.
	if _, err := outage.Record(engine.Root, "overloaded", "API Error: 529", "mission-runner", time.Now()); err != nil {
		t.Fatal(err)
	}

	signal := filepath.Join(t.TempDir(), "start.json")
	code := engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	if code != 0 {
		// Surface the run's own trail before any assertion: the runner log,
		// the signal, and the newest turn's record.
		if data, err := os.ReadFile(signal); err == nil {
			t.Logf("start signal: %s", data)
		}
		_, _, logPath := engine.runnerPaths()
		if data, err := os.ReadFile(logPath); err == nil {
			t.Logf("runner log tail: %s", tailOf(string(data), 800))
		}
		turns, _ := filepath.Glob(filepath.Join(engine.missionDir(), "turns", "*", "turn.json"))
		for _, turn := range turns {
			if data, err := os.ReadFile(turn); err == nil {
				t.Logf("%s: %s", turn, data)
			}
		}
	}

	// The mission ran real cycles: turn one exists and is terminal, the
	// state file advanced, and the ledger booked at least one cycle. The
	// exact terminal (completed, parked, budget) is the mission's business —
	// the cycle plumbing under test either carried it there or errored.
	statePath := filepath.Join(engine.missionDir(), "state.json")
	state, err := readJSONDoc(statePath)
	if err != nil {
		t.Fatalf("no mission state after the run (rc=%d): %v", code, err)
	}
	status, _ := state["status"].(string)
	if status == "" || status == "running" {
		t.Fatalf("the mission did not reach a terminal: status=%q rc=%d", status, code)
	}
	turns, _ := filepath.Glob(filepath.Join(engine.missionDir(), "turns", "*", "turn.json"))
	if len(turns) == 0 {
		t.Fatalf("no turns ran (rc=%d, status=%s)", code, status)
	}
	firstTurn, err := readJSONDoc(turns[0])
	if err != nil {
		t.Fatal(err)
	}
	turnStatus, _ := firstTurn["status"].(string)
	if turnStatus == "pending" || turnStatus == "running" {
		t.Fatalf("turn one never settled: %q", turnStatus)
	}
	// The host really ran: the launch stamped its identity onto the turn.
	if tag, _ := firstTurn["instanceTag"].(string); !strings.HasPrefix(tag, "metasystem-host-") {
		t.Fatalf("turn one never reached a host: instanceTag=%v error=%v detail=%v",
			firstTurn["instanceTag"], firstTurn["error"], firstTurn["detail"])
	}
	ledger := filepath.Join(engine.missionDir(), "ledger.md")
	if data, err := os.ReadFile(ledger); err != nil || !strings.Contains(string(data), "Cycle 1") {
		t.Fatalf("the ledger booked nothing: %v", err)
	}
	measurement, err := readJSONDoc(filepath.Join(filepath.Dir(turns[0]), "measurement.json"))
	if err != nil {
		t.Fatalf("turn one measurement is absent: %v", err)
	}
	reading, _ := measurement["measurement"].(map[string]any)
	metrics, _ := reading["metrics"].(map[string]any)
	guards, _ := reading["guards"].(map[string]any)
	if measurement["gatePassed"] != true || metrics["score"] != "1" || guards["audit"] != "1" {
		t.Fatalf("turn one did not record a passing gate and audit guard: %v", measurement)
	}
	ledgerBytes, err := os.ReadFile(ledger)
	if err != nil || !strings.Contains(string(ledgerBytes), "- Classification: unresolved;") {
		t.Fatalf("turn one did not classify the real baseline measurement: %v %s", err, ledgerBytes)
	}
	if _, ok := outage.Read(engine.Root); ok {
		t.Fatal("a completed host turn must clear the seeded outage mark")
	}
}

func tailOf(text string, n int) string {
	if len(text) <= n {
		return text
	}
	return text[len(text)-n:]
}

// The fault alternations, on the same fixture: a host that exits non-zero
// drives the failed-turn conclusion; the mission still books the cycle and
// reaches a terminal rather than hanging or losing the record.
func TestInternalRunHostFailureCycle(t *testing.T) {
	engine := buildGitFreeHostCycle(t, "FAKEHOST:exit-nonzero")
	signal := filepath.Join(t.TempDir(), "start.json")
	code := engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	state, err := readJSONDoc(filepath.Join(engine.missionDir(), "state.json"))
	if err != nil {
		t.Fatalf("no state (rc=%d): %v", code, err)
	}
	status, _ := state["status"].(string)
	if status == "" || status == "running" {
		t.Fatalf("no terminal after host failures: %q rc=%d", status, code)
	}
	turns, _ := filepath.Glob(filepath.Join(engine.missionDir(), "turns", "*", "turn.json"))
	if len(turns) == 0 {
		t.Fatal("no turns ran")
	}
	first, _ := readJSONDoc(turns[0])
	if outcome, _ := first["outcome"].(string); outcome != "failed" {
		t.Fatalf("turn one's outcome: %q", outcome)
	}
}

// A malformed return drives the adjudication rejection: the cycle keeps its
// duties (drain, measure, conclude with both facts) and the turn records
// the protocol error.
func TestInternalRunMalformedReturnCycle(t *testing.T) {
	engine := buildGitFreeHostCycle(t, "FAKEHOST:return-malformed")
	signal := filepath.Join(t.TempDir(), "start.json")
	code := engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	state, err := readJSONDoc(filepath.Join(engine.missionDir(), "state.json"))
	if err != nil {
		t.Fatalf("no state (rc=%d): %v", code, err)
	}
	if status, _ := state["status"].(string); status == "" || status == "running" {
		t.Fatalf("no terminal: %q", status)
	}
	turns, _ := filepath.Glob(filepath.Join(engine.missionDir(), "turns", "*", "turn.json"))
	if len(turns) == 0 {
		t.Fatal("no turns")
	}
	first, _ := readJSONDoc(turns[0])
	if errField, _ := first["error"].(string); errField != "protocol-error" {
		t.Fatalf("turn one's error: %q detail=%v", errField, first["detail"])
	}
}

// A host that never writes a return concludes as a host failure.
func TestInternalRunNoReturnCycle(t *testing.T) {
	engine := buildGitFreeHostCycle(t, "FAKEHOST:no-return")
	signal := filepath.Join(t.TempDir(), "start.json")
	code := engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	state, err := readJSONDoc(filepath.Join(engine.missionDir(), "state.json"))
	if err != nil {
		t.Fatalf("no state (rc=%d): %v", code, err)
	}
	if status, _ := state["status"].(string); status == "" || status == "running" {
		t.Fatalf("no terminal: %q", status)
	}
}

// A park-request return drives the ask pipeline: the mission parks with a
// reserved decision and the proposed ask lands on disk.
func TestInternalRunParkRequestCycle(t *testing.T) {
	engine := buildGitFreeHostCycle(t, "FAKEHOST:park-request")
	signal := filepath.Join(t.TempDir(), "start.json")
	code := engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	state, err := readJSONDoc(filepath.Join(engine.missionDir(), "state.json"))
	if err != nil {
		t.Fatalf("no state (rc=%d): %v", code, err)
	}
	if status, _ := state["status"].(string); status != "parked" {
		t.Fatalf("a park request did not park: %q rc=%d", status, code)
	}
	// The TRUE path, not the false one: a REJECTED return also parks
	// and also mints an ask — a rejected-* host-failure ask after the
	// breaker. Demand the accepted shape end to end: the all-streams
	// park, the reserved-decision reason, and the accepted-candidate
	// ask id.
	if reason, _ := state["parkReason"].(string); reason != "all-streams-parked" {
		t.Fatalf("the park must come from the accepted stream update: %q", reason)
	}
	asks, _ := filepath.Glob(filepath.Join(asksDirPath(engine.Root, engine.Mission), "*.json"))
	if len(asks) == 0 {
		t.Fatal("no ask landed for the park")
	}
	askDoc, err := readJSONDoc(asks[0])
	if err != nil {
		t.Fatal(err)
	}
	if reasonClass, _ := askDoc["reasonClass"].(string); reasonClass != "reserved-decision" {
		t.Fatalf("the ask must be the requested reserved decision: %v", askDoc)
	}
	if askID, _ := askDoc["askId"].(string); !strings.HasPrefix(askID, "ask-") {
		t.Fatalf("the ask must be the accepted candidate, not a rejection's: %v", askDoc["askId"])
	}
}

// A close-stream return concludes the mission's only stream: the mission
// reaches a terminal and the ledger books the cycle that did it.
func TestInternalRunCloseStreamCycle(t *testing.T) {
	engine := buildGitFreeHostCycle(t, "FAKEHOST:close-stream")
	signal := filepath.Join(t.TempDir(), "start.json")
	code := engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	state, err := readJSONDoc(filepath.Join(engine.missionDir(), "state.json"))
	if err != nil {
		t.Fatalf("no state (rc=%d): %v", code, err)
	}
	if status, _ := state["status"].(string); status == "" || status == "running" {
		t.Fatalf("no terminal: %q rc=%d", status, code)
	}
	ledger, err := os.ReadFile(filepath.Join(engine.missionDir(), "ledger.md"))
	if err != nil || !strings.Contains(string(ledger), "Cycle 1") {
		t.Fatalf("the ledger booked nothing: %v", err)
	}
	// Status renders the terminal without disturbing it, and the public
	// resume refuses a non-running mission by naming its state.
	if code := engine.Status(); code == 7 {
		t.Fatal("Status could not read the terminal state")
	}
	if err := engine.launch("resume", false); err == nil {
		t.Fatal("a terminal mission resumed")
	}

	// The wall's open-turn lifecycle across the real cycle: the
	// concluded mission holds no open turn, the turn record carries the
	// pre-tree it opened under, and that tree is anchored against garbage
	// collection under the mission's ref namespace.
	if state["openTurn"] != nil {
		t.Fatalf("a concluded mission left a turn open: %v", state["openTurn"])
	}
	turns, _ := filepath.Glob(filepath.Join(engine.missionDir(), "turns", "*", "turn.json"))
	if len(turns) == 0 {
		t.Fatal("no turn record on disk")
	}
	turnDoc, err := readJSONDoc(turns[0])
	if err != nil {
		t.Fatal(err)
	}
	preTree, _ := turnDoc["preTree"].(string)
	if !regexp.MustCompile(`^[0-9a-f]{40,64}$`).MatchString(preTree) {
		t.Fatalf("turn record preTree is not a tree id: %q", preTree)
	}
	anchors, err := engine.wallWorkspace(engine.Root).RefMap()
	if err != nil {
		t.Fatalf("read the pre-tree anchor ref: %v", err)
	}
	ref := mission.MissionRefNamespace(engine.Mission) + preTree
	if anchors[ref] != preTree {
		t.Fatalf("anchor %q points at %q, not the pre-tree %q", ref, anchors[ref], preTree)
	}
}

// The resume path through a real cycle: a completed mission refuses resume
// by naming its state, and a parked mission's resume is refused toward the
// park reason — both through the public launch spine.
func TestInternalRunResumeVerdicts(t *testing.T) {
	engine := buildGitFreeHostCycle(t, "FAKEHOST:park-request")
	signal := filepath.Join(t.TempDir(), "start.json")
	engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	state, err := readJSONDoc(filepath.Join(engine.missionDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	// The park is DEMANDED, not skipped over: a skip would hide a
	// mission that stopped parking.
	if status, _ := state["status"].(string); status != "parked" {
		t.Fatalf("the park-request mission must park: %q", status)
	}
	// The public resume refuses a parked mission toward its park reason.
	if err := engine.launch("resume", false); err == nil ||
		!strings.Contains(err.Error(), "answer its park reason before resume") {
		t.Fatalf("parked resume: %v", err)
	}
}

// dispatch-terminal: the orchestrator return that certifies a terminal job
// drives the certified-entry adjudication path end to end.
func TestInternalRunDispatchTerminalCycle(t *testing.T) {
	engine := buildGitFreeHostCycle(t, "FAKEHOST:dispatch-terminal")
	signal := filepath.Join(t.TempDir(), "start.json")
	code := engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	state, err := readJSONDoc(filepath.Join(engine.missionDir(), "state.json"))
	if err != nil {
		t.Fatalf("no state (rc=%d): %v", code, err)
	}
	if status, _ := state["status"].(string); status == "" || status == "running" {
		t.Fatalf("no terminal: %q rc=%d", status, code)
	}
}

// The answer-and-resume chain on a parked mission: Status reads the park,
// Answer applies the human's decision to the ask, and the resumed run
// drives another real cycle — the full human-in-the-loop round trip.
func TestInternalRunAnswerAndResumeChain(t *testing.T) {
	engine := buildGitFreeHostCycle(t, "FAKEHOST:park-request")
	signal := filepath.Join(t.TempDir(), "start.json")
	engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	state, err := readJSONDoc(filepath.Join(engine.missionDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	// The park and its shape are DEMANDED, not skipped over: a
	// rejected-return mission also parks, through the host-failure
	// breaker with a rejected-* ask, and a skip here would hide that
	// false path's return.
	if status, _ := state["status"].(string); status != "parked" {
		t.Fatalf("the park-request mission must park: %q", status)
	}
	if reason, _ := state["parkReason"].(string); reason != "all-streams-parked" {
		t.Fatalf("the park must come from the accepted stream update: %q", reason)
	}

	// Status renders the park without disturbing it.
	if code := engine.Status(); code == 7 {
		t.Fatal("Status could not read a lawful parked state")
	}

	// Answer the proposed ask; an unknown ask id is refused first.
	if code := engine.Answer("not-an-ask", "approve: nothing"); code == 0 {
		t.Fatal("an unknown ask id was answered")
	}
	asks, _ := filepath.Glob(filepath.Join(asksDirPath(engine.Root, engine.Mission), "*.json"))
	if len(asks) == 0 {
		t.Fatal("no ask on disk")
	}
	askDoc, err := readJSONDoc(asks[0])
	if err != nil {
		t.Fatal(err)
	}
	if reasonClass, _ := askDoc["reasonClass"].(string); reasonClass != "reserved-decision" {
		t.Fatalf("the ask must be the requested reserved decision: %v", askDoc)
	}
	askID, _ := askDoc["askId"].(string)
	if !strings.HasPrefix(askID, "ask-") {
		t.Fatalf("the ask must be the accepted candidate, not a rejection's: %q", askID)
	}
	if code := engine.Answer(askID, "approve: proceed as asked"); code != 0 {
		t.Fatalf("answering the ask failed with %d", code)
	}

	// The resumed run drives at least one more real cycle before the next
	// park (the contract's directive parks every turn — that repetition is
	// the point: resumeState and the answered-ask heal both run).
	code := engine.internalRun("resume", "metasystem-mission-runner-alpha-fixture-2", signal)
	turns, _ := filepath.Glob(filepath.Join(engine.missionDir(), "turns", "*", "turn.json"))
	if len(turns) < 2 {
		t.Fatalf("the resume ran no further turn (rc=%d, turns=%d)", code, len(turns))
	}
}

// The small verdict helpers' remaining branches, driven directly.
func TestJSONIntSpellings(t *testing.T) {
	if v, ok := jsonInt(json.Number("7")); !ok || v != 7 {
		t.Fatalf("json.Number: %d %v", v, ok)
	}
	if v, ok := jsonInt(float64(7)); !ok || v != 7 {
		t.Fatalf("whole float: %d %v", v, ok)
	}
	if _, ok := jsonInt(0.5); ok {
		t.Fatal("fractional float accepted")
	}
	if _, ok := jsonInt(json.Number("1.5")); ok {
		t.Fatal("fractional number accepted")
	}
	if _, ok := jsonInt("7"); ok {
		t.Fatal("string accepted")
	}
	if v, ok := jsonInt(int(3)); !ok || v != 3 {
		t.Fatalf("int: %d %v", v, ok)
	}
	if v, ok := jsonInt(int64(4)); !ok || v != 4 {
		t.Fatalf("int64: %d %v", v, ok)
	}
}

// Answer's refusal grammar for the reserved-decision park: an empty answer
// and an answer to a mission with no asks directory.
func TestAnswerRefusalGrammar(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-answer"}
	if code := engine.Answer("ask-1", "approve: x"); code == 0 {
		t.Fatal("an askless mission answered")
	}
	os.MkdirAll(asksDirPath(engine.Root, engine.Mission), 0o755)
	if code := engine.Answer("", "approve: x"); code == 0 {
		t.Fatal("an empty ask id answered")
	}
}

func TestMissionJobStatusesReads(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	os.MkdirAll(jobs, 0o755)
	os.WriteFile(filepath.Join(jobs, "m-j1.json"), []byte(`{"jobId":"m-j1","mission":"m","status":"running"}`), 0o644)
	os.WriteFile(filepath.Join(jobs, "m-j2.json"), []byte(`{"jobId":"m-j2","mission":"m","status":"completed"}`), 0o644)
	os.WriteFile(filepath.Join(jobs, "other.json"), []byte(`{"jobId":"other","mission":"n","status":"running"}`), 0o644)
	statuses := missionJobStatuses(root, "m")
	if statuses["m-j1"] != "running" || statuses["m-j2"] != "completed" {
		t.Fatalf("statuses: %v", statuses)
	}
	if _, present := statuses["other"]; present {
		t.Fatal("a foreign mission's job leaked in")
	}
}

func TestAllocateTurnSequence(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-alloc"}
	os.MkdirAll(engine.missionDir(), 0o755)
	firstID, firstDir, err := engine.allocateTurn(1)
	if err != nil {
		t.Fatal(err)
	}
	if firstID == "" || !pathExists(firstDir) {
		t.Fatalf("first allocation: %q %q", firstID, firstDir)
	}
	secondID, secondDir, err := engine.allocateTurn(2)
	if err != nil {
		t.Fatal(err)
	}
	if secondID == firstID || secondDir == firstDir {
		t.Fatal("turn allocations collided")
	}
}

func TestStatusRendersEveryClass(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-status"}
	// Missing state.
	if code := engine.Status(); code != 7 {
		t.Fatalf("missing state must be unreadable-class: %d", code)
	}
	// Malformed state.
	os.MkdirAll(engine.missionDir(), 0o755)
	statePath := filepath.Join(engine.missionDir(), "state.json")
	os.WriteFile(statePath, []byte("{broken"), 0o644)
	if code := engine.Status(); code != 7 {
		t.Fatalf("malformed state must be unreadable-class: %d", code)
	}
}

// The solo-build shape under the recovery ladder (D117, slice A): the
// host authors a product file in the checkout and returns a clean
// empty-dispatch envelope, EVERY turn. The first offense is the
// dominant mechanical case — the rung restores the file, the whole
// posture re-verifies, the turn concludes with the recovery record in
// its acceptance entry, and the mission keeps moving. The second
// offense is a repeat: the rung refuses, the wall parks the mission
// with taint exactly as it always did, and no further run mode opens a
// turn until a human resolution clears it.
func TestInternalRunSoloBuildRecoversThenRepeatParks(t *testing.T) {
	engine := buildFullCycleRoot(t, "FAKEHOST:solo-build")
	signal := filepath.Join(t.TempDir(), "start.json")
	engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	state, err := readJSONDoc(filepath.Join(engine.missionDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if state["status"] != "parked" || state["parkReason"] != "wall-violation" {
		t.Fatalf("the repeat offense must park on the wall: status=%v reason=%v", state["status"], state["parkReason"])
	}
	// Exactly one taint — the repeat's. The first offense left no taint:
	// it left a recovery record on its acceptance entry instead.
	taint, _ := state["workspaceTaint"].(map[string]any)
	entries, _ := taint["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("only the repeat may taint the workspace: %v", taint)
	}
	entry := entries[0].(map[string]any)
	if entry["resolution"] != nil || !strings.Contains(entry["reason"].(string), "solo.go") {
		t.Fatalf("taint entry: %v", entry)
	}
	if state["openTurn"] == nil {
		t.Fatal("the violated turn's marker must survive for the resolution")
	}
	// The violated repeat turn never concludes into the log: the chain
	// holds exactly ONE acceptance — the recovered first turn's, carrying
	// the recovery record — and nothing for the turn the taint names.
	repeatTurn, _ := entry["turnId"].(string)
	acceptances := 0
	recovered := 0
	for _, raw := range turnLogOf(state) {
		logged, _ := raw.(map[string]any)
		wall, _ := logged["wall"].(map[string]any)
		if wall == nil {
			continue
		}
		if kind, _ := logged["kind"].(string); kind == mission.WallVerificationKind {
			continue
		}
		acceptances++
		if id, _ := logged["turnId"].(string); id == repeatTurn {
			t.Fatalf("the violated turn must not conclude into the log: %v", logged)
		}
		record, _ := wall["recovered"].(map[string]any)
		if record == nil {
			continue
		}
		recovered++
		if v, _ := record["violation"].(string); !strings.Contains(v, "solo.go") {
			t.Fatalf("the recovery record must name the offense: %v", record)
		}
		paths, _ := record["restoredPaths"].([]any)
		if len(paths) != 1 || paths[0] != "solo.go" {
			t.Fatalf("the recovery record must name the restored path: %v", record)
		}
	}
	if acceptances != 1 || recovered != 1 {
		t.Fatalf("exactly one acceptance, and it carries the record: acceptances=%d recovered=%d", acceptances, recovered)
	}
	// The repeat consumed nothing: no authorization is indexed to it.
	consumed, err := mission.ConsumedAuthorizations(state)
	if err != nil {
		t.Fatal(err)
	}
	for digest, turn := range consumed {
		if turn == repeatTurn {
			t.Fatalf("the violated turn must consume nothing: %s", digest)
		}
	}
	// Two turns of wall evidence: the recovered pass, then the repeat's
	// violation.
	turns, _ := filepath.Glob(filepath.Join(engine.missionDir(), "turns", "*", "wall.json"))
	if len(turns) != 2 {
		t.Fatalf("wall evidence: %v", turns)
	}
	verdicts := map[string]int{}
	var violatedDoc map[string]any
	for _, path := range turns {
		evidence, err := readJSONDoc(path)
		if err != nil {
			t.Fatal(err)
		}
		verdict, _ := evidence["verdict"].(string)
		verdicts[verdict]++
		if verdict == "violated" {
			violatedDoc = evidence
		}
	}
	if verdicts["passed"] != 1 || verdicts["violated"] != 1 {
		t.Fatalf("one recovered pass and one repeat violation: %v", verdicts)
	}
	if !strings.Contains(violatedDoc["violation"].(string), "solo.go") {
		t.Fatalf("the repeat's evidence must name the path: %v", violatedDoc)
	}
	// Every tree the evidence names is anchored: the
	// violation's post tree must survive garbage collection for the
	// resolution to verify against.
	postTree, _ := violatedDoc["postTree"].(string)
	anchorOut, anchorErr := exec.Command("git", "-C", engine.Root, "rev-parse", "--verify",
		"refs/metasystem/missions/"+engine.Mission+"/"+postTree).Output()
	if anchorErr != nil || strings.TrimSpace(string(anchorOut)) != postTree {
		t.Fatalf("violated post tree is not anchored: %v %q", anchorErr, anchorOut)
	}
	// The repeat's offense bytes stay on disk for the human to adjudicate.
	if _, err := os.Lstat(filepath.Join(engine.Root, "solo.go")); err != nil {
		t.Fatalf("the repeat's disputed bytes must stay for the human: %v", err)
	}

	// The repeat's ask arrives with the ladder's context: the human
	// reads that the rung already ran once and refused the second pass.
	asks, _ := filepath.Glob(filepath.Join(asksDirPath(engine.Root, engine.Mission), "wall-violation*.json"))
	if len(asks) != 1 {
		t.Fatalf("the repeat park must raise one ask: %v", asks)
	}
	ask := readTestDoc(t, asks[0])
	if note, _ := ask["recoveryNote"].(string); !strings.Contains(note, "repeat offense") {
		t.Fatalf("the ask must carry the repeat refusal: %v", ask["recoveryNote"])
	}

	// The taint STOP: a fresh run refuses before any turn machinery.
	code := engine.internalRun("resume", "metasystem-mission-runner-alpha-fixture-2", signal)
	if code == 0 {
		t.Fatal("a tainted mission resumed")
	}
	turnsAfter, _ := filepath.Glob(filepath.Join(engine.missionDir(), "turns", "*", "turn.json"))
	if len(turnsAfter) != 2 {
		t.Fatalf("the tainted mission opened another turn: %v", turnsAfter)
	}
}

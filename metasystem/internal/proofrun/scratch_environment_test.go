package proofrun

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

const scratchEnvSecret = "GOPROXY=https://secret-proxy.invalid\n"

type scratchEnvFixture struct {
	host    string
	goEnv   string
	base    []string
	control string
}

// newScratchEnvFixture is a host HOME holding an inherited Go env file at the
// explicit GOENV path, plus variables a host shell would carry.
func newScratchEnvFixture(t *testing.T) scratchEnvFixture {
	t.Helper()
	host := t.TempDir()
	goEnv := filepath.Join(host, "goenv")
	if err := os.WriteFile(goEnv, []byte(scratchEnvSecret), 0o600); err != nil {
		t.Fatal(err)
	}
	return scratchEnvFixture{host: host, goEnv: goEnv, control: t.TempDir(), base: []string{
		"HOME=" + host, "PATH=/usr/bin:/bin", "HOST_ONLY=1", "GOENV=" + goEnv,
		"GOMODCACHE=" + filepath.Join(host, "modcache"), "TMPDIR=" + filepath.Join(host, "tmp"),
	}}
}

func scratchEnvGroups() []testpolicy.Group {
	var groups []testpolicy.Group
	for _, adapter := range []string{"go", "command", "section"} {
		for _, mode := range []string{"inherit", "explicit"} {
			groups = append(groups, testpolicy.Group{ID: adapter + "-" + mode, Adapter: adapter, EnvironmentMode: mode,
				Env: map[string]string{"PATH": "/usr/bin:/bin"}})
		}
	}
	return groups
}

func scratchEnvRequest(base []string, groups []testpolicy.Group) TestRunRequest {
	request := TestRunRequest{Environment: append([]string(nil), base...)}
	request.Contract.Groups = groups
	for _, group := range groups {
		request.Plan.SelectedGroups = append(request.Plan.SelectedGroups, group.ID)
	}
	return request
}

func prepareScratchEnv(t *testing.T, control string, request TestRunRequest) (TestRunRequest, *ScratchRun) {
	t.Helper()
	run, err := CreateScratchRun(control)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = run.Cleanup(nil) })
	if err := PrepareScratchEnvironment(&request, run); err != nil {
		t.Fatal(err)
	}
	return request, run
}

func scratchEnvDigests(request TestRunRequest) map[string]string {
	digests := map[string]string{}
	for _, group := range request.Contract.Groups {
		digests[group.ID] = digestScratchGroupEnvironment(request, group, groupTestEnvironment(request, group))
	}
	return digests
}

func TestScratchEnvironmentManagesEveryAdapterInBothModes(t *testing.T) {
	t.Parallel()
	fixture := newScratchEnvFixture(t)
	request, run := prepareScratchEnv(t, fixture.control, scratchEnvRequest(fixture.base, scratchEnvGroups()))
	for _, group := range request.Contract.Groups {
		environment := groupTestEnvironment(request, group)
		dir := filepath.Join(run.Root(), "groups", group.ID, scratchEnvironmentDir)
		want := map[string]string{
			"HOME": filepath.Join(dir, "home"), "XDG_CONFIG_HOME": filepath.Join(dir, "home", ".config"),
			"XDG_CACHE_HOME": filepath.Join(dir, "home", ".cache"), "XDG_DATA_HOME": filepath.Join(dir, "home", ".local", "share"),
			"TMPDIR": filepath.Join(dir, "tmp"), "TMP": filepath.Join(dir, "tmp"), "TEMP": filepath.Join(dir, "tmp"),
			"GOTMPDIR": filepath.Join(dir, "tmp"), "GOCACHE": run.Dir("gocache"), "STATICCHECK_CACHE": run.Dir("staticcheck"),
			"GOENV": filepath.Join(run.Dir("goenv"), "env"),
			// Dependency stores stay where the caller's Go keeps them.
			"GOPATH": filepath.Join(fixture.host, "go"), "GOMODCACHE": filepath.Join(fixture.host, "modcache"),
		}
		for name, value := range want {
			if got := lookupEnvironment(environment, name); got != value {
				t.Errorf("%s %s = %q, want %q", group.ID, name, got, value)
			}
		}
		if _, inherited := lookupEnvironmentSet(environment, "HOST_ONLY"); inherited != (group.EnvironmentMode == "inherit") {
			t.Errorf("%s inherited HOST_ONLY = %v", group.ID, inherited)
		}
		// A group process writing through its home, cache and temp variables
		// lands only beneath the run root.
		script := `echo x > "$HOME/h" && echo x > "$XDG_CACHE_HOME/c" && echo x > "$TMPDIR/t"`
		command := exec.Command("/bin/sh", "-c", script)
		command.Env = environment
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%s write: %v %s", group.ID, err, output)
		}
		for _, path := range []string{filepath.Join(dir, "home", "h"), filepath.Join(dir, "home", ".cache", "c"), filepath.Join(dir, "tmp", "t")} {
			if _, err := os.Stat(path); err != nil {
				t.Errorf("%s: %v", group.ID, err)
			}
		}
	}
	if err := ValidateScratchEnvironment(request, run); err != nil {
		t.Fatal(err)
	}
	// Concurrent readers of one request derive identical, group-private views.
	var wait sync.WaitGroup
	results := make([][]string, 8)
	for index := range results {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results[index] = groupTestEnvironment(request, request.Contract.Groups[index%2])
		}()
	}
	wait.Wait()
	for index := range results {
		if strings.Join(results[index], "\n") != strings.Join(results[index%2], "\n") {
			t.Fatalf("concurrent environment %d differs", index)
		}
	}
	if lookupEnvironment(results[0], "HOME") == lookupEnvironment(results[1], "HOME") {
		t.Fatal("two groups share a managed HOME")
	}
}

func TestScratchEnvironmentGoEnvModes(t *testing.T) {
	t.Parallel()
	fixture := newScratchEnvFixture(t)
	without := func(name string) []string {
		var result []string
		for _, entry := range fixture.base {
			if !strings.HasPrefix(entry, name+"=") {
				result = append(result, entry)
			}
		}
		return result
	}
	defaultPath := filepath.Join(fixture.host, ".config", "go", "env")
	if runtime.GOOS == "darwin" {
		defaultPath = filepath.Join(fixture.host, "Library", "Application Support", "go", "env")
	}
	if err := os.MkdirAll(filepath.Dir(defaultPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defaultPath, []byte("GOFLAGS=-mod=mod\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		base     []string
		snapshot string
		goEnv    string
	}{
		{"explicit", fixture.base, scratchEnvSecret, ""},
		{"default", without("GOENV"), "GOFLAGS=-mod=mod\n", ""},
		{"absent", append(without("GOENV"), "GOENV="+filepath.Join(fixture.host, "missing")), "", ""},
		{"off", append(without("GOENV"), "GOENV=off"), "", "off"},
	}
	group := scratchEnvGroups()[1]
	for _, test := range cases {
		request, run := prepareScratchEnv(t, fixture.control, scratchEnvRequest(test.base, []testpolicy.Group{group}))
		environment := groupTestEnvironment(request, group)
		snapshot := filepath.Join(run.Dir("goenv"), "env")
		if test.name == "default" {
			// An unset caller GOENV snapshots the default file at the group's
			// own managed default location instead of a run-wide file.
			_, snapshot, _, _ = request.ScratchEnvironment.defaultGoEnvPaths(request.Environment, group)
		}
		if test.goEnv == "off" {
			if got := lookupEnvironment(environment, "GOENV"); got != "off" {
				t.Fatalf("off: GOENV = %q", got)
			}
			if _, err := os.Stat(snapshot); !os.IsNotExist(err) {
				t.Fatalf("off wrote a snapshot: %v", err)
			}
		} else {
			contents, err := os.ReadFile(snapshot)
			if err != nil || string(contents) != test.snapshot {
				t.Fatalf("%s snapshot = %q, %v", test.name, contents, err)
			}
		}
		encoded, _ := json.Marshal(request)
		if strings.Contains(string(encoded), "secret-proxy") {
			t.Fatalf("%s: Go env contents leaked into the request", test.name)
		}
		if err := ValidateScratchEnvironment(request, run); err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
	}
	// A relative inherited GOENV fails before any group tree exists.
	run, err := CreateScratchRun(fixture.control)
	if err != nil {
		t.Fatal(err)
	}
	defer run.Cleanup(nil)
	request := scratchEnvRequest(append(without("GOENV"), "GOENV=relative/env"), []testpolicy.Group{group})
	if err := PrepareScratchEnvironment(&request, run); err == nil || request.ScratchEnvironment != nil {
		t.Fatalf("relative GOENV prepared: %v", err)
	}
	if _, err := os.Stat(filepath.Join(run.Dir("groups"), group.ID)); !os.IsNotExist(err) {
		t.Fatalf("group tree created before the snapshot failed: %v", err)
	}
}

func TestScratchEnvironmentIdentity(t *testing.T) {
	t.Parallel()
	fixture := newScratchEnvFixture(t)
	declaredHome := filepath.Join(fixture.host, "declared-home")
	declaredGoEnv := filepath.Join(fixture.host, "declared-goenv")
	for _, path := range []string{declaredHome} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	sentinel := filepath.Join(declaredHome, "sentinel")
	if err := os.WriteFile(sentinel, []byte("user"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(declaredGoEnv, []byte("GOFLAGS=-v\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	groups := append(scratchEnvGroups(), testpolicy.Group{ID: "declared", Adapter: "command", EnvironmentMode: "explicit",
		Env: map[string]string{"HOME": declaredHome, "GOENV": declaredGoEnv}})
	first, firstRun := prepareScratchEnv(t, fixture.control, scratchEnvRequest(fixture.base, groups))
	second, secondRun := prepareScratchEnv(t, fixture.control, scratchEnvRequest(fixture.base, groups))
	if firstRun.Root() == secondRun.Root() {
		t.Fatal("runs share a root")
	}
	firstDigests, secondDigests := scratchEnvDigests(first), scratchEnvDigests(second)
	for id, digest := range firstDigests {
		if secondDigests[id] != digest {
			t.Errorf("%s identity changed with the run root", id)
		}
	}
	declared := groups[len(groups)-1]
	environment := groupTestEnvironment(first, declared)
	if lookupEnvironment(environment, "HOME") != declaredHome || lookupEnvironment(environment, "GOENV") != declaredGoEnv {
		t.Fatal("declared HOME or GOENV was replaced")
	}
	// A declared path digests literally: moving it changes identity.
	moved := declared
	moved.Env = map[string]string{"HOME": declaredHome + "-2", "GOENV": declaredGoEnv}
	if digestScratchGroupEnvironment(first, moved, groupTestEnvironment(first, moved)) == firstDigests["declared"] {
		t.Fatal("declared HOME digested as a managed token")
	}
	// A variable claiming a generated-looking value is not normalized unless
	// it equals this descriptor's path for this group.
	forged := append(groupTestEnvironment(first, groups[1]), "HOME="+filepath.Join(secondRun.Root(), "groups", groups[1].ID, scratchEnvironmentDir, "home"))
	if digestScratchGroupEnvironment(first, groups[1], forged) == firstDigests[groups[1].ID] {
		t.Fatal("foreign root path normalized")
	}
	// Changed inherited config, or a changed declared config, invalidates.
	if err := os.WriteFile(fixture.goEnv, []byte("GOFLAGS=-mod=vendor\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(declaredGoEnv, []byte("GOFLAGS=-x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	third, _ := prepareScratchEnv(t, fixture.control, scratchEnvRequest(fixture.base, groups))
	for id, digest := range scratchEnvDigests(third) {
		if digest == firstDigests[id] {
			t.Errorf("%s identity survived a Go env change", id)
		}
	}
	// The worker rejects a declared config that changed after preparation.
	if err := ValidateScratchEnvironment(first, firstRun); err == nil {
		t.Fatal("changed declared GOENV accepted")
	}
	// Legacy callers keep the legacy digest.
	legacy := scratchEnvRequest(fixture.base, groups)
	if digestScratchGroupEnvironment(legacy, groups[0], groupTestEnvironment(legacy, groups[0])) != digestGroupEnvironment(groups[0], groupTestEnvironment(legacy, groups[0])) {
		t.Fatal("legacy digest changed")
	}
	// Root cleanup removes owned bytes; the declared home is untouched.
	if err := secondRun.Cleanup(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(secondRun.Root()); !os.IsNotExist(err) {
		t.Fatalf("run root survived cleanup: %v", err)
	}
	if contents, err := os.ReadFile(sentinel); err != nil || string(contents) != "user" {
		t.Fatalf("declared HOME sentinel: %q %v", contents, err)
	}
}

func TestScratchEnvironmentValidationRejectsTampering(t *testing.T) {
	t.Parallel()
	fixture := newScratchEnvFixture(t)
	groups := scratchEnvGroups()
	prepare := func() (TestRunRequest, *ScratchRun) {
		return prepareScratchEnv(t, fixture.control, scratchEnvRequest(fixture.base, groups))
	}
	if err := ValidateScratchEnvironment(scratchEnvRequest(fixture.base, groups), nil); err != nil {
		t.Fatalf("legacy request rejected: %v", err)
	}
	request, run := prepare()
	other, otherRun := prepare()
	cases := map[string]func() (TestRunRequest, *ScratchRun){
		"missing descriptor": func() (TestRunRequest, *ScratchRun) {
			stripped := request
			stripped.ScratchEnvironment = nil
			return stripped, run
		},
		"other run": func() (TestRunRequest, *ScratchRun) { return other, run },
		"policy": func() (TestRunRequest, *ScratchRun) {
			copied := *request.ScratchEnvironment
			copied.Policy = "scratch-environment/v0"
			edited := request
			edited.ScratchEnvironment = &copied
			return edited, run
		},
		"unprepared group": func() (TestRunRequest, *ScratchRun) {
			edited := request
			edited.Plan.SelectedGroups = append(append([]string(nil), request.Plan.SelectedGroups...), "extra")
			return edited, run
		},
		"snapshot bytes": func() (TestRunRequest, *ScratchRun) {
			if err := os.WriteFile(filepath.Join(otherRun.Dir("goenv"), "env"), []byte("GOFLAGS=-x\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			return other, otherRun
		},
	}
	for name, build := range cases {
		edited, against := build()
		if err := ValidateScratchEnvironment(edited, against); err == nil {
			t.Errorf("%s accepted", name)
		} else if strings.Contains(err.Error(), "secret-proxy") {
			t.Errorf("%s error leaked Go env contents", name)
		}
	}
	// A group tree replaced by a symlink to user bytes is refused before the
	// worker reads or removes anything through it.
	outside := t.TempDir()
	home := filepath.Join(run.Root(), "groups", groups[0].ID, scratchEnvironmentDir, "home")
	if err := os.RemoveAll(home); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, home); err != nil {
		t.Fatal(err)
	}
	if err := ValidateScratchEnvironment(request, run); err == nil {
		t.Fatal("symlinked managed HOME accepted")
	}
	// Preparation refuses an absent selected group before creating trees.
	missing := scratchEnvRequest(fixture.base, groups)
	missing.Plan.SelectedGroups = append(missing.Plan.SelectedGroups, "absent")
	fresh, err := CreateScratchRun(fixture.control)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Cleanup(nil)
	if err := PrepareScratchEnvironment(&missing, fresh); err == nil || missing.ScratchEnvironment != nil {
		t.Fatalf("absent group prepared: %v", err)
	}
}

func TestScratchEnvironmentDependencyStoresFollowEachGroup(t *testing.T) {
	t.Parallel()
	fixture := newScratchEnvFixture(t)
	external := filepath.Join(fixture.host, "external-home")
	base := []string{"HOME=" + fixture.host, "PATH=/usr/bin:/bin", "GOENV=" + fixture.goEnv}
	explicit := func(id string, env map[string]string) testpolicy.Group {
		return testpolicy.Group{ID: id, Adapter: "go", EnvironmentMode: "explicit", Env: env}
	}
	groups := []testpolicy.Group{
		explicit("plain", map[string]string{}),
		explicit("declared-home", map[string]string{"HOME": external}),
		explicit("declared-store", map[string]string{"GOPATH": "/declared/gopath", "GOMODCACHE": "/declared/mod"}),
		{ID: "inherit-home", Adapter: "command", EnvironmentMode: "inherit", Env: map[string]string{"HOME": external}},
	}
	request, run := prepareScratchEnv(t, fixture.control, scratchEnvRequest(base, groups))
	want := map[string][2]string{
		"plain":          {filepath.Join(fixture.host, "go"), ""},
		"declared-home":  {filepath.Join(external, "go"), ""},
		"declared-store": {"/declared/gopath", "/declared/mod"},
		"inherit-home":   {filepath.Join(external, "go"), ""},
	}
	for _, group := range groups {
		environment := groupTestEnvironment(request, group)
		if got := lookupEnvironment(environment, "GOPATH"); got != want[group.ID][0] || strings.HasPrefix(got, run.Root()) {
			t.Errorf("%s GOPATH = %q, want %q", group.ID, got, want[group.ID][0])
		}
		if got := lookupEnvironment(environment, "GOMODCACHE"); got != want[group.ID][1] {
			t.Errorf("%s GOMODCACHE = %q, want %q", group.ID, got, want[group.ID][1])
		}
	}
	// An inherited Go env file that sets GOPATH travels with the snapshot; a
	// group that turns GOENV off falls back to its own pre-managed HOME/go,
	// never to a fresh store under the managed HOME.
	if err := os.WriteFile(fixture.goEnv, []byte("GOPATH=/from/file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	groups = []testpolicy.Group{explicit("plain", map[string]string{}), explicit("off", map[string]string{"GOENV": "off"})}
	request, _ = prepareScratchEnv(t, fixture.control, scratchEnvRequest(base, groups))
	if got, set := lookupEnvironmentSet(groupTestEnvironment(request, groups[0]), "GOPATH"); set {
		t.Errorf("file-set GOPATH was overridden with %q", got)
	}
	if got := lookupEnvironment(groupTestEnvironment(request, groups[1]), "GOPATH"); got != filepath.Join(fixture.host, "go") {
		t.Errorf("GOENV=off group GOPATH = %q", got)
	}
}

func TestScratchEnvironmentSnapshotSymlinkRejected(t *testing.T) {
	t.Parallel()
	fixture := newScratchEnvFixture(t)
	request, run := prepareScratchEnv(t, fixture.control, scratchEnvRequest(fixture.base, scratchEnvGroups()[:1]))
	snapshot := filepath.Join(run.Dir("goenv"), "env")
	// The external target has the snapshot's exact bytes, so only a
	// no-follow read can tell them apart.
	if err := os.Remove(snapshot); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(fixture.goEnv, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := ValidateScratchEnvironment(request, run); err == nil {
		t.Fatal("symlinked Go env snapshot accepted")
	}
	if err := os.Remove(snapshot); err != nil {
		t.Fatal(err)
	}
	if err := ValidateScratchEnvironment(request, run); err == nil {
		t.Fatal("missing Go env snapshot accepted")
	}
}

func TestScratchEnvironmentGoEnvFileForFollowsEachPlatformRule(t *testing.T) {
	t.Parallel()
	cases := []struct {
		goos string
		env  []string
		want string
	}{
		{"linux", []string{"HOME=/h", "XDG_CONFIG_HOME=/x"}, "/x/go/env"},
		{"linux", []string{"HOME=/h", "XDG_CONFIG_HOME=relative"}, "/h/.config/go/env"},
		{"linux", []string{"HOME=/h"}, "/h/.config/go/env"},
		{"linux", nil, ""},
		{"darwin", []string{"HOME=/h", "XDG_CONFIG_HOME=/x"}, "/h/Library/Application Support/go/env"},
		{"darwin", []string{"HOME=/h", "GOENV=/explicit"}, "/explicit"},
	}
	for _, test := range cases {
		got, off, err := goEnvFileFor(test.env, test.goos)
		if err != nil || off || got != test.want {
			t.Errorf("%s %v = %q %v %v, want %q", test.goos, test.env, got, off, err, test.want)
		}
	}
	if _, off, err := goEnvFileFor([]string{"GOENV=off", "HOME=/h"}, "linux"); !off || err != nil {
		t.Fatalf("GOENV=off = %v %v", off, err)
	}
}

func scratchGoModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	files["go.mod"] = "module example.test/workload\n\ngo 1.21\n"
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func scratchGoPath(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not on PATH")
	}
	return filepath.Dir(path) + ":/usr/bin:/bin"
}

func TestScratchEnvironmentDiscoverySharesOnlyManagedDifferences(t *testing.T) {
	t.Parallel()
	fixture := newScratchEnvFixture(t)
	module := scratchGoModule(t, map[string]string{"a_test.go": "package workload\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n"})
	declared := filepath.Join(fixture.host, "declared-goenv")
	if err := os.WriteFile(declared, []byte("GOFLAGS=-mod=mod\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := scratchGoPath(t)
	group := func(id string, env map[string]string) testpolicy.Group {
		env["PATH"], env["GOTOOLCHAIN"] = path, "local"
		return testpolicy.Group{ID: id, Adapter: "go", EnvironmentMode: "explicit", Env: env}
	}
	groups := []testpolicy.Group{group("first", map[string]string{}), group("second", map[string]string{}),
		group("configured", map[string]string{"GOENV": declared})}
	request, _ := prepareScratchEnv(t, fixture.control, scratchEnvRequest(fixture.base, groups))
	cache := &goDiscoveryCache{catalogs: map[string]goPackageCatalog{}, scratch: request.ScratchEnvironment}
	loads := 0
	for _, group := range groups {
		_, started, err := discoverGoTestsCached(t.Context(), module, groupTestEnvironment(request, group), []string{"./..."}, nil, false, cache)
		if err != nil {
			t.Fatalf("%s: %v", group.ID, err)
		}
		if started {
			loads++
		}
	}
	// first and second differ only in owner-generated paths and share one
	// catalog load; the declared Go config keeps its own.
	if loads != 2 || len(cache.catalogs) != 2 {
		t.Fatalf("catalog loads = %d, entries = %d, want 2 and 2", loads, len(cache.catalogs))
	}
	forged := append(groupTestEnvironment(request, groups[0]), "HOME=/elsewhere/home")
	if digestEnvironment(scratchDiscoveryEnvironment(request.ScratchEnvironment, forged)) ==
		digestEnvironment(scratchDiscoveryEnvironment(request.ScratchEnvironment, groupTestEnvironment(request, groups[0]))) {
		t.Fatal("non-generated HOME normalized in the discovery key")
	}
}

// TestScratchEnvironmentRealGoWorkload is A5 for the Go adapter: a real go
// test writes through HOME, the user cache and config dirs and the temp dir
// in its managed environment, and root cleanup removes exactly those bytes.
func TestScratchEnvironmentRealGoWorkload(t *testing.T) {
	t.Parallel()
	fixture := newScratchEnvFixture(t)
	sentinel := filepath.Join(fixture.host, "sentinel")
	if err := os.WriteFile(sentinel, []byte("user"), 0o600); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(fixture.host, "written")
	module := scratchGoModule(t, map[string]string{"writer_test.go": `package workload

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWrite(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	config, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	var written []string
	for _, dir := range []string{home, cache, config, os.Getenv("XDG_DATA_HOME"), os.TempDir()} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "workload-bytes")
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		written = append(written, path)
	}
	if err := os.WriteFile(os.Getenv("WORKLOAD_REPORT"), []byte(strings.Join(written, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
}
`})
	group := testpolicy.Group{ID: "workload", Adapter: "go", EnvironmentMode: "explicit",
		Env: map[string]string{"PATH": scratchGoPath(t), "GOTOOLCHAIN": "local", "WORKLOAD_REPORT": report}}
	request, run := prepareScratchEnv(t, fixture.control, scratchEnvRequest(fixture.base, []testpolicy.Group{group}))
	if err := ValidateScratchEnvironment(request, run); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	command, err := explicitEnvironmentCommand(t.Context(), module, groupTestEnvironment(request, group), []string{"go", "test", "-count=1", "./..."})
	if err != nil {
		t.Fatal(err)
	}
	command.Stdout, command.Stderr = &output, &output
	if err := RunResourceCommand(t.Context(), command, nil); err != nil {
		t.Fatalf("go test: %v\n%s", err, output.String())
	}
	contents, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	written := strings.Split(string(contents), "\n")
	if len(written) != 5 {
		t.Fatalf("written = %q", written)
	}
	for _, path := range written {
		if !strings.HasPrefix(path, filepath.Join(run.Root(), "groups", "workload", scratchEnvironmentDir)+string(filepath.Separator)) {
			t.Errorf("workload wrote outside its managed tree: %s", path)
		}
	}
	if err := run.Cleanup(nil); err != nil {
		t.Fatal(err)
	}
	for _, path := range append(written, run.Root()) {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("%s survived cleanup: %v", path, err)
		}
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "user" {
		t.Fatalf("user sentinel: %q %v", got, err)
	}
}

// A declared empty GOENV means go's default config resolution, not off: the
// group keeps its pre-managed default file's settings and store, which
// override the inherited explicit GOENV file, and identity binds the bytes.
func TestScratchEnvironmentDeclaredEmptyGoEnvKeepsDefaultConfig(t *testing.T) {
	t.Parallel()
	fixture := newScratchEnvFixture(t)
	if err := os.WriteFile(fixture.goEnv, []byte("GOFLAGS=-tags=inherited\nGOPATH=/from/inherited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(fixture.host, "declared-home")
	defaultFile := func(home string) string {
		path, _, _ := goEnvFileFor([]string{"HOME=" + home}, runtime.GOOS)
		return path
	}
	write := func(path, contents string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(defaultFile(fixture.host), "GOFLAGS=-tags=default\nGOPATH=/from/default\n")
	write(defaultFile(external), "GOFLAGS=-tags=external\n")
	path := scratchGoPath(t)
	groups := []testpolicy.Group{
		{ID: "empty", Adapter: "go", EnvironmentMode: "explicit", Env: map[string]string{"GOENV": "", "PATH": path, "GOTOOLCHAIN": "local"}},
		{ID: "empty-home", Adapter: "go", EnvironmentMode: "explicit", Env: map[string]string{"GOENV": "", "HOME": external, "PATH": path, "GOTOOLCHAIN": "local"}},
	}
	want := map[string]string{"empty": "-tags=default\n/from/default\n", "empty-home": "-tags=external\n" + filepath.Join(external, "go") + "\n"}
	request, run := prepareScratchEnv(t, fixture.control, scratchEnvRequest(fixture.base, groups))
	for _, group := range groups {
		environment := groupTestEnvironment(request, group)
		if value, set := lookupEnvironmentSet(environment, "GOENV"); !set || value != "" {
			t.Fatalf("%s GOENV rewritten to %q", group.ID, value)
		}
		command, err := explicitEnvironmentCommand(t.Context(), t.TempDir(), environment, []string{"go", "env", "GOFLAGS", "GOPATH"})
		if err != nil {
			t.Fatal(err)
		}
		output, err := command.Output()
		if err != nil || string(output) != want[group.ID] {
			t.Errorf("%s go env = %q %v, want %q", group.ID, output, err, want[group.ID])
		}
	}
	if err := ValidateScratchEnvironment(request, run); err != nil {
		t.Fatal(err)
	}
	// Relocation reuses; a changed default file invalidates.
	second, _ := prepareScratchEnv(t, fixture.control, scratchEnvRequest(fixture.base, groups))
	first := scratchEnvDigests(request)
	for id, digest := range scratchEnvDigests(second) {
		if digest != first[id] {
			t.Errorf("%s identity changed with the run root", id)
		}
	}
	write(defaultFile(fixture.host), "GOFLAGS=-tags=changed\nGOPATH=/from/default\n")
	write(defaultFile(external), "GOFLAGS=-tags=changed\n")
	third, _ := prepareScratchEnv(t, fixture.control, scratchEnvRequest(fixture.base, groups))
	for id, digest := range scratchEnvDigests(third) {
		if digest == first[id] {
			t.Errorf("%s identity survived a default config change", id)
		}
	}
	var failures []error
	// The external default file changed after preparation.
	if prepared, _ := request.ScratchEnvironment.group("empty-home"); prepared.DefaultGoEnv == "external" {
		failures = append(failures, ValidateScratchEnvironment(request, run))
	}
	// The managed default snapshot replaced by a symlink to identical bytes.
	snapshot := defaultFile(lookupEnvironment(groupTestEnvironment(second, groups[0]), "HOME"))
	outside := filepath.Join(fixture.host, "same-bytes")
	write(outside, "GOFLAGS=-tags=default\nGOPATH=/from/default\n")
	if err := os.Remove(snapshot); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, snapshot); err != nil {
		t.Fatal(err)
	}
	prepared, _ := second.ScratchEnvironment.group(groups[0].ID)
	failures = append(failures, validateDefaultGoEnv(second.ScratchEnvironment, second.Environment, groups[0], prepared, true))
	for _, err := range failures {
		if err == nil {
			t.Fatal("changed default Go env accepted")
		}
		if strings.Contains(err.Error(), "tags=") || strings.Contains(err.Error(), "/from/") {
			t.Fatalf("error leaked config contents: %v", err)
		}
	}
}

// scratchGoEnvOutput runs the real `go env` for names in a group's managed
// environment; it launches no compiler.
func scratchGoEnvOutput(t *testing.T, request TestRunRequest, group testpolicy.Group, names ...string) string {
	t.Helper()
	command, err := explicitEnvironmentCommand(t.Context(), t.TempDir(), groupTestEnvironment(request, group), append([]string{"go", "env"}, names...))
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.Output()
	if err != nil {
		t.Fatalf("%s go env: %v", group.ID, err)
	}
	return string(output)
}

func writeScratchConfig(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

// With the caller's GOENV unset, a group overriding HOME or
// XDG_CONFIG_HOME keeps its own pre-managed default config, and that
// config's bytes bind its identity.
func TestScratchEnvironmentUnsetGoEnvFollowsGroupDefaultConfig(t *testing.T) {
	t.Parallel()
	fixture := newScratchEnvFixture(t)
	base := []string{"HOME=" + fixture.host, "PATH=/usr/bin:/bin"}
	groupHome := filepath.Join(fixture.host, "group-home")
	xdg := filepath.Join(fixture.host, "group-xdg")
	defaultFile := func(home string) string {
		path, _, _ := goEnvFileFor([]string{"HOME=" + home}, runtime.GOOS)
		return path
	}
	writeScratchConfig(t, defaultFile(fixture.host), "GOFLAGS=-tags=caller\n")
	writeScratchConfig(t, defaultFile(groupHome), "GOFLAGS=-tags=group\nGOPATH=/from/group\n")
	writeScratchConfig(t, filepath.Join(xdg, "go", "env"), "GOFLAGS=-tags=xdg\n")
	path := scratchGoPath(t)
	group := func(id string, env map[string]string) testpolicy.Group {
		env["PATH"], env["GOTOOLCHAIN"] = path, "local"
		return testpolicy.Group{ID: id, Adapter: "go", EnvironmentMode: "explicit", Env: env}
	}
	groups := []testpolicy.Group{group("plain", map[string]string{}), group("home", map[string]string{"HOME": groupHome}),
		group("xdg", map[string]string{"XDG_CONFIG_HOME": xdg})}
	callerStore := filepath.Join(fixture.host, "go")
	want := map[string]string{"plain": "-tags=caller\n" + callerStore + "\n", "home": "-tags=group\n/from/group\n",
		"xdg": "-tags=xdg\n" + callerStore + "\n"}
	if runtime.GOOS == "darwin" {
		want["xdg"] = "-tags=caller\n" + callerStore + "\n" // os.UserConfigDir ignores XDG on darwin
	}
	request, run := prepareScratchEnv(t, fixture.control, scratchEnvRequest(base, groups))
	for _, group := range groups {
		if got := scratchGoEnvOutput(t, request, group, "GOFLAGS", "GOPATH"); got != want[group.ID] {
			t.Errorf("%s go env = %q, want %q", group.ID, got, want[group.ID])
		}
	}
	if err := ValidateScratchEnvironment(request, run); err != nil {
		t.Fatal(err)
	}
	first := scratchEnvDigests(request)
	writeScratchConfig(t, defaultFile(groupHome), "GOFLAGS=-tags=changed\nGOPATH=/from/group\n")
	changed, _ := prepareScratchEnv(t, fixture.control, scratchEnvRequest(base, groups))
	second := scratchEnvDigests(changed)
	if second["home"] == first["home"] || second["plain"] != first["plain"] {
		t.Fatalf("group default config change: home %v, plain %v", second["home"] != first["home"], second["plain"] == first["plain"])
	}
}

// A declared empty GOPATH is go's default, derived from the group's
// pre-managed HOME and never from the disposable managed HOME.
func TestScratchEnvironmentDeclaredEmptyGoPathKeepsDefaultStore(t *testing.T) {
	t.Parallel()
	fixture := newScratchEnvFixture(t)
	external := filepath.Join(fixture.host, "declared-home")
	base := []string{"HOME=" + fixture.host, "PATH=/usr/bin:/bin", "GOPATH=/inherited/gopath", "GOENV=" + fixture.goEnv}
	path := scratchGoPath(t)
	groups := []testpolicy.Group{
		{ID: "inherit", Adapter: "go", EnvironmentMode: "inherit", Env: map[string]string{"GOPATH": "", "PATH": path, "GOTOOLCHAIN": "local"}},
		{ID: "explicit", Adapter: "go", EnvironmentMode: "explicit", Env: map[string]string{"GOPATH": "", "HOME": external, "PATH": path, "GOTOOLCHAIN": "local"}},
	}
	want := map[string]string{"inherit": filepath.Join(fixture.host, "go"), "explicit": filepath.Join(external, "go")}
	request, run := prepareScratchEnv(t, fixture.control, scratchEnvRequest(base, groups))
	for _, group := range groups {
		got := strings.TrimSpace(scratchGoEnvOutput(t, request, group, "GOPATH"))
		if got != want[group.ID] || strings.HasPrefix(got, run.Root()) {
			t.Errorf("%s GOPATH = %q, want %q", group.ID, got, want[group.ID])
		}
		if modcache := strings.TrimSpace(scratchGoEnvOutput(t, request, group, "GOMODCACHE")); strings.HasPrefix(modcache, run.Root()) {
			t.Errorf("%s module store is disposable: %s", group.ID, modcache)
		}
	}
}

package stateroot

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func installFixture(t *testing.T, template bool) (installation, app string) {
	t.Helper()
	app = t.TempDir()
	installation = app
	if template {
		installation = filepath.Join(app, "metasystem")
		if err := os.MkdirAll(filepath.Join(app, "development"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(app, "development", "metasystem-design.md"), []byte("design\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(installation, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("evidence.root="+filepath.Join(app, "durable")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return installation, app
}

type topCall struct {
	path string
	root string
	err  error
}

type topRecorder struct {
	t    *testing.T
	want []topCall
	seen []string
}

func (r *topRecorder) expect(path, root string, err error) {
	r.t.Helper()
	r.want = append(r.want, topCall{path, root, err})
}

func (r *topRecorder) expectCanonical(path, root string, err error) {
	r.t.Helper()
	canonical, canonicalErr := filepath.EvalSymlinks(path)
	if canonicalErr != nil {
		r.t.Fatal(canonicalErr)
	}
	r.expect(canonical, root, err)
}

func (r *topRecorder) read(path string) (string, error) {
	r.t.Helper()
	r.seen = append(r.seen, string(append([]byte(nil), path...)))
	if len(r.want) == 0 || r.want[0].path != path {
		r.t.Fatalf("unexpected repository-top request %q; pending %v", path, r.want)
	}
	call := r.want[0]
	r.want = r.want[1:]
	return call.root, call.err
}

func resolverFixture(t *testing.T, installation string) (Resolver, *topRecorder) {
	t.Helper()
	recorder := &topRecorder{t: t}
	t.Cleanup(func() {
		if len(recorder.want) != 0 {
			t.Errorf("repository-top requests not consumed: %v; saw %v", recorder.want, recorder.seen)
		}
	})
	return NewResolver(recorder.read, func() (string, error) { return filepath.Join(installation, "bin", "metasystem"), nil }), recorder
}

func TestStateRootResolvesEveryKindInTemplateAndAdoptedModes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		kind Kind
		rel  string
	}{
		{Registers, "memory"}, {Receipts, "memory"}, {Records, "records"},
		{Goals, "plans/goals"}, {OpenWork, "plans"},
		{Steward, "artifacts/agents/steward"},
	}
	for _, template := range []bool{true, false} {
		t.Run(map[bool]string{true: "template", false: "adopted"}[template], func(t *testing.T) {
			installation, app := installFixture(t, template)
			resolver, top := resolverFixture(t, installation)
			base := app
			if template {
				base = installation
			}
			if !template {
				top.expect(installation, app, nil)
			}
			gotBase, err := resolver.RootForInstallation(installation)
			if err != nil || gotBase != base {
				t.Fatalf("RootForInstallation() = %q, %v; want %q", gotBase, err, base)
			}
			for _, test := range tests {
				if !template {
					top.expect(installation, app, nil)
				}
				got, err := resolver.StateRoot(test.kind)
				if err != nil || got != filepath.Join(base, filepath.FromSlash(test.rel)) {
					t.Errorf("StateRoot(%q) = %q, %v; want %q", test.kind, got, err, filepath.Join(base, filepath.FromSlash(test.rel)))
				}
			}
			if !template {
				top.expect(installation, app, nil)
			}
			got, err := resolver.StateRoot(Evidence)
			if err != nil || got != filepath.Join(app, "durable") {
				t.Errorf("StateRoot(%q) = %q, %v; want configured durable root", Evidence, got, err)
			}
		})
	}
}

func TestStateRootRefusesUnknownKindsAndInvalidInstallationFacts(t *testing.T) {
	t.Parallel()
	if got, err := RelativeRoot(Receipts); err != nil || got != "memory" {
		t.Fatalf("RelativeRoot(%q) = %q, %v; want memory", Receipts, got, err)
	}
	if _, err := RelativeRoot(Kind("unknown")); err == nil {
		t.Fatal("an unknown relative state kind must refuse")
	}
	if _, err := StateRoot(Kind("unknown")); err == nil {
		t.Fatal("an unknown state kind must refuse")
	}
	resolver := NewResolver(func(string) (string, error) { t.Fatal("unexpected Git lookup"); return "", nil }, func() (string, error) { return "", errors.New("unavailable") })
	if _, err := resolver.StateRoot(Registers); err == nil {
		t.Fatal("an unavailable executable path must refuse")
	}
}

func TestRootForInstallationUsesOnlyTheExactTemplateMarker(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	design := filepath.Join(root, "development", "metasystem-design.md")
	if err := os.MkdirAll(filepath.Dir(design), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(design, []byte("design\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	template := filepath.Join(root, "metasystem")
	if err := os.MkdirAll(template, 0o755); err != nil {
		t.Fatal(err)
	}
	resolver, top := resolverFixture(t, template)
	got, err := resolver.RootForInstallation(template)
	if err != nil || got != template {
		t.Fatalf("exact template marker resolved to %q, %v; want %q", got, err, template)
	}

	adopted := filepath.Join(root, "metasystem-copy")
	if err := os.MkdirAll(adopted, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "application")
	top.expect(adopted, want, nil)
	got, err = resolver.RootForInstallation(adopted)
	if err != nil || got != want {
		t.Fatalf("adopted installation resolved to %q, %v; want %q", got, err, want)
	}
}

func TestRootForInstallationPropagatesAdoptedRepositoryFailure(t *testing.T) {
	t.Parallel()
	installation := filepath.Join(t.TempDir(), "metasystem")
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	resolver, top := resolverFixture(t, installation)
	top.expect(installation, "", errors.New("not in a repository"))
	if _, err := resolver.RootForInstallation(installation); err == nil || !strings.Contains(err.Error(), "not in a repository") {
		t.Fatalf("adopted repository failure was hidden: %v", err)
	}
}

func TestRootForCandidateCanonicalizesAndValidatesTheInstallation(t *testing.T) {
	t.Parallel()
	outer := t.TempDir()
	installation := filepath.Join(outer, "metasystem")
	if err := os.MkdirAll(filepath.Join(installation, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(outer, "development"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outer, "development", "metasystem-design.md"), []byte("design\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "installation-link")
	if err := os.Symlink(installation, link); err != nil {
		t.Fatal(err)
	}

	got, err := RootForCandidate(link)
	want, wantErr := filepath.EvalSymlinks(installation)
	if wantErr != nil {
		t.Fatal(wantErr)
	}
	if err != nil || got != want {
		t.Fatalf("RootForCandidate() = %q, %v; want %q", got, err, want)
	}
}

func TestRootForCandidateKeepsNestedAdoptedStateAtTheInstallation(t *testing.T) {
	t.Parallel()
	outer := t.TempDir()
	installation := filepath.Join(outer, "vendor", "metasystem-runtime")
	if err := os.MkdirAll(filepath.Join(installation, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := RootForCandidate(installation)
	want, wantErr := filepath.EvalSymlinks(installation)
	if wantErr != nil {
		t.Fatal(wantErr)
	}
	if err != nil || got != want {
		t.Fatalf("RootForCandidate() = %q, %v; want installation %q", got, err, want)
	}
}

func TestRootForCandidateRefusesAPathWithoutInstallationShape(t *testing.T) {
	t.Parallel()
	if _, err := RootForCandidate(t.TempDir()); err == nil || !strings.Contains(err.Error(), "not a metasystem installation") {
		t.Fatalf("candidate without an installation shape was accepted: %v", err)
	}
}

type commandRecorder struct {
	t      *testing.T
	want   commandRequest
	output []byte
	err    error
	seen   []commandRequest
}

func (r *commandRecorder) run(request commandRequest) ([]byte, error) {
	r.t.Helper()
	copyRequest := commandRequest{name: request.name, args: append([]string(nil), request.args...), env: append([]string(nil), request.env...)}
	r.seen = append(r.seen, copyRequest)
	if len(r.seen) != 1 || !reflect.DeepEqual(copyRequest, r.want) {
		r.t.Fatalf("unexpected command request: %+v", copyRequest)
	}
	return append([]byte(nil), r.output...), r.err
}

func TestRepositoryTopBuildsCommandAndScrubsEverySteeringVariable(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "installation")
	environment := []string{"UNRELATED=kept"}
	for _, name := range []string{
		"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE",
		"GIT_CEILING_DIRECTORIES", "GIT_DISCOVERY_ACROSS_FILESYSTEM",
		"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
		"GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT",
		"GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_NOSYSTEM",
		"GIT_GRAFT_FILE", "GIT_SHALLOW_FILE", "GIT_REPLACE_REF_BASE",
		"GIT_IMPLICIT_WORK_TREE", "GIT_NO_REPLACE_OBJECTS", "GIT_PREFIX",
	} {
		environment = append(environment, name+"=poison")
	}
	want := commandRequest{name: "git", args: []string{"-C", path, "rev-parse", "--show-toplevel"}, env: []string{"UNRELATED=kept"}}
	success := &commandRecorder{t: t, want: want, output: []byte("  " + path + "\n")}
	got, err := repositoryTopWith(path, environment, success.run)
	if err != nil || got != path || len(success.seen) != 1 {
		t.Fatalf("repository top = %q, %v; requests %v", got, err, success.seen)
	}
	relative := filepath.Join("relative", "repository with spaces")
	wantAbsolute, err := filepath.Abs(relative)
	if err != nil {
		t.Fatal(err)
	}
	relativeSuccess := &commandRecorder{t: t, want: want, output: []byte("  " + relative + "\n")}
	got, err = repositoryTopWith(path, environment, relativeSuccess.run)
	if err != nil || got != wantAbsolute || len(relativeSuccess.seen) != 1 {
		t.Fatalf("relative repository top = %q, %v; want %q; requests %v", got, err, wantAbsolute, relativeSuccess.seen)
	}
	failure := &commandRecorder{t: t, want: want, output: []byte("fatal: inaccessible\n"), err: errors.New("exit status 128")}
	_, err = repositoryTopWith(path, environment, failure.run)
	if err == nil || !strings.Contains(err.Error(), "fatal: inaccessible") || len(failure.seen) != 1 {
		t.Fatalf("combined output lost: %v; requests %v", err, failure.seen)
	}
}

func TestEvidenceRootMustBeConfiguredAndAbsolute(t *testing.T) {
	t.Parallel()
	installation, app := installFixture(t, false)
	resolver, top := resolverFixture(t, installation)
	if err := os.WriteFile(filepath.Join(app, "metasystem.conf"), []byte("evidence.root=relative\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	top.expect(installation, app, nil)
	if _, err := resolver.StateRoot(Evidence); err == nil {
		t.Fatal("a relative durable evidence root must refuse")
	}
}

func TestResolveLayoutSupportsNestedAndAdoptedRepositoriesFromSubdirectories(t *testing.T) {
	t.Parallel()
	for _, nested := range []bool{false, true} {
		t.Run(map[bool]string{false: "adopted", true: "nested"}[nested], func(t *testing.T) {
			repo := filepath.Join(t.TempDir(), "repository with spaces")
			if err := os.MkdirAll(repo, 0o755); err != nil {
				t.Fatal(err)
			}
			installation := repo
			if nested {
				installation = filepath.Join(repo, "metasystem")
				writeLayoutFile(t, filepath.Join(repo, "development", "metasystem-design.md"), "design\n")
			}
			writeLayoutFile(t, filepath.Join(installation, "metasystem.conf"), "metasystem.runtimes=claude\n")
			if err := os.MkdirAll(filepath.Join(installation, "scripts", "agents"), 0o755); err != nil {
				t.Fatal(err)
			}
			subdir := filepath.Join(repo, "sub", "directory")
			if err := os.MkdirAll(subdir, 0o755); err != nil {
				t.Fatal(err)
			}
			resolver, top := resolverFixture(t, installation)
			top.expectCanonical(subdir, repo, nil)
			layout, err := resolver.ResolveLayout(subdir)
			if err != nil {
				t.Fatal(err)
			}
			wantRepo, _ := filepath.EvalSymlinks(repo)
			wantInstallation, _ := filepath.EvalSymlinks(installation)
			if layout.RepositoryRoot != wantRepo || layout.InstallationRoot != wantInstallation || layout.Template != nested {
				t.Fatalf("layout = %+v; want repository %s installation %s nested %v", layout, wantRepo, wantInstallation, nested)
			}
		})
	}
}

func TestResolveLayoutSupportsFreshAdoptedTargetBeforeGitInit(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "fresh adopted target")
	writeLayoutFile(t, filepath.Join(root, "metasystem.conf"), "metasystem.runtimes=claude\n")
	if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "skills", "demo")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	resolver, top := resolverFixture(t, root)
	top.expectCanonical(child, "", errors.New("not in a repository"))
	layout, err := resolver.ResolveLayout(child)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(root)
	if layout.RepositoryRoot != want || layout.InstallationRoot != want || layout.Template {
		t.Fatalf("fresh adopted layout = %+v; want root %s", layout, want)
	}
}

func TestResolveLayoutSelectsExplicitNestedAdoptedInstallation(t *testing.T) {
	t.Parallel()
	app := filepath.Join(t.TempDir(), "containing application")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}

	writeLayoutFile(t, filepath.Join(app, "metasystem.conf"), "metasystem.runtimes=claude\n")
	if err := os.MkdirAll(filepath.Join(app, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	installation := filepath.Join(app, "vendor", "nested runtime")
	writeLayoutFile(t, filepath.Join(installation, "metasystem.conf"), "metasystem.runtimes=codex\n")
	child := filepath.Join(installation, "skills", "demo")
	if err := os.MkdirAll(filepath.Join(installation, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	resolver, top := resolverFixture(t, installation)
	top.expectCanonical(child, app, nil)
	nested, err := resolver.ResolveLayout(child)
	if err != nil {
		t.Fatal(err)
	}
	wantApp, _ := filepath.EvalSymlinks(app)
	wantInstallation, _ := filepath.EvalSymlinks(installation)
	if nested.GitRoot != wantApp || nested.RepositoryRoot != wantInstallation || nested.InstallationRoot != wantInstallation || nested.InstallationRel != "vendor/nested runtime" || nested.Template {
		t.Fatalf("nested adopted layout = %+v", nested)
	}

	top.expectCanonical(app, app, nil)
	parent, err := resolver.ResolveLayout(app)
	if err != nil {
		t.Fatal(err)
	}
	if parent.GitRoot != wantApp || parent.RepositoryRoot != wantApp || parent.InstallationRoot != wantApp || parent.InstallationRel != "." || parent.Template {
		t.Fatalf("parent adopted layout = %+v", parent)
	}
}

func writeLayoutFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

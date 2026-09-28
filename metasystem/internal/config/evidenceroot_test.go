package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

// evidenceFixture is an installation at <base>/repo/metasystem whose checkout
// is <base>/repo (a .git FILE two levels above the conf, as a worktree has),
// with the given committed and .local bodies ("" local means no .local file).
type evidenceFixture struct {
	base, checkout, installation, conf string
}

func newEvidenceFixture(t *testing.T, committed, local string) evidenceFixture {
	t.Helper()
	base := t.TempDir()
	checkout := filepath.Join(base, "repo")
	installation := filepath.Join(checkout, "metasystem")
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(checkout, ".git"), "gitdir: /elsewhere\n")
	conf := filepath.Join(installation, "metasystem.conf")
	putFile(t, conf, committed)
	if local != "" {
		putFile(t, conf+".local", local)
	}
	return evidenceFixture{base: base, checkout: checkout, installation: installation, conf: conf}
}

func envMap(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

const evidencePlaceholder = EvidenceRootKey + "=<durable evidence root, outside the repository>\n"

func TestResolveEvidenceRootOrder(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	envRoot := filepath.Join(outside, "env")
	localRoot := filepath.Join(outside, "local")
	committedRoot := filepath.Join(outside, "committed")
	home := filepath.Join(outside, "home")
	envKey := EnvName(EvidenceRootKey)
	cases := []struct {
		name, committed, local string
		env                    map[string]string
		wantPath, wantOrigin   string
	}{
		{"env wins over .local", EvidenceRootKey + "=" + committedRoot + "\n", EvidenceRootKey + "=" + localRoot + "\n",
			map[string]string{envKey: envRoot, "HOME": home}, envRoot, "env"},
		{".local wins over committed", EvidenceRootKey + "=" + committedRoot + "\n", EvidenceRootKey + "=" + localRoot + "\n",
			map[string]string{"HOME": home}, localRoot, "conf-local"},
		{"committed wins over the default", EvidenceRootKey + "=" + committedRoot + "\n", "",
			map[string]string{"HOME": home}, committedRoot, "conf"},
		{"m1c-shaped: a placeholder in .local falls through to a committed real path", EvidenceRootKey + "=" + committedRoot + "\n", evidencePlaceholder,
			map[string]string{"HOME": home}, committedRoot, "conf"},
		{"placeholder everywhere resolves to the default", evidencePlaceholder, evidencePlaceholder,
			map[string]string{envKey: "<unset>", "HOME": home}, "", "default"},
		{"a set-but-empty environment variable is unspecified", "x=1\n", EvidenceRootKey + "=" + localRoot + "\n",
			map[string]string{envKey: "  ", "HOME": home}, localRoot, "conf-local"},
		{"an empty value is unspecified", EvidenceRootKey + "=\n", EvidenceRootKey + "=   \n",
			map[string]string{"HOME": home}, "", "default"},
		{"an absent key is unspecified", "x=1\n", "y=2\n",
			map[string]string{"HOME": home}, "", "default"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := newEvidenceFixture(t, c.committed, c.local)
			got, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: f.conf, LookupEnv: envMap(c.env)})
			if err != nil {
				t.Fatal(err)
			}
			want := c.wantPath
			if want == "" {
				want = filepath.Join(home, "metasystem-evidence", "repo")
			}
			if got.Path != want || got.Origin != c.wantOrigin {
				t.Fatalf("got %+v; want %s from %s", got, want, c.wantOrigin)
			}
			if got.Default() != (c.wantOrigin == "default") {
				t.Fatalf("Default() = %v for origin %s", got.Default(), got.Origin)
			}
		})
	}
}

func TestResolveEvidenceRootRefusesSpecifiedBadValues(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	t.Run("a relative .local value is refused naming .local", func(t *testing.T) {
		t.Parallel()
		outside := t.TempDir()
		f := newEvidenceFixture(t, EvidenceRootKey+"="+outside+"\n", EvidenceRootKey+"=relative/dir\n")
		_, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: f.conf, LookupEnv: envMap(map[string]string{"HOME": home})})
		if err == nil || err.Error() != `evidence.root must be absolute (metasystem.conf.local reads "relative/dir")` {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("a relative environment value is refused naming the variable", func(t *testing.T) {
		t.Parallel()
		f := newEvidenceFixture(t, "x=1\n", "")
		_, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: f.conf, LookupEnv: envMap(map[string]string{"HOME": home, EnvName(EvidenceRootKey): "rel"})})
		if err == nil || !strings.Contains(err.Error(), "must be absolute (METASYSTEM_EVIDENCE_ROOT reads \"rel\")") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("a value inside the checkout is refused", func(t *testing.T) {
		t.Parallel()
		f := newEvidenceFixture(t, "x=1\n", "")
		inside := filepath.Join(f.checkout, "evidence")
		putFile(t, f.conf, EvidenceRootKey+"="+inside+"\n")
		_, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: f.conf, LookupEnv: envMap(map[string]string{"HOME": home})})
		if err == nil || err.Error() != `evidence.root must be outside the repository (metasystem.conf reads "`+inside+`")` {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("the checkout itself is refused", func(t *testing.T) {
		t.Parallel()
		f := newEvidenceFixture(t, "x=1\n", "")
		putFile(t, f.conf+".local", EvidenceRootKey+"="+f.checkout+"\n")
		if _, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: f.conf, LookupEnv: envMap(map[string]string{"HOME": home})}); err == nil || !strings.Contains(err.Error(), "must be outside the repository") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("a duplicate key is an error", func(t *testing.T) {
		t.Parallel()
		f := newEvidenceFixture(t, "x=1\n", EvidenceRootKey+"=/a\n"+EvidenceRootKey+"=/b\n")
		if _, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: f.conf, LookupEnv: envMap(map[string]string{"HOME": home})}); err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("an unset HOME is an error naming the fix", func(t *testing.T) {
		t.Parallel()
		f := newEvidenceFixture(t, evidencePlaceholder, "")
		_, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: f.conf, LookupEnv: envMap(nil)})
		if err == nil || !strings.Contains(err.Error(), "HOME") || !strings.Contains(err.Error(), "metasystem.conf.local") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestResolveEvidenceRootNamesTheCheckout(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	lookup := envMap(map[string]string{"HOME": home})
	t.Run("a .git-less installation names its own basename", func(t *testing.T) {
		t.Parallel()
		installation := filepath.Join(t.TempDir(), "fresh-app")
		if err := os.MkdirAll(installation, 0o755); err != nil {
			t.Fatal(err)
		}
		conf := filepath.Join(installation, "metasystem.conf")
		putFile(t, conf, "x=1\n")
		got, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: conf, LookupEnv: lookup})
		if err != nil || got.Path != filepath.Join(home, "metasystem-evidence", "fresh-app") {
			t.Fatalf("got %+v, %v", got, err)
		}
	})
	t.Run("a nested <repo>/metasystem names <repo> through a .git directory", func(t *testing.T) {
		t.Parallel()
		repo := filepath.Join(t.TempDir(), "my-app")
		if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(repo, "metasystem"), 0o755); err != nil {
			t.Fatal(err)
		}
		conf := filepath.Join(repo, "metasystem", "metasystem.conf")
		putFile(t, conf, evidencePlaceholder)
		got, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: conf, LookupEnv: lookup})
		if err != nil || got.Path != filepath.Join(home, "metasystem-evidence", "my-app") || got.Origin != "default" {
			t.Fatalf("got %+v, %v", got, err)
		}
	})
}

// ERD-03: a caller validating candidate.conf is judged on candidate.conf and
// its own .local, never on the metasystem.conf beside it.
func TestResolveEvidenceRootJudgesTheFileItIsGiven(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	lookup := envMap(map[string]string{"HOME": home})
	outside := t.TempDir()
	t.Run("candidate relative, standard absolute", func(t *testing.T) {
		t.Parallel()
		f := newEvidenceFixture(t, EvidenceRootKey+"="+outside+"\n", "")
		candidate := filepath.Join(f.installation, "candidate.conf")
		putFile(t, candidate, EvidenceRootKey+"=relative\n")
		if _, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: candidate, LookupEnv: lookup}); err == nil || !strings.Contains(err.Error(), `candidate.conf reads "relative"`) {
			t.Fatalf("candidate: got %v", err)
		}
		if got, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: f.conf, LookupEnv: lookup}); err != nil || got.Path != outside {
			t.Fatalf("standard: got %+v, %v", got, err)
		}
	})
	t.Run("candidate absolute, standard relative", func(t *testing.T) {
		t.Parallel()
		f := newEvidenceFixture(t, EvidenceRootKey+"=relative\n", "")
		candidate := filepath.Join(f.installation, "candidate.conf")
		putFile(t, candidate, EvidenceRootKey+"="+outside+"\n")
		if got, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: candidate, LookupEnv: lookup}); err != nil || got.Path != outside || got.Origin != "conf" {
			t.Fatalf("candidate: got %+v, %v", got, err)
		}
		if _, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: f.conf, LookupEnv: lookup}); err == nil {
			t.Fatal("standard: a relative root was accepted")
		}
	})
	t.Run("candidate .local is its own", func(t *testing.T) {
		t.Parallel()
		f := newEvidenceFixture(t, "x=1\n", EvidenceRootKey+"=relative\n")
		candidate := filepath.Join(f.installation, "candidate.conf")
		putFile(t, candidate, "x=1\n")
		putFile(t, candidate+".local", EvidenceRootKey+"="+outside+"\n")
		if got, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: candidate, LookupEnv: lookup}); err != nil || got.Path != outside || got.Origin != "conf-local" {
			t.Fatalf("candidate: got %+v, %v", got, err)
		}
	})
	t.Run("candidate with no metasystem.conf beside it", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join(t.TempDir(), "lonely")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		candidate := filepath.Join(dir, "candidate.conf")
		putFile(t, candidate, "x=1\n")
		if got, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: candidate, LookupEnv: lookup}); err != nil || got.Path != filepath.Join(home, "metasystem-evidence", "lonely") {
			t.Fatalf("got %+v, %v", got, err)
		}
	})
}

func TestEvidenceRootLine(t *testing.T) {
	t.Parallel()
	for origin, want := range map[string]string{
		"default":    "evidence root: /e (default; set evidence.root in metasystem.conf.local to change)",
		"conf-local": "evidence root: /e (metasystem.conf.local)",
		"env":        "evidence root: /e (METASYSTEM_EVIDENCE_ROOT)",
		"conf":       "evidence root: /e (metasystem.conf)",
	} {
		if got := (EvidenceRoot{Path: "/e", Origin: origin}).Line(); got != want {
			t.Errorf("%s: got %q; want %q", origin, got, want)
		}
	}
}

// ResolvePath resolves existing ancestors when the leaf is absent, so a
// missing leaf under a linked parent names the link's target (ERD-02).
func TestResolvePathThroughALinkedParent(t *testing.T) {
	t.Parallel()
	bed, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(bed, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(bed, "alias")); err != nil {
		t.Fatal(err)
	}
	if got := realpath.Resolve(filepath.Join(bed, "alias", "new")); got != filepath.Join(real, "new") {
		t.Fatalf("got %s", got)
	}
}

// A conf file that is not there names no root: it is unspecified, as an
// absent key is, so an unset root never refuses (decision 5); a conf that is
// there but unreadable is still an error.
func TestResolveEvidenceRootTreatsAMissingConfAsUnspecified(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dir := filepath.Join(t.TempDir(), "bare-seat")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: filepath.Join(dir, "metasystem.conf"), LookupEnv: envMap(map[string]string{"HOME": home})})
	if err != nil || got.Path != filepath.Join(home, "metasystem-evidence", "bare-seat") || !got.Default() {
		t.Fatalf("got %+v, %v", got, err)
	}
	if err := os.Mkdir(filepath.Join(dir, "metasystem.conf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: filepath.Join(dir, "metasystem.conf"), LookupEnv: envMap(map[string]string{"HOME": home})}); err == nil {
		t.Fatal("an unreadable conf was accepted")
	}
}

// SOL-ER-01: the default is judged like a named root. A HOME that is the
// checkout (or lies inside it) would put the default inside the repository.
func TestResolveEvidenceRootRefusesADefaultInsideTheCheckout(t *testing.T) {
	t.Parallel()
	f := newEvidenceFixture(t, evidencePlaceholder, "")
	_, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: f.conf, LookupEnv: envMap(map[string]string{"HOME": f.checkout})})
	if err == nil || !strings.Contains(err.Error(), "must be outside the repository") {
		t.Fatalf("a default under a HOME that is the checkout = %v, want the outside-the-repository refusal", err)
	}
}

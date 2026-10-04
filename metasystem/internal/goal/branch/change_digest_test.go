package branch

import (
	"bytes"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestChangePatchNormalization(t *testing.T) {
	t.Parallel()
	patch := "diff --git a/metasystem/source.go b/metasystem/source.go\n" +
		"old mode 100644\nnew mode 100755\nindex aaaa..bbbb 100644\n" +
		"--- a/metasystem/source.go\n+++ b/metasystem/source.go\n" +
		"@@ -10,3 +20,3 @@ old function\n context\n-old\n+new\n\\ No newline at end of file\n"
	binary := "diff --git a/metasystem/data.bin b/metasystem/data.bin\nindex aaaa..bbbb 100644\n" +
		"GIT binary patch\nliteral 3\nKcmZQzU|?Vb0000\n\ndelta 2\nJcmZQz00000\n\n"
	generated := "diff --git a/metasystem/generated/out.txt b/metasystem/generated/out.txt\n" +
		"new file mode 100644\nindex 0000..bbbb\n--- /dev/null\n+++ b/metasystem/generated/out.txt\n@@ -0,0 +1 @@\n+generated\n"
	quoted := `diff --git "a/metasystem/generated/sp\040ace\t\303\251.txt" "b/metasystem/generated/sp\040ace\t\303\251.txt"` + "\nnew file mode 100644\n"
	isGenerated := func(path string) bool {
		return conflict.GeneratedBy([]testpolicy.Generated{{Paths: []string{"generated/**"}}}, "metasystem/", path)
	}
	normalize := func(p string) []byte {
		t.Helper()
		got, err := normalizeChangePatch([]byte(p), isGenerated)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	want := strings.ReplaceAll(strings.ReplaceAll(patch, "index aaaa..bbbb 100644\n", ""), "@@ -10,3 +20,3 @@ old function\n", "@@\n")
	if got := normalize(patch); string(got) != want {
		t.Fatalf("normalized patch = %q, want %q", got, want)
	}
	for _, test := range []struct {
		name, changed string
		equal         bool
	}{
		{"base blobs", strings.ReplaceAll(patch, "aaaa..bbbb", "cccc..dddd"), true},
		{"hunk position and function", strings.ReplaceAll(patch, "@@ -10,3 +20,3 @@ old function", "@@ -100,3 +200,3 @@ another function"), true},
		{"context", strings.ReplaceAll(patch, " context\n", " changed context\n"), false},
		{"added whitespace", strings.ReplaceAll(patch, "+new\n", "+new \n"), false},
		{"removed whitespace", strings.ReplaceAll(patch, "-old\n", "-old\t\n"), false},
		{"context whitespace", strings.ReplaceAll(patch, " context\n", "  context\n"), false},
		{"mode", strings.ReplaceAll(patch, "new mode 100755", "new mode 100644"), false},
		{"no newline", strings.ReplaceAll(patch, "\\ No newline at end of file\n", ""), false},
		{"generated content", patch + generated, true},
		{"generated quoted path", patch + quoted, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if equal := digestRawEntries(normalize(patch)) == digestRawEntries(normalize(test.changed)); equal != test.equal {
				t.Fatalf("hash equality = %t, want %t", equal, test.equal)
			}
		})
	}
	if got := normalize(generated + quoted); len(got) != 0 || digestRawEntries(got) != policyHash(nil) {
		t.Fatalf("all generated = %q", got)
	}
	outside := strings.ReplaceAll(generated, "metasystem/generated/", "generated/")
	if got := normalize(outside); len(got) == 0 {
		t.Fatal("file outside the installation was dropped")
	}
	wantBinary := strings.ReplaceAll(binary, "index aaaa..bbbb 100644\n", "")
	if got := normalize(binary); string(got) != wantBinary {
		t.Fatalf("binary bytes = %q, want %q", got, wantBinary)
	}
	for _, changed := range []string{
		strings.ReplaceAll(binary, "literal 3", "literal 4"),
		strings.ReplaceAll(binary, "delta 2", "delta 3"),
		strings.ReplaceAll(binary, "KcmZQzU|?Vb0000", "KcmZQzU|?Vb0001"),
		strings.ReplaceAll(binary, "\n\n", "\n"),
	} {
		if bytes.Equal(normalize(binary), normalize(changed)) {
			t.Fatal("different binary patches normalized equally")
		}
	}
	mode := "diff --git a/metasystem/mode b/metasystem/mode\nold mode 100644\nnew mode 100755\n"
	if string(normalize(mode)) != mode {
		t.Fatal("mode-only change was altered")
	}
}

func TestChangeDigestReadsGeneratedPolicyAtCommit(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, policy string
		absent, bad  bool
	}{
		{name: "absent", absent: true},
		{name: "unknown keys", policy: `{"future":{"anything":true},"generated":[{"paths":["generated/**"]}]}`},
		{name: "invalid JSON", policy: `{"generated":`, bad: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
			f.patches[f.unit] = []byte("diff --git a/metasystem/generated/out b/metasystem/generated/out\nnew file mode 100644\n")
			f.expect("ChangePatch", f.root, f.unit)
			f.expect("Prefix", f.root)
			f.expect("TreeEntry", f.root, f.unit, "testing.json")
			if !test.absent {
				f.treeEntries[f.unit+":testing.json"] = "100644 blob " + policyID("1")
				f.snapshots[f.unit] = map[string][]byte{"testing.json": []byte(test.policy)}
				f.expect("SnapshotFile", f.root, f.unit, "testing.json")
			}
			// This fixture's repository root is also its installation root.
			policy := strings.ReplaceAll(test.policy, "generated/**", "metasystem/generated/**")
			if !test.absent {
				f.snapshots[f.unit]["testing.json"] = []byte(policy)
			}
			got, err := changeDigestWithReads(f, f.root, f.unit)
			if (err != nil) != test.bad {
				t.Fatalf("change = %q, error = %v", got, err)
			}
			if !test.bad && ((got == policyHash(nil)) == test.absent) {
				t.Fatalf("change = %q, absent = %t", got, test.absent)
			}
		})
	}
}

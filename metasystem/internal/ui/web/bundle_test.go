package web

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// sourceTree is the npm project this package's bundle is built from, read
// relative to the package directory, so the Go tests need no Node.
const sourceTree = "_app"

const rebuild = "run npm ci --ignore-scripts && npm run bundle in internal/ui/web/_app"

func TestReadManifestAbsent(t *testing.T) {
	t.Parallel()

	_, err := readManifest(fstest.MapFS{"bundle/README.txt": {Data: []byte("no bundle here\n")}})
	testutil.Expect(t, "absent manifest", errors.Is(err, ErrNoManifest), true)
}

func TestReadManifestUnparsable(t *testing.T) {
	t.Parallel()

	_, err := readManifest(fstest.MapFS{ManifestPath: {Data: []byte("{\"schemaVersion\": 1,\n")}})
	testutil.Require(t, "unparsable manifest refuses", err != nil, true)
	testutil.Expect(t, "not reported as absent", errors.Is(err, ErrNoManifest), false)
	testutil.Expect(t, "refusal names the manifest", strings.Contains(err.Error(), "unparsable"), true)
}

func TestReadManifestReadsTheCanonicalForm(t *testing.T) {
	t.Parallel()

	const written = `{
  "schemaVersion": 1,
  "sourceDigest": "sha256:aa",
  "source": [
    { "path": ".nvmrc", "sha256": "bb" }
  ],
  "files": [
    { "path": "index.html", "sha256": "cc" }
  ],
  "tools": { "node": "v24.21.0", "vite": "8.3.0", "typescript": "7.0.2", "tailwindcss": "4.3.3" }
}
`
	manifest, err := readManifest(fstest.MapFS{ManifestPath: {Data: []byte(written)}})
	testutil.Require(t, "manifest read", err, nil)
	testutil.Expect(t, "schema version", manifest.SchemaVersion, SchemaVersion)
	testutil.Expect(t, "source digest", manifest.SourceDigest, "sha256:aa")
	testutil.Expect(t, "source", manifest.Source, []FileDigest{{Path: ".nvmrc", SHA256: "bb"}})
	testutil.Expect(t, "files", manifest.Files, []FileDigest{{Path: "index.html", SHA256: "cc"}})
	testutil.Expect(t, "tools", manifest.Tools, map[string]string{"node": "v24.21.0", "vite": "8.3.0", "typescript": "7.0.2", "tailwindcss": "4.3.3"})
}

// TestBundleMatchesManifest is the one test that refuses an executable without
// a bundle: the manifest is the publish point, so its absence is a build that
// never finished, and every other bundle test stands down behind this one.
func TestBundleMatchesManifest(t *testing.T) {
	t.Parallel()

	manifest, err := ReadManifest()
	if err != nil {
		t.Fatalf("bundle missing: %v; %s", err, rebuild)
	}
	testutil.Expect(t, "schema version", manifest.SchemaVersion, SchemaVersion)
	testutil.Expect(t, "files are sorted by path", slices.IsSortedFunc(manifest.Files, func(a, b FileDigest) int { return strings.Compare(a.Path, b.Path) }), true)
	testutil.Expect(t, "source is sorted by path", slices.IsSortedFunc(manifest.Source, func(a, b FileDigest) int { return strings.Compare(a.Path, b.Path) }), true)

	recorded := make(map[string]string, len(manifest.Files))
	for _, file := range manifest.Files {
		recorded[file.Path] = file.SHA256
	}
	var missing, extra, differing []string
	served := servedFiles(t)
	for path, digest := range served {
		switch want, known := recorded[path]; {
		case !known:
			extra = append(extra, path)
		case want != digest:
			differing = append(differing, path)
		}
	}
	for path := range recorded {
		if _, known := served[path]; !known {
			missing = append(missing, path)
		}
	}
	slices.Sort(missing)
	slices.Sort(extra)
	slices.Sort(differing)
	testutil.Expect(t, "files the manifest records but the bundle does not carry", missing, []string(nil))
	testutil.Expect(t, "files the bundle carries but the manifest does not record", extra, []string(nil))
	testutil.Expect(t, "files whose digest differs from the manifest", differing, []string(nil))
}

// TestBundleIsCurrent proves that the committed bundle was built from the
// committed source: a change to either without the other is a stale bundle,
// and nobody can see that by reading a diff.
func TestBundleIsCurrent(t *testing.T) {
	t.Parallel()

	if _, err := os.Stat(sourceTree); errors.Is(err, fs.ErrNotExist) {
		t.Skip("this checkout carries no interface source tree")
	}
	manifest, ok := builtBundle(t)
	if !ok {
		return
	}
	digest, files, err := SourceDigest(sourceTree)
	testutil.Require(t, "source digest", err, nil)
	if digest == manifest.SourceDigest {
		return
	}
	recorded := make(map[string]string, len(manifest.Source))
	for _, file := range manifest.Source {
		recorded[file.Path] = file.SHA256
	}
	var changed, added, removed []string
	for _, file := range files {
		switch want, known := recorded[file.Path]; {
		case !known:
			added = append(added, file.Path)
		case want != file.SHA256:
			changed = append(changed, file.Path)
		}
		delete(recorded, file.Path)
	}
	for path := range recorded {
		removed = append(removed, path)
	}
	slices.Sort(removed)
	t.Fatalf("the interface bundle is stale: changed %v, added %v, removed %v; %s", changed, added, removed, rebuild)
}

func TestDistIsServable(t *testing.T) {
	t.Parallel()

	if _, ok := builtBundle(t); !ok {
		return
	}
	for path := range servedFiles(t) {
		testutil.Expect(t, "a valid path: "+path, fs.ValidPath(path), true)
		_, known := ContentType(path)
		testutil.Expect(t, "a known content type: "+path, known, true)
	}
}

// TestIndexIsStrict holds the page to what the policy allows: one nonce, no
// inline script or style, and no source outside this origin.
func TestIndexIsStrict(t *testing.T) {
	t.Parallel()

	if _, ok := builtBundle(t); !ok {
		return
	}
	page, err := fs.ReadFile(Dist(), "index.html")
	testutil.Require(t, "read the page", err, nil)
	index := string(page)
	lowered := strings.ToLower(index)
	testutil.Expect(t, "nonce placeholders", strings.Count(index, NoncePlaceholder), 1)
	testutil.Expect(t, "inline <style>", strings.Contains(lowered, "<style"), false)
	testutil.Expect(t, "a style attribute", strings.Contains(lowered, "style="), false)
	testutil.Expect(t, "every <script> names a source", everyScriptNamesASource(lowered), true)
	for _, attribute := range []string{"src=\"http", "src='http", "href=\"http", "href='http"} {
		testutil.Expect(t, "an external "+attribute, strings.Contains(lowered, attribute), false)
	}
}

// everyScriptNamesASource reports whether every <script> in the page carries a
// src; one without it is inline, which the policy refuses.
func everyScriptNamesASource(lowered string) bool {
	for _, after := range strings.Split(lowered, "<script")[1:] {
		tag, _, found := strings.Cut(after, ">")
		if !found || !strings.Contains(tag, "src=") {
			return false
		}
	}
	return true
}

// builtBundle reports the manifest of a built bundle, or false when this
// executable carries none; TestBundleMatchesManifest is what refuses that.
func builtBundle(t *testing.T) (Manifest, bool) {
	t.Helper()
	manifest, err := ReadManifest()
	if err != nil {
		t.Skipf("this executable carries no interface bundle: %v; %s", err, rebuild)
		return Manifest{}, false
	}
	return manifest, true
}

// servedFiles digests every file the server would hand out, over the bytes as
// served: the manifest records these unnormalised, because that is what a
// browser receives.
func servedFiles(t *testing.T) map[string]string {
	t.Helper()
	files := make(map[string]string)
	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := fs.ReadFile(Dist(), path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(content)
		files[path] = hex.EncodeToString(sum[:])
		return nil
	}
	testutil.Require(t, "walk the served bundle", fs.WalkDir(Dist(), ".", walk), nil)
	return files
}

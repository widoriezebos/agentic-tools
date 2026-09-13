package narratordigest

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func rawPayloadBody(t *testing.T, data []byte, source string) []byte {
	t.Helper()
	marker := []byte(" — RAW-PAYLOAD (source: " + source + ") bytes=")
	start := bytes.Index(data, marker)
	if start < 0 {
		t.Fatalf("raw payload source marker %q is absent from %q", source, data)
	}
	headerEnd := bytes.IndexByte(data[start:], '\n')
	if headerEnd < 0 {
		t.Fatalf("raw payload header has no newline: %q", data[start:])
	}
	headerEnd += start
	sizeStart := start + len(marker)
	size, err := strconv.Atoi(string(data[sizeStart:headerEnd]))
	if err != nil || headerEnd+1+size > len(data) {
		t.Fatalf("raw payload header has an invalid byte count: %q", data[start:headerEnd])
	}
	return data[headerEnd+1 : headerEnd+1+size]
}

func TestRawPayloadPreservesRendererBytesAndDeduplicatesItsSource(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	rendered := []byte("Counselor heading.\n\nWarning:  spaces stay.\n")
	payload := Payload{Kind: "lowlight", Body: rendered, SourceType: "counselor-brief", SourceID: "period-1"}
	if err := AppendPayload(root, payload, now); err != nil {
		t.Fatal(err)
	}
	retry := payload
	retry.Body = []byte("a retry rendered different bytes\n")
	if err := AppendPayload(root, retry, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(Path(root))
	if err != nil {
		t.Fatal(err)
	}
	if got := rawPayloadBody(t, data, "counselor-brief period-1"); !bytes.Equal(got, rendered) {
		t.Fatalf("digest softened the rendered bytes:\n got %q\nwant %q", got, rendered)
	}
	if count := bytes.Count(data, []byte("RAW-PAYLOAD")); count != 1 {
		t.Fatalf("one source produced %d payload frames", count)
	}
	pending, err := Pending(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(pending.Message, string(rendered)) {
		t.Fatalf("pending delivery changed the payload suffix: %q", pending.Message)
	}
}

func TestPendingCursorLeavesEventsAppendedDuringDelivery(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	first := Entry{Kind: "highlight", Text: "The first landing shipped.", SourceType: "commit", SourceID: "abc"}
	if err := Append(root, []Entry{first, first}, now); err != nil {
		t.Fatal(err)
	}
	pending, err := Pending(root)
	if err != nil || strings.Count(pending.Message, "The first landing shipped") != 1 {
		t.Fatalf("first pending digest was not deduplicated: %+v %v", pending, err)
	}
	if err := Append(root, []Entry{{
		Kind: "lowlight", Text: "A later check found a breach.", SourceType: "episode", SourceID: "stop-2",
	}}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := Advance(root, pending.Cursor, pending.PrefixSHA256); err != nil {
		t.Fatal(err)
	}
	next, err := Pending(root)
	if err != nil || strings.Contains(next.Message, "first landing") || !strings.Contains(next.Message, "later check") {
		t.Fatalf("cursor did not preserve the event appended during delivery: %+v %v", next, err)
	}
}

func TestPendingRecoversFromARewrittenStory(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	if err := Append(root, []Entry{{
		Kind: "highlight", Text: "The landing shipped.", SourceType: "commit", SourceID: "abc",
	}}, now); err != nil {
		t.Fatal(err)
	}
	pending, err := Pending(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := Advance(root, pending.Cursor, pending.PrefixSHA256); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(Path(root))
	if err != nil {
		t.Fatal(err)
	}
	data[0] = '9'
	if err := os.WriteFile(Path(root), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// The story was rewritten under the cursor (a sync or a checkout of the
	// tracked log): the reader is shown the recent tail, never nothing, and
	// the cursor parks at the new end.
	rewritten, err := Pending(root)
	if err != nil || !strings.HasPrefix(rewritten.Message, "NARRATOR DIGEST, the story was rewritten since the last check-in") ||
		!strings.Contains(rewritten.Message, "The landing shipped.") || rewritten.Cursor != int64(len(data)) {
		t.Fatalf("a rewritten story did not recover with its tail: %+v %v", rewritten, err)
	}
	// A rewrite shows one line, never a page: a sync rewrites this file often.
	if lines := strings.Count(strings.TrimSuffix(rewritten.Message, "\n"), "\n"); lines != 1 {
		t.Fatalf("a rewritten story showed %d body lines, not one: %q", lines, rewritten.Message)
	}
	if err := Advance(root, rewritten.Cursor, rewritten.PrefixSHA256); err != nil {
		t.Fatalf("advance after a rewrite: %v", err)
	}
	if again, err := Pending(root); err != nil || again.Message != "" {
		t.Fatalf("the check-in after a rewrite was not incremental: %+v %v", again, err)
	}
	// The story shrank below the cursor (a checkout to the committed log):
	// the same recovery, and the advance to a smaller cursor is accepted.
	if err := os.WriteFile(Path(root), data[:len(data)/2], 0o644); err != nil {
		t.Fatal(err)
	}
	shrunk, err := Pending(root)
	if err != nil || shrunk.Cursor != int64(len(data)/2) || !strings.HasPrefix(shrunk.Message, "NARRATOR DIGEST, the story was rewritten") {
		t.Fatalf("a shrunk story did not recover: %+v %v", shrunk, err)
	}
	if err := Advance(root, shrunk.Cursor, shrunk.PrefixSHA256); err != nil {
		t.Fatalf("advance to the shrunk end: %v", err)
	}
	// A fresh cursor still refuses a valid pair that points backwards, and
	// a negative cursor is refused, never sliced.
	half := data[:len(data)/2]
	quarter := half[:len(half)/2]
	if err := Advance(root, int64(len(quarter)), digest(quarter)); err == nil {
		t.Fatal("a backward advance against a fresh cursor was accepted")
	}
	if err := Advance(root, -1, digest(nil)); err == nil {
		t.Fatal("a negative cursor was accepted")
	}
}

func TestAdvanceRefusesCursorEdgesThatWereNotEmitted(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	if err := Append(root, []Entry{{
		Kind: "lowlight", Text: "The check found a breach.", SourceType: "episode", SourceID: "stop-1",
	}}, now); err != nil {
		t.Fatal(err)
	}
	pending, err := Pending(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := Advance(root, pending.Cursor, pending.PrefixSHA256); err != nil {
		t.Fatal(err)
	}

	for name, cursor := range map[string]struct {
		cursor int64
		prefix string
	}{
		"backward cursor": {pending.Cursor - 1, pending.PrefixSHA256},
		"past log edge":   {pending.Cursor + 1, pending.PrefixSHA256},
		"wrong prefix":    {pending.Cursor, strings.Repeat("0", 64)},
	} {
		t.Run(name, func(t *testing.T) {
			if err := Advance(root, cursor.cursor, cursor.prefix); err == nil || !strings.Contains(err.Error(), "does not name the emitted prefix") {
				t.Fatalf("unemitted cursor edge was accepted: %v", err)
			}
		})
	}
}

func TestNamedBrainCursorLeavesHumanCursorUntouched(t *testing.T) {
	root := t.TempDir()
	if err := Append(root, []Entry{{
		Kind: "highlight", Text: "The brain sees this line.", SourceType: "fixture", SourceID: "brain",
	}}, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	brainPending, err := Pending(root, "brain")
	if err != nil {
		t.Fatal(err)
	}
	if err := Advance(root, brainPending.Cursor, brainPending.PrefixSHA256, "brain"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(CursorPath(root, "brain")); err != nil {
		t.Fatalf("brain cursor absent: %v", err)
	}
	if _, err := os.Stat(CursorPath(root)); !os.IsNotExist(err) {
		t.Fatalf("brain advance changed the human cursor: %v", err)
	}
	humanPending, err := Pending(root)
	if err != nil || !strings.Contains(humanPending.Message, "brain sees this line") {
		t.Fatalf("human default cursor did not retain its independent pending log: %+v %v", humanPending, err)
	}
}

// A template checkout keeps the installation in its metasystem directory. A
// caller that names the git toplevel (the Stop hook's repository scope, the
// steward armed with --repo <checkout>) must reach the same digest, cursor
// and lock as one that names the installation, or the steward writes one
// digest and the hook reads another.
func TestTemplateToplevelResolvesToTheInstallationDigest(t *testing.T) {
	toplevel := t.TempDir()
	installation := filepath.Join(toplevel, "metasystem")
	for _, directory := range []string{filepath.Join(toplevel, "development"), filepath.Join(installation, "scripts", "agents")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(toplevel, "development", "metasystem-design.md"), []byte("# template\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", toplevel, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if Path(toplevel) != Path(installation) || CursorPath(toplevel) != CursorPath(installation) {
		t.Fatalf("toplevel and installation name different digest files: %q vs %q", Path(toplevel), Path(installation))
	}
	resolvedInstallation, err := filepath.EvalSymlinks(installation)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(Path(toplevel), resolvedInstallation+string(filepath.Separator)) {
		t.Fatalf("the digest does not live under the installation: %q", Path(toplevel))
	}
	now := time.Date(2026, 9, 13, 8, 0, 0, 0, time.UTC)
	if err := Append(toplevel, []Entry{{Kind: "highlight", Text: "A landing moved the repository storyline to commit abc123.", SourceType: "commit", SourceID: "abc123"}}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(toplevel, "records", "narrator-digest.log")); !os.IsNotExist(err) {
		t.Fatalf("a digest was written beside the repository root: %v", err)
	}
	pending, err := Pending(installation)
	if err != nil || !strings.Contains(pending.Message, "commit abc123") {
		t.Fatalf("the installation did not see the toplevel caller's entry: %+v %v", pending, err)
	}
	if err := Advance(toplevel, pending.Cursor, pending.PrefixSHA256); err != nil {
		t.Fatalf("advance through the toplevel: %v", err)
	}
	if after, err := Pending(installation); err != nil || after.Message != "" {
		t.Fatalf("the cursor advanced through the toplevel was not the installation's: %+v %v", after, err)
	}

	// An adopted installation (no template marker) is its own root.
	adopted := t.TempDir()
	if err := os.MkdirAll(filepath.Join(adopted, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adopted, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", adopted, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init adopted: %v: %s", err, output)
	}
	resolvedAdopted, err := filepath.EvalSymlinks(adopted)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(Path(adopted), resolvedAdopted+string(filepath.Separator)) && !strings.HasPrefix(Path(adopted), adopted+string(filepath.Separator)) {
		t.Fatalf("an adopted installation's digest moved: %q", Path(adopted))
	}
}

// A plain directory that is neither an installation nor inside a repository
// keeps every existing caller working: it resolves to itself.
func TestBareDirectoryResolvesToItself(t *testing.T) {
	bare := t.TempDir()
	resolved, err := filepath.EvalSymlinks(bare)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(Path(bare), resolved+string(filepath.Separator)) && !strings.HasPrefix(Path(bare), bare+string(filepath.Separator)) {
		t.Fatalf("a bare directory's digest moved: %q", Path(bare))
	}
}

// A reader with no cursor is shown the recent tail, never the whole story,
// and its cursor then stands at the end; a short story is shown whole.
func TestFirstCheckInShowsTheRecentTailAndParksTheCursorAtTheEnd(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 13, 8, 0, 0, 0, time.UTC)
	var entries []Entry
	for i := 1; i <= 60; i++ {
		entries = append(entries, Entry{Kind: "highlight", Text: "line " + strconv.Itoa(i), SourceType: "fixture", SourceID: strconv.Itoa(i)})
	}
	if err := Append(root, entries, now); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(Path(root))
	if err != nil {
		t.Fatal(err)
	}
	pending, err := Pending(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(pending.Message, "line 20 ") || !strings.Contains(pending.Message, "line 21 ") || !strings.Contains(pending.Message, "line 60 ") ||
		!strings.HasPrefix(pending.Message, "NARRATOR DIGEST, first check-in (the last 40 of 60 lines") {
		t.Fatalf("first check-in did not show the last forty lines: %q", pending.Message[:200])
	}
	if pending.Cursor != int64(len(data)) {
		t.Fatalf("first check-in cursor %d is not the end of the story %d", pending.Cursor, len(data))
	}
	if err := Advance(root, pending.Cursor, pending.PrefixSHA256); err != nil {
		t.Fatalf("advance after the first check-in: %v", err)
	}
	if again, err := Pending(root); err != nil || again.Message != "" {
		t.Fatalf("the second check-in was not incremental: %+v %v", again, err)
	}
	short := t.TempDir()
	if err := Append(short, entries[:3], now); err != nil {
		t.Fatal(err)
	}
	if brief, err := Pending(short); err != nil || !strings.HasPrefix(brief.Message, "NARRATOR DIGEST since last check-in:\n") || !strings.Contains(brief.Message, "line 1 ") {
		t.Fatalf("a short story was not shown whole: %+v %v", brief, err)
	}
}

// A vendored installation inside another repository keeps its digest and
// its cursor under itself; nothing is written at the Git repository scope.
func TestVendoredInstallationKeepsItsOwnDigest(t *testing.T) {
	scope := t.TempDir()
	vendored := filepath.Join(scope, "vendor", "metasystem")
	if err := os.MkdirAll(filepath.Join(vendored, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendored, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", scope, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	now := time.Date(2026, 9, 13, 9, 0, 0, 0, time.UTC)
	if err := Append(vendored, []Entry{{Kind: "highlight", Text: "vendored line", SourceType: "fixture", SourceID: "v"}}, now); err != nil {
		t.Fatal(err)
	}
	pending, err := Pending(vendored)
	if err != nil || !strings.Contains(pending.Message, "vendored line") {
		t.Fatalf("the vendored installation did not read its own digest: %+v %v", pending, err)
	}
	if err := Advance(vendored, pending.Cursor, pending.PrefixSHA256); err != nil {
		t.Fatal(err)
	}
	for _, stray := range []string{filepath.Join(scope, "records"), filepath.Join(scope, "artifacts")} {
		if _, err := os.Stat(stray); !os.IsNotExist(err) {
			t.Fatalf("control state appeared at the repository scope: %s (%v)", stray, err)
		}
	}
	resolved, _ := filepath.EvalSymlinks(vendored)
	if !strings.HasPrefix(Path(vendored), resolved+string(filepath.Separator)) && !strings.HasPrefix(Path(vendored), vendored+string(filepath.Separator)) {
		t.Fatalf("the vendored digest moved: %q", Path(vendored))
	}
}

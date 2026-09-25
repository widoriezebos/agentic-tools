package contractgit

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
)

const (
	stubBlob    = "0123456789abcdef0123456789abcdef01234567"
	overlayTree = "overlay-tree"
)

var errStubAccess = errors.New("stub access refused")

// stubAccess answers CommitAccess from fixed facts so the transition policy is
// judged without Git. failAt names the one method that returns errStubAccess.
type stubAccess struct {
	paths      string
	attributes map[string]string
	entry      string
	files      map[string][]byte
	failAt     string

	mu        sync.Mutex
	calls     []string
	onlyPaths []string
	set       []string
	dropped   []string
	cleanups  int
}

func (s *stubAccess) record(call string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, call)
	if s.failAt == call {
		return errStubAccess
	}
	return nil
}

func (s *stubAccess) called(call string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Contains(s.calls, call)
}

func (s *stubAccess) Paths(_, _, _, onlyPath string) ([]byte, error) {
	s.onlyPaths = append(s.onlyPaths, onlyPath)
	return []byte(s.paths), s.record("Paths")
}
func (s *stubAccess) Attributes(_, tree string, _ []string) ([]byte, error) {
	return []byte(s.attributes[tree]), s.record("Attributes")
}
func (s *stubAccess) OpenAttributeIndex() (string, func(), error) {
	if err := s.record("OpenAttributeIndex"); err != nil {
		return "", nil, err
	}
	return "stub-index", func() { s.mu.Lock(); s.cleanups++; s.mu.Unlock() }, nil
}
func (s *stubAccess) LoadAttributeBase(_, _, _ string) error { return s.record("LoadAttributeBase") }
func (s *stubAccess) AttributeEntry(_, _, _, _ string) ([]byte, error) {
	return []byte(s.entry), s.record("AttributeEntry")
}
func (s *stubAccess) DropAttribute(_, _, path string) error {
	s.dropped = append(s.dropped, path)
	return s.record("DropAttribute")
}
func (s *stubAccess) SetAttribute(_, _, mode, blob, path string) error {
	s.set = append(s.set, mode+","+blob+","+path)
	return s.record("SetAttribute")
}
func (s *stubAccess) AttributeTree(_, _ string) (string, error) {
	return overlayTree, s.record("AttributeTree")
}
func (s *stubAccess) File(_, tree, path string) ([]byte, error) {
	if path != TestingContractPath {
		return nil, errors.New("unexpected contract path " + path)
	}
	return s.files[tree], s.record("File")
}

func attrRecord(path, value string) string { return path + "\x00merge\x00" + value + "\x00" }

func TestPreflightCommitAttributesWithRefusesDriverIntroducedByCommitAttributes(t *testing.T) {
	t.Parallel()
	a := &stubAccess{
		paths: "notes.txt\x00.gitattributes\x00",
		attributes: map[string]string{
			"before":    attrRecord("notes.txt", "unspecified") + attrRecord(".gitattributes", "unspecified"),
			overlayTree: attrRecord("notes.txt", testingContractMergeDriver) + attrRecord(".gitattributes", "unspecified"),
		},
		entry: "100644 blob " + stubBlob + "\t.gitattributes\x00",
	}
	err := PreflightCommitAttributesWith("repo", "before", "commit", a)
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Code != AttributeMisuseCode || !strings.Contains(refusal.Detail, "notes.txt") || !strings.Contains(refusal.Detail, "commit") {
		t.Fatalf("overlay misuse err=%v", err)
	}
	if want := []string{"100644," + stubBlob + ",.gitattributes"}; !slices.Equal(a.set, want) {
		t.Fatalf("overlay staged %v, want %v", a.set, want)
	}
	if a.cleanups != 1 {
		t.Fatalf("overlay index cleanups=%d, want 1", a.cleanups)
	}
}

func TestPreflightCommitAttributesWithRefusesCurrentTreeMisuseBeforeOverlay(t *testing.T) {
	t.Parallel()
	a := &stubAccess{
		paths:      "src/data.json\x00",
		attributes: map[string]string{"before": attrRecord("src/data.json", testingContractMergeDriver)},
	}
	if err := PreflightCommitAttributesWith("repo", "before", "commit", a); !IsRefusal(err) || !strings.Contains(err.Error(), AttributeMisuseCode) {
		t.Fatalf("current-tree misuse err=%v", err)
	}
	if a.called("OpenAttributeIndex") {
		t.Fatal("overlay index opened after the current tree already refused")
	}
}

func TestPreflightCommitAttributesWithAcceptsContractDriverAndRemovedAttributes(t *testing.T) {
	t.Parallel()
	clean := attrRecord(TestingContractPath, testingContractMergeDriver) + attrRecord("docs/.gitattributes", "unspecified")
	a := &stubAccess{
		paths:      TestingContractPath + "\x00docs/.gitattributes\x00",
		attributes: map[string]string{"before": clean, overlayTree: clean},
	}
	if err := PreflightCommitAttributesWith("repo", "before", "commit", a); err != nil {
		t.Fatalf("contract path keeping its own driver was refused: %v", err)
	}
	if !slices.Equal(a.dropped, []string{"docs/.gitattributes"}) || len(a.set) != 0 || a.cleanups != 1 {
		t.Fatalf("removed attributes: dropped=%v set=%v cleanups=%d", a.dropped, a.set, a.cleanups)
	}
}

func TestPreflightCommitAttributesWithSkipsCommitWithoutPaths(t *testing.T) {
	t.Parallel()
	a := &stubAccess{}
	if err := PreflightCommitAttributesWith("repo", "before", "commit", a); err != nil {
		t.Fatal(err)
	}
	if a.called("Attributes") || a.called("OpenAttributeIndex") {
		t.Fatalf("empty commit consulted attributes: %v", a.calls)
	}
}

func TestPreflightCommitAttributesWithRefusesMalformedAttributeEntries(t *testing.T) {
	t.Parallel()
	for name, entry := range map[string]string{
		"not a blob":   "040000 tree " + stubBlob + "\t.gitattributes\x00",
		"missing tab":  "100644 blob " + stubBlob + "\x00",
		"octal mode":   "100648 blob " + stubBlob + "\t.gitattributes\x00",
		"short fields": "blob " + stubBlob + "\t.gitattributes\x00",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := &stubAccess{paths: ".gitattributes\x00", entry: entry}
			err := PreflightCommitAttributesWith("repo", "before", "commit", a)
			if err == nil || IsRefusal(err) || !strings.Contains(err.Error(), "malformed") {
				t.Fatalf("entry %q err=%v", entry, err)
			}
			if len(a.set) != 0 || a.cleanups != 1 {
				t.Fatalf("malformed entry staged %v, cleanups=%d", a.set, a.cleanups)
			}
		})
	}
}

func TestPreflightCommitAttributesWithPropagatesAccessFailures(t *testing.T) {
	t.Parallel()
	for _, step := range []struct {
		method  string
		entry   string
		cleanup int
	}{
		{"Paths", "", 0},
		{"Attributes", "", 0},
		{"OpenAttributeIndex", "", 0},
		{"LoadAttributeBase", "", 1},
		{"AttributeEntry", "", 1},
		{"DropAttribute", "", 1},
		{"SetAttribute", "100644 blob " + stubBlob + "\t.gitattributes\x00", 1},
		{"AttributeTree", "", 1},
	} {
		t.Run(step.method, func(t *testing.T) {
			t.Parallel()
			a := &stubAccess{paths: ".gitattributes\x00", entry: step.entry, failAt: step.method}
			if err := PreflightCommitAttributesWith("repo", "before", "commit", a); !errors.Is(err, errStubAccess) {
				t.Fatalf("failure in %s returned %v", step.method, err)
			}
			if a.cleanups != step.cleanup {
				t.Fatalf("failure in %s: cleanups=%d, want %d", step.method, a.cleanups, step.cleanup)
			}
		})
	}
}

func contractFiles(t *testing.T, after []byte) map[string][]byte {
	t.Helper()
	return map[string][]byte{
		"commit^": historyContract(t, "base"),
		"commit":  historyContract(t, "theirs"),
		"before":  historyContract(t, "ours"),
		"after":   after,
	}
}

func TestCheckCommitContractWithAcceptsOnlyTheSemanticMerge(t *testing.T) {
	t.Parallel()
	a := &stubAccess{paths: TestingContractPath + "\x00", files: contractFiles(t, historyContract(t, "want"))}
	if err := CheckCommitContractWith("repo", "before", "after", "commit", "pick", a); err != nil {
		t.Fatalf("semantic merge refused: %v", err)
	}
	if !slices.Equal(a.onlyPaths, []string{TestingContractPath}) {
		t.Fatalf("change detection scoped to %v", a.onlyPaths)
	}

	handMerged := &stubAccess{paths: TestingContractPath + "\x00", files: contractFiles(t, historyContract(t, "ours"))}
	err := CheckCommitContractWith("repo", "before", "after", "commit", "pick abc", handMerged)
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Code != ContractHandMergedCode || !strings.HasPrefix(refusal.Detail, "pick abc differs") {
		t.Fatalf("hand-merged contract err=%v", err)
	}
}

func TestCheckCommitContractWithSkipsUnchangedContract(t *testing.T) {
	t.Parallel()
	a := &stubAccess{}
	if err := CheckCommitContractWith("repo", "before", "after", "commit", "pick", a); err != nil {
		t.Fatal(err)
	}
	if a.called("File") {
		t.Fatal("unchanged contract read contract bytes")
	}
}

func TestCheckCommitContractWithPropagatesAccessFailures(t *testing.T) {
	t.Parallel()
	for _, method := range []string{"Paths", "File"} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			a := &stubAccess{paths: TestingContractPath + "\x00", files: contractFiles(t, nil), failAt: method}
			if err := CheckCommitContractWith("repo", "before", "after", "commit", "pick", a); !errors.Is(err, errStubAccess) {
				t.Fatalf("failure in %s returned %v", method, err)
			}
		})
	}
}

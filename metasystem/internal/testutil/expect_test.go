package testutil

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

const expectSubprocessCase = "METASYSTEM_EXPECT_SUBPROCESS_CASE"

func TestExpectPrintsBlock(t *testing.T) {
	switch os.Getenv(expectSubprocessCase) {
	case "prints-block":
		expectPrintsBlockFixture(t)
		return
	case "prints-match-block":
		expectMatchPrintsBlockFixture(t)
		return
	case "different-integer-types":
		expectDifferentIntegerTypesFixture(t)
		return
	case "string-and-integer":
		expectStringAndIntegerFixture(t)
		return
	case "trailing-newline":
		expectTrailingNewlineFixture(t)
		return
	case "carriage-return":
		expectCarriageReturnFixture(t)
		return
	case "control-bytes":
		expectControlBytesFixture(t)
		return
	case "equal-beyond-cut":
		expectEqualBeyondCutFixture(t)
		return
	case "nan-same-print":
		expectNaNSamePrintFixture(t)
		return
	case "multiline":
		expectMultilineFixture(t)
		return
	case "plain-output":
		expectPlainOutputFixture(t)
		return
	}

	site := fixtureSite(t, expectPrintsBlockFixture, 1)
	want := fmt.Sprintf("FAIL TestExpectPrintsBlock %s:%d status text\n  expected: ready\n  observed: waiting", site.file, site.line)
	assertSubprocessOutput(t, "prints-block", want, site.line)

	matchSite := fixtureSite(t, expectMatchPrintsBlockFixture, 1)
	want = fmt.Sprintf("FAIL TestExpectPrintsBlock %s:%d status pattern\n  expected: ^ready:[0-9]+$\n  observed: waiting", matchSite.file, matchSite.line)
	assertSubprocessOutput(t, "prints-match-block", want, matchSite.line)

	integerSite := fixtureSite(t, expectDifferentIntegerTypesFixture, 1)
	want = fmt.Sprintf("FAIL TestExpectPrintsBlock %s:%d integer types\n  expected: (int) 10\n  observed: (int64) 10", integerSite.file, integerSite.line)
	assertSubprocessOutput(t, "different-integer-types", want, integerSite.line)

	stringSite := fixtureSite(t, expectStringAndIntegerFixture, 1)
	want = fmt.Sprintf("FAIL TestExpectPrintsBlock %s:%d string and integer\n  expected: (int) 3\n  observed: (string) 3", stringSite.file, stringSite.line)
	assertSubprocessOutput(t, "string-and-integer", want, stringSite.line)

	newlineSite := fixtureSite(t, expectTrailingNewlineFixture, 1)
	want = fmt.Sprintf("FAIL TestExpectPrintsBlock %s:%d trailing newline\n  expected: ready\n  observed: \"ready\\n\"", newlineSite.file, newlineSite.line)
	assertSubprocessOutput(t, "trailing-newline", want, newlineSite.line)

	carriageSite := fixtureSite(t, expectCarriageReturnFixture, 1)
	want = fmt.Sprintf("FAIL TestExpectPrintsBlock %s:%d carriage return\n  expected: other\n  observed: \"left\\rright\"", carriageSite.file, carriageSite.line)
	assertSubprocessOutput(t, "carriage-return", want, carriageSite.line)

	escapeSite := fixtureSite(t, expectEscapeByteFixture, 0)
	nulSite := fixtureSite(t, expectNULByteFixture, 0)
	want = fmt.Sprintf("FAIL TestExpectPrintsBlock %s:%d escape byte\n  expected: other\n  observed: \"left\\x1bright\"\n", escapeSite.file, escapeSite.line) +
		fmt.Sprintf("FAIL TestExpectPrintsBlock %s:%d nul byte\n  expected: other\n  observed: \"left\\x00right\"", nulSite.file, nulSite.line)
	assertSubprocessOutput(t, "control-bytes", want, escapeSite.line, nulSite.line)

	equalBeyondCutSite := fixtureSite(t, expectEqualBeyondCutFixture, 1)
	boundedAs := strings.Repeat("a", 2048) + "[... 953 more bytes]"
	want = fmt.Sprintf("FAIL TestExpectPrintsBlock %s:%d equal beyond cut\n", equalBeyondCutSite.file, equalBeyondCutSite.line) +
		"  expected: " + boundedAs + "\n" +
		"  observed: " + boundedAs + " [first difference at byte 3000]"
	assertSubprocessOutput(t, "equal-beyond-cut", want, equalBeyondCutSite.line)

	nanSite := fixtureSite(t, expectNaNSamePrintFixture, 1)
	want = fmt.Sprintf("FAIL TestExpectPrintsBlock %s:%d NaN values\n", nanSite.file, nanSite.line) +
		"  expected: (float64) NaN\n" +
		"  observed: (float64) NaN [the values differ but print the same]"
	assertSubprocessOutput(t, "nan-same-print", want, nanSite.line)

	multilineSite := fixtureSite(t, expectMultilineFixture, 1)
	want = fmt.Sprintf("FAIL TestExpectPrintsBlock %s:%d multiline value\n", multilineSite.file, multilineSite.line) +
		"  expected: ready\n" +
		"  observed: line one\n" +
		"            FAIL TestOther x.go:1 other\n" +
		"            last"
	assertSubprocessOutput(t, "multiline", want, multilineSite.line)

	plainSite := fixtureSite(t, expectPlainOutputFixture, 1)
	want = fmt.Sprintf("FAIL TestExpectPrintsBlock %s:%d plain output\n  expected: ready\n  observed: waiting", plainSite.file, plainSite.line)
	assertPlainSubprocessOutput(t, "plain-output", want, plainSite.line)
}

func TestExpectKeepsGoing(t *testing.T) {
	switch os.Getenv(expectSubprocessCase) {
	case "keeps-going":
		expectKeepsGoingFixture(t)
		return
	case "require-stops":
		requireStopsFixture(t)
		return
	case "require-match-stops":
		requireMatchStopsFixture(t)
		return
	}

	first := fixtureSite(t, expectFirstFailure, 0)
	second := fixtureSite(t, expectSecondFailure, 0)
	want := fmt.Sprintf("FAIL TestExpectKeepsGoing %s:%d first failure\n  expected: expected one\n  observed: observed one\n", first.file, first.line) +
		fmt.Sprintf("FAIL TestExpectKeepsGoing %s:%d second failure\n  expected: []int{3}\n  observed: []int{1, 2}", second.file, second.line)
	assertSubprocessOutput(t, "keeps-going", want, first.line, second.line)

	required := fixtureSite(t, requireFirstFailure, 0)
	want = fmt.Sprintf("FAIL TestExpectKeepsGoing %s:%d required value\n  expected: ready\n  observed: waiting", required.file, required.line)
	assertSubprocessOutput(t, "require-stops", want, required.line)

	requiredMatch := fixtureSite(t, requireFirstMatchFailure, 1)
	want = fmt.Sprintf("FAIL TestExpectKeepsGoing %s:%d required pattern\n  expected: ^ready:[0-9]+$\n  observed: waiting", requiredMatch.file, requiredMatch.line)
	assertSubprocessOutput(t, "require-match-stops", want, requiredMatch.line)
}

func TestExpectDuplicateLabel(t *testing.T) {
	switch os.Getenv(expectSubprocessCase) {
	case "duplicate-label":
		expectDuplicateLabelFixture(t)
		return
	case "require-duplicate-label":
		requireDuplicateLabelFixture(t)
		return
	case "escaped-duplicate-label":
		expectEscapedDuplicateLabelFixture(t)
		return
	}
	for _, name := range []string{"first subtest", "second subtest"} {
		var finishedSubtest *testing.T
		t.Run(name, func(t *testing.T) {
			finishedSubtest = t
			Expect(t, "label reused across subtests", "same", "same")
		})
		labels.Lock()
		_, retained := labels.byTest[finishedSubtest]
		labels.Unlock()
		if retained {
			t.Errorf("label record for finished subtest %q: expected released, observed retained", name)
		}
	}

	first := fixtureSite(t, expectFirstLabelUse, 0)
	duplicate := fixtureSite(t, expectDuplicateLabelUse, 1)
	want := fmt.Sprintf("FAIL TestExpectDuplicateLabel %s:%d duplicate label repeated label\n", duplicate.file, duplicate.line) +
		fmt.Sprintf("  expected: label first used at %s:%d\n", first.file, first.line) +
		fmt.Sprintf("  observed: label reused at %s:%d", duplicate.file, duplicate.line)
	assertSubprocessOutput(t, "duplicate-label", want, duplicate.line)

	requireFirst := fixtureSite(t, requireFirstLabelUse, 1)
	requireDuplicate := fixtureSite(t, requireDuplicateLabelUse, 1)
	want = fmt.Sprintf("FAIL TestExpectDuplicateLabel %s:%d duplicate label repeated require label\n", requireDuplicate.file, requireDuplicate.line) +
		fmt.Sprintf("  expected: label first used at %s:%d\n", requireFirst.file, requireFirst.line) +
		fmt.Sprintf("  observed: label reused at %s:%d", requireDuplicate.file, requireDuplicate.line)
	assertSubprocessOutput(t, "require-duplicate-label", want, requireDuplicate.line)

	escapedFirst := fixtureSite(t, expectFirstEscapedLabelUse, 1)
	escapedDuplicate := fixtureSite(t, expectDuplicateEscapedLabelUse, 1)
	want = fmt.Sprintf("FAIL TestExpectDuplicateLabel %s:%d duplicate label line\\ncarriage\\rlabel\n", escapedDuplicate.file, escapedDuplicate.line) +
		fmt.Sprintf("  expected: label first used at %s:%d\n", escapedFirst.file, escapedFirst.line) +
		fmt.Sprintf("  observed: label reused at %s:%d", escapedDuplicate.file, escapedDuplicate.line)
	assertSubprocessOutput(t, "escaped-duplicate-label", want, escapedDuplicate.line)
}

func TestExpectBounds(t *testing.T) {
	switch os.Getenv(expectSubprocessCase) {
	case "bounds":
		expectBoundsFixture(t)
		return
	case "bounds-eacute":
		expectBoundsEAcuteFixture(t)
		return
	case "bounds-euro":
		expectBoundsEuroFixture(t)
		return
	case "bounds-exact":
		expectBoundsExactFixture(t)
		return
	case "bounds-emoji":
		expectBoundsEmojiFixture(t)
		return
	case "bounds-continuation-bytes":
		expectBoundsContinuationBytesFixture(t)
		return
	}

	site := fixtureSite(t, expectBoundsFixture, 1)
	want := fmt.Sprintf("FAIL TestExpectBounds %s:%d bounded values\n", site.file, site.line) +
		"  expected: " + strings.Repeat("e", 2048) + "[... 1024 more bytes]\n" +
		"  observed: " + strings.Repeat("o", 2048) + "[... 1024 more bytes]"
	assertSubprocessOutput(t, "bounds", want, site.line)

	eAcuteSite := fixtureSite(t, expectBoundsEAcuteFixture, 1)
	want = fmt.Sprintf("FAIL TestExpectBounds %s:%d e acute boundary\n", eAcuteSite.file, eAcuteSite.line) +
		"  expected: " + strings.Repeat("a", 2047) + "[... 2 more bytes]\n" +
		"  observed: short"
	assertSubprocessOutput(t, "bounds-eacute", want, eAcuteSite.line)

	euroSite := fixtureSite(t, expectBoundsEuroFixture, 1)
	want = fmt.Sprintf("FAIL TestExpectBounds %s:%d euro boundary\n", euroSite.file, euroSite.line) +
		"  expected: " + strings.Repeat("x", 2046) + "[... 3 more bytes]\n" +
		"  observed: short"
	assertSubprocessOutput(t, "bounds-euro", want, euroSite.line)

	exactSite := fixtureSite(t, expectBoundsExactFixture, 1)
	want = fmt.Sprintf("FAIL TestExpectBounds %s:%d exact boundary\n", exactSite.file, exactSite.line) +
		"  expected: " + strings.Repeat("z", 2048) + "\n" +
		"  observed: short"
	assertSubprocessOutput(t, "bounds-exact", want, exactSite.line)

	emojiSite := fixtureSite(t, expectBoundsEmojiFixture, 1)
	want = fmt.Sprintf("FAIL TestExpectBounds %s:%d emoji boundary\n", emojiSite.file, emojiSite.line) +
		"  expected: " + strings.Repeat("a", 2045) + "[... 104 more bytes]\n" +
		"  observed: short"
	assertSubprocessOutput(t, "bounds-emoji", want, emojiSite.line)

	continuationSite := fixtureSite(t, expectBoundsContinuationBytesFixture, 1)
	want = fmt.Sprintf("FAIL TestExpectBounds %s:%d continuation bytes\n", continuationSite.file, continuationSite.line) +
		"  expected: " + strings.Repeat(string([]byte{0x80}), 2048) + "[... 952 more bytes]\n" +
		"  observed: short"
	assertSubprocessOutput(t, "bounds-continuation-bytes", want, continuationSite.line)
}

func TestExpectRepositoryRelative(t *testing.T) {
	t.Run("git directory", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatalf("create Git directory: %v", err)
		}
		assertRelativeFixturePath(t, root)
	})

	t.Run("git file", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: elsewhere\n"), 0o644); err != nil {
			t.Fatalf("create Git file: %v", err)
		}
		assertRelativeFixturePath(t, root)
	})

	t.Run("module only", func(t *testing.T) {
		root := t.TempDir()
		if gitRoot := ancestorWithEntry(filepath.Dir(root), ".git"); gitRoot != "" {
			t.Skipf("go.mod-only path is below a .git entry in %s", gitRoot)
		}
		if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/path\n"), 0o644); err != nil {
			t.Fatalf("create module file: %v", err)
		}
		assertRelativeFixturePath(t, root)
	})

	t.Run("nearest module", func(t *testing.T) {
		root := t.TempDir()
		if gitRoot := ancestorWithEntry(filepath.Dir(root), ".git"); gitRoot != "" {
			t.Skipf("nested go.mod-only path is below a .git entry in %s", gitRoot)
		}
		if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/outer\n"), 0o644); err != nil {
			t.Fatalf("create outer module file: %v", err)
		}
		inner := filepath.Join(root, "inner")
		if err := os.Mkdir(inner, 0o755); err != nil {
			t.Fatalf("create inner module directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(inner, "go.mod"), []byte("module example.test/inner\n"), 0o644); err != nil {
			t.Fatalf("create inner module file: %v", err)
		}
		assertRelativeFixturePath(t, inner)
	})
}

func expectPrintsBlockFixture(t testing.TB) {
	Expect(t, "status text", "waiting", "ready")
}

func expectMatchPrintsBlockFixture(t testing.TB) {
	ExpectMatch(t, "status pattern", "waiting", regexp.MustCompile(`^ready:[0-9]+$`))
}

func expectDifferentIntegerTypesFixture(t testing.TB) {
	Expect(t, "integer types", int64(10), int(10))
}

func expectStringAndIntegerFixture(t testing.TB) {
	Expect(t, "string and integer", "3", 3)
}

func expectTrailingNewlineFixture(t testing.TB) {
	Expect(t, "trailing newline", "ready\n", "ready")
}

func expectCarriageReturnFixture(t testing.TB) {
	Expect(t, "carriage return", "left\rright", "other")
}

func expectControlBytesFixture(t testing.TB) {
	expectEscapeByteFixture(t)
	expectNULByteFixture(t)
}

func expectEscapeByteFixture(t testing.TB) { Expect(t, "escape byte", "left\x1bright", "other") }

func expectNULByteFixture(t testing.TB) { Expect(t, "nul byte", "left\x00right", "other") }

func expectEqualBeyondCutFixture(t testing.TB) {
	Expect(t, "equal beyond cut", strings.Repeat("a", 3000)+"y", strings.Repeat("a", 3000)+"x")
}

func expectNaNSamePrintFixture(t testing.TB) {
	Expect(t, "NaN values", math.NaN(), math.NaN())
}

func expectMultilineFixture(t testing.TB) {
	Expect(t, "multiline value", "line one\nFAIL TestOther x.go:1 other\nlast", "ready")
}

func expectPlainOutputFixture(t testing.TB) {
	Expect(t, "plain output", "waiting", "ready")
}

func expectKeepsGoingFixture(t testing.TB) {
	expectFirstFailure(t)
	expectSecondFailure(t)
}

func expectFirstFailure(t testing.TB) { Expect(t, "first failure", "observed one", "expected one") }

func expectSecondFailure(t testing.TB) { Expect(t, "second failure", []int{1, 2}, []int{3}) }

func requireStopsFixture(t testing.TB) {
	requireFirstFailure(t)
	expectUnreachableFailure(t)
}

func requireFirstFailure(t testing.TB) { Require(t, "required value", "waiting", "ready") }

func requireMatchStopsFixture(t testing.TB) {
	requireFirstMatchFailure(t)
	expectUnreachableFailure(t)
}

func requireFirstMatchFailure(t testing.TB) {
	RequireMatch(t, "required pattern", "waiting", regexp.MustCompile(`^ready:[0-9]+$`))
}

func expectUnreachableFailure(t testing.TB) { Expect(t, "unreachable value", "continued", "stopped") }

func expectDuplicateLabelFixture(t testing.TB) {
	expectFirstLabelUse(t)
	expectDuplicateLabelUse(t)
}

func expectFirstLabelUse(t testing.TB) { Expect(t, "repeated label", "same", "same") }

func expectDuplicateLabelUse(t testing.TB) {
	Expect(t, "repeated label", "ignored observed", "ignored expected")
}

func requireDuplicateLabelFixture(t testing.TB) {
	requireFirstLabelUse(t)
	requireDuplicateLabelUse(t)
	expectUnreachableFailure(t)
}

func requireFirstLabelUse(t testing.TB) {
	Expect(t, "repeated require label", "same", "same")
}

func requireDuplicateLabelUse(t testing.TB) {
	Require(t, "repeated require label", "ignored observed", "ignored expected")
}

func expectEscapedDuplicateLabelFixture(t testing.TB) {
	expectFirstEscapedLabelUse(t)
	expectDuplicateEscapedLabelUse(t)
}

func expectFirstEscapedLabelUse(t testing.TB) {
	Expect(t, "line\ncarriage\rlabel", "same", "same")
}

func expectDuplicateEscapedLabelUse(t testing.TB) {
	Expect(t, "line\ncarriage\rlabel", "ignored observed", "ignored expected")
}

func expectBoundsFixture(t testing.TB) {
	Expect(t, "bounded values", strings.Repeat("o", 3*1024), strings.Repeat("e", 3*1024))
}

func expectBoundsEAcuteFixture(t testing.TB) {
	Expect(t, "e acute boundary", "short", strings.Repeat("a", 2047)+"é")
}

func expectBoundsEuroFixture(t testing.TB) {
	Expect(t, "euro boundary", "short", strings.Repeat("x", 2046)+"€")
}

func expectBoundsExactFixture(t testing.TB) {
	Expect(t, "exact boundary", "short", strings.Repeat("z", 2048))
}

func expectBoundsEmojiFixture(t testing.TB) {
	Expect(t, "emoji boundary", "short", strings.Repeat("a", 2045)+"😀"+strings.Repeat("a", 100))
}

func expectBoundsContinuationBytesFixture(t testing.TB) {
	Expect(t, "continuation bytes", "short", strings.Repeat(string([]byte{0x80}), 3000))
}

func assertSubprocessOutput(t *testing.T, selectedCase, wantBlock string, callerLines ...int) {
	t.Helper()
	assertSubprocessOutputMode(t, selectedCase, true, wantBlock, callerLines...)
}

func assertPlainSubprocessOutput(t *testing.T, selectedCase, wantBlock string, callerLines ...int) {
	t.Helper()
	assertSubprocessOutputMode(t, selectedCase, false, wantBlock, callerLines...)
}

func assertSubprocessOutputMode(t *testing.T, selectedCase string, verbose bool, wantBlock string, callerLines ...int) {
	t.Helper()
	arguments := []string{"-test.run=^" + regexp.QuoteMeta(t.Name()) + "$"}
	if verbose {
		arguments = append(arguments, "-test.v")
	}
	command := exec.Command(os.Args[0], arguments...)
	command.Env = append(os.Environ(), expectSubprocessCase+"="+selectedCase)
	output, err := command.CombinedOutput()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("subprocess exit: expected status 1, observed error %v\n%s", err, output)
	}
	got := normalizeSubprocessOutput(output)
	want := expectedSubprocessOutput(t, wantBlock, callerLines, verbose)
	if !bytes.Equal(got, []byte(want)) {
		t.Errorf("subprocess output differs\nexpected:\n%s\nobserved:\n%s", want, got)
	}
}

func normalizeSubprocessOutput(output []byte) []byte {
	output = regexp.MustCompile(`\([0-9]+(?:\.[0-9]+)?s\)`).ReplaceAll(output, []byte("(DURATION)"))
	coverageLine := regexp.MustCompile(`^coverage: [0-9]+(?:\.[0-9]+)?% of statements$`)
	lines := bytes.Split(output, []byte("\n"))
	kept := lines[:0]
	for _, line := range lines {
		if coverageLine.Match(line) {
			continue
		}
		kept = append(kept, line)
	}
	return bytes.Join(kept, []byte("\n"))
}

func expectedSubprocessOutput(t *testing.T, wantBlock string, callerLines []int, verbose bool) string {
	t.Helper()
	lines := strings.Split(wantBlock, "\n")
	var output strings.Builder
	if verbose {
		fmt.Fprintf(&output, "=== RUN   %s\n", t.Name())
	} else {
		fmt.Fprintf(&output, "--- FAIL: %s (DURATION)\n", t.Name())
	}
	failureIndex := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "FAIL ") {
			if failureIndex >= len(callerLines) {
				t.Fatalf("expected block has more failures than caller lines")
			}
			fmt.Fprintf(&output, "    expect_test.go:%d: %s\n", callerLines[failureIndex], line)
			failureIndex++
			continue
		}
		fmt.Fprintf(&output, "        %s\n", line)
	}
	if failureIndex != len(callerLines) {
		t.Fatalf("expected block has %d failures for %d caller lines", failureIndex, len(callerLines))
	}
	if verbose {
		fmt.Fprintf(&output, "--- FAIL: %s (DURATION)\n", t.Name())
	}
	output.WriteString("FAIL\n")
	return output.String()
}

func fixtureSite(t *testing.T, fixture any, callLineOffset int) callSite {
	t.Helper()
	programCounter := reflect.ValueOf(fixture).Pointer()
	function := runtime.FuncForPC(programCounter)
	if function == nil {
		t.Fatal("fixture has no runtime function")
	}
	file, line := function.FileLine(programCounter)
	return callSite{file: expectedFixtureFile(t, file), line: line + callLineOffset}
}

func expectedFixtureFile(t *testing.T, file string) string {
	t.Helper()
	for dir := filepath.Dir(file); ; dir = filepath.Dir(dir) {
		if entryExists(filepath.Join(dir, "go.mod")) {
			if gitRoot := ancestorWithEntry(dir, ".git"); gitRoot != "" {
				return relativeTestPath(t, gitRoot, file)
			}
			return relativeTestPath(t, dir, file)
		}
		if parent := filepath.Dir(dir); parent == dir {
			break
		}
	}
	t.Fatalf("fixture source has no module root: %s", file)
	return ""
}

func assertRelativeFixturePath(t *testing.T, root string) {
	t.Helper()
	directory := filepath.Join(root, "pkg")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	file := filepath.Join(directory, "case.go")
	if err := os.WriteFile(file, []byte("package pkg\n"), 0o644); err != nil {
		t.Fatalf("create fixture source: %v", err)
	}
	if got, want := repositoryRelative(file), "pkg/case.go"; got != want {
		t.Fatalf("repository-relative path: expected %q, observed %q", want, got)
	}
}

func ancestorWithEntry(start, name string) string {
	for dir := start; ; dir = filepath.Dir(dir) {
		if entryExists(filepath.Join(dir, name)) {
			return dir
		}
		if parent := filepath.Dir(dir); parent == dir {
			return ""
		}
	}
}

func entryExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func relativeTestPath(t *testing.T, root, file string) string {
	t.Helper()
	relative, err := filepath.Rel(root, file)
	if err != nil {
		t.Fatalf("make fixture path relative: %v", err)
	}
	return filepath.ToSlash(relative)
}

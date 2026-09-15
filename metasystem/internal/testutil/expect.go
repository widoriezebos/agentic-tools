package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
)

const renderedValueLimit = 2048

type callSite struct {
	file string
	line int
}

var labels = struct {
	sync.Mutex
	byTest map[testing.TB]map[string]callSite
}{
	byTest: make(map[testing.TB]map[string]callSite),
}

// Expect reports unequal values while allowing the test to continue. A wrapper
// marked with t.Helper() is named by its own call line in the block, while the
// testing framework's prefix names the test's line.
func Expect(t testing.TB, what string, observed, expected any) {
	t.Helper()
	site := expectCallSite()
	if first, duplicate := recordLabel(t, what, site); duplicate {
		t.Errorf("%s", duplicateLabelBlock(t.Name(), what, first, site))
		return
	}
	if reflect.DeepEqual(observed, expected) {
		return
	}
	expectedText, observedText := renderPair(expected, observed)
	t.Errorf("%s", failureBlock(t.Name(), site, what, expectedText, observedText))
}

// ExpectMatch reports a pattern mismatch while allowing the test to continue.
func ExpectMatch(t testing.TB, what string, observed string, pattern *regexp.Regexp) {
	t.Helper()
	site := expectCallSite()
	if first, duplicate := recordLabel(t, what, site); duplicate {
		t.Errorf("%s", duplicateLabelBlock(t.Name(), what, first, site))
		return
	}
	if pattern.MatchString(observed) {
		return
	}
	expectedText, observedText := renderMatchPair(pattern, observed)
	t.Errorf("%s", failureBlock(t.Name(), site, what, expectedText, observedText))
}

// Require stops the test when a precondition does not equal its expected value.
// It stops the test only when called from the goroutine running it, as with
// t.FailNow.
func Require(t testing.TB, what string, observed, expected any) {
	t.Helper()
	site := expectCallSite()
	if first, duplicate := recordLabel(t, what, site); duplicate {
		t.Errorf("%s", duplicateLabelBlock(t.Name(), what, first, site))
		t.FailNow()
	}
	if reflect.DeepEqual(observed, expected) {
		return
	}
	expectedText, observedText := renderPair(expected, observed)
	t.Fatalf("%s", failureBlock(t.Name(), site, what, expectedText, observedText))
}

// RequireMatch stops the test when a precondition does not match its pattern.
// It stops the test only when called from the goroutine running it, as with
// t.FailNow.
func RequireMatch(t testing.TB, what string, observed string, pattern *regexp.Regexp) {
	t.Helper()
	site := expectCallSite()
	if first, duplicate := recordLabel(t, what, site); duplicate {
		t.Errorf("%s", duplicateLabelBlock(t.Name(), what, first, site))
		t.FailNow()
	}
	if pattern.MatchString(observed) {
		return
	}
	expectedText, observedText := renderMatchPair(pattern, observed)
	t.Fatalf("%s", failureBlock(t.Name(), site, what, expectedText, observedText))
}

func recordLabel(t testing.TB, what string, site callSite) (callSite, bool) {
	labels.Lock()
	defer labels.Unlock()

	testLabels, ok := labels.byTest[t]
	if !ok {
		testLabels = make(map[string]callSite)
		labels.byTest[t] = testLabels
		t.Cleanup(func() {
			labels.Lock()
			defer labels.Unlock()
			delete(labels.byTest, t)
		})
	}
	if first, ok := testLabels[what]; ok {
		return first, true
	}
	testLabels[what] = site
	return callSite{}, false
}

func failureBlock(testName string, site callSite, what, expected, observed string) string {
	what = strings.NewReplacer("\n", `\n`, "\r", `\r`).Replace(what)
	return fmt.Sprintf(
		"FAIL %s %s:%d %s\n  expected: %s\n  observed: %s",
		testName,
		site.file,
		site.line,
		what,
		indentValue(expected),
		indentValue(observed),
	)
}

func duplicateLabelBlock(testName, what string, first, duplicate callSite) string {
	return failureBlock(
		testName,
		duplicate,
		"duplicate label "+what,
		bounded(fmt.Sprintf("label first used at %s:%d", first.file, first.line)),
		bounded(fmt.Sprintf("label reused at %s:%d", duplicate.file, duplicate.line)),
	)
}

func renderPair(expected, observed any) (string, string) {
	expectedText := renderValue(expected)
	observedText := renderValue(observed)
	return finishRenderedPair(expectedText, observedText, expected, observed)
}

func finishRenderedPair(expectedText, observedText string, expected, observed any) (string, string) {
	if expectedText == observedText {
		expectedText = fmt.Sprintf("(%T) %s", expected, expectedText)
		observedText = fmt.Sprintf("(%T) %s", observed, observedText)
	}
	expectedFull := expectedText
	observedFull := observedText
	expectedText = bounded(expectedFull)
	observedText = bounded(observedFull)
	if expectedText != observedText {
		return expectedText, observedText
	}
	if expectedFull != observedFull {
		observedText += fmt.Sprintf(" [first difference at byte %d]", firstDifference(expectedFull, observedFull))
	} else {
		observedText += " [the values differ but print the same]"
	}
	return expectedText, observedText
}

func renderMatchPair(pattern *regexp.Regexp, observed string) (string, string) {
	expectedText := renderString(pattern.String())
	observedText := renderString(observed)
	return finishRenderedPair(expectedText, observedText, pattern, observed)
}

func firstDifference(expected, observed string) int {
	limit := len(expected)
	if len(observed) < limit {
		limit = len(observed)
	}
	for index := 0; index < limit; index++ {
		if expected[index] != observed[index] {
			return index
		}
	}
	return limit
}

// renderValue returns the complete printed form. Bounding happens afterward,
// so a quoted value is cut in quoted form and its omitted count counts printed
// bytes.
func renderValue(value any) string {
	valueType := reflect.TypeOf(value)
	if valueType != nil && valueType.Kind() == reflect.String {
		return renderString(reflect.ValueOf(value).String())
	}
	return fmt.Sprintf("%#v", value)
}

func renderString(value string) string {
	if strings.HasSuffix(value, "\n") || hasUnsafeControlByte(value) {
		return fmt.Sprintf("%q", value)
	}
	return value
}

func hasUnsafeControlByte(value string) bool {
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character == '\n' || character == '\t' {
			continue
		}
		if character < 0x20 || character == 0x7f {
			return true
		}
	}
	return false
}

func indentValue(value string) string {
	return strings.ReplaceAll(value, "\n", "\n            ")
}

func bounded(value string) string {
	if len(value) <= renderedValueLimit {
		return value
	}
	cut := renderedValueLimit
	for lookback := 0; lookback <= 3; lookback++ {
		candidate := renderedValueLimit - lookback
		if value[candidate]&0xc0 != 0x80 {
			cut = candidate
			break
		}
	}
	return fmt.Sprintf("%s[... %d more bytes]", value[:cut], len(value)-cut)
}

func expectCallSite() callSite {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return callSite{file: "unknown", line: 0}
	}
	return callSite{file: repositoryRelative(file), line: line}
}

func repositoryRelative(file string) string {
	if absolute, err := filepath.Abs(file); err == nil {
		file = absolute
	}
	moduleRoot := ""
	dir := filepath.Dir(file)
	for {
		if pathEntryExists(filepath.Join(dir, ".git")) {
			return relativeTo(dir, file)
		}
		if moduleRoot == "" && pathEntryExists(filepath.Join(dir, "go.mod")) {
			moduleRoot = dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if moduleRoot != "" {
		return relativeTo(moduleRoot, file)
	}
	return filepath.ToSlash(file)
}

func pathEntryExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func relativeTo(root, file string) string {
	relative, err := filepath.Rel(root, file)
	if err != nil {
		return filepath.ToSlash(file)
	}
	return filepath.ToSlash(relative)
}

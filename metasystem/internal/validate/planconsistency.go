package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// A plan retires a term by writing, anywhere in its text:
//
//	RETIRED: <term> -- <what replaced it>
var retiredLineRe = regexp.MustCompile(`^RETIRED:\s*(.+?)\s*--\s*(.+?)\s*$`)

// A line that mentions a retired term while explaining the change,
// rather than prescribing it. Kept small on purpose: every word here is
// a way to say "this used to be true", and a longer list would start
// excusing real drift.
var explainsRe = regexp.MustCompile(
	`RETIRED|SUPERSEDED|superseded|replaces|replaced|no longer|removed|` +
		`used to|earlier|first draft|was vacuous`)

// PlanConsistency reports every term a plan retires that some plan line
// still prescribes as though it were current. A mention is allowed only
// on the retiring line itself or on a line that explains the change. It
// returns the number of retired terms and the violations ordered by
// term, then file, then line.
func PlanConsistency(plansDir string) (int, []string, error) {
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		return 0, nil, err
	}
	var names []string
	texts := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(plansDir, name))
		if err != nil {
			continue
		}
		names = append(names, name)
		texts[name] = string(data)
	}
	sort.Strings(names)

	type retirement struct{ declaredIn, replacement string }
	retired := map[string]retirement{}
	for _, name := range names {
		for _, line := range splitLines(texts[name]) {
			match := retiredLineRe.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			term := strings.TrimSpace(match[1])
			if term != "" {
				retired[term] = retirement{name, strings.TrimSpace(match[2])}
			}
		}
	}

	var terms []string
	for term := range retired {
		terms = append(terms, term)
	}
	sort.Strings(terms)

	var violations []string
	for _, term := range terms {
		lowered := strings.ToLower(term)
		declaration := retired[term]
		for _, name := range names {
			for number, line := range splitLines(texts[name]) {
				if !strings.Contains(strings.ToLower(line), lowered) {
					continue
				}
				if strings.HasPrefix(strings.TrimLeftFunc(line, unicode.IsSpace), "RETIRED:") {
					continue
				}
				if explainsRe.MatchString(line) {
					continue
				}
				violations = append(violations, fmt.Sprintf(
					"%s:%d: prescribes '%s', retired in %s in favour of %s",
					name, number+1, term, declaration.declaredIn, declaration.replacement))
			}
		}
	}
	return len(retired), violations, nil
}

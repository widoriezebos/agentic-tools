package validate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// A verbatim quote block in a role preamble:
//
//	<!-- quote source="path/from/metasystem/root" -->
//	byte-exact content copied from that source
//	<!-- /quote -->
//
// The final line ending before the closing marker is marker framing,
// not quote content.
var quoteBlockRe = regexp.MustCompile(`(?ms)^<!-- quote source="([^"\r\n]+)" -->\n(.*?)^<!-- /quote -->$`)

// PreambleQuotes checks every Markdown role preamble in rolesDir: each
// quote block's content bytes, including Markdown punctuation and
// internal line endings, must occur unchanged and contiguously in the
// named source document under the metasystem root. It returns every
// violation found.
func PreambleQuotes(root, rolesDir string) []string {
	var violations []string
	rootResolved := resolvePath(root)
	rolesResolved := resolvePath(rolesDir)

	info, err := os.Stat(rolesResolved)
	if err != nil || !info.IsDir() {
		return []string{fmt.Sprintf("roles directory does not exist: %s", rolesResolved)}
	}
	entries, err := os.ReadDir(rolesResolved)
	if err != nil {
		return []string{fmt.Sprintf("roles directory does not exist: %s", rolesResolved)}
	}
	var preambles []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".md") {
			preambles = append(preambles, filepath.Join(rolesResolved, entry.Name()))
		}
	}
	sort.Strings(preambles)
	if len(preambles) == 0 {
		return []string{fmt.Sprintf("roles directory contains no Markdown preambles: %s", rolesResolved)}
	}

	for _, preamble := range preambles {
		body, err := os.ReadFile(preamble)
		if err != nil {
			violations = append(violations, fmt.Sprintf("%s: could not be read: %v", preamble, err))
			continue
		}
		matches := quoteBlockRe.FindAllSubmatchIndex(body, -1)
		startCount := bytes.Count(body, []byte(`<!-- quote source="`))
		endCount := bytes.Count(body, []byte(`<!-- /quote -->`))
		if len(matches) == 0 {
			violations = append(violations, fmt.Sprintf("%s: no verbatim quote block", preamble))
			continue
		}
		if startCount != len(matches) || endCount != len(matches) {
			violations = append(violations, fmt.Sprintf("%s: malformed or unpaired quote marker", preamble))
			continue
		}
		for _, match := range matches {
			sourceName := string(body[match[2]:match[3]])
			candidate := sourceName
			if !filepath.IsAbs(candidate) {
				candidate = filepath.Join(rootResolved, sourceName)
			}
			source := resolvePath(candidate)
			if !pathWithin(rootResolved, source) {
				violations = append(violations, fmt.Sprintf(
					"%s: quote source escapes the metasystem root: %s", preamble, sourceName))
				continue
			}
			sourceInfo, err := os.Stat(source)
			if err != nil || !sourceInfo.Mode().IsRegular() {
				violations = append(violations, fmt.Sprintf(
					"%s: quote source does not exist: %s", preamble, sourceName))
				continue
			}
			quoted := body[match[4]:match[5]]
			quoted = bytes.TrimSuffix(quoted, []byte("\n"))
			if len(quoted) == 0 {
				violations = append(violations, fmt.Sprintf("%s: quote from %s is empty", preamble, sourceName))
				continue
			}
			sourceBytes, err := os.ReadFile(source)
			if err != nil || !bytes.Contains(sourceBytes, quoted) {
				violations = append(violations, fmt.Sprintf("%s: quote drifted from %s", preamble, sourceName))
			}
		}
	}
	return violations
}

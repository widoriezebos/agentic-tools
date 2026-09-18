package launch

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const defaultTemplateDirectory = "scripts/agents/templates"

var (
	placeholderPattern = regexp.MustCompile(`<[^<>\n]+>`)
	citedRangePattern  = regexp.MustCompile("`([^`\\n]+):([0-9]+)-([0-9]+)`")
	fencePattern       = regexp.MustCompile("^([ \\t]+)(`{3,})[^`]*$")
)

type citedRange struct {
	label      string
	path       string
	start, end int
	briefLine  int
}

// CheckPack validates the context prepared for design and read launches.
// Other launch kinds have no context-pack contract.
func (m *Manager) CheckPack(spec StartSpec) (int, error) {
	templateName := map[string]string{"design": "design-brief.md", "read": "review-brief.md"}[spec.Kind]
	if templateName == "" {
		return 0, nil
	}
	templateDirectory := m.TemplateDirectory
	if templateDirectory == "" {
		templateDirectory = defaultTemplateDirectory
	}
	template, err := os.ReadFile(filepath.Join(templateDirectory, templateName))
	if err != nil {
		return 0, err
	}
	brief, err := os.ReadFile(spec.Brief)
	if err != nil {
		return 0, err
	}
	if kept := keptPlaceholders(template, brief); len(kept) > 0 {
		lines := []string{fmt.Sprintf("LAUNCH_BRIEF_PACK_UNFILLED kind=%s", spec.Kind)}
		for _, item := range kept {
			lines = append(lines, fmt.Sprintf("placeholder=%s line=%d", item.token, item.line))
		}
		return 0, fmt.Errorf("%s", strings.Join(lines, "\n"))
	}

	ranges := citedRanges(brief)
	briefLines := splitLines(brief)
	for _, item := range ranges {
		file, err := readCitedFile(spec.WorkingDirectory, item.path)
		if err != nil {
			return 0, packDrift(item, "missing file")
		}
		fileLines := splitLines(file)
		if item.start < 1 || item.start > item.end || item.end > len(fileLines) {
			return 0, packDrift(item, fmt.Sprintf("past end (%d lines)", len(fileLines)))
		}
		if spec.Kind != "design" {
			continue
		}
		excerpt, ok := followingIndentedFence(briefLines, item.briefLine)
		if !ok {
			continue
		}
		want := fileLines[item.start-1 : item.end]
		for offset := 0; offset < len(want) || offset < len(excerpt); offset++ {
			var source, got []byte
			if offset < len(want) {
				source = want[offset]
			}
			if offset < len(excerpt) {
				got = excerpt[offset]
			}
			if !bytes.Equal(source, got) {
				return 0, packDrift(item, fmt.Sprintf("line %d differs source_bytes=%d excerpt_bytes=%d", item.start+offset, len(source), len(got)))
			}
		}
	}
	return len(ranges), nil
}

type keptPlaceholder struct {
	token string
	line  int
}

func keptPlaceholders(template, brief []byte) []keptPlaceholder {
	known := map[string]bool{}
	for _, match := range placeholderPattern.FindAll(template, -1) {
		known[string(match)] = true
	}
	var kept []keptPlaceholder
	for _, match := range placeholderPattern.FindAllIndex(brief, -1) {
		token := string(brief[match[0]:match[1]])
		if known[token] {
			kept = append(kept, keptPlaceholder{token: token, line: 1 + bytes.Count(brief[:match[0]], []byte("\n"))})
		}
	}
	return kept
}

func citedRanges(brief []byte) []citedRange {
	var ranges []citedRange
	for _, match := range citedRangePattern.FindAllSubmatchIndex(brief, -1) {
		start, startErr := strconv.Atoi(string(brief[match[4]:match[5]]))
		end, endErr := strconv.Atoi(string(brief[match[6]:match[7]]))
		if startErr != nil || endErr != nil {
			continue
		}
		ranges = append(ranges, citedRange{
			label:     string(brief[match[0]:match[1]]),
			path:      string(brief[match[2]:match[3]]),
			start:     start,
			end:       end,
			briefLine: 1 + bytes.Count(brief[:match[0]], []byte("\n")),
		})
	}
	return ranges
}

func readCitedFile(directory, name string) ([]byte, error) {
	if filepath.IsAbs(name) {
		return os.ReadFile(name)
	}
	if directory == "" {
		directory = "."
	}
	data, err := os.ReadFile(filepath.Join(directory, name))
	if err == nil || !os.IsNotExist(err) {
		return data, err
	}
	return os.ReadFile(filepath.Join(directory, "metasystem", name))
}

func splitLines(data []byte) [][]byte {
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func followingIndentedFence(lines [][]byte, citedLine int) ([][]byte, bool) {
	for index := citedLine; index < len(lines); index++ {
		if len(bytes.TrimSpace(lines[index])) == 0 {
			continue
		}
		match := fencePattern.FindSubmatch(lines[index])
		if match == nil {
			return nil, false
		}
		indent, fence := match[1], match[2]
		var excerpt [][]byte
		for index++; index < len(lines); index++ {
			line := lines[index]
			hasIndent := bytes.HasPrefix(line, indent)
			trimmed := bytes.TrimPrefix(line, indent)
			if hasIndent && bytes.HasPrefix(trimmed, fence) && len(bytes.TrimSpace(bytes.TrimPrefix(trimmed, fence))) == 0 {
				return excerpt, true
			}
			if hasIndent {
				line = line[len(indent):]
			}
			excerpt = append(excerpt, line)
		}
		return excerpt, true
	}
	return nil, false
}

func packDrift(item citedRange, reason string) error {
	return fmt.Errorf("LAUNCH_BRIEF_PACK_DRIFTED range=%s %s", item.label, reason)
}

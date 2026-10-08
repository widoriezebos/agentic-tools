package dispatch

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

func briefCitationPath(value string) bool {
	bare := briefPathLine.ReplaceAllString(value, "")
	if filepath.Base(bare) == "metasystem.conf.local" && !briefPathLine.MatchString(value) {
		return false
	}
	return !strings.ContainsAny(bare, "*<>${\t\n") && !strings.Contains(bare, "://") &&
		(briefPathLine.MatchString(value) || strings.Contains(bare, "/") || strings.Contains(" .go .md .json .txt .sh .conf .toml .yaml .yml .ts .tsx ", " "+filepath.Ext(bare)+" ")) && !strings.Contains(bare, " --") && !strings.HasSuffix(bare, "...") && !strings.HasSuffix(bare, "/")
}
func ResolveBriefCitation(original, root, base, prefix string, roots []string, facts briefTreeFacts) (string, bool, error) {
	name := briefPathLine.ReplaceAllString(original, "")
	fail := func(normalized, problem string) (string, bool, error) {
		return normalized, false, &BriefAuthorityRefusal{MissingPaths: []string{original}, Details: map[string]string{original: "normalized path " + normalized + "; base " + base + "; " + problem}}
	}
	candidates := map[string]bool{}
	add := func(value string) {
		value = filepath.ToSlash(filepath.Clean(value))
		if value != ".." && !strings.HasPrefix(value, "../") && !filepath.IsAbs(value) {
			candidates[value] = true
		}
	}
	if filepath.IsAbs(name) {
		rel, err := filepath.Rel(roots[1], name)
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
			return fail(name, "outside the repository")
		}
		add(rel)
	} else {
		add(name)
		if prefix != "" && !strings.HasPrefix(name, prefix+"/") {
			add(prefix + "/" + name)
		}
		for _, directory := range roots {
			rel, err := filepath.Rel(roots[1], filepath.Join(directory, filepath.FromSlash(name)))
			if err == nil {
				add(rel)
			}
		}
	}
	var matches []string
	names := slices.Sorted(maps.Keys(candidates))
	for _, candidate := range names {
		if filepath.Base(candidate) == "metasystem.conf.local" {
			return fail(candidate, "local configuration is not a citation input")
		}
		present, err := facts.HasPath(root, base, candidate)
		if err != nil {
			return candidate, false, fmt.Errorf("citation %s; normalized path %s; base %s: %w", original, candidate, base, err)
		}
		if present {
			matches = append(matches, candidate)
		}
	}
	if len(matches) > 1 {
		return fail(strings.Join(matches, ", "), "ambiguous citation")
	}
	if len(matches) == 0 {
		return append(names, filepath.ToSlash(filepath.Clean(name)))[0], false, nil
	}
	normalized := matches[0]
	if suffix := briefPathLine.FindStringSubmatch(original); suffix != nil {
		first, err := strconv.Atoi(suffix[1])
		last := first
		if suffix[2] != "" {
			last, err = strconv.Atoi(suffix[2])
		}
		if err != nil || first < 1 || last < first {
			return fail(normalized, "invalid line range")
		}
		reader, ok := facts.(interface {
			Blob(string, string, string) ([]byte, error)
		})
		if !ok {
			return fail(normalized, "base blob reader unavailable")
		}
		content, err := reader.Blob(root, base, normalized)
		if err != nil {
			return normalized, false, fmt.Errorf("citation %s; normalized path %s; base %s: %w", original, normalized, base, err)
		}
		count := strings.Count(string(content), "\n")
		if len(content) > 0 && content[len(content)-1] != '\n' {
			count++
		}
		if last > count {
			return fail(normalized, fmt.Sprintf("line %d does not exist; blob has %d lines", last, count))
		}
	}
	return normalized, true, nil
}
func (f gitBriefTreeFacts) Blob(root, base, name string) ([]byte, error) {
	if f.run != nil {
		output, err := f.output(root, "cat-file", "blob", base+":"+name)
		return []byte(output), err
	}
	return gitRawOutput(root, "cat-file", "blob", base+":"+name)
}
func ValidateBriefCitations(data []byte, document, install, root, caller string, git func(string, ...string) (string, error)) error {
	facts := &gitBriefTreeFacts{run: git}
	return validateBriefAuthority(data, BriefBounds{}, root, root, facts, func() (string, error) { return facts.InstallPrefix(install) }, "", filepath.Dir(document), caller, install)
}

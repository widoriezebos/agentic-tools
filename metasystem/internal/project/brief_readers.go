package project

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

var briefInlineName = regexp.MustCompile("`([^`\n]+)`")
var briefName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)
var briefQualifiedName = regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_-]*(?:\.[A-Za-z_][A-Za-z0-9_-]*)+\b`)

// BriefReaders observes references on one immutable tree. Its warnings are
// incomplete evidence, never authority to add callers or widen a unit.
func BriefReaders(decision, readers, root, base string, git func(string, ...string) ([]byte, error), gitInput func(string, []byte, ...string) ([]byte, error)) string {
	names := map[string]bool{}
	add := func(name string) {
		if briefName.MatchString(name) && !slices.Contains([]string{"go", "git", "make", "bash", "sh", "low", "medium", "high", "xhigh", "max", "true", "false", "auto", "none"}, name) && !strings.Contains(" .go .md .json .sh .txt .conf .toml .yaml .yml .ts .tsx ", " "+filepath.Ext(name)+" ") {
			names[name] = true
		}
	}
	for _, match := range briefInlineName.FindAllStringSubmatch(decision+"\n"+readers, -1) {
		add(match[1])
	}
	plain := briefInlineName.ReplaceAllString(decision+"\n"+readers, " ")
	for _, token := range strings.Fields(plain) {
		if !strings.ContainsAny(token, "/\\[]") {
			for _, name := range briefQualifiedName.FindAllString(token, -1) {
				add(name)
			}
		}
	}
	for _, name := range strings.FieldsFunc(readers, func(r rune) bool { return r == ',' || r == ';' || r == '\n' || r == '`' }) {
		add(strings.TrimSpace(name))
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	slices.Sort(ordered)
	var out strings.Builder
	fmt.Fprintf(&out, "Textual reference search on base %s; evidence to inspect, not a proved call graph.\nSearched names: %s\n", base, strings.Join(ordered, ", "))
	if len(ordered) == 0 {
		out.WriteString("No explicitly cited reader names.\n")
		return out.String()
	}
	listing, err := git(root, "ls-tree", "-rz", "--full-tree", base)
	if err != nil {
		fmt.Fprintf(&out, "Coverage incomplete: cannot list base %s: %v. Restore tree access and regenerate the brief.\n", base, err)
		return out.String()
	}
	entries := strings.Split(strings.TrimSuffix(string(listing), "\x00"), "\x00")
	slices.SortFunc(entries, func(a, b string) int {
		_, ap, _ := strings.Cut(a, "\t")
		_, bp, _ := strings.Cut(b, "\t")
		return strings.Compare(ap, bp)
	})
	var paths []string
	var input strings.Builder
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		metadata, path, ok := strings.Cut(entry, "\t")
		fields := strings.Fields(metadata)
		if !ok || len(fields) != 3 {
			fmt.Fprintln(&out, "Coverage incomplete: malformed base tree listing. Restore tree access and regenerate the brief.")
			return out.String()
		}
		if filepath.Base(path) == "metasystem.conf.local" || slices.Contains(strings.Split(path, "/"), "artifacts") || fields[1] != "blob" {
			fmt.Fprintf(&out, "Excluded from search: %s.\n", path)
			continue
		}
		paths = append(paths, path)
		input.WriteString(fields[2] + "\n")
	}
	matches := map[string]bool{}
	patterns := map[string]*regexp.Regexp{}
	for _, name := range ordered {
		patterns[name] = regexp.MustCompile(`(^|[^\pL\pN_])` + regexp.QuoteMeta(name) + `($|[^\pL\pN_])`)
	}
	complete := true
	var batch []byte
	if len(paths) > 0 {
		batch, err = gitInput(root, []byte(input.String()), "cat-file", "--batch")
		if err != nil {
			complete = false
			fmt.Fprintf(&out, "Coverage incomplete: cannot read base blobs: %v. Restore blob access and regenerate the brief.\n", err)
		}
	}
	for _, path := range paths {
		header, rest, ok := bytes.Cut(batch, []byte("\n"))
		fields := strings.Fields(string(header))
		if ok && len(fields) == 2 && fields[1] == "missing" {
			complete, batch = false, rest
			fmt.Fprintf(&out, "Coverage incomplete: cannot read %s at %s: object missing. Restore blob access and regenerate the brief.\n", path, base)
			continue
		}
		size := -1
		if len(fields) == 3 && fields[1] == "blob" {
			size, err = strconv.Atoi(fields[2])
		}
		if !ok || err != nil || size < 0 || size >= len(rest) || rest[size] != '\n' {
			complete = false
			fmt.Fprintf(&out, "Coverage incomplete: cannot read %s at %s: malformed or truncated batch. Restore blob access and regenerate the brief.\n", path, base)
			break
		}
		blob := rest[:size]
		batch = rest[size+1:]
		if bytes.ContainsRune(blob, 0) || !utf8.Valid(blob) {
			fmt.Fprintf(&out, "Skipped binary blob: %s.\n", path)
			continue
		}
		var sites strings.Builder
		for number, line := range strings.Split(string(blob), "\n") {
			for _, name := range ordered {
				if patterns[name].MatchString(line) {
					matches[name] = true
					fmt.Fprintf(&sites, "- %s → %s:%d\n", name, path, number+1)
				}
			}
		}
		if sites.Len() > 0 {
			fmt.Fprintf(&out, "\nReferences in %s:\n%s", path, sites.String())
		}
	}
	for _, name := range ordered {
		if !matches[name] {
			declaredNew := regexp.MustCompile(`(?i)\b(?:new|proposed)\s+` + "`?" + regexp.QuoteMeta(name) + "`?" + `\b|` + "`" + regexp.QuoteMeta(name) + "`" + `\s*\((?:new|proposed)\)`)
			if declaredNew.MatchString(decision + "\n" + readers) {
				if complete {
					fmt.Fprintf(&out, "- %s: no existing match; proposed name.\n", name)
				} else {
					fmt.Fprintf(&out, "- %s: no observed match; proposed name; coverage incomplete.\n", name)
				}
			} else {
				fmt.Fprintf(&out, "- %s: unresolved-reader warning; no observed match. Correct the cited name or restore incomplete coverage and regenerate; do not invent a caller.\n", name)
			}
		}
	}
	return out.String()
}

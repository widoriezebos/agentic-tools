package review

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/markdown"
)

// The evidence owner (g1-s71 D4, §6).
//
// A review record's Evidence line is copied from its design's, and names where
// the build lane kept what it saw of the work running: screenshots and reports,
// usually outside the checkout, where neither the Partner's own reads nor the
// document reader go. This owner is bounded to that one path by an anchored
// root open — the lexical inside is not containment, a link is — and answers
// two reads: a listing, so the Partner and the human know what is there, and
// one file, an image or text. The listing, the present offer and the browser's
// read name a file by one convention: its path relative to the evidence path.

// The two kinds of evidence this owner serves.
const (
	EvidenceImage = "image"
	EvidenceText  = "text"
)

// The evidence bounds: how many entries a listing carries, how large one file
// may be, and how many lines of a text are shown — the desk's diff bound, the
// largest a desk shows.
const (
	MaxEvidenceEntries = 500
	MaxEvidenceBytes   = 4 << 20
	MaxEvidenceLines   = MaxDiffLines
)

// evidenceTypes are the files this owner serves, by extension: images by their
// content type, and text.
var evidenceTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp",
	".md": "", ".markdown": "", ".txt": "", ".log": "", ".json": "", ".csv": "", ".tsv": "", ".yaml": "", ".yml": "",
}

// EvidenceEntry is one file of the listing.
type EvidenceEntry struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Size int64  `json:"size"`
}

// EvidenceListing is what the evidence path holds, images and text only, in
// path order, bounded, saying the whole.
type EvidenceListing struct {
	Root     string          `json:"root"`
	Entries  []EvidenceEntry `json:"entries"`
	Supplied int             `json:"supplied"`
	Total    int             `json:"total"`
}

// EvidenceFile is one file: an image's bytes and type, or a text's lines within
// the bound.
type EvidenceFile struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Type string `json:"-"`
	Body []byte `json:"-"`
	Text string `json:"text,omitempty"`
	// Blocks are the text as the section renderer draws it, parsed from the
	// bytes this read returned: headings or none, a report is a report.
	Blocks   []markdown.Block `json:"blocks,omitempty"`
	Supplied int              `json:"supplied,omitempty"`
	Total    int              `json:"total,omitempty"`
}

// EvidenceIn is the path a record's head names on its Evidence line, or "".
func EvidenceIn(source string) string {
	for _, line := range strings.Split(source, "\n") {
		if strings.HasPrefix(line, "## ") {
			break
		}
		if key, value, isHead := headLine(line); isHead && strings.EqualFold(key, "evidence") {
			return value
		}
	}
	return ""
}

// evidenceRoot is the directory an Evidence line names: a leading ~ is the
// account's home, and a relative path is the checkout's.
func (o Owner) evidenceRoot(named string) (string, error) {
	named = strings.TrimSpace(named)
	if named == "" {
		return "", refused("this review's record names no Evidence path, so there is no evidence to read")
	}
	if rest, homed := strings.CutPrefix(named, "~"); homed && (rest == "" || strings.HasPrefix(rest, "/")) {
		home := o.Home
		if home == "" {
			found, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("cannot find the home directory the Evidence path names: %w", err)
			}
			home = found
		}
		return filepath.Join(home, filepath.FromSlash(rest)), nil
	}
	if filepath.IsAbs(named) {
		return filepath.Clean(named), nil
	}
	if strings.TrimSpace(o.Checkout) == "" {
		return "", errors.New("this reader was given no checkout to read a relative Evidence path under")
	}
	return filepath.Join(o.Checkout, filepath.FromSlash(named)), nil
}

// openEvidence is the anchored root of the evidence path.
func (o Owner) openEvidence(named string) (*os.Root, string, error) {
	dir, err := o.evidenceRoot(named)
	if err != nil {
		return nil, "", err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, "", refused("the Evidence path %s is not a directory this reader can open", named)
	}
	return root, dir, nil
}

// EvidenceList is what the evidence path holds.
func (o Owner) EvidenceList(named string) (EvidenceListing, error) {
	root, dir, err := o.openEvidence(named)
	if err != nil {
		return EvidenceListing{}, err
	}
	defer func() { _ = root.Close() }()
	entries := []EvidenceEntry{}
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.Type().IsRegular() {
			// A link is not listed: what it names may lie outside the path.
			return nil
		}
		kind, _, known := evidenceKind(name)
		if !known {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		entries = append(entries, EvidenceEntry{Path: name, Kind: kind, Size: info.Size()})
		return nil
	})
	if err != nil {
		return EvidenceListing{}, fmt.Errorf("cannot list the evidence at %s: %w", dir, err)
	}
	sort.Slice(entries, func(a, b int) bool { return entries[a].Path < entries[b].Path })
	total := len(entries)
	if len(entries) > MaxEvidenceEntries {
		entries = entries[:MaxEvidenceEntries]
	}
	return EvidenceListing{Root: named, Entries: entries, Supplied: len(entries), Total: total}, nil
}

// evidenceKind is a file's kind and content type by its extension.
func evidenceKind(name string) (string, string, bool) {
	contentType, known := evidenceTypes[strings.ToLower(path.Ext(name))]
	if !known {
		return "", "", false
	}
	if contentType == "" {
		return EvidenceText, "", true
	}
	return EvidenceImage, contentType, true
}

// EvidenceFile is one file under the evidence path, by its evidence-relative
// path: an image, or text within the bounds. Anything else is refused in words.
func (o Owner) EvidenceFile(named, file string) (EvidenceFile, error) {
	clean, err := insideOf(file, "the evidence")
	if err != nil {
		return EvidenceFile{}, err
	}
	kind, contentType, known := evidenceKind(clean)
	if !known {
		return EvidenceFile{}, refused("%s is neither an image nor text, and the evidence read serves images and text", clean)
	}
	root, _, err := o.openEvidence(named)
	if err != nil {
		return EvidenceFile{}, err
	}
	defer func() { _ = root.Close() }()
	opened, err := root.Open(clean)
	if err != nil {
		return EvidenceFile{}, refused("%s is not a file of the evidence", clean)
	}
	defer func() { _ = opened.Close() }()
	info, err := opened.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return EvidenceFile{}, refused("%s is not a file of the evidence", clean)
	}
	body, err := io.ReadAll(io.LimitReader(opened, MaxEvidenceBytes+1))
	if err != nil {
		return EvidenceFile{}, fmt.Errorf("cannot read %s: %w", clean, err)
	}
	if len(body) > MaxEvidenceBytes {
		return EvidenceFile{}, refused("%s is larger than the evidence read serves", clean)
	}
	if kind == EvidenceImage {
		return EvidenceFile{Path: clean, Kind: kind, Type: contentType, Body: body}, nil
	}
	if !utf8.Valid(body) || binary(body) {
		return EvidenceFile{}, refused("%s is not UTF-8 text, and the evidence read shows text as it is", clean)
	}
	lines := strings.SplitAfter(strings.TrimSuffix(string(body), "\n"), "\n")
	total := len(lines)
	if len(lines) > MaxEvidenceLines {
		lines = lines[:MaxEvidenceLines]
	}
	text := strings.Join(lines, "")
	if !strings.HasSuffix(text, "\n") && strings.HasSuffix(string(body), "\n") || len(lines) < total {
		text = strings.TrimSuffix(text, "\n") + "\n"
	}
	return EvidenceFile{Path: clean, Kind: kind, Text: text, Blocks: markdown.Parse([]byte(text)).Blocks,
		Supplied: len(lines), Total: total}, nil
}

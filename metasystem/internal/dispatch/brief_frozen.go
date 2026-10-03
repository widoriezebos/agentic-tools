package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// A frozen input is a repository path a brief cites that the delegate's base
// tree does not hold yet: a seat's draft (a design page or its brief, written
// in the seat's checkout and not committed). The caller copies the draft's
// bytes into its runtime artifacts and names the copy on a brief line; the
// delegate reads the copy, never the moving checkout. Admission accepts such a
// path only when the copy lies inside the dispatcher's checkout and holds
// exactly the recorded SHA-256, so every cited path still names bytes the
// delegate can read.

// briefFrozenPrefix opens a brief line naming one frozen input.
const briefFrozenPrefix = "Frozen input: "

// FrozenInputLine is the brief line naming a frozen copy of a repository
// path: the path, the SHA-256 of the bytes frozen, and the copy's absolute
// path.
func FrozenInputLine(path, sha256Hex, copyPath string) string {
	return briefFrozenPrefix + path + " sha256:" + sha256Hex + " " + copyPath
}

type frozenInput struct{ digest, copy string }

// briefFrozenInputs reads the brief's frozen-input lines by cited path. A
// malformed line names nothing, so its path stays unadmitted.
func briefFrozenInputs(data []byte) map[string][]frozenInput {
	inputs := map[string][]frozenInput{}
	for _, line := range strings.Split(string(data), "\n") {
		rest, found := strings.CutPrefix(line, briefFrozenPrefix)
		if !found {
			continue
		}
		name, rest, ok := strings.Cut(rest, " sha256:")
		if !ok || name == "" || strings.ContainsAny(name, " \t") {
			continue
		}
		digest, copyPath, ok := strings.Cut(rest, " ")
		if !ok || len(digest) != sha256.Size*2 || copyPath == "" {
			continue
		}
		inputs[name] = append(inputs[name], frozenInput{digest: digest, copy: strings.TrimRight(copyPath, " \t\r")})
	}
	return inputs
}

// frozenInputHolds reports whether one of the inputs is an absolute regular
// file inside diskRoot whose bytes have the recorded SHA-256. Any failure to
// prove that is a no.
func frozenInputHolds(inputs []frozenInput, diskRoot string) bool {
	root, err := filepath.EvalSymlinks(diskRoot)
	if err != nil {
		return false
	}
	for _, input := range inputs {
		if !filepath.IsAbs(input.copy) {
			continue
		}
		resolved, err := filepath.EvalSymlinks(input.copy)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(root, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			continue
		}
		info, err := os.Lstat(resolved)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		content, err := os.ReadFile(resolved)
		if err != nil {
			continue
		}
		digest := sha256.Sum256(content)
		if hex.EncodeToString(digest[:]) == input.digest {
			return true
		}
	}
	return false
}

// BriefDraft is a path a brief cites that the delegate's tree lacks, with
// the bytes a checkout holds for it.
type BriefDraft struct {
	Path    string
	Content []byte
}

// BriefDraftsHeld returns the drafts text cites, read as admission reads it
// for a delegate dispatched from installation root, that one of the
// checkouts (installation folders, in order) holds as a regular file: at
// the cited path from the checkout's repository top, else under the
// installation folder itself, as a brief written from the installation
// names it. The first checkout holding a path gives its bytes. A cited path
// no checkout holds is left out, so admission still refuses it.
func BriefDraftsHeld(text []byte, root string, checkouts ...string) ([]BriefDraft, error) {
	top, err := gitOutput(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	prefix, err := gitBriefTreeFacts{}.InstallPrefix(root)
	if err != nil {
		return nil, err
	}
	cited, err := briefDraftPaths(text, top, prefix)
	if err != nil {
		return nil, err
	}
	var drafts []BriefDraft
	for _, rel := range cited {
		if content, found := draftHeld(rel, checkouts); found {
			drafts = append(drafts, BriefDraft{Path: rel, Content: content})
		}
	}
	return drafts, nil
}

func draftHeld(rel string, checkouts []string) ([]byte, bool) {
	for _, checkout := range checkouts {
		if checkout == "" {
			continue
		}
		var locations []string
		if top, err := gitOutput(checkout, "rev-parse", "--show-toplevel"); err == nil {
			locations = append(locations, filepath.Join(top, filepath.FromSlash(rel)))
		}
		locations = append(locations, filepath.Join(checkout, filepath.FromSlash(rel)))
		for _, path := range locations {
			if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
				continue
			}
			if content, err := os.ReadFile(path); err == nil {
				return content, true
			}
		}
	}
	return nil, false
}

// BriefDraftPaths returns the repository paths text cites, by the same
// reading admission uses, that the commit at root's HEAD does not hold
// (runtime artifact paths excluded): the drafts a caller must freeze before
// a delegate can be admitted to read them.
func BriefDraftPaths(text []byte, root string) ([]string, error) {
	prefix, err := gitBriefTreeFacts{}.InstallPrefix(root)
	if err != nil {
		return nil, err
	}
	return briefDraftPaths(text, root, prefix)
}

// briefDraftPaths is BriefDraftPaths for root's repository read from an
// installation folder prefix: a path cited from the installation that the
// tree holds under the prefix is no draft.
func briefDraftPaths(text []byte, root, prefix string) ([]string, error) {
	facts := gitBriefTreeFacts{}
	bounds, err := parseBriefBounds(scanBriefHeaders(text), func() (string, error) { return prefix, nil })
	if err != nil {
		return nil, err
	}
	commit, err := facts.BaseCommit(root)
	if err != nil {
		return nil, err
	}
	top, err := facts.Directories(root, commit)
	if err != nil {
		return nil, err
	}
	nested := map[string]bool{}
	if top["metasystem"] {
		if nested, err = facts.Directories(root, commit+":metasystem"); err != nil {
			return nil, err
		}
	}
	var drafts []string
	for _, candidate := range extractBriefAuthorityPaths(string(text), bounds, top, nested) {
		if artifactAuthorityPath(candidate) {
			continue
		}
		if present, _ := treeHolds(facts, root, []string{commit}, candidate, prefix); !present {
			drafts = append(drafts, candidate)
		}
	}
	return drafts, nil
}

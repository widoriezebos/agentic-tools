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

// BriefDraftPaths returns the repository paths text cites, by the same
// reading admission uses, that the commit at root's HEAD does not hold
// (runtime artifact paths excluded): the drafts a caller must freeze before
// a delegate can be admitted to read them.
func BriefDraftPaths(text []byte, root string) ([]string, error) {
	facts := gitBriefTreeFacts{}
	bounds, err := parseBriefBounds(scanBriefHeaders(text), func() (string, error) { return facts.InstallPrefix(root) })
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
		if present, _ := facts.HasPath(root, commit, candidate); !present {
			drafts = append(drafts, candidate)
		}
	}
	return drafts, nil
}

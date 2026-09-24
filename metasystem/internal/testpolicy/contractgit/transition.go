package contractgit

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
)

// PreflightCommitAttributes prevents an incoming commit from assigning the
// testing-contract driver to an ordinary path. Both the current tree and the
// tree with the commit's own attribute-file changes are checked.
func PreflightCommitAttributes(repo, before, commit string) error {
	return PreflightCommitAttributesWith(repo, before, commit, DefaultCommitAccess())
}

func PreflightCommitAttributesWith(repo, before, commit string, a CommitAccess) error {
	paths, err := commitPaths(a, repo, commit+"^", commit, "")
	if err != nil || len(paths) == 0 {
		return err
	}
	if err := refuseMisusedAttributeWith(a, repo, before, commit, paths); err != nil {
		return err
	}
	overlay, cleanup, err := attributeOverlayWith(a, repo, before, commit, paths)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return err
	}
	return refuseMisusedAttributeWith(a, repo, overlay, commit, paths)
}

// PreflightPatchAttributes applies only attribute-file hunks to an isolated
// index, then checks every path named by the full-index patch against the
// attributes before and after those hunks.
func PreflightPatchAttributes(repo, before, label string, patch []byte) error {
	paths, err := patchPaths(repo, patch)
	if err != nil || len(paths) == 0 {
		return err
	}
	if err := refuseMisusedAttribute(repo, before, label, paths); err != nil {
		return err
	}
	var attributes []string
	for _, path := range paths {
		if filepath.Base(path) == ".gitattributes" {
			attributes = append(attributes, path)
		}
	}
	if len(attributes) == 0 {
		return nil
	}
	overlay, cleanup, err := patchAttributeOverlay(repo, before, patch, attributes)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return err
	}
	return refuseMisusedAttribute(repo, overlay, label, paths)
}

// CheckCommitContract requires the applied contract bytes to be the semantic
// merge of the source parent, the pre-apply tree, and the source commit.
func CheckCommitContract(repo, before, after, commit, label string) error {
	return CheckCommitContractWith(repo, before, after, commit, label, DefaultCommitAccess())
}

func CheckCommitContractWith(repo, before, after, commit, label string, a CommitAccess) error {
	paths, err := commitPaths(a, repo, commit+"^", commit, TestingContractPath)
	changed := len(paths) != 0
	if err != nil || !changed {
		return err
	}
	base, err := a.File(repo, commit+"^", TestingContractPath)
	if err != nil {
		return err
	}
	theirs, err := a.File(repo, commit, TestingContractPath)
	if err != nil {
		return err
	}
	return checkContractBytesWith(a.File, repo, before, after, base, theirs, label)
}

// CheckPatchContract applies the same invariant to a full-index patch. The
// old and new blob ids are the source transition's authoritative endpoints.
func CheckPatchContract(repo, before, after string, patch []byte, label string) error {
	base, theirs, present, err := patchContractBlobs(patch)
	if err != nil || !present {
		return err
	}
	return checkContractBlobs(repo, before, after, base, theirs, label)
}

func checkContractBlobs(repo, before, after, baseBlob, theirsBlob, label string) error {
	base, err := runGit(repo, nil, "cat-file", "blob", baseBlob)
	if err != nil {
		return err
	}
	theirs, err := runGit(repo, nil, "cat-file", "blob", theirsBlob)
	if err != nil {
		return err
	}
	return checkContractBytes(repo, before, after, base, theirs, label)
}

func checkContractBytes(repo, before, after string, base, theirs []byte, label string) error {
	return checkContractBytesWith(treeFile, repo, before, after, base, theirs, label)
}

func checkContractBytesWith(file func(string, string, string) ([]byte, error), repo, before, after string, base, theirs []byte, label string) error {
	ours, err := file(repo, before, TestingContractPath)
	if err != nil {
		return err
	}
	actual, err := file(repo, after, TestingContractPath)
	if err != nil {
		return err
	}
	want, err := contractmerge.MergeBytes(base, ours, theirs)
	if err != nil {
		return err
	}
	if bytes.Equal(actual, want) {
		return nil
	}
	offset := firstDifference(actual, want)
	return &Refusal{Code: ContractHandMergedCode, Detail: fmt.Sprintf("%s differs from the semantic testing contract merge at byte %d", label, offset)}
}

func firstDifference(left, right []byte) int {
	limit := min(len(left), len(right))
	for i := 0; i < limit; i++ {
		if left[i] != right[i] {
			return i
		}
	}
	return limit
}

func patchContractBlobs(patch []byte) (string, string, bool, error) {
	lines := bytes.Split(patch, []byte{'\n'})
	wanted := []byte("diff --git a/" + TestingContractPath + " b/" + TestingContractPath)
	for i, line := range lines {
		if !bytes.Equal(line, wanted) {
			continue
		}
		for _, header := range lines[i+1:] {
			if bytes.HasPrefix(header, []byte("diff --git ")) {
				break
			}
			if !bytes.HasPrefix(header, []byte("index ")) {
				continue
			}
			pair := strings.Fields(strings.TrimPrefix(string(header), "index "))
			if len(pair) == 0 {
				break
			}
			base, theirs, ok := strings.Cut(pair[0], "..")
			if !ok || len(base) != 40 || len(theirs) != 40 || strings.Trim(base, "0") == "" || strings.Trim(theirs, "0") == "" {
				return "", "", false, fmt.Errorf("testing contract patch lacks full nonzero blob ids")
			}
			return base, theirs, true, nil
		}
		return "", "", false, fmt.Errorf("testing contract patch lacks an index line")
	}
	return "", "", false, nil
}

func patchPaths(repo string, patch []byte) ([]string, error) {
	out, err := runGitInput(repo, nil, patch, "apply", "--numstat", "-z", "-")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, record := range bytes.Split(out, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		fields := bytes.SplitN(record, []byte{'\t'}, 3)
		if len(fields) != 3 || len(fields[2]) == 0 {
			return nil, fmt.Errorf("full-index patch has a malformed numstat record")
		}
		paths = append(paths, string(fields[2]))
	}
	return paths, nil
}

func refuseMisusedAttribute(repo, tree, commit string, paths []string) error {
	return refuseMisusedAttributeWith(DefaultCommitAccess(), repo, tree, commit, paths)
}

func refuseMisusedAttributeWith(a CommitAccess, repo, tree, commit string, paths []string) error {
	out, err := a.Attributes(repo, tree, paths)
	if err != nil {
		return err
	}
	fields := bytes.Split(out, []byte{0})
	for i := 0; i+2 < len(fields); i += 3 {
		path, value := string(fields[i]), string(fields[i+2])
		if value == testingContractMergeDriver && path != TestingContractPath {
			return &Refusal{Code: AttributeMisuseCode, Detail: fmt.Sprintf("path %s selects %s in commit %s", path, testingContractMergeDriver, commit)}
		}
	}
	return nil
}

func attributeOverlayWith(a CommitAccess, repo, before, commit string, paths []string) (string, func(), error) {
	name, cleanup, err := a.OpenAttributeIndex()
	if err != nil {
		return "", nil, err
	}
	if err := a.LoadAttributeBase(repo, name, before); err != nil {
		return "", cleanup, err
	}
	for _, path := range paths {
		if filepath.Base(path) != ".gitattributes" {
			continue
		}
		entry, err := a.AttributeEntry(repo, name, commit, path)
		if err != nil {
			return "", cleanup, err
		}
		if len(entry) == 0 {
			if err := a.DropAttribute(repo, name, path); err != nil {
				return "", cleanup, err
			}
			continue
		}
		meta, _, ok := bytes.Cut(bytes.TrimSuffix(entry, []byte{0}), []byte{'\t'})
		parts := strings.Fields(string(meta))
		if !ok || len(parts) != 3 || parts[1] != "blob" {
			return "", cleanup, fmt.Errorf("attribute entry for %s is malformed", path)
		}
		if _, err := strconv.ParseUint(parts[0], 8, 32); err != nil {
			return "", cleanup, fmt.Errorf("attribute mode for %s is malformed", path)
		}
		if err := a.SetAttribute(repo, name, parts[0], parts[2], path); err != nil {
			return "", cleanup, err
		}
	}
	tree, err := a.AttributeTree(repo, name)
	return tree, cleanup, err
}

func patchAttributeOverlay(repo, before string, patch []byte, paths []string) (string, func(), error) {
	name, cleanup, err := temporaryIndex()
	if err != nil {
		return "", nil, err
	}
	env := []string{"GIT_INDEX_FILE=" + name}
	if _, err := runGit(repo, env, "read-tree", before); err != nil {
		return "", cleanup, err
	}
	args := []string{"apply", "--cached", "--whitespace=nowarn"}
	for _, path := range paths {
		args = append(args, "--include="+path)
	}
	args = append(args, "-")
	if _, err := runGitInput(repo, env, patch, args...); err != nil {
		return "", cleanup, err
	}
	out, err := runGit(repo, env, "write-tree")
	return strings.TrimSpace(string(out)), cleanup, err
}

func temporaryIndex() (string, func(), error) {
	index, err := os.CreateTemp("", "metasystem-testing-attrs-*.index")
	if err != nil {
		return "", nil, err
	}
	name := index.Name()
	if err := index.Close(); err != nil {
		_ = os.Remove(name)
		return "", nil, err
	}
	if err := os.Remove(name); err != nil && !os.IsNotExist(err) {
		return "", nil, err
	}
	cleanup := func() { _ = os.Remove(name) }
	return name, cleanup, nil
}

func treeFile(repo, tree, path string) ([]byte, error) {
	return runGit(repo, nil, "show", tree+":"+path)
}

func runGit(repo string, extraEnv []string, args ...string) ([]byte, error) {
	return runGitInput(repo, extraEnv, nil, args...)
}

func runGitInput(repo string, extraEnv []string, input []byte, args ...string) ([]byte, error) {
	full := append([]string{"-C", repo, "-c", "core.useReplaceRefs=false", "-c", "core.hooksPath=/dev/null"}, args...)
	command := exec.Command("git", full...)
	command.Env = scrubbedEnv(extraEnv...)
	command.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return out, nil
}

func scrubbedEnv(extra ...string) []string {
	out := make([]string, 0, len(os.Environ())+len(extra))
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "GIT_CONFIG") || name == "GIT_DIR" || name == "GIT_WORK_TREE" || name == "GIT_INDEX_FILE" {
			continue
		}
		out = append(out, entry)
	}
	return append(out, extra...)
}

func IsRefusal(err error) bool {
	var refusal *Refusal
	return errors.As(err, &refusal)
}

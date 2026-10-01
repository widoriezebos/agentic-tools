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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
)

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

func temporaryIndex() (string, func(), error) {
	index, done, err := diskstore.ScratchFile("metasystem-testing-attrs-*.index")
	if err != nil {
		return "", nil, err
	}
	name := index.Name()
	if err := index.Close(); err != nil {
		done()
		return "", nil, err
	}
	if err := os.Remove(name); err != nil && !os.IsNotExist(err) {
		done()
		return "", nil, err
	}
	return name, done, nil
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

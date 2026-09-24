package branch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

type Entry struct {
	SrcMode, DstMode, SrcBlob, DstBlob, Status, Path string
}

func gitOutput(repo string, args ...string) ([]byte, error) {
	full, err := branchGitCommand(repo, args...)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("git", full...)
	cmd.Env = gittree.ScrubbedEnviron("LC_ALL=C")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return out, nil
}

func rawEntries(repo, commit string) ([]byte, error) {
	return rawEntriesWithGit(repo, commit, gitOutput)
}

func rawEntriesWithGit(repo, commit string, gitRead func(string, ...string) ([]byte, error)) ([]byte, error) {
	parents, err := gitRead(repo, "rev-list", "--parents", "-n", "1", commit)
	if err != nil {
		return nil, err
	}
	if fields := strings.Fields(string(parents)); len(fields) < 2 {
		return nil, refuse(commit, "the unit digest requires a parent")
	}
	return gitRead(repo, "diff-tree", "-r", "-z", "--no-renames", "--full-index", commit+"^", commit)
}

func UnitDigest(repo, commit string) (string, error) {
	return unitDigestWithGit(repo, commit, gitOutput)
}

func unitDigestWithGit(repo, commit string, gitRead func(string, ...string) ([]byte, error)) (string, error) {
	raw, err := rawEntriesWithGit(repo, commit, gitRead)
	if err != nil {
		return "", err
	}
	return digestRawEntries(raw), nil
}

func digestRawEntries(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func RawEntries(repo, commit string) ([]Entry, error) {
	return rawEntriesWithGitParsed(repo, commit, gitOutput)
}

func RawEntriesWithRaw(repo, commit string, read func(string, ...string) ([]byte, error)) ([]Entry, error) {
	return rawEntriesWithGitParsed(repo, commit, read)
}

func UnitDigestWithRaw(repo, commit string, read func(string, ...string) ([]byte, error)) (string, error) {
	return unitDigestWithGit(repo, commit, read)
}

func rawEntriesWithGitParsed(repo, commit string, gitRead func(string, ...string) ([]byte, error)) ([]Entry, error) {
	raw, err := rawEntriesWithGit(repo, commit, gitRead)
	if err != nil {
		return nil, err
	}
	return parseRawEntries(commit, raw)
}

func parseRawEntries(commit string, raw []byte) ([]Entry, error) {
	parts, entries := bytes.Split(raw, []byte{0}), []Entry{}
	for i := 0; i+1 < len(parts) && len(parts[i]) > 0; i += 2 {
		fields := strings.Fields(string(parts[i]))
		if len(fields) != 5 || !strings.HasPrefix(fields[0], ":") {
			return nil, refuse(commit, "git returned a malformed raw tree entry")
		}
		entries = append(entries, Entry{strings.TrimPrefix(fields[0], ":"), fields[1], fields[2], fields[3], fields[4], string(parts[i+1])})
	}
	return entries, nil
}

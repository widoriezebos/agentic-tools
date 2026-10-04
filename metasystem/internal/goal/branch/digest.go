package branch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type Entry struct {
	SrcMode, DstMode, SrcBlob, DstBlob, Status, Path string
}

func gitOutput(repo string, args ...string) ([]byte, error) {
	return gitOutputContext(context.Background(), repo, args...)
}

// gitOutputContext is gitOutput bound to ctx: the command is killed when ctx
// ends, so a stalled git never outlives its caller's budget.
func gitOutputContext(ctx context.Context, repo string, args ...string) ([]byte, error) {
	full, err := branchGitCommand(repo, args...)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "git", full...)
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
		return nil, rangeRefusal("", commit, "a build commit needs a parent to compare against, and this one has none")
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

// changeDigestWithReads keeps a build's edits and nearby context while ignoring
// base-dependent locations, blob names and declared generated outputs.
func changeDigestWithReads(r attestationReads, repo, commit string) (string, error) {
	patch, err := r.ChangePatch(repo, commit)
	if err != nil {
		return "", err
	}
	prefix, err := r.Prefix(repo)
	if err != nil {
		return "", err
	}
	entry, err := r.TreeEntry(repo, commit, "testing.json")
	if err != nil {
		return "", err
	}
	var policy struct {
		Generated []testpolicy.Generated `json:"generated"`
	}
	if entry != "" {
		data, err := r.SnapshotFile(repo, commit, prefix+"testing.json")
		if err != nil {
			return "", err
		}
		if err := json.Unmarshal(data, &policy); err != nil {
			return "", fmt.Errorf("cannot read generated files in testing.json at %s:\n%w", commit, err)
		}
	}
	normalized, err := normalizeChangePatch(patch, func(path string) bool {
		return conflict.GeneratedBy(policy.Generated, prefix, path)
	})
	if err != nil {
		return "", err
	}
	return digestRawEntries(normalized), nil
}

func normalizeChangePatch(patch []byte, generated func(string) bool) ([]byte, error) {
	var kept bytes.Buffer
	skip, header := false, false
	for _, line := range bytes.SplitAfter(patch, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("diff --git ")) {
			paths := strings.TrimSuffix(string(line[len("diff --git "):]), "\n")
			half := len(paths) / 2
			if len(paths)%2 != 1 || paths[half] != ' ' {
				return nil, fmt.Errorf("git listed a changed file in a form this engine can't read")
			}
			path := paths[:half]
			if strings.HasPrefix(path, `"`) {
				var err error
				path, err = strconv.Unquote(path)
				if err != nil {
					return nil, fmt.Errorf("cannot read a changed file's path: %w", err)
				}
			}
			if !strings.HasPrefix(path, "a/") {
				return nil, fmt.Errorf("git listed a changed file without its source prefix")
			}
			skip, header = generated(strings.TrimPrefix(path, "a/")), true
		} else if header {
			if bytes.HasPrefix(line, []byte("index ")) {
				continue
			}
			if bytes.HasPrefix(line, []byte("--- ")) || bytes.HasPrefix(line, []byte("+++ ")) || bytes.HasPrefix(line, []byte("GIT binary patch")) {
				header = false
			}
		}
		if skip {
			continue
		}
		if bytes.HasPrefix(line, []byte("@@ ")) {
			kept.WriteString("@@")
			if bytes.HasSuffix(line, []byte("\n")) {
				kept.WriteByte('\n')
			}
		} else {
			kept.Write(line)
		}
	}
	return kept.Bytes(), nil
}

func RawEntries(repo, commit string) ([]Entry, error) {
	return rawEntriesWithGitParsed(repo, commit, gitOutput)
}

func RawEntriesWithRaw(repo, commit string, read func(string, ...string) ([]byte, error)) ([]Entry, error) {
	return rawEntriesWithGitParsed(repo, commit, read)
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
			return nil, rangeRefusal("", commit, "git listed its changed files in a form this engine can't read")
		}
		entries = append(entries, Entry{strings.TrimPrefix(fields[0], ":"), fields[1], fields[2], fields[3], fields[4], string(parts[i+1])})
	}
	return entries, nil
}

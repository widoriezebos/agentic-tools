package proofrun

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

type FrozenExport struct {
	Digest       string
	Root         string
	ProjectRoot  string
	SnapshotRoot string
}

type frozenCustody struct {
	SchemaVersion int    `json:"schemaVersion"`
	OwnerRoot     string `json:"ownerRoot"`
	SnapshotRoot  string `json:"snapshotRoot"`
	ExecutionRoot string `json:"executionRoot"`
}

const frozenCustodyName = "frozen-export-custody.json"

// Freeze exports one manifest projection and publishes it only when both the
// exported projection and a fresh read of the source still equal the initial
// digest. The gate can therefore use the export without consulting live bytes.
func Freeze(root string) (result FrozenExport, err error) {
	return freezeWithHook(root, nil)
}

func FreezeCandidate(root, candidateTree string) (result FrozenExport, err error) {
	return freezeCandidateWithHookAt(root, candidateTree, "", nil)
}

func freezeWithHook(root string, afterExport func(projectRoot string)) (result FrozenExport, err error) {
	return freezeCandidateWithHookAt(root, "", "", afterExport)
}

func freezeWithHookAt(root, temporaryRoot string, afterExport func(projectRoot string)) (result FrozenExport, err error) {
	return freezeCandidateWithHookAt(root, "", temporaryRoot, afterExport)
}

func freezeCandidateWithHookAt(root, candidateTree, temporaryRoot string, afterExport func(projectRoot string)) (result FrozenExport, err error) {
	before, location, err := readCompleteManifest(root)
	if err != nil {
		return FrozenExport{}, err
	}
	rawParent, err := os.MkdirTemp(temporaryRoot, "metasystem-witness-freeze-")
	if err != nil {
		return FrozenExport{}, fmt.Errorf("create private frozen-export directory: %w", err)
	}
	parent, err := filepath.EvalSymlinks(rawParent)
	if err != nil {
		_ = os.RemoveAll(rawParent)
		return FrozenExport{}, fmt.Errorf("resolve private frozen-export directory: %w", err)
	}
	if err := os.Chmod(parent, 0700); err != nil {
		os.RemoveAll(parent)
		return FrozenExport{}, fmt.Errorf("protect frozen-export directory: %w", err)
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(parent)
		}
	}()

	snapshotRoot := filepath.Join(parent, "tree")
	if location.hasGit {
		snapshotRoot = filepath.Join(parent, "project")
		if err := cloneFrozenProject(location.projectRoot, snapshotRoot, parent); err != nil {
			return FrozenExport{}, err
		}
	} else if err := os.Mkdir(snapshotRoot, 0700); err != nil {
		return FrozenExport{}, fmt.Errorf("create frozen-export tree: %w", err)
	}
	if err := exportEntries(location.projectRoot, snapshotRoot, before.entries); err != nil {
		return FrozenExport{}, err
	}
	if candidateTree != "" && !location.hasGit {
		return FrozenExport{}, fmt.Errorf("freeze candidate requires a Git project")
	}
	if location.hasGit {
		if err := commitFrozenProject(snapshotRoot, filepath.Join(root, "metasystem.conf")); err != nil {
			return FrozenExport{}, err
		}
	}
	executionRoot := snapshotRoot
	if location.installationPrefix != "" {
		executionRoot = filepath.Join(snapshotRoot, filepath.FromSlash(location.installationPrefix))
	}
	if candidateTree != "" {
		if err := (gittree.Workspace{Dir: root}).TransferTreeClosure(candidateTree, gittree.Workspace{Dir: executionRoot}); err != nil {
			return FrozenExport{}, fmt.Errorf("transfer frozen candidate tree: %w", err)
		}
		if err := applyCandidateTree(executionRoot, snapshotRoot, candidateTree); err != nil {
			return FrozenExport{}, err
		}
	}
	if afterExport != nil {
		afterExport(snapshotRoot)
	}
	exported, _, err := readCompleteManifest(executionRoot)
	if err != nil {
		return FrozenExport{}, fmt.Errorf("verify frozen export: %w", err)
	}
	after, _, err := readCompleteManifest(root)
	if err != nil {
		return FrozenExport{}, fmt.Errorf("re-read source after frozen export: %w", err)
	}
	beforeDigest, exportedDigest, afterDigest := before.fullDigest(), exported.fullDigest(), after.fullDigest()
	if beforeDigest != exportedDigest || beforeDigest != afterDigest {
		return FrozenExport{}, fmt.Errorf("frozen export voided because the source changed while it was copied: before %s, export %s, after %s",
			beforeDigest, exportedDigest, afterDigest)
	}
	if err := writeFrozenCustody(parent, snapshotRoot, executionRoot); err != nil {
		return FrozenExport{}, err
	}
	published = true
	return FrozenExport{Digest: beforeDigest, Root: executionRoot, ProjectRoot: snapshotRoot, SnapshotRoot: snapshotRoot}, nil
}

func applyCandidateTree(executionRoot, snapshotRoot, candidateTree string) error {
	if len(candidateTree) != 40 {
		return fmt.Errorf("freeze candidate tree is malformed")
	}
	configurationPath := filepath.Join(executionRoot, "metasystem.conf")
	paths, err := runFreezeGitRaw(executionRoot, configurationPath,
		"ls-tree", "-r", "--name-only", "-z", candidateTree)
	if err != nil {
		return fmt.Errorf("inspect frozen candidate tree: %w", err)
	}
	for _, rawName := range bytes.Split(paths, []byte{0}) {
		if name := string(rawName); name != "" && isLocalConfiguration(filepath.ToSlash(name)) {
			return fmt.Errorf("freeze candidate tree contains local configuration at %s", name)
		}
	}
	wholeTree, err := (gittree.Workspace{Dir: executionRoot}).GraftProjectTree("HEAD", candidateTree)
	if err != nil {
		return fmt.Errorf("graft frozen candidate tree: %w", err)
	}
	if _, err := runFreezeGit(snapshotRoot, configurationPath, "read-tree", "--reset", "-u", wholeTree); err != nil {
		return fmt.Errorf("checkout frozen candidate tree: %w", err)
	}
	return commitFrozenProject(snapshotRoot, configurationPath)
}

func (f *FrozenExport) Close() error {
	if f == nil || f.SnapshotRoot == "" {
		return nil
	}
	err := CleanupFrozenExport(f.SnapshotRoot)
	f.SnapshotRoot = ""
	return err
}

func CleanupFrozenExport(snapshotRoot string) error {
	canonical, err := filepath.Abs(snapshotRoot)
	if err == nil {
		canonical, err = filepath.EvalSymlinks(canonical)
	}
	if err != nil {
		return fmt.Errorf("resolve frozen snapshot cleanup root: %w", err)
	}
	owner := filepath.Dir(canonical)
	temporaryRoot, tempErr := filepath.EvalSymlinks(os.TempDir())
	ownerInfo, ownerErr := os.Lstat(owner)
	if tempErr != nil || filepath.Dir(owner) != temporaryRoot ||
		!strings.HasPrefix(filepath.Base(owner), "metasystem-witness-freeze-") ||
		(filepath.Base(canonical) != "project" && filepath.Base(canonical) != "tree") ||
		ownerErr != nil || !ownerInfo.IsDir() || ownerInfo.Mode().Perm() != 0o700 {
		return fmt.Errorf("frozen snapshot cleanup root is outside private custody: %s", canonical)
	}
	marker := filepath.Join(owner, frozenCustodyName)
	info, err := os.Lstat(marker)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return fmt.Errorf("frozen snapshot cleanup custody is absent or not a regular file: %s", marker)
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		return fmt.Errorf("read frozen snapshot cleanup custody: %w", err)
	}
	var custody frozenCustody
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&custody); err != nil || custody.SchemaVersion != 1 ||
		custody.OwnerRoot != owner || custody.SnapshotRoot != canonical ||
		(custody.ExecutionRoot != canonical && !strings.HasPrefix(custody.ExecutionRoot, canonical+string(filepath.Separator))) {
		return fmt.Errorf("frozen snapshot cleanup custody does not bind %s", canonical)
	}
	if err := os.RemoveAll(owner); err != nil {
		return fmt.Errorf("remove frozen snapshot %s: %w", owner, err)
	}
	return nil
}

func cloneFrozenProject(sourceRoot, snapshotRoot, ownerRoot string) error {
	origin := filepath.Join(ownerRoot, "origin.git")
	if _, err := runFreezeGit(sourceRoot, filepath.Join(sourceRoot, "metasystem.conf"),
		"clone", "-q", "--bare", "--no-local", "--", sourceRoot, origin); err != nil {
		return fmt.Errorf("create private frozen origin: %w", err)
	}
	if _, err := runFreezeGit(ownerRoot, filepath.Join(sourceRoot, "metasystem.conf"),
		"clone", "-q", "--no-checkout", "--", origin, snapshotRoot); err != nil {
		return fmt.Errorf("create private frozen project: %w", err)
	}
	return nil
}

func commitFrozenProject(snapshotRoot, configurationPath string) error {
	if _, err := runFreezeGit(snapshotRoot, configurationPath, "read-tree", "HEAD"); err != nil {
		return fmt.Errorf("seed private frozen project index: %w", err)
	}
	if _, err := runFreezeGit(snapshotRoot, configurationPath, "add", "-A", "-f", "--", "."); err != nil {
		return fmt.Errorf("capture private frozen project inputs: %w", err)
	}
	tree, err := runFreezeGit(snapshotRoot, configurationPath, "write-tree")
	if err != nil {
		return fmt.Errorf("write private frozen project tree: %w", err)
	}
	base, err := runFreezeGit(snapshotRoot, configurationPath, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return fmt.Errorf("resolve private frozen project base: %w", err)
	}
	commit, err := runFreezeGit(snapshotRoot, configurationPath,
		"-c", "user.name=MetaSystem", "-c", "user.email=metasystem@invalid",
		"commit-tree", strings.TrimSpace(tree), "-p", strings.TrimSpace(base), "-m", "frozen project inputs")
	if err != nil {
		return fmt.Errorf("commit private frozen project inputs: %w", err)
	}
	if _, err := runFreezeGit(snapshotRoot, configurationPath, "update-ref", "--no-deref", "HEAD", strings.TrimSpace(commit), strings.TrimSpace(base)); err != nil {
		return fmt.Errorf("bind private frozen project HEAD: %w", err)
	}
	if _, err := runFreezeGit(snapshotRoot, configurationPath, "diff-index", "--quiet", "HEAD", "--"); err != nil {
		return fmt.Errorf("private frozen project differs from its candidate HEAD: %w", err)
	}
	return nil
}

func runFreezeGit(directory, configurationPath string, args ...string) (string, error) {
	stdout, err := runFreezeGitRaw(directory, configurationPath, args...)
	return strings.TrimSpace(string(stdout)), err
}

func runFreezeGitRaw(directory, configurationPath string, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", directory, "-c", "core.hooksPath=/dev/null", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := boundedexec.Run(command, boundedexec.Timeout(configurationPath, boundedexec.Local), "git "+strings.Join(args, " ")); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("%s", detail)
	}
	return stdout.Bytes(), nil
}

func writeFrozenCustody(ownerRoot, snapshotRoot, executionRoot string) error {
	custody := frozenCustody{SchemaVersion: 1, OwnerRoot: ownerRoot, SnapshotRoot: snapshotRoot, ExecutionRoot: executionRoot}
	data, err := json.Marshal(custody)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(ownerRoot, frozenCustodyName), data, 0600); err != nil {
		return fmt.Errorf("write frozen snapshot custody: %w", err)
	}
	return nil
}

func exportEntries(sourceRoot, exportRoot string, entries []entry) error {
	directories := make([]entry, 0)
	for _, item := range entries {
		source := filepath.Join(sourceRoot, filepath.FromSlash(item.path))
		destination := filepath.Join(exportRoot, filepath.FromSlash(item.path))
		switch item.kind {
		case 'd':
			if err := os.Mkdir(destination, 0700); err != nil {
				return fmt.Errorf("export directory %q: %w", item.path, err)
			}
			directories = append(directories, item)
		case 'l':
			if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
				return fmt.Errorf("export symlink parent %q: %w", item.path, err)
			}
			target, err := os.Readlink(source)
			if err != nil {
				return fmt.Errorf("export symlink %q: %w", item.path, err)
			}
			if err := os.Symlink(target, destination); err != nil {
				return fmt.Errorf("write exported symlink %q: %w", item.path, err)
			}
		case 'f':
			// Directories are no longer manifest entries wherever the
			// projection skips their bare names; each file creates its
			// own parents.
			if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
				return fmt.Errorf("export file parent %q: %w", item.path, err)
			}
			if err := copyFile(source, destination, exportedMode(item)); err != nil {
				return fmt.Errorf("export file %q: %w", item.path, err)
			}
		default:
			return fmt.Errorf("export entry %q has unknown kind %q", item.path, item.kind)
		}
	}
	for i := len(directories) - 1; i >= 0; i-- {
		item := directories[i]
		if err := os.Chmod(filepath.Join(exportRoot, filepath.FromSlash(item.path)), exportedMode(item)); err != nil {
			return fmt.Errorf("set exported directory mode %q: %w", item.path, err)
		}
	}
	return nil
}

func exportedMode(item entry) os.FileMode {
	if item.kind == 'd' {
		if item.executable {
			return 0755
		}
		return 0644
	}
	if item.executable {
		return 0755
	}
	return 0644
}

func copyFile(source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

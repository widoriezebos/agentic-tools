package conflict

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

// Restore cannot remove an unmerged path missing from HEAD. Remove those
// paths from the index and checkout, and restore the paths main still has.
func TakeMain(git Git, checkout string, paths []string) error {
	listed, err := git(append([]string{"ls-tree", "-r", "--name-only", "-z", "HEAD", "--"}, paths...)...)
	if err != nil {
		return err
	}
	present := strings.FieldsFunc(listed, func(r rune) bool { return r == 0 })
	var missing []string
	for _, name := range paths {
		if !slices.Contains(present, name) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		if _, err := git(append([]string{"rm", "--cached", "--ignore-unmatch", "--"}, missing...)...); err != nil {
			return err
		}
		for _, name := range missing {
			if err := os.Remove(filepath.Join(checkout, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	if len(present) > 0 {
		_, err = git(append([]string{"restore", "--source=HEAD", "--staged", "--worktree", "--"}, present...)...)
	}
	return err
}

func GeneratedFiles(git Git, generated func(string) bool, conflicts []string) ([]string, error) {
	names := append([]string(nil), conflicts...)
	for _, args := range [][]string{{"ls-files", "-z"}, {"ls-files", "--others", "--exclude-standard", "-z"}} {
		listed, err := git(args...)
		if err != nil {
			return nil, err
		}
		for _, name := range strings.Split(listed, "\x00") {
			if name != "" && generated(name) && !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
	}
	return names, nil
}

// StageGenerated stages only declared outputs, including deleted tracked files.
func StageGenerated(git Git, generated func(string) bool) error {
	names, err := GeneratedFiles(git, generated, nil)
	if err != nil || len(names) == 0 {
		return err
	}
	_, err = git(append([]string{"add", "-A", "--"}, names...)...)
	return err
}

// RunRegenerationArgv runs literal arguments in a process group and records output.
func RunRegenerationArgv(argv []string, dir string, log *os.File, started func(int64) error) error {
	command := exec.Command(argv[0], argv[1:]...)
	command.Dir, command.Stdout, command.Stderr = dir, log, log
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return err
	}
	if err := started(int64(command.Process.Pid)); err != nil {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		_ = command.Wait()
		return err
	}
	return command.Wait()
}

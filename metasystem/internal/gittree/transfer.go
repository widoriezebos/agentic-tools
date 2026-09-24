package gittree

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// TransferTreeClosure copies one exact tree and every object reachable from it
// into another repository. The destination owns the resulting objects and does
// not depend on the source repository or an alternate object database.
func (w Workspace) TransferTreeClosure(tree string, destination Workspace) error {
	if !treeID.MatchString(tree) {
		return fmt.Errorf("gittree transfer: malformed tree")
	}
	objectType, err := w.gitLine(nil, "cat-file", "-t", tree)
	if err != nil || objectType != "tree" {
		return fmt.Errorf("gittree transfer: source object %s is not a tree", tree)
	}
	sourceTop, err := w.topLevel()
	if err != nil {
		return fmt.Errorf("gittree transfer: resolve source repository: %w", err)
	}
	destinationTop, err := destination.topLevel()
	if err != nil {
		return fmt.Errorf("gittree transfer: resolve destination repository: %w", err)
	}

	pack, err := os.CreateTemp(destination.Dir, ".metasystem-tree-closure-*")
	if err != nil {
		return fmt.Errorf("gittree transfer: create private pack: %w", err)
	}
	packPath := pack.Name()
	defer os.Remove(packPath)

	producer := w.runRaw(w.rawRequest(sourceTop, nil, []byte(tree+"\n"), "git pack-objects",
		"pack-objects", "--stdout", "--revs"), nil, pack)
	if w.RawSource != nil && producer.Err == nil && producer.ExitCode == 0 {
		if _, err := pack.Write(producer.Stdout); err != nil {
			_ = pack.Close()
			return fmt.Errorf("gittree transfer: create tree pack: %w", err)
		}
	}
	if producer.Err != nil || producer.ExitCode != 0 {
		_ = pack.Close()
		return fmt.Errorf("gittree transfer: create tree pack: %s", rawCommandError(producer))
	}
	if err := pack.Close(); err != nil {
		return fmt.Errorf("gittree transfer: close tree pack: %w", err)
	}

	pack, err = os.Open(packPath)
	if err != nil {
		return fmt.Errorf("gittree transfer: reopen tree pack: %w", err)
	}
	var packBytes []byte
	if destination.RawSource != nil {
		packBytes, err = io.ReadAll(pack)
		if err != nil {
			_ = pack.Close()
			return fmt.Errorf("gittree transfer: read tree pack: %w", err)
		}
	}
	consumer := destination.runRaw(destination.rawRequest(destinationTop, nil, packBytes, "git index-pack",
		"index-pack", "--stdin"), pack, nil)
	closeErr := pack.Close()
	if consumer.Err != nil || consumer.ExitCode != 0 {
		return fmt.Errorf("gittree transfer: import tree pack: %s", rawCommandError(consumer))
	}
	if closeErr != nil {
		return fmt.Errorf("gittree transfer: close imported tree pack: %w", closeErr)
	}
	if objectType, err := destination.gitLine(nil, "cat-file", "-t", tree); err != nil || objectType != "tree" {
		return fmt.Errorf("gittree transfer: destination did not receive tree %s (type=%q, cause=%v)", tree, objectType, err)
	}
	if _, err := destination.git(nil, "fsck", "--connectivity-only", "--no-dangling", tree); err != nil {
		return fmt.Errorf("gittree transfer: destination tree closure is incomplete: %w", err)
	}
	return nil
}

func rawCommandError(result RawResult) string {
	if detail := strings.TrimSpace(string(result.Stderr)); detail != "" {
		return detail
	}
	if result.Err != nil {
		return result.Err.Error()
	}
	if result.ExitDetail != "" {
		return result.ExitDetail
	}
	return fmt.Sprintf("exit status %d", result.ExitCode)
}

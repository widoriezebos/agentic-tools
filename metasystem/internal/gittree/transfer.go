package gittree

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
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

	producer := exec.Command("git", append(append([]string{"-C", sourceTop}, configPins...), "pack-objects", "--stdout", "--revs")...)
	producer.Env = ScrubbedEnviron()
	producer.Stdin = strings.NewReader(tree + "\n")
	producer.Stdout = pack
	var producerError bytes.Buffer
	producer.Stderr = &producerError
	if err := boundedexec.Run(producer, boundedexec.Timeout(filepath.Join(w.Dir, "metasystem.conf"), boundedexec.Local), "git pack-objects"); err != nil {
		_ = pack.Close()
		return fmt.Errorf("gittree transfer: create tree pack: %s", commandError(producerError.String(), err))
	}
	if err := pack.Close(); err != nil {
		return fmt.Errorf("gittree transfer: close tree pack: %w", err)
	}

	pack, err = os.Open(packPath)
	if err != nil {
		return fmt.Errorf("gittree transfer: reopen tree pack: %w", err)
	}
	consumer := exec.Command("git", append(append([]string{"-C", destinationTop}, configPins...), "index-pack", "--stdin")...)
	consumer.Env = ScrubbedEnviron()
	consumer.Stdin = pack
	var consumerOutput, consumerError bytes.Buffer
	consumer.Stdout, consumer.Stderr = &consumerOutput, &consumerError
	runErr := boundedexec.Run(consumer, boundedexec.Timeout(filepath.Join(destination.Dir, "metasystem.conf"), boundedexec.Local), "git index-pack")
	closeErr := pack.Close()
	if runErr != nil {
		return fmt.Errorf("gittree transfer: import tree pack: %s", commandError(consumerError.String(), runErr))
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

func commandError(stderr string, err error) string {
	if detail := strings.TrimSpace(stderr); detail != "" {
		return detail
	}
	return err.Error()
}

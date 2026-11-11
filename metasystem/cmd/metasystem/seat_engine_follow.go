package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/deploy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func followsDeployedEngine(installation string) bool {
	value, _, err := config.Get(config.GetParams{Key: "metasystem.engine-delivery", ConfPath: filepath.Join(installation, "metasystem.conf")})
	if err != nil || value != "central" {
		return false
	}
	_, err = deploy.Declared(installation, installation)
	return err == nil
}

func activeDeployedEngine(installation, root string) (steward.DeployedEngine, error) {
	project, err := resolveDeployProject(deployOwners{origin: deploy.OriginURL, home: board.Home}, installation, root)
	if err != nil {
		return steward.DeployedEngine{}, err
	}
	lines, err := deploy.Lines(project.dir)
	if err != nil {
		return steward.DeployedEngine{}, err
	}
	line := deploy.Current(lines)
	if line == nil {
		return steward.DeployedEngine{}, fmt.Errorf("this project has no active deployment")
	}
	engine := steward.DeployedEngine{Commit: line.Commit, Artifact: line.Artifact, Digest: "sha256:" + strings.TrimPrefix(line.Digest, "sha256:")}
	if engine.Commit == "" || engine.Artifact == "" || engine.Digest == "sha256:" {
		return engine, fmt.Errorf("the active deployment has an empty commit, artifact or digest")
	}
	file, err := os.Open(engine.Artifact)
	if err != nil {
		return engine, err
	}
	defer file.Close()
	stamp, err := enginebuild.ReadStamp(file)
	if err != nil {
		return engine, err
	}
	if stamp != engine.Commit {
		return engine, fmt.Errorf("the deployed artifact's stamp %q differs from its commit %s", stamp, engine.Commit)
	}
	sum, err := fileSHA256(engine.Artifact)
	if err == nil && "sha256:"+sum != engine.Digest {
		err = fmt.Errorf("the deployed artifact's sha256 differs from its deployment")
	}
	return engine, err
}

func installDeployedEngine(installation string, engine steward.DeployedEngine) error {
	return installDeployedEngineWithRename(installation, engine, os.Rename)
}

func installDeployedEngineWithRename(installation string, engine steward.DeployedEngine, rename func(string, string) error) error {
	source, err := os.Open(engine.Artifact)
	if err != nil {
		return err
	}
	defer source.Close()
	staging := filepath.Join(installation, "bin", fmt.Sprintf(".metasystem.follow-%d", os.Getpid()))
	defer func() { _ = os.Remove(staging) }()
	target, err := os.OpenFile(staging, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer target.Close()
	if _, err := io.Copy(target, source); err != nil {
		return err
	}
	for _, step := range []func() error{target.Sync, func() error { return target.Chmod(0o755) }, target.Close} {
		if err := step(); err != nil {
			return err
		}
	}
	sum, err := fileSHA256(staging)
	if err != nil {
		return err
	}
	if "sha256:"+sum != engine.Digest {
		return fmt.Errorf("the copied artifact's sha256 differs from its deployment")
	}
	return rename(staging, filepath.Join(installation, "bin", "metasystem"))
}

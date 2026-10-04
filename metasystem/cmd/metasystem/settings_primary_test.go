package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestTheLandingGateInAGoalWorktreeReadsThePrimarysHumanFromTier(t *testing.T) {
	primary, worktree := t.TempDir(), t.TempDir()
	for root, body := range map[string]string{primary: "launch.build.window.tokens=12000\n", worktree: "launch.build.window.tokens=7000\n"} {
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(primary, "metasystem.conf.local"), []byte("landing.review.human-from-tier=4\nlaunch.build.window.tokens=16000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	layout := stateroot.Layout{InstallationRoot: stateroot.Installation(worktree)}
	serving := func(root string) (string, string) {
		if root != worktree {
			t.Fatalf("serving root = %s; want %s", root, worktree)
		}
		return primary, ""
	}
	settings, err := landingGateSettings(layout.InstallationRoot.Path(), serving)
	if err != nil || settings.HumanFromTier != 4 {
		t.Fatalf("primary gate settings = %+v, %v", settings, err)
	}
	settings, err = landingGateSettings(layout.InstallationRoot.Path(), func(root string) (string, string) { return root, "" })
	if err != nil || settings.HumanFromTier != 2 {
		t.Fatalf("self-serving gate settings = %+v, %v", settings, err)
	}
	launchSettings, err := unitLaunchSettings(layout, serving, func(string) (string, bool) { return "", false })
	if err != nil || launchSettings.BuildWindow != 16000 {
		t.Fatalf("primary launch settings = %+v, %v", launchSettings, err)
	}
	launchSettings, err = unitLaunchSettings(layout, func(root string) (string, string) { return root, "" }, func(string) (string, bool) { return "", false })
	if err != nil || launchSettings.BuildWindow != 7000 {
		t.Fatalf("self-serving launch settings = %+v, %v", launchSettings, err)
	}
}

package launch

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

const PartnerOwnerLineage = "project-partner"

// PartnerCommand starts the runtime's ordinary interactive session. It inherits
// the terminal and replaces the caller, without a delegate or seat supervisor.
func PartnerCommand(runtime, model, directory string) (Command, error) {
	if runtime != "claude" && runtime != "codex" && runtime != "devin" {
		return Command{}, fmt.Errorf("unknown partner runtime %q; choose claude, codex or devin", runtime)
	}
	program, err := exec.LookPath(runtime)
	if err != nil {
		return Command{}, fmt.Errorf("the partner runtime %s is not installed on PATH; install %s, then run metasystem partner start", runtime, runtime)
	}
	command := Command{Program: program, Directory: directory, Environment: []string{
		"METASYSTEM_OWNER_LINEAGE=" + PartnerOwnerLineage, "METASYSTEM_DELEGATE_ROOT=", "METASYSTEM_SESSION_ID=",
	}}
	if model != "" {
		command.Args = []string{"--model", model}
	}
	return command, nil
}

func ReplaceWithPartner(command Command) error {
	if err := os.Chdir(command.Directory); err != nil {
		return err
	}
	env := os.Environ()
	for _, setting := range command.Environment {
		key, _, _ := strings.Cut(setting, "=")
		for i := len(env) - 1; i >= 0; i-- {
			if strings.HasPrefix(env[i], key+"=") {
				env = append(env[:i], env[i+1:]...)
			}
		}
		env = append(env, setting)
	}
	return syscall.Exec(command.Program, append([]string{command.Program}, command.Args...), env)
}

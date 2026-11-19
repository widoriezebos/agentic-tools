package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	directory := os.Args[1]
	if len(os.Args) > 2 && os.Args[2] == "custodian" {
		_ = syscall.Kill(syscall.Getpgrp(), syscall.SIGKILL)
		return
	}
	signal.Ignore(syscall.SIGTERM)
	if len(os.Args) > 2 && os.Args[2] == "child" {
		_ = os.WriteFile(filepath.Join(directory, "child"), []byte(fmt.Sprint(os.Getpid())), 0600)
		ready := os.NewFile(3, "ready")
		_, _ = ready.Write([]byte{1})
		_ = ready.Close()
	} else {
		read, write, err := os.Pipe()
		if err != nil {
			panic(err)
		}
		defer read.Close()
		command := exec.Command(os.Args[0], directory, "child")
		command.ExtraFiles = []*os.File{write}
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		if err := command.Start(); err != nil {
			panic(err)
		}
		_ = write.Close()
		var ready [1]byte
		_, _ = read.Read(ready[:])
		if len(os.Args) > 2 && os.Args[2] == "exit" {
			return
		}
	}
	file, err := os.OpenFile(filepath.Join(directory, "hold"), os.O_RDONLY, 0)
	if err != nil {
		panic(err)
	}
	var one [1]byte
	_, _ = file.Read(one[:])
}

package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestBasicRun(t *testing.T) {
	os.Setenv("GOBIN", "blah")
	os.Setenv("GOPATH", "wha")
	str, err := os.Getwd()
	if err != nil {
		t.Fatalf("WHAAAA %v", err)
	}
	rc := &rCommand{command: exec.Command(str + "/run.sh")}
	err = run(rc)
	if err != nil {
		t.Fatalf("Error running command: %v", err)
	}
	rc.command.Wait()

	if rc.output != "hello" {
		t.Errorf("No output: %v", rc)
	}
}

func TestAppendRun(t *testing.T) {
	os.Unsetenv("GOBIN")
	os.Unsetenv("GOPATH")
	str, err := os.Getwd()
	if err != nil {
		t.Fatalf("WHAAAA %v", err)
	}
	rc := &rCommand{command: exec.Command(str + "/run.sh")}
	err = run(rc)
	if err != nil {
		t.Fatalf("Error running command: %v", err)
	}
	rc.command.Wait()

	if rc.output != "hello" {
		t.Errorf("No output: %v", rc)
	}
}

func TestBadCommand(t *testing.T) {
	rc := &rCommand{command: exec.Command("run.sh")}
	err := run(rc)
	if err == nil {
		t.Errorf("No error running command")
	}
}

package main

import (
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

func TestRandomComplete(t *testing.T) {
	s := Scheduler{rMutex: &sync.Mutex{}, cMutex: &sync.Mutex{}, rMap: make(map[string]*rCommand)}
	if s.schedulerComplete("madeup") {
		t.Errorf("Made up lookup has not failed")
	}
}

func TestMarkComplete(t *testing.T) {
	str, err := os.Getwd()
	if err != nil {
		t.Fatalf("WHAAAA %v", err)
	}
	rc := &rCommand{command: exec.Command(str + "/run.sh")}
	s := Scheduler{rMutex: &sync.Mutex{}, cMutex: &sync.Mutex{}, rMap: make(map[string]*rCommand)}
	s.Schedule("running", rc)
	s.markComplete("running")

	if rc.endTime == 0 {
		t.Errorf("Mark complete failed")
	}
}

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
	for rc.endTime == 0 {
		time.Sleep(time.Millisecond * 100)
	}

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

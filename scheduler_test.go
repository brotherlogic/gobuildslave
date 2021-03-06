package main

import (
	"log"
	"os"
	"os/exec"

	"testing"
	"time"
)

func Log(s string) {
	log.Printf(s)
}
func InitTestScheduler() Scheduler {
	s := Scheduler{blockingQueue: make(chan *rCommand), nonblockingQueue: make(chan *rCommand), complete: make([]*rCommand, 0)}
	go s.processBlockingCommands()
	go s.processNonblockingCommands()
	return s
}

func TestGetErr(t *testing.T) {
	s := InitTestScheduler()
	s.complete = append(s.complete, &rCommand{key: "balls", block: true})
	_, err := s.getErrOutput("balls")
	if err != nil {
		t.Errorf("Bad err get")
	}
}

func TestGetState(t *testing.T) {
	s := InitTestScheduler()
	state := s.getState("blah")
	if state != "UNKNOWN" {
		t.Errorf("Error: %v", state)
	}
}
func TestGetStateGot(t *testing.T) {
	s := InitTestScheduler()
	s.complete = append(s.complete, &rCommand{key: "balls", block: true})
	state := s.getState("balls")
	if state == "UNKNOWN" {
		t.Errorf("Error: %v", state)
	}
}

func TestBadRun(t *testing.T) {
	s := InitTestScheduler()
	s.Log = Log
	rc := &rCommand{command: exec.Command("balls"), block: true}
	s.Schedule(rc)
}

func TestEmptyState(t *testing.T) {
	s := InitTestScheduler()
	state := s.getState("blah")
	if state != "UNKNOWN" {
		t.Errorf("Weird state: %v", state)
	}
}

func TestKillSchedJob(t *testing.T) {
	os.Unsetenv("GOBIN")
	os.Unsetenv("GOPATH")
	str, err := os.Getwd()
	if err != nil {
		t.Fatalf("WHAAAA %v", err)
	}
	rc := &rCommand{command: exec.Command(str + "/run.sh")}
	run(rc)
}

func TestCleanSchedJob(t *testing.T) {
	os.Unsetenv("GOBIN")
	os.Unsetenv("GOPATH")
	str, err := os.Getwd()
	if err != nil {
		t.Fatalf("WHAAAA %v", err)
	}
	rc := &rCommand{command: exec.Command(str + "/run.sh"), endTime: time.Now().Add(-time.Hour).Unix()}
	run(rc)
}

func TestFailStderr(t *testing.T) {
	os.Unsetenv("GOBIN")
	os.Unsetenv("GOPATH")
	str, err := os.Getwd()
	if err != nil {
		t.Fatalf("WHAAAA %v", err)
	}
	rc := &rCommand{command: exec.Command(str + "/run.sh"), crash1: true}
	err = run(rc)
}

func TestFailStdout(t *testing.T) {
	os.Unsetenv("GOBIN")
	os.Unsetenv("GOPATH")
	str, err := os.Getwd()
	if err != nil {
		t.Fatalf("WHAAAA %v", err)
	}
	rc := &rCommand{command: exec.Command(str + "/run.sh"), crash2: true}
	err = run(rc)
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
		time.Sleep(time.Second)
	}

	if rc.output != "hello" {
		t.Errorf("No output: %v given %v from running %v -> %v", rc.output, rc.endTime, rc.command, rc.mainOut)
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
	for rc.endTime == 0 {
		time.Sleep(time.Second)
	}

	if rc.output != "hello" {
		t.Errorf("No output: %v given %v from running %v -> %v", rc.output, rc.endTime, rc.command, rc.mainOut)
	}
}

func TestBadCommand(t *testing.T) {
	rc := &rCommand{command: exec.Command("run.sh")}
	err := run(rc)
	if err == nil {
		t.Errorf("No error running command")
	}
}

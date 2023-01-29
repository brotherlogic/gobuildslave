package main

import (
	"context"
	"log"
	"os/exec"

	"testing"
)

func Log(ctx context.Context, s string) {
	log.Print(s)
}
func InitTestScheduler() Scheduler {
	s := Scheduler{blockingQueue: make(chan *rCommand), nonblockingQueue: make(chan *rCommand), complete: make([]*rCommand, 0)}
	s.Log = Log
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

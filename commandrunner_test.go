package main

import (
	"log"
	"os/exec"
	"testing"
	"time"

	pb "github.com/brotherlogic/gobuildslave/proto"
)

func InitTest() *Runner {
	r := &Runner{}
	r.runner = testRunCommand

	go r.run()

	return r
}

func testRunCommand(c *runnerCommand) {
	//We do nothing
	log.Printf("WHAT")
	time.Sleep(200 * time.Millisecond)
	log.Printf("SLEPT")
	c.complete = true
}

func TestRun(t *testing.T) {
	r := InitTest()
	r.Run(&pb.JobSpec{Name: "testrepo"})
	r.LameDuck(true)
	if r.commandsRun != 1 {
		t.Errorf("Not enough commands: (%v) %v", r.commandsRun, r.commands)
	}
	if len(r.backgroundTasks) != 1 {
		t.Errorf("Not enough background tasks running %v", len(r.backgroundTasks))
	}
}

func TestCheckout(t *testing.T) {
	r := InitTest()
	log.Printf("TESTREPO CHECKOUT")
	r.Checkout("testrepo")
	log.Printf("LAMEDUCKING")
	r.LameDuck(true)

	if r.commandsRun != 2 {
		t.Errorf("Not enough commands: %v", r.commands)
	}
}

func TestUpdate(t *testing.T) {
	command := exec.Command("ls", "")
	com := &runnerCommand{command: command, details: &pb.JobDetails{}}
	updateState(com)
	if !com.details.Running {
		t.Errorf("Problem with testing update: %v", com.details)
	}

	command.Start()
	command.Wait()

	updateState(com)
	if com.details.Running {
		t.Errorf("Problem with testing update: %v", com.details)
	}
}

func TestLongUpdate(t *testing.T) {
	command := exec.Command("sleep 1", "")
	com := &runnerCommand{command: command, details: &pb.JobDetails{}}
	updateState(com)
	if !com.details.Running {
		t.Errorf("Problem with testing update: %v", com.details)
	}

	command.Start()

	updateState(com)
	if !com.details.Running {
		t.Errorf("Problem with testing update: %v", com.details)
	}
}

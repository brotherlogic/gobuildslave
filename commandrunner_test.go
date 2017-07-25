package main

import (
	"log"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/context"

	pb "github.com/brotherlogic/gobuildslave/proto"
)

type testDiskChecker struct{}

func (diskChecker testDiskChecker) diskUsage(path string) int64 {
	if strings.HasSuffix(path, "1") {
		return int64(200)
	}

	return -1
}

func InitTest() *Runner {
	r := &Runner{}
	r.runner = testRunCommand

	go r.run()

	return r
}

func TestDiskUsage(t *testing.T) {
	v := diskUsage("/")
	if v <= 0 {
		t.Errorf("Error getting disk usage: %v", v)
	}
}

func TestDiskUsageFail(t *testing.T) {
	v := diskUsage("/madeuppath")
	if v > 0 {
		t.Errorf("Disk usage on made up path did not fail")
	}
}

func TestGetMachineCapabilities(t *testing.T) {
	s := Server{}
	s.disk = testDiskChecker{}
	props, err := s.GetConfig(context.Background(), &pb.Empty{})

	if err != nil {
		t.Fatalf("Get Config has returned an error: %v", err)
	}

	if props.Disk == 0 || props.Memory == 0 {
		t.Errorf("Failed to pull machine details: %v", props)
	}
}

func testRunCommand(c *runnerCommand) {
	oldPath := c.command.Path
	oldArgs := c.command.Args
	//We do nothing
	log.Printf("RUNNING COMMAND %v", c.command.Path)
	if strings.Contains(c.command.Path, "repols") {

		c.command.Path = "/bin/sleep"
		c.command.Args = []string{"10"}
		log.Printf("RUNNING %v", c.command)
		c.command.Start()
		log.Printf("PROC = %v", c.command.Process)
	}

	time.Sleep(200 * time.Millisecond)
	log.Printf("SLEPT")
	c.command.Path = oldPath
	c.command.Args = oldArgs
	c.complete = true
}

func TestRun(t *testing.T) {
	r := InitTest()
	r.Run(&pb.JobSpec{Name: "testrepo"})
	r.LameDuck(true)
	if r.commandsRun != 3 {
		t.Errorf("Not enough commands: (%v) %v", r.commandsRun, r.commands)
	}
	if len(r.backgroundTasks) != 1 {
		t.Errorf("Not enough background tasks running %v", len(r.backgroundTasks))
	}
}

func TestRunWithAtuguments(t *testing.T) {
	r := InitTest()
	r.Run(&pb.JobSpec{Name: "testrepo", Args: []string{"--argkey", "argvalue"}})
	r.LameDuck(true)
	if r.commandsRun != 3 {
		t.Fatalf("Not enough commands: (%v) %v", r.commandsRun, r.commands)
	}
	log.Printf("HERE = %v, %v", r.commands, len(r.commands))
	if len(r.commands) != 3 {
		log.Printf("HALP")
	}
	if len(r.runCommands) != 3 || len(r.runCommands[2].command.Args) != 3 || r.runCommands[2].command.Args[1] != "--argkey" {
		t.Fatalf("Command has wrong args: %v", r.runCommands[2].command.Args[1])
	}
	if len(r.backgroundTasks) != 1 {
		t.Errorf("Not enough background tasks running %v", len(r.backgroundTasks))
	}
}

func TestRebuild(t *testing.T) {
	r := InitTest()
	r.Run(&pb.JobSpec{Name: "testrepo-rebuild"})
	log.Printf("Requesting rebuild")
	r.Rebuild(&pb.JobSpec{Name: "testrepo-rebuild"}, "madeuphash")
	r.LameDuck(true)
	if r.commandsRun != 9 {
		t.Errorf("Not enough commands: (%v) %v", r.commandsRun, r.commands)
	}
	if len(r.backgroundTasks) != 1 {
		t.Errorf("Not enough background tasks running %v", len(r.backgroundTasks))
	}
}

func TestDoubleRun(t *testing.T) {
	r := InitTest()
	r.Run(&pb.JobSpec{Name: "testrepo"})
	r.Run(&pb.JobSpec{Name: "testrepo"})
	r.LameDuck(true)

	if r.commandsRun != 7 {
		t.Errorf("Wrong number of commands: (%v) %v", r.commandsRun, r.commands)
	}

	if len(r.backgroundTasks) != 1 {
		t.Errorf("Wrong number of tasks runnning %v", r.backgroundTasks)
	}
}

func TestUpdate(t *testing.T) {
	r := InitTest()
	r.Run(&pb.JobSpec{Name: "testrepo"})
	r.Update(&pb.JobSpec{Name: "testrepo", Args: []string{"arg1"}})
	r.LameDuck(true)

	if r.commandsRun != 7 {
		t.Errorf("Wrong number of commands: (%v) %v", r.commandsRun, r.commands)
	}

	if len(r.backgroundTasks) != 1 {
		t.Fatalf("Wrong number of tasks runnning %v", r.backgroundTasks)
	}

	if len(r.backgroundTasks[0].command.Args) != 2 {
		t.Errorf("No args: %v", r.backgroundTasks[0].command.Args)
	}

}

func TestKill(t *testing.T) {
	r := InitTest()
	r.Run(&pb.JobSpec{Name: "testrepols"})
	r.LameDuck(true)
	r.kill(&pb.JobSpec{Name: "testrepols"})

	if r.commandsRun != 4 {
		t.Errorf("Not enough commands: (%v) %v", r.commandsRun, r.commands)
	}
	if len(r.backgroundTasks) != 0 {
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
